# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added
- **Cross-scan duplicate detection**: Pre-indexes existing files in destination directories before processing, so duplicates from previous scans with different source directories are detected
  - New `dest_index` file status in journal for pre-indexed destination files
  - `preIndexDestinations()` walks all configured destination directories at scan start
  - Batch inserts destination files in a single SQLite transaction (`INSERT OR IGNORE`)
  - Skips source directory during destination walk to avoid self-indexing
  - Cleans up `dest_index` rows before computing final stats
- **SQLite journal** (`pkg/db/journal.go`): All file operations tracked in a SQLite database for resume support and global deduplication
  - Uses `modernc.org/sqlite` (pure Go, no CGo)
  - WAL mode for concurrent read performance
  - 14 unit tests in `pkg/db/journal_test.go`
- **Streaming pipeline**: Files start moving as soon as metadata is extracted, no more waiting for full scan
  - Walker goroutine → N metadata workers → single organizer goroutine → N mover workers
  - Organizer serializes all DB writes (no concurrent write contention)
- **Lazy hashing**: SHA-256 only computed when another file with the same `file_size` exists in DB
  - Backfills unhashed files when a second same-size file appears
- **Global duplicate detection**: Dedup across all files via hash matching (not just same-timestamp groups)
- **Resume support**: Interrupted operations resume automatically from the journal
  - Completed files are skipped on re-run
  - Failed files are retried
  - Pending files with assigned destinations are re-queued
- **`--db` flag**: Custom path to SQLite journal database (default: `<source>/.mediaorganizer.db`)
- **`--fresh` flag**: Force a fresh start, ignoring existing database
- **Signal handling**: SIGINT/SIGTERM gracefully closes the journal and logs resume instructions
- **Progress reporting**: Updated ticker shows `Scanned: X/Y | Organized: Z`

### Changed
- **Lazy hashing backfill**: Always backfills unhashed same-size files (was previously limited to the 1-to-2 transition). Required for correct cross-scan dedup when destination has multiple same-size files.
- `pkg/processor/scanner.go`: Complete rewrite from two-phase (scan-all-then-organize) to streaming pipeline
- `pkg/media/metadata.go`: `ExtractFileMetadata()` no longer computes hash; `ComputeFileHash()` remains as standalone function
- `pkg/config/config.go`: Added `DBPath` and `Fresh` fields with CLI flag support
- `main.go`: Wired up journal initialization, resume detection, signal handling
- `.gitignore`: Added `*.db`, `*.db-wal`, `*.db-shm`
- `config-example.yaml`: Documented `db_path` and `fresh` options

## [1.1.0] - Organization Schemes

### Added
- **Organization schemes**: Two ways to structure output directories
  - `extension_first` (default): `<dest>/<ext>/YYYY/YYYY-MM/YYYY-MM-DD/`
  - `date_first`: `<dest>/YYYY/YYYY-MM/YYYY-MM-DD/<ext>/`
- **Unified destination** (`--dest` flag): Single output directory for `date_first` scheme
- **`--scheme` flag**: Select organization scheme from CLI
- **`--space-replace` flag**: Replace spaces in filenames (default: `_`)
- **`--no-original-name` flag**: Discard original filename, use only timestamp and dimension
- **Config options**: `organization_scheme`, `destination`, `space_replacement` in YAML/JSON config
- Unit tests for organization schemes in `pkg/media/types_test.go`
- Unit tests for scheme validation in `pkg/config/config_test.go`

### Changed
- `date_first` scheme uses underscore-based filenames: `YYYYMMDD-HHMMSS_dim_name.ext`
- `extension_first` scheme uses parentheses format: `YYYYMMDD-HHMMSS_dim (name).ext`
- Updated documentation (README.md, CLAUDE.md, config-example.yaml)

## [1.0.0] - Initial Release

### Features
- Recursive media file scanning
- EXIF metadata extraction for creation dates
- Concurrent processing with configurable worker pool
- Support for images (JPEG, PNG, RAW formats), videos (MP4, MOV, etc.), and audio (MP3, FLAC, etc.)
- Dry-run mode for previewing changes
- Copy or move file operations
- Duplicate handling with sequence numbers
- Extension-specific destination directories
- Empty directory cleanup after moving files
- YAML/JSON configuration file support
- CLI flags with config file override
