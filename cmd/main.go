package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/imyashkale/buildserver/internal/config"
	"github.com/imyashkale/buildserver/internal/database"
	"github.com/imyashkale/buildserver/internal/handlers"
	"github.com/imyashkale/buildserver/internal/logger"
	"github.com/imyashkale/buildserver/internal/queue"
	"github.com/imyashkale/buildserver/internal/repository"
	"github.com/imyashkale/buildserver/internal/router"
	"github.com/imyashkale/buildserver/internal/services"
)

func main() {

	ctx := context.Background()

	// Load application configuration
	cfg := config.New()

	// Initialize logger with configured log level
	logger.Init(cfg.LogLevel)

	logger.Infof("Configuration loaded successfully - LogLevel: %s", cfg.LogLevel)

	// Initialize database configuration
	dbConfig := database.NewConfig(cfg)

	logger.Infof("Initializing DynamoDB client for table: %s in region: %s", dbConfig.TableName, dbConfig.Region)

	// Create DynamoDB client
	dbClient, err := database.NewClient(ctx, dbConfig)
	if err != nil {
		logger.Fatalf("Failed to initialize DynamoDB client: %v", err)
	}

	logger.Info("DynamoDB client initialized successfully")

	// Initialize database operations
	mdb := database.NewMCPServer(dbClient, dbClient.TableName)

	// Initialize GitHub database operations
	githubDB := database.NewGitHubDB(dbClient, cfg.GitHubConnectionsTableName, cfg.GitHubOAuthStatesTableName)
	logger.Info("GitHub database initialized")

	// Initialize deployment database operations
	deploymentDB := database.NewDeploymentOperations(dbClient, cfg.DeploymentsTableName)
	logger.Info("Deployment database initialized")

	// Initialize repositories
	mrepo := repository.NewMCPRepository(mdb)
	githubRepo := repository.NewGitHubRepository(githubDB)
	deploymentRepo := repository.NewDeploymentRepository(deploymentDB)
	logger.Info("Repositories initialized with DynamoDB backend")

	// Initialize GitHub service with config values
	githubService := services.NewGitHubService(
		githubRepo,
		cfg.GitHubClientID,
		cfg.GitHubClientSecret,
		cfg.GitHubCallbackURL,
		cfg.GitHubTokenEncryptionKey,
	)

	logger.Info("GitHub service initialized")

	// Initialize job queue (with buffer size of 100)
	jobQueue := queue.NewJobQueue(100)
	logger.Info("Job queue initialized")

	// Load AWS configuration for ECR
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Fatalf("Failed to load AWS configuration: %v", err)
	}

	// Initialize ECR service
	ecrService := services.NewECRService(awsCfg, cfg.AWSAccountID)
	logger.Info("ECR service initialized")

	// Initialize pipeline service
	pipelineService := services.NewPipelineService(deploymentRepo, githubService, ecrService, mrepo, githubRepo)
	logger.Info("Pipeline service initialized")

	// Initialize worker pool (5 concurrent workers)
	workerPool := queue.NewWorkerPool(jobQueue, 5)
	logger.Info("Worker pool created with 5 concurrent workers")

	// Start workers
	workerPool.Start(func(job *queue.BuildJob) error {
		return pipelineService.ExecuteBuild(ctx, job)
	})
	logger.Info("Build workers started")

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler()
	buildHandler := handlers.NewBuildHandler(
		mrepo,
		deploymentRepo,
		githubRepo,
		jobQueue,
	)
	logger.Info("Handlers initialized")

	// Setup router
	r := router.Setup(healthHandler, buildHandler)
	logger.Info("Router setup completed")

	// Setup graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		logger.Info("Shutdown signal received")

		// Close job queue to stop accepting new jobs
		jobQueue.Close()
		logger.Info("Job queue closed, waiting for workers to finish")

		// Wait for workers to finish processing current jobs
		workerPool.Wait()
		logger.Info("All workers stopped gracefully")

		logger.Info("Server shutdown complete")
		os.Exit(0)
	}()

	// Start server
	logger.Infof("Starting HTTP server on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
}
