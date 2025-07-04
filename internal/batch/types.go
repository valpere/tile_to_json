// internal/batch/types.go - Batch processing types
package batch

import (
	"context"
	"time"

	"github.com/valpere/tile_to_json/internal/tile"
)

// Job represents a batch processing job
type Job struct {
	ID          string             `json:"id"`
	TileRanges  []*tile.TileRange  `json:"tile_ranges"`
	Config      *JobConfig         `json:"config"`
	Status      JobStatus          `json:"status"`
	Progress    *JobProgress       `json:"progress"`
	CreatedAt   time.Time          `json:"created_at"`
	StartedAt   *time.Time         `json:"started_at,omitempty"`
	CompletedAt *time.Time         `json:"completed_at,omitempty"`
	Error       error              `json:"error,omitempty"`
}

// JobConfig contains configuration for a batch processing job
type JobConfig struct {
	Concurrency    int           `json:"concurrency"`
	ChunkSize      int           `json:"chunk_size"`
	Timeout        time.Duration `json:"timeout"`
	Resume         bool          `json:"resume"`
	OutputPath     string        `json:"output_path"`
	OutputFormat   string        `json:"output_format"`
	FailOnError    bool          `json:"fail_on_error"`
	RetryFailed    bool          `json:"retry_failed"`
	MaxRetries     int           `json:"max_retries"`
	MultiFile      bool          `json:"multi_file"`
	Compression    bool          `json:"compression"`
}

// JobStatus represents the current status of a batch job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCanceled  JobStatus = "canceled"
	JobStatusPaused    JobStatus = "paused"
)

// JobProgress tracks the progress of a batch processing job
type JobProgress struct {
	TotalTiles     int64         `json:"total_tiles"`
	ProcessedTiles int64         `json:"processed_tiles"`
	FailedTiles    int64         `json:"failed_tiles"`
	SuccessTiles   int64         `json:"success_tiles"`
	CurrentChunk   int           `json:"current_chunk"`
	TotalChunks    int           `json:"total_chunks"`
	StartTime      time.Time     `json:"start_time"`
	EstimatedEnd   *time.Time    `json:"estimated_end,omitempty"`
	Throughput     float64       `json:"throughput"`
	BytesWritten   int64         `json:"bytes_written"`
}

// WorkItem represents a single unit of work in a batch job
type WorkItem struct {
	Request   *tile.TileRequest `json:"request"`
	ChunkID   int               `json:"chunk_id"`
	ItemID    int               `json:"item_id"`
	Retry     int               `json:"retry"`
	Priority  int               `json:"priority"`
}

// WorkResult represents the result of processing a work item
type WorkResult struct {
	Item      *WorkItem           `json:"item"`
	Tile      *tile.ProcessedTile `json:"tile,omitempty"`
	Error     error               `json:"error,omitempty"`
	Duration  time.Duration       `json:"duration"`
	Attempts  int                 `json:"attempts"`
}

// ChunkResult represents the result of processing a chunk of work items
type ChunkResult struct {
	ChunkID      int            `json:"chunk_id"`
	Results      []*WorkResult  `json:"results"`
	Duration     time.Duration  `json:"duration"`
	SuccessCount int            `json:"success_count"`
	FailureCount int            `json:"failure_count"`
}

// Coordinator defines the interface for managing batch jobs
type Coordinator interface {
	SubmitJob(job *Job) error
	GetJob(id string) (*Job, error)
	CancelJob(id string) error
	PauseJob(id string) error
	ResumeJob(id string) error
	ListJobs() ([]*Job, error)
	CleanupJob(id string) error
}

// Processor defines the interface for executing batch processing jobs
type Processor interface {
	Process(ctx context.Context, job *Job) error
	ProcessChunk(ctx context.Context, workItems []*WorkItem) (*ChunkResult, error)
}

// ProgressReporter defines the interface for reporting job progress
type ProgressReporter interface {
	ReportProgress(job *Job) error
	ReportChunkComplete(job *Job, chunk *ChunkResult) error
	ReportJobComplete(job *Job) error
	ReportJobFailed(job *Job, err error) error
}

// JobStore defines the interface for persisting job state
type JobStore interface {
	SaveJob(job *Job) error
	LoadJob(id string) (*Job, error)
	DeleteJob(id string) error
	ListJobs() ([]*Job, error)
	SaveProgress(jobID string, progress *JobProgress) error
}

// NewJob creates a new batch processing job
func NewJob(id string, ranges []*tile.TileRange, config *JobConfig) *Job {
	return &Job{
		ID:         id,
		TileRanges: ranges,
		Config:     config,
		Status:     JobStatusPending,
		Progress:   NewJobProgress(),
		CreatedAt:  time.Now(),
	}
}

// NewJobConfig creates a new job configuration with default values
func NewJobConfig() *JobConfig {
	return &JobConfig{
		Concurrency:  10,
		ChunkSize:    100,
		Timeout:      5 * time.Minute,
		Resume:       false,
		FailOnError:  false,
		RetryFailed:  true,
		MaxRetries:   3,
		MultiFile:    false,
		Compression:  false,
		OutputFormat: "geojson",
	}
}

// NewJobProgress creates a new job progress tracker
func NewJobProgress() *JobProgress {
	return &JobProgress{
		StartTime:      time.Now(),
		TotalTiles:     0,
		ProcessedTiles: 0,
		FailedTiles:    0,
		SuccessTiles:   0,
		CurrentChunk:   0,
		TotalChunks:    0,
		Throughput:     0,
		BytesWritten:   0,
	}
}

// NewWorkItem creates a new work item
func NewWorkItem(request *tile.TileRequest, chunkID, itemID int) *WorkItem {
	return &WorkItem{
		Request:  request,
		ChunkID:  chunkID,
		ItemID:   itemID,
		Retry:    0,
		Priority: 0,
	}
}

// IsComplete returns true if the job has finished (successfully or with error)
func (j *Job) IsComplete() bool {
	return j.Status == JobStatusCompleted || j.Status == JobStatusFailed || j.Status == JobStatusCanceled
}

// IsRunning returns true if the job is currently being processed
func (j *Job) IsRunning() bool {
	return j.Status == JobStatusRunning
}

// CanResume returns true if the job can be resumed
func (j *Job) CanResume() bool {
	return j.Status == JobStatusPaused || (j.Status == JobStatusFailed && j.Config.Resume)
}

// EstimateCompletion estimates when the job will complete based on current progress
func (p *JobProgress) EstimateCompletion() time.Time {
	if p.Throughput == 0 || p.ProcessedTiles == 0 {
		return time.Now().Add(time.Hour) // Default to 1 hour if no data
	}

	remaining := p.TotalTiles - p.ProcessedTiles
	if remaining <= 0 {
		return time.Now()
	}

	secondsRemaining := float64(remaining) / p.Throughput
	return time.Now().Add(time.Duration(secondsRemaining) * time.Second)
}

// CalculateProgress calculates the completion percentage
func (p *JobProgress) CalculateProgress() float64 {
	if p.TotalTiles == 0 {
		return 0
	}
	return float64(p.ProcessedTiles) / float64(p.TotalTiles) * 100
}

// UpdateThroughput updates the processing throughput based on elapsed time
func (p *JobProgress) UpdateThroughput() {
	elapsed := time.Since(p.StartTime)
	if elapsed.Seconds() > 0 && p.ProcessedTiles > 0 {
		p.Throughput = float64(p.ProcessedTiles) / elapsed.Seconds()
	}
}

// String returns a string representation of the job status
func (s JobStatus) String() string {
	return string(s)
}

// IsValid checks if the job status is valid
func (s JobStatus) IsValid() bool {
	switch s {
	case JobStatusPending, JobStatusRunning, JobStatusCompleted, JobStatusFailed, JobStatusCanceled, JobStatusPaused:
		return true
	default:
		return false
	}
}
