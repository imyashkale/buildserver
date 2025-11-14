# Build Server Project Memory

## System Architecture Overview

This is a **build server** that is part of a three-tier system:

### Components

1. **Build Server** (current project)
   - Orchestrates and executes builds
   - Depends on values created from mbackend
   - Coordinates with both frontend and backend services

2. **mbackend** (Backend Repository)
   - Location: `cd ../mbackend`
   - Responsible for creating/generating values that the build server depends on
   - Exposes multiple API endpoints for integration
   - Receives requests from mfrontend
   - Serves data to the build server

3. **mfrontend** (Frontend Repository)
   - Location: `cd ../mfrontend`
   - Frontend user interface
   - Sends values/requests to mbackend
   - Feeds into the build pipeline

### Data Flow

```
Frontend (mfrontend) → Backend (mbackend) → Build Server (orchestration)
```

### Key Points
- All endpoints that expose data can be found in the mbackend repository
- Build server operations depend on data/values from mbackend
- This is a distributed system with clear separation of concerns

## Coding Standards
- Always use Pascal Case for DynamoDB field names
