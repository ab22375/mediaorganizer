package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"mediaorganizer/pkg/config"
	"mediaorganizer/pkg/db"
	"mediaorganizer/pkg/processor"
)

func main() {
	// Set log level to debug by default for troubleshooting
	logrus.SetLevel(logrus.DebugLevel)

	// Load configuration
	logrus.Debugf("Loading configuration...")
	cfg, err := config.LoadConfig()
	if err != nil {
		logrus.Fatalf("Error loading configuration: %v", err)
	}

	logrus.Debugf("Configuration loaded successfully")

	// Print all configuration values
	logrus.Debugf("Source directory: %s", cfg.SourceDir)
	for mediaType, destDir := range cfg.DestDirs {
		logrus.Debugf("Destination for %s: %s", mediaType, destDir)
	}
	logrus.Debugf("Dry run: %v", cfg.DryRun)
	logrus.Debugf("Copy files: %v", cfg.CopyFiles)
	logrus.Debugf("Delete empty dirs: %v", cfg.DeleteEmptyDirs)
	logrus.Debugf("Verbose: %v", cfg.Verbose)
	logrus.Debugf("Log file: %s", cfg.LogFile)
	logrus.Debugf("Concurrent jobs: %d", cfg.ConcurrentJobs)
	logrus.Debugf("Organization scheme: %s", cfg.OrganizationScheme)
	logrus.Debugf("Database path: %s", cfg.DBPath)

	// Check if the source directory exists
	logrus.Debugf("Checking if source directory exists...")
	_, err = os.Stat(cfg.SourceDir)
	if err != nil {
		logrus.Fatalf("Source directory does not exist: %s", cfg.SourceDir)
	}
	logrus.Debugf("Source directory exists")

	// Handle --fresh: delete existing database
	if cfg.Fresh {
		if _, err := os.Stat(cfg.DBPath); err == nil {
			logrus.Infof("Fresh start: removing existing database %s", cfg.DBPath)
			if err := os.Remove(cfg.DBPath); err != nil {
				logrus.Fatalf("Failed to remove database: %v", err)
			}
			// Also remove WAL/SHM files if present
			os.Remove(cfg.DBPath + "-wal")
			os.Remove(cfg.DBPath + "-shm")
		}
	}

	// Determine resume mode: DB exists and not fresh
	resumeMode := false
	if !cfg.Fresh {
		if _, err := os.Stat(cfg.DBPath); err == nil {
			resumeMode = true
			logrus.Infof("Existing database found, resuming from previous run")
		}
	}

	// Initialize journal
	journal, err := db.InitJournal(cfg.DBPath)
	if err != nil {
		logrus.Fatalf("Failed to initialize journal database: %v", err)
	}
	defer journal.Close()

	// Signal handler for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logrus.Infof("Received signal %v, shutting down gracefully...", sig)
		logrus.Infof("Journal database saved at: %s", cfg.DBPath)
		logrus.Infof("Re-run the same command to resume from where it left off.")
		journal.Close()
		os.Exit(1)
	}()

	// Print configuration
	logrus.Infof("Media Organizer")
	logrus.Infof("Source directory: %s", cfg.SourceDir)
	logrus.Infof("Organization scheme: %s", cfg.OrganizationScheme)
	if cfg.OrganizationScheme == config.SchemeDateFirst && cfg.Destination != "" {
		logrus.Infof("Destination: %s", cfg.Destination)
	} else {
		for mediaType, destDir := range cfg.DestDirs {
			logrus.Infof("Destination for %s: %s", mediaType, destDir)
		}
	}
	for extension, destDir := range cfg.ExtensionDirs {
		logrus.Infof("Destination for extension .%s: %s", extension, destDir)
	}
	if cfg.DryRun {
		logrus.Infof("Running in DRY-RUN mode (no files will be moved/copied)")
	} else {
		if cfg.CopyFiles {
			logrus.Infof("COPY MODE ENABLED (files will be copied instead of moved)")
		} else {
			logrus.Infof("MOVE MODE ENABLED (files will be moved from source to destination)")
			if cfg.DeleteEmptyDirs {
				logrus.Infof("DELETE EMPTY DIRS ENABLED (empty folders will be removed after moving files)")
			}
		}
	}

	// Create and start scanner
	logrus.Debugf("Creating scanner...")
	logrus.Debugf("Duplicates directory: %s", cfg.DuplicatesDir)
	scanner := processor.NewMediaScanner(cfg.SourceDir, cfg.Destination, cfg.DestDirs, cfg.ExtensionDirs, string(cfg.OrganizationScheme), cfg.SpaceReplacement, cfg.NoOriginalName, cfg.DuplicatesDir, cfg.DryRun, cfg.CopyFiles, cfg.ConcurrentJobs, cfg.DeleteEmptyDirs, journal, resumeMode)

	logrus.Infof("Starting scan with %d concurrent workers...", cfg.ConcurrentJobs)
	startTime := time.Now()

	logrus.Debugf("Beginning scan process...")

	// Start progress reporter in a separate goroutine
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				fmt.Printf("Scanned: %d/%d | Organized: %d\n", scanner.GetProcessedCount(), scanner.GetTotalFiles(), scanner.GetOrganizedCount())
			case <-done:
				return
			}
		}
	}()

	// Run the scanner
	logrus.Debugf("Calling scanner.Scan()...")
	result := scanner.Scan()
	close(done)

	// Print results
	logrus.Infof("Scan completed in %s", time.Since(startTime))
	logrus.Debugf("Scan complete, printing results...")
	logrus.Infof("Total files: %d", result.TotalFiles)
	logrus.Infof("Processed files: %d", result.ProcessedFiles)
	logrus.Infof("Organized files: %d", result.OrganizedFiles)
	logrus.Infof("Skipped files: %d", result.SkippedFiles)
	logrus.Infof("Errors: %d", result.ErrorCount)
	logrus.Infof("Duplicates: %d", result.DuplicateCount)
	logrus.Infof("Journal database: %s", cfg.DBPath)

	// Final message to verify program completed
	logrus.Infof("Program completed successfully")

	// Check if log file was specified
	if cfg.LogFile != "" {
		fmt.Printf("Log file written to: %s\n", cfg.LogFile)
	}
}
