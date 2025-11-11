package router

import (
	"github.com/gin-gonic/gin"
	"github.com/imyashkale/buildserver/internal/handlers"
	"github.com/imyashkale/buildserver/internal/middleware"
)

// Setup configures and returns the application router
func Setup(
	healthHandler *handlers.HealthHandler,
	mcpHandler *handlers.MCPHandler,
	authHandler *handlers.AuthHandler,
	githubHandler *handlers.GitHubHandler,
	deploymentHandler *handlers.DeploymentHandler,
	buildHandler *handlers.BuildHandler,
) *gin.Engine {

	// Create a new Gin router
	router := gin.Default()

	// Apply CORS middleware globally
	router.Use(middleware.CORS())

	// API v1 routes
	v1 := router.Group("/api/v1")

	// Apply authentication middleware to all routes
	v1.Use(middleware.Authentication())

	// Health check
	v1.GET("/health", healthHandler.Check)

	// Build routes
	build := v1.Group("/build")
	{
		build.POST("/:server_id/:deployment_id/initiate", buildHandler.InitiateBuild)
		build.GET("/:server_id/:deployment_id", buildHandler.GetBuildDetails)
	}

	return router
}
