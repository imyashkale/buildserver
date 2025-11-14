package models

import "time"

// MCPServer represents the domain model for an MCP server
// This is a database-agnostic business entity
type MCPServer struct {
	ServerId             string                `dynamodbav:"ServerId"` // DynamoDB partition key
	UserId               string                `dynamodbav:"UserId"`   // Auth0 user ID
	Name                 string                `dynamodbav:"Name"`
	Description          string                `dynamodbav:"Description"`
	Repository           string                `dynamodbav:"Repository"`
	Status               string                `dynamodbav:"Status"` // e.g., "active", "inactive", "deploying", "failed"
	EnvironmentVariables []EnvironmentVariable `dynamodbav:"Envs"`
	ECRRepositoryName    string                `dynamodbav:"ECRRepositoryName"`
	ECRRepositoryURI     string                `dynamodbav:"ECRRepositoryURI"`
	CreatedAt            time.Time             `dynamodbav:"CreatedAt"`
	UpdatedAt            time.Time             `dynamodbav:"UpdatedAt"`
}
