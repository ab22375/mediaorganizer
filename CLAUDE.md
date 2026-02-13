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

- **main.go** - Entry point, loads config, initializes SQLite journal, wires up signal handling, starts scanner
- **pkg/config** - Configuration loading via CLI flags (spf13/pflag) and YAML/JSON files (spf13/viper)
- **pkg/db** - SQLite journal layer (modernc.org/sqlite, pure Go) for tracking file operations, resume, and dedup
- **pkg/media** - Media type definitions and metadata extraction (EXIF for images via rwcarlsen/goexif)
- **pkg/processor** - Streaming pipeline scanner with concurrent workers
- **pkg/utils** - Progress reporting utilities

### Streaming Pipeline (pkg/processor/scanner.go)

The scanner uses a 4-stage streaming pipeline:

```
walker goroutine → metadata workers (N) → organizer goroutine → mover workers (N)
```

1. **Walker**: Walks source directory, skips completed files (resume mode), sends paths to channel
2. **Metadata workers** (N goroutines): Extract metadata via `media.ExtractFileMetadata()` — no hashing at this stage
3. **Organizer** (single goroutine): Serializes all SQLite writes
   - Inserts file records into journal
   - Lazy hashing: SHA-256 only when `CountByFileSize > 1`, backfills earlier same-size files
   - Global dedup: `GetByHash()` to detect duplicates across all files
   - Sequence numbering: `CountByTimestampKey()` for files sharing a timestamp
   - Computes destination path, updates journal, sends move jobs
4. **Mover workers** (N goroutines): MkdirAll + move/copy file, update journal status

### SQLite Journal (pkg/db/journal.go)

- Database default location: `<source>/.mediaorganizer.db`
- Pragmas: WAL mode, synchronous=NORMAL, busy_timeout=5000
- Single `files` table with indexes on status, file_size, hash, timestamp_key
- FileStatus values: `pending`, `completed`, `failed`, `dry_run`, `dest_index`
- Resume: `GetCompletedSourcePaths()` to skip, `GetPendingFiles()` to re-queue, `ResetFailed()` for retry
- Destination pre-index: `InsertDestFiles()` batch inserts, `ClearDestIndex()` cleans up

### Key Design Decisions

- **Organizer is single-goroutine**: No concurrent SQLite write contention. All DB mutations happen here.
- **Lazy hashing**: Avoids expensive SHA-256 for unique-size files. Only hashes when size collision detected. Backfill runs for ALL unhashed same-size files (not just on the 1-to-2 transition) to support destination pre-indexed files.
- **Hash is empty by default**: `ExtractFileMetadata()` does NOT compute hash. The organizer calls `media.ComputeFileHash()` only when needed.
- **Duplicate detection is global**: Any two files anywhere with matching SHA-256 hash are duplicates (not just same-timestamp groups).
- **Cross-scan dedup via destination pre-indexing**: `preIndexDestinations()` runs before the pipeline, walking all destination dirs and inserting existing files as `dest_index` rows (path + size only, no file reads). The existing `CountByFileSize`/`GetByHash` logic naturally picks them up. Rows are cleaned up before final stats.

### Output Structure

The output structure depends on the selected organization scheme (`--scheme` flag or `organization_scheme` config):

**extension_first (default):**
Uses per-media-type destinations (`--image-dest`, `--video-dest`, `--audio-dest`):
`<dest>/<extension>/YYYY/YYYY-MM/YYYY-MM-DD/YYYYMMDD-HHMMSS_<dimension> (<original_name>).<ext>`

**date_first:**
Uses a unified destination (`--dest`), all media types in one directory:
`<dest>/YYYY/YYYY-MM/YYYY-MM-DD/<extension>/YYYYMMDD-HHMMSS_<dimension>_<original_name>.<ext>`

Duplicates (same hash) get sequence suffixes (`_002`, `_003`) and can be routed to a `duplicates/` subfolder.

### Configuration Priority

CLI flags override config file values. Extension-specific destinations (`extension_destinations` in config) override media type destinations. Default DB path is `<source>/.mediaorganizer.db` unless `--db` is specified.
