package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

// ECRService handles ECR repository and image operations
type ECRService struct {
	ecrClient *ecr.Client
	region    string
	accountID string
}

// NewECRService creates a new ECR service
func NewECRService(cfg aws.Config) *ECRService {
	ecrClient := ecr.NewFromConfig(cfg)
	accountID := os.Getenv("AWS_ACCOUNT_ID")

	return &ECRService{
		ecrClient: ecrClient,
		region:    cfg.Region,
		accountID: accountID,
	}
}

// GetOrCreateRepository gets or creates an ECR repository for the server
// Repository name format: mcp-{server_id}
func (es *ECRService) GetOrCreateRepository(ctx context.Context, serverID string) (string, error) {
	repoName := fmt.Sprintf("mcp-%s", serverID)

	// Try to get existing repository
	describeOutput, err := es.ecrClient.DescribeRepositories(ctx, &ecr.DescribeRepositoriesInput{
		RepositoryNames: []string{repoName},
	})

	if err == nil && len(describeOutput.Repositories) > 0 {
		return repoName, nil
	}

	// Create new repository if it doesn't exist
	createOutput, err := es.ecrClient.CreateRepository(ctx, &ecr.CreateRepositoryInput{
		RepositoryName: aws.String(repoName),
		Tags: []types.Tag{
			{
				Key:   aws.String("managed-by"),
				Value: aws.String("buildserver"),
			},
		},
	})

	if err != nil {
		return "", fmt.Errorf("failed to create ECR repository: %w", err)
	}

	log.Printf("Created ECR repository: %s\n", *createOutput.Repository.RepositoryUri)
	return repoName, nil
}

// GetRepositoryURI returns the full ECR repository URI
func (es *ECRService) GetRepositoryURI(repoName string) string {
	return fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s", es.accountID, es.region, repoName)
}

// PushImage pushes a Docker image to ECR
// Parameters:
//   - ctx: context
//   - repoName: ECR repository name
//   - imageName: local Docker image name (e.g., "server-id:commit-hash")
//   - tags: list of tags to apply (e.g., ["latest", "branch-commit"])
func (es *ECRService) PushImage(ctx context.Context, repoName, imageName string, tags []string) (string, error) {
	repoURI := es.GetRepositoryURI(repoName)

	// Login to ECR (requires AWS CLI installed)
	if err := es.loginToECR(ctx); err != nil {
		return "", fmt.Errorf("ECR login failed: %w", err)
	}

	// Tag the image for ECR
	for _, tag := range tags {
		fullImageName := fmt.Sprintf("%s:%s", repoURI, tag)
		cmd := exec.CommandContext(ctx, "docker", "tag", imageName, fullImageName)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to tag Docker image: %w", err)
		}
	}

	// Push the image
	for _, tag := range tags {
		fullImageName := fmt.Sprintf("%s:%s", repoURI, tag)
		cmd := exec.CommandContext(ctx, "docker", "push", fullImageName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to push Docker image %s: %w", fullImageName, err)
		}
	}

	// Return the primary image URI (first tag or "latest")
	if len(tags) > 0 {
		return fmt.Sprintf("%s:%s", repoURI, tags[0]), nil
	}

	return fmt.Sprintf("%s:latest", repoURI), nil
}

// loginToECR authenticates with ECR
func (es *ECRService) loginToECR(ctx context.Context) error {
	// Get authorization token
	authOutput, err := es.ecrClient.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return fmt.Errorf("failed to get ECR authorization token: %w", err)
	}

	if len(authOutput.AuthorizationData) == 0 {
		return fmt.Errorf("no authorization data returned")
	}

	authData := authOutput.AuthorizationData[0]

	// Extract username and password from authorization token
	// Token format is: username:password in base64
	decodedToken := *authData.AuthorizationToken
	parts := strings.Split(decodedToken, ":")
	if len(parts) < 2 {
		return fmt.Errorf("invalid authorization token format")
	}

	username := parts[0]
	password := strings.Join(parts[1:], ":")
	endpoint := *authData.ProxyEndpoint

	// Docker login
	cmd := exec.CommandContext(ctx, "docker", "login",
		"-u", username,
		"-p", password,
		endpoint,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker login to ECR failed: %w", err)
	}

	return nil
}

// DeleteImage deletes a specific image from ECR repository
func (es *ECRService) DeleteImage(ctx context.Context, repoName, tag string) error {
	_, err := es.ecrClient.BatchDeleteImage(ctx, &ecr.BatchDeleteImageInput{
		RepositoryName: aws.String(repoName),
		ImageIds: []types.ImageIdentifier{
			{
				ImageTag: aws.String(tag),
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to delete image from ECR: %w", err)
	}

	return nil
}
