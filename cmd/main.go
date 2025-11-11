package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/imyashkale/buildserver/internal/config"
	"github.com/imyashkale/buildserver/internal/database"
	"github.com/imyashkale/buildserver/internal/handlers"
	"github.com/imyashkale/buildserver/internal/queue"
	"github.com/imyashkale/buildserver/internal/repository"
	"github.com/imyashkale/buildserver/internal/router"
	"github.com/imyashkale/buildserver/internal/services"
)

func main() {

	ctx := context.Background()

	// Load application configuration
	cfg := config.New()
	log.Println("Configuration loaded successfully")

	// Initialize database configuration
	dbConfig := database.NewConfig(cfg)

	log.Printf("Initializing DynamoDB client for table: %s in region: %s", dbConfig.TableName, dbConfig.Region)

	// Create DynamoDB client
	dbClient, err := database.NewClient(ctx, dbConfig)
	if err != nil {
		log.Fatalf("Failed to initialize DynamoDB client: %v", err)
	}

	log.Println("DynamoDB client initialized successfully")

	// Initialize database operations
	mdb := database.NewMCPServer(dbClient, dbClient.TableName)

	// Initialize GitHub database operations
	githubDB := database.NewGitHubDB(dbClient, cfg.GitHubConnectionsTableName, cfg.GitHubOAuthStatesTableName)
	log.Println("GitHub database initialized")

	// Initialize deployment database operations
	deploymentDB := database.NewDeploymentOperations(dbClient, cfg.DeploymentsTableName)
	log.Println("Deployment database initialized")

	// Initialize repositories
	mrepo := repository.NewMCPRepository(mdb)
	githubRepo := repository.NewGitHubRepository(githubDB)
	deploymentRepo := repository.NewDeploymentRepository(deploymentDB)
	log.Println("Repositories initialized with DynamoDB backend")

	// Initialize GitHub service with config values
	githubService := services.NewGitHubService(
		githubRepo,
		cfg.GitHubClientID,
		cfg.GitHubClientSecret,
		cfg.GitHubCallbackURL,
		cfg.GitHubTokenEncryptionKey,
	)

	log.Println("GitHub service initialized")

	// Initialize job queue (with buffer size of 100)
	jobQueue := queue.NewJobQueue(100)
	log.Println("Job queue initialized")

	// Load AWS configuration for ECR
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to load AWS configuration: %v", err)
	}

	// Initialize ECR service
	ecrService := services.NewECRService(awsCfg, cfg.AWSAccountID)
	log.Println("ECR service initialized")

	// Initialize pipeline service
	pipelineService := services.NewPipelineService(deploymentRepo, githubService, ecrService, mrepo, githubRepo)
	log.Println("Pipeline service initialized")

	// Initialize worker pool (5 concurrent workers)
	workerPool := queue.NewWorkerPool(jobQueue, 5)
	log.Println("Worker pool created with 5 concurrent workers")

	// Start workers
	workerPool.Start(func(job *queue.BuildJob) error {
		return pipelineService.ExecuteBuild(ctx, job)
	})
	log.Println("Build workers started")

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler()
	buildHandler := handlers.NewBuildHandler(
		mrepo,
		deploymentRepo,
		githubRepo,
		githubService,
		pipelineService,
		jobQueue,
	)
	log.Println("Handlers initialized")

	// Setup router
	r := router.Setup(healthHandler, buildHandler)

	// Setup graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down server gracefully...")

		// Close job queue to stop accepting new jobs
		jobQueue.Close()
		log.Println("Job queue closed, waiting for workers to finish...")

		// Wait for workers to finish processing current jobs
		workerPool.Wait()
		log.Println("All workers stopped")

		os.Exit(0)
	}()

	// Start server
	log.Printf("Starting server on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
