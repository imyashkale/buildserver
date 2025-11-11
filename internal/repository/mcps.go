package repository

import (
	"context"

	"github.com/imyashkale/buildserver/internal/database"
	"github.com/imyashkale/buildserver/internal/models"
)

// Re-export errors from database package for backward compatibility
var (
	ErrNotFound      = database.ErrNotFound
	ErrAlreadyExists = database.ErrAlreadyExists
)

// MCPRepository defines the interface for MCP server operations
type MCPRepository interface {
	Create(ctx context.Context, server *models.MCPServer) error
	Get(ctx context.Context, id string) (*models.MCPServer, error)
	GetAll(ctx context.Context) ([]*models.MCPServer, error)
	GetByID(ctx context.Context, id string) (*models.MCPServer, error)
	GetByUserId(ctx context.Context, userId string) ([]*models.MCPServer, error)
	Delete(ctx context.Context, id string) error
}

// dynamoMCPRepository implements MCPRepository using DynamoDB
type dynamoMCPRepository struct {
	db *database.MCPServer
}

// NewMCPRepository creates a new DynamoDB-backed MCP repository
func NewMCPRepository(db *database.MCPServer) MCPRepository {
	return &dynamoMCPRepository{
		db: db,
	}
}

// Create creates a new MCP server
func (r *dynamoMCPRepository) Create(ctx context.Context, server *models.MCPServer) error {
	return r.db.CreateMCP(ctx, server)
}

// Get retrieves an MCP server by ID
func (r *dynamoMCPRepository) Get(ctx context.Context, id string) (*models.MCPServer, error) {
	return r.db.GetMCP(ctx, id)
}

// GetAll retrieves all MCP servers
func (r *dynamoMCPRepository) GetAll(ctx context.Context) ([]*models.MCPServer, error) {
	return r.db.GetAllMCPs(ctx)
}

// GetByID retrieves an MCP server by ID (same as Get for compatibility)
func (r *dynamoMCPRepository) GetByID(ctx context.Context, id string) (*models.MCPServer, error) {
	return r.db.GetMCP(ctx, id)
}

// GetByUserId retrieves all MCP servers for a specific user
func (r *dynamoMCPRepository) GetByUserId(ctx context.Context, userId string) ([]*models.MCPServer, error) {
	return r.db.GetMCPsByUserId(ctx, userId)
}

// Delete deletes an MCP server by ID
func (r *dynamoMCPRepository) Delete(ctx context.Context, id string) error {
	return r.db.DeleteMCP(ctx, id)
}
