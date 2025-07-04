# TileToJson Coding Standards and Architecture Guidelines

## Overview

This document establishes coding standards and architectural principles for the TileToJson project, ensuring consistency, maintainability, and adherence to Phase 1 foundation requirements.

## Design Principles

### Primary Principles
- **DRY (Don't Repeat Yourself)**: Eliminate code duplication through proper abstraction
- **YAGNI (You Aren't Gonna Need It)**: Implement only current requirements
- **KISS (Keep It Simple, Stupid)**: Prefer simple, understandable solutions
- **Encapsulation**: Hide implementation details behind clean interfaces
- **PoLA (Principle of Least Astonishment)**: Code behavior should be predictable

### SOLID Principles
- **Single Responsibility**: Each component has one reason to change
- **Open/Closed**: Open for extension, closed for modification
- **Liskov Substitution**: Subtypes must be substitutable for base types
- **Interface Segregation**: Clients shouldn't depend on unused interfaces
- **Dependency Inversion**: Depend on abstractions, not concretions

## Package Organization

### Directory Structure
```
tile_to_json/
├── cmd/                 # Command-line interface
├── internal/            # Internal application packages
│   ├── batch/          # Batch processing logic
│   ├── config/         # Configuration management
│   ├── output/         # Output formatting and writing
│   ├── tile/           # Tile fetching and processing
│   └── types.go        # Common internal types
├── pkg/                # Public API packages
│   └── mvt/           # MVT conversion utilities
├── build/             # Build outputs
├── docs/              # Documentation
└── scripts/           # Build and utility scripts
```

### Type Organization Rules

1. **Common types** shared across multiple packages within `internal/` → `internal/types.go`
2. **Package-specific types** → `{package}/types.go`
3. **Hierarchy precedence**: Lower-level packages define types; higher-level packages import
4. **No circular dependencies**: Enforce strict dependency flow

## Source Type Architecture

### Dual Source Support
The application must support both HTTP and local file sources seamlessly:

```go
type SourceType string

const (
    SourceTypeHTTP  SourceType = "http"
    SourceTypeLocal SourceType = "local"
)
```

### Factory Pattern Implementation
Use factory pattern for source-agnostic component creation:

```go
type FetcherFactory interface {
    CreateFetcher() (Fetcher, error)
    CreateFetcherForType(sourceType SourceType) (Fetcher, error)
    ValidateConfiguration(sourceType SourceType) error
}
```

### Configuration Management
Support multiple configuration sources with validation:

```go
type Config struct {
    Server  ServerConfig  // HTTP source configuration
    Local   LocalConfig   // Local file configuration
    Source  SourceConfig  // Source type determination
    // ... other configurations
}
```

## Error Handling Standards

### Application-Specific Errors
Use structured error types with contextual information:

```go
type Error struct {
    Code    string
    Message string
    Cause   error
}

// Error codes for categorization
const (
    ErrorCodeNetwork    = "NETWORK_ERROR"
    ErrorCodeProcessing = "PROCESSING_ERROR"
    ErrorCodeValidation = "VALIDATION_ERROR"
    ErrorCodeConfig     = "CONFIG_ERROR"
    ErrorCodeNotFound   = "NOT_FOUND"
    ErrorCodeTimeout    = "TIMEOUT_ERROR"
    ErrorCodeFileSystem = "FILESYSTEM_ERROR"
    ErrorCodePermission = "PERMISSION_ERROR"
)
```

### Error Wrapping
Preserve error context through the call stack:

```go
func processData(data []byte) error {
    if err := validate(data); err != nil {
        return fmt.Errorf("data validation failed: %w", err)
    }
    return nil
}
```

## Interface Design

### Core Interfaces
Define clear, focused interfaces:

```go
type Fetcher interface {
    Fetch(request *TileRequest) (*TileResponse, error)
    FetchWithRetry(request *TileRequest) (*TileResponse, error)
}

type Processor interface {
    Process(response *TileResponse) (*ProcessedTile, error)
    ProcessBatch(responses []*TileResponse) ([]*ProcessedTile, error)
}

type Writer interface {
    Write(tile *ProcessedTile) error
    WriteBatch(tiles []*ProcessedTile) error
    Close() error
}
```

### Interface Segregation
Keep interfaces small and focused:

```go
// Good: Separate concerns
type Reader interface {
    Read() (*Data, error)
}

type Writer interface {
    Write(*Data) error
}

// Avoid: Large, multi-purpose interfaces
type DataHandler interface {
    Read() (*Data, error)
    Write(*Data) error
    Validate(*Data) error
    Transform(*Data) (*Data, error)
}
```

## Configuration Standards

### Hierarchical Configuration
Support multiple configuration sources:

1. Command-line flags (highest priority)
2. Environment variables
3. Configuration files
4. Default values (lowest priority)

### Validation
Validate configuration at application startup:

```go
func Validate(config *Config) error {
    if err := validateSource(&config.Source); err != nil {
        return fmt.Errorf("source configuration invalid: %w", err)
    }
    // ... validate other sections
    return nil
}
```

### Source Type Detection
Implement intelligent source type detection:

```go
func (c *Config) DetermineSourceType() SourceType {
    if !c.Source.AutoDetect {
        return SourceType(c.Source.Type)
    }
    
    // Auto-detection logic
    if c.Local.BasePath != "" && c.Server.BaseURL == "" {
        return SourceTypeLocal
    }
    if c.Server.BaseURL != "" && c.Local.BasePath == "" {
        return SourceTypeHTTP
    }
    
    return SourceType(c.Source.DefaultType)
}
```

## Concurrency Patterns

### Worker Pool Pattern
Use worker pools for concurrent processing:

```go
func (bp *BatchProcessor) ProcessChunk(ctx context.Context, workItems []*WorkItem) (*ChunkResult, error) {
    workChan := make(chan *WorkItem, len(workItems))
    resultChan := make(chan *WorkResult, len(workItems))

    // Start workers
    var wg sync.WaitGroup
    for i := 0; i < concurrency; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            bp.worker(ctx, workChan, resultChan)
        }()
    }
    
    // ... process results
}
```

### Context Propagation
Always propagate context for cancellation:

```go
func (f *HTTPFetcher) Fetch(ctx context.Context, request *TileRequest) (*TileResponse, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", request.URL, nil)
    if err != nil {
        return nil, err
    }
    // ... continue processing
}
```

## Testing Standards

### Test Organization
Follow Go testing conventions:

```go
// func_test.go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        want    OutputType
        wantErr bool
    }{
        {
            name:    "valid input",
            input:   validInput,
            want:    expectedOutput,
            wantErr: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := FunctionName(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("FunctionName() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("FunctionName() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Mock Interfaces
Use interfaces for testability:

```go
type MockFetcher struct {
    responses map[string]*TileResponse
}

func (m *MockFetcher) Fetch(request *TileRequest) (*TileResponse, error) {
    if response, exists := m.responses[request.URL]; exists {
        return response, nil
    }
    return nil, errors.New("tile not found")
}
```

## Logging Standards

### Structured Logging
Use structured logging with consistent fields:

```go
log.WithFields(log.Fields{
    "tile_z": z,
    "tile_x": x,
    "tile_y": y,
    "source": sourceType,
    "duration": duration,
}).Info("tile processed successfully")
```

### Log Levels
Use appropriate log levels:

- **Debug**: Detailed diagnostic information
- **Info**: General operational messages
- **Warn**: Warning conditions that don't prevent operation
- **Error**: Error conditions that may affect operation
- **Fatal**: Critical errors that cause program termination

## Performance Guidelines

### Memory Management
- Use object pooling for frequently allocated objects
- Minimize memory allocations in hot paths
- Close resources properly (defer statements)

### Network Optimization
- Implement connection pooling
- Use appropriate timeouts
- Implement retry logic with exponential backoff

### File I/O Optimization
- Use buffered I/O for large operations
- Minimize file system calls
- Implement proper file locking for concurrent access

## Documentation Standards

### Code Comments
- Document public APIs thoroughly
- Explain complex algorithms and business logic
- Use godoc-compatible comments

### Package Documentation
Each package should have a doc.go file:

```go
// Package tile provides functionality for fetching and processing
// Mapbox Vector Tiles from various sources including HTTP servers
// and local file systems.
//
// The package supports both remote tile servers via HTTP/HTTPS and
// local tile files with automatic source type detection.
package tile
```

### README Requirements
- Clear installation instructions
- Usage examples for common scenarios
- Configuration documentation
- Troubleshooting guide

## Security Considerations

### Input Validation
- Validate all external inputs
- Sanitize file paths and URLs
- Implement proper bounds checking

### Authentication
- Support multiple authentication methods
- Secure credential storage
- Implement proper session management

### Network Security
- Use HTTPS for remote connections
- Validate SSL certificates
- Implement proper proxy support

## Build and Deployment

### Build Configuration
Use comprehensive Makefile targets:

```makefile
.PHONY: build test lint fmt clean
build: deps fmt lint test
    go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) .

test:
    go test -v -race -coverprofile=coverage.out ./...

lint:
    golangci-lint run ./...
```

### Dependency Management
- Pin dependency versions in go.mod
- Regular security updates
- Minimize external dependencies

### Release Process
- Semantic versioning
- Automated testing in CI/CD
- Multi-platform builds
- Comprehensive changelog

## Maintenance Guidelines

### Code Review Process
- All changes require code review
- Automated testing before merge
- Documentation updates with code changes

### Refactoring Standards
- Maintain backward compatibility
- Update tests with refactoring
- Document breaking changes

### Technical Debt Management
- Regular code quality assessment
- Prioritize technical debt in planning
- Track metrics and improve over time

## Compliance and Quality Metrics

### Code Quality Metrics
- Test coverage > 80%
- Cyclomatic complexity < 10
- No security vulnerabilities
- Documentation coverage > 90%

### Performance Benchmarks
- Single tile conversion < 100ms
- Batch processing > 1000 tiles/minute
- Memory usage remains stable
- No memory leaks in long-running operations
