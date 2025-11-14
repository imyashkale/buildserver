package handlers

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/imyashkale/buildserver/internal/logger"
	"github.com/imyashkale/buildserver/internal/queue"
	"github.com/imyashkale/buildserver/internal/repository"
	"github.com/imyashkale/buildserver/internal/services"
	"gopkg.in/yaml.v2"
)

// BuildHandler handles build-related requests
type BuildHandler struct {
	mcpRepo         repository.MCPRepository
	deploymentRepo  repository.DeploymentRepository
	githubRepo      repository.GitHubRepository
	githubService   *services.GitHubService
	pipelineService *services.PipelineService
	jobQueue        *queue.JobQueue
}

// NewBuildHandler creates a new build handler
func NewBuildHandler(
	mcpRepo repository.MCPRepository,
	deploymentRepo repository.DeploymentRepository,
	githubRepo repository.GitHubRepository,
	githubService *services.GitHubService,
	pipelineService *services.PipelineService,
	jobQueue *queue.JobQueue,
) *BuildHandler {
	return &BuildHandler{
		mcpRepo:         mcpRepo,
		deploymentRepo:  deploymentRepo,
		githubRepo:      githubRepo,
		githubService:   githubService,
		pipelineService: pipelineService,
		jobQueue:        jobQueue,
	}
}

// InitiateBuild handles initiating a new build asynchronously
func (h *BuildHandler) InitiateBuild(c *gin.Context) {
	logger.Debug("InitiateBuild handler invoked")

	// Get user ID from context (set by auth middleware)
	userId, exists := c.Get("user_id")
	if !exists {
		logger.Warn("Build initiation failed: user_id not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "User ID not found in context",
		})
		return
	}

	// Validate user ID
	userIdStr, ok := userId.(string)
	if !ok {
		logger.Error("Build initiation failed: invalid user_id format in context")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Invalid user ID format",
		})
		return
	}

	// Get server ID and deployment ID from URL parameters
	serverId := c.Param("server_id")
	deploymentId := c.Param("deployment_id")

	if serverId == "" {
		logger.WithField("user_id", userIdStr).Warn("Build initiation failed: server_id is required")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "bad_request",
			"message": "Server ID is required",
		})
		return
	}

	if deploymentId == "" {
		logger.WithFields(map[string]interface{}{
			"user_id":   userIdStr,
			"server_id": serverId,
		}).Warn("Build initiation failed: deployment_id is required")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "bad_request",
			"message": "Deployment ID is required",
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"server_id":     serverId,
		"deployment_id": deploymentId,
		"user_id":       userIdStr,
	}).Debug("Build initiation request received")

	ctx := c.Request.Context()

	// Validate GitHub connection
	githubConn, err := h.githubRepo.GetConnectionByUserId(ctx, userIdStr)
	if err != nil || githubConn == nil {
		logger.WithFields(map[string]interface{}{
			"user_id":       userIdStr,
			"server_id":     serverId,
			"deployment_id": deploymentId,
			"error":         err,
		}).Warn("Build initiation failed: GitHub connection not found")
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "github_not_connected",
			"message": "No active GitHub connection found. Please connect your GitHub account first.",
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"user_id":      userIdStr,
		"connected_at": githubConn.ConnectedAt,
	}).Debug("GitHub connection validated")

	// Validate MCP server
	mcp, err := h.mcpRepo.Get(ctx, serverId)
	if err != nil || mcp == nil {
		logger.WithFields(map[string]interface{}{
			"user_id":       userIdStr,
			"server_id":     serverId,
			"deployment_id": deploymentId,
			"error":         err,
		}).Warn("Build initiation failed: MCP server not found")
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "mcp_not_found",
			"message": "MCP server not found",
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"user_id":   userIdStr,
		"server_id": serverId,
		"mcp_name":  mcp.Name,
	}).Debug("MCP server retrieved")

	if mcp.UserId != userIdStr {
		logger.WithFields(map[string]interface{}{
			"user_id":       userIdStr,
			"server_id":     serverId,
			"deployment_id": deploymentId,
			"mcp_owner_id":  mcp.UserId,
		}).Warn("Build initiation failed: permission denied for MCP server")
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "You don't have permission to build this MCP server",
		})
		return
	}

	// Validate deployment
	deployment, err := h.deploymentRepo.Get(ctx, serverId, deploymentId)
	if err != nil || deployment == nil {
		logger.WithFields(map[string]interface{}{
			"user_id":       userIdStr,
			"server_id":     serverId,
			"deployment_id": deploymentId,
			"error":         err,
		}).Warn("Build initiation failed: deployment not found")
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "deployment_not_found",
			"message": "Deployment not found",
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"user_id":       userIdStr,
		"deployment_id": deployment.DeploymentId,
		"server_id":     deployment.ServerId,
		"branch":        deployment.Branch,
		"commit_hash":   deployment.CommitHash,
	}).Debug("Deployment record validated")

	if deployment.UserId != userIdStr {
		logger.WithFields(map[string]interface{}{
			"user_id":              userIdStr,
			"server_id":            serverId,
			"deployment_id":        deploymentId,
			"deployment_owner_id":  deployment.UserId,
		}).Warn("Build initiation failed: permission denied for deployment")
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "You don't have permission to build this deployment",
		})
		return
	}

	// Enqueue build job (async execution)
	job := &queue.BuildJob{
		DeploymentID: deploymentId,
		ServerID:     serverId,
		UserID:       userIdStr,
		Branch:       deployment.Branch,
		CommitHash:   deployment.CommitHash,
	}

	// Enqueue the job for asynchronous processing
	h.jobQueue.Enqueue(job)

	logger.WithFields(map[string]interface{}{
		"user_id":       userIdStr,
		"server_id":     serverId,
		"deployment_id": deploymentId,
	}).Info("Build initiated successfully")

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Build initiated successfully",
	})
}

// GetBuildDetails handles retrieving build details
func (h *BuildHandler) GetBuildDetails(c *gin.Context) {
	logger.Debug("GetBuildDetails handler invoked")

	// Get user ID from context (set by auth middleware)
	userId, exists := c.Get("user_id")
	if !exists {
		logger.Warn("GetBuildDetails failed: user_id not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "User ID not found in context",
		})
		return
	}

	userIdStr, ok := userId.(string)
	if !ok {
		logger.Error("GetBuildDetails failed: invalid user_id format in context")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Invalid user ID format",
		})
		return
	}

	// Get server ID and deployment ID from URL parameters
	serverId := c.Param("server_id")
	deploymentId := c.Param("deployment_id")

	if serverId == "" || deploymentId == "" {
		logger.WithField("user_id", userIdStr).Warn("GetBuildDetails failed: server_id and deployment_id are required")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "bad_request",
			"message": "Server ID and Deployment ID are required",
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"user_id":       userIdStr,
		"server_id":     serverId,
		"deployment_id": deploymentId,
	}).Debug("GetBuildDetails request received")

	ctx := c.Request.Context()

	// Get MCP server
	mcp, err := h.mcpRepo.Get(ctx, serverId)
	if err != nil || mcp == nil {
		logger.WithFields(map[string]interface{}{
			"user_id":       userIdStr,
			"server_id":     serverId,
			"deployment_id": deploymentId,
			"error":         err,
		}).Warn("GetBuildDetails failed: MCP server not found")
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "mcp_not_found",
			"message": "MCP server not found",
		})
		return
	}

	// Verify MCP ownership
	if mcp.UserId != userIdStr {
		logger.WithFields(map[string]interface{}{
			"user_id":       userIdStr,
			"server_id":     serverId,
			"deployment_id": deploymentId,
			"mcp_owner_id":  mcp.UserId,
		}).Warn("GetBuildDetails failed: permission denied for MCP server")
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "You don't have permission to view this MCP server",
		})
		return
	}

	// Get deployment details using server ID and deployment ID as commit hash
	deployment, err := h.deploymentRepo.Get(ctx, serverId, deploymentId)
	if err != nil || deployment == nil {
		logger.WithFields(map[string]interface{}{
			"user_id":       userIdStr,
			"server_id":     serverId,
			"deployment_id": deploymentId,
			"error":         err,
		}).Warn("GetBuildDetails failed: deployment not found")
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "deployment_not_found",
			"message": "Deployment not found",
		})
		return
	}

	// Verify deployment ownership
	if deployment.UserId != userIdStr {
		logger.WithFields(map[string]interface{}{
			"user_id":             userIdStr,
			"server_id":           serverId,
			"deployment_id":       deploymentId,
			"deployment_owner_id": deployment.UserId,
		}).Warn("GetBuildDetails failed: permission denied for deployment")
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "You don't have permission to view this deployment",
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"user_id":       userIdStr,
		"server_id":     serverId,
		"deployment_id": deploymentId,
		"status":        deployment.Status,
	}).Info("Build details retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"mcp":        mcp,
		"deployment": deployment.ToResponse(),
	})
}

// injectGitHubToken adds GitHub authentication to repository URL
func (h *BuildHandler) injectGitHubToken(repoURL, token string) string {

	// Handle HTTPS URLs
	if len(repoURL) > 8 && repoURL[:8] == "https://" {
		return fmt.Sprintf("https://x-access-token:%s@%s", token, repoURL[8:])
	}

	// Handle GitHub SSH URLs
	if len(repoURL) > 10 && repoURL[:10] == "git@github" {
		// SSH URLs don't need token injection, return as-is
		return repoURL
	}

	return repoURL
}

// validateConfig checks if mhive.config.yaml is valid YAML
func (h *BuildHandler) validateConfig(configPath string) error {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("mhive.config.yaml not found")
	}

	// Read the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read mhive.config.yaml: %w", err)
	}

	// Validate YAML syntax
	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("invalid YAML syntax: %w", err)
	}

	// Config is valid if we got here
	return nil
}

// buildDockerImage builds a Docker image from the cloned repository
func (h *BuildHandler) buildDockerImage(repoDir, imageName string) error {
	logger.Infof("Starting Docker image build for image: %s", imageName)

	// Check if Dockerfile exists
	dockerfilePath := filepath.Join(repoDir, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		return fmt.Errorf("Dockerfile not found in repository")
	}

	// Build the Docker image with output logging
	cmd := exec.Command("docker", "build", "-t", imageName, repoDir)

	// Capture stdout and stderr
	output, err := cmd.CombinedOutput()
	if output != nil {
		logger.WithField("image_name", imageName).Infof("Docker build output: %s", string(output))
	}

	if err != nil {
		logger.WithFields(map[string]interface{}{
			"image_name": imageName,
			"error":      err.Error(),
		}).Error("Docker build failed")
		return fmt.Errorf("docker build failed: %w", err)
	}

	logger.WithField("image_name", imageName).Info("Docker build completed successfully")
	return nil
}
