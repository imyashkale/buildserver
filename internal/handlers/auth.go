package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/imyashkale/buildserver/internal/models"
	"github.com/imyashkale/buildserver/internal/repository"
	"github.com/imyashkale/buildserver/internal/services"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	githubRepo    repository.GitHubRepository
	githubService *services.GitHubService
}

// NewAuthHandler creates a new AuthHandler instance
func NewAuthHandler(githubRepo repository.GitHubRepository, githubService *services.GitHubService) *AuthHandler {
	return &AuthHandler{
		githubRepo:    githubRepo,
		githubService: githubService,
	}
}

// InitiateGitHubOAuth initiates the GitHub OAuth flow
// POST /api/v1/auth/github/connect
func (h *AuthHandler) InitiateGitHubOAuth(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userId, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "unauthorized",
			Message: "User ID not found in context",
		})
		return
	}

	userIdStr, ok := userId.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "internal_error",
			Message: "Invalid user ID format",
		})
		return
	}

	// Generate authorization URL and state token
	authURL, state, err := h.githubService.GenerateAuthURL(c.Request.Context(), userIdStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "failed_to_generate_auth_url",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.InitiateGitHubOAuthResponse{
		AuthorizationUrl: authURL,
		State:            state,
	})
}

// HandleGitHubCallback handles the GitHub OAuth callback
// POST /api/v1/auth/github/callback
func (h *AuthHandler) HandleGitHubCallback(c *gin.Context) {
	// Get user ID from context
	userId, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "unauthorized",
			Message: "User ID not found in context",
		})
		return
	}

	userIdStr, ok := userId.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "internal_error",
			Message: "Invalid user ID format",
		})
		return
	}

	// Bind request body
	var req models.GitHubCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	// Verify state token
	if err := h.githubService.VerifyStateToken(c.Request.Context(), req.State, userIdStr); err != nil {
		if errors.Is(err, services.ErrInvalidStateToken) {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "invalid_state",
				Message: "State token mismatch (CSRF protection)",
			})
			return
		}
		if errors.Is(err, services.ErrStateTokenExpired) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "state_expired",
				Message: "State token has expired",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "state_verification_failed",
			Message: err.Error(),
		})
		return
	}

	// Exchange code for access token
	accessToken, err := h.githubService.ExchangeCodeForToken(c.Request.Context(), req.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "token_exchange_failed",
			Message: err.Error(),
		})
		return
	}

	// Get GitHub user information
	githubUser, err := h.githubService.GetGitHubUser(c.Request.Context(), accessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "failed_to_get_github_user",
			Message: err.Error(),
		})
		return
	}

	fmt.Println("GitHub User:", githubUser)

	// Encrypt access token
	encryptedToken, err := h.githubService.EncryptToken(accessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "token_encryption_failed",
			Message: err.Error(),
		})
		return
	}

	// Store GitHub connection
	connection := &models.GitHubConnection{
		Id:             uuid.New().String(),
		UserId:         userIdStr,
		GitHubUserId:   githubUser.Id,
		GitHubUsername: githubUser.Login,
		AccessToken:    encryptedToken,
		GitHubUserData: map[string]interface{}{
			"login":        githubUser.Login,
			"id":           githubUser.Id,
			"avatar_url":   githubUser.AvatarUrl,
			"name":         githubUser.Name,
			"email":        githubUser.Email,
			"bio":          githubUser.Bio,
			"public_repos": githubUser.PublicRepos,
		},
		ConnectedAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Check if connection already exists
	exists, existsErr := h.githubRepo.ConnectionExists(c.Request.Context(), userIdStr)
	if existsErr != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "database_error",
			Message: existsErr.Error(),
		})
		return
	}

	if exists {
		// Update existing connection
		if err := h.githubRepo.UpdateConnection(c.Request.Context(), connection); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "failed_to_update_connection",
				Message: err.Error(),
			})
			return
		}
	} else {
		// Create new connection
		if err := h.githubRepo.CreateConnection(c.Request.Context(), connection); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "failed_to_create_connection",
				Message: err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, models.GitHubCallbackResponse{
		Success: true,
		User:    githubUser,
	})
}

// GetGitHubStatus returns the GitHub connection status for the authenticated user
// GET /api/v1/auth/github/status
func (h *AuthHandler) GetGitHubStatus(c *gin.Context) {
	// Get user ID from context
	userId, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "unauthorized",
			Message: "User ID not found in context",
		})
		return
	}

	userIdStr, ok := userId.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "internal_error",
			Message: "Invalid user ID format",
		})
		return
	}

	// Get GitHub connection
	connection, err := h.githubRepo.GetConnectionByUserId(c.Request.Context(), userIdStr)
	if err != nil {
		if errors.Is(err, repository.ErrGitHubConnectionNotFound) {
			c.JSON(http.StatusOK, models.GitHubStatusResponse{
				Connected: false,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "database_error",
			Message: err.Error(),
		})
		return
	}

	// Convert to GitHubUser
	githubUser := connection.ToGitHubUser()

	c.JSON(http.StatusOK, models.GitHubStatusResponse{
		Connected:   true,
		User:        githubUser,
		ConnectedAt: &connection.ConnectedAt,
	})
}

// DisconnectGitHub disconnects the user's GitHub account
// DELETE /api/v1/auth/github/disconnect
func (h *AuthHandler) DisconnectGitHub(c *gin.Context) {
	// Get user ID from context
	userId, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "unauthorized",
			Message: "User ID not found in context",
		})
		return
	}

	userIdStr, ok := userId.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "internal_error",
			Message: "Invalid user ID format",
		})
		return
	}

	// Get existing connection to retrieve access token
	connection, err := h.githubRepo.GetConnectionByUserId(c.Request.Context(), userIdStr)
	if err != nil {
		if errors.Is(err, repository.ErrGitHubConnectionNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_connected",
				Message: "No GitHub connection found for user",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "database_error",
			Message: err.Error(),
		})
		return
	}

	// Decrypt access token
	accessToken, err := h.githubService.DecryptToken(connection.AccessToken)
	if err != nil {
		// Log error but continue with deletion
		c.Error(err)
	} else {
		// Revoke token on GitHub (optional, best effort)
		if err := h.githubService.RevokeGitHubToken(c.Request.Context(), accessToken); err != nil {
			// Log error but continue with deletion
			c.Error(err)
		}
	}

	// Delete connection from database
	if err := h.githubRepo.DeleteConnection(c.Request.Context(), userIdStr); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "failed_to_disconnect",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.DisconnectGitHubResponse{
		Success: true,
		Message: "GitHub account disconnected successfully",
	})
}
