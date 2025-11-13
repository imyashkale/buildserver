# DynamoDB Schema

## McpServers
- `Id` (String) - Unique identifier for MCP server
- `UserId` (String) - Auth0 user ID who owns the server
- `Name` (String) - Display name of the MCP server
- `Description` (String) - Detailed description of the MCP server's purpose
- `Repository` (String) - GitHub repository URL for the server code
- `Status` (String) - Current status (e.g., "active", "inactive", "deploying", "failed")
- `Envs` (List) - Environment variables required by the server
  - `Name` (String) - Environment variable name (e.g., "API_KEY", "DATABASE_URL")
  - `Value` (String) - Environment variable value (encrypted if marked as secret)
  - `IsSecret` (Boolean) - Whether value is sensitive and should be encrypted
- `CreatedAt` (Number) - Unix timestamp of when server was created
- `UpdatedAt` (Number) - Unix timestamp of last modification

## Deployments
- `ServerId` (String) - ID of the MCP server being deployed (Primary Key)
- `DeploymentId` (String) - Unique identifier for this specific deployment
- `UserId` (String) - Auth0 user ID who triggered the deployment
- `Branch` (String) - Git branch being deployed (e.g., "main", "develop")
- `CommitHash` (String) - Git commit hash being deployed
- `Status` (String) - Deployment status ("queued", "in_progress", "completed", "failed")
- `Stages` (Map) - Progress of individual build stages
  - `Status` (String) - Stage status ("pending", "in_progress", "completed", "failed")
  - `StartedAt` (Number) - Unix timestamp of when stage started
  - `CompletedAt` (Number) - Unix timestamp of when stage completed
  - `Error` (String) - Error message if stage failed
- `Logs` (List) - Structured build and deployment log entries
  - `Timestamp` (Number) - Unix timestamp of log entry
  - `Stage` (String) - Build stage name (e.g., "build", "test", "push", "deploy")
  - `Level` (String) - Log level ("info", "warning", "error")
  - `Message` (String) - Log message content
- `ImageURI` (String) - ECR Docker image URI
- `CreatedAt` (Number) - Unix timestamp of deployment creation
- `UpdatedAt` (Number) - Unix timestamp of last status update

## GitHubConnections
- `Id` (String) - Unique identifier for the connection record
- `UserId` (String) - Auth0 user ID linked to this GitHub account
- `GitHubUserId` (Number) - GitHub user ID (numeric identifier from GitHub API)
- `GitHubUsername` (String) - GitHub login username
- `AccessToken` (String) - Encrypted GitHub OAuth access token for API calls
- `GitHubUserData` (Map) - Full GitHub user profile data (JSON)
- `ConnectedAt` (Number) - Unix timestamp of when GitHub account was connected
- `UpdatedAt` (Number) - Unix timestamp of last refresh/update

## GitHubOAuthStates
- `Id` (String) - Unique identifier for the state record
- `StateToken` (String) - Random OAuth state token for CSRF protection
- `UserId` (String) - Auth0 user ID initiating the OAuth flow
- `CreatedAt` (Number) - Unix timestamp of when state token was generated
- `ExpiresAt` (Number) - Unix timestamp of when state token expires (typically 10 minutes)

