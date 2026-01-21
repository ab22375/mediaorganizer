package media

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type MediaType string

const (
	TypeImage MediaType = "image"
	TypeVideo MediaType = "video"
	TypeAudio MediaType = "audio"
	TypeUnknown MediaType = "unknown"
)

type MediaFile struct {
	SourcePath      string
	Type            MediaType
	CreationTime    time.Time
	LargerDimension int
	FileSize        int64
	Hash            string
	OriginalName    string
}

func (m *MediaFile) GetExtension() string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(m.SourcePath)), ".")
}

func (m *MediaFile) GetDestinationPath(baseDir string, extensionDir string, isDuplicate bool, scheme string, duplicatesDir string) string {
	year := m.CreationTime.Format("2006")
	month := m.CreationTime.Format("01")
	day := m.CreationTime.Format("02")
	ext := m.GetExtension()

	var destPath string

	dateStructure := filepath.Join(year, fmt.Sprintf("%s-%s", year, month), fmt.Sprintf("%s-%s-%s", year, month, day))

	if extensionDir != "" {
		// Use the extension-specific directory (scheme doesn't apply here)
		if isDuplicate && filepath.IsAbs(duplicatesDir) {
			// Absolute duplicates path: use it directly with date structure
			destPath = filepath.Join(duplicatesDir, dateStructure)
		} else if isDuplicate {
			// Relative duplicates path: append to extension dir
			destPath = filepath.Join(extensionDir, duplicatesDir, dateStructure)
		} else {
			destPath = filepath.Join(extensionDir, dateStructure)
		}
	} else if scheme == "date_first" {
		// date_first: <dest>/YYYY/YYYY-MM/YYYY-MM-DD/<ext>
		if isDuplicate && filepath.IsAbs(duplicatesDir) {
			// Absolute duplicates path: use it directly with date structure and extension
			destPath = filepath.Join(duplicatesDir, dateStructure, ext)
		} else if isDuplicate {
			// Relative duplicates path: append to base dir
			destPath = filepath.Join(baseDir, duplicatesDir, dateStructure, ext)
		} else {
			destPath = filepath.Join(baseDir, dateStructure, ext)
		}
	} else {
		// extension_first (default): <dest>/<ext>/YYYY/YYYY-MM/YYYY-MM-DD
		if isDuplicate && filepath.IsAbs(duplicatesDir) {
			// Absolute duplicates path: use it directly with extension and date structure
			destPath = filepath.Join(duplicatesDir, ext, dateStructure)
		} else if isDuplicate {
			// Relative duplicates path: append to base dir with extension
			destPath = filepath.Join(baseDir, ext, duplicatesDir, dateStructure)
		} else {
			destPath = filepath.Join(baseDir, ext, dateStructure)
		}
	}

	return destPath
}

func (m *MediaFile) GetNewFilename(scheme string, spaceReplacement string, noOriginalName bool) string {
	ext := strings.ToLower(filepath.Ext(m.SourcePath))
	timestamp := m.CreationTime.Format("20060102-150405")

	// Get original name without extension for suffix
	origNameWithoutExt := ""
	if !noOriginalName {
		origNameWithoutExt = m.OriginalName
		if len(origNameWithoutExt) > 0 {
			// Remove extension(s)
			for {
				fileExt := filepath.Ext(origNameWithoutExt)
				if fileExt == "" {
					break
				}
				origNameWithoutExt = strings.TrimSuffix(origNameWithoutExt, fileExt)
			}
		}

		// Replace spaces in the original name
		if spaceReplacement != "" && spaceReplacement != " " {
			origNameWithoutExt = strings.ReplaceAll(origNameWithoutExt, " ", spaceReplacement)
		}
	}

	if scheme == "date_first" {
		// date_first scheme: YYYYMMDD-HHMMSS_dim_name.ext (images) or YYYYMMDD-HHMMSS_name.ext (video/audio)
		dimension := ""
		if m.LargerDimension > 0 && m.Type == TypeImage {
			dimension = fmt.Sprintf("_%d", m.LargerDimension)
		}

		// Skip original name if it matches the timestamp format
		namePart := ""
		if origNameWithoutExt != "" && !strings.HasPrefix(origNameWithoutExt, timestamp) {
			namePart = "_" + origNameWithoutExt
		}

		return fmt.Sprintf("%s%s%s%s", timestamp, dimension, namePart, ext)
	}

	// extension_first scheme (default): YYYYMMDD-HHMMSS_dim (name).ext
	dimension := ""
	if m.LargerDimension > 0 {
		dimension = fmt.Sprintf("_%d", m.LargerDimension)
	}

	// Check if the original filename already matches our format (YYYYMMDD-HHMMSS)
	// If it does, don't add it in parentheses
	namePart := ""
	if origNameWithoutExt != "" && !strings.HasPrefix(origNameWithoutExt, timestamp) {
		namePart = " (" + origNameWithoutExt + ")"
	}

	return fmt.Sprintf("%s%s%s%s", timestamp, dimension, namePart, ext)
}

func DetermineMediaType(filePath string) MediaType {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".tif", ".nef", ".arw", ".cr2", ".cr3", ".dng", ".heic", ".raf":
		return TypeImage
	case ".mp4", ".avi", ".mov", ".mkv", ".wmv", ".flv", ".webm", ".m4v", ".mpeg", ".mpg", ".3gp", ".asf", ".m2v", ".vob", ".m2t", ".mts":
		return TypeVideo
	case ".mp3", ".wav", ".aac", ".ogg", ".flac", ".m4a", ".wma", ".amr":
		return TypeAudio
	default:
		return TypeUnknown
	}
}