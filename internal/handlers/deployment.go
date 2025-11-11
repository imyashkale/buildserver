package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/imyashkale/buildserver/internal/models"
	"github.com/imyashkale/buildserver/internal/repository"
)

// DeploymentHandler handles deployment-related requests
type DeploymentHandler struct {
	repo repository.DeploymentRepository
}

// NewDeploymentHandler creates a new deployment handler
func NewDeploymentHandler(repo repository.DeploymentRepository) *DeploymentHandler {
	return &DeploymentHandler{
		repo: repo,
	}
}

// Create handles creating a new deployment
func (h *DeploymentHandler) Create(c *gin.Context) {
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

	var req models.CreateDeploymentRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
		})
		return
	}

	// Convert request DTO to domain model
	deployment := req.ToDomain()

	// Set the user ID
	deployment.UserId = userIdStr

	// Store in repository
	if err := h.repo.Create(c.Request.Context(), deployment); err != nil {
		fmt.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to create deployment",
		})
		return
	}

	// Convert to response DTO
	response := deployment.ToResponse()

	c.JSON(http.StatusCreated, response)
}

// List handles listing all deployments for the authenticated user and specific server
func (h *DeploymentHandler) List(c *gin.Context) {
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

	// Get server ID from URL parameter
	serverId := c.Param("server_id")
	if serverId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "bad_request",
			"message": "Server ID is required",
		})
		return
	}

	// Get deployments for the specific user and server
	deployments, err := h.repo.GetByUserIdAndServerId(c.Request.Context(), userIdStr, serverId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to retrieve deployments",
		})
		return
	}

	// Convert to response DTOs
	responses := make([]models.DeploymentResponse, 0, len(deployments))
	for _, deployment := range deployments {
		responses = append(responses, deployment.ToResponse())
	}

	// Return list response with total count
	listResponse := models.DeploymentListResponse{
		Deployments: responses,
		Total:       len(responses),
	}

	c.JSON(http.StatusOK, listResponse)
}

// Get handles retrieving a single deployment by server ID
func (h *DeploymentHandler) Get(c *gin.Context) {
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

	serverId := c.Param("server_id")
	commitHash := c.Param("commit_hash")

	if commitHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "bad_request",
			"message": "Commit hash is required",
		})
		return
	}

	deployment, err := h.repo.Get(c.Request.Context(), serverId, commitHash)
	if err != nil {
		if err == repository.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"message": "Deployment not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to retrieve deployment",
		})
		return
	}

	// Verify ownership
	if deployment.UserId != userIdStr {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "You don't have permission to access this deployment",
		})
		return
	}

	// Convert to response DTO
	response := deployment.ToResponse()

	c.JSON(http.StatusOK, response)
}
