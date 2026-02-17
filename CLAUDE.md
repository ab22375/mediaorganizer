# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run Commands

```bash
# Build the binary (injects version from git tag via ldflags)
make build

# Build with explicit version
VERSION=v1.2.0 make build

# Cross-compile for Linux (manual ldflags needed)
VERSION=$(git describe --tags --always --dirty) GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$VERSION" -o mediaorganizer-linux .

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

- **main.go** - Entry point, defines `var version` (set via `-ldflags`), loads config, initializes SQLite journal, wires up signal handling, starts scanner
- **pkg/config** - Configuration loading via CLI flags (spf13/pflag) and YAML/JSON files (spf13/viper). `LoadConfig(version string)` accepts version for `--help`/`--version` display. Custom `pflag.Usage` groups flags into logical sections.
- **pkg/db** - SQLite journal layer (modernc.org/sqlite, pure Go) for tracking file operations, resume, and dedup
- **pkg/media** - Media type definitions and metadata extraction (EXIF for images via rwcarlsen/goexif, ffprobe for video/audio, xxHash for dedup)
- **pkg/processor** - Streaming pipeline scanner with concurrent workers
- **pkg/utils** - Progress reporting utilities

### Streaming Pipeline (pkg/processor/scanner.go)

The scanner uses a 4-stage streaming pipeline:

```
walker goroutine → metadata workers (N) → organizer goroutine → mover workers (N)
```

1. **Walker**: Uses `filepath.WalkDir` (not Walk) to walk source directory, skips symlinks and completed files (resume mode), sends paths to channel
2. **Metadata workers** (N goroutines): Extract metadata via `media.ExtractFileMetadata()` — no hashing at this stage
3. **Organizer** (single goroutine): Serializes all SQLite writes
   - Inserts file records into journal
   - Lazy hashing: xxHash only when `CountByFileSize > 1`, backfills earlier same-size files (source paths only, never dest paths mid-pipeline)
   - Global dedup: `GetByHash()` to detect duplicates across all files
   - Sequence numbering: `CountByTimestampKey()` for files sharing a timestamp
   - Computes destination path, updates journal, sends move jobs
4. **Mover workers** (N goroutines): Checks dest collision → MkdirAll → move/copy file (with O_EXCL) → verify size → preserve timestamps → update journal status

### SQLite Journal (pkg/db/journal.go)

- Database default location: `<source>/.mediaorganizer.db`
- Pragmas: WAL mode, synchronous=NORMAL, busy_timeout=5000
- Single `files` table with indexes on status, file_size, hash, timestamp_key
- FileStatus values: `pending`, `completed`, `failed`, `dry_run`, `dest_index`
- Resume: `GetCompletedSourcePaths()` to skip, `GetPendingFiles()` to re-queue, `ResetFailed()` for retry
- Destination pre-index: `InsertDestFiles()` batch inserts, `ClearDestIndex()` removes only unhashed rows (hashed rows persist across runs)
- Helper: `GetFirstByTimestampKey()` for retroactive sequence renaming

### Key Design Decisions

- **Organizer is single-goroutine**: No concurrent SQLite write contention. All DB mutations happen here.
- **Lazy hashing**: Avoids expensive hashing for unique-size files. Only hashes when size collision detected. Backfill runs for ALL unhashed same-size files but only reads source paths (never dest paths mid-pipeline to avoid race with movers).
- **xxHash (XXH64) for dedup**: Non-cryptographic, 5-10x faster than SHA-256. Sufficient collision resistance for dedup. Output is 16-char hex string.
- **Hash is empty by default**: `ExtractFileMetadata()` does NOT compute hash. The organizer calls `media.ComputeFileHash()` only when needed.
- **Duplicate detection is global**: Any two files anywhere with matching hash are duplicates (not just same-timestamp groups).
- **Cross-scan dedup via destination pre-indexing**: `preIndexDestinations()` runs before the pipeline, walking all destination dirs and inserting existing files as `dest_index` rows (path + size only, no file reads). Hashed rows persist across runs to avoid re-hashing. The existing `CountByFileSize`/`GetByHash` logic naturally picks them up.
- **Sequence numbering consistency**: When a second file shares a timestamp key, `retroFixFirstSequence()` retroactively renames the first file to `_001` (including on disk if already moved).
- **Symlink safety**: All `WalkDir` calls skip symlinks. No risk of infinite loops from cyclic symlinks.
- **Dest collision protection**: Mover checks `os.Stat` before writing; `copyFileImpl` uses `O_EXCL`. Copy also verifies written size matches source and preserves mtime.
- **ffprobe for video/audio**: `extractCreationTimeViaFFprobe()` gets real creation dates from container metadata. Graceful fallback to ModTime if ffprobe absent.
- **Atomic counters**: `totalFiles`, `processed`, `organized` are all `int32` with `sync/atomic` — safe for concurrent goroutine access.

### Output Structure

The output structure depends on the selected organization scheme (`--scheme` flag or `organization_scheme` config):

**extension_first (default):**
Uses per-media-type destinations (`--image-dest`, `--video-dest`, `--audio-dest`):
`<dest>/<extension>/YYYY/YYYY-MM/YYYY-MM-DD/YYYYMMDD-HHMMSS_<dimension> (<original_name>).<ext>`

**date_first:**
Uses a unified destination (`--dest`), all media types in one directory:
`<dest>/YYYY/YYYY-MM/YYYY-MM-DD/<extension>/YYYYMMDD-HHMMSS_<dimension>_<original_name>.<ext>`

Duplicates (same hash) get sequence suffixes (`_001`, `_002`, `_003`) and can be routed to a `duplicates/` subfolder. When multiple files share a timestamp, all receive sequence suffixes for consistency.

### Configuration Priority

### Versioning

- `var version = "dev"` in `main.go`, overridden at build time via `-ldflags "-X main.version=$(VERSION)"`
- `VERSION` in Makefile defaults to `git describe --tags --always --dirty` (tag > commit hash > "dev")
- To release: `git tag v1.2.0 && make build`

### Configuration Priority

CLI flags override config file values. Extension-specific destinations (`extension_destinations` in config) override media type destinations. Default DB path is `<source>/.mediaorganizer.db` unless `--db` is specified.
