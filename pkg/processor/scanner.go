package processor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"mediaorganizer/pkg/db"
	"mediaorganizer/pkg/media"
)

type ScanResult struct {
	TotalFiles     int
	ProcessedFiles int
	SkippedFiles   int
	OrganizedFiles int
	ErrorCount     int
	DuplicateCount int
	StartTime      time.Time
	EndTime        time.Time
}

type MediaScanner struct {
	sourceDir        string
	destination      string // Unified destination for date_first scheme
	destinationDirs  map[string]string
	extensionDirs    map[string]string
	scheme           string
	spaceReplacement string
	noOriginalName   bool
	duplicatesDir    string
	dryRun           bool
	copyFiles        bool
	deleteEmptyDirs  bool
	concurrency      int
	journal          *db.Journal
	resumeMode       bool
	result           ScanResult
	processed        int32 // Atomic counter for metadata-extracted files
	organized        int32 // Atomic counter for moved/copied files
}

type metadataResult struct {
	File *media.MediaFile
	Err  error
}

type moveJob struct {
	RecordID    int64
	File        *media.MediaFile
	DestPath    string
	IsDuplicate bool
}

func NewMediaScanner(sourceDir string, destination string, destDirs map[string]string, extensionDirs map[string]string, scheme string, spaceReplacement string, noOriginalName bool, duplicatesDir string, dryRun bool, copyFiles bool, concurrency int, deleteEmptyDirs bool, journal *db.Journal, resumeMode bool) *MediaScanner {
	return &MediaScanner{
		sourceDir:        sourceDir,
		destination:      destination,
		destinationDirs:  destDirs,
		extensionDirs:    extensionDirs,
		scheme:           scheme,
		spaceReplacement: spaceReplacement,
		noOriginalName:   noOriginalName,
		duplicatesDir:    duplicatesDir,
		dryRun:           dryRun,
		copyFiles:        copyFiles,
		deleteEmptyDirs:  deleteEmptyDirs,
		concurrency:      concurrency,
		journal:          journal,
		resumeMode:       resumeMode,
		result: ScanResult{
			StartTime: time.Now(),
		},
	}
}

func (s *MediaScanner) Scan() *ScanResult {
	logrus.Debugf("Scanner.Scan() started")

	logrus.Debugf("Source directory: %s", s.sourceDir)
	for mediaType, destDir := range s.destinationDirs {
		logrus.Debugf("Using destination for %s: %s", mediaType, destDir)
	}
	for ext, destDir := range s.extensionDirs {
		logrus.Debugf("Using destination for extension .%s: %s", ext, destDir)
	}

	// Pre-index destination directories for cross-scan duplicate detection
	s.preIndexDestinations()

	// Resume support: load completed paths and pending records
	var completedPaths map[string]bool
	var pendingRecords []*db.FileRecord
	if s.resumeMode {
		var err error
		completedPaths, err = s.journal.GetCompletedSourcePaths()
		if err != nil {
			logrus.Errorf("Failed to load completed paths: %v", err)
		} else if len(completedPaths) > 0 {
			logrus.Infof("Resuming: %d files already completed, will be skipped", len(completedPaths))
		}

		// Reset failed records for retry
		resetCount, err := s.journal.ResetFailed()
		if err != nil {
			logrus.Errorf("Failed to reset failed records: %v", err)
		} else if resetCount > 0 {
			logrus.Infof("Resuming: reset %d failed records for retry", resetCount)
		}

		pendingRecords, err = s.journal.GetPendingFiles()
		if err != nil {
			logrus.Errorf("Failed to load pending records: %v", err)
		} else if len(pendingRecords) > 0 {
			logrus.Infof("Resuming: %d pending files to process", len(pendingRecords))
		}
	}

	// Pipeline channels
	pathsCh := make(chan string, 100)
	metaCh := make(chan metadataResult, 100)
	moveCh := make(chan moveJob, 100)

	// --- Stage 1: Walker goroutine ---
	go func() {
		defer close(pathsCh)
		filepath.Walk(s.sourceDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				logrus.Errorf("Error accessing path %s: %v", path, err)
				return nil
			}
			if info.IsDir() {
				return nil
			}

			// Skip macOS hidden files
			fileName := filepath.Base(path)
			if strings.HasPrefix(fileName, "._") {
				return nil
			}

			// Skip the journal database file
			if strings.HasSuffix(path, ".mediaorganizer.db") ||
				strings.HasSuffix(path, ".mediaorganizer.db-wal") ||
				strings.HasSuffix(path, ".mediaorganizer.db-shm") {
				return nil
			}

			if media.DetermineMediaType(path) == media.TypeUnknown {
				return nil
			}

			// Skip already completed files in resume mode
			if completedPaths != nil && completedPaths[path] {
				logrus.Debugf("Skipping completed file: %s", path)
				return nil
			}

			atomic.AddInt32(&s.processed, 0) // just count total via TotalFiles
			s.result.TotalFiles++
			pathsCh <- path
			return nil
		})
	}()

	// --- Stage 2: Metadata worker goroutines ---
	var metaWg sync.WaitGroup
	for i := 0; i < s.concurrency; i++ {
		metaWg.Add(1)
		go func() {
			defer metaWg.Done()
			for filePath := range pathsCh {
				mf, err := media.ExtractFileMetadata(filePath)
				if err != nil {
					logrus.Errorf("Error extracting metadata for %s: %v", filePath, err)
					metaCh <- metadataResult{Err: fmt.Errorf("%s: %w", filePath, err)}
					continue
				}
				metaCh <- metadataResult{File: mf}
			}
		}()
	}

	// Close metaCh when all metadata workers are done
	go func() {
		metaWg.Wait()
		close(metaCh)
	}()

	// --- Stage 3: Organizer goroutine (single, serializes all DB writes) ---
	var organizerWg sync.WaitGroup
	organizerWg.Add(1)
	go func() {
		defer organizerWg.Done()
		defer close(moveCh)

		// First: re-queue pending records from resume
		for _, rec := range pendingRecords {
			// Verify source file still exists before re-queuing
			if _, err := os.Stat(rec.SourcePath); os.IsNotExist(err) {
				// Source is gone — check if dest already has the file (previous move succeeded but status wasn't updated)
				if rec.DestPath != "" {
					if _, err := os.Stat(rec.DestPath); err == nil {
						logrus.Infof("Resume: source gone but dest exists, marking completed: %s", rec.DestPath)
						s.journal.UpdateStatus(rec.ID, db.StatusCompleted, "")
						atomic.AddInt32(&s.organized, 1)
						continue
					}
				}
				logrus.Warnf("Resume: source file no longer exists, marking failed: %s", rec.SourcePath)
				s.journal.UpdateStatus(rec.ID, db.StatusFailed, "source file missing on resume")
				continue
			}

			mf := recordToMediaFile(rec)
			moveCh <- moveJob{
				RecordID:    rec.ID,
				File:        mf,
				DestPath:    rec.DestPath,
				IsDuplicate: rec.IsDuplicate,
			}
		}

		// Then: process new metadata results
		for mr := range metaCh {
			if mr.Err != nil {
				s.result.ErrorCount++
				s.result.SkippedFiles++
				continue
			}

			file := mr.File
			atomic.AddInt32(&s.processed, 1)
			s.result.ProcessedFiles++

			// Build timestamp key
			tsKey := file.CreationTime.Format("20060102-150405") + "_" + string(file.Type) + "_" + filepath.Ext(file.SourcePath)

			// Insert into journal
			rec := &db.FileRecord{
				SourcePath:      file.SourcePath,
				FileSize:        file.FileSize,
				MediaType:       string(file.Type),
				Extension:       file.GetExtension(),
				CreationTime:    file.CreationTime.Format("2006-01-02 15:04:05"),
				LargerDimension: file.LargerDimension,
				OriginalName:    file.OriginalName,
				TimestampKey:    tsKey,
				Status:          db.StatusPending,
			}

			id, err := s.journal.InsertFile(rec)
			if err != nil {
				if err == db.ErrAlreadyExists {
					logrus.Debugf("Skipping already-journaled file: %s", file.SourcePath)
					continue
				}
				logrus.Errorf("Failed to insert journal record for %s: %v", file.SourcePath, err)
				s.result.ErrorCount++
				continue
			}

			// --- Lazy hashing ---
			sizeCount, err := s.journal.CountByFileSize(file.FileSize)
			if err != nil {
				logrus.Errorf("CountByFileSize error: %v", err)
			}

			var fileHash string
			if sizeCount >= 2 {
				// Hash the new file
				h, err := media.ComputeFileHash(file.SourcePath)
				if err != nil {
					logrus.Warnf("Could not hash %s: %v", file.SourcePath, err)
				} else {
					fileHash = h
					s.journal.UpdateHash(id, fileHash)
				}

				// Backfill unhashed files with same size (includes pre-indexed destination files)
				unhashed, err := s.journal.GetUnhashedByFileSize(file.FileSize)
				if err != nil {
					logrus.Errorf("GetUnhashedByFileSize error: %v", err)
				}
				for _, ur := range unhashed {
					// Try source path first, fall back to dest path if source was already moved
					hashPath := ur.SourcePath
					if _, statErr := os.Stat(hashPath); os.IsNotExist(statErr) && ur.DestPath != "" {
						hashPath = ur.DestPath
					}
					bh, err := media.ComputeFileHash(hashPath)
					if err != nil {
						logrus.Warnf("Could not backfill hash for %s: %v", ur.SourcePath, err)
						continue
					}
					s.journal.UpdateHash(ur.ID, bh)
				}
			}

			// --- Global dedup ---
			isDuplicate := false
			if fileHash != "" {
				matches, err := s.journal.GetByHash(fileHash)
				if err != nil {
					logrus.Errorf("GetByHash error: %v", err)
				}
				// If there's another record with the same hash (not this one), it's a duplicate
				for _, m := range matches {
					if m.ID != id {
						isDuplicate = true
						logrus.Debugf("Duplicate detected (hash %s): %s", fileHash[:12], file.SourcePath)
						break
					}
				}
			}

			// --- Sequence numbering ---
			tsCount, err := s.journal.CountByTimestampKey(tsKey)
			if err != nil {
				logrus.Errorf("CountByTimestampKey error: %v", err)
			}
			seqNum := 0
			if tsCount > 1 {
				seqNum = tsCount
			}

			// --- Compute destination path ---
			destPath := s.computeDestPath(file, isDuplicate, seqNum)

			// Update journal
			s.journal.UpdateDestPath(id, destPath, seqNum, isDuplicate)

			moveCh <- moveJob{
				RecordID:    id,
				File:        file,
				DestPath:    destPath,
				IsDuplicate: isDuplicate,
			}
		}
	}()

	// --- Stage 4: Mover worker goroutines ---
	var moverWg sync.WaitGroup
	for i := 0; i < s.concurrency; i++ {
		moverWg.Add(1)
		go func() {
			defer moverWg.Done()
			for job := range moveCh {
				s.executeMoveJob(job)
			}
		}()
	}

	// Wait for all stages to complete
	moverWg.Wait()

	// Delete empty directories if enabled
	if s.deleteEmptyDirs && !s.dryRun && !s.copyFiles {
		logrus.Infof("Cleaning up empty directories in source...")
		s.cleanupEmptyDirectories()
	}

	// Clean up dest_index rows before computing stats
	if err := s.journal.ClearDestIndex(); err != nil {
		logrus.Errorf("Failed to clear dest_index rows: %v", err)
	}

	// Populate result from journal stats
	s.populateResultFromJournal()

	s.result.EndTime = time.Now()
	logrus.Debugf("Scan complete, processed %d files", s.result.ProcessedFiles)
	return &s.result
}

func (s *MediaScanner) computeDestPath(file *media.MediaFile, isDuplicate bool, seqNum int) string {
	ext := filepath.Ext(file.SourcePath)
	if len(ext) > 0 {
		ext = ext[1:] // Remove leading dot
	}

	// Get base destination directory
	var baseDestDir string
	if s.scheme == "date_first" && s.destination != "" {
		baseDestDir = s.destination
	} else {
		baseDestDir = s.destinationDirs[string(file.Type)]
		if baseDestDir == "" {
			logrus.Warnf("No destination directory configured for media type: %s", file.Type)
			return ""
		}
	}

	// Extension-specific directory
	extensionDir := ""
	if ext != "" {
		extensionDir = s.extensionDirs[ext]
	}

	fileDir := file.GetDestinationPath(baseDestDir, extensionDir, isDuplicate, s.scheme, s.duplicatesDir)
	fileName := file.GetNewFilename(s.scheme, s.spaceReplacement, s.noOriginalName)

	// Add sequence suffix
	if seqNum > 1 {
		fileExt := filepath.Ext(fileName)
		baseName := fileName[:len(fileName)-len(fileExt)]
		fileName = baseName + "_" + formatSequence(seqNum) + fileExt
	}

	return filepath.Join(fileDir, fileName)
}

func (s *MediaScanner) executeMoveJob(job moveJob) {
	operation := "move"
	if s.copyFiles {
		operation = "copy"
	}

	if s.dryRun {
		dupLabel := ""
		if job.IsDuplicate {
			dupLabel = " [DUPLICATE]"
		}
		logrus.Infof("[DRY RUN] Would %s%s: %s -> \n%s", operation, dupLabel, job.File.SourcePath, job.DestPath)
		s.journal.UpdateStatus(job.RecordID, db.StatusDryRun, "")
		atomic.AddInt32(&s.organized, 1)
		return
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(job.DestPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		logrus.Errorf("Failed to create directory %s: %v", destDir, err)
		s.journal.UpdateStatus(job.RecordID, db.StatusFailed, err.Error())
		return
	}

	var err error
	if s.copyFiles {
		err = copyFileImpl(job.File.SourcePath, job.DestPath)
		if err == nil {
			logrus.Infof("Copied: %s -> \n%s", job.File.SourcePath, job.DestPath)
		}
	} else {
		err = moveFileImpl(job.File.SourcePath, job.DestPath)
		if err == nil {
			logrus.Infof("Moved: %s -> \n%s", job.File.SourcePath, job.DestPath)
		}
	}

	if err != nil {
		logrus.Errorf("Failed to %s file %s to %s: %v", operation, job.File.SourcePath, job.DestPath, err)
		s.journal.UpdateStatus(job.RecordID, db.StatusFailed, err.Error())
		return
	}

	s.journal.UpdateStatus(job.RecordID, db.StatusCompleted, "")
	atomic.AddInt32(&s.organized, 1)
}

func (s *MediaScanner) populateResultFromJournal() {
	stats, err := s.journal.Stats()
	if err != nil {
		logrus.Errorf("Failed to read journal stats: %v", err)
		return
	}

	s.result.OrganizedFiles = stats[db.StatusCompleted] + stats[db.StatusDryRun]
	s.result.ErrorCount = stats[db.StatusFailed]

	dupCount, err := s.journal.DuplicateCount()
	if err != nil {
		logrus.Errorf("Failed to read duplicate count: %v", err)
	} else {
		s.result.DuplicateCount = dupCount
	}

	total, err := s.journal.TotalCount()
	if err != nil {
		logrus.Errorf("Failed to read total count: %v", err)
	} else {
		s.result.TotalFiles = total
		s.result.ProcessedFiles = total
	}
}

// recordToMediaFile converts a journal FileRecord back to a MediaFile for re-queuing.
func recordToMediaFile(rec *db.FileRecord) *media.MediaFile {
	t, _ := time.Parse("2006-01-02 15:04:05", rec.CreationTime)
	return &media.MediaFile{
		SourcePath:      rec.SourcePath,
		Type:            media.MediaType(rec.MediaType),
		CreationTime:    t,
		LargerDimension: rec.LargerDimension,
		FileSize:        rec.FileSize,
		Hash:            rec.Hash,
		OriginalName:    rec.OriginalName,
	}
}

func formatSequence(num int) string {
	return fmt.Sprintf("%03d", num)
}

// moveFileImpl moves a file, falling back to copy+delete for cross-device moves.
func moveFileImpl(srcPath, destPath string) error {
	err := os.Rename(srcPath, destPath)
	if err != nil {
		if strings.Contains(err.Error(), "cross-device link") {
			if err := copyFileImpl(srcPath, destPath); err != nil {
				return err
			}
			return os.Remove(srcPath)
		}
		return err
	}
	return nil
}

func copyFileImpl(srcPath, destPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return err
	}

	if err = dst.Sync(); err != nil {
		return err
	}

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	return os.Chmod(destPath, srcInfo.Mode())
}

// GetProcessedCount returns the current count of metadata-extracted files.
func (s *MediaScanner) GetProcessedCount() int {
	return int(atomic.LoadInt32(&s.processed))
}

// GetOrganizedCount returns the current count of moved/copied files.
func (s *MediaScanner) GetOrganizedCount() int {
	return int(atomic.LoadInt32(&s.organized))
}

// GetTotalFiles returns the total count of files to process.
func (s *MediaScanner) GetTotalFiles() int {
	return s.result.TotalFiles
}

// preIndexDestinations walks all destination directories and indexes existing files
// in the journal so that cross-scan duplicates can be detected via the existing
// lazy-hash and GetByHash dedup logic.
func (s *MediaScanner) preIndexDestinations() {
	// Collect unique destination directories to scan
	destDirs := make(map[string]bool)

	if s.scheme == "date_first" && s.destination != "" {
		destDirs[s.destination] = true
	}
	for _, dir := range s.destinationDirs {
		if dir != "" {
			destDirs[dir] = true
		}
	}
	for _, dir := range s.extensionDirs {
		if dir != "" {
			destDirs[dir] = true
		}
	}
	// Include absolute duplicates dir if configured
	if filepath.IsAbs(s.duplicatesDir) {
		destDirs[s.duplicatesDir] = true
	}

	if len(destDirs) == 0 {
		return
	}

	// Clear stale dest_index rows from previous runs
	if err := s.journal.ClearDestIndex(); err != nil {
		logrus.Errorf("Failed to clear old dest_index rows: %v", err)
	}

	// Walk each destination and collect media files
	var files []db.DestFile
	visited := make(map[string]bool) // avoid duplicates from overlapping walks

	for dir := range destDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			logrus.Debugf("Destination directory does not exist yet, skipping pre-index: %s", dir)
			continue
		}

		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				// Skip source directory to avoid self-indexing
				if path == s.sourceDir || strings.HasPrefix(path, s.sourceDir+string(os.PathSeparator)) {
					return filepath.SkipDir
				}
				return nil
			}

			// Skip already-visited paths (overlapping dest dirs)
			if visited[path] {
				return nil
			}

			// Skip hidden files and journal DB files
			fileName := filepath.Base(path)
			if strings.HasPrefix(fileName, "._") {
				return nil
			}
			if strings.HasSuffix(path, ".mediaorganizer.db") ||
				strings.HasSuffix(path, ".mediaorganizer.db-wal") ||
				strings.HasSuffix(path, ".mediaorganizer.db-shm") {
				return nil
			}

			mediaType := media.DetermineMediaType(path)
			if mediaType == media.TypeUnknown {
				return nil
			}

			ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
			visited[path] = true
			files = append(files, db.DestFile{
				Path:      path,
				Size:      info.Size(),
				MediaType: string(mediaType),
				Extension: ext,
			})
			return nil
		})
	}

	if len(files) == 0 {
		logrus.Debugf("No existing files found in destination directories")
		return
	}

	inserted, err := s.journal.InsertDestFiles(files)
	if err != nil {
		logrus.Errorf("Failed to pre-index destination files: %v", err)
		return
	}
	logrus.Infof("Pre-indexed %d existing files from destination directories for duplicate detection", inserted)
}

// cleanupEmptyDirectories removes empty directories within the source directory.
func (s *MediaScanner) cleanupEmptyDirectories() {
	var emptyDirs []string
	var deletedCount int

	filepath.Walk(s.sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logrus.Errorf("Error accessing path while cleaning up: %s: %v", path, err)
			return nil
		}

		if !info.IsDir() || path == s.sourceDir {
			return nil
		}

		entries, err := os.ReadDir(path)
		if err != nil {
			logrus.Errorf("Error reading directory %s: %v", path, err)
			return nil
		}

		if len(entries) == 0 {
			emptyDirs = append(emptyDirs, path)
		}

		return nil
	})

	// Sort by length descending to remove deepest directories first
	sort.Slice(emptyDirs, func(i, j int) bool {
		return len(emptyDirs[i]) > len(emptyDirs[j])
	})

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
