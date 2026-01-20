# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run Commands

```bash
# Build the binary
make build

# Run tests
make test

# Run the application
make run

# Run with a config file
make run-with-config

# Run in dry-run mode (preview changes)
make dry-run

# Clean build artifacts
make clean
```

## Architecture Overview

Media Organizer is a Go utility that organizes media files (images, videos, audio) by extracting creation dates from metadata and organizing them into a structured directory hierarchy.

### Package Structure

- **main.go** - Entry point, loads config and starts the scanner
- **pkg/config** - Configuration loading via CLI flags (spf13/pflag) and YAML/JSON files (spf13/viper)
- **pkg/media** - Media type definitions and metadata extraction (EXIF for images via rwcarlsen/goexif)
- **pkg/processor** - Concurrent file scanner and organizer with worker pool pattern
- **pkg/utils** - Progress reporting utilities

### Key Processing Flow

1. `config.LoadConfig()` merges CLI flags with optional config file (CLI takes precedence)
2. `processor.NewMediaScanner()` creates a scanner with configurable concurrency
3. `scanner.Scan()` walks source directory, queues media files to workers
4. Workers extract metadata via `media.ExtractFileMetadata()` and build a map keyed by timestamp+type+extension
5. `organizeFiles()` processes the map, handles duplicates with sequence numbers, and moves/copies files

### Output Structure

Files are organized as: `<dest>/<extension>/YYYY/YYYY-MM/YYYY-MM-DD/YYYYMMDD-HHMMSS_<dimension> (<original_name>).<ext>`

Duplicates (same timestamp) get sequence suffixes (`_001`, `_002`) and can be routed to a `duplicates/` subfolder.

### Configuration Priority

CLI flags override config file values. Extension-specific destinations (`extension_destinations` in config) override media type destinations.
