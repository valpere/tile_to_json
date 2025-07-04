// internal/batch/processor.go - Batch processing implementation
package batch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/valpere/tile_to_json/internal/output"
	"github.com/valpere/tile_to_json/internal/tile"
)

// BatchProcessor implements the Processor interface for batch processing operations
type BatchProcessor struct {
	fetcher   tile.Fetcher
	processor tile.Processor
	writer    output.Writer
	reporter  ProgressReporter
	mutex     sync.RWMutex
}

// NewBatchProcessor creates a new batch processor with the specified components
func NewBatchProcessor(fetcher tile.Fetcher, processor tile.Processor, writer output.Writer, reporter ProgressReporter) *BatchProcessor {
	return &BatchProcessor{
		fetcher:   fetcher,
		processor: processor,
		writer:    writer,
		reporter:  reporter,
	}
}

// Process executes a complete batch processing job
func (bp *BatchProcessor) Process(ctx context.Context, job *Job) error {
	// Update job status to running
	bp.mutex.Lock()
	job.Status = JobStatusRunning
	now := time.Now()
	job.StartedAt = &now
	job.Progress.StartTime = now
	bp.mutex.Unlock()

	// Report job started
	if bp.reporter != nil {
		bp.reporter.ReportProgress(job)
	}

	// Generate work items from tile ranges
	workItems, err := bp.generateWorkItems(job.TileRanges, job.Config)
	if err != nil {
		bp.completeJobWithError(job, fmt.Errorf("failed to generate work items: %w", err))
		return err
	}

	// Update total tiles count
	bp.mutex.Lock()
	job.Progress.TotalTiles = int64(len(workItems))
	job.Progress.TotalChunks = (len(workItems) + job.Config.ChunkSize - 1) / job.Config.ChunkSize
	bp.mutex.Unlock()

	// Process work items in chunks
	chunkResults := make([]*ChunkResult, 0, job.Progress.TotalChunks)
	
	for chunkStart := 0; chunkStart < len(workItems); chunkStart += job.Config.ChunkSize {
		select {
		case <-ctx.Done():
			bp.completeJobWithError(job, ctx.Err())
			return ctx.Err()
		default:
		}

		// Prepare chunk
		chunkEnd := chunkStart + job.Config.ChunkSize
		if chunkEnd > len(workItems) {
			chunkEnd = len(workItems)
		}

		chunk := workItems[chunkStart:chunkEnd]
		chunkID := len(chunkResults)

		// Update current chunk
		bp.mutex.Lock()
		job.Progress.CurrentChunk = chunkID + 1
		bp.mutex.Unlock()

		// Process chunk
		chunkResult, err := bp.ProcessChunk(ctx, chunk)
		if err != nil {
			if job.Config.FailOnError {
				bp.completeJobWithError(job, fmt.Errorf("chunk %d failed: %w", chunkID, err))
				return err
			}
			// Continue with next chunk on error if not failing fast
		}

		chunkResults = append(chunkResults, chunkResult)

		// Update progress
		bp.updateJobProgress(job, chunkResult)

		// Report chunk completion
		if bp.reporter != nil {
			bp.reporter.ReportChunkComplete(job, chunkResult)
		}
	}

	// Complete job successfully
	bp.completeJobSuccessfully(job)
	
	if bp.reporter != nil {
		bp.reporter.ReportJobComplete(job)
	}

	return nil
}

// ProcessChunk processes a chunk of work items concurrently
func (bp *BatchProcessor) ProcessChunk(ctx context.Context, workItems []*WorkItem) (*ChunkResult, error) {
	start := time.Now()
	
	workChan := make(chan *WorkItem, len(workItems))
	resultChan := make(chan *WorkResult, len(workItems))

	// Send work items to channel
	for _, item := range workItems {
		workChan <- item
	}
	close(workChan)

	// Start worker goroutines
	var wg sync.WaitGroup
	concurrency := min(len(workItems), 10) // Limit concurrency for chunk processing
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bp.worker(ctx, workChan, resultChan)
		}()
	}

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var results []*WorkResult
	var processedTiles []*tile.ProcessedTile
	successCount := 0
	failureCount := 0

	for result := range resultChan {
		results = append(results, result)
		
		if result.Error != nil {
			failureCount++
		} else {
			successCount++
			if result.Tile != nil {
				processedTiles = append(processedTiles, result.Tile)
			}
		}
	}

	// Write processed tiles
	if len(processedTiles) > 0 {
		if err := bp.writer.WriteBatch(processedTiles); err != nil {
			return &ChunkResult{
				ChunkID:      workItems[0].ChunkID,
				Results:      results,
				Duration:     time.Since(start),
				SuccessCount: successCount,
				FailureCount: failureCount,
			}, fmt.Errorf("failed to write batch: %w", err)
		}
	}

	return &ChunkResult{
		ChunkID:      workItems[0].ChunkID,
		Results:      results,
		Duration:     time.Since(start),
		SuccessCount: successCount,
		FailureCount: failureCount,
	}, nil
}

// worker processes individual work items
func (bp *BatchProcessor) worker(ctx context.Context, workChan <-chan *WorkItem, resultChan chan<- *WorkResult) {
	for workItem := range workChan {
		select {
		case <-ctx.Done():
			resultChan <- &WorkResult{
				Item:     workItem,
				Error:    ctx.Err(),
				Duration: 0,
				Attempts: 1,
			}
			return
		default:
		}

		result := bp.processWorkItem(ctx, workItem)
		resultChan <- result
	}
}

// processWorkItem processes a single work item with retry logic
func (bp *BatchProcessor) processWorkItem(ctx context.Context, workItem *WorkItem) *WorkResult {
	start := time.Now()
	var lastErr error

	for attempt := 0; attempt <= 3; attempt++ { // Max 3 retry attempts
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		// Fetch tile
		response, err := bp.fetcher.Fetch(workItem.Request)
		if err != nil {
			lastErr = fmt.Errorf("fetch failed: %w", err)
			continue
		}

		// Process tile
		processedTile, err := bp.processor.Process(response)
		if err != nil {
			lastErr = fmt.Errorf("process failed: %w", err)
			continue
		}

		// Success
		return &WorkResult{
			Item:     workItem,
			Tile:     processedTile,
			Duration: time.Since(start),
			Attempts: attempt + 1,
		}
	}

	// All attempts failed
	return &WorkResult{
		Item:     workItem,
		Error:    lastErr,
		Duration: time.Since(start),
		Attempts: 4,
	}
}

// generateWorkItems creates work items from tile ranges
func (bp *BatchProcessor) generateWorkItems(tileRanges []*tile.TileRange, config *JobConfig) ([]*WorkItem, error) {
	var workItems []*WorkItem
	itemID := 0

	for _, tileRange := range tileRanges {
		for z := tileRange.MinZ; z <= tileRange.MaxZ; z++ {
			for x := tileRange.MinX; x <= tileRange.MaxX; x++ {
				for y := tileRange.MinY; y <= tileRange.MaxY; y++ {
					// Validate coordinates
					if err := tile.ValidateCoordinates(z, x, y); err != nil {
						return nil, fmt.Errorf("invalid tile coordinates %d/%d/%d: %w", z, x, y, err)
					}

					request := &tile.TileRequest{
						Z:   z,
						X:   x,
						Y:   y,
						URL: bp.buildTileURL(z, x, y),
					}

					workItem := NewWorkItem(request, 0, itemID)
					workItems = append(workItems, workItem)
					itemID++
				}
			}
		}
	}

	return workItems, nil
}

// buildTileURL constructs a tile URL from coordinates
func (bp *BatchProcessor) buildTileURL(z, x, y int) string {
	// This should come from configuration
	return fmt.Sprintf("https://example.com/tiles/%d/%d/%d.mvt", z, x, y)
}

// updateJobProgress updates job progress based on chunk results
func (bp *BatchProcessor) updateJobProgress(job *Job, chunkResult *ChunkResult) {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()

	job.Progress.ProcessedTiles += int64(len(chunkResult.Results))
	job.Progress.SuccessTiles += int64(chunkResult.SuccessCount)
	job.Progress.FailedTiles += int64(chunkResult.FailureCount)
	job.Progress.UpdateThroughput()
	
	estimatedEnd := job.Progress.EstimateCompletion()
	job.Progress.EstimatedEnd = &estimatedEnd
}

// completeJobSuccessfully marks the job as completed
func (bp *BatchProcessor) completeJobSuccessfully(job *Job) {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()

	job.Status = JobStatusCompleted
	now := time.Now()
	job.CompletedAt = &now
}

// completeJobWithError marks the job as failed
func (bp *BatchProcessor) completeJobWithError(job *Job, err error) {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()

	job.Status = JobStatusFailed
	job.Error = err
	now := time.Now()
	job.CompletedAt = &now

	if bp.reporter != nil {
		bp.reporter.ReportJobFailed(job, err)
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
