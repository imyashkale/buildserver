package services

import (
	"bytes"
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

	"github.com/google/uuid"
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

// GenerateAuthURL generates a GitHub OAuth authorization URL with state token
func (s *GitHubService) GenerateAuthURL(ctx context.Context, userId string) (string, string, error) {
	// Generate secure state token
	stateToken := uuid.New().String()

	// Create OAuth state record
	oauthState := &models.OAuthState{
		Id:         uuid.New().String(),
		UserId:     userId,
		StateToken: stateToken,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(15 * time.Minute),
	}

	// Save state to database
	if err := s.repo.CreateOAuthState(ctx, oauthState); err != nil {
		return "", "", fmt.Errorf("failed to create oauth state: %w", err)
	}

	// Build authorization URL
	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&state=%s&redirect_uri=%s&scope=repo,user",
		s.clientID,
		stateToken,
		url.QueryEscape(s.redirectURL),
	)

	return authURL, stateToken, nil
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

// ExchangeCodeForToken exchanges the authorization code for an access token
func (s *GitHubService) ExchangeCodeForToken(ctx context.Context, code string) (string, error) {
	// Prepare request to exchange code for token
	data := url.Values{}
	data.Set("client_id", s.clientID)
	data.Set("client_secret", s.clientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", s.redirectURL)

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"https://github.com/login/oauth/access_token",
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.URL.RawQuery = data.Encode()
	req.Header.Set("Accept", "application/json")

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: status code %d", ErrGitHubAPIError, resp.StatusCode)
	}

	// Parse response
	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != "" {
		return "", fmt.Errorf("%w: %s - %s", ErrGitHubAPIError, result.Error, result.ErrorDesc)
	}

	return result.AccessToken, nil
}

// GetGitHubUser fetches user information from GitHub API
func (s *GitHubService) GetGitHubUser(ctx context.Context, accessToken string) (*models.GitHubUser, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		"https://api.github.com/user",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status code %d", ErrGitHubAPIError, resp.StatusCode)
	}

	var user models.GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

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

	url := fmt.Sprintf(
		"https://api.github.com/user/repos?page=%d&per_page=%d&sort=updated&affiliation=owner,collaborator",
		page,
		perPage,
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
		return nil, fmt.Errorf("failed to get repositories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status code %d", ErrGitHubAPIError, resp.StatusCode)
	}

	var repos []models.GitHubRepository
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("failed to decode repositories: %w", err)
	}

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

// RevokeGitHubToken revokes a GitHub access token
func (s *GitHubService) RevokeGitHubToken(ctx context.Context, accessToken string) error {
	url := fmt.Sprintf("https://api.github.com/applications/%s/token", s.clientID)

	reqBody := map[string]string{
		"access_token": accessToken,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Use basic auth with client credentials
	req.SetBasicAuth(s.clientID, s.clientSecret)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("%w: status code %d", ErrGitHubAPIError, resp.StatusCode)
	}

	return nil
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
