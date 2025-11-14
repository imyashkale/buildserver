package models

import "time"

// BuildStageStatus represents the status of a single build stage
type BuildStageStatus struct {
	Status      string     `json:"status" dynamodbav:"Status"` // "pending", "in_progress", "completed", "failed"
	StartedAt   *time.Time `json:"started_at,omitempty" dynamodbav:"StartedAt"`
	CompletedAt *time.Time `json:"completed_at,omitempty" dynamodbav:"CompletedAt"`
	Error       string     `json:"error,omitempty" dynamodbav:"Error"`
}

// BuildLogEntry represents a single log entry from the build process
type BuildLogEntry struct {
	Timestamp time.Time `json:"timestamp" dynamodbav:"Timestamp"`
	Stage     string    `json:"stage" dynamodbav:"Stage"`
	Level     string    `json:"level" dynamodbav:"Level"` // "info", "warning", "error"
	Message   string    `json:"message" dynamodbav:"Message"`
}

// Deployment represents the domain model for a deployment
// This is a database-agnostic business entity
type Deployment struct {
	ServerId     string                       `dynamodbav:"ServerId"`
	DeploymentId string                       `dynamodbav:"DeploymentId"`
	UserId       string                       `dynamodbav:"UserId"` // Auth0 user ID
	Branch       string                       `dynamodbav:"Branch"`
	CommitHash   string                       `dynamodbav:"CommitHash"`
	Status       string                       `dynamodbav:"Status"` // e.g., "queued", "in_progress", "completed", "failed"
	Stages       map[string]*BuildStageStatus `dynamodbav:"Stages"`
	BuildLogs    []BuildLogEntry              `dynamodbav:"Logs"`
	ImageURI     string                       `dynamodbav:"ImageURI"`
	CreatedAt    time.Time                    `dynamodbav:"CreatedAt"`
	UpdatedAt    time.Time                    `dynamodbav:"UpdatedAt"`
}
