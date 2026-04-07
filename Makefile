BINARY     := nut-webgui
MODULE     := github.com/ospfx/nut_webgui
BUILD_DIR  := bin
MAIN       := ./cmd/nut-webgui

GO         := go
GOFLAGS    :=
LDFLAGS    := -s -w

.PHONY: all build test vet lint clean run help

all: build

## build: Compile the binary into ./bin/
build:
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(MAIN)
	@echo "Built $(BUILD_DIR)/$(BINARY)"

## test: Run all tests
test:
	$(GO) test ./... -timeout 60s

## test-verbose: Run tests with verbose output
test-verbose:
	$(GO) test ./... -v -timeout 60s

## vet: Run go vet
vet:
	$(GO) vet ./...

## lint: Run golangci-lint (requires golangci-lint in PATH)
lint:
	golangci-lint run ./...

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)

## run: Build and run locally (requires UPSD_ADDR to be set)
run: build
	$(BUILD_DIR)/$(BINARY)

## docker-build: Build a Docker image
docker-build:
	docker build -t nut-webgui:latest .

## help: Show available targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
