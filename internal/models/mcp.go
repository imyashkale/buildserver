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
	CreatedAt            time.Time
	UpdatedAt            time.Time
}
