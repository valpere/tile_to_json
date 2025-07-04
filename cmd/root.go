// cmd/root.go - Root command implementation
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tile-to-json",
	Short: "Convert Mapbox Vector Tiles to JSON format",
	Long: `TileToJson is a high-performance command-line tool for converting Mapbox Vector Tiles 
from Protocol Buffer format to JSON/GeoJSON format. It supports both single tile 
conversion and large-scale batch processing operations with multiple data sources.

Data Sources:
- Remote tile servers via HTTP/HTTPS
- Local tile files and directories
- Automatic source type detection

Features:
- Convert individual tiles or batch process tile ranges
- Support for GeoJSON and custom JSON output formats
- Concurrent processing for optimal performance
- Comprehensive error handling and retry mechanisms
- Configurable output destinations and compression

Examples:
  # Convert a single remote tile
  tile-to-json convert --url "https://example.com/tiles/14/8362/5956.mvt" --output tile.geojson

  # Convert a local tile file
  tile-to-json convert --file "/path/to/tiles/14/8362/5956.mvt" --output tile.geojson

  # Convert using coordinates with remote server
  tile-to-json convert --base-url "https://example.com/tiles" --z 14 --x 8362 --y 5956

  # Convert using coordinates with local files
  tile-to-json convert --base-path "/path/to/tiles" --z 14 --x 8362 --y 5956

  # Batch process remote tiles
  tile-to-json batch --base-url "https://example.com/tiles" --min-zoom 10 --max-zoom 12 --bbox "-74.0,40.7,-73.9,40.8"

  # Batch process local tile directory
  tile-to-json batch --base-path "/path/to/tiles" --min-zoom 10 --max-zoom 12 --bbox "-74.0,40.7,-73.9,40.8"

  # Use configuration file
  tile-to-json convert --config config.yaml --z 14 --x 8362 --y 5956`,
	Version: "1.0.0",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.tile-to-json.yaml)")
	
	// Source configuration flags
	rootCmd.PersistentFlags().String("source-type", "auto", "data source type (auto, http, local)")
	rootCmd.PersistentFlags().String("base-url", "", "base URL for tile server (HTTP source)")
	rootCmd.PersistentFlags().String("base-path", "", "base path for local tiles (local source)")
	rootCmd.PersistentFlags().String("api-key", "", "API key for authentication (HTTP source)")
	
	// Output flags
	rootCmd.PersistentFlags().StringP("format", "f", "geojson", "output format (geojson, json)")
	rootCmd.PersistentFlags().Bool("pretty", true, "pretty print JSON output")
	rootCmd.PersistentFlags().Bool("compression", false, "compress output files")
	
	// Processing flags
	rootCmd.PersistentFlags().Bool("verbose", false, "verbose output")
	rootCmd.PersistentFlags().Int("concurrency", 10, "number of concurrent requests")
	rootCmd.PersistentFlags().Duration("timeout", 30*1000000000, "request timeout (HTTP source)")
	rootCmd.PersistentFlags().Int("retries", 3, "number of retry attempts")

	// Bind flags to viper
	viper.BindPFlag("source.type", rootCmd.PersistentFlags().Lookup("source-type"))
	viper.BindPFlag("server.base_url", rootCmd.PersistentFlags().Lookup("base-url"))
	viper.BindPFlag("local.base_path", rootCmd.PersistentFlags().Lookup("base-path"))
	viper.BindPFlag("server.api_key", rootCmd.PersistentFlags().Lookup("api-key"))
	viper.BindPFlag("output.format", rootCmd.PersistentFlags().Lookup("format"))
	viper.BindPFlag("output.pretty", rootCmd.PersistentFlags().Lookup("pretty"))
	viper.BindPFlag("output.compression", rootCmd.PersistentFlags().Lookup("compression"))
	viper.BindPFlag("logging.verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("batch.concurrency", rootCmd.PersistentFlags().Lookup("concurrency"))
	viper.BindPFlag("server.timeout", rootCmd.PersistentFlags().Lookup("timeout"))
	viper.BindPFlag("server.max_retries", rootCmd.PersistentFlags().Lookup("retries"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".tile-to-json" (without extension)
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".tile-to-json")
	}

	// Environment variables
	viper.SetEnvPrefix("TILE_TO_JSON")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		if viper.GetBool("logging.verbose") {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}
