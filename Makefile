# TileToJson Makefile
# Comprehensive build and operations automation for the TileToJson application

# Application configuration
APP_NAME := tile-to-json
VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v1.0.0")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go configuration
GO_VERSION := 1.24.4
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

# Build configuration
BUILD_DIR := build
DIST_DIR := dist
COVERAGE_DIR := coverage
DOCS_DIR := docs

# Build flags and linker settings
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILD_DATE) -w -s"
BUILD_FLAGS := -trimpath -mod=readonly

# Testing configuration
TEST_FLAGS := -v -race -coverprofile=$(COVERAGE_DIR)/coverage.out
BENCHMARK_FLAGS := -benchmem -run=^$$ -bench=.

# Supported platforms for cross-compilation
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

# Default target
.PHONY: all
all: clean test build

# Development targets
.PHONY: dev
dev: deps fmt lint test build

.PHONY: quick
quick: fmt build

# Dependency management
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod verify
	go mod tidy

.PHONY: deps-update
deps-update:
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

.PHONY: deps-vendor
deps-vendor: deps
	@echo "Vendoring dependencies..."
	go mod vendor

# Code quality and formatting
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

.PHONY: lint
lint:
	@echo "Running linters..."
	golangci-lint run ./...

.PHONY: vet
vet:
	@echo "Running go vet..."
	go vet ./...

.PHONY: security
security:
	@echo "Running security analysis..."
	gosec ./...

# Testing targets
.PHONY: test
test: test-setup
	@echo "Running tests..."
	go test $(TEST_FLAGS) ./...

.PHONY: test-short
test-short: test-setup
	@echo "Running short tests..."
	go test -short $(TEST_FLAGS) ./...

.PHONY: test-verbose
test-verbose: test-setup
	@echo "Running verbose tests..."
	go test -v $(TEST_FLAGS) ./...

.PHONY: test-integration
test-integration: test-setup
	@echo "Running integration tests..."
	go test -tags=integration $(TEST_FLAGS) ./...

.PHONY: benchmark
benchmark: test-setup
	@echo "Running benchmarks..."
	go test $(BENCHMARK_FLAGS) ./...

.PHONY: test-setup
test-setup:
	@mkdir -p $(COVERAGE_DIR)

# Coverage reporting
.PHONY: coverage
coverage: test
	@echo "Generating coverage report..."
	go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	go tool cover -func=$(COVERAGE_DIR)/coverage.out

.PHONY: coverage-ci
coverage-ci: test
	@echo "Generating coverage report for CI..."
	go tool cover -func=$(COVERAGE_DIR)/coverage.out | grep total | awk '{print "Coverage: " $$3}'

# Build targets
.PHONY: build
build: build-setup
	@echo "Building $(APP_NAME) for $(GOOS)/$(GOARCH)..."
	go build $(BUILD_FLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) .

.PHONY: build-debug
build-debug: build-setup
	@echo "Building debug version..."
	go build -gcflags="all=-N -l" -o $(BUILD_DIR)/$(APP_NAME)-debug .

.PHONY: build-race
build-race: build-setup
	@echo "Building with race detection..."
	go build -race $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-race .

.PHONY: build-setup
build-setup:
	@mkdir -p $(BUILD_DIR)

# Cross-platform compilation
.PHONY: build-all
build-all: clean-dist
	@echo "Building for all platforms..."
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d/ -f1); \
		GOARCH=$$(echo $$platform | cut -d/ -f2); \
		output_name=$(APP_NAME)-$$GOOS-$$GOARCH; \
		if [ $$GOOS = "windows" ]; then output_name=$$output_name.exe; fi; \
		echo "Building for $$GOOS/$$GOARCH..."; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build $(BUILD_FLAGS) $(LDFLAGS) -o $(DIST_DIR)/$$output_name .; \
	done

# Installation targets
.PHONY: install
install: build
	@echo "Installing $(APP_NAME)..."
	go install $(BUILD_FLAGS) $(LDFLAGS) .

.PHONY: install-tools
install-tools:
	@echo "Installing development tools..."
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest

# Package and distribution
.PHONY: package
package: build-all
	@echo "Creating distribution packages..."
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d/ -f1); \
		GOARCH=$$(echo $$platform | cut -d/ -f2); \
		binary_name=$(APP_NAME)-$$GOOS-$$GOARCH; \
		if [ $$GOOS = "windows" ]; then binary_name=$$binary_name.exe; fi; \
		package_name=$(APP_NAME)-$(VERSION)-$$GOOS-$$GOARCH; \
		echo "Packaging $$package_name..."; \
		mkdir -p $(DIST_DIR)/$$package_name; \
		cp $(DIST_DIR)/$$binary_name $(DIST_DIR)/$$package_name/$(APP_NAME)$$(if [ $$GOOS = "windows" ]; then echo .exe; fi); \
		cp README.md LICENSE $(DIST_DIR)/$$package_name/; \
		if [ $$GOOS = "windows" ]; then \
			cd $(DIST_DIR) && zip -r $$package_name.zip $$package_name/; \
		else \
			cd $(DIST_DIR) && tar -czf $$package_name.tar.gz $$package_name/; \
		fi; \
		rm -rf $(DIST_DIR)/$$package_name; \
	done

.PHONY: checksums
checksums: package
	@echo "Generating checksums..."
	cd $(DIST_DIR) && sha256sum * > checksums.txt

# Documentation targets
.PHONY: docs
docs: docs-setup
	@echo "Generating documentation..."
	go doc -all . > $(DOCS_DIR)/api.txt
	./$(BUILD_DIR)/$(APP_NAME) --help > $(DOCS_DIR)/help.txt
	./$(BUILD_DIR)/$(APP_NAME) convert --help > $(DOCS_DIR)/convert-help.txt
	./$(BUILD_DIR)/$(APP_NAME) batch --help > $(DOCS_DIR)/batch-help.txt

.PHONY: docs-setup
docs-setup:
	@mkdir -p $(DOCS_DIR)

# Docker targets
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	docker build -t $(APP_NAME):$(VERSION) .
	docker build -t $(APP_NAME):latest .

.PHONY: docker-run
docker-run:
	@echo "Running Docker container..."
	docker run --rm -it $(APP_NAME):latest --help

.PHONY: docker-test
docker-test: docker-build
	@echo "Testing Docker image..."
	docker run --rm $(APP_NAME):latest --version

# Release targets
.PHONY: release-check
release-check: clean test lint security
	@echo "Release checks passed"

.PHONY: release-dry-run
release-dry-run: release-check build-all package checksums
	@echo "Release dry run completed"
	@echo "Version: $(VERSION)"
	@echo "Files:"
	@ls -la $(DIST_DIR)/

.PHONY: release
release: release-dry-run
	@echo "Creating release $(VERSION)..."
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)

# Development and debugging
.PHONY: run
run: build
	@echo "Running $(APP_NAME)..."
	./$(BUILD_DIR)/$(APP_NAME)

.PHONY: run-example
run-example: build
	@echo "Running example conversion..."
	./$(BUILD_DIR)/$(APP_NAME) convert --help

.PHONY: debug
debug: build-debug
	@echo "Starting debug session..."
	dlv exec $(BUILD_DIR)/$(APP_NAME)-debug

.PHONY: profile-cpu
profile-cpu: build
	@echo "Running CPU profiling..."
	./$(BUILD_DIR)/$(APP_NAME) -cpuprofile=cpu.prof convert --help
	go tool pprof cpu.prof

.PHONY: profile-mem
profile-mem: build
	@echo "Running memory profiling..."
	./$(BUILD_DIR)/$(APP_NAME) -memprofile=mem.prof convert --help
	go tool pprof mem.prof

# Maintenance and cleanup
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f *.prof
	go clean -cache
	go clean -testcache

.PHONY: clean-dist
clean-dist:
	@echo "Cleaning distribution artifacts..."
	rm -rf $(DIST_DIR)

.PHONY: clean-coverage
clean-coverage:
	@echo "Cleaning coverage reports..."
	rm -rf $(COVERAGE_DIR)

.PHONY: clean-all
clean-all: clean clean-dist clean-coverage
	@echo "Cleaning all artifacts..."
	rm -rf $(DOCS_DIR)

.PHONY: reset
reset: clean-all
	@echo "Resetting to clean state..."
	go clean -modcache

# CI/CD targets
.PHONY: ci-test
ci-test: deps test-short lint coverage-ci

.PHONY: ci-build
ci-build: deps build-all

.PHONY: ci-release
ci-release: deps release-check build-all package checksums

# Information and help
.PHONY: info
info:
	@echo "TileToJson Build Information"
	@echo "=========================="
	@echo "App Name:    $(APP_NAME)"
	@echo "Version:     $(VERSION)"
	@echo "Commit:      $(COMMIT)"
	@echo "Build Date:  $(BUILD_DATE)"
	@echo "Go Version:  $(GO_VERSION)"
	@echo "Platform:    $(GOOS)/$(GOARCH)"
	@echo ""
	@echo "Build Flags: $(BUILD_FLAGS)"
	@echo "LD Flags:    $(LDFLAGS)"

.PHONY: help
help:
	@echo "TileToJson Makefile Commands"
	@echo "============================"
	@echo ""
	@echo "Development:"
	@echo "  dev          - Full development build (deps, fmt, lint, test, build)"
	@echo "  quick        - Quick build (fmt, build)"
	@echo "  run          - Build and run the application"
	@echo "  run-example  - Build and run with example"
	@echo ""
	@echo "Dependencies:"
	@echo "  deps         - Download and verify dependencies"
	@echo "  deps-update  - Update all dependencies"
	@echo "  deps-vendor  - Vendor dependencies"
	@echo ""
	@echo "Code Quality:"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linters"
	@echo "  vet          - Run go vet"
	@echo "  security     - Run security analysis"
	@echo ""
	@echo "Testing:"
	@echo "  test         - Run all tests"
	@echo "  test-short   - Run short tests only"
	@echo "  test-integration - Run integration tests"
	@echo "  benchmark    - Run benchmarks"
	@echo "  coverage     - Generate coverage report"
	@echo ""
	@echo "Building:"
	@echo "  build        - Build for current platform"
	@echo "  build-debug  - Build debug version"
	@echo "  build-race   - Build with race detection"
	@echo "  build-all    - Build for all platforms"
	@echo ""
	@echo "Distribution:"
	@echo "  package      - Create distribution packages"
	@echo "  checksums    - Generate checksums"
	@echo "  install      - Install to GOPATH"
	@echo ""
	@echo "Docker:"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"
	@echo "  docker-test  - Test Docker image"
	@echo ""
	@echo "Release:"
	@echo "  release-check - Run release checks"
	@echo "  release-dry-run - Perform release dry run"
	@echo "  release      - Create and tag release"
	@echo ""
	@echo "Debugging:"
	@echo "  debug        - Start debug session"
	@echo "  profile-cpu  - Run CPU profiling"
	@echo "  profile-mem  - Run memory profiling"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean        - Clean build artifacts"
	@echo "  clean-all    - Clean all artifacts"
	@echo "  reset        - Reset to clean state"
	@echo ""
	@echo "CI/CD:"
	@echo "  ci-test      - CI test pipeline"
	@echo "  ci-build     - CI build pipeline"
	@echo "  ci-release   - CI release pipeline"
	@echo ""
	@echo "Information:"
	@echo "  info         - Show build information"
	@echo "  help         - Show this help message"

# Ensure directories exist
$(BUILD_DIR) $(DIST_DIR) $(COVERAGE_DIR) $(DOCS_DIR):
	@mkdir -p $@
