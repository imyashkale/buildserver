package queue

import (
	"sync"
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
	select {
	case jq.jobs <- job:
		return nil
	case <-jq.done:
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
				return
			}
			if job != nil {
				_ = handler(job)
			}
		case <-wp.done:
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
