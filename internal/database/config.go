package database

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	appConfig "github.com/imyashkale/buildserver/internal/config"
)

// Config holds the DynamoDB configuration
type Config struct {
	TableName string
	Region    string
}

// Client wraps the DynamoDB client
type Client struct {
	DynamoDB  *dynamodb.Client
	TableName string
}

// NewConfig creates a new database configuration from the application config
func NewConfig(appCfg *appConfig.Config) *Config {
	return &Config{
		TableName: appCfg.DynamoDBTableName,
		Region:    appCfg.AWSRegion,
	}
}

// NewClient creates a new DynamoDB client
func NewClient(ctx context.Context, cfg *Config) (*Client, error) {
	// Load AWS SDK config
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS SDK config: %w", err)
	}

	// Create DynamoDB client
	dynamoClient := dynamodb.NewFromConfig(awsCfg)

	// Verify table exists or create it
	if err := ensureTableExists(ctx, dynamoClient, cfg.TableName); err != nil {
		log.Printf("Warning: Could not verify table existence: %v", err)
	}

	return &Client{
		DynamoDB:  dynamoClient,
		TableName: cfg.TableName,
	}, nil
}

// ensureTableExists checks if the DynamoDB table exists
func ensureTableExists(ctx context.Context, client *dynamodb.Client, tableName string) error {
	_, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})

	if err != nil {
		return fmt.Errorf("table %s does not exist or cannot be accessed: %w", tableName, err)
	}

	log.Printf("DynamoDB table '%s' verified successfully", tableName)
	return nil
}
