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
	"github.com/imyashkale/buildserver/internal/logger"
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
		"Id":                   server.Id,
		"UserId":               server.UserId,
		"Name":                 server.Name,
		"Description":          server.Description,
		"Repository":           server.Repository,
		"Status":               server.Status,
		"Envs":                 server.EnvironmentVariables,
		"ECRRepositoryName":    server.ECRRepositoryName,
		"ECRRepositoryURI":     server.ECRRepositoryURI,
		"CreatedAt":            server.CreatedAt.Unix(),
		"UpdatedAt":            server.UpdatedAt.Unix(),
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
	logger.WithField("server_id", id).Debug("Retrieving MCP server from DynamoDB")

	// Get the item from DynamoDB
	result, err := ms.client.DynamoDB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(ms.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
	})

	if err != nil {
		logger.WithFields(map[string]interface{}{
			"server_id": id,
			"error":     err.Error(),
		}).Error("Failed to get MCP server from DynamoDB")
		return nil, fmt.Errorf("failed to get MCP server: %w", err)
	}

	if result.Item == nil {
		logger.WithField("server_id", id).Warn("MCP server not found in DynamoDB")
		return nil, ErrNotFound
	}

	// Unmarshal the item into MCPServer domain model
	server, err := ms.unmarshalMCPServer(result.Item)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"server_id": id,
			"error":     err.Error(),
		}).Error("Failed to unmarshal MCP server")
		return nil, fmt.Errorf("failed to unmarshal MCP server: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"server_id": id,
		"name":      server.Name,
	}).Debug("MCP server retrieved successfully from DynamoDB")

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
	// Use Scan with FilterExpression to filter by UserId
	result, err := ms.client.DynamoDB.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(ms.tableName),
		FilterExpression: aws.String("UserId = :userId"),
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

// UpdateMCP updates an existing MCP server in DynamoDB
func (ms *MCPServer) UpdateMCP(ctx context.Context, server *models.MCPServer) error {
	logger.WithFields(map[string]interface{}{
		"server_id": server.Id,
		"name":      server.Name,
	}).Debug("Updating MCP server in DynamoDB")

	// Update the MCP server using UpdateItem
	_, err := ms.client.DynamoDB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(ms.tableName),
		Key: map[string]types.AttributeValue{
			"Id": &types.AttributeValueMemberS{Value: server.Id},
		},
		UpdateExpression: aws.String("SET #name = :name, #desc = :desc, #repo = :repo, #status = :status, #envs = :envs, #ecrRepoName = :ecrRepoName, #ecrRepoURI = :ecrRepoURI, UpdatedAt = :updated_at"),
		ExpressionAttributeNames: map[string]string{
			"#name":        "Name",
			"#desc":        "Description",
			"#repo":        "Repository",
			"#status":      "Status",
			"#envs":        "Envs",
			"#ecrRepoName": "ECRRepositoryName",
			"#ecrRepoURI":  "ECRRepositoryURI",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":name":        &types.AttributeValueMemberS{Value: server.Name},
			":desc":        &types.AttributeValueMemberS{Value: server.Description},
			":repo":        &types.AttributeValueMemberS{Value: server.Repository},
			":status":      &types.AttributeValueMemberS{Value: server.Status},
			":updated_at":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", server.UpdatedAt.Unix())},
			":ecrRepoName": &types.AttributeValueMemberS{Value: server.ECRRepositoryName},
			":ecrRepoURI":  &types.AttributeValueMemberS{Value: server.ECRRepositoryURI},
		},
	})

	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			logger.WithField("server_id", server.Id).Warn("MCP server not found during update")
			return ErrNotFound
		}
		logger.WithFields(map[string]interface{}{
			"server_id": server.Id,
			"error":     err.Error(),
		}).Error("Failed to update MCP server in DynamoDB")
		return fmt.Errorf("failed to update MCP server: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"server_id": server.Id,
		"name":      server.Name,
	}).Info("MCP server updated successfully in DynamoDB")

	return nil
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
		Id                   string                       `dynamodbav:"Id"`
		UserId               string                       `dynamodbav:"UserId"`
		Name                 string                       `dynamodbav:"Name"`
		Description          string                       `dynamodbav:"Description"`
		Repository           string                       `dynamodbav:"Repository"`
		Status               string                       `dynamodbav:"Status"`
		EnvironmentVariables []models.EnvironmentVariable `dynamodbav:"Envs"`
		ECRRepositoryName    string                       `dynamodbav:"ECRRepositoryName"`
		ECRRepositoryURI     string                       `dynamodbav:"ECRRepositoryURI"`
		CreatedAt            int64                        `dynamodbav:"CreatedAt"`
		UpdatedAt            int64                        `dynamodbav:"UpdatedAt"`
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
		ECRRepositoryName:    temp.ECRRepositoryName,
		ECRRepositoryURI:     temp.ECRRepositoryURI,
		CreatedAt:            time.Unix(temp.CreatedAt, 0),
		UpdatedAt:            time.Unix(temp.UpdatedAt, 0),
	}

	return server, nil
}
