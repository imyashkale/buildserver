package models

import "time"

// GitHub OAuth Endpoints DTOs

// InitiateGitHubOAuthResponse is the response for initiating GitHub OAuth flow
type InitiateGitHubOAuthResponse struct {
	AuthorizationUrl string `json:"authorization_url"`
	State            string `json:"state"`
}

// GitHubCallbackRequest is the request body for GitHub OAuth callback
type GitHubCallbackRequest struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state" binding:"required"`
}

// GitHubCallbackResponse is the response for GitHub OAuth callback
type GitHubCallbackResponse struct {
	Success bool        `json:"success"`
	User    *GitHubUser `json:"user"`
}

// GitHubStatusResponse is the response for GitHub connection status
type GitHubStatusResponse struct {
	Connected   bool        `json:"connected"`
	User        *GitHubUser `json:"user,omitempty"`
	ConnectedAt *time.Time  `json:"connected_at,omitempty"`
}

// DisconnectGitHubResponse is the response for disconnecting GitHub account
type DisconnectGitHubResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// GitHubRepositoryListResponse is a list of GitHub repositories
type GitHubRepositoryListResponse []GitHubRepository

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
