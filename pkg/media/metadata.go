package media

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/sirupsen/logrus"
	_ "golang.org/x/image/tiff"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

func ExtractFileMetadata(filePath string) (*MediaFile, error) {
	fileInfo, err := os.Stat(filePath)
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

// ComputeFileHash returns the xxHash (XXH64) hex digest of the file at filePath.
// xxHash is non-cryptographic but provides excellent collision resistance for dedup
// at 5-10x the speed of SHA-256, which matters for large media files.
func ComputeFileHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := xxhash.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%016x", h.Sum64()), nil
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

// ffprobeFormat represents the relevant fields from ffprobe JSON output.
type ffprobeOutput struct {
	Format struct {
		Tags map[string]string `json:"tags"`
	} `json:"format"`
}

func extractMediaMetadata(filePath string, mediaFile *MediaFile) (time.Time, error) {
	// Try ffprobe first for accurate creation dates
	if t, err := extractCreationTimeViaFFprobe(filePath); err == nil && !t.IsZero() {
		return t, nil
	}

	// Fall back to file modification time
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return time.Time{}, err
	}
	return fileInfo.ModTime(), nil
}

// extractCreationTimeViaFFprobe shells out to ffprobe to get the creation_time
// tag from media container metadata. Returns zero time if ffprobe is not
// available or the file has no creation_time tag.
func extractCreationTimeViaFFprobe(filePath string) (time.Time, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		filePath,
	)
	out, err := cmd.Output()
	if err != nil {
		return time.Time{}, err
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(out, &probe); err != nil {
		return time.Time{}, err
	}

	// Try common tag names (case-insensitive keys vary by container)
	for _, key := range []string{"creation_time", "date", "Creation Time"} {
		val, ok := probe.Format.Tags[key]
		if !ok {
			// Try case-insensitive match
			for k, v := range probe.Format.Tags {
				if strings.EqualFold(k, key) {
					val = v
					ok = true
					break
				}
			}
		}
		if !ok || val == "" {
			continue
		}

		// Try common timestamp formats
		for _, layout := range []string{
			time.RFC3339,
			"2006-01-02T15:04:05.000000Z",
			"2006-01-02 15:04:05",
			"2006:01:02 15:04:05",
		} {
			if t, err := time.Parse(layout, val); err == nil {
				return t, nil
			}
		}
	}

	return time.Time{}, fmt.Errorf("no creation_time tag found")
}