// cmd/batch.go - Batch processing command
package cmd

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/valpere/tile_to_json/internal"
	"github.com/valpere/tile_to_json/internal/batch"
	"github.com/valpere/tile_to_json/internal/config"
	"github.com/valpere/tile_to_json/internal/output"
	"github.com/valpere/tile_to_json/internal/tile"
)

// batchCmd represents the batch command
var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Batch process multiple Mapbox Vector Tiles",
	Long: `Batch process multiple Mapbox Vector Tiles from Protocol Buffer format to JSON/GeoJSON format.

This command supports processing tile ranges from both remote servers and local file systems.
It provides concurrent processing capabilities and comprehensive progress reporting for 
large-scale conversion operations.

Data Sources:
- Remote tile servers via HTTP/HTTPS with URL patterns
- Local tile directories with standard z/x/y organization
- Mixed mode with automatic source detection

Examples:
  # Process remote tiles in a zoom range with bounding box
  tile-to-json batch --base-url "https://example.com/tiles" --min-zoom 10 --max-zoom 12 --bbox "-74.0,40.7,-73.9,40.8" --output-dir ./tiles/

  # Process local tiles in a directory
  tile-to-json batch --base-path "/path/to/tiles" --min-zoom 10 --max-zoom 12 --bbox "-74.0,40.7,-73.9,40.8" --output-dir ./output/

  # Process specific zoom level with full extent (remote)
  tile-to-json batch --base-url "https://example.com/tiles" --zoom 14 --output-dir ./tiles/

  # Process local tiles with custom concurrency and chunk size
  tile-to-json batch --base-path "/path/to/tiles" --min-zoom 10 --max-zoom 11 --bbox "-74.0,40.7,-73.9,40.8" --concurrency 20 --chunk-size 50 --output-dir ./output/

  # Process to single file with compression
  tile-to-json batch --base-path "/path/to/tiles" --zoom 10 --bbox "-74.0,40.7,-73.9,40.8" --output tiles.geojson.gz --single-file

  # Resume failed batch job
  tile-to-json batch --resume --job-id previous-job-id`,
	RunE: runBatch,
}

func init() {
	rootCmd.AddCommand(batchCmd)

	// Tile range flags
	batchCmd.Flags().Int("zoom", 0, "single zoom level to process")
	batchCmd.Flags().Int("min-zoom", 0, "minimum zoom level")
	batchCmd.Flags().Int("max-zoom", 0, "maximum zoom level")
	batchCmd.Flags().String("bbox", "", "bounding box: 'min_lon,min_lat,max_lon,max_lat'")
	batchCmd.Flags().String("tiles", "", "specific tiles list: 'z/x/y,z/x/y,...'")

	// Source override flags
	batchCmd.Flags().String("source-type", "", "override source type (http, local)")

	// Output flags
	batchCmd.Flags().String("output-dir", "./output", "output directory for tiles")
	batchCmd.Flags().StringP("output", "o", "", "single output file (use with --single-file)")
	batchCmd.Flags().Bool("single-file", false, "combine all tiles into single file")
	batchCmd.Flags().Bool("multi-file", true, "output each tile to separate file")

	// Processing flags
	batchCmd.Flags().Int("chunk-size", 100, "number of tiles per processing chunk")
	batchCmd.Flags().Bool("fail-on-error", false, "stop processing on first error")
	batchCmd.Flags().Bool("resume", false, "resume previous batch job")
	batchCmd.Flags().String("job-id", "", "job ID for resume operation")

	// Progress flags
	batchCmd.Flags().Bool("progress", true, "show progress indicator")
	batchCmd.Flags().Duration("progress-interval", 5*time.Second, "progress update interval")

	// Mark mutually exclusive flags
	batchCmd.MarkFlagsMutuallyExclusive("zoom", "min-zoom")
	batchCmd.MarkFlagsMutuallyExclusive("zoom", "max-zoom")
	batchCmd.MarkFlagsMutuallyExclusive("single-file", "multi-file")
	batchCmd.MarkFlagsMutuallyExclusive("output-dir", "output")
}

func runBatch(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Get command flags
	zoom, _ := cmd.Flags().GetInt("zoom")
	minZoom, _ := cmd.Flags().GetInt("min-zoom")
	maxZoom, _ := cmd.Flags().GetInt("max-zoom")
	bboxStr, _ := cmd.Flags().GetString("bbox")
	tilesStr, _ := cmd.Flags().GetString("tiles")
	sourceTypeOverride, _ := cmd.Flags().GetString("source-type")
	outputDir, _ := cmd.Flags().GetString("output-dir")
	outputFile, _ := cmd.Flags().GetString("output")
	singleFile, _ := cmd.Flags().GetBool("single-file")
	multiFile, _ := cmd.Flags().GetBool("multi-file")
	chunkSize, _ := cmd.Flags().GetInt("chunk-size")
	failOnError, _ := cmd.Flags().GetBool("fail-on-error")
	resume, _ := cmd.Flags().GetBool("resume")
	jobID, _ := cmd.Flags().GetString("job-id")
	showProgress, _ := cmd.Flags().GetBool("progress")

	// Override source type if specified
	if sourceTypeOverride != "" {
		switch sourceTypeOverride {
		case "http":
			cfg.Source.Type = "http"
		case "local":
			cfg.Source.Type = "local"
		default:
			return fmt.Errorf("invalid source type: %s (must be 'http' or 'local')", sourceTypeOverride)
		}
	}

	// Determine and validate source type
	sourceType := cfg.DetermineSourceType()
	factory := tile.NewFetcherFactory(cfg)
	
	if err := factory.ValidateConfiguration(sourceType); err != nil {
		return fmt.Errorf("source configuration validation failed: %w", err)
	}

	// Handle resume functionality
	if resume && jobID != "" {
		// TODO: Implement job resume logic
		return fmt.Errorf("resume functionality not yet implemented")
	}

	// Parse tile ranges
	var tileRanges []*tile.TileRange

	if tilesStr != "" {
		// Parse specific tiles list
		tileRanges, err = parseTilesList(tilesStr)
		if err != nil {
			return fmt.Errorf("failed to parse tiles list: %w", err)
		}
	} else {
		// Parse zoom levels and bounding box
		if zoom > 0 {
			minZoom = zoom
			maxZoom = zoom
		}

		if minZoom == 0 && maxZoom == 0 {
			return fmt.Errorf("zoom level(s) must be specified")
		}

		if maxZoom == 0 {
			maxZoom = minZoom
		}

		// Parse bounding box
		var bbox *BoundingBox
		if bboxStr != "" {
			bbox, err = parseBoundingBox(bboxStr)
			if err != nil {
				return fmt.Errorf("failed to parse bounding box: %w", err)
			}
		}

		// Generate tile ranges
		tileRanges, err = generateTileRanges(minZoom, maxZoom, bbox)
		if err != nil {
			return fmt.Errorf("failed to generate tile ranges: %w", err)
		}
	}

	if len(tileRanges) == 0 {
		return fmt.Errorf("no tiles to process")
	}

	// For local sources, validate that tiles exist
	if sourceType == internal.SourceTypeLocal {
		if err := validateLocalTileRanges(cfg, tileRanges); err != nil {
			return fmt.Errorf("local tile validation failed: %w", err)
		}
	}

	// Calculate total tiles
	var totalTiles int64
	for _, tr := range tileRanges {
		totalTiles += tr.Count()
	}

	if viper.GetBool("logging.verbose") {
		fmt.Fprintf(os.Stderr, "Processing %d tiles across %d ranges\n", totalTiles, len(tileRanges))
		fmt.Fprintf(os.Stderr, "Source type: %s\n", sourceType)
	}

	// Create job configuration
	jobConfig := &batch.JobConfig{
		Concurrency:  cfg.Batch.Concurrency,
		ChunkSize:    chunkSize,
		Timeout:      cfg.Batch.Timeout,
		FailOnError:  failOnError,
		MultiFile:    multiFile,
		Compression:  cfg.Output.Compression,
		OutputFormat: cfg.Output.Format,
	}

	// Determine output path
	if singleFile {
		if outputFile == "" {
			return fmt.Errorf("output file must be specified when using --single-file")
		}
		jobConfig.OutputPath = outputFile
		jobConfig.MultiFile = false
	} else {
		jobConfig.OutputPath = outputDir
		jobConfig.MultiFile = true
	}

	// Create batch components
	fetcher, err := factory.CreateFetcherForType(sourceType)
	if err != nil {
		return fmt.Errorf("failed to create fetcher: %w", err)
	}

	processor := tile.NewMVTProcessor()

	// Create writer
	writerConfig := &output.WriterConfig{
		Format:      output.Format(cfg.Output.Format),
		Pretty:      cfg.Output.Pretty,
		Compression: cfg.Output.Compression,
		Metadata:    true,
	}

	var writer output.Writer
	if singleFile {
		writer, err = output.NewFileWriter(writerConfig, outputFile)
	} else {
		writer, err = output.NewMultiFileWriter(writerConfig, outputDir)
	}

	if err != nil {
		return fmt.Errorf("failed to create writer: %w", err)
	}
	defer writer.Close()

	// Create progress reporter
	var reporter batch.ProgressReporter
	if showProgress {
		reporter = NewConsoleProgressReporter()
	}

	// Create batch processor
	batchProcessor := batch.NewBatchProcessor(fetcher, processor, writer, reporter)

	// Create and submit job
	job := batch.NewJob(generateJobID(), tileRanges, jobConfig)

	// Process the job
	ctx, cancel := context.WithTimeout(context.Background(), jobConfig.Timeout)
	defer cancel()

	if viper.GetBool("logging.verbose") {
		fmt.Fprintf(os.Stderr, "Starting batch processing job: %s\n", job.ID)
	}

	if err := batchProcessor.Process(ctx, job); err != nil {
		return fmt.Errorf("batch processing failed: %w", err)
	}

	// Print completion summary
	if viper.GetBool("logging.verbose") || showProgress {
		elapsed := time.Since(job.Progress.StartTime)
		fmt.Fprintf(os.Stderr, "\nBatch processing completed successfully!\n")
		fmt.Fprintf(os.Stderr, "Processed: %d tiles\n", job.Progress.ProcessedTiles)
		fmt.Fprintf(os.Stderr, "Success: %d, Failed: %d\n", job.Progress.SuccessTiles, job.Progress.FailedTiles)
		fmt.Fprintf(os.Stderr, "Duration: %v\n", elapsed)
		fmt.Fprintf(os.Stderr, "Throughput: %.2f tiles/second\n", job.Progress.Throughput)
		fmt.Fprintf(os.Stderr, "Source: %s\n", sourceType)
	}

	return nil
}

// validateLocalTileRanges validates that local tiles exist for the specified ranges
func validateLocalTileRanges(cfg *config.Config, tileRanges []*tile.TileRange) error {
	if cfg.DetermineSourceType() != internal.SourceTypeLocal {
		return nil // Skip validation for non-local sources
	}

	localFetcher := tile.NewLocalFetcher(cfg)
	sampleCount := 0
	maxSamples := 10 // Validate a sample of tiles to avoid excessive checking

	for _, tileRange := range tileRanges {
		for z := tileRange.MinZ; z <= tileRange.MaxZ && sampleCount < maxSamples; z++ {
			for x := tileRange.MinX; x <= tileRange.MaxX && sampleCount < maxSamples; x++ {
				for y := tileRange.MinY; y <= tileRange.MaxY && sampleCount < maxSamples; y++ {
					if err := localFetcher.ValidateTileExists(z, x, y); err != nil {
						// If tile doesn't exist, warn but don't fail
						if viper.GetBool("logging.verbose") {
							fmt.Fprintf(os.Stderr, "Warning: tile %d/%d/%d not found locally\n", z, x, y)
						}
					}
					sampleCount++
				}
			}
		}
	}

	return nil
}

// BoundingBox represents a geographic bounding box
type BoundingBox struct {
	MinLon, MinLat, MaxLon, MaxLat float64
}

// parseBoundingBox parses a bounding box string
func parseBoundingBox(bbox string) (*BoundingBox, error) {
	parts := strings.Split(bbox, ",")
	if len(parts) != 4 {
		return nil, fmt.Errorf("bounding box must have 4 values: min_lon,min_lat,max_lon,max_lat")
	}

	coords := make([]float64, 4)
	for i, part := range parts {
		val, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid coordinate value: %s", part)
		}
		coords[i] = val
	}

	return &BoundingBox{
		MinLon: coords[0],
		MinLat: coords[1],
		MaxLon: coords[2],
		MaxLat: coords[3],
	}, nil
}

// parseTilesList parses a comma-separated list of tile coordinates
func parseTilesList(tiles string) ([]*tile.TileRange, error) {
	parts := strings.Split(tiles, ",")
	var ranges []*tile.TileRange

	for _, part := range parts {
		coords := strings.Split(strings.TrimSpace(part), "/")
		if len(coords) != 3 {
			return nil, fmt.Errorf("invalid tile format: %s (expected z/x/y)", part)
		}

		z, err := strconv.Atoi(coords[0])
		if err != nil {
			return nil, fmt.Errorf("invalid zoom level: %s", coords[0])
		}

		x, err := strconv.Atoi(coords[1])
		if err != nil {
			return nil, fmt.Errorf("invalid x coordinate: %s", coords[1])
		}

		y, err := strconv.Atoi(coords[2])
		if err != nil {
			return nil, fmt.Errorf("invalid y coordinate: %s", coords[2])
		}

		// Create a single-tile range
		ranges = append(ranges, tile.NewTileRange(z, z, x, x, y, y))
	}

	return ranges, nil
}

// generateTileRanges creates tile ranges from zoom levels and optional bounding box
func generateTileRanges(minZoom, maxZoom int, bbox *BoundingBox) ([]*tile.TileRange, error) {
	var ranges []*tile.TileRange

	for z := minZoom; z <= maxZoom; z++ {
		var minX, maxX, minY, maxY int

		if bbox != nil {
			// Calculate tile bounds from bounding box
			minX, minY = deg2tile(bbox.MinLon, bbox.MaxLat, z)
			maxX, maxY = deg2tile(bbox.MaxLon, bbox.MinLat, z)
		} else {
			// Use full extent for zoom level
			maxTile := (1 << uint(z)) - 1
			minX, minY = 0, 0
			maxX, maxY = maxTile, maxTile
		}

		ranges = append(ranges, tile.NewTileRange(z, z, minX, maxX, minY, maxY))
	}

	return ranges, nil
}

// deg2tile converts geographic coordinates to tile coordinates
func deg2tile(lon, lat float64, z int) (int, int) {
	// Implementation of standard web mercator tile calculation
	n := 1 << uint(z)
	x := int((lon + 180.0) / 360.0 * float64(n))
	latRad := lat * math.Pi / 180.0
	y := int((1.0 - math.Asinh(math.Tan(latRad))/math.Pi) / 2.0 * float64(n))
	return x, y
}

// generateJobID creates a unique job ID
func generateJobID() string {
	return fmt.Sprintf("batch-%d", time.Now().Unix())
}

// ConsoleProgressReporter implements progress reporting to console
type ConsoleProgressReporter struct {
	lastUpdate time.Time
}

// NewConsoleProgressReporter creates a new console progress reporter
func NewConsoleProgressReporter() *ConsoleProgressReporter {
	return &ConsoleProgressReporter{}
}

// ReportProgress reports job progress to console
func (r *ConsoleProgressReporter) ReportProgress(job *batch.Job) error {
	if time.Since(r.lastUpdate) < time.Second {
		return nil // Rate limit updates
	}

	progress := job.Progress.CalculateProgress()
	fmt.Fprintf(os.Stderr, "\rProgress: %.1f%% (%d/%d tiles, %.2f tiles/sec)",
		progress, job.Progress.ProcessedTiles, job.Progress.TotalTiles, job.Progress.Throughput)

	r.lastUpdate = time.Now()
	return nil
}

// ReportChunkComplete reports chunk completion
func (r *ConsoleProgressReporter) ReportChunkComplete(job *batch.Job, chunk *batch.ChunkResult) error {
	return r.ReportProgress(job)
}

// ReportJobComplete reports job completion
func (r *ConsoleProgressReporter) ReportJobComplete(job *batch.Job) error {
	fmt.Fprintf(os.Stderr, "\rCompleted: 100%% (%d tiles processed)\n", job.Progress.ProcessedTiles)
	return nil
}

// ReportJobFailed reports job failure
func (r *ConsoleProgressReporter) ReportJobFailed(job *batch.Job, err error) error {
	fmt.Fprintf(os.Stderr, "\rFailed: %s\n", err.Error())
	return nil
}
