package models

import "time"

// GitHubConnection represents the domain model for a user's GitHub connection
type GitHubConnection struct {
	Id             string                 `json:"id"`
	UserId         string                 `json:"user_id"`          // Auth0 user ID
	GitHubUserId   int64                  `json:"github_user_id"`   // GitHub user ID
	GitHubUsername string                 `json:"github_username"`  // GitHub username
	AccessToken    string                 `json:"access_token"`     // Encrypted GitHub access token
	GitHubUserData map[string]interface{} `json:"github_user_data"` // Full GitHub user profile
	ConnectedAt    time.Time              `json:"connected_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// OAuthState represents a temporary OAuth state token for CSRF protection
type OAuthState struct {
	Id         string    `json:"id"`
	UserId     string    `json:"user_id"`
	StateToken string    `json:"state_token"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// GitHubUser represents the GitHub user profile data
type GitHubUser struct {
	Login       string `json:"login"`
	Id          int64  `json:"id"`
	AvatarUrl   string `json:"avatar_url"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Bio         string `json:"bio"`
	PublicRepos int    `json:"public_repos"`
}

// GitHubRepository represents a GitHub repository
type GitHubRepository struct {
	Id          int64       `json:"id"`
	Name        string      `json:"name"`
	FullName    string      `json:"full_name"`
	Description string      `json:"description"`
	HtmlUrl     string      `json:"html_url"`
	CloneUrl    string      `json:"clone_url"`
	Private     bool        `json:"private"`
	Owner       GitHubOwner `json:"owner"`
	CreatedAt   string      `json:"created_at"`
	UpdatedAt   string      `json:"updated_at"`
}

// GitHubOwner represents the owner of a GitHub repository
type GitHubOwner struct {
	Login     string `json:"login"`
	AvatarUrl string `json:"avatar_url"`
}

// GitHubBranch represents a GitHub repository branch
type GitHubBranch struct {
	Name      string       `json:"name"`
	Commit    GitHubCommit `json:"commit"`
	Protected bool         `json:"protected"`
}

// GitHubCommit represents a GitHub commit reference
type GitHubCommit struct {
	Sha string `json:"sha"`
	Url string `json:"url"`
}

// ToGitHubUser converts GitHubConnection's user data to GitHubUser
func (gc *GitHubConnection) ToGitHubUser() *GitHubUser {
	if gc.GitHubUserData == nil {
		return nil
	}

	user := &GitHubUser{
		Id:    gc.GitHubUserId,
		Login: gc.GitHubUsername,
	}

	if avatarUrl, ok := gc.GitHubUserData["avatar_url"].(string); ok {
		user.AvatarUrl = avatarUrl
	}
	if name, ok := gc.GitHubUserData["name"].(string); ok {
		user.Name = name
	}
	if email, ok := gc.GitHubUserData["email"].(string); ok {
		user.Email = email
	}
	if bio, ok := gc.GitHubUserData["bio"].(string); ok {
		user.Bio = bio
	}
	if publicRepos, ok := gc.GitHubUserData["public_repos"].(float64); ok {
		user.PublicRepos = int(publicRepos)
	}

	return user
}
