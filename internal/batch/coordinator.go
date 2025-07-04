// internal/batch/coordinator.go - Batch coordination implementation
package batch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/valpere/tile_to_json/internal"
	"github.com/valpere/tile_to_json/internal/tile"
)

// DefaultCoordinator implements the Coordinator interface
type DefaultCoordinator struct {
	jobs      map[string]*Job
	processor Processor
	store     JobStore
	mutex     sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewDefaultCoordinator creates a new default batch coordinator
func NewDefaultCoordinator(processor Processor, store JobStore) *DefaultCoordinator {
	ctx, cancel := context.WithCancel(context.Background())

	return &DefaultCoordinator{
		jobs:      make(map[string]*Job),
		processor: processor,
		store:     store,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// SubmitJob submits a new batch processing job
func (c *DefaultCoordinator) SubmitJob(job *Job) error {
	if job.ID == "" {
		return internal.NewError(internal.ErrorCodeValidation, "job ID is required", nil)
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if job already exists
	if _, exists := c.jobs[job.ID]; exists {
		return internal.NewError(internal.ErrorCodeValidation, fmt.Sprintf("job %s already exists", job.ID), nil)
	}

	// Validate job configuration
	if err := c.validateJob(job); err != nil {
		return internal.NewError(internal.ErrorCodeValidation, "job validation failed", err)
	}

	// Initialize job progress
	job.Progress = NewJobProgress()
	job.CreatedAt = time.Now()
	job.Status = JobStatusPending

	// Store job
	c.jobs[job.ID] = job

	// Persist job if store is available
	if c.store != nil {
		if err := c.store.SaveJob(job); err != nil {
			delete(c.jobs, job.ID)
			return internal.NewError(internal.ErrorCodeProcessing, "failed to persist job", err)
		}
	}

	// Start processing asynchronously
	go func() {
		jobCtx, jobCancel := context.WithTimeout(c.ctx, job.Config.Timeout)
		defer jobCancel()

		if err := c.processor.Process(jobCtx, job); err != nil {
			c.mutex.Lock()
			job.Status = JobStatusFailed
			job.Error = err
			now := time.Now()
			job.CompletedAt = &now
			c.mutex.Unlock()

			// Update stored job
			if c.store != nil {
				c.store.SaveJob(job)
			}
		}
	}()

	return nil
}

// GetJob retrieves a job by its ID
func (c *DefaultCoordinator) GetJob(id string) (*Job, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	job, exists := c.jobs[id]
	if !exists {
		// Try to load from store
		if c.store != nil {
			storedJob, err := c.store.LoadJob(id)
			if err == nil {
				c.jobs[id] = storedJob
				return storedJob, nil
			}
		}
		return nil, internal.NewError(internal.ErrorCodeNotFound, fmt.Sprintf("job %s not found", id), nil)
	}

	return job, nil
}

// CancelJob cancels a running or pending job
func (c *DefaultCoordinator) CancelJob(id string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	job, exists := c.jobs[id]
	if !exists {
		return internal.NewError(internal.ErrorCodeNotFound, fmt.Sprintf("job %s not found", id), nil)
	}

	if job.IsComplete() {
		return internal.NewError(internal.ErrorCodeValidation, fmt.Sprintf("job %s is already complete", id), nil)
	}

	job.Status = JobStatusCanceled
	now := time.Now()
	job.CompletedAt = &now

	// Update stored job
	if c.store != nil {
		c.store.SaveJob(job)
	}

	return nil
}

// PauseJob pauses a running job
func (c *DefaultCoordinator) PauseJob(id string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	job, exists := c.jobs[id]
	if !exists {
		return internal.NewError(internal.ErrorCodeNotFound, fmt.Sprintf("job %s not found", id), nil)
	}

	if !job.IsRunning() {
		return internal.NewError(internal.ErrorCodeValidation, fmt.Sprintf("job %s is not running", id), nil)
	}

	job.Status = JobStatusPaused

	// Update stored job
	if c.store != nil {
		c.store.SaveJob(job)
	}

	return nil
}

// ResumeJob resumes a paused job
func (c *DefaultCoordinator) ResumeJob(id string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	job, exists := c.jobs[id]
	if !exists {
		return internal.NewError(internal.ErrorCodeNotFound, fmt.Sprintf("job %s not found", id), nil)
	}

	if !job.CanResume() {
		return internal.NewError(internal.ErrorCodeValidation, fmt.Sprintf("job %s cannot be resumed", id), nil)
	}

	job.Status = JobStatusPending

	// Update stored job
	if c.store != nil {
		c.store.SaveJob(job)
	}

	// Restart processing
	go func() {
		jobCtx, jobCancel := context.WithTimeout(c.ctx, job.Config.Timeout)
		defer jobCancel()

		if err := c.processor.Process(jobCtx, job); err != nil {
			c.mutex.Lock()
			job.Status = JobStatusFailed
			job.Error = err
			now := time.Now()
			job.CompletedAt = &now
			c.mutex.Unlock()

			if c.store != nil {
				c.store.SaveJob(job)
			}
		}
	}()

	return nil
}

// ListJobs returns all jobs managed by the coordinator
func (c *DefaultCoordinator) ListJobs() ([]*Job, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	jobs := make([]*Job, 0, len(c.jobs))
	for _, job := range c.jobs {
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// CleanupJob removes a completed job from memory and optionally from storage
func (c *DefaultCoordinator) CleanupJob(id string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	job, exists := c.jobs[id]
	if !exists {
		return internal.NewError(internal.ErrorCodeNotFound, fmt.Sprintf("job %s not found", id), nil)
	}

	if !job.IsComplete() {
		return internal.NewError(internal.ErrorCodeValidation, fmt.Sprintf("job %s is not complete", id), nil)
	}

	// Remove from memory
	delete(c.jobs, id)

	// Remove from storage
	if c.store != nil {
		if err := c.store.DeleteJob(id); err != nil {
			return internal.NewError(internal.ErrorCodeProcessing, "failed to delete job from storage", err)
		}
	}

	return nil
}

// Shutdown gracefully shuts down the coordinator
func (c *DefaultCoordinator) Shutdown() error {
	c.cancel()

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Cancel all running jobs
	for _, job := range c.jobs {
		if job.IsRunning() {
			job.Status = JobStatusCanceled
			now := time.Now()
			job.CompletedAt = &now

			if c.store != nil {
				c.store.SaveJob(job)
			}
		}
	}

	return nil
}

// validateJob validates job configuration and requirements
func (c *DefaultCoordinator) validateJob(job *Job) error {
	if job.Config == nil {
		return fmt.Errorf("job configuration is required")
	}

	if len(job.TileRanges) == 0 {
		return fmt.Errorf("at least one tile range is required")
	}

	if job.Config.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be positive")
	}

	if job.Config.ChunkSize <= 0 {
		return fmt.Errorf("chunk size must be positive")
	}

	if job.Config.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	// Validate tile ranges
	for i, tileRange := range job.TileRanges {
		if err := c.validateTileRange(tileRange); err != nil {
			return fmt.Errorf("tile range %d is invalid: %w", i, err)
		}
	}

	return nil
}

// validateTileRange validates a single tile range
func (c *DefaultCoordinator) validateTileRange(tileRange *tile.TileRange) error {
	if tileRange.MinZ < 0 || tileRange.MaxZ > 22 {
		return fmt.Errorf("zoom levels must be between 0 and 22")
	}

	if tileRange.MinZ > tileRange.MaxZ {
		return fmt.Errorf("min zoom (%d) cannot be greater than max zoom (%d)", tileRange.MinZ, tileRange.MaxZ)
	}

	if tileRange.MinX > tileRange.MaxX {
		return fmt.Errorf("min X (%d) cannot be greater than max X (%d)", tileRange.MinX, tileRange.MaxX)
	}

	if tileRange.MinY > tileRange.MaxY {
		return fmt.Errorf("min Y (%d) cannot be greater than max Y (%d)", tileRange.MinY, tileRange.MaxY)
	}

	// Validate that coordinates are within bounds for each zoom level
	for z := tileRange.MinZ; z <= tileRange.MaxZ; z++ {
		maxTile := 1 << uint(z)
		if tileRange.MinX < 0 || tileRange.MaxX >= maxTile {
			return fmt.Errorf("X coordinates for zoom %d must be between 0 and %d", z, maxTile-1)
		}
		if tileRange.MinY < 0 || tileRange.MaxY >= maxTile {
			return fmt.Errorf("Y coordinates for zoom %d must be between 0 and %d", z, maxTile-1)
		}
	}

	return nil
}

// GetJobStatistics returns statistics about all jobs
func (c *DefaultCoordinator) GetJobStatistics() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	stats := map[string]interface{}{
		"total_jobs": len(c.jobs),
		"pending":    0,
		"running":    0,
		"completed":  0,
		"failed":     0,
		"canceled":   0,
		"paused":     0,
	}

	for _, job := range c.jobs {
		switch job.Status {
		case JobStatusPending:
			stats["pending"] = stats["pending"].(int) + 1
		case JobStatusRunning:
			stats["running"] = stats["running"].(int) + 1
		case JobStatusCompleted:
			stats["completed"] = stats["completed"].(int) + 1
		case JobStatusFailed:
			stats["failed"] = stats["failed"].(int) + 1
		case JobStatusCanceled:
			stats["canceled"] = stats["canceled"].(int) + 1
		case JobStatusPaused:
			stats["paused"] = stats["paused"].(int) + 1
		}
	}

	return stats
}
