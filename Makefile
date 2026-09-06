# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Deb build info
RELEASE_VERSION ?= 2.0.0
ARCH ?= amd64
PACKAGE_NAME = raven
BUILD_DIR = build
DEB_DIR = $(BUILD_DIR)/$(PACKAGE_NAME)_$(RELEASE_VERSION)_$(ARCH)

# Enhanced build information
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "$(RELEASE_VERSION)")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
GO_VERSION := $(shell go version | cut -d' ' -f3)

# Package for build info injection
PKG := raven2/internal/web

# Enhanced LDFLAGS with build info for web interface
LDFLAGS := -X main.Version=$(VERSION) \
           -X main.BuildTime=$(BUILD_TIME) \
           -X main.Commit=$(COMMIT) \
           -X '$(PKG).Version=$(VERSION)' \
           -X '$(PKG).GitCommit=$(COMMIT)' \
           -X '$(PKG).GitBranch=$(GIT_BRANCH)' \
           -X '$(PKG).BuildTime=$(BUILD_TIME)' \
           -X '$(PKG).BuildFlags=-trimpath'

# Build flags
GO_BUILD_FLAGS := -ldflags="$(LDFLAGS)" -trimpath

# Makefile for building and deployment
.PHONY: build run test clean docker deploy info help

all: build discover

# Build main program with enhanced build info
build:
	@echo "Building Raven v$(VERSION)..."
	@echo "  Git Commit: $(COMMIT)"
	@echo "  Git Branch: $(GIT_BRANCH)" 
	@echo "  Build Time: $(BUILD_TIME)"
	@echo "  Go Version: $(GO_VERSION)"
	@mkdir -p bin
	CGO_ENABLED=1 $(GOCMD) build $(GO_BUILD_FLAGS) -o bin/raven ./cmd/raven

# Build the discovery utility
discover:
	@echo "Building Raven Discovery..."
	@mkdir -p bin
	CGO_ENABLED=1 $(GOCMD) build $(GO_BUILD_FLAGS) -o bin/raven-discover ./cmd/raven-discover

# Build for development (with race detector)
dev-build:
	@echo "Building Raven for development (with race detector)..."
	@mkdir -p bin
	CGO_ENABLED=1 $(GOCMD) build $(GO_BUILD_FLAGS) -race -o bin/raven-dev ./cmd/raven

# Build for multiple platforms
build-all: build-linux build-windows build-darwin

build-linux:
	@echo "Building for Linux..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOCMD) build $(GO_BUILD_FLAGS) -o bin/raven-linux-amd64 ./cmd/raven
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOCMD) build $(GO_BUILD_FLAGS) -o bin/raven-linux-arm64 ./cmd/raven
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOCMD) build $(GO_BUILD_FLAGS) -o bin/raven-discover-linux-amd64 ./cmd/raven-discover
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOCMD) build $(GO_BUILD_FLAGS) -o bin/raven-discover-linux-arm64 ./cmd/raven-discover

build-windows:
	@echo "Building for Windows..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOCMD) build $(GO_BUILD_FLAGS) -o bin/raven-windows-amd64.exe ./cmd/raven
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOCMD) build $(GO_BUILD_FLAGS) -o bin/raven-discover-windows-amd64.exe ./cmd/raven-discover

build-darwin:
	@echo "Building for macOS..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOCMD) build $(GO_BUILD_FLAGS) -o bin/raven-darwin-amd64 ./cmd/raven
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOCMD) build $(GO_BUILD_FLAGS) -o bin/raven-darwin-arm64 ./cmd/raven
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOCMD) build $(GO_BUILD_FLAGS) -o bin/raven-discover-darwin-amd64 ./cmd/raven-discover
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOCMD) build $(GO_BUILD_FLAGS) -o bin/raven-discover-darwin-arm64 ./cmd/raven-discover

run: build
	./bin/raven -config config.yaml

test:
	go test -v ./...

clean: clean-discover clean-deb
	rm -rf bin/ data/

docker:
	docker build -t raven:$(RELEASE_VERSION) .
	docker tag raven:$(RELEASE_VERSION) raven:latest

deploy: docker
	docker-compose up -d

dev: dev-build
	./bin/raven-dev -config config.yaml

install-deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

# Clean up discovery binary
clean-discover:
	rm -f bin/raven-discover*

format:
	go fmt ./...
	go vet ./...

lint:
	golangci-lint run

migrate:
	@echo "Running database migrations..."
	./bin/raven -config config.yaml -migrate

backup:
	@echo "Creating database backup..."
	cp data/raven.db data/raven-backup-$(shell date +%Y%m%d-%H%M%S).db

# Development setup
setup-dev:
	@echo "Setting up development environment..."
	mkdir -p data plugins
	go mod download
	@echo "Development environment ready!"

# Show build information
info:
	@echo "🐦 Raven Network Monitoring - Build Information"
	@echo "=============================================="
	@echo "Version:       $(VERSION)"
	@echo "Git Commit:    $(COMMIT)"
	@echo "Git Branch:    $(GIT_BRANCH)"
	@echo "Build Time:    $(BUILD_TIME)"
	@echo "Go Version:    $(GO_VERSION)"
	@echo "Package:       $(PKG)"
	@echo ""
	@echo "LDFLAGS:       $(LDFLAGS)"
	@echo ""
	@echo "Build Targets:"
	@echo "  Local:       make build"
	@echo "  All Platforms: make build-all"
	@echo "  Development: make dev-build"
	@echo "  Debian Package: make deb"

# Create release package
package: build discover
	@echo "Creating release package..."
	@mkdir -p $(BUILD_DIR)/release
	@cp bin/raven $(BUILD_DIR)/release/
	@cp bin/raven-discover $(BUILD_DIR)/release/
	@if [ -d "web" ]; then cp -r web $(BUILD_DIR)/release/; fi
	@if [ -f "README.md" ]; then cp README.md $(BUILD_DIR)/release/; fi
	@if [ -f "LICENSE" ]; then cp LICENSE $(BUILD_DIR)/release/; fi
	@if [ -f "config.example.yaml" ]; then cp config.example.yaml $(BUILD_DIR)/release/config.yaml; fi
	@cd $(BUILD_DIR) && tar -czf raven-$(VERSION).tar.gz release/
	@echo "Release package created: $(BUILD_DIR)/raven-$(VERSION).tar.gz"

# Build Debian package (enhanced with build info)
.PHONY: deb
deb: build discover
	@echo "Building Debian package v$(RELEASE_VERSION) with build info..."
	@echo "  Version: $(VERSION)"
	@echo "  Commit:  $(COMMIT)"
	@echo "  Branch:  $(GIT_BRANCH)"
	@rm -rf $(BUILD_DIR)
	@mkdir -p $(DEB_DIR)/DEBIAN
	@mkdir -p $(DEB_DIR)/usr/bin
	@mkdir -p $(DEB_DIR)/usr/lib/raven
	@mkdir -p $(DEB_DIR)/etc/raven
	@mkdir -p $(DEB_DIR)/etc/systemd/system
	@mkdir -p $(DEB_DIR)/var/lib/raven/data
	@mkdir -p $(DEB_DIR)/var/log/raven
	@mkdir -p $(DEB_DIR)/usr/share/doc/raven

	# Copy binaries
	@cp bin/raven $(DEB_DIR)/usr/bin/
	@cp bin/raven-discover $(DEB_DIR)/usr/bin/
	@chmod 755 $(DEB_DIR)/usr/bin/raven
	@chmod 755 $(DEB_DIR)/usr/bin/raven-discover

	# Copy web assets
	@cp -r web $(DEB_DIR)/usr/lib/raven/

	# Copy Debian control files
	@cp debian/DEBIAN/control $(DEB_DIR)/DEBIAN/
	@cp debian/DEBIAN/postinst $(DEB_DIR)/DEBIAN/
	@cp debian/DEBIAN/prerm $(DEB_DIR)/DEBIAN/
	@cp debian/DEBIAN/postrm $(DEB_DIR)/DEBIAN/
	@cp debian/DEBIAN/conffiles $(DEB_DIR)/DEBIAN/
	@chmod 755 $(DEB_DIR)/DEBIAN/postinst
	@chmod 755 $(DEB_DIR)/DEBIAN/prerm
	@chmod 755 $(DEB_DIR)/DEBIAN/postrm
	@chmod 644 $(DEB_DIR)/DEBIAN/conffiles

	# Copy systemd service
	@cp config/raven.service $(DEB_DIR)/etc/systemd/system/

	# Create default config
	@cp debian/etc/raven/config.yaml $(DEB_DIR)/etc/raven/

	# Copy documentation files
	@mkdir -p $(DEB_DIR)/usr/share/doc/raven/examples
	@cp README.md $(DEB_DIR)/usr/share/doc/raven/
	@cp docs/Configuration.md $(DEB_DIR)/usr/share/doc/raven/
	@cp config/alert-rules.yaml $(DEB_DIR)/usr/share/doc/raven/examples/
	@cp config/prometheus.yml $(DEB_DIR)/usr/share/doc/raven/examples/
	
	# Create enhanced installation summary with build info
	@echo "Raven Network Monitoring System v$(VERSION)" > $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "=========================================" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "Build Information:" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "  Version:           $(VERSION)" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "  Git Commit:        $(COMMIT)" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "  Git Branch:        $(GIT_BRANCH)" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "  Build Time:        $(BUILD_TIME)" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "  Go Version:        $(GO_VERSION)" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "Installation Summary:" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "  Configuration:     /etc/raven/config.yaml" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "  Data Directory:    /var/lib/raven/" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "  Web Interface:     http://localhost:8000" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "  Service Control:   systemctl start/stop/status raven" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "  Logs:              journalctl -u raven -f" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "Quick Start:" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "  1. sudo raven-discover -network 192.168.1.0/24 -output /tmp/config.yaml" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "  2. sudo cp /tmp/config.yaml /etc/raven/config.yaml" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "  3. sudo systemctl start raven" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "Documentation:" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "  Main README:       /usr/share/doc/raven/README.md" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "  Configuration:     /usr/share/doc/raven/Configuration.md" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "  Examples:          /usr/share/doc/raven/examples/" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL
	@echo "  Build Info:        Available in web interface under About tab" >> $(DEB_DIR)/usr/share/doc/raven/INSTALL

	# Build the package
	@dpkg-deb --build $(DEB_DIR)
	@echo "Debian package created: $(DEB_DIR).deb"
	@echo "Build info embedded for web interface About tab"

# Install the built deb package
.PHONY: install-deb
install-deb: deb
	@echo "Installing Raven Debian package..."
	@sudo dpkg -i $(DEB_DIR).deb

# Clean up build artifacts
.PHONY: clean-deb
clean-deb:
	@rm -rf $(BUILD_DIR)

# Create directory structure for development
.PHONY: setup-debian
setup-debian:
	@mkdir -p debian/DEBIAN
	@mkdir -p debian/etc/raven
	@mkdir -p debian/etc/systemd/system

# Help target
help:
	@echo "🐦 Raven Network Monitoring - Build System"
	@echo "=========================================="
	@echo ""
	@echo "Build Targets:"
	@echo "  build          Build main raven binary"
	@echo "  discover       Build raven-discover utility"
	@echo "  build-all      Build for all platforms (Linux, Windows, macOS)"
	@echo "  dev-build      Build development version with race detector"
	@echo "  package        Create release package (.tar.gz)"
	@echo ""
	@echo "Platform-Specific Builds:"
	@echo "  build-linux    Build for Linux (amd64, arm64)"
	@echo "  build-windows  Build for Windows (amd64)"
	@echo "  build-darwin   Build for macOS (amd64, arm64)"
	@echo ""
	@echo "Development:"
	@echo "  dev            Build and run development version"
	@echo "  run            Build and run with config.yaml"
	@echo "  test           Run all tests"
	@echo "  format         Format and vet code"
	@echo "  lint           Run linter"
	@echo ""
	@echo "Packaging:"
	@echo "  deb            Build Debian package"
	@echo "  install-deb    Build and install Debian package"
	@echo "  docker         Build Docker image"
	@echo "  deploy         Deploy with Docker Compose"
	@echo ""
	@echo "Database:"
	@echo "  migrate        Run database migrations"
	@echo "  backup         Create database backup"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean          Remove build artifacts"
	@echo "  install-deps   Install Go dependencies"
	@echo "  setup-dev      Setup development environment"
	@echo ""
	@echo "Information:"
	@echo "  info           Show build information"
	@echo "  help           Show this help message"
	@echo ""
	@echo "Environment Variables:"
	@echo "  RELEASE_VERSION  Override release version (default: 2.0.0)"
	@echo "  ARCH            Target architecture (default: amd64)"
	@echo ""
	@echo "Examples:"
	@echo "  make build"
	@echo "  make RELEASE_VERSION=2.1.0 deb"
	@echo "  make build-all"
