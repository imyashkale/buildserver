package models

import "time"

// CreateMCPServerRequest represents the request body for creating a new MCP server
type CreateMCPServerRequest struct {
	Id                   string                `json:"id"` // ID will be set server-side
	Name                 string                `json:"name" binding:"required"`
	Description          string                `json:"description"`
	Repository           string                `json:"repository" binding:"required"`
	EnvironmentVariables []EnvironmentVariable `json:"envs"`
}

// ToDomain converts CreateMCPServerRequest DTO to domain MCPServer model
func (req *CreateMCPServerRequest) ToDomain() *MCPServer {
	now := time.Now()
	return &MCPServer{
		Id:                   req.Id, // Will be set by handler if empty
		Name:                 req.Name,
		Description:          req.Description,
		Repository:           req.Repository,
		Status:               "pending", // Default status
		EnvironmentVariables: req.EnvironmentVariables,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

// MCPServerResponse represents the response structure for a single MCP server
type MCPServerResponse struct {
	Id                   string                `json:"id"`
	UserId               string                `json:"user_id"`
	Name                 string                `json:"name"`
	Description          string                `json:"description"`
	Repository           string                `json:"repository"`
	Status               string                `json:"status"`
	EnvironmentVariables []EnvironmentVariable `json:"envs"`
	CreatedAt            time.Time             `json:"created_at"`
	UpdatedAt            time.Time             `json:"updated_at"`
}

// MCPServerListResponse represents the response structure for listing MCP servers
type MCPServerListResponse struct {
	Servers []MCPServerResponse `json:"servers"`
	Total   int                 `json:"total"`
}

// ToResponse converts a domain MCPServer to an MCPServerResponse DTO
func (m *MCPServer) ToResponse() MCPServerResponse {
	return MCPServerResponse{
		Id:                   m.Id,
		UserId:               m.UserId,
		Name:                 m.Name,
		Description:          m.Description,
		Repository:           m.Repository,
		Status:               m.Status,
		EnvironmentVariables: m.EnvironmentVariables,
		CreatedAt:            m.CreatedAt,
		UpdatedAt:            m.UpdatedAt,
	}
}

type MCPOverviewResponse struct {
	Server MCPServerResponse `json:"server"`
}
