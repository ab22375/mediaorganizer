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

func (m *MediaFile) GetDestinationPath(baseDir string) string {
	year := m.CreationTime.Format("2006")
	month := m.CreationTime.Format("01")
	day := m.CreationTime.Format("02")
	
	// Using the baseDir directly without adding the media type again
	// Since the baseDir already includes the media type-specific path
	destDir := filepath.Join(baseDir, year, fmt.Sprintf("%s-%s", year, month), fmt.Sprintf("%s-%s-%s", year, month, day))
	
	return destDir
}

func (m *MediaFile) GetNewFilename() string {
	ext := strings.ToLower(filepath.Ext(m.SourcePath))
	timestamp := m.CreationTime.Format("20060102-150405")
	
	dimension := ""
	if m.LargerDimension > 0 {
		dimension = fmt.Sprintf("_%d", m.LargerDimension)
	}
	
	// Get original name without extension for suffix
	origNameWithoutExt := m.OriginalName
	if len(origNameWithoutExt) > 0 {
		// Remove extension(s)
		for {
			fileExt := filepath.Ext(origNameWithoutExt)
			if fileExt == "" {
				break
			}
			origNameWithoutExt = strings.TrimSuffix(origNameWithoutExt, fileExt)
		}
		
		// Check if the original filename already matches our format (YYYYMMDD-HHMMSS)
		// If it does, don't add it in parentheses
		if !strings.HasPrefix(origNameWithoutExt, timestamp) {
			// Add parentheses around the original name
			origNameWithoutExt = " (" + origNameWithoutExt + ")"
		} else {
			origNameWithoutExt = ""
		}
	}
	
	return fmt.Sprintf("%s%s%s%s", timestamp, dimension, origNameWithoutExt, ext)
}

func DetermineMediaType(filePath string) MediaType {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".tif", ".nef", ".arw", ".cr2", ".cr3", ".dng", ".heic", ".raf":
		return TypeImage
	case ".mp4", ".avi", ".mov", ".mkv", ".wmv", ".flv", ".webm", ".m4v", ".mpeg", ".mpg", ".3gp", ".asf", ".m2v", ".vob":
		return TypeVideo
	case ".mp3", ".wav", ".aac", ".ogg", ".flac", ".m4a", ".wma", ".amr":
		return TypeAudio
	default:
		return TypeUnknown
	}
}