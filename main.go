package main

import (
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"mediaorganizer/pkg/config"
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

	// Check if the source directory exists
	logrus.Debugf("Checking if source directory exists...")
	_, err = os.Stat(cfg.SourceDir)
	if err != nil {
		logrus.Fatalf("Source directory does not exist: %s", cfg.SourceDir)
	}
	logrus.Debugf("Source directory exists")

	// Print configuration
	logrus.Infof("Media Organizer")
	logrus.Infof("Source directory: %s", cfg.SourceDir)
	for mediaType, destDir := range cfg.DestDirs {
		logrus.Infof("Destination for %s: %s", mediaType, destDir)
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
	scanner := processor.NewMediaScanner(cfg.SourceDir, cfg.DestDirs, cfg.ExtensionDirs, cfg.DryRun, cfg.CopyFiles, cfg.ConcurrentJobs, cfg.DeleteEmptyDirs)
	
	logrus.Infof("Starting scan with %d concurrent workers...", cfg.ConcurrentJobs)
	startTime := time.Now()
	
	logrus.Debugf("Beginning scan process...")
	
	// Start progress reporter in a separate goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			// This will only report approximate progress as we don't know total beforehand
			fmt.Printf("Processed files: %d\n", scanner.GetProcessedCount())
		}
	}()
	
	// Run the scanner
	logrus.Debugf("Calling scanner.Scan()...")
	result := scanner.Scan()
	
	// Print results
	logrus.Infof("Scan completed in %s", time.Since(startTime))
	logrus.Debugf("Scan complete, printing results...")
	logrus.Infof("Total files: %d", result.TotalFiles)
	logrus.Infof("Processed files: %d", result.ProcessedFiles)
	logrus.Infof("Organized files: %d", result.OrganizedFiles)
	logrus.Infof("Skipped files: %d", result.SkippedFiles)
	logrus.Infof("Errors: %d", result.ErrorCount)
	logrus.Infof("Duplicates: %d", result.DuplicateCount)
	
	// Final message to verify program completed
	logrus.Infof("Program completed successfully")
	
	// Check if log file was specified
	if cfg.LogFile != "" {
		fmt.Printf("Log file written to: %s\n", cfg.LogFile)
	}
}