package models

import "time"

// MCPServer represents the domain model for an MCP server
// This is a database-agnostic business entity
type MCPServer struct {
	Id                   string
	UserId               string // Auth0 user ID
	Name                 string
	Description          string
	Repository           string
	Status               string // e.g., "active", "inactive", "deploying", "failed"
	EnvironmentVariables []EnvironmentVariable
	ECRRepositoryName    string // ECR repository name (e.g., "mcp-server-id")
	ECRRepositoryURI     string // Full ECR repository URI (e.g., "123456789.dkr.ecr.us-east-1.amazonaws.com/mcp-server-id")
	CreatedAt            time.Time
	UpdatedAt            time.Time
}
