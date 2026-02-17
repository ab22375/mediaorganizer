# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added
- **Video/audio metadata via ffprobe**: Shells out to `ffprobe` for real `creation_time` tags from video/audio containers (MP4, MOV, MKV, etc.). Falls back to file modification time if ffprobe is not installed
- **Destination collision protection**: Mover checks if dest file exists before writing; `copyFileImpl` uses `O_EXCL` flag to prevent silent overwrites
- **Copy integrity verification**: Verifies written byte count matches source size after copy to catch partial writes
- **Consistent sequence numbering**: When multiple files share a timestamp, all get sequence suffixes (`_001`, `_002`, ...). Previously the first file had no suffix, creating an inconsistent gap
- **Dest-index hash persistence**: Pre-indexed destination files retain their computed hashes across runs, avoiding re-hashing gigabytes of already-organized files on every scan

### Changed
- **xxHash replaces SHA-256**: Switched from `crypto/sha256` to `github.com/cespare/xxhash/v2` (XXH64) for file dedup hashing. 5-10x faster on large media files with equivalent collision resistance for dedup
- **`filepath.WalkDir` replaces `filepath.Walk`**: All directory walks (walker, pre-index, cleanup) use `WalkDir` to avoid an extra `Stat` syscall per entry. Significant speedup on large directory trees
- **Symlink safety**: `WalkDir` does not follow symlinks to directories. Files that are symlinks are explicitly skipped to prevent infinite loops and double-processing
- **Backfill hashing safety**: No longer falls back to reading dest files during backfill hashing; skips files whose source is missing to avoid racing with concurrent mover goroutines
- **Atomic TotalFiles counter**: Fixed data race where walker goroutine incremented `TotalFiles` non-atomically while the progress reporter read it from the main goroutine
- **Timestamp preservation on copy**: `copyFileImpl` now preserves source file modification time via `os.Chtimes`
- **`ExtractFileMetadata` avoids double open**: Uses `os.Stat()` instead of `os.Open()`+`file.Stat()` for the initial stat, eliminating a redundant file descriptor
- **Journal helpers cleaned up**: Replaced custom `contains`/`searchString` functions with `strings.Contains`
- **Stats exclude dest_index**: `Stats()` and `TotalCount()` queries now exclude `dest_index` rows for accurate reporting
- **`ClearDestIndex` preserves hashed rows**: Only removes dest_index rows without a computed hash; rows with hashes are kept for the next run

### Previous (pre-changelog)
- **Linux cross-compilation**: Verified fully cross-platform (pure Go, no CGo). Built and deployed to Linux (PopOS) via `GOOS=linux GOARCH=amd64 go build`
- **Cross-scan duplicate detection**: Pre-indexes existing files in destination directories before processing, so duplicates from previous scans with different source directories are detected
- **SQLite journal** (`pkg/db/journal.go`): All file operations tracked in a SQLite database for resume support and global deduplication
- **Streaming pipeline**: Files start moving as soon as metadata is extracted, no more waiting for full scan
- **Lazy hashing**: Only computed when another file with the same `file_size` exists in DB
- **Global duplicate detection**: Dedup across all files via hash matching (not just same-timestamp groups)
- **Resume support**: Interrupted operations resume automatically from the journal
- **Signal handling**: SIGINT/SIGTERM gracefully closes the journal and logs resume instructions

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
