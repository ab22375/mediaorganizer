package media

import (
	"errors"
	"image"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/sirupsen/logrus"
	_ "golang.org/x/image/tiff"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

func ExtractFileMetadata(filePath string) (*MediaFile, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}

	mediaType := DetermineMediaType(filePath)
	if mediaType == TypeUnknown {
		return nil, errors.New("unsupported file type")
	}

	mediaFile := &MediaFile{
		SourcePath:   filePath,
		Type:         mediaType,
		FileSize:     fileInfo.Size(),
		OriginalName: filepath.Base(filePath),
	}

	// Get creation time
	var creationTime time.Time
	var timeErr error

	switch mediaType {
	case TypeImage:
		creationTime, timeErr = extractImageMetadata(filePath, mediaFile)
	case TypeVideo, TypeAudio:
		creationTime, timeErr = extractMediaMetadata(filePath, mediaFile)
	}

	// Fallback to file creation time if metadata extraction failed
	if timeErr != nil || creationTime.IsZero() {
		logrus.Debugf("Could not extract time from metadata for %s: %v. Using file info time.", filePath, timeErr)
		creationTime = fileInfo.ModTime()
	}

	mediaFile.CreationTime = creationTime
	return mediaFile, nil
}

func extractImageMetadata(filePath string, mediaFile *MediaFile) (time.Time, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return time.Time{}, err
	}
	defer file.Close()

	// Try to get dimensions first
	img, _, err := image.DecodeConfig(file)
	if err == nil {
		if img.Width > img.Height {
			mediaFile.LargerDimension = img.Width
		} else {
			mediaFile.LargerDimension = img.Height
		}
	} else {
		logrus.Debugf("Could not decode image dimensions: %v", err)
	}

	// Rewind file for EXIF reading
	file.Seek(0, io.SeekStart)
	
	// First try with rwcarlsen/goexif
	exifData, err := exif.Decode(file)
	if err == nil {
		dateTime, err := exifData.DateTime()
		if err == nil {
			return dateTime, nil
		}
		
		// Try with DateTimeOriginal tag
		tag, err := exifData.Get(exif.DateTimeOriginal)
		if err == nil {
			if str, err := tag.StringVal(); err == nil {
				t, err := time.Parse("2006:01:02 15:04:05", str)
				if err == nil {
					return t, nil
				}
			}
		}
	}
	
	// Try to extract creation time from file modification time as a fallback
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return time.Time{}, err
	}
	
	// For now, return the file's modification time
	// In a production environment, you would want to improve this with more
	// robust metadata extraction techniques
	modTime := fileInfo.ModTime()
	
	// For simplicity, we're returning the modification time
	// You could enhance this with specific libraries for media metadata extraction
	return modTime, nil
}

func extractMediaMetadata(filePath string, mediaFile *MediaFile) (time.Time, error) {
	// For now, we'll use file modification time for audio/video files
	// In a production environment, you'd want to use a library like ffmpeg or mediainfo
	// to extract the actual creation time from media metadata
	
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return time.Time{}, err
	}
	
	return fileInfo.ModTime(), nil
}