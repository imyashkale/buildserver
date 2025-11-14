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
	Get(ctx context.Context, id string) (*models.MCPServer, error)
	Update(ctx context.Context, server *models.MCPServer) error
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

// Get retrieves an MCP server by ID
func (r *dynamoMCPRepository) Get(ctx context.Context, id string) (*models.MCPServer, error) {
	return r.db.GetMCP(ctx, id)
}

// Update updates an existing MCP server
func (r *dynamoMCPRepository) Update(ctx context.Context, server *models.MCPServer) error {
	return r.db.UpdateMCP(ctx, server)
}
