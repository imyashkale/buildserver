package repository

import (
	"context"

	"github.com/imyashkale/buildserver/internal/database"
	"github.com/imyashkale/buildserver/internal/models"
)

// DeploymentRepository defines the interface for deployment operations
type DeploymentRepository interface {
	Get(ctx context.Context, serverId, deploymentId string) (*models.Deployment, error)
	GetAll(ctx context.Context) ([]*models.Deployment, error)
	GetByUserId(ctx context.Context, userId string) ([]*models.Deployment, error)
	GetByUserIdAndServerId(ctx context.Context, userId, serverId string) ([]*models.Deployment, error)
	UpdateStatus(ctx context.Context, serverId, deploymentId, status string) error
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

// GetAll retrieves all deployments
func (r *dynamoDeploymentRepository) GetAll(ctx context.Context) ([]*models.Deployment, error) {
	return r.db.GetAllDeployments(ctx)
}

// GetByUserId retrieves all deployments for a specific user
func (r *dynamoDeploymentRepository) GetByUserId(ctx context.Context, userId string) ([]*models.Deployment, error) {
	return r.db.GetDeploymentsByUserId(ctx, userId)
}

// GetByUserIdAndServerId retrieves all deployments for a specific user and server
func (r *dynamoDeploymentRepository) GetByUserIdAndServerId(ctx context.Context, userId, serverId string) ([]*models.Deployment, error) {
	return r.db.GetDeploymentsByUserIdAndServerId(ctx, userId, serverId)
}

// UpdateStatus updates the status of a deployment
func (r *dynamoDeploymentRepository) UpdateStatus(ctx context.Context, serverId, deploymentId, status string) error {
	return r.db.UpdateDeploymentStatus(ctx, serverId, deploymentId, status)
}

// Update updates a deployment record with all fields
func (r *dynamoDeploymentRepository) Update(ctx context.Context, deployment *models.Deployment) error {
	return r.db.UpdateDeployment(ctx, deployment)
}
