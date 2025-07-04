// cmd/convert.go - Single tile conversion command
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"mvtpbf-to-geojson/internal/config"
	"mvtpbf-to-geojson/internal/output"
	"mvtpbf-to-geojson/internal/tile"
)

// convertCmd represents the convert command
var convertCmd = &cobra.Command{
	Use:   "convert",
	Short: "Convert a single Mapbox Vector Tile to JSON format",
	Long: `Convert a single Mapbox Vector Tile from Protocol Buffer format to JSON/GeoJSON format.

This command fetches a single tile from the specified URL or constructs the URL using
the provided coordinates and base URL, then converts it to the specified output format.

Examples:
  # Convert using direct URL
  tile-to-json convert --url "https://example.com/tiles/14/8362/5956.mvt" --output tile.geojson

  # Convert using coordinates and base URL
  tile-to-json convert --base-url "https://example.com/tiles" --z 14 --x 8362 --y 5956 --output tile.geojson

  # Convert to stdout with pretty formatting
  tile-to-json convert --url "https://example.com/tiles/14/8362/5956.mvt" --pretty

  # Convert with custom format and compression
  tile-to-json convert --url "https://example.com/tiles/14/8362/5956.mvt" --format json --output tile.json.gz`,
	RunE: runConvert,
}

func init() {
	rootCmd.AddCommand(convertCmd)

	// Tile source flags
	convertCmd.Flags().String("url", "", "direct URL to the tile")
	convertCmd.Flags().Int("z", 0, "tile zoom level")
	convertCmd.Flags().Int("x", 0, "tile x coordinate")
	convertCmd.Flags().Int("y", 0, "tile y coordinate")

	// Output flags
	convertCmd.Flags().StringP("output", "o", "", "output file path (default: stdout)")
	convertCmd.Flags().Bool("compression", false, "compress output file")
	convertCmd.Flags().Bool("metadata", false, "include tile metadata in output")

	// Mark required flags
	convertCmd.MarkFlagsRequiredTogether("z", "x", "y")
	convertCmd.MarkFlagsMutuallyExclusive("url", "z")
}

func runConvert(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Get command flags
	url, _ := cmd.Flags().GetString("url")
	z, _ := cmd.Flags().GetInt("z")
	x, _ := cmd.Flags().GetInt("x")
	y, _ := cmd.Flags().GetInt("y")
	outputPath, _ := cmd.Flags().GetString("output")
	compression, _ := cmd.Flags().GetBool("compression")
	metadata, _ := cmd.Flags().GetBool("metadata")

	// Validate input parameters
	if url == "" && (z == 0 && x == 0 && y == 0) {
		return fmt.Errorf("either --url or --z/--x/--y coordinates must be specified")
	}

	// Build tile request
	var tileRequest *tile.TileRequest
	if url != "" {
		tileRequest = &tile.TileRequest{
			URL: url,
			// Extract coordinates from URL if possible
			Z: z, X: x, Y: y,
		}
	} else {
		// Validate coordinates
		if err := tile.ValidateCoordinates(z, x, y); err != nil {
			return fmt.Errorf("invalid tile coordinates: %w", err)
		}

		if cfg.Server.BaseURL == "" {
			return fmt.Errorf("base URL is required when using coordinates")
		}

		tileRequest = tile.NewTileRequest(z, x, y, cfg.Server.BaseURL)
	}

	// Create fetcher and processor
	fetcher := tile.NewHTTPFetcher(cfg)
	processor := tile.NewMVTProcessor()

	// Fetch the tile
	if viper.GetBool("logging.verbose") {
		fmt.Fprintf(os.Stderr, "Fetching tile from: %s\n", tileRequest.URL)
	}

	response, err := fetcher.FetchWithRetry(tileRequest)
	if err != nil {
		return fmt.Errorf("failed to fetch tile: %w", err)
	}

	// Process the tile
	if viper.GetBool("logging.verbose") {
		fmt.Fprintf(os.Stderr, "Processing tile data (%d bytes)\n", len(response.Data))
	}

	processedTile, err := processor.Process(response)
	if err != nil {
		return fmt.Errorf("failed to process tile: %w", err)
	}

	// Create writer configuration
	writerConfig := &output.WriterConfig{
		Format:      output.Format(cfg.Output.Format),
		Pretty:      cfg.Output.Pretty,
		Compression: compression,
		Metadata:    metadata,
	}

	// Create writer
	var writer output.Writer
	if outputPath == "" || outputPath == "-" {
		writer, err = output.NewStdoutWriter(writerConfig.Format, writerConfig.Pretty)
	} else {
		// Ensure output directory exists
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		writer, err = output.NewFileWriter(writerConfig, outputPath)
	}

	if err != nil {
		return fmt.Errorf("failed to create writer: %w", err)
	}
	defer writer.Close()

	// Write the processed tile
	if err := writer.Write(processedTile); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	// Success message
	if viper.GetBool("logging.verbose") {
		if outputPath == "" || outputPath == "-" {
			fmt.Fprintf(os.Stderr, "Tile converted successfully to stdout\n")
		} else {
			fmt.Fprintf(os.Stderr, "Tile converted successfully to: %s\n", outputPath)
		}

		if processedTile.Metadata != nil {
			fmt.Fprintf(os.Stderr, "Features: %d, Layers: %v, Size: %d bytes\n",
				processedTile.Metadata.FeatureCount,
				processedTile.Metadata.Layers,
				processedTile.Metadata.Size)
		}
	}

	return nil
}
