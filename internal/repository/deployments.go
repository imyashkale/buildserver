package repository

import (
	"context"

	"github.com/imyashkale/buildserver/internal/database"
	"github.com/imyashkale/buildserver/internal/models"
)

// DeploymentRepository defines the interface for deployment operations
type DeploymentRepository interface {
	Get(ctx context.Context, serverId, deploymentId string) (*models.Deployment, error)
	Update(ctx context.Context, deployment *models.Deployment) error
}

// dynamoDeploymentRepository implements DeploymentRepository using DynamoDB
type dynamoDeploymentRepository struct {
	db *database.DeploymentOperations
}

// NewDeploymentRepository creates a new DynamoDB-backed deployment repository
func NewDeploymentRepository(db *database.DeploymentOperations) DeploymentRepository {
	return &dynamoDeploymentRepository{
		db: db,
	}
}

// Get retrieves a deployment by server ID and commit hash
func (r *dynamoDeploymentRepository) Get(ctx context.Context, serverId, deploymentId string) (*models.Deployment, error) {
	return r.db.GetDeployment(ctx, serverId, deploymentId)
}

// Update updates a deployment record with all fields
func (r *dynamoDeploymentRepository) Update(ctx context.Context, deployment *models.Deployment) error {
	return r.db.UpdateDeployment(ctx, deployment)
}
