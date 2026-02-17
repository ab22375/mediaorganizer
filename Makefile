.PHONY: build clean test run

# Binary name
BINARY_NAME=mediaorganizer

# Version from git tag, fallback to short commit hash
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

# Build the application
build:
	go build $(LDFLAGS) -o $(BINARY_NAME) main.go

# Clean build artifacts
clean:
	go clean
	rm -f $(BINARY_NAME)

# Run tests
test:
	go test -v ./...

# Run the application
run:
	go run $(LDFLAGS) main.go

# Run with config file
run-with-config:
	go run $(LDFLAGS) main.go --config config.yaml

# Run in dry-run mode
dry-run:
	go run $(LDFLAGS) main.go --dry-run
