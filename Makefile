.PHONY: build clean test run

# Binary name
BINARY_NAME=mediaorganizer

# Build the application
build:
	go build -o $(BINARY_NAME) main.go

# Clean build artifacts
clean:
	go clean
	rm -f $(BINARY_NAME)

# Run tests
test:
	go test -v ./...

# Run the application
run:
	go run main.go

# Run with config file
run-with-config:
	go run main.go --config config.yaml

# Run in dry-run mode
dry-run:
	go run main.go --dry-run