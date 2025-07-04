// cmd/convert.go - Single tile conversion command
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/valpere/tile_to_json/internal"
	"github.com/valpere/tile_to_json/internal/config"
	"github.com/valpere/tile_to_json/internal/output"
	"github.com/valpere/tile_to_json/internal/tile"
)

// convertCmd represents the convert command
var convertCmd = &cobra.Command{
	Use:   "convert",
	Short: "Convert a single Mapbox Vector Tile to JSON format",
	Long: `Convert a single Mapbox Vector Tile from Protocol Buffer format to JSON/GeoJSON format.

This command supports multiple input methods:
- Direct URL to a remote tile server
- Direct file path to a local tile file
- Coordinates with base URL (remote) or base path (local)

The command automatically detects the source type based on the provided parameters
or uses the configured default source type.

Examples:
  # Convert using direct URL (remote)
  tile-to-json convert --url "https://example.com/tiles/14/8362/5956.mvt" --output tile.geojson

  # Convert using direct file path (local)
  tile-to-json convert --file "/path/to/tiles/14/8362/5956.mvt" --output tile.geojson

  # Convert using coordinates and base URL (remote)
  tile-to-json convert --base-url "https://example.com/tiles" --z 14 --x 8362 --y 5956 --output tile.geojson

  # Convert using coordinates and base path (local)
  tile-to-json convert --base-path "/path/to/tiles" --z 14 --x 8362 --y 5956 --output tile.geojson

  # Convert to stdout with pretty formatting
  tile-to-json convert --url "https://example.com/tiles/14/8362/5956.mvt" --pretty

  # Convert with custom format and compression
  tile-to-json convert --file "/path/to/tiles/14/8362/5956.mvt" --format json --output tile.json.gz`,
	RunE: runConvert,
}

func init() {
	rootCmd.AddCommand(convertCmd)

	// Tile source flags
	convertCmd.Flags().String("url", "", "direct URL to the remote tile")
	convertCmd.Flags().String("file", "", "direct path to the local tile file")
	convertCmd.Flags().Int("z", 0, "tile zoom level")
	convertCmd.Flags().Int("x", 0, "tile x coordinate")
	convertCmd.Flags().Int("y", 0, "tile y coordinate")

	// Source override flags
	convertCmd.Flags().String("source-type", "", "override source type (http, local)")

	// Output flags
	convertCmd.Flags().StringP("output", "o", "", "output file path (default: stdout)")
	convertCmd.Flags().Bool("metadata", false, "include tile metadata in output")

	// Mark required flags and mutual exclusions
	convertCmd.MarkFlagsRequiredTogether("z", "x", "y")
	convertCmd.MarkFlagsMutuallyExclusive("url", "file")
	convertCmd.MarkFlagsMutuallyExclusive("url", "z")
	convertCmd.MarkFlagsMutuallyExclusive("file", "z")
}

func runConvert(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Get command flags
	url, _ := cmd.Flags().GetString("url")
	filePath, _ := cmd.Flags().GetString("file")
	z, _ := cmd.Flags().GetInt("z")
	x, _ := cmd.Flags().GetInt("x")
	y, _ := cmd.Flags().GetInt("y")
	sourceTypeOverride, _ := cmd.Flags().GetString("source-type")
	outputPath, _ := cmd.Flags().GetString("output")
	metadata, _ := cmd.Flags().GetBool("metadata")

	// Validate input parameters
	if url == "" && filePath == "" && (z == 0 && x == 0 && y == 0) {
		return fmt.Errorf("must specify either --url, --file, or --z/--x/--y coordinates")
	}

	// Determine source type and override configuration if needed
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

	// Create fetcher factory and determine source type
	factory := tile.NewFetcherFactory(cfg)

	var sourceType internal.SourceType
	var tileRequest *tile.TileRequest

	// Determine source type based on input parameters
	if url != "" {
		sourceType = internal.SourceTypeHTTP
		tileRequest = &tile.TileRequest{
			URL: url,
			Z:   z, X: x, Y: y, // May be zero if not provided
		}
	} else if filePath != "" {
		sourceType = internal.SourceTypeLocal
		tileRequest = &tile.TileRequest{
			URL: filePath,      // Use URL field for file path in local mode
			Z:   z, X: x, Y: y, // May be zero if not provided
		}
	} else {
		// Using coordinates - determine source type from configuration
		sourceType = cfg.DetermineSourceType()

		// Validate coordinates
		if err := tile.ValidateCoordinates(z, x, y); err != nil {
			return fmt.Errorf("invalid tile coordinates: %w", err)
		}

		// Validate source configuration
		switch sourceType {
		case internal.SourceTypeHTTP:
			if cfg.Server.BaseURL == "" {
				return fmt.Errorf("base URL is required for HTTP source with coordinates")
			}
			tileRequest = tile.NewTileRequest(z, x, y, cfg.Server.BaseURL)
		case internal.SourceTypeLocal:
			if cfg.Local.BasePath == "" {
				return fmt.Errorf("base path is required for local source with coordinates")
			}
			tileRequest = &tile.TileRequest{Z: z, X: x, Y: y}
		default:
			return fmt.Errorf("unable to determine source type from configuration")
		}
	}

	// Validate source configuration
	if err := factory.ValidateConfiguration(sourceType); err != nil {
		return fmt.Errorf("source configuration validation failed: %w", err)
	}

	// Create appropriate fetcher
	fetcher, err := factory.CreateFetcherForType(sourceType)
	if err != nil {
		return fmt.Errorf("failed to create fetcher: %w", err)
	}

	// Create processor
	processor := tile.NewMVTProcessor()

	// Report what we're doing
	if viper.GetBool("logging.verbose") {
		if sourceType == internal.SourceTypeHTTP {
			fmt.Fprintf(os.Stderr, "Fetching tile from URL: %s\n", tileRequest.URL)
		} else {
			if filePath != "" {
				fmt.Fprintf(os.Stderr, "Reading tile from file: %s\n", filePath)
			} else {
				tilePath := cfg.GetTilePath(z, x, y)
				fmt.Fprintf(os.Stderr, "Reading tile from: %s\n", tilePath)
			}
		}
	}

	// Fetch the tile
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
		Compression: viper.GetBool("output.compression"),
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

		fmt.Fprintf(os.Stderr, "Source: %s\n", sourceType)
	}

	return nil
}
