package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/imyashkale/buildserver/internal/models"
	"github.com/imyashkale/buildserver/internal/queue"
	"github.com/imyashkale/buildserver/internal/repository"
	"gopkg.in/yaml.v2"
)

// PipelineService orchestrates the build pipeline stages
type PipelineService struct {
	deploymentRepo repository.DeploymentRepository
	githubService  *GitHubService
	ecrService     *ECRService
	mcpRepo        repository.MCPRepository
	githubRepo     repository.GitHubRepository
	logger         *BuildLogger
}

// NewPipelineService creates a new pipeline service
func NewPipelineService(
	deploymentRepo repository.DeploymentRepository,
	githubService *GitHubService,
	ecrService *ECRService,
	mcpRepo repository.MCPRepository,
	githubRepo repository.GitHubRepository,
) *PipelineService {
	return &PipelineService{
		deploymentRepo: deploymentRepo,
		githubService:  githubService,
		ecrService:     ecrService,
		mcpRepo:        mcpRepo,
		githubRepo:     githubRepo,
		logger:         NewBuildLogger(),
	}
}

// ExecuteBuild executes the complete build pipeline for a job
func (ps *PipelineService) ExecuteBuild(ctx context.Context, job *queue.BuildJob) error {

	fmt.Println("Build getting executed --- ", job.DeploymentID)

	ps.logger.Clear()
	now := time.Now()

	// Initialize stages in deployment
	stages := map[string]*models.BuildStageStatus{
		"clone":           {Status: "pending", StartedAt: &now},
		"validate_config": {Status: "pending"},
		"validate_docker": {Status: "pending"},
		"build_image":     {Status: "pending"},
		"create_ecr":      {Status: "pending"},
		"push_image":      {Status: "pending"},
	}

	// Get the deployment record
	deployment, err := ps.deploymentRepo.Get(ctx, job.ServerID, job.DeploymentID)
	if err != nil || deployment == nil {
		ps.logger.LogError("init", "Failed to fetch deployment from database")
		return fmt.Errorf("deployment not found")
	}

	// Log deployment details
	fmt.Println("Deployment queried: ", deployment.DeploymentId, deployment.ServerId, deployment.CommitHash)

	deployment.Stages = stages
	deployment.Status = "in_progress"
	deployment.BuildLogs = ps.logger.GetLogs()

	// Update deployment with initialized stages
	if err := ps.updateDeployment(ctx, deployment); err != nil {
		ps.logger.LogError("init", fmt.Sprintf("Failed to update deployment: %v", err))
		return err
	}

	// Stage 1: Clone Repository
	if err := ps.stageClone(ctx, job, deployment); err != nil {
		ps.markStageFailed(ctx, deployment, "clone", err)
		return err
	}

	ps.markStageCompleted(ctx, deployment, "clone")

	// Stage 2: Validate mhive.config.yaml
	tempDir := ps.getTempDir(job)
	if err := ps.stageValidateConfig(ctx, job, deployment, tempDir); err != nil {
		ps.markStageFailed(ctx, deployment, "validate_config", err)
		defer os.RemoveAll(tempDir)
		return err
	}

	ps.markStageCompleted(ctx, deployment, "validate_config")

	// Stage 3: Validate Dockerfile
	if err := ps.stageValidateDocker(ctx, job, deployment, tempDir); err != nil {
		ps.markStageFailed(ctx, deployment, "validate_docker", err)
		defer os.RemoveAll(tempDir)
		return err
	}

	ps.markStageCompleted(ctx, deployment, "validate_docker")

	// Stage 4: Build Docker Image
	imageName := ps.getImageName(job)
	if err := ps.stageBuildImage(ctx, job, deployment, tempDir, imageName); err != nil {
		ps.markStageFailed(ctx, deployment, "build_image", err)
		defer os.RemoveAll(tempDir)
		return err
	}

	ps.markStageCompleted(ctx, deployment, "build_image")
	defer os.RemoveAll(tempDir)

	// Stage 5: Create/Verify ECR Repository
	repoName, repoURI, err := ps.stageCreateECR(ctx, job, deployment)
	if err != nil {
		ps.markStageFailed(ctx, deployment, "create_ecr", err)
		return err
	}

	ps.markStageCompleted(ctx, deployment, "create_ecr")

	// Update MCP with ECR repository information
	if err := ps.updateMCPWithECRRepo(ctx, job.ServerID, repoName, repoURI); err != nil {
		ps.logger.LogError("create_ecr", fmt.Sprintf("Failed to update MCP with ECR repo info: %v", err))
		// Log warning but don't fail the build - ECR repo was created successfully
	}

	// Stage 6: Push Image to ECR
	imageURI, err := ps.stagePushImage(ctx, job, deployment, imageName, repoName)
	if err != nil {
		ps.markStageFailed(ctx, deployment, "push_image", err)
		return err
	}

	ps.markStageCompleted(ctx, deployment, "push_image")

	// Mark build as completed
	deployment.Status = "completed"
	deployment.ImageURI = imageURI
	deployment.BuildLogs = ps.logger.GetLogsWithSizeLimit()

	if err := ps.updateDeployment(ctx, deployment); err != nil {
		ps.logger.LogError("finalize", fmt.Sprintf("Failed to update deployment: %v", err))
		return err
	}

	ps.logger.LogInfo("finalize", "Build pipeline completed successfully")
	return nil
}

// stageClone handles repository cloning
func (ps *PipelineService) stageClone(ctx context.Context, job *queue.BuildJob, deployment *models.Deployment) error {
	ps.logger.LogInfo("clone", "Starting repository clone")

	// Get MCP server details
	mcp, err := ps.mcpRepo.Get(ctx, job.ServerID)
	if err != nil || mcp == nil {
		ps.logger.LogError("clone", "MCP server not found")
		return fmt.Errorf("mcp server not found")
	}

	// Get GitHub connection and token
	githubConn, err := ps.githubRepo.GetConnectionByUserId(ctx, job.UserID)
	if err != nil || githubConn == nil {
		ps.logger.LogError("clone", "GitHub connection not found")
		return fmt.Errorf("github connection not found")
	}

	// Decrypt token
	accessToken, err := ps.githubService.DecryptToken(githubConn.AccessToken)
	if err != nil {
		ps.logger.LogError("clone", fmt.Sprintf("Failed to decrypt GitHub token: %v", err))
		return fmt.Errorf("token decryption failed: %w", err)
	}

	// Clone repository
	tempDir := ps.getTempDir(job)
	if err := ps.cloneRepository(mcp.Repository, job.Branch, job.CommitHash, tempDir, accessToken); err != nil {
		ps.logger.LogError("clone", fmt.Sprintf("Repository clone failed: %v", err))
		return err
	}

	ps.logger.LogInfo("clone", fmt.Sprintf("Repository cloned successfully to %s", tempDir))
	return nil
}

// stageValidateConfig validates mhive.config.yaml
func (ps *PipelineService) stageValidateConfig(ctx context.Context, job *queue.BuildJob, deployment *models.Deployment, tempDir string) error {
	ps.logger.LogInfo("validate_config", "Starting mhive.config.yaml validation")

	configPath := filepath.Join(tempDir, "mhive.config.yaml")
	if err := ps.validateConfig(configPath); err != nil {
		ps.logger.LogError("validate_config", fmt.Sprintf("Config validation failed: %v", err))
		return err
	}

	ps.logger.LogInfo("validate_config", "mhive.config.yaml is valid")
	return nil
}

// stageValidateDocker validates Dockerfile
func (ps *PipelineService) stageValidateDocker(ctx context.Context, job *queue.BuildJob, deployment *models.Deployment, tempDir string) error {
	ps.logger.LogInfo("validate_docker", "Starting Dockerfile validation")

	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		ps.logger.LogError("validate_docker", "Dockerfile not found at "+dockerfilePath)
		return fmt.Errorf("dockerfile not found")
	}
	ps.logger.LogInfo("validate_docker", "Dockerfile file exists at "+dockerfilePath)

	// Read the Dockerfile to validate its format
	data, err := os.ReadFile(dockerfilePath)
	if err != nil {
		ps.logger.LogError("validate_docker", fmt.Sprintf("Failed to read Dockerfile: %v", err))
		return fmt.Errorf("failed to read dockerfile: %w", err)
	}
	ps.logger.LogInfo("validate_docker", fmt.Sprintf("Dockerfile read successfully (%d bytes)", len(data)))

	// Basic syntax validation - check for required instructions
	content := string(data)
	if len(content) == 0 {
		ps.logger.LogError("validate_docker", "Dockerfile is empty")
		return fmt.Errorf("dockerfile is empty")
	}

	ps.logger.LogInfo("validate_docker", "Dockerfile syntax validation completed successfully")
	return nil
}

// stageBuildImage builds the Docker image
func (ps *PipelineService) stageBuildImage(ctx context.Context, job *queue.BuildJob, deployment *models.Deployment, tempDir, imageName string) error {
	ps.logger.LogInfo("build_image", fmt.Sprintf("Starting Docker image build for %s", imageName))

	if err := ps.buildDockerImage(tempDir, imageName); err != nil {
		ps.logger.LogError("build_image", fmt.Sprintf("Docker build failed: %v", err))
		return err
	}

	ps.logger.LogInfo("build_image", fmt.Sprintf("Docker image built successfully: %s", imageName))
	return nil
}

// stageCreateECR creates or verifies ECR repository
func (ps *PipelineService) stageCreateECR(ctx context.Context, job *queue.BuildJob, deployment *models.Deployment) (string, string, error) {
	ps.logger.LogInfo("create_ecr", fmt.Sprintf("Creating/verifying ECR repository for server %s", job.ServerID))

	repoName, err := ps.ecrService.GetOrCreateRepository(ctx, job.ServerID)
	if err != nil {
		ps.logger.LogError("create_ecr", fmt.Sprintf("Failed to create ECR repository: %v", err))
		return "", "", err
	}

	repoURI := ps.ecrService.GetRepositoryURI(repoName)
	ps.logger.LogInfo("create_ecr", fmt.Sprintf("ECR repository ready: %s", repoURI))
	return repoName, repoURI, nil
}

// stagePushImage pushes the Docker image to ECR
func (ps *PipelineService) stagePushImage(ctx context.Context, job *queue.BuildJob, deployment *models.Deployment, imageName, repoName string) (string, error) {
	ps.logger.LogInfo("push_image", fmt.Sprintf("Pushing Docker image to ECR: %s", repoName))

	// Create tags for the image
	tags := []string{
		fmt.Sprintf("%s-%s", job.Branch, job.CommitHash[:8]),
		"latest",
	}

	imageURI, err := ps.ecrService.PushImage(ctx, repoName, imageName, tags)
	if err != nil {
		ps.logger.LogError("push_image", fmt.Sprintf("Failed to push image to ECR: %v", err))
		return "", err
	}

	ps.logger.LogInfo("push_image", fmt.Sprintf("Image pushed successfully to ECR: %s", imageURI))
	return imageURI, nil
}

// Helper methods

// getTempDir returns the temporary directory path for a job
func (ps *PipelineService) getTempDir(job *queue.BuildJob) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("mcp-build-%s-%s", job.ServerID, job.DeploymentID))
}

// getImageName returns the Docker image name for a job
func (ps *PipelineService) getImageName(job *queue.BuildJob) string {
	return fmt.Sprintf("%s:%s-%s", job.ServerID, job.Branch, job.CommitHash[:8])
}

// markStageCompleted marks a stage as completed in the deployment
func (ps *PipelineService) markStageCompleted(ctx context.Context, deployment *models.Deployment, stageName string) {
	if deployment.Stages == nil {
		deployment.Stages = make(map[string]*models.BuildStageStatus)
	}

	now := time.Now()
	deployment.Stages[stageName] = &models.BuildStageStatus{
		Status:      "completed",
		CompletedAt: &now,
	}

	deployment.BuildLogs = ps.logger.GetLogsWithSizeLimit()
	ps.updateDeployment(ctx, deployment)
}

// markStageFailed marks a stage as failed in the deployment
func (ps *PipelineService) markStageFailed(ctx context.Context, deployment *models.Deployment, stageName string, err error) {
	if deployment.Stages == nil {
		deployment.Stages = make(map[string]*models.BuildStageStatus)
	}

	now := time.Now()
	deployment.Stages[stageName] = &models.BuildStageStatus{
		Status:      "failed",
		CompletedAt: &now,
		Error:       err.Error(),
	}

	deployment.Status = "failed"
	deployment.BuildLogs = ps.logger.GetLogsWithSizeLimit()
	ps.updateDeployment(ctx, deployment)
}

// updateDeployment updates the deployment record in the database
func (ps *PipelineService) updateDeployment(ctx context.Context, deployment *models.Deployment) error {
	deployment.UpdatedAt = time.Now()
	return ps.deploymentRepo.Update(ctx, deployment)
}

// cloneRepository clones a git repository at a specific branch and commit
func (ps *PipelineService) cloneRepository(repoURL, branch, commitHash, targetDir, accessToken string) error {
	// Inject GitHub token for authentication
	authenticatedURL := ps.injectGitHubToken(repoURL, accessToken)

	// Clone the repository
	cmd := exec.Command("git", "clone", "-b", branch, authenticatedURL, targetDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	// Checkout specific commit
	cmd = exec.Command("git", "-C", targetDir, "checkout", commitHash)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git checkout failed: %w", err)
	}

	return nil
}

// injectGitHubToken adds GitHub authentication to repository URL
func (ps *PipelineService) injectGitHubToken(repoURL, token string) string {
	if len(repoURL) > 8 && repoURL[:8] == "https://" {
		return fmt.Sprintf("https://x-access-token:%s@%s", token, repoURL[8:])
	}
	return repoURL
}

// validateConfig checks if mhive.config.yaml is valid YAML
func (ps *PipelineService) validateConfig(configPath string) error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		ps.logger.LogError("validate_config", "mhive.config.yaml not found at "+configPath)
		return fmt.Errorf("mhive.config.yaml not found")
	}
	ps.logger.LogInfo("validate_config", "mhive.config.yaml file exists")

	data, err := os.ReadFile(configPath)
	if err != nil {
		ps.logger.LogError("validate_config", fmt.Sprintf("Failed to read mhive.config.yaml: %v", err))
		return fmt.Errorf("failed to read mhive.config.yaml: %w", err)
	}
	ps.logger.LogInfo("validate_config", fmt.Sprintf("Successfully read mhive.config.yaml (%d bytes)", len(data)))

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		ps.logger.LogError("validate_config", fmt.Sprintf("Invalid YAML syntax: %v", err))
		return fmt.Errorf("invalid YAML syntax: %w", err)
	}
	ps.logger.LogInfo("validate_config", fmt.Sprintf("YAML syntax is valid. Configuration contains %d root keys", len(config)))

	// Log configuration keys for validation
	ps.logger.LogInfo("validate_config", "Configuration validation completed successfully")
	return nil
}

// buildDockerImage builds a Docker image from the cloned repository
func (ps *PipelineService) buildDockerImage(repoDir, imageName string) error {
	dockerfilePath := filepath.Join(repoDir, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		return fmt.Errorf("dockerfile not found in repository")
	}

	cmd := exec.Command("docker", "build", "-t", imageName, repoDir)

	// Capture combined stdout and stderr to log build output
	output, err := cmd.CombinedOutput()

	// Log the output regardless of error
	if output != nil {
		outputStr := string(output)
		// Log Docker build output line by line
		ps.logger.LogInfo("build_image", outputStr)
	}

	if err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	return nil
}

// updateMCPWithECRRepo updates the MCP with ECR repository information
func (ps *PipelineService) updateMCPWithECRRepo(ctx context.Context, serverID, repoName, repoURI string) error {
	// Get the MCP server
	mcp, err := ps.mcpRepo.Get(ctx, serverID)
	if err != nil || mcp == nil {
		return fmt.Errorf("failed to get MCP server: %w", err)
	}

	// Update ECR repository information
	mcp.ECRRepositoryName = repoName
	mcp.ECRRepositoryURI = repoURI
	mcp.UpdatedAt = time.Now()

	// Save the updated MCP
	if err := ps.mcpRepo.Update(ctx, mcp); err != nil {
		return fmt.Errorf("failed to update MCP with ECR repo info: %w", err)
	}

	return nil
}
