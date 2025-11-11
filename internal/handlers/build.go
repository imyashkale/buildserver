package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gin-gonic/gin"
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

	// Get user ID from context (set by auth middleware)
	userId, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "User ID not found in context",
		})
		return
	}

	log.Printf("InitiateBuild called by user: %v", userId)
	
	// Validate user ID
	userIdStr, ok := userId.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Invalid user ID format",
		})
		return
	}

	log.Printf("User ID validated: %s", userIdStr)

	// Get server ID and deployment ID from URL parameters
	serverId := c.Param("server_id")
	deploymentId := c.Param("deployment_id")

	if serverId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "bad_request",
			"message": "Server ID is required",
		})
		return
	}

	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "bad_request",
			"message": "Deployment ID is required",
		})
		return
	}

	fmt.Println("Got the serverId and deploymentId", deploymentId, serverId)

	ctx := c.Request.Context()

	// Validate GitHub connection
	githubConn, err := h.githubRepo.GetConnectionByUserId(ctx, userIdStr)
	if err != nil || githubConn == nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "github_not_connected",
			"message": "No active GitHub connection found. Please connect your GitHub account first.",
		})
		return
	}

	fmt.Println("Connected to github ", githubConn.ConnectedAt)

	// Validate MCP server
	mcp, err := h.mcpRepo.Get(ctx, serverId)
	if err != nil || mcp == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "mcp_not_found",
			"message": "MCP server not found",
		})
		return
	}

	fmt.Println("Getting the mcp", mcp.Id, mcp.Name)

	if mcp.UserId != userIdStr {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "You don't have permission to build this MCP server",
		})
		return
	}

	// Validate deployment
	deployment, err := h.deploymentRepo.Get(ctx, serverId, deploymentId)
	if err != nil || deployment == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "deployment_not_found",
			"message": "Deployment not found",
		})
		return
	}

	fmt.Println("Getting the deployment", deployment.DeploymentId, deployment.ServerId)

	if deployment.UserId != userIdStr {
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

	// Execute build synchronously for testing purposes
	err = h.pipelineService.ExecuteBuild(ctx, job)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "build_error",
			"message": "Failed to execute build: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Build initiated successfully",
	})
}

// GetBuildDetails handles retrieving build details
func (h *BuildHandler) GetBuildDetails(c *gin.Context) {
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

	// Get server ID and deployment ID from URL parameters
	serverId := c.Param("server_id")
	deploymentId := c.Param("deployment_id")

	if serverId == "" || deploymentId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "bad_request",
			"message": "Server ID and Deployment ID are required",
		})
		return
	}

	ctx := c.Request.Context()

	// Get MCP server
	mcp, err := h.mcpRepo.Get(ctx, serverId)
	if err != nil || mcp == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "mcp_not_found",
			"message": "MCP server not found",
		})
		return
	}

	// Verify MCP ownership
	if mcp.UserId != userIdStr {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "You don't have permission to view this MCP server",
		})
		return
	}

	// Get deployment details using server ID and deployment ID as commit hash
	deployment, err := h.deploymentRepo.Get(ctx, serverId, deploymentId)
	if err != nil || deployment == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "deployment_not_found",
			"message": "Deployment not found",
		})
		return
	}

	// Verify deployment ownership
	if deployment.UserId != userIdStr {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "You don't have permission to view this deployment",
		})
		return
	}

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
		fmt.Printf("Docker build output for image %s:\n%s\n", imageName, string(output))
	}

	if err != nil {
		fmt.Printf("Docker build failed for image %s with error: %v\n", imageName, err)
		return fmt.Errorf("docker build failed: %w", err)
	}

	fmt.Printf("Docker build completed successfully for image %s\n", imageName)
	return nil
}
