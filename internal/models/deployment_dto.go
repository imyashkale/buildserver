package models

import "time"

// CreateDeploymentRequest represents the request body for creating a new deployment
type CreateDeploymentRequest struct {
	ServerId   string `json:"server_id" binding:"required"`
	Branch     string `json:"branch" binding:"required"`
	CommitHash string `json:"commit_hash" binding:"required"`
}

// ToDomain converts CreateDeploymentRequest DTO to domain Deployment model
func (req *CreateDeploymentRequest) ToDomain() *Deployment {
	now := time.Now()
	return &Deployment{
		ServerId:   req.ServerId,
		Branch:     req.Branch,
		CommitHash: req.CommitHash,
		Status:     "queued", // Default status
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// DeploymentResponse represents the response structure for a single deployment
type DeploymentResponse struct {
	ServerId   string                          `json:"server_id"`
	UserId     string                          `json:"user_id,omitempty"`
	Branch     string                          `json:"branch"`
	CommitHash string                          `json:"commit_hash"`
	Status     string                          `json:"status"`
	Stages     map[string]*BuildStageStatus    `json:"stages,omitempty"`
	BuildLogs  []BuildLogEntry                 `json:"build_logs,omitempty"`
	ImageURI   string                          `json:"image_uri,omitempty"`
	CreatedAt  time.Time                       `json:"created_at"`
	UpdatedAt  time.Time                       `json:"updated_at"`
}

// DeploymentListResponse represents the response structure for listing deployments
type DeploymentListResponse struct {
	Deployments []DeploymentResponse `json:"deployments"`
	Total       int                  `json:"total"`
}

// ToResponse converts a domain Deployment to a DeploymentResponse DTO
func (d *Deployment) ToResponse() DeploymentResponse {
	return DeploymentResponse{
		ServerId:   d.ServerId,
		UserId:     d.UserId,
		Branch:     d.Branch,
		CommitHash: d.CommitHash,
		Status:     d.Status,
		Stages:     d.Stages,
		BuildLogs:  d.BuildLogs,
		ImageURI:   d.ImageURI,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}
