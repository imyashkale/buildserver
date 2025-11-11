package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/imyashkale/buildserver/internal/models"
)

var (
	// ErrNotFound is returned when a record is not found
	ErrNotFound = errors.New("record not found")
	// ErrAlreadyExists is returned when a record already exists
	ErrAlreadyExists = errors.New("record already exists")
)

// MCPOperations handles all DynamoDB operations for MCP servers
type MCPServer struct {
	client    *Client
	tableName string
}

// NewMCPOperations creates a new MCPOperations instance
func NewMCPServer(client *Client, tableName string) *MCPServer {
	return &MCPServer{
		client:    client,
		tableName: tableName,
	}
}

// CreateMCP creates a new MCP server in DynamoDB
func (ms *MCPServer) CreateMCP(ctx context.Context, server *models.MCPServer) error {

	// Marshal the MCP server into a DynamoDB attribute value map
	av, err := attributevalue.MarshalMap(map[string]interface{}{
		"id":          server.Id,
		"user_id":     server.UserId,
		"name":        server.Name,
		"description": server.Description,
		"repository":  server.Repository,
		"status":      server.Status,
		"envs":        server.EnvironmentVariables,
		"created_at":  server.CreatedAt.Unix(),
		"updated_at":  server.UpdatedAt.Unix(),
	})

	if err != nil {
		return fmt.Errorf("failed to marshal MCP server: %w", err)
	}

	log.Println("Creating MCP server with ID:", server.Id, server.Name)

	// Check if item already exists
	_, err = ms.client.DynamoDB.PutItem(ctx, &dynamodb.PutItemInput{

		TableName: aws.String(ms.tableName),
		Item:      av,
	})

	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	return nil
}

// GetMCP retrieves an MCP server by ID from DynamoDB
func (ms *MCPServer) GetMCP(ctx context.Context, id string) (*models.MCPServer, error) {

	// Get the item from DynamoDB
	result, err := ms.client.DynamoDB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(ms.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get MCP server: %w", err)
	}

	if result.Item == nil {
		return nil, ErrNotFound
	}

	// Unmarshal the item into MCPServer domain model
	server, err := ms.unmarshalMCPServer(result.Item)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal MCP server: %w", err)
	}

	return server, nil
}

// GetAllMCPs retrieves all MCP servers from DynamoDB
func (ms *MCPServer) GetAllMCPs(ctx context.Context) ([]*models.MCPServer, error) {
	result, err := ms.client.DynamoDB.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(ms.client.TableName),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan MCP servers: %w", err)
	}

	servers := make([]*models.MCPServer, 0, len(result.Items))
	for _, item := range result.Items {
		server, err := ms.unmarshalMCPServer(item)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal MCP server: %w", err)
		}
		servers = append(servers, server)
	}

	return servers, nil
}

// GetMCPsByUserId retrieves all MCP servers for a specific user from DynamoDB
func (ms *MCPServer) GetMCPsByUserId(ctx context.Context, userId string) ([]*models.MCPServer, error) {
	// Use Scan with FilterExpression to filter by user_id
	result, err := ms.client.DynamoDB.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(ms.tableName),
		FilterExpression: aws.String("user_id = :userId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":userId": &types.AttributeValueMemberS{Value: userId},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan MCP servers by user_id: %w", err)
	}

	servers := make([]*models.MCPServer, 0, len(result.Items))
	for _, item := range result.Items {
		server, err := ms.unmarshalMCPServer(item)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal MCP server: %w", err)
		}
		servers = append(servers, server)
	}

	return servers, nil
}

// DeleteMCP deletes an MCP server from DynamoDB
func (ops *MCPServer) DeleteMCP(ctx context.Context, id string) error {
	_, err := ops.client.DynamoDB.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(ops.client.TableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
		ConditionExpression: aws.String("attribute_exists(id)"),
	})

	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to delete MCP server: %w", err)
	}

	return nil
}

// MCPExists checks if an MCP server exists in DynamoDB
func (ops *MCPServer) MCPExists(ctx context.Context, id string) (bool, error) {
	result, err := ops.client.DynamoDB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(ops.client.TableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
		ProjectionExpression: aws.String("id"),
	})

	if err != nil {
		return false, fmt.Errorf("failed to check MCP existence: %w", err)
	}

	return result.Item != nil, nil
}

// unmarshalMCPServer is a helper function to unmarshal DynamoDB item to MCPServer domain model
func (ops *MCPServer) unmarshalMCPServer(item map[string]types.AttributeValue) (*models.MCPServer, error) {
	// Unmarshal into a temporary map to handle custom conversions
	var temp struct {
		Id                   string                       `dynamodbav:"id"`
		UserId               string                       `dynamodbav:"user_id"`
		Name                 string                       `dynamodbav:"name"`
		Description          string                       `dynamodbav:"description"`
		Repository           string                       `dynamodbav:"repository"`
		Status               string                       `dynamodbav:"status"`
		EnvironmentVariables []models.EnvironmentVariable `dynamodbav:"envs"`
		CreatedAt            int64                        `dynamodbav:"created_at"`
		UpdatedAt            int64                        `dynamodbav:"updated_at"`
	}

	err := attributevalue.UnmarshalMap(item, &temp)
	if err != nil {
		return nil, err
	}

	// Convert to domain model with proper time.Time conversion
	server := &models.MCPServer{
		Id:                   temp.Id,
		UserId:               temp.UserId,
		Name:                 temp.Name,
		Description:          temp.Description,
		Repository:           temp.Repository,
		Status:               temp.Status,
		EnvironmentVariables: temp.EnvironmentVariables,
		CreatedAt:            time.Unix(temp.CreatedAt, 0),
		UpdatedAt:            time.Unix(temp.UpdatedAt, 0),
	}

	return server, nil
}
