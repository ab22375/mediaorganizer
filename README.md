# Media Organizer

A Golang utility for organizing media files (images, videos, audio) by creation date.

## Features

- Recursively scans source directories for media files
- Extracts creation dates from EXIF metadata, file metadata, or fallback to file creation date
- Organizes files into a structured directory hierarchy based on dates
- Renames files with creation timestamp and resolution information
- Handles duplicate files with sequence numbering
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
dry_run: false
copy_files: false
verbose: true
log_file: organizer.log
concurrent_jobs: 8
```

## Output Directory Structure

The program organizes files into the following structure:

```
<destination>/<mediatype>/YYYY/YYYY-MM/YYYY-MM-DD/YYYYMMDD-HHMMSS_<dimension> <original_name>.<ext>
```

For example:
```
/output/images/2023/2023-05/2023-05-20/20230520-143015_4032 IMG_1234.jpg
/output/videos/2022/2022-12/2022-12-25/20221225-103045_1920 VID_5678.mp4
```

## Requirements

- Go 1.18 or higher

## License

MIT