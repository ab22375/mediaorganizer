package processor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"mediaorganizer/pkg/media"
)

type ScanResult struct {
	TotalFiles      int
	ProcessedFiles  int
	SkippedFiles    int
	OrganizedFiles  int
	ErrorCount      int
	DuplicateCount  int
	StartTime       time.Time
	EndTime         time.Time
}

type MediaScanner struct {
	sourceDir        string
	destinationDirs  map[string]string
	dryRun           bool
	copyFiles        bool
	deleteEmptyDirs  bool
	concurrency      int
	processingQueue  chan string
	mediaMap         map[string][]*media.MediaFile // Maps date+dimension to files with same timestamp
	mediaMapMutex    sync.Mutex
	wg               sync.WaitGroup
	result           ScanResult
	processed        int32 // Atomic counter for progress reporting
}

func NewMediaScanner(sourceDir string, destDirs map[string]string, dryRun bool, copyFiles bool, concurrency int, deleteEmptyDirs bool) *MediaScanner {
	return &MediaScanner{
		sourceDir:       sourceDir,
		destinationDirs: destDirs,
		dryRun:          dryRun,
		copyFiles:       copyFiles,
		deleteEmptyDirs: deleteEmptyDirs,
		concurrency:     concurrency,
		processingQueue: make(chan string, 100),
		mediaMap:        make(map[string][]*media.MediaFile),
		result: ScanResult{
			StartTime: time.Now(),
		},
	}
}

func (s *MediaScanner) Scan() *ScanResult {
	logrus.Debugf("Scanner.Scan() started")
	
	// Verify destination directories
	logrus.Debugf("Source directory: %s", s.sourceDir)
	for mediaType, destDir := range s.destinationDirs {
		logrus.Debugf("Using destination for %s: %s", mediaType, destDir)
	}
	
	// Start worker goroutines
	logrus.Debugf("Starting %d worker goroutines", s.concurrency)
	for i := 0; i < s.concurrency; i++ {
		s.wg.Add(1)
		go s.processWorker()
	}

	// Walk through the source directory
	logrus.Debugf("Walking source directory: %s", s.sourceDir)
	filepath.Walk(s.sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logrus.Errorf("Error accessing path %s: %v", path, err)
			s.result.ErrorCount++
			return nil
		}

		if info.IsDir() {
			return nil
		}

		// If the file is a media file, add it to the processing queue
		if media.DetermineMediaType(path) != media.TypeUnknown {
			s.result.TotalFiles++
			s.processingQueue <- path
		}

		return nil
	})

	// Close the queue and wait for all workers to finish
	logrus.Debugf("Closing processing queue")
	close(s.processingQueue)
	
	logrus.Debugf("Waiting for workers to finish")
	s.wg.Wait()

	// Organize files by creating sequences for files with identical timestamps
	logrus.Debugf("Organizing files")
	s.organizeFiles()

	// Delete empty directories if enabled and not in dry run mode
	if s.deleteEmptyDirs && !s.dryRun && !s.copyFiles {
		logrus.Infof("Cleaning up empty directories in source...")
		s.cleanupEmptyDirectories()
	}

	s.result.EndTime = time.Now()
	logrus.Debugf("Scan complete, processed %d files", s.result.ProcessedFiles)
	return &s.result
}

func (s *MediaScanner) processWorker() {
	defer s.wg.Done()

	for filePath := range s.processingQueue {
		mediaFile, err := media.ExtractFileMetadata(filePath)
		if err != nil {
			logrus.Errorf("Error processing %s: %v", filePath, err)
			s.result.ErrorCount++
			s.result.SkippedFiles++
			continue
		}

		// Add to media map to handle duplicates and sequences later
		key := mediaFile.CreationTime.Format("20060102-150405") + "_" + string(mediaFile.Type) + "_" + filepath.Ext(filePath)
		
		s.mediaMapMutex.Lock()
		s.mediaMap[key] = append(s.mediaMap[key], mediaFile)
		s.mediaMapMutex.Unlock()

		s.result.ProcessedFiles++
		atomic.AddInt32(&s.processed, 1)
	}
}

func (s *MediaScanner) organizeFiles() {
	for _, files := range s.mediaMap {
		for i, file := range files {
			// For files with the same timestamp, we need to add a sequence number
			sequenceNum := ""
			if len(files) > 1 {
				// Always add sequence numbers when multiple files have the same timestamp
				sequenceNum = "_" + formatSequence(i+1)
			}

			destDir := s.destinationDirs[string(file.Type)]
			if destDir == "" {
				logrus.Warnf("No destination directory configured for media type: %s", file.Type)
				continue
			}

			fileDir := file.GetDestinationPath(destDir)
			fileName := file.GetNewFilename()
			
			// Add sequence if multiple files with same timestamp
			if sequenceNum != "" {
				ext := filepath.Ext(fileName)
				baseName := fileName[:len(fileName)-len(ext)]
				fileName = baseName + sequenceNum + ext
			}
			
			destPath := filepath.Join(fileDir, fileName)

			operation := "move"
			if s.copyFiles {
				operation = "copy"
			}

			if s.dryRun {
				logrus.Infof("[DRY RUN] Would %s: %s -> %s", operation, file.SourcePath, destPath)
				s.result.OrganizedFiles++
				continue
			}

			// Ensure destination directory exists
			if err := os.MkdirAll(fileDir, 0755); err != nil {
				logrus.Errorf("Failed to create directory %s: %v", fileDir, err)
				s.result.ErrorCount++
				continue
			}

			var err error
			if s.copyFiles {
				// Copy the file
				err = copyFile(file.SourcePath, destPath)
				if err == nil {
					logrus.Infof("Copied: %s -> %s", file.SourcePath, destPath)
				}
			} else {
				// Move the file
				err = moveFile(file.SourcePath, destPath)
				if err == nil {
					logrus.Infof("Moved: %s -> %s", file.SourcePath, destPath)
				}
			}

			if err != nil {
				logrus.Errorf("Failed to %s file %s to %s: %v", operation, file.SourcePath, destPath, err)
				s.result.ErrorCount++
				continue
			}
			s.result.OrganizedFiles++
		}
	}
}

func formatSequence(num int) string {
	return fmt.Sprintf("%03d", num)
}

func moveFile(srcPath, destPath string) error {
	// Check if destination already exists
	if _, err := os.Stat(destPath); err == nil {
		logrus.Infof("Skipped moving file (already exists at destination): %s -> %s", srcPath, destPath)
		return nil
	}

	// Move (rename) the file
	return os.Rename(srcPath, destPath)
}

func copyFile(srcPath, destPath string) error {
	// Check if destination already exists
	if _, err := os.Stat(destPath); err == nil {
		logrus.Infof("Skipped copying file (already exists at destination): %s -> %s", srcPath, destPath)
		return nil
	}

	// Open the source file
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	// Create the destination file
	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	// Copy the contents
	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	// Flush the write buffer to disk
	err = dst.Sync()
	if err != nil {
		return err
	}

	// Copy file permissions
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	return os.Chmod(destPath, srcInfo.Mode())
}

// GetProcessedCount returns the current count of processed files
func (s *MediaScanner) GetProcessedCount() int {
	return int(atomic.LoadInt32(&s.processed))
}

// cleanupEmptyDirectories removes empty directories within the source directory
func (s *MediaScanner) cleanupEmptyDirectories() {
	var emptyDirs []string
	var emptyDirsMutex sync.Mutex
	var deletedCount int

	// Walk the directory tree bottom-up to find empty directories
	filepath.Walk(s.sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logrus.Errorf("Error accessing path while cleaning up: %s: %v", path, err)
			return nil
		}

		// Skip if it's not a directory or if it's the source directory itself
		if !info.IsDir() || path == s.sourceDir {
			return nil
		}

		// Check if directory is empty
		entries, err := os.ReadDir(path)
		if err != nil {
			logrus.Errorf("Error reading directory %s: %v", path, err)
			return nil
		}

		if len(entries) == 0 {
			emptyDirsMutex.Lock()
			emptyDirs = append(emptyDirs, path)
			emptyDirsMutex.Unlock()
		}

		return nil
	})

	// Sort directories by length in descending order to remove deepest directories first
	sort.Slice(emptyDirs, func(i, j int) bool {
		return len(emptyDirs[i]) > len(emptyDirs[j])
	})

	// Delete empty directories
	for _, dir := range emptyDirs {
		if err := os.Remove(dir); err != nil {
			logrus.Errorf("Failed to remove empty directory %s: %v", dir, err)
		} else {
			logrus.Infof("Removed empty directory: %s", dir)
			deletedCount++
		}
	}

	if deletedCount > 0 {
		logrus.Infof("Removed %d empty directories", deletedCount)
	} else {
		logrus.Infof("No empty directories found to remove")
	}
}