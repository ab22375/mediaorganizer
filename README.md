q# Media Organizer

A Golang utility for organizing media files (images, videos, audio) by creation date.

## Features

- **Streaming pipeline**: Files start moving as soon as metadata is extracted (no waiting for full scan)
- **SQLite journal**: All operations tracked for automatic resume on interruption
- **Global deduplication**: SHA-256 hash-based duplicate detection across all files (not just same-timestamp groups)
- **Cross-scan deduplication**: Pre-indexes existing destination files at startup, detecting duplicates even when scanning from different source directories
- **Lazy hashing**: Only computes file hashes when two files share the same size, minimizing I/O
- Recursively scans source directories for media files
- Extracts creation dates from EXIF metadata, file metadata, or fallback to file creation date
- Organizes files into a structured directory hierarchy based on dates
- Renames files with creation timestamp and resolution information
- Handles duplicate files with sequence numbering
- Extension-specific organization (organize by file extension or specify custom paths for specific extensions)
- Configurable via command line flags or configuration file
- Support for dry-run mode to preview changes
- Concurrent processing for improved performance

## Supported Media Types

### Images
- JPEG (.jpg, .jpeg)
- PNG (.png)
- GIF (.gif)
- BMP (.bmp)
- WEBP (.webp)
- TIFF (.tif, .tiff)
- RAW formats (.nef, .arw, .cr2, .cr3, .dng)

### Videos
- MP4 (.mp4)
- AVI (.avi)
- MOV (.mov)
- MKV (.mkv)
- WMV (.wmv)
- FLV (.flv)
- WEBM (.webm)
- M4V (.m4v)

### Audio
- MP3 (.mp3)
- WAV (.wav)
- AAC (.aac)
- OGG (.ogg)
- FLAC (.flac)
- M4A (.m4a)
- WMA (.wma)

## Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/media-organizer.git
cd media-organizer

# Build the application
go build
```

## Usage

```bash
# Basic usage with source and destination directories
./mediaorganizer --source /path/to/media/files --image-dest /path/to/images --video-dest /path/to/videos --audio-dest /path/to/audio

# Using dry-run mode to preview changes without moving files
./mediaorganizer --source /path/to/media/files --dry-run

# Copy files instead of moving them
./mediaorganizer --source /path/to/media/files --copy

# Using a configuration file
./mediaorganizer --config config.yaml

# With verbose logging
./mediaorganizer --source /path/to/media/files --verbose

# Setting number of concurrent jobs
./mediaorganizer --source /path/to/media/files --jobs 8

# Delete empty directories after moving files
./mediaorganizer --source /path/to/media/files --delete-empty-dirs

# Resume an interrupted run (just re-run the same command)
./mediaorganizer --source /path/to/media/files

# Force a fresh start, ignoring previous journal
./mediaorganizer --source /path/to/media/files --fresh

# Use a custom database path
./mediaorganizer --source /path/to/media/files --db /path/to/journal.db

# Use date_first organization scheme with unified destination
./mediaorganizer --source /path/to/media/files --scheme date_first --dest /path/to/output

# Replace spaces in filenames with underscores
./mediaorganizer --source /path/to/media/files --space-replace

# Replace spaces with custom character (hyphen)
./mediaorganizer --source /path/to/media/files --space-replace="-"

# Discard original filename, use only timestamp and dimension
./mediaorganizer --source /path/to/media/files --no-original-name


SRC="/path/to/source"
DST="/path/to/destination"
DUP="/path/to/duplicates"
./mediaorganizer -j 6 --no-original-name --source "$SRC" --scheme date_first --dest "$DST" --duplicates-dir "$DUP"

```

## Configuration File

You can use a YAML or JSON configuration file:

```yaml
# config.yaml
source: /path/to/media/files
destinations:
  image: /path/to/organized/images
  video: /path/to/organized/videos
  audio: /path/to/organized/audio

# Optional: Custom paths for specific file extensions
extension_destinations:
  jpg: /path/to/custom/jpeg/photos
  png: /path/to/custom/png/graphics
  mp4: /path/to/custom/mp4/videos
  mp3: /path/to/custom/mp3/music

dry_run: false
copy_files: false
verbose: true
log_file: organizer.log
concurrent_jobs: 8
delete_empty_dirs: false
```

## Organization Schemes

The program supports two organization schemes that determine how files are structured in the destination:

### Extension First (default)

Files are organized with the extension/media type directory before the date structure:

```
<destination>/<extension>/YYYY/YYYY-MM/YYYY-MM-DD/YYYYMMDD-HHMMSS_<dimension> (<original_name>).<ext>
```

Example:
```
/output/images/jpeg/2025/2025-11/2025-11-23/20251123-103622_3264 (IMG01).jpeg
/output/videos/mov/2025/2025-11/2025-11-23/20251123-090020 (movie2).mov
```

### Date First

Files are organized with the date structure before the extension directory. All media types go to a single unified destination:

```
<destination>/YYYY/YYYY-MM/YYYY-MM-DD/<extension>/YYYYMMDD-HHMMSS_<dimension>_<original_name>.<ext>
```

Example (all files in one `/output` directory):
```
/output/2025/2025-11/2025-11-23/jpeg/20251123-103622_3264_IMG01.jpeg
/output/2025/2025-11/2025-11-23/mov/20251123-090020_movie2.mov
/output/2025/2025-11/2025-11-23/mp3/20251123-143000_song.mp3
```

To use the date_first scheme, use the `--scheme` and `--dest` flags:

```bash
./mediaorganizer --source /path/to/media/files --scheme date_first --dest /path/to/output
```

Or in config.yaml:
```yaml
organization_scheme: date_first
destination: /path/to/output
```

## Resume Support

The program creates a SQLite journal database (default: `<source>/.mediaorganizer.db`) that tracks every file operation. If interrupted (Ctrl+C, crash, etc.), simply re-run the same command and it will:

- Skip files that were already successfully moved/copied
- Retry files that previously failed
- Continue processing any remaining files

Use `--fresh` to start over from scratch, or `--db` to specify a custom journal path.

## Cross-Platform

The binary is fully cross-platform (pure Go, no CGo dependencies). To build for Linux from macOS:

```bash
GOOS=linux GOARCH=amd64 go build -o mediaorganizer-linux .
```

## Requirements

- Go 1.24 or higher

## License

MIT
