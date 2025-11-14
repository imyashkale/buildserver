package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	// Server configuration
	Port string

	// Logging configuration
	LogLevel string

	// AWS configuration
	AWSRegion    string
	AWSAccountID string

	// DynamoDB configuration
	DynamoDBTableName          string
	GitHubConnectionsTableName string
	GitHubOAuthStatesTableName string
	DeploymentsTableName       string

	// GitHub OAuth configuration
	GitHubClientID           string
	GitHubClientSecret       string
	GitHubTokenEncryptionKey string
	GitHubCallbackURL        string

	// Auth0 configuration (optional)
	Auth0Domain   string
	Auth0Audience string
}

// New creates a new Config instance by loading environment variables
// from .env file (if present) and OS environment.
// OS environment variables take precedence over .env file values.
// Panics if required configuration values are missing or invalid.
func New() *Config {
	// Load .env file from project root (silently ignore if not found)
	// We use the directory where the binary is run from as the base
	envPath := filepath.Join(".", ".env")
	_ = godotenv.Load(envPath)

	cfg := &Config{
		// Server configuration
		Port: getEnvOrDefault("PORT", "3001"),

		// Logging configuration
		LogLevel: getEnvOrDefault("LOG_LEVEL", "INFO"),

		// AWS configuration
		AWSRegion:    getEnvOrDefault("AWS_REGION", "us-east-1"),
		AWSAccountID: os.Getenv("AWS_ACCOUNT_ID"),

		// DynamoDB configuration
		DynamoDBTableName:          getEnvOrDefault("DYNAMODB_TABLE_NAME", "McpServers"),
		GitHubConnectionsTableName: getEnvOrDefault("GITHUB_CONNECTIONS_TABLE_NAME", "GitHubConnections"),
		GitHubOAuthStatesTableName: getEnvOrDefault("GITHUB_OAUTH_STATES_TABLE_NAME", "GitHubOAuthStates"),
		DeploymentsTableName:       getEnvOrDefault("DYNAMODB_DEPLOYMENTS_TABLE", "Deployments"),

		// GitHub OAuth configuration
		GitHubClientID:           os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret:       os.Getenv("GITHUB_CLIENT_SECRET"),
		GitHubTokenEncryptionKey: os.Getenv("GITHUB_TOKEN_ENCRYPTION_KEY"),
		GitHubCallbackURL:        getEnvOrDefault("GITHUB_CALLBACK_URL", "http://localhost:3000/api/v1/auth/github/callback"),

		// Auth0 configuration (optional)
		Auth0Domain:   os.Getenv("AUTH0_DOMAIN"),
		Auth0Audience: os.Getenv("AUTH0_AUDIENCE"),
	}

	// Validate required configuration
	cfg.validate()

	return cfg
}

// validate checks that all required configuration values are present and valid
func (c *Config) validate() {
	var missing []string

	// Check required AWS configuration
	if c.AWSAccountID == "" {
		missing = append(missing, "AWS_ACCOUNT_ID")
	}

	// Check required GitHub OAuth configuration
	if c.GitHubClientID == "" {
		missing = append(missing, "GITHUB_CLIENT_ID")
	}
	if c.GitHubClientSecret == "" {
		missing = append(missing, "GITHUB_CLIENT_SECRET")
	}
	if c.GitHubTokenEncryptionKey == "" {
		missing = append(missing, "GITHUB_TOKEN_ENCRYPTION_KEY")
	}

	if len(missing) > 0 {
		panic(fmt.Sprintf("Missing required configuration values: %v", missing))
	}

	// Validate encryption key length (must be 32 characters for AES-256)
	if len(c.GitHubTokenEncryptionKey) != 32 {
		panic(fmt.Sprintf("GITHUB_TOKEN_ENCRYPTION_KEY must be exactly 32 characters (got %d)", len(c.GitHubTokenEncryptionKey)))
	}

	// Validate AWS Account ID format (should be 12 digits)
	if len(c.AWSAccountID) != 12 || !isNumeric(c.AWSAccountID) {
		panic(fmt.Sprintf("AWS_ACCOUNT_ID must be exactly 12 digits (got '%s')", c.AWSAccountID))
	}
}

// isNumeric checks if a string contains only numeric characters
func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Helper methods for accessing configuration values

// GetPort returns the server port
func (c *Config) GetPort() string {
	return c.Port
}

// GetLogLevel returns the logging level
func (c *Config) GetLogLevel() string {
	return c.LogLevel
}

// GetAWSRegion returns the AWS region
func (c *Config) GetAWSRegion() string {
	return c.AWSRegion
}

// GetAWSAccountID returns the AWS account ID
func (c *Config) GetAWSAccountID() string {
	return c.AWSAccountID
}

// GetDynamoDBTableName returns the main DynamoDB table name
func (c *Config) GetDynamoDBTableName() string {
	return c.DynamoDBTableName
}

// GetGitHubConnectionsTableName returns the GitHub connections table name
func (c *Config) GetGitHubConnectionsTableName() string {
	return c.GitHubConnectionsTableName
}

// GetGitHubOAuthStatesTableName returns the GitHub OAuth states table name
func (c *Config) GetGitHubOAuthStatesTableName() string {
	return c.GitHubOAuthStatesTableName
}

// GetGitHubClientID returns the GitHub OAuth client ID
func (c *Config) GetGitHubClientID() string {
	return c.GitHubClientID
}

// GetGitHubClientSecret returns the GitHub OAuth client secret
func (c *Config) GetGitHubClientSecret() string {
	return c.GitHubClientSecret
}

// GetGitHubTokenEncryptionKey returns the GitHub token encryption key
func (c *Config) GetGitHubTokenEncryptionKey() string {
	return c.GitHubTokenEncryptionKey
}

// GetGitHubCallbackURL returns the GitHub OAuth callback URL
func (c *Config) GetGitHubCallbackURL() string {
	return c.GitHubCallbackURL
}

// GetAuth0Domain returns the Auth0 domain (may be empty)
func (c *Config) GetAuth0Domain() string {
	return c.Auth0Domain
}

// GetAuth0Audience returns the Auth0 audience (may be empty)
func (c *Config) GetAuth0Audience() string {
	return c.Auth0Audience
}

// GetDeploymentsTableName returns the deployments table name
func (c *Config) GetDeploymentsTableName() string {
	return c.DeploymentsTableName
}
