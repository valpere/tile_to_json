// cmd/batch.go - Batch processing command
package cmd

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

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

This command supports processing tile ranges defined by zoom levels and bounding boxes,
or individual tile lists. It provides concurrent processing capabilities and comprehensive
progress reporting for large-scale conversion operations.

Examples:
  # Process tiles in a zoom range with bounding box
  tile-to-json batch --min-zoom 10 --max-zoom 12 --bbox "-74.0,40.7,-73.9,40.8" --output-dir ./tiles/

  # Process specific zoom level with full extent
  tile-to-json batch --zoom 14 --output-dir ./tiles/

  # Process with custom concurrency and chunk size
  tile-to-json batch --min-zoom 10 --max-zoom 11 --bbox "-74.0,40.7,-73.9,40.8" --concurrency 20 --chunk-size 50 --output-dir ./tiles/

  # Process to single file with compression
  tile-to-json batch --zoom 10 --bbox "-74.0,40.7,-73.9,40.8" --output tiles.geojson.gz --single-file

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
	outputDir, _ := cmd.Flags().GetString("output-dir")
	outputFile, _ := cmd.Flags().GetString("output")
	singleFile, _ := cmd.Flags().GetBool("single-file")
	multiFile, _ := cmd.Flags().GetBool("multi-file")
	chunkSize, _ := cmd.Flags().GetInt("chunk-size")
	failOnError, _ := cmd.Flags().GetBool("fail-on-error")
	resume, _ := cmd.Flags().GetBool("resume")
	jobID, _ := cmd.Flags().GetString("job-id")
	showProgress, _ := cmd.Flags().GetBool("progress")

	// Validate base URL
	if cfg.Server.BaseURL == "" {
		return fmt.Errorf("base URL is required for batch processing")
	}

	// Parse tile ranges
	var tileRanges []*tile.TileRange

	if resume && jobID != "" {
		// TODO: Implement job resume logic
		return fmt.Errorf("resume functionality not yet implemented")
	}

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

	// Calculate total tiles
	var totalTiles int64
	for _, tr := range tileRanges {
		totalTiles += tr.Count()
	}

	if viper.GetBool("logging.verbose") {
		fmt.Fprintf(os.Stderr, "Processing %d tiles across %d ranges\n", totalTiles, len(tileRanges))
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
	fetcher := tile.NewHTTPFetcher(cfg)
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
