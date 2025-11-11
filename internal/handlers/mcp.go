package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/imyashkale/buildserver/internal/models"
	"github.com/imyashkale/buildserver/internal/repository"
)

// MCPHandler handles MCP server-related requests
type MCPHandler struct {
	repo repository.MCPRepository
}

// NewMCPHandler creates a new MCP handler
func NewMCPHandler(repo repository.MCPRepository) *MCPHandler {
	return &MCPHandler{
		repo: repo,
	}
}

// Create handles creating a new MCP server
func (h *MCPHandler) Create(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userId, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "User ID not found in context",
		})
		return
	}

	userIdStr, ok := userId.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Invalid user ID format",
		})
		return
	}

	var req models.CreateMCPServerRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
		})
		return
	}

	// Convert request DTO to domain model
	server := req.ToDomain()

	// Generate a new UUID for the server ID
	server.Id = uuid.New().String()

	// Set the user ID
	server.UserId = userIdStr

	// Initialize empty slices if nil
	if server.EnvironmentVariables == nil {
		server.EnvironmentVariables = []models.EnvironmentVariable{}
	}

	// Store in repository
	if err := h.repo.Create(c.Request.Context(), server); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to create MCP server",
		})
		return
	}

	// Convert to response DTO
	response := server.ToResponse()

	c.JSON(http.StatusCreated, response)
}

// List handles listing all MCP servers with optional search
func (h *MCPHandler) List(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userId, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "User ID not found in context",
		})
		return
	}

	userIdStr, ok := userId.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Invalid user ID format",
		})
		return
	}

	// Get optional search query parameter
	searchTerm := c.Query("search")

	// Get only the servers belonging to the authenticated user
	servers, err := h.repo.GetByUserId(c.Request.Context(), userIdStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to retrieve MCP servers",
		})
		return
	}

	// Filter and convert to response DTOs
	responses := make([]models.MCPServerResponse, 0)

	for _, server := range servers {
		// Apply search filter if provided
		if searchTerm != "" {
			searchLower := strings.ToLower(searchTerm)
			nameLower := strings.ToLower(server.Name)
			descLower := strings.ToLower(server.Description)

			if !strings.Contains(nameLower, searchLower) && !strings.Contains(descLower, searchLower) {
				continue
			}
		}

		responses = append(responses, server.ToResponse())
	}

	// Return list response with total count
	listResponse := models.MCPServerListResponse{
		Servers: responses,
		Total:   len(responses),
	}

	c.JSON(http.StatusOK, listResponse)
}

// Get handles retrieving a single MCP server by ID
func (h *MCPHandler) Get(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userId, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "User ID not found in context",
		})
		return
	}

	userIdStr, ok := userId.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Invalid user ID format",
		})
		return
	}

	id := c.Param("id")

	server, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "MCP server not found",
		})
		return
	}

	// Verify ownership
	if server.UserId != userIdStr {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "You don't have permission to access this MCP server",
		})
		return
	}

	// Convert to response DTO
	response := server.ToResponse()

	// Return response
	c.JSON(http.StatusOK, response)
}

// Delete handles deleting an MCP server
func (h *MCPHandler) Delete(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userId, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "User ID not found in context",
		})
		return
	}

	userIdStr, ok := userId.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Invalid user ID format",
		})
		return
	}

	id := c.Param("id")

	// First, get the MCP to verify ownership
	server, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "MCP server not found",
		})
		return
	}

	// Verify ownership
	if server.UserId != userIdStr {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "You don't have permission to delete this MCP server",
		})
		return
	}

	// Delete the MCP server
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to delete MCP server",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "MCP server deleted successfully",
	})
}
