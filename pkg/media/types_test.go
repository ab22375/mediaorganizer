package media

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDetermineMediaType(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected MediaType
	}{
		// Images
		{"JPEG lowercase", "photo.jpg", TypeImage},
		{"JPEG uppercase", "photo.JPG", TypeImage},
		{"JPEG full", "photo.jpeg", TypeImage},
		{"PNG", "image.png", TypeImage},
		{"GIF", "animation.gif", TypeImage},
		{"BMP", "bitmap.bmp", TypeImage},
		{"WEBP", "modern.webp", TypeImage},
		{"TIFF", "scan.tiff", TypeImage},
		{"TIF", "scan.tif", TypeImage},
		{"NEF (Nikon RAW)", "raw.nef", TypeImage},
		{"ARW (Sony RAW)", "raw.arw", TypeImage},
		{"CR2 (Canon RAW)", "raw.cr2", TypeImage},
		{"CR3 (Canon RAW)", "raw.cr3", TypeImage},
		{"DNG", "raw.dng", TypeImage},
		{"HEIC", "photo.heic", TypeImage},
		{"RAF (Fuji RAW)", "raw.raf", TypeImage},

		// Videos
		{"MP4", "video.mp4", TypeVideo},
		{"AVI", "video.avi", TypeVideo},
		{"MOV", "video.mov", TypeVideo},
		{"MKV", "video.mkv", TypeVideo},
		{"WMV", "video.wmv", TypeVideo},
		{"FLV", "video.flv", TypeVideo},
		{"WEBM", "video.webm", TypeVideo},
		{"M4V", "video.m4v", TypeVideo},
		{"MPEG", "video.mpeg", TypeVideo},
		{"MPG", "video.mpg", TypeVideo},
		{"3GP", "video.3gp", TypeVideo},
		{"MTS", "video.mts", TypeVideo},

		// Audio
		{"MP3", "song.mp3", TypeAudio},
		{"WAV", "audio.wav", TypeAudio},
		{"AAC", "audio.aac", TypeAudio},
		{"OGG", "audio.ogg", TypeAudio},
		{"FLAC", "audio.flac", TypeAudio},
		{"M4A", "audio.m4a", TypeAudio},
		{"WMA", "audio.wma", TypeAudio},

		// Unknown
		{"Text file", "document.txt", TypeUnknown},
		{"PDF", "document.pdf", TypeUnknown},
		{"No extension", "file", TypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineMediaType(tt.filePath)
			if result != tt.expected {
				t.Errorf("DetermineMediaType(%q) = %v, want %v", tt.filePath, result, tt.expected)
			}
		})
	}
}

func TestGetDestinationPath_ExtensionFirst(t *testing.T) {
	creationTime := time.Date(2025, 11, 23, 10, 36, 22, 0, time.UTC)

	tests := []struct {
		name         string
		mediaFile    *MediaFile
		baseDir      string
		extensionDir string
		isDuplicate  bool
		expected     string
	}{
		{
			name: "Basic image path",
			mediaFile: &MediaFile{
				SourcePath:   "/source/IMG01.jpeg",
				Type:         TypeImage,
				CreationTime: creationTime,
			},
			baseDir:      "/dest/images",
			extensionDir: "",
			isDuplicate:  false,
			expected:     filepath.Join("/dest/images", "jpeg", "2025", "2025-11", "2025-11-23"),
		},
		{
			name: "Video path",
			mediaFile: &MediaFile{
				SourcePath:   "/source/video.mp4",
				Type:         TypeVideo,
				CreationTime: creationTime,
			},
			baseDir:      "/dest/videos",
			extensionDir: "",
			isDuplicate:  false,
			expected:     filepath.Join("/dest/videos", "mp4", "2025", "2025-11", "2025-11-23"),
		},
		{
			name: "Duplicate image path",
			mediaFile: &MediaFile{
				SourcePath:   "/source/IMG02.jpeg",
				Type:         TypeImage,
				CreationTime: creationTime,
			},
			baseDir:      "/dest/images",
			extensionDir: "",
			isDuplicate:  true,
			expected:     filepath.Join("/dest/images", "jpeg", "duplicates", "2025", "2025-11", "2025-11-23"),
		},
		{
			name: "Extension-specific directory",
			mediaFile: &MediaFile{
				SourcePath:   "/source/IMG01.jpeg",
				Type:         TypeImage,
				CreationTime: creationTime,
			},
			baseDir:      "/dest/images",
			extensionDir: "/custom/jpeg/path",
			isDuplicate:  false,
			expected:     filepath.Join("/custom/jpeg/path", "2025", "2025-11", "2025-11-23"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.mediaFile.GetDestinationPath(tt.baseDir, tt.extensionDir, tt.isDuplicate, "extension_first")
			if result != tt.expected {
				t.Errorf("GetDestinationPath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetDestinationPath_DateFirst(t *testing.T) {
	creationTime := time.Date(2025, 11, 23, 10, 36, 22, 0, time.UTC)

	tests := []struct {
		name         string
		mediaFile    *MediaFile
		baseDir      string
		extensionDir string
		isDuplicate  bool
		expected     string
	}{
		{
			name: "Unified destination - image",
			mediaFile: &MediaFile{
				SourcePath:   "/source/IMG01.jpeg",
				Type:         TypeImage,
				CreationTime: creationTime,
			},
			baseDir:      "/output",
			extensionDir: "",
			isDuplicate:  false,
			expected:     filepath.Join("/output", "2025", "2025-11", "2025-11-23", "jpeg"),
		},
		{
			name: "Unified destination - video",
			mediaFile: &MediaFile{
				SourcePath:   "/source/video.mov",
				Type:         TypeVideo,
				CreationTime: creationTime,
			},
			baseDir:      "/output",
			extensionDir: "",
			isDuplicate:  false,
			expected:     filepath.Join("/output", "2025", "2025-11", "2025-11-23", "mov"),
		},
		{
			name: "Unified destination - audio",
			mediaFile: &MediaFile{
				SourcePath:   "/source/song.mp3",
				Type:         TypeAudio,
				CreationTime: creationTime,
			},
			baseDir:      "/output",
			extensionDir: "",
			isDuplicate:  false,
			expected:     filepath.Join("/output", "2025", "2025-11", "2025-11-23", "mp3"),
		},
		{
			name: "Duplicate path with unified destination",
			mediaFile: &MediaFile{
				SourcePath:   "/source/IMG02.jpeg",
				Type:         TypeImage,
				CreationTime: creationTime,
			},
			baseDir:      "/output",
			extensionDir: "",
			isDuplicate:  true,
			expected:     filepath.Join("/output", "duplicates", "2025", "2025-11", "2025-11-23", "jpeg"),
		},
		{
			name: "Extension-specific directory ignores scheme",
			mediaFile: &MediaFile{
				SourcePath:   "/source/IMG01.jpeg",
				Type:         TypeImage,
				CreationTime: creationTime,
			},
			baseDir:      "/output",
			extensionDir: "/custom/jpeg/path",
			isDuplicate:  false,
			expected:     filepath.Join("/custom/jpeg/path", "2025", "2025-11", "2025-11-23"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.mediaFile.GetDestinationPath(tt.baseDir, tt.extensionDir, tt.isDuplicate, "date_first")
			if result != tt.expected {
				t.Errorf("GetDestinationPath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetNewFilename_ExtensionFirst(t *testing.T) {
	creationTime := time.Date(2025, 11, 23, 10, 36, 22, 0, time.UTC)

	tests := []struct {
		name     string
		media    *MediaFile
		expected string
	}{
		{
			name: "Image with dimension",
			media: &MediaFile{
				SourcePath:      "/source/IMG01.jpeg",
				Type:            TypeImage,
				CreationTime:    creationTime,
				LargerDimension: 3264,
				OriginalName:    "IMG01.jpeg",
			},
			expected: "20251123-103622_3264 (IMG01).jpeg",
		},
		{
			name: "Video without dimension",
			media: &MediaFile{
				SourcePath:   "/source/movie.mov",
				Type:         TypeVideo,
				CreationTime: creationTime,
				OriginalName: "movie.mov",
			},
			expected: "20251123-103622 (movie).mov",
		},
		{
			name: "Already formatted name is not duplicated",
			media: &MediaFile{
				SourcePath:      "/source/20251123-103622_3264.jpeg",
				Type:            TypeImage,
				CreationTime:    creationTime,
				LargerDimension: 3264,
				OriginalName:    "20251123-103622_3264.jpeg",
			},
			expected: "20251123-103622_3264.jpeg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.media.GetNewFilename("extension_first")
			if result != tt.expected {
				t.Errorf("GetNewFilename() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetNewFilename_DateFirst(t *testing.T) {
	creationTime := time.Date(2025, 11, 23, 10, 36, 22, 0, time.UTC)

	tests := []struct {
		name     string
		media    *MediaFile
		expected string
	}{
		{
			name: "Image with dimension - underscore format",
			media: &MediaFile{
				SourcePath:      "/source/IMG01.jpeg",
				Type:            TypeImage,
				CreationTime:    creationTime,
				LargerDimension: 3264,
				OriginalName:    "IMG01.jpeg",
			},
			expected: "20251123-103622_3264_IMG01.jpeg",
		},
		{
			name: "Video without dimension in filename",
			media: &MediaFile{
				SourcePath:      "/source/movie2.mov",
				Type:            TypeVideo,
				CreationTime:    creationTime,
				LargerDimension: 1920, // Video has dimension but it's not included in date_first scheme
				OriginalName:    "movie2.mov",
			},
			expected: "20251123-103622_movie2.mov",
		},
		{
			name: "Audio file",
			media: &MediaFile{
				SourcePath:   "/source/song.mp3",
				Type:         TypeAudio,
				CreationTime: creationTime,
				OriginalName: "song.mp3",
			},
			expected: "20251123-103622_song.mp3",
		},
		{
			name: "Image without dimension",
			media: &MediaFile{
				SourcePath:   "/source/photo.jpeg",
				Type:         TypeImage,
				CreationTime: creationTime,
				OriginalName: "photo.jpeg",
			},
			expected: "20251123-103622_photo.jpeg",
		},
		{
			name: "Already formatted name is not duplicated",
			media: &MediaFile{
				SourcePath:      "/source/20251123-103622_3264_IMG01.jpeg",
				Type:            TypeImage,
				CreationTime:    creationTime,
				LargerDimension: 3264,
				OriginalName:    "20251123-103622_3264_IMG01.jpeg",
			},
			expected: "20251123-103622_3264.jpeg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.media.GetNewFilename("date_first")
			if result != tt.expected {
				t.Errorf("GetNewFilename() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetExtension(t *testing.T) {
	tests := []struct {
		name       string
		sourcePath string
		expected   string
	}{
		{"JPEG lowercase", "/path/to/file.jpeg", "jpeg"},
		{"JPEG uppercase", "/path/to/file.JPEG", "jpeg"},
		{"JPG", "/path/to/file.jpg", "jpg"},
		{"PNG", "/path/to/file.png", "png"},
		{"MP4", "/path/to/file.mp4", "mp4"},
		{"No extension", "/path/to/file", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MediaFile{SourcePath: tt.sourcePath}
			result := m.GetExtension()
			if result != tt.expected {
				t.Errorf("GetExtension() = %v, want %v", result, tt.expected)
			}
		})
	}
}
