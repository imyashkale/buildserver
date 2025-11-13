package models

// EnvironmentVariable represents an environment variable for an MCP server
type EnvironmentVariable struct {
	Name     string `json:"name" dynamodbav:"Name" binding:"required"`
	Value    string `json:"value" dynamodbav:"Value" binding:"required"`
	IsSecret bool   `json:"is_secret" dynamodbav:"IsSecret"`
}

// Tag represents a categorization tag for an MCP server
type Tag struct {
	Name  string `json:"name" binding:"required"`
	Color string `json:"color,omitempty"`
}
