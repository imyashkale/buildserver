package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/imyashkale/buildserver/internal/logger"
	"github.com/imyashkale/buildserver/internal/models"
)

var (
	ErrGitHubConnectionNotFound      = errors.New("github connection not found")
	ErrGitHubConnectionAlreadyExists = errors.New("github connection already exists")
	ErrOAuthStateNotFound            = errors.New("oauth state not found")
	ErrOAuthStateExpired             = errors.New("oauth state expired")
)

// GitHubDB handles GitHub connection database operations
type GitHubDB struct {
	client               *Client
	connectionsTableName string
	oauthStatesTableName string
}

// NewGitHubDB creates a new GitHubDB instance
func NewGitHubDB(client *Client, connectionsTableName, oauthStatesTableName string) *GitHubDB {
	return &GitHubDB{
		client:               client,
		connectionsTableName: connectionsTableName,
		oauthStatesTableName: oauthStatesTableName,
	}
}

// GetGitHubConnectionByUserId retrieves a GitHub connection by Auth0 user ID
func (db *GitHubDB) GetGitHubConnectionByUserId(ctx context.Context, userId string) (*models.GitHubConnection, error) {
	logger.WithField("user_id", userId).Debug("Retrieving GitHub connection from DynamoDB")

	// For simplicity, we'll use Scan with filter
	result, err := db.client.DynamoDB.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(db.connectionsTableName),
		FilterExpression: aws.String("UserId = :userId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":userId": &types.AttributeValueMemberS{Value: userId},
		},
	})

	// Handle errors
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userId,
			"error":   err.Error(),
		}).Error("Failed to query GitHub connection from DynamoDB")
		return nil, fmt.Errorf("failed to query github connection: %w", err)
	}

	if len(result.Items) == 0 {
		logger.WithField("user_id", userId).Warn("GitHub connection not found in DynamoDB")
		return nil, ErrGitHubConnectionNotFound
	}

	var conn models.GitHubConnection
	err = attributevalue.UnmarshalMap(result.Items[0], &conn)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userId,
			"error":   err.Error(),
		}).Error("Failed to unmarshal GitHub connection")
		return nil, fmt.Errorf("failed to unmarshal github connection: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"user_id":         userId,
		"github_username": conn.GitHubUsername,
	}).Debug("GitHub connection retrieved successfully from DynamoDB")

	return &conn, nil
}

// GetGitHubConnectionById retrieves a GitHub connection by ID
func (db *GitHubDB) GetGitHubConnectionById(ctx context.Context, id string) (*models.GitHubConnection, error) {

	// Get item from DynamoDB
	result, err := db.client.DynamoDB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(db.connectionsTableName),
		Key: map[string]types.AttributeValue{
			"Id": &types.AttributeValueMemberS{Value: id},
		},
	})

	// Handle errors
	if err != nil {
		return nil, fmt.Errorf("failed to get github connection: %w", err)
	}

	if result.Item == nil {
		return nil, ErrGitHubConnectionNotFound
	}

	var conn models.GitHubConnection
	err = attributevalue.UnmarshalMap(result.Item, &conn)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal github connection: %w", err)
	}

	return &conn, nil
}

// GitHubConnectionExists checks if a GitHub connection exists for a user
func (db *GitHubDB) GitHubConnectionExists(ctx context.Context, userId string) (bool, error) {
	result, err := db.client.DynamoDB.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(db.connectionsTableName),
		FilterExpression: aws.String("UserId = :userId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":userId": &types.AttributeValueMemberS{Value: userId},
		},
		Select: types.SelectCount,
	})
	if err != nil {
		return false, fmt.Errorf("failed to check github connection existence: %w", err)
	}

	return result.Count > 0, nil
}

// GetOAuthState retrieves an OAuth state by state token
func (db *GitHubDB) GetOAuthState(ctx context.Context, stateToken string) (*models.OAuthState, error) {
	result, err := db.client.DynamoDB.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(db.oauthStatesTableName),
		FilterExpression: aws.String("StateToken = :stateToken"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":stateToken": &types.AttributeValueMemberS{Value: stateToken},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query oauth state: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, ErrOAuthStateNotFound
	}

	var state models.OAuthState
	err = attributevalue.UnmarshalMap(result.Items[0], &state)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal oauth state: %w", err)
	}

	return &state, nil
}
