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

// DeploymentOperations handles all DynamoDB operations for deployments
type DeploymentOperations struct {
	client    *Client
	tableName string
}

// NewDeploymentOperations creates a new DeploymentOperations instance
func NewDeploymentOperations(client *Client, tableName string) *DeploymentOperations {
	return &DeploymentOperations{
		client:    client,
		tableName: tableName,
	}
}

// CreateDeployment creates or updates a deployment in DynamoDB
func (do *DeploymentOperations) CreateDeployment(ctx context.Context, deployment *models.Deployment) error {
	logger.WithFields(map[string]interface{}{
		"server_id":     deployment.ServerId,
		"deployment_id": deployment.DeploymentId,
		"user_id":       deployment.UserId,
	}).Debug("Creating deployment in DynamoDB")

	// Marshal the deployment into a DynamoDB attribute value map
	av, err := attributevalue.MarshalMap(map[string]interface{}{
		"ServerId":     deployment.ServerId,
		"DeploymentId": deployment.DeploymentId,
		"UserId":       deployment.UserId,
		"Branch":       deployment.Branch,
		"CommitHash":   deployment.CommitHash,
		"Status":       deployment.Status,
		"CreatedAt":    deployment.CreatedAt.Unix(),
		"UpdatedAt":    deployment.UpdatedAt.Unix(),
	})

	if err != nil {
		logger.WithFields(map[string]interface{}{
			"server_id":     deployment.ServerId,
			"deployment_id": deployment.DeploymentId,
			"error":         err.Error(),
		}).Error("Failed to marshal deployment")
		return fmt.Errorf("failed to marshal deployment: %w", err)
	}

	log.Println("Creating/updating deployment for server ID:", deployment.ServerId)

	// Put item (will overwrite if exists with same server_id)
	_, err = do.client.DynamoDB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(do.tableName),
		Item:      av,
	})

	if err != nil {
		logger.WithFields(map[string]interface{}{
			"server_id":     deployment.ServerId,
			"deployment_id": deployment.DeploymentId,
			"error":         err.Error(),
		}).Error("Failed to create deployment in DynamoDB")
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"server_id":     deployment.ServerId,
		"deployment_id": deployment.DeploymentId,
	}).Info("Deployment created successfully in DynamoDB")

	return nil
}

// GetDeployment retrieves a deployment by server ID and deployment ID from DynamoDB
func (do *DeploymentOperations) GetDeployment(ctx context.Context, serverId, deploymentId string) (*models.Deployment, error) {
	logger.WithFields(map[string]interface{}{
		"server_id":     serverId,
		"deployment_id": deploymentId,
	}).Debug("Retrieving deployment from DynamoDB")

	// Query by ServerId (partition key) and filter by DeploymentId
	result, err := do.client.DynamoDB.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(do.tableName),
		KeyConditionExpression: aws.String("ServerId = :serverId"),
		FilterExpression:       aws.String("DeploymentId = :deploymentId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":serverId":     &types.AttributeValueMemberS{Value: serverId},
			":deploymentId": &types.AttributeValueMemberS{Value: deploymentId},
		},
	})

	if err != nil {
		logger.WithFields(map[string]interface{}{
			"server_id":     serverId,
			"deployment_id": deploymentId,
			"error":         err.Error(),
		}).Error("Failed to query deployment from DynamoDB")
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	if len(result.Items) == 0 {
		logger.WithFields(map[string]interface{}{
			"server_id":     serverId,
			"deployment_id": deploymentId,
		}).Warn("Deployment not found in DynamoDB")
		return nil, ErrNotFound
	}

	// Unmarshal the first item into Deployment domain model
	deployment, err := do.unmarshalDeployment(result.Items[0])
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"server_id":     serverId,
			"deployment_id": deploymentId,
			"error":         err.Error(),
		}).Error("Failed to unmarshal deployment")
		return nil, fmt.Errorf("failed to unmarshal deployment: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"server_id":     serverId,
		"deployment_id": deploymentId,
		"status":        deployment.Status,
	}).Debug("Deployment retrieved successfully from DynamoDB")

	return deployment, nil
}

// GetAllDeployments retrieves all deployments from DynamoDB
func (do *DeploymentOperations) GetAllDeployments(ctx context.Context) ([]*models.Deployment, error) {
	result, err := do.client.DynamoDB.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(do.tableName),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan deployments: %w", err)
	}

	deployments := make([]*models.Deployment, 0, len(result.Items))
	for _, item := range result.Items {
		deployment, err := do.unmarshalDeployment(item)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal deployment: %w", err)
		}
		deployments = append(deployments, deployment)
	}

	return deployments, nil
}

// GetDeploymentsByUserId retrieves all deployments for a specific user from DynamoDB
func (do *DeploymentOperations) GetDeploymentsByUserId(ctx context.Context, userId string) ([]*models.Deployment, error) {
	// Use Scan with FilterExpression to filter by UserId
	result, err := do.client.DynamoDB.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(do.tableName),
		FilterExpression: aws.String("UserId = :userId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":userId": &types.AttributeValueMemberS{Value: userId},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan deployments by user_id: %w", err)
	}

	deployments := make([]*models.Deployment, 0, len(result.Items))
	for _, item := range result.Items {
		deployment, err := do.unmarshalDeployment(item)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal deployment: %w", err)
		}
		deployments = append(deployments, deployment)
	}

	return deployments, nil
}

// GetDeploymentsByUserIdAndServerId retrieves all deployments for a specific user and server from DynamoDB
func (do *DeploymentOperations) GetDeploymentsByUserIdAndServerId(ctx context.Context, userId, serverId string) ([]*models.Deployment, error) {
	// Use Scan with FilterExpression to filter by both UserId and ServerId
	result, err := do.client.DynamoDB.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(do.tableName),
		FilterExpression: aws.String("UserId = :userId AND ServerId = :serverId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":userId":   &types.AttributeValueMemberS{Value: userId},
			":serverId": &types.AttributeValueMemberS{Value: serverId},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan deployments by user_id and serverId: %w", err)
	}

	deployments := make([]*models.Deployment, 0, len(result.Items))
	for _, item := range result.Items {
		deployment, err := do.unmarshalDeployment(item)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal deployment: %w", err)
		}
		deployments = append(deployments, deployment)
	}

	return deployments, nil
}

// UpdateDeploymentStatus updates the status of a deployment
func (do *DeploymentOperations) UpdateDeploymentStatus(ctx context.Context, serverId, deploymentId, status string) error {
	_, err := do.client.DynamoDB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(do.tableName),
		Key: map[string]types.AttributeValue{
			"ServerId": &types.AttributeValueMemberS{Value: serverId},
		},
		UpdateExpression: aws.String("SET #status = :status, UpdatedAt = :updated_at"),
		ExpressionAttributeNames: map[string]string{
			"#status": "Status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status":       &types.AttributeValueMemberS{Value: status},
			":updated_at":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Unix())},
			":deploymentId": &types.AttributeValueMemberS{Value: deploymentId},
		},
		ConditionExpression: aws.String("DeploymentId = :deploymentId"),
	})

	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	return nil
}

// UpdateDeployment updates a deployment with all fields including stages and logs
func (do *DeploymentOperations) UpdateDeployment(ctx context.Context, deployment *models.Deployment) error {
	logger.WithFields(map[string]interface{}{
		"server_id":     deployment.ServerId,
		"deployment_id": deployment.DeploymentId,
		"status":        deployment.Status,
	}).Debug("Updating deployment in DynamoDB")

	// Prepare the attributes to update
	updateExpr := "SET #status = :status, #stages = :stages, #logs = :logs, #imageUri = :imageUri, UpdatedAt = :updated_at"
	exprAttrNames := map[string]string{
		"#status":   "Status",
		"#stages":   "Stages",
		"#logs":     "Logs",
		"#imageUri": "ImageURI",
	}

	// Convert stages to DynamoDB attribute values
	stagesAv, _ := attributevalue.Marshal(deployment.Stages)
	logsAv, _ := attributevalue.Marshal(deployment.BuildLogs)

	exprAttrVals := map[string]types.AttributeValue{
		":status":       &types.AttributeValueMemberS{Value: deployment.Status},
		":stages":       stagesAv,
		":logs":         logsAv,
		":imageUri":     &types.AttributeValueMemberS{Value: deployment.ImageURI},
		":updated_at":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", deployment.UpdatedAt.Unix())},
		":deploymentId": &types.AttributeValueMemberS{Value: deployment.DeploymentId},
	}

	_, err := do.client.DynamoDB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(do.tableName),
		Key: map[string]types.AttributeValue{
			"ServerId": &types.AttributeValueMemberS{Value: deployment.ServerId},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeNames:  exprAttrNames,
		ExpressionAttributeValues: exprAttrVals,
		ConditionExpression:       aws.String("DeploymentId = :deploymentId"),
	})

	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			logger.WithFields(map[string]interface{}{
				"server_id":     deployment.ServerId,
				"deployment_id": deployment.DeploymentId,
			}).Warn("Deployment not found during update")
			return ErrNotFound
		}
		logger.WithFields(map[string]interface{}{
			"server_id":     deployment.ServerId,
			"deployment_id": deployment.DeploymentId,
			"error":         err.Error(),
		}).Error("Failed to update deployment in DynamoDB")
		return fmt.Errorf("failed to update deployment: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"server_id":     deployment.ServerId,
		"deployment_id": deployment.DeploymentId,
		"status":        deployment.Status,
	}).Info("Deployment updated successfully in DynamoDB")

	return nil
}

// unmarshalDeployment is a helper function to unmarshal DynamoDB item to Deployment domain model
func (do *DeploymentOperations) unmarshalDeployment(item map[string]types.AttributeValue) (*models.Deployment, error) {
	// Unmarshal into a temporary struct to handle custom conversions
	var temp struct {
		ServerId     string                              `dynamodbav:"ServerId"`
		DeploymentId string                              `dynamodbav:"DeploymentId"`
		UserId       string                              `dynamodbav:"UserId"`
		Branch       string                              `dynamodbav:"Branch"`
		CommitHash   string                              `dynamodbav:"CommitHash"`
		Status       string                              `dynamodbav:"Status"`
		Stages       map[string]*models.BuildStageStatus `dynamodbav:"Stages"`
		BuildLogs    []models.BuildLogEntry              `dynamodbav:"Logs"`
		ImageURI     string                              `dynamodbav:"ImageURI"`
		CreatedAt    int64                               `dynamodbav:"CreatedAt"`
		UpdatedAt    int64                               `dynamodbav:"UpdatedAt"`
	}

	err := attributevalue.UnmarshalMap(item, &temp)
	if err != nil {
		return nil, err
	}

	// Convert to domain model with proper time.Time conversion
	deployment := &models.Deployment{
		ServerId:     temp.ServerId,
		DeploymentId: temp.DeploymentId,
		UserId:       temp.UserId,
		Branch:       temp.Branch,
		CommitHash:   temp.CommitHash,
		Status:       temp.Status,
		Stages:       temp.Stages,
		BuildLogs:    temp.BuildLogs,
		ImageURI:     temp.ImageURI,
		CreatedAt:    time.Unix(temp.CreatedAt, 0),
		UpdatedAt:    time.Unix(temp.UpdatedAt, 0),
	}

	return deployment, nil
}
