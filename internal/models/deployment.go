package models

import "time"

// BuildStageStatus represents the status of a single build stage
type BuildStageStatus struct {
	Status    string    `json:"status"` // "pending", "in_progress", "completed", "failed"
	StartedAt *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// BuildLogEntry represents a single log entry from the build process
type BuildLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Stage     string    `json:"stage"`
	Level     string    `json:"level"` // "info", "warning", "error"
	Message   string    `json:"message"`
}

// Deployment represents the domain model for a deployment
// This is a database-agnostic business entity
type Deployment struct {
	ServerId   string
	UserId     string // Auth0 user ID
	Branch     string
	CommitHash string
	Status     string // e.g., "queued", "in_progress", "completed", "failed"
	Stages     map[string]*BuildStageStatus `json:"stages,omitempty"` // Track individual stage progress
	BuildLogs  []BuildLogEntry `json:"build_logs,omitempty"` // Structured logs
	ImageURI   string `json:"image_uri,omitempty"` // ECR image reference
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
