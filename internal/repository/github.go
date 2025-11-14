package repository

import (
	"context"

	"github.com/imyashkale/buildserver/internal/database"
	"github.com/imyashkale/buildserver/internal/models"
)

// Re-export database errors for use in handlers
var (
	ErrGitHubConnectionNotFound      = database.ErrGitHubConnectionNotFound
	ErrGitHubConnectionAlreadyExists = database.ErrGitHubConnectionAlreadyExists
	ErrOAuthStateNotFound            = database.ErrOAuthStateNotFound
	ErrOAuthStateExpired             = database.ErrOAuthStateExpired
)

// GitHubRepository defines the interface for GitHub-related operations
type GitHubRepository interface {
	// GitHub Connection operations
	GetConnectionByUserId(ctx context.Context, userId string) (*models.GitHubConnection, error)
	GetOAuthState(ctx context.Context, stateToken string) (*models.OAuthState, error)
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

// GetConnectionByUserId retrieves a GitHub connection by user ID
func (r *gitHubRepository) GetConnectionByUserId(ctx context.Context, userId string) (*models.GitHubConnection, error) {
	return r.db.GetGitHubConnectionByUserId(ctx, userId)
}

// GetOAuthState retrieves an OAuth state by state token
func (r *gitHubRepository) GetOAuthState(ctx context.Context, stateToken string) (*models.OAuthState, error) {
	return r.db.GetOAuthState(ctx, stateToken)
}
