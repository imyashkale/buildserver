# BuildServer - Complete Technical Documentation

## Table of Contents
1. [Overview](#overview)
2. [System Architecture](#system-architecture)
3. [Configuration Parameters](#configuration-parameters)
4. [Image Building Process](#image-building-process)
5. [API Reference](#api-reference)
6. [Data Models](#data-models)
7. [Request Flow](#request-flow)
8. [Database Schema](#database-schema)

---

## Overview

BuildServer is a production-grade microservice that automates the building and deployment of MCP (Model Context Protocol) servers. It orchestrates a complete pipeline from GitHub repository cloning through Docker image building to AWS ECR registry push.

### Key Features
- **Automated Build Pipeline**: 6-stage build process with comprehensive validation
- **GitHub Integration**: OAuth-based authentication and repository access
- **AWS ECR Integration**: Automatic ECR repository creation and image pushing
- **Comprehensive Logging**: Structured, stage-based logging with size limits
- **Database Persistence**: DynamoDB-backed deployment tracking
- **Concurrent Processing**: Worker pool for handling multiple builds simultaneously
- **JWT Authentication**: Secure API access control

---

## System Architecture

### 1. Layered Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        CLIENT APPLICATIONS                       │
│                   (Web UI, Mobile App, etc.)                     │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    HTTP/REST API LAYER                           │
│  (Gin Framework)                                                 │
│  ┌──────────────────┐  ┌──────────────────┐                     │
│  │ /api/v1/build    │  │  /api/v1/health  │                     │
│  │  :initiate       │  │                  │                     │
│  │  :get-details    │  └──────────────────┘                     │
│  └──────────────────┘                                            │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                   MIDDLEWARE LAYER                               │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ • CORS Middleware                                        │  │
│  │ • JWT Authentication Middleware                          │  │
│  │   - Token validation & expiration check                  │  │
│  │   - User ID extraction from JWT 'sub' claim             │  │
│  └──────────────────────────────────────────────────────────┘  │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                   HANDLER LAYER                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ BuildHandler                                             │  │
│  │ • InitiateBuild() - Validates & enqueues build job      │  │
│  │ • GetBuildDetails() - Retrieves build status & logs     │  │
│  └──────────────────────────────────────────────────────────┘  │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                   SERVICE LAYER                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │ Pipeline     │  │ GitHub       │  │ ECR Service  │           │
│  │ Service      │  │ Service      │  │ Service      │           │
│  │              │  │              │  │              │           │
│  │ • Execute    │  │ • OAuth      │  │ • Create     │           │
│  │   6-stage    │  │   Exchange   │  │   Repo       │           │
│  │   pipeline   │  │ • Token      │  │ • Push Image │           │
│  │ • Track      │  │   Encrypt/   │  │ • Login      │           │
│  │   status     │  │   Decrypt    │  └──────────────┘           │
│  │ • Log output │  │ • API Calls  │                             │
│  └──────────────┘  └──────────────┘                             │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                QUEUE & WORKER POOL                               │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ JobQueue (Channel Buffer: 100)                           │  │
│  │        ▼                                                  │  │
│  │ WorkerPool (5 concurrent workers)                        │  │
│  │        ▼                                                  │  │
│  │ Pipeline.ExecuteBuild()                                  │  │
│  └──────────────────────────────────────────────────────────┘  │
└────────────────────────┬────────────────────────────────────────┘
                         │
                    ┌────┴────┬────────────┬────────────┐
                    ▼         ▼            ▼            ▼
        ┌────────────────┐ ┌──────────┐ ┌────────┐ ┌──────────┐
        │   DynamoDB     │ │   ECR    │ │ Docker │ │ GitHub   │
        │   (Deployment  │ │ Registry │ │ Daemon │ │   API    │
        │    Records)    │ │          │ │        │ │          │
        └────────────────┘ └──────────┘ └────────┘ └──────────┘
         (AWS DynamoDB)    (AWS ECR)  (Local/Remote)  (GitHub)
```

### 2. Component Interaction Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    HTTP Request                              │
│     POST /api/v1/build/{id}/{deploymentId}/initiate         │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────┐
│ Middleware                       │
│ • Validate JWT Token             │
│ • Extract User ID                │
└────────────┬─────────────────────┘
             │
             ▼
┌──────────────────────────────────┐
│ BuildHandler                     │
│ • Validate server/deployment     │
│ • Check ownership                │
└────────────┬─────────────────────┘
             │
             ▼
┌──────────────────────────────────┐
│ Repository Layer                 │
│ • Get MCP Server                 │
│ • Get GitHub Connection          │
│ • Get Deployment Record          │
└────────────┬─────────────────────┘
             │
             ▼
┌──────────────────────────────────┐
│ Job Queue                        │
│ • Enqueue BuildJob               │
└────────────┬─────────────────────┘
             │
             ▼
┌──────────────────────────────────┐
│ Worker Pool                      │
│ • Dequeue Job                    │
│ • Execute Pipeline               │
└────────────┬─────────────────────┘
             │
             ├─────► Stage 1: Clone Repository
             ├─────► Stage 2: Validate Config
             ├─────► Stage 3: Validate Dockerfile
             ├─────► Stage 4: Build Docker Image
             ├─────► Stage 5: Create ECR Repository
             └─────► Stage 6: Push to ECR
                     │
                     ▼
             ┌──────────────────────┐
             │ Update Deployment    │
             │ Status in DynamoDB   │
             └──────────────────────┘
```

---

## Configuration Parameters

### 1. Environment Variables (Required & Optional)

| Parameter | Type | Default | Required | Description |
|-----------|------|---------|----------|-------------|
| `PORT` | int | 3001 | No | HTTP server port |
| `DYNAMODB_TABLE_NAME` | string | mcp-servers | No | MCP servers table in DynamoDB |
| `AWS_REGION` | string | us-east-1 | No | AWS region for DynamoDB & ECR |
| `GITHUB_CONNECTIONS_TABLE_NAME` | string | github-connections | No | GitHub connections table |
| `GITHUB_OAUTH_STATES_TABLE_NAME` | string | github-oauth-states | No | OAuth state tracking table |
| `DYNAMODB_DEPLOYMENTS_TABLE` | string | deployments | No | Deployment records table |
| `GITHUB_CLIENT_ID` | string | - | **Yes** | GitHub OAuth application ID |
| `GITHUB_CLIENT_SECRET` | string | - | **Yes** | GitHub OAuth application secret |
| `GITHUB_TOKEN_ENCRYPTION_KEY` | string | - | **Yes** | 32-character AES-256 encryption key |
| `GITHUB_CALLBACK_URL` | string | http://localhost:3000/api/v1/auth/github/callback | No | OAuth callback URL |

### 2. Build Request Parameters

#### URL Parameters
```
POST /api/v1/build/{server_id}/{deployment_id}/initiate
```
- `server_id` (string): Unique identifier for the MCP server
- `deployment_id` (string): Unique identifier for the deployment

#### Required Context (from Authorization)
- `JWT Token`: User authentication token
  - Must contain `sub` claim with user ID
  - Must not be expired

#### Data Retrieved from Database

**From Deployment Record:**
```json
{
  "branch": "main",
  "commit_hash": "abc123def456...",
  "server_id": "server-1",
  "deployment_id": "deploy-1",
  "user_id": "user-1"
}
```

**From MCP Server Record:**
```json
{
  "id": "server-1",
  "repository": "https://github.com/owner/repo.git",
  "user_id": "user-1"
}
```

**From GitHub Connection:**
```json
{
  "user_id": "user-1",
  "access_token": "<encrypted-github-oauth-token>"
}
```

### 3. Encryption Configuration

**GitHub Token Encryption:**
- Algorithm: AES-256-GCM (Galois/Counter Mode)
- Key Length: 32 bytes (256 bits)
- Key Format: String (exactly 32 characters)
- Encryption Method: Key-based symmetric encryption
- Use Case: Secure storage of GitHub OAuth access tokens in DynamoDB

```
Example Key Generation:
$ head -c 32 /dev/urandom | base64 | head -c 32

Or use: openssl rand -base64 32 | cut -c1-32
```

---

## Image Building Process

### 1. Complete Build Pipeline Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                    BUILD JOB INITIATED                               │
│  Trigger: POST /api/v1/build/{server_id}/{deployment_id}/initiate   │
└─────────────────────┬───────────────────────────────────────────────┘
                      │
                      ▼
        ┌─────────────────────────────────────┐
        │  STAGE 1: CLONE REPOSITORY          │
        │  ─────────────────────────────────  │
        │  Status: in_progress                │
        ├─────────────────────────────────────┤
        │ 1. Fetch MCP Server details         │
        │    - Repository URL                 │
        │    - User ownership validation      │
        │                                     │
        │ 2. Fetch GitHub Connection          │
        │    - Decrypt access token           │
        │    - Token injection into URL       │
        │                                     │
        │ 3. Execute Git Operations           │
        │    $ git clone -b {branch}          │
        │         {url}                       │
        │         /tmp/mcp-build-{id}        │
        │                                     │
        │ 4. Checkout Specific Commit         │
        │    $ git checkout {commit_hash}     │
        │                                     │
        │ 5. Verify Clone Success             │
        │    - Check directory exists         │
        │    - Log completion                 │
        └────────────┬────────────────────────┘
                     │ [Success]
                     ▼
        ┌─────────────────────────────────────┐
        │  STAGE 2: VALIDATE CONFIG           │
        │  ─────────────────────────────────  │
        │  Status: in_progress                │
        ├─────────────────────────────────────┤
        │ 1. Check File Existence             │
        │    mhive.config.yaml must exist     │
        │    Path: /tmp/mcp-build-{id}/       │
        │           mhive.config.yaml         │
        │                                     │
        │ 2. Read and Parse YAML              │
        │    - Load file contents             │
        │    - Parse YAML syntax              │
        │    - Validate structure             │
        │                                     │
        │ 3. Validate Configuration           │
        │    - Check required fields          │
        │    - Verify value types             │
        │    - Validate ranges                │
        │                                     │
        │ 4. Log Configuration Details        │
        └────────────┬────────────────────────┘
                     │ [Success]
                     ▼
        ┌─────────────────────────────────────┐
        │  STAGE 3: VALIDATE DOCKERFILE       │
        │  ─────────────────────────────────  │
        │  Status: in_progress                │
        ├─────────────────────────────────────┤
        │ 1. Check File Existence             │
        │    Dockerfile must exist            │
        │    Path: /tmp/mcp-build-{id}/       │
        │           Dockerfile                │
        │                                     │
        │ 2. Verify File Accessibility       │
        │    - Check read permissions        │
        │    - Verify non-empty               │
        │                                     │
        │ 3. Log Validation Result            │
        └────────────┬────────────────────────┘
                     │ [Success]
                     ▼
        ┌─────────────────────────────────────┐
        │  STAGE 4: BUILD DOCKER IMAGE        │
        │  ─────────────────────────────────  │
        │  Status: in_progress                │
        ├─────────────────────────────────────┤
        │ 1. Prepare Build Parameters         │
        │    Image Name: {server_id}:         │
        │                {branch}-            │
        │                {commit[:8]}         │
        │    Example: server-1:main-a1b2c3d4  │
        │                                     │
        │ 2. Execute Docker Build             │
        │    $ docker build                   │
        │      -t {imageName}                 │
        │      {repoDir}                      │
        │                                     │
        │ 3. Capture Build Output             │
        │    - Stdout logs                    │
        │    - Stderr logs                    │
        │    - Build steps                    │
        │    - Layer information              │
        │                                     │
        │ 4. Verify Build Success             │
        │    - Image exists locally           │
        │    - Log completion                 │
        └────────────┬────────────────────────┘
                     │ [Success]
                     ▼
        ┌─────────────────────────────────────┐
        │  STAGE 5: CREATE ECR REPOSITORY     │
        │  ─────────────────────────────────  │
        │  Status: in_progress                │
        ├─────────────────────────────────────┤
        │ 1. Determine Repository Name        │
        │    Format: mcp-{server_id}          │
        │    Example: mcp-server-1            │
        │                                     │
        │ 2. Check Repository Existence       │
        │    - Query AWS ECR                  │
        │    - Get repository details         │
        │                                     │
        │ 3. Create if Not Exists             │
        │    - CreateRepository() API call    │
        │    - Tag: managed-by=buildserver    │
        │    - Set lifecycle policies         │
        │                                     │
        │ 4. Retrieve Repository URI          │
        │    Format: {accountId}.dkr.ecr.     │
        │             {region}.amazonaws.    │
        │             com/{repo-name}         │
        │    Example: 123456789.dkr.ecr.     │
        │             us-east-1.amazonaws.    │
        │             com/mcp-server-1        │
        │                                     │
        │ 5. Log Repository Information       │
        └────────────┬────────────────────────┘
                     │ [Success]
                     ▼
        ┌─────────────────────────────────────┐
        │  STAGE 6: PUSH IMAGE TO ECR         │
        │  ─────────────────────────────────  │
        │  Status: in_progress                │
        ├─────────────────────────────────────┤
        │ 1. Login to ECR                     │
        │    - Get authorization token       │
        │    - $ docker login to ECR          │
        │                                     │
        │ 2. Tag Image for ECR (Tag 1)        │
        │    $ docker tag                     │
        │      {localImageName}               │
        │      {repoURI}:{branch}-            │
        │      {commit[:8]}                   │
        │                                     │
        │ 3. Tag Image for ECR (Tag 2)        │
        │    $ docker tag                     │
        │      {localImageName}               │
        │      {repoURI}:latest               │
        │                                     │
        │ 4. Push Tag 1 to ECR                │
        │    $ docker push                    │
        │      {repoURI}:{branch}-            │
        │      {commit[:8]}                   │
        │                                     │
        │ 5. Push Tag 2 to ECR                │
        │    $ docker push                    │
        │      {repoURI}:latest               │
        │                                     │
        │ 6. Log Final Image URI              │
        │    Return imageURI for deployment   │
        └────────────┬────────────────────────┘
                     │ [Success]
                     ▼
        ┌─────────────────────────────────────┐
        │  ALL STAGES COMPLETED               │
        │  ─────────────────────────────────  │
        │  Status: completed                  │
        │                                     │
        │  Final Deployment Status:           │
        │  {                                  │
        │    "status": "completed",           │
        │    "image_uri":                     │
        │      "123456789.dkr.ecr.            │
        │       us-east-1.amazonaws.com/      │
        │       mcp-server-1:main-a1b2c3d4"   │
        │  }                                  │
        └─────────────────────────────────────┘
```

### 2. Error Handling Flow

```
┌─────────────────────────────────────┐
│     STAGE EXECUTION ERROR           │
└────────────────┬────────────────────┘
                 │
                 ▼
     ┌───────────────────────────┐
     │  Mark Stage as FAILED     │
     │  • Status: failed         │
     │  • Capture error message  │
     │  • Record timestamp       │
     └────────────┬──────────────┘
                  │
                  ▼
     ┌───────────────────────────┐
     │  Stop Pipeline Execution  │
     │  • Skip remaining stages  │
     │  • Do not proceed         │
     └────────────┬──────────────┘
                  │
                  ▼
     ┌───────────────────────────┐
     │  Cleanup Resources        │
     │  • Delete temp directory  │
     │  • Clean up partial files │
     │  • Release locks          │
     └────────────┬──────────────┘
                  │
                  ▼
     ┌───────────────────────────┐
     │  Update Deployment        │
     │  Status: failed           │
     │  • Record all error logs  │
     │  • Persist to DynamoDB    │
     │  • Return error response  │
     └───────────────────────────┘
```

### 3. Build Log Structure

```
BuildLogEntry {
  timestamp: "2024-11-11T10:30:45Z"
  stage: "clone|validate_config|validate_docker|build_image|create_ecr|push_image"
  level: "info|warning|error"
  message: "Log message content"
}

Examples:
[
  {
    "timestamp": "2024-11-11T10:30:45Z",
    "stage": "clone",
    "level": "info",
    "message": "Starting repository clone..."
  },
  {
    "timestamp": "2024-11-11T10:30:46Z",
    "stage": "clone",
    "level": "info",
    "message": "Cloning into '/tmp/mcp-build-server-1-deploy-1'..."
  },
  {
    "timestamp": "2024-11-11T10:30:50Z",
    "stage": "clone",
    "level": "info",
    "message": "Repository cloned successfully"
  },
  {
    "timestamp": "2024-11-11T10:30:51Z",
    "stage": "validate_config",
    "level": "info",
    "message": "Checking mhive.config.yaml..."
  },
  {
    "timestamp": "2024-11-11T10:31:15Z",
    "stage": "build_image",
    "level": "info",
    "message": "Docker build completed: server-1:main-a1b2c3d4"
  }
]

Log Size Management:
• Max size per deployment: 400 KB
• When exceeded: Keep latest entries, truncate oldest
• Ensures DynamoDB item size stays within limits (400 KB)
```

---

## API Reference

### 1. Initiate Build Endpoint

```http
POST /api/v1/build/:server_id/:deployment_id/initiate
Authorization: Bearer <JWT_TOKEN>
```

**Request Parameters:**
- `server_id` (path): MCP server identifier
- `deployment_id` (path): Deployment identifier
- `Authorization` (header): JWT authentication token

**Success Response:**
```json
HTTP/1.1 202 Accepted
Content-Type: application/json

{
  "message": "Build initiated successfully",
  "server_id": "server-1",
  "deployment_id": "deploy-1",
  "status": "in_progress"
}
```

**Error Responses:**

```json
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "validation_error",
  "message": "Invalid server_id or deployment_id"
}
```

```json
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "error": "unauthorized",
  "message": "Missing or invalid JWT token"
}
```

```json
HTTP/1.1 403 Forbidden
Content-Type: application/json

{
  "error": "forbidden",
  "message": "User does not own this server or deployment"
}
```

```json
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "error": "not_found",
  "message": "Server or deployment not found"
}
```

```json
HTTP/1.1 500 Internal Server Error
Content-Type: application/json

{
  "error": "build_failed",
  "message": "Failed to initiate build: <error_details>"
}
```

### 2. Get Build Details Endpoint

```http
GET /api/v1/build/:server_id/:deployment_id
Authorization: Bearer <JWT_TOKEN>
```

**Request Parameters:**
- `server_id` (path): MCP server identifier
- `deployment_id` (path): Deployment identifier
- `Authorization` (header): JWT authentication token

**Success Response:**
```json
HTTP/1.1 200 OK
Content-Type: application/json

{
  "server_id": "server-1",
  "deployment_id": "deploy-1",
  "user_id": "auth0|user123",
  "branch": "main",
  "commit_hash": "a1b2c3d4e5f6...",
  "status": "completed",
  "image_uri": "123456789.dkr.ecr.us-east-1.amazonaws.com/mcp-server-1:main-a1b2c3d4",
  "stages": {
    "clone": {
      "status": "completed",
      "started_at": "2024-11-11T10:30:45Z",
      "completed_at": "2024-11-11T10:30:50Z"
    },
    "validate_config": {
      "status": "completed",
      "started_at": "2024-11-11T10:30:51Z",
      "completed_at": "2024-11-11T10:30:52Z"
    },
    "validate_docker": {
      "status": "completed",
      "started_at": "2024-11-11T10:30:52Z",
      "completed_at": "2024-11-11T10:30:53Z"
    },
    "build_image": {
      "status": "completed",
      "started_at": "2024-11-11T10:30:54Z",
      "completed_at": "2024-11-11T10:31:15Z"
    },
    "create_ecr": {
      "status": "completed",
      "started_at": "2024-11-11T10:31:16Z",
      "completed_at": "2024-11-11T10:31:20Z"
    },
    "push_image": {
      "status": "completed",
      "started_at": "2024-11-11T10:31:21Z",
      "completed_at": "2024-11-11T10:31:45Z"
    }
  },
  "build_logs": [
    {
      "timestamp": "2024-11-11T10:30:45Z",
      "stage": "clone",
      "level": "info",
      "message": "Starting repository clone..."
    },
    {
      "timestamp": "2024-11-11T10:31:45Z",
      "stage": "push_image",
      "level": "info",
      "message": "Image pushed to ECR successfully"
    }
  ],
  "created_at": "2024-11-11T10:30:00Z",
  "updated_at": "2024-11-11T10:31:45Z"
}
```

**Error Responses:**
```json
HTTP/1.1 401 Unauthorized
HTTP/1.1 403 Forbidden
HTTP/1.1 404 Not Found
HTTP/1.1 500 Internal Server Error
```

### 3. Health Check Endpoint

```http
GET /api/v1/health
```

**Success Response:**
```json
HTTP/1.1 200 OK
Content-Type: application/json

{
  "status": "ok"
}
```

---

## Data Models

### 1. Deployment Model

```go
type Deployment struct {
  // Primary identifiers
  ServerId     string                       // MCP server ID (Partition Key)
  DeploymentId string                       // Deployment ID (Sort Key)

  // User information
  UserId       string                       // Auth0 user ID for ownership

  // Git information
  Branch       string                       // Git branch to build
  CommitHash   string                       // Git commit SHA

  // Build status
  Status       string                       // queued|in_progress|completed|failed

  // Stage tracking
  Stages       map[string]*BuildStageStatus // Per-stage status tracking

  // Build logs
  BuildLogs    []BuildLogEntry              // Structured log entries

  // Build artifact
  ImageURI     string                       // Final ECR image URI

  // Timestamps
  CreatedAt    time.Time                    // Record creation time
  UpdatedAt    time.Time                    // Last update time
}
```

**Example Deployment:**
```json
{
  "server_id": "server-1",
  "deployment_id": "deploy-1",
  "user_id": "auth0|user123",
  "branch": "main",
  "commit_hash": "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
  "status": "completed",
  "stages": {
    "clone": {
      "status": "completed",
      "started_at": "2024-11-11T10:30:45Z",
      "completed_at": "2024-11-11T10:30:50Z",
      "error": null
    },
    "validate_config": {
      "status": "completed",
      "started_at": "2024-11-11T10:30:51Z",
      "completed_at": "2024-11-11T10:30:52Z",
      "error": null
    },
    "validate_docker": {
      "status": "completed",
      "started_at": "2024-11-11T10:30:52Z",
      "completed_at": "2024-11-11T10:30:53Z",
      "error": null
    },
    "build_image": {
      "status": "completed",
      "started_at": "2024-11-11T10:30:54Z",
      "completed_at": "2024-11-11T10:31:15Z",
      "error": null
    },
    "create_ecr": {
      "status": "completed",
      "started_at": "2024-11-11T10:31:16Z",
      "completed_at": "2024-11-11T10:31:20Z",
      "error": null
    },
    "push_image": {
      "status": "completed",
      "started_at": "2024-11-11T10:31:21Z",
      "completed_at": "2024-11-11T10:31:45Z",
      "error": null
    }
  },
  "build_logs": [
    {
      "timestamp": "2024-11-11T10:30:45Z",
      "stage": "clone",
      "level": "info",
      "message": "Starting repository clone..."
    }
  ],
  "image_uri": "123456789.dkr.ecr.us-east-1.amazonaws.com/mcp-server-1:main-a1b2c3d4",
  "created_at": "2024-11-11T10:30:00Z",
  "updated_at": "2024-11-11T10:31:45Z"
}
```

### 2. MCP Server Model

```go
type MCPServer struct {
  Id                   string                    // Server ID (Primary Key)
  UserId               string                    // Auth0 user ID (owner)
  Name                 string                    // Server name
  Description          string                    // Server description
  Repository           string                    // GitHub repo URL (HTTPS)
  Status               string                    // active|inactive|archived
  EnvironmentVariables []EnvironmentVariable     // Build env vars
  CreatedAt            time.Time                 // Creation timestamp
  UpdatedAt            time.Time                 // Last update timestamp
}

type EnvironmentVariable struct {
  Key   string
  Value string
}
```

### 3. GitHub Connection Model

```go
type GitHubConnection struct {
  UserId       string    // Auth0 user ID (Primary Key)
  AccessToken  string    // Encrypted GitHub OAuth token
  TokenType    string    // "bearer"
  ExpiresIn    int       // Token expiration in seconds
  RefreshToken string    // For token refresh
  CreatedAt    time.Time // Token grant timestamp
  UpdatedAt    time.Time // Last refresh timestamp
}
```

### 4. Build Stage Status

```go
type BuildStageStatus struct {
  Status      string     // pending|in_progress|completed|failed
  StartedAt   *time.Time // Stage start time (null if not started)
  CompletedAt *time.Time // Stage completion time (null if incomplete)
  Error       string     // Error message if failed
}

// Stage names:
// - "clone": Repository cloning
// - "validate_config": Config file validation
// - "validate_docker": Dockerfile validation
// - "build_image": Docker image building
// - "create_ecr": ECR repository creation
// - "push_image": Push to ECR
```

### 5. Build Log Entry

```go
type BuildLogEntry struct {
  Timestamp time.Time // Log entry timestamp (RFC3339 format)
  Stage     string    // Stage name (6 stages available)
  Level     string    // "info"|"warning"|"error"
  Message   string    // Log message content
}
```

---

## Request Flow

### Complete Request Lifecycle Diagram

```
┌──────────────────────────────────────────────────────────────────┐
│ 1. CLIENT INITIATES BUILD REQUEST                                │
│    ────────────────────────────────────────────────────────────  │
│    POST /api/v1/build/server-1/deploy-1/initiate                │
│    Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9... │
│                                                                  │
│    Headers:                                                      │
│    • Content-Type: application/json                             │
│    • Authorization: Bearer <JWT_TOKEN>                          │
└────────────────────────┬───────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────────┐
│ 2. CORS MIDDLEWARE                                               │
│    ────────────────────────────────────────────────────────────  │
│    • Validate request origin                                    │
│    • Check allowed methods                                      │
│    • Add CORS headers to response                               │
└────────────────────────┬───────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────────┐
│ 3. JWT AUTHENTICATION MIDDLEWARE                                 │
│    ────────────────────────────────────────────────────────────  │
│    • Extract token from Authorization header                    │
│    • Validate token format (Bearer scheme)                      │
│    • Verify JWT signature                                       │
│    • Check token expiration                                     │
│    • Extract 'sub' claim (user ID)                              │
│    • Set user_id in request context                             │
│                                                                  │
│    If fails: Return 401 Unauthorized                            │
│    ✓ user_id: "auth0|user123"                                   │
└────────────────────────┬───────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────────┐
│ 4. BUILD HANDLER - InitiateBuild()                               │
│    ────────────────────────────────────────────────────────────  │
│    a) Extract Parameters                                        │
│       • server_id: "server-1"                                   │
│       • deployment_id: "deploy-1"                               │
│       • user_id: "auth0|user123" (from context)                 │
│                                                                  │
│    b) Validate Parameters                                       │
│       ✓ server_id not empty                                     │
│       ✓ deployment_id not empty                                 │
│                                                                  │
│    If validation fails: Return 400 Bad Request                  │
└────────────────────────┬───────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────────┐
│ 5. DATABASE QUERIES - REPOSITORY LAYER                           │
│    ────────────────────────────────────────────────────────────  │
│                                                                  │
│    Query A: Get Deployment Record                               │
│    • Table: deployments                                         │
│    • Key: server_id="server-1", deployment_id="deploy-1"       │
│    • Returns: branch, commit_hash, status                       │
│    ✓ Found and valid                                            │
│                                                                  │
│    Query B: Check Ownership                                     │
│    • Verify deployment.user_id == context.user_id               │
│    • OR deployment.user_id is in allowed users                  │
│    ✓ User owns deployment                                       │
│                                                                  │
│    Query C: Get MCP Server Record                               │
│    • Table: mcp-servers                                         │
│    • Key: server_id="server-1"                                  │
│    • Returns: repository, user_id, status                       │
│    ✓ Found and valid                                            │
│                                                                  │
│    Query D: Check Server Ownership                              │
│    • Verify server.user_id == context.user_id                   │
│    ✓ User owns server                                           │
│                                                                  │
│    Query E: Get GitHub Connection                               │
│    • Table: github-connections                                  │
│    • Key: user_id="auth0|user123"                               │
│    • Returns: encrypted access_token                            │
│    ✓ Connection exists                                          │
│                                                                  │
│    If any query fails or ownership check fails:                 │
│    Return 403 Forbidden or 404 Not Found                        │
└────────────────────────┬───────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────────┐
│ 6. BUILD JOB CREATION                                            │
│    ────────────────────────────────────────────────────────────  │
│    Create BuildJob object:                                      │
│    {                                                             │
│      "deployment_id": "deploy-1",                               │
│      "server_id": "server-1",                                   │
│      "user_id": "auth0|user123",                                │
│      "branch": "main",                                          │
│      "commit_hash": "a1b2c3d4...",                              │
│      "repository": "https://github.com/owner/repo.git"         │
│    }                                                             │
└────────────────────────┬───────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────────┐
│ 7. JOB QUEUE ENQUEUE                                             │
│    ────────────────────────────────────────────────────────────  │
│    • Add job to channel buffer (max 100)                        │
│    • Job status: queued                                         │
│    • Update deployment status in DynamoDB: "queued"             │
│                                                                  │
│    If queue is full: Return 429 Too Many Requests               │
│    ✓ Job enqueued successfully                                  │
└────────────────────────┬───────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────────┐
│ 8. IMMEDIATE RESPONSE TO CLIENT                                  │
│    ────────────────────────────────────────────────────────────  │
│    HTTP 202 Accepted                                            │
│    {                                                             │
│      "message": "Build initiated successfully",                 │
│      "server_id": "server-1",                                   │
│      "deployment_id": "deploy-1",                               │
│      "status": "in_progress"                                    │
│    }                                                             │
│                                                                  │
│    NOTE: Build continues asynchronously in worker pool          │
└────────────────────────┬───────────────────────────────────────┘
                         │
            ┌────────────┴────────────┐
            │ (Asynchronous Process)  │
            ▼                         ▼
    ┌─────────────────┐     ┌──────────────────┐
    │ Worker 1 picks  │     │ Worker 2 picks   │
    │ job from queue  │     │ job from queue   │
    │ if available    │     │ if available     │
    └────────┬────────┘     └────────┬─────────┘
             │                       │
             ▼                       ▼
    ┌──────────────────────────────────────────┐
    │ 9. PIPELINE EXECUTION - 6-STAGE PROCESS  │
    │    ──────────────────────────────────    │
    │                                          │
    │ Stage 1: Clone Repository               │
    │  • Decrypt GitHub token                │
    │  • Git clone with token in URL         │
    │  • Git checkout commit hash            │
    │  ✓ Success → Update deployment status  │
    │    ✗ Failure → Mark stage failed, stop│
    │                                          │
    │ Stage 2: Validate Config                │
    │  • Check mhive.config.yaml exists      │
    │  • Parse and validate YAML             │
    │  ✓ Success → Update deployment status  │
    │    ✗ Failure → Mark stage failed, stop│
    │                                          │
    │ Stage 3: Validate Dockerfile           │
    │  • Check Dockerfile exists             │
    │  • Verify readability                  │
    │  ✓ Success → Update deployment status  │
    │    ✗ Failure → Mark stage failed, stop│
    │                                          │
    │ Stage 4: Build Docker Image            │
    │  • Execute docker build command        │
    │  • Capture output                      │
    │  ✓ Success → Update deployment status  │
    │    ✗ Failure → Mark stage failed, stop│
    │                                          │
    │ Stage 5: Create ECR Repository         │
    │  • Check if repo exists in AWS ECR     │
    │  • Create if not exists                │
    │  ✓ Success → Update deployment status  │
    │    ✗ Failure → Mark stage failed, stop│
    │                                          │
    │ Stage 6: Push Image to ECR             │
    │  • Docker login to ECR                 │
    │  • Tag image (two tags)                │
    │  • docker push both tags               │
    │  ✓ Success → Update deployment status  │
    │    ✗ Failure → Mark stage failed, stop│
    └────────────┬─────────────────────────────┘
                 │
                 ▼
    ┌──────────────────────────────────────────┐
    │ 10. PIPELINE COMPLETION                  │
    │     ──────────────────────────────────   │
    │                                          │
    │ Update Deployment Record:                │
    │ • Status: "completed" or "failed"        │
    │ • All stages with final status          │
    │ • Build logs (if not exceeding 400KB)   │
    │ • Image URI (if successful)             │
    │ • Updated timestamp                     │
    │                                          │
    │ Persist to DynamoDB                     │
    │ ✓ Deployment record updated             │
    │                                          │
    │ Cleanup:                                 │
    │ • Delete temporary build directory      │
    │ • Clean up Docker build artifacts       │
    └──────────────────────────────────────────┘

TIME FLOW:
T0:   Client sends request
T1:   Authentication & validation (< 100ms)
T2:   Database queries (< 500ms)
T3:   Job enqueued, return 202 (< 100ms)
T4+:  Asynchronous pipeline execution (5-30 minutes depending on)
      - Repository size
      - Dockerfile complexity
      - Network latency
      - ECR performance
```

---

## Database Schema

### 1. DynamoDB Table: deployments

```
Table Name: deployments
Billing Mode: PAY_PER_REQUEST

Primary Key:
  • Partition Key: serverId (String)
  • Sort Key: deploymentId (String)

Attributes:
├── serverId (String, Partition Key)
├── deploymentId (String, Sort Key)
├── userId (String)
├── branch (String)
├── commitHash (String)
├── status (String)
│   └─ Values: "queued", "in_progress", "completed", "failed"
├── stages (Map)
│   ├── clone (Map)
│   │   ├── status (String)
│   │   ├── startedAt (String, ISO8601)
│   │   ├── completedAt (String, ISO8601)
│   │   └── error (String, nullable)
│   ├── validate_config (Map) → Same structure
│   ├── validate_docker (Map) → Same structure
│   ├── build_image (Map) → Same structure
│   ├── create_ecr (Map) → Same structure
│   └── push_image (Map) → Same structure
├── buildLogs (List)
│   └── [0..N] (Map)
│       ├── timestamp (String, ISO8601)
│       ├── stage (String)
│       ├── level (String) - "info", "warning", "error"
│       └── message (String)
├── imageUri (String, nullable)
├── createdAt (String, ISO8601)
└── updatedAt (String, ISO8601)

Global Secondary Indexes:
  • userId-deploymentId-index
    Partition Key: userId
    Sort Key: deploymentId
    (Enables querying deployments by user)

Example Item:
{
  "serverId": "server-1",
  "deploymentId": "deploy-1",
  "userId": "auth0|user123",
  "branch": "main",
  "commitHash": "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
  "status": "completed",
  "stages": {
    "clone": {
      "status": "completed",
      "startedAt": "2024-11-11T10:30:45Z",
      "completedAt": "2024-11-11T10:30:50Z",
      "error": null
    },
    "build_image": {
      "status": "completed",
      "startedAt": "2024-11-11T10:30:54Z",
      "completedAt": "2024-11-11T10:31:15Z",
      "error": null
    },
    "push_image": {
      "status": "completed",
      "startedAt": "2024-11-11T10:31:21Z",
      "completedAt": "2024-11-11T10:31:45Z",
      "error": null
    }
  },
  "buildLogs": [
    {
      "timestamp": "2024-11-11T10:30:45Z",
      "stage": "clone",
      "level": "info",
      "message": "Starting repository clone..."
    },
    {
      "timestamp": "2024-11-11T10:31:45Z",
      "stage": "push_image",
      "level": "info",
      "message": "Image pushed successfully"
    }
  ],
  "imageUri": "123456789.dkr.ecr.us-east-1.amazonaws.com/mcp-server-1:main-a1b2c3d4",
  "createdAt": "2024-11-11T10:30:00Z",
  "updatedAt": "2024-11-11T10:31:45Z"
}
```

### 2. DynamoDB Table: mcp-servers

```
Table Name: mcp-servers (default, configurable)
Billing Mode: PAY_PER_REQUEST

Primary Key:
  • Partition Key: id (String)

Attributes:
├── id (String, Primary Key)
├── userId (String)
├── name (String)
├── description (String)
├── repository (String) - GitHub HTTPS URL
├── status (String) - "active", "inactive", "archived"
├── environmentVariables (List)
│   └── [0..N] (Map)
│       ├── key (String)
│       └── value (String)
├── createdAt (String, ISO8601)
└── updatedAt (String, ISO8601)

Global Secondary Indexes:
  • userId-id-index
    Partition Key: userId
    Sort Key: id
    (Enables querying servers by user)

Example Item:
{
  "id": "server-1",
  "userId": "auth0|user123",
  "name": "My MCP Server",
  "description": "Custom MCP server for AI applications",
  "repository": "https://github.com/owner/mcp-server.git",
  "status": "active",
  "environmentVariables": [
    {
      "key": "API_KEY",
      "value": "secret-value"
    },
    {
      "key": "DEBUG",
      "value": "false"
    }
  ],
  "createdAt": "2024-11-11T10:00:00Z",
  "updatedAt": "2024-11-11T10:30:00Z"
}
```

### 3. DynamoDB Table: github-connections

```
Table Name: github-connections (default, configurable)
Billing Mode: PAY_PER_REQUEST

Primary Key:
  • Partition Key: userId (String)

Attributes:
├── userId (String, Primary Key) - Auth0 user ID
├── accessToken (String) - AES-256-GCM encrypted
├── tokenType (String) - "bearer"
├── expiresIn (Number) - Seconds until expiration
├── refreshToken (String, nullable) - For token refresh
├── createdAt (String, ISO8601)
└── updatedAt (String, ISO8601)

Example Item (encrypted):
{
  "userId": "auth0|user123",
  "accessToken": "eyJ0eXAiOiJKV1QiLCJhbGc...[encrypted]",
  "tokenType": "bearer",
  "expiresIn": 3600,
  "refreshToken": "ghu_1234567890abcdef...",
  "createdAt": "2024-11-11T10:00:00Z",
  "updatedAt": "2024-11-11T10:30:00Z"
}

Note: accessToken is encrypted using:
- Algorithm: AES-256-GCM
- Key: 32-character string from GITHUB_TOKEN_ENCRYPTION_KEY env var
- Never stored or logged in plaintext
```

### 4. Query Patterns

**Query Pattern 1: Get Deployment**
```
Table: deployments
Key: {serverId: "server-1", deploymentId: "deploy-1"}
Result: Complete deployment record with all stages and logs
```

**Query Pattern 2: Get User's Deployments**
```
Table: deployments
Index: userId-deploymentId-index
Query: userId = "auth0|user123"
Result: All deployments owned by user
```

**Query Pattern 3: Get GitHub Connection**
```
Table: github-connections
Key: {userId: "auth0|user123"}
Result: Encrypted GitHub access token and metadata
```

**Query Pattern 4: Get MCP Server**
```
Table: mcp-servers
Key: {id: "server-1"}
Result: Server details including repository URL and environment variables
```

**Query Pattern 5: Get User's MCP Servers**
```
Table: mcp-servers
Index: userId-id-index
Query: userId = "auth0|user123"
Result: All MCP servers owned by user
```

---

## Summary

The BuildServer implements a robust, production-grade pipeline for containerized application deployment:

### Parameters Required:
1. **Environment Configuration**: 11 environment variables for AWS, GitHub, and server settings
2. **Build Request Parameters**: server_id, deployment_id, JWT token
3. **Database Records**: Deployment, MCP Server, GitHub Connection
4. **Build Inputs**: Git branch, commit hash, Dockerfile, mhive.config.yaml

### Build Process:
1. **Clone** → **Validate Config** → **Validate Dockerfile** → **Build Image** → **Create ECR** → **Push Image**
2. Each stage is tracked with status, timestamps, and error information
3. Comprehensive logging with 400KB size limit per deployment
4. Asynchronous job processing with worker pool (default 5 workers)
5. Complete deployment tracking in DynamoDB with detailed stage information

### Key Features:
- Multi-stage pipeline with fine-grained error handling
- ECR repository auto-creation and tagging
- GitHub OAuth token encryption at rest
- Request-response API with comprehensive error handling
- Graceful shutdown and resource cleanup
