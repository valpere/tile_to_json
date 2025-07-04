# TileToJson

A high-performance command-line tool for converting Mapbox Vector Tiles (MVT) from Protocol Buffer format to JSON/GeoJSON format. Supports both remote tile servers and local tile files.

## Features

- **Multiple Data Sources**: Process tiles from remote HTTP servers or local file systems
- **Single Tile Conversion**: Convert individual tiles with precise coordinate specification
- **Batch Processing**: High-throughput processing of tile ranges with concurrent execution
- **Automatic Source Detection**: Intelligently determines whether to use HTTP or local file access
- **Multiple Output Formats**: Support for GeoJSON, JSON, and custom formats
- **Flexible Output Options**: Single file, multi-file, or stdout output with optional compression
- **Robust Error Handling**: Comprehensive retry mechanisms and graceful error recovery
- **Progress Monitoring**: Real-time progress tracking for batch operations
- **Configurable Processing**: Extensive configuration options via CLI flags, config files, or environment variables

## Installation

### From Source

```bash
git clone https://github.com/valpere/tile_to_json.git
cd tile_to_json
go build -o tile-to-json
```

### Pre-built Binaries

Download the latest release from the [releases page](https://github.com/valpere/tile_to_json/releases).

## Quick Start

### Convert a Single Tile

```bash
# Remote tile via direct URL
tile-to-json convert --url "https://example.com/tiles/14/8362/5956.mvt" --output tile.geojson

# Local tile via direct file path
tile-to-json convert --file "/path/to/tiles/14/8362/5956.mvt" --output tile.geojson

# Remote tile using coordinates and base URL
tile-to-json convert --base-url "https://example.com/tiles" --z 14 --x 8362 --y 5956 --output tile.geojson

# Local tile using coordinates and base path
tile-to-json convert --base-path "/path/to/tiles" --z 14 --x 8362 --y 5956 --output tile.geojson

# Output to stdout with pretty formatting
tile-to-json convert --url "https://example.com/tiles/14/8362/5956.mvt" --pretty
```

### Batch Process Multiple Tiles

```bash
# Process remote tiles in a zoom range with bounding box
tile-to-json batch --base-url "https://example.com/tiles" --min-zoom 10 --max-zoom 12 --bbox "-74.0,40.7,-73.9,40.8" --output-dir ./tiles/

# Process local tile directory
tile-to-json batch --base-path "/path/to/tiles" --min-zoom 10 --max-zoom 12 --bbox "-74.0,40.7,-73.9,40.8" --output-dir ./output/

# Process specific zoom level from local files
tile-to-json batch --base-path "/path/to/tiles" --zoom 14 --bbox "-74.0,40.7,-73.9,40.8" --output-dir ./output/

# Combine all tiles into single file
tile-to-json batch --base-path "/path/to/tiles" --zoom 10 --bbox "-74.0,40.7,-73.9,40.8" --output tiles.geojson --single-file
```

## Data Sources

TileToJson supports two primary data sources:

### Remote Tile Servers (HTTP/HTTPS)

Access tiles from remote servers using standard HTTP protocols:

- **URL Templates**: Configurable URL patterns for different tile server types
- **Authentication**: Support for API keys, custom headers, and authentication tokens
- **Network Resilience**: Automatic retry with exponential backoff and connection pooling
- **Rate Limiting**: Respect server rate limits and implement request throttling

### Local Tile Files

Process tiles stored on the local file system:

- **Directory Structure**: Standard z/x/y.mvt hierarchy or custom organization patterns
- **File Formats**: Support for both uncompressed (.mvt) and compressed (.mvt.gz) files
- **Path Templates**: Configurable file path patterns for different storage layouts
- **Validation**: Pre-processing validation to ensure tile availability

### Automatic Source Detection

The application automatically detects the appropriate source type based on:

- Configuration parameters (base-url vs base-path)
- Command-line flags (--url vs --file)
- File system checks and URL validation

## Command Reference

### Global Options

| Flag | Description | Default |
|------|-------------|---------|
| `--config` | Configuration file path | `$HOME/.tile-to-json.yaml` |
| `--source-type` | Data source type (auto, http, local) | `auto` |
| `--base-url` | Base URL for tile server (HTTP source) | - |
| `--base-path` | Base path for local tiles (local source) | - |
| `--api-key` | API key for authentication (HTTP source) | - |
| `--format` | Output format (geojson, json) | `geojson` |
| `--pretty` | Pretty print JSON output | `true` |
| `--compression` | Compress output files | `false` |
| `--verbose` | Verbose output | `false` |
| `--concurrency` | Number of concurrent requests | `10` |
| `--timeout` | Request timeout (HTTP source) | `30s` |
| `--retries` | Number of retry attempts | `3` |

### Convert Command

Convert a single Mapbox Vector Tile to JSON format.

```bash
tile-to-json convert [flags]
```

#### Flags

| Flag | Description | Required |
|------|-------------|----------|
| `--url` | Direct URL to remote tile | Either URL, file, or coordinates |
| `--file` | Direct path to local tile file | Either URL, file, or coordinates |
| `--z` | Tile zoom level | Either URL, file, or coordinates |
| `--x` | Tile x coordinate | Either URL, file, or coordinates |
| `--y` | Tile y coordinate | Either URL, file, or coordinates |
| `--source-type` | Override source type (http, local) | No |
| `--output, -o` | Output file path (default: stdout) | No |
| `--metadata` | Include tile metadata in output | No |

### Batch Command

Batch process multiple Mapbox Vector Tiles with concurrent execution.

```bash
tile-to-json batch [flags]
```

#### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--zoom` | Single zoom level to process | - |
| `--min-zoom` | Minimum zoom level | - |
| `--max-zoom` | Maximum zoom level | - |
| `--bbox` | Bounding box: 'min_lon,min_lat,max_lon,max_lat' | - |
| `--tiles` | Specific tiles list: 'z/x/y,z/x/y,...' | - |
| `--source-type` | Override source type (http, local) | - |
| `--output-dir` | Output directory for tiles | `./output` |
| `--output, -o` | Single output file (use with --single-file) | - |
| `--single-file` | Combine all tiles into single file | `false` |
| `--multi-file` | Output each tile to separate file | `true` |
| `--chunk-size` | Number of tiles per processing chunk | `100` |
| `--fail-on-error` | Stop processing on first error | `false` |
| `--progress` | Show progress indicator | `true` |

## Configuration

TileToJson supports configuration via YAML files, environment variables, and command-line flags.

### Configuration File

Create a configuration file at `$HOME/.tile-to-json.yaml`:

```yaml
# Source configuration
source:
  type: "auto"              # auto, http, local
  default_type: "http"      # Default when auto-detection is ambiguous
  auto_detect: true         # Enable automatic source detection

# HTTP server configuration
server:
  base_url: "https://your-tile-server.com/tiles"
  api_key: "your-api-key"
  timeout: 30s
  max_retries: 3
  url_template: "{base_url}/{z}/{x}/{y}.mvt"
  headers:
    User-Agent: "TileToJson/1.0"

# Local file configuration  
local:
  base_path: "/path/to/tiles"
  path_template: "{base_path}/{z}/{x}/{y}.mvt"
  extension: ".mvt"
  compressed: false         # Set to true if files are .mvt.gz

# Output configuration
output:
  format: "geojson"
  pretty: true
  compression: false

# Batch processing configuration
batch:
  concurrency: 20
  chunk_size: 100
  timeout: 5m
  fail_on_error: false

# Network configuration (HTTP sources)
network:
  proxy_url: ""
  keep_alive: 30s
  max_idle_conns: 100
  idle_conn_timeout: 90s

# Logging configuration
logging:
  level: "info"
  format: "text"
  verbose: false
```

### Environment Variables

All configuration options can be set via environment variables with the `TILE_TO_JSON_` prefix:

```bash
export TILE_TO_JSON_SOURCE_TYPE="local"
export TILE_TO_JSON_LOCAL_BASE_PATH="/path/to/tiles"
export TILE_TO_JSON_SERVER_BASE_URL="https://your-tile-server.com/tiles"
export TILE_TO_JSON_SERVER_API_KEY="your-api-key"
export TILE_TO_JSON_BATCH_CONCURRENCY=20
export TILE_TO_JSON_OUTPUT_FORMAT="geojson"
```
| `--metadata` | Include tile metadata in output | No |

### Batch Command

Batch process multiple Mapbox Vector Tiles with concurrent execution.

```bash
tile-to-json batch [flags]
```

#### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--zoom` | Single zoom level to process | - |
| `--min-zoom` | Minimum zoom level | - |
| `--max-zoom` | Maximum zoom level | - |
| `--bbox` | Bounding box: 'min_lon,min_lat,max_lon,max_lat' | - |
| `--tiles` | Specific tiles list: 'z/x/y,z/x/y,...' | - |
| `--output-dir` | Output directory for tiles | `./output` |
| `--output, -o` | Single output file (use with --single-file) | - |
| `--single-file` | Combine all tiles into single file | `false` |
| `--multi-file` | Output each tile to separate file | `true` |
| `--chunk-size` | Number of tiles per processing chunk | `100` |
| `--fail-on-error` | Stop processing on first error | `false` |
| `--progress` | Show progress indicator | `true` |

## Configuration

TileToJson supports configuration via YAML files, environment variables, and command-line flags.

### Configuration File

Create a configuration file at `$HOME/.tile-to-json.yaml`:

```yaml
server:
  base_url: "https://your-tile-server.com/tiles"
  api_key: "your-api-key"
  timeout: 30s
  max_retries: 3
  headers:
    User-Agent: "TileToJson/1.0"

output:
  format: "geojson"
  pretty: true
  compression: false

batch:
  concurrency: 20
  chunk_size: 100
  timeout: 5m
  fail_on_error: false

network:
  proxy_url: ""
  keep_alive: 30s
  max_idle_conns: 100
  idle_conn_timeout: 90s

logging:
  level: "info"
  format: "text"
  verbose: false
```

### Environment Variables

All configuration options can be set via environment variables with the `TILE_TO_JSON_` prefix:

```bash
export TILE_TO_JSON_SERVER_BASE_URL="https://your-tile-server.com/tiles"
export TILE_TO_JSON_SERVER_API_KEY="your-api-key"
export TILE_TO_JSON_BATCH_CONCURRENCY=20
export TILE_TO_JSON_OUTPUT_FORMAT="geojson"
```

## Output Formats

### GeoJSON (default)

Standard GeoJSON FeatureCollection format:

```json
{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "geometry": {
        "type": "Point",
        "coordinates": [-74.006, 40.7128]
      },
      "properties": {
        "name": "New York",
        "_layer": "places"
      }
    }
  ]
}
```

### JSON

Structured JSON with tile metadata:

```json
{
  "coordinate": {"z": 14, "x": 8362, "y": 5956},
  "data": { /* GeoJSON data */ },
  "metadata": {
    "layers": ["places", "roads"],
    "feature_count": 1250,
    "size": 45678,
    "process_time": "15ms"
  }
}
```

## Performance Optimization

### Concurrency Settings

- **Single tile**: Use default settings for optimal balance
- **Batch processing**: Increase `--concurrency` for faster processing (recommended: 10-50)
- **Large datasets**: Adjust `--chunk-size` based on memory constraints

### Network Optimization

- Configure `keep_alive` and connection pooling settings
- Use compression for large output files
- Consider proxy settings for corporate environments

### Memory Management

- Use multi-file output for very large datasets
- Adjust chunk sizes based on available memory
- Monitor memory usage during batch operations

## Error Handling

TileToJson implements comprehensive error handling:

- **Network errors**: Automatic retry with exponential backoff
- **Tile processing errors**: Continue processing remaining tiles (unless `--fail-on-error`)
- **Output errors**: Graceful handling with detailed error messages
- **Configuration errors**: Early validation with helpful feedback

## Examples

### Basic Usage

```bash
# Convert single tile to GeoJSON
tile-to-json convert \
  --base-url "https://tiles.example.com" \
  --z 14 --x 8362 --y 5956 \
  --output manhattan.geojson

# Batch process Manhattan area at multiple zoom levels
tile-to-json batch \
  --base-url "https://tiles.example.com" \
  --min-zoom 10 --max-zoom 14 \
  --bbox "-74.02,40.70,-73.93,40.80" \
  --output-dir ./manhattan-tiles/ \
  --concurrency 20
```

### Advanced Usage

```bash
# High-performance batch processing with compression
tile-to-json batch \
  --config production.yaml \
  --min-zoom 8 --max-zoom 12 \
  --bbox "-180,-85,180,85" \
  --output-dir ./world-tiles/ \
  --concurrency 50 \
  --chunk-size 200 \
  --format geojson \
  --compression \
  --progress

# Process specific tiles with custom output
tile-to-json batch \
  --tiles "14/8362/5956,14/8363/5956,14/8362/5957" \
  --output custom-tiles.json \
  --single-file \
  --format json \
  --metadata
```

### Integration Examples

```bash
# Pipeline with other tools
tile-to-json convert --url "https://tiles.example.com/14/8362/5956.mvt" | \
  jq '.features[].properties.name' | \
  sort | uniq

# Batch processing with progress monitoring
tile-to-json batch \
  --min-zoom 10 --max-zoom 12 \
  --bbox "-74.0,40.7,-73.9,40.8" \
  --output-dir ./tiles/ \
  --progress \
  2>/dev/null | tee processing.log
```

## Troubleshooting

### Common Issues

**Connection timeouts**
```bash
# Increase timeout and retries
tile-to-json convert --timeout 60s --retries 5 --url "..."
```

**Memory issues with large batches**
```bash
# Reduce chunk size and concurrency
tile-to-json batch --chunk-size 50 --concurrency 5 --min-zoom 10 --max-zoom 12
```

**Authentication errors**
```bash
# Set API key via environment variable
export TILE_TO_JSON_SERVER_API_KEY="your-key"
tile-to-json convert --z 14 --x 8362 --y 5956
```

### Debug Mode

Enable verbose output for debugging:

```bash
tile-to-json --verbose convert --url "..." --output debug.json
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Support

- **Documentation**: [https://valpere.github.io/tile_to_json/](https://valpere.github.io/tile_to_json/)
- **Issues**: [GitHub Issues](https://github.com/valpere/tile_to_json/issues)
- **Discussions**: [GitHub Discussions](https://github.com/valpere/tile_to_json/discussions)

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for release history and changes.
