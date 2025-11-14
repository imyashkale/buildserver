package services

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/imyashkale/buildserver/internal/logger"
	"github.com/imyashkale/buildserver/internal/models"
	"github.com/imyashkale/buildserver/internal/repository"
)

var (
	ErrInvalidStateToken     = errors.New("invalid state token")
	ErrStateTokenExpired     = errors.New("state token expired")
	ErrGitHubAPIError        = errors.New("github api error")
	ErrTokenEncryptionFailed = errors.New("token encryption failed")
	ErrTokenDecryptionFailed = errors.New("token decryption failed")
)

// GitHubService handles GitHub OAuth and API operations
type GitHubService struct {
	repo          repository.GitHubRepository
	clientID      string
	clientSecret  string
	redirectURL   string
	encryptionKey []byte
}

// NewGitHubService creates a new GitHubService instance
func NewGitHubService(
	repo repository.GitHubRepository,
	clientID string,
	clientSecret string,
	redirectURL string,
	encryptionKey string,
) *GitHubService {
	return &GitHubService{
		repo:          repo,
		clientID:      clientID,
		clientSecret:  clientSecret,
		redirectURL:   redirectURL,
		encryptionKey: []byte(encryptionKey),
	}
}

// VerifyStateToken verifies the OAuth state token
func (s *GitHubService) VerifyStateToken(ctx context.Context, stateToken string, userId string) error {

	// Get state from database
	state, err := s.repo.GetOAuthState(ctx, stateToken)
	if err != nil {
		if errors.Is(err, repository.ErrOAuthStateNotFound) {
			return ErrInvalidStateToken
		}
		return fmt.Errorf("failed to get oauth state: %w", err)
	}

	// Verify state belongs to user
	if state.UserId != userId {
		return ErrInvalidStateToken
	}

	// Check if expired
	if time.Now().After(state.ExpiresAt) {
		// Delete expired state
		_ = s.repo.DeleteOAuthState(ctx, state.Id)
		return ErrStateTokenExpired
	}

	// Delete state after verification (one-time use)
	if err := s.repo.DeleteOAuthState(ctx, state.Id); err != nil {
		return fmt.Errorf("failed to delete oauth state: %w", err)
	}

	return nil
}

// GetGitHubUser fetches user information from GitHub API
func (s *GitHubService) GetGitHubUser(ctx context.Context, accessToken string) (*models.GitHubUser, error) {
	logger.Debug("Fetching GitHub user information")

	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		"https://api.github.com/user",
		nil,
	)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Failed to create GitHub API request")
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.WithField("error", err.Error()).Error("GitHub API request failed")
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.WithField("status_code", resp.StatusCode).Warn("GitHub API returned non-OK status")
		return nil, fmt.Errorf("%w: status code %d", ErrGitHubAPIError, resp.StatusCode)
	}

	var user models.GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		logger.WithField("error", err.Error()).Error("Failed to decode GitHub user response")
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	logger.WithField("username", user.Login).Info("GitHub user information fetched successfully")
	return &user, nil
}

// GetUserRepositories fetches user repositories from GitHub API
func (s *GitHubService) GetUserRepositories(ctx context.Context, accessToken string, page, perPage int) ([]models.GitHubRepository, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 30
	}

	logger.WithFields(map[string]interface{}{
		"page":     page,
		"per_page": perPage,
	}).Debug("Fetching user repositories from GitHub")

	url := fmt.Sprintf(
		"https://api.github.com/user/repos?page=%d&per_page=%d&sort=updated&affiliation=owner,collaborator",
		page,
		perPage,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Failed to create GitHub API request")
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.WithField("error", err.Error()).Error("GitHub API request for repositories failed")
		return nil, fmt.Errorf("failed to get repositories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.WithField("status_code", resp.StatusCode).Warn("GitHub API returned non-OK status for repositories")
		return nil, fmt.Errorf("%w: status code %d", ErrGitHubAPIError, resp.StatusCode)
	}

	var repos []models.GitHubRepository
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		logger.WithField("error", err.Error()).Error("Failed to decode GitHub repositories response")
		return nil, fmt.Errorf("failed to decode repositories: %w", err)
	}

	logger.WithField("repo_count", len(repos)).Info("GitHub repositories fetched successfully")
	return repos, nil
}

// SearchUserRepositories searches user repositories from GitHub API
func (s *GitHubService) SearchUserRepositories(ctx context.Context, accessToken, username, query string) ([]models.GitHubRepository, error) {
	searchURL := fmt.Sprintf(
		"https://api.github.com/search/repositories?q=%s+user:%s",
		url.QueryEscape(query),
		url.QueryEscape(username),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search repositories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status code %d", ErrGitHubAPIError, resp.StatusCode)
	}

	var result struct {
		Items []models.GitHubRepository `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	return result.Items, nil
}

// EncryptToken encrypts a token using AES-256
func (s *GitHubService) EncryptToken(token string) (string, error) {

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenEncryptionFailed, err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenEncryptionFailed, err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenEncryptionFailed, err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(token), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptToken decrypts a token using AES-256
func (s *GitHubService) DecryptToken(encryptedToken string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedToken)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenDecryptionFailed, err)
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenDecryptionFailed, err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenDecryptionFailed, err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("%w: ciphertext too short", ErrTokenDecryptionFailed)
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenDecryptionFailed, err)
	}

	return string(plaintext), nil
}

// GetRepositoryBranches fetches branches for a specific repository from GitHub API
func (s *GitHubService) GetRepositoryBranches(ctx context.Context, accessToken, owner, repo string) ([]models.GitHubBranch, error) {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/branches",
		owner,
		repo,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get branches: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status code %d", ErrGitHubAPIError, resp.StatusCode)
	}

	var branches []models.GitHubBranch
	if err := json.NewDecoder(resp.Body).Decode(&branches); err != nil {
		return nil, fmt.Errorf("failed to decode branches: %w", err)
	}

	return branches, nil
}
