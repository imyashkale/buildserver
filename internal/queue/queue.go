package queue

import (
	"sync"

	"github.com/imyashkale/buildserver/internal/logger"
)

// BuildJob represents a build job in the queue
type BuildJob struct {
	DeploymentID string
	ServerID     string
	UserID       string
	Branch       string
	CommitHash   string
}

// JobQueue manages the job queue with a channel-based system
type JobQueue struct {
	jobs chan *BuildJob
	done chan bool
	mu   sync.Mutex
}

// NewJobQueue creates a new job queue with the specified buffer size
func NewJobQueue(bufferSize int) *JobQueue {
	return &JobQueue{
		jobs: make(chan *BuildJob, bufferSize),
		done: make(chan bool),
	}
}

// Enqueue adds a job to the queue
func (jq *JobQueue) Enqueue(job *BuildJob) error {
	logger.WithFields(map[string]interface{}{
		"deployment_id": job.DeploymentID,
		"server_id":     job.ServerID,
		"user_id":       job.UserID,
	}).Debug("Enqueueing build job")

	select {
	case jq.jobs <- job:
		logger.WithFields(map[string]interface{}{
			"deployment_id": job.DeploymentID,
			"server_id":     job.ServerID,
		}).Info("Build job enqueued successfully")
		return nil
	case <-jq.done:
		logger.WithFields(map[string]interface{}{
			"deployment_id": job.DeploymentID,
			"server_id":     job.ServerID,
		}).Warn("Failed to enqueue job: queue is closed")
		return ErrQueueClosed
	}
}

// Dequeue retrieves the next job from the queue
// Returns nil if the queue is closed
func (jq *JobQueue) Dequeue() *BuildJob {
	return <-jq.jobs
}

// Jobs returns the underlying channel for job consumption
func (jq *JobQueue) Jobs() <-chan *BuildJob {
	return jq.jobs
}

// Close closes the queue
func (jq *JobQueue) Close() {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	select {
	case <-jq.done:
		return // Already closed
	default:
		close(jq.done)
		close(jq.jobs)
	}
}

// WorkerPool manages multiple workers processing jobs
type WorkerPool struct {
	queue   *JobQueue
	workers int
	jobs    chan *BuildJob
	wg      sync.WaitGroup
	done    chan bool
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(queue *JobQueue, numWorkers int) *WorkerPool {
	return &WorkerPool{
		queue:   queue,
		workers: numWorkers,
		jobs:    queue.jobs,
		done:    make(chan bool),
	}
}

// Start starts all workers
func (wp *WorkerPool) Start(handler func(*BuildJob) error) {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(handler)
	}
}

// worker processes jobs from the queue
func (wp *WorkerPool) worker(handler func(*BuildJob) error) {
	defer wp.wg.Done()

	for {
		select {
		case job, ok := <-wp.jobs:
			if !ok {
				logger.Debug("Worker exiting: jobs channel closed")
				return
			}
			if job != nil {
				logger.WithFields(map[string]interface{}{
					"deployment_id": job.DeploymentID,
					"server_id":     job.ServerID,
					"user_id":       job.UserID,
				}).Info("Worker processing build job")

				err := handler(job)
				if err != nil {
					logger.WithFields(map[string]interface{}{
						"deployment_id": job.DeploymentID,
						"server_id":     job.ServerID,
						"error":         err.Error(),
					}).Error("Worker failed to process build job")
				} else {
					logger.WithFields(map[string]interface{}{
						"deployment_id": job.DeploymentID,
						"server_id":     job.ServerID,
					}).Info("Worker completed build job successfully")
				}
			}
		case <-wp.done:
			logger.Debug("Worker exiting: stop signal received")
			return
		}
	}
}

// Stop stops all workers
func (wp *WorkerPool) Stop() {
	close(wp.done)
	wp.wg.Wait()
}

// Wait waits for all workers to finish
func (wp *WorkerPool) Wait() {
	wp.wg.Wait()
}
