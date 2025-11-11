package repository

import (
	"context"

	"github.com/imyashkale/buildserver/internal/database"
	"github.com/imyashkale/buildserver/internal/models"
)

// GitHubRepository defines the interface for GitHub-related operations
type GitHubRepository interface {
	// GitHub Connection operations
	CreateConnection(ctx context.Context, conn *models.GitHubConnection) error
	GetConnectionByUserId(ctx context.Context, userId string) (*models.GitHubConnection, error)
	GetConnectionById(ctx context.Context, id string) (*models.GitHubConnection, error)
	UpdateConnection(ctx context.Context, conn *models.GitHubConnection) error
	DeleteConnection(ctx context.Context, userId string) error
	ConnectionExists(ctx context.Context, userId string) (bool, error)

	// OAuth State operations
	CreateOAuthState(ctx context.Context, state *models.OAuthState) error
	GetOAuthState(ctx context.Context, stateToken string) (*models.OAuthState, error)
	DeleteOAuthState(ctx context.Context, id string) error
}

// gitHubRepository is the concrete implementation of GitHubRepository
type gitHubRepository struct {
	db *database.GitHubDB
}

// NewGitHubRepository creates a new instance of GitHubRepository
func NewGitHubRepository(db *database.GitHubDB) GitHubRepository {
	return &gitHubRepository{
		db: db,
	}
}

// CreateConnection creates a new GitHub connection
func (r *gitHubRepository) CreateConnection(ctx context.Context, conn *models.GitHubConnection) error {
	return r.db.CreateGitHubConnection(ctx, conn)
}

// GetConnectionByUserId retrieves a GitHub connection by user ID
func (r *gitHubRepository) GetConnectionByUserId(ctx context.Context, userId string) (*models.GitHubConnection, error) {
	return r.db.GetGitHubConnectionByUserId(ctx, userId)
}

// GetConnectionById retrieves a GitHub connection by ID
func (r *gitHubRepository) GetConnectionById(ctx context.Context, id string) (*models.GitHubConnection, error) {
	return r.db.GetGitHubConnectionById(ctx, id)
}

// UpdateConnection updates an existing GitHub connection
func (r *gitHubRepository) UpdateConnection(ctx context.Context, conn *models.GitHubConnection) error {
	return r.db.UpdateGitHubConnection(ctx, conn)
}

// DeleteConnection deletes a GitHub connection
func (r *gitHubRepository) DeleteConnection(ctx context.Context, userId string) error {
	return r.db.DeleteGitHubConnection(ctx, userId)
}

// ConnectionExists checks if a GitHub connection exists for a user
func (r *gitHubRepository) ConnectionExists(ctx context.Context, userId string) (bool, error) {
	return r.db.GitHubConnectionExists(ctx, userId)
}

// CreateOAuthState creates a new OAuth state token
func (r *gitHubRepository) CreateOAuthState(ctx context.Context, state *models.OAuthState) error {
	return r.db.CreateOAuthState(ctx, state)
}

// GetOAuthState retrieves an OAuth state by state token
func (r *gitHubRepository) GetOAuthState(ctx context.Context, stateToken string) (*models.OAuthState, error) {
	return r.db.GetOAuthState(ctx, stateToken)
}

// DeleteOAuthState deletes an OAuth state
func (r *gitHubRepository) DeleteOAuthState(ctx context.Context, id string) error {
	return r.db.DeleteOAuthState(ctx, id)
}

// Re-export database errors for use in handlers
var (
	ErrGitHubConnectionNotFound      = database.ErrGitHubConnectionNotFound
	ErrGitHubConnectionAlreadyExists = database.ErrGitHubConnectionAlreadyExists
	ErrOAuthStateNotFound            = database.ErrOAuthStateNotFound
	ErrOAuthStateExpired             = database.ErrOAuthStateExpired
)
