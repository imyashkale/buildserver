package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/imyashkale/buildserver/internal/models"
	"github.com/imyashkale/buildserver/internal/repository"
	"github.com/imyashkale/buildserver/internal/services"
)

// GitHubHandler handles GitHub repository-related requests
type GitHubHandler struct {
	githubRepo    repository.GitHubRepository
	mcpRepo       repository.MCPRepository
	githubService *services.GitHubService
}

// NewGitHubHandler creates a new GitHubHandler instance
func NewGitHubHandler(githubRepo repository.GitHubRepository, mcpRepo repository.MCPRepository, githubService *services.GitHubService) *GitHubHandler {
	return &GitHubHandler{
		githubRepo:    githubRepo,
		mcpRepo:       mcpRepo,
		githubService: githubService,
	}
}

// ListRepositories lists the user's GitHub repositories
// GET /api/v1/github/repositories
func (h *GitHubHandler) ListRepositories(c *gin.Context) {
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
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "not_connected",
				Message: "GitHub account not connected",
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
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "decryption_failed",
			Message: err.Error(),
		})
		return
	}

	// Parse pagination parameters
	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	perPage := 30
	if perPageStr := c.Query("per_page"); perPageStr != "" {
		if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 100 {
			perPage = pp
		}
	}

	// Fetch repositories from GitHub
	repos, err := h.githubService.GetUserRepositories(c.Request.Context(), accessToken, page, perPage)
	if err != nil {
		if errors.Is(err, services.ErrGitHubAPIError) {
			c.JSON(http.StatusBadGateway, models.ErrorResponse{
				Error:   "github_api_error",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "failed_to_fetch_repositories",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.GitHubRepositoryListResponse(repos))
}

// SearchRepositories searches the user's GitHub repositories
// GET /api/v1/github/repositories/search
func (h *GitHubHandler) SearchRepositories(c *gin.Context) {
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

	// Get search query
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "missing_query",
			Message: "Search query parameter 'q' is required",
		})
		return
	}

	// Get GitHub connection
	connection, err := h.githubRepo.GetConnectionByUserId(c.Request.Context(), userIdStr)
	if err != nil {
		if errors.Is(err, repository.ErrGitHubConnectionNotFound) {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "not_connected",
				Message: "GitHub account not connected",
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
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "decryption_failed",
			Message: err.Error(),
		})
		return
	}

	// Search repositories from GitHub
	repos, err := h.githubService.SearchUserRepositories(c.Request.Context(), accessToken, connection.GitHubUsername, query)
	if err != nil {
		if errors.Is(err, services.ErrGitHubAPIError) {
			c.JSON(http.StatusBadGateway, models.ErrorResponse{
				Error:   "github_api_error",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "failed_to_search_repositories",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.GitHubRepositoryListResponse(repos))
}

// ListBranches lists branches for a specific MCP's repository
// GET /api/v1/mcps/:id/branches
func (h *GitHubHandler) ListBranches(c *gin.Context) {
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

	// Get MCP ID from URL parameter
	mcpId := c.Param("id")
	if mcpId == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "missing_mcp_id",
			Message: "MCP ID is required",
		})
		return
	}

	// Get MCP by ID
	mcp, err := h.mcpRepo.GetByID(c.Request.Context(), mcpId)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "mcp_not_found",
				Message: "MCP server not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "database_error",
			Message: err.Error(),
		})
		return
	}

	// Verify ownership
	if mcp.UserId != userIdStr {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "forbidden",
			Message: "You don't have permission to access this MCP server",
		})
		return
	}

	// Parse repository field (format: "owner/repo")
	repoParts := parseRepository(mcp.Repository)
	if len(repoParts) != 2 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_repository_format",
			Message: "Repository format should be 'owner/repo'",
		})
		return
	}

	// Extract owner and repo
	owner := repoParts[0]
	repo := repoParts[1]

	// Get GitHub connection
	connection, err := h.githubRepo.GetConnectionByUserId(c.Request.Context(), userIdStr)
	if err != nil {
		if errors.Is(err, repository.ErrGitHubConnectionNotFound) {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "not_connected",
				Message: "GitHub account not connected",
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
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "decryption_failed",
			Message: err.Error(),
		})
		return
	}

	// Fetch branches from GitHub
	branches, err := h.githubService.GetRepositoryBranches(c.Request.Context(), accessToken, owner, repo)
	if err != nil {
		if errors.Is(err, services.ErrGitHubAPIError) {
			c.JSON(http.StatusBadGateway, models.ErrorResponse{
				Error:   "github_api_error",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "failed_to_fetch_branches",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"branches": branches,
		"total":    len(branches),
	})
}

// parseRepository parses a repository string in format "owner/repo"
func parseRepository(repository string) []string {
	// Handle both "owner/repo" and full GitHub URLs
	if repository == "" {
		return nil
	}

	// If it's a URL, extract owner/repo from it
	if len(repository) > 4 && (repository[:4] == "http" || repository[:3] == "git") {
		// Extract from URLs like https://github.com/owner/repo or git@github.com:owner/repo.git
		parts := make([]string, 0)
		if idx := strings.Index(repository, "github.com"); idx != -1 {
			remainder := repository[idx+len("github.com"):]
			// Remove leading : or /
			remainder = strings.TrimPrefix(remainder, ":")
			remainder = strings.TrimPrefix(remainder, "/")
			// Remove trailing .git
			remainder = strings.TrimSuffix(remainder, ".git")
			parts = strings.Split(remainder, "/")
			if len(parts) >= 2 {
				return []string{parts[0], parts[1]}
			}
		}
		return nil
	}

	// Simple case: "owner/repo"
	return strings.Split(repository, "/")
}
