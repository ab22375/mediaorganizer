# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

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
