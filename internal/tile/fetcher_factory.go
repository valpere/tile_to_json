// internal/tile/fetcher_factory.go - Fetcher factory implementation
package tile

import (
	"fmt"

	"github.com/valpere/tile_to_json/internal"
	"github.com/valpere/tile_to_json/internal/config"
)

// FetcherFactory creates appropriate fetchers based on configuration
type FetcherFactory struct {
	config *config.Config
}

// NewFetcherFactory creates a new fetcher factory
func NewFetcherFactory(cfg *config.Config) *FetcherFactory {
	return &FetcherFactory{
		config: cfg,
	}
}

// CreateFetcher creates the appropriate fetcher based on configuration
func (f *FetcherFactory) CreateFetcher() (Fetcher, error) {
	sourceType := f.config.DetermineSourceType()

	switch sourceType {
	case internal.SourceTypeHTTP:
		return NewHTTPFetcher(f.config), nil
	case internal.SourceTypeLocal:
		return NewLocalFetcher(f.config), nil
	default:
		return nil, fmt.Errorf("unsupported source type: %s", sourceType)
	}
}

// CreateFetcherForType creates a fetcher for a specific source type
func (f *FetcherFactory) CreateFetcherForType(sourceType internal.SourceType) (Fetcher, error) {
	switch sourceType {
	case internal.SourceTypeHTTP:
		if f.config.Server.BaseURL == "" {
			return nil, fmt.Errorf("base_url is required for HTTP fetcher")
		}
		return NewHTTPFetcher(f.config), nil
	case internal.SourceTypeLocal:
		if f.config.Local.BasePath == "" {
			return nil, fmt.Errorf("base_path is required for local fetcher")
		}
		return NewLocalFetcher(f.config), nil
	default:
		return nil, fmt.Errorf("unsupported source type: %s", sourceType)
	}
}

// ValidateConfiguration validates that the configuration supports the requested source type
func (f *FetcherFactory) ValidateConfiguration(sourceType internal.SourceType) error {
	switch sourceType {
	case internal.SourceTypeHTTP:
		if f.config.Server.BaseURL == "" {
			return fmt.Errorf("base_url is required for HTTP source")
		}
		if f.config.Server.URLTemplate == "" {
			return fmt.Errorf("url_template is required for HTTP source")
		}
	case internal.SourceTypeLocal:
		if f.config.Local.BasePath == "" {
			return fmt.Errorf("base_path is required for local source")
		}
		if f.config.Local.PathTemplate == "" {
			return fmt.Errorf("path_template is required for local source")
		}
		// Validate that base path exists
		if err := config.ValidateLocalTileDirectory(f.config); err != nil {
			return fmt.Errorf("local tile directory validation failed: %w", err)
		}
	default:
		return fmt.Errorf("unsupported source type: %s", sourceType)
	}

	return nil
}

// GetSupportedSourceTypes returns the source types that can be created with current configuration
func (f *FetcherFactory) GetSupportedSourceTypes() []internal.SourceType {
	var supported []internal.SourceType

	// Check HTTP support
	if f.config.Server.BaseURL != "" {
		supported = append(supported, internal.SourceTypeHTTP)
	}

	// Check local support
	if f.config.Local.BasePath != "" {
		supported = append(supported, internal.SourceTypeLocal)
	}

	return supported
}

// AutoDetectSourceType attempts to automatically detect the best source type
func (f *FetcherFactory) AutoDetectSourceType() internal.SourceType {
	return f.config.DetermineSourceType()
}

// CreateOptimalFetcher creates the best fetcher based on current configuration and preferences
func (f *FetcherFactory) CreateOptimalFetcher() (Fetcher, error) {
	supportedTypes := f.GetSupportedSourceTypes()

	if len(supportedTypes) == 0 {
		return nil, fmt.Errorf("no valid source configuration found")
	}

	// If only one type is supported, use it
	if len(supportedTypes) == 1 {
		return f.CreateFetcherForType(supportedTypes[0])
	}

	// Multiple types supported - use auto-detection
	detectedType := f.AutoDetectSourceType()
	return f.CreateFetcherForType(detectedType)
}

// ConvenientFetcher wraps a fetcher with additional convenience methods
type ConvenientFetcher struct {
	Fetcher
	factory *FetcherFactory
	config  *config.Config
}

// NewConvenientFetcher creates a fetcher with convenience methods
func NewConvenientFetcher(cfg *config.Config) (*ConvenientFetcher, error) {
	factory := NewFetcherFactory(cfg)
	fetcher, err := factory.CreateOptimalFetcher()
	if err != nil {
		return nil, err
	}

	return &ConvenientFetcher{
		Fetcher: fetcher,
		factory: factory,
		config:  cfg,
	}, nil
}

// FetchTile is a convenience method for fetching tiles by coordinates
func (cf *ConvenientFetcher) FetchTile(z, x, y int) (*TileResponse, error) {
	var request *TileRequest

	sourceType := cf.config.DetermineSourceType()
	switch sourceType {
	case internal.SourceTypeHTTP:
		url := cf.config.GetTileURL(z, x, y)
		request = &TileRequest{
			Z:   z,
			X:   x,
			Y:   y,
			URL: url,
		}
	case internal.SourceTypeLocal:
		request = &TileRequest{
			Z: z,
			X: x,
			Y: y,
		}
	default:
		return nil, fmt.Errorf("unsupported source type: %s", sourceType)
	}

	return cf.FetchWithRetry(request)
}

// ValidateTileAvailability checks if a tile is available from the configured source
func (cf *ConvenientFetcher) ValidateTileAvailability(z, x, y int) error {
	sourceType := cf.config.DetermineSourceType()

	switch sourceType {
	case internal.SourceTypeLocal:
		if localFetcher, ok := cf.Fetcher.(*LocalFetcher); ok {
			return localFetcher.ValidateTileExists(z, x, y)
		}
		return fmt.Errorf("fetcher is not a local fetcher")
	case internal.SourceTypeHTTP:
		// For HTTP sources, we can only validate by attempting to fetch
		// or by making a HEAD request (not implemented here for simplicity)
		return nil
	default:
		return fmt.Errorf("unsupported source type: %s", sourceType)
	}
}

// GetSourceType returns the source type being used by this fetcher
func (cf *ConvenientFetcher) GetSourceType() internal.SourceType {
	return cf.config.DetermineSourceType()
}

// IsLocal returns true if this fetcher uses local file access
func (cf *ConvenientFetcher) IsLocal() bool {
	return cf.GetSourceType() == internal.SourceTypeLocal
}

// IsRemote returns true if this fetcher uses remote HTTP access
func (cf *ConvenientFetcher) IsRemote() bool {
	return cf.GetSourceType() == internal.SourceTypeHTTP
}
