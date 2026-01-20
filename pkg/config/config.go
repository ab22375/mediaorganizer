package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// OrganizationScheme defines how files are organized in the destination
type OrganizationScheme string

const (
	// SchemeExtensionFirst organizes as: <dest>/<ext>/YYYY/YYYY-MM/YYYY-MM-DD/filename
	SchemeExtensionFirst OrganizationScheme = "extension_first"
	// SchemeDateFirst organizes as: <dest>/YYYY/YYYY-MM/YYYY-MM-DD/<ext>/filename
	SchemeDateFirst OrganizationScheme = "date_first"
)

// ValidSchemes contains all valid organization scheme values
var ValidSchemes = []OrganizationScheme{SchemeExtensionFirst, SchemeDateFirst}

// IsValidScheme checks if the given scheme is valid
func IsValidScheme(scheme string) bool {
	for _, valid := range ValidSchemes {
		if string(valid) == scheme {
			return true
		}
	}
	return false
}

type Config struct {
	SourceDir          string                       `mapstructure:"source"`
	Destination        string                       `mapstructure:"destination"`
	DestDirs           map[string]string            `mapstructure:"destinations"`
	ExtensionDirs      map[string]string            `mapstructure:"extension_destinations"`
	OrganizationScheme OrganizationScheme           `mapstructure:"organization_scheme"`
	DryRun             bool                         `mapstructure:"dry_run"`
	Verbose            bool                         `mapstructure:"verbose"`
	LogFile            string                       `mapstructure:"log_file"`
	ConcurrentJobs     int                          `mapstructure:"concurrent_jobs"`
	CopyFiles          bool                         `mapstructure:"copy_files"`
	DeleteEmptyDirs    bool                         `mapstructure:"delete_empty_dirs"`
}

func LoadConfig() (*Config, error) {
	// Default configuration
	config := &Config{
		DestDirs: map[string]string{
			"image": "./output/images",
			"video": "./output/videos",
			"audio": "./output/audio",
		},
		ExtensionDirs:      make(map[string]string),
		OrganizationScheme: SchemeExtensionFirst,
		ConcurrentJobs:     4,
	}

	// Set up command line flags
	pflag.StringVarP(&config.SourceDir, "source", "s", "", "Source directory to scan for media files")
	
	// Define variables to hold command line values
	var destFlag, imageDestFlag, videoDestFlag, audioDestFlag string
	var schemeFlag string

	// Define flags with default values
	pflag.StringVar(&destFlag, "dest", "", "Unified destination directory (used with date_first scheme)")
	pflag.StringVar(&imageDestFlag, "image-dest", config.DestDirs["image"], "Destination directory for images")
	pflag.StringVar(&videoDestFlag, "video-dest", config.DestDirs["video"], "Destination directory for videos")
	pflag.StringVar(&audioDestFlag, "audio-dest", config.DestDirs["audio"], "Destination directory for audio files")
	
	pflag.BoolVarP(&config.DryRun, "dry-run", "d", false, "Simulate the organization process without moving files")
	pflag.BoolVarP(&config.Verbose, "verbose", "v", false, "Enable verbose logging")
	pflag.BoolVarP(&config.CopyFiles, "copy", "c", false, "Copy files instead of moving them")
	pflag.BoolVar(&config.DeleteEmptyDirs, "delete-empty-dirs", false, "Delete empty folders in source directory after moving files")
	pflag.StringVarP(&config.LogFile, "log-file", "l", "", "Log file path")
	pflag.IntVarP(&config.ConcurrentJobs, "jobs", "j", config.ConcurrentJobs, "Number of concurrent processing jobs")
	pflag.StringVar(&schemeFlag, "scheme", string(config.OrganizationScheme), "Organization scheme: extension_first (default) or date_first")

	configFile := pflag.String("config", "", "Path to configuration file (YAML/JSON)")
	
	pflag.Parse()

	// Read from config file first if provided
	if *configFile != "" {
		viper.SetConfigFile(*configFile)
		if err := viper.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		
		// Load config from file
		if err := viper.Unmarshal(config); err != nil {
			return nil, fmt.Errorf("error unmarshaling config: %w", err)
		}
		
		logrus.Debugf("Loaded configuration from file: %s", *configFile)
	}

	// Override with command line flags if they were explicitly set
	if pflag.Lookup("source").Changed {
		config.SourceDir = pflag.Lookup("source").Value.String()
	}

	if pflag.Lookup("dest").Changed {
		config.Destination = destFlag
	}

	if pflag.Lookup("image-dest").Changed {
		config.DestDirs["image"] = imageDestFlag
	}
	
	if pflag.Lookup("video-dest").Changed {
		config.DestDirs["video"] = videoDestFlag
	}
	
	if pflag.Lookup("audio-dest").Changed {
		config.DestDirs["audio"] = audioDestFlag
	}
	
	if pflag.Lookup("dry-run").Changed {
		config.DryRun = pflag.Lookup("dry-run").Value.String() == "true"
	}
	
	if pflag.Lookup("verbose").Changed {
		config.Verbose = pflag.Lookup("verbose").Value.String() == "true"
	}
	
	if pflag.Lookup("copy").Changed {
		config.CopyFiles = pflag.Lookup("copy").Value.String() == "true"
	}
	
	if pflag.Lookup("delete-empty-dirs").Changed {
		config.DeleteEmptyDirs = pflag.Lookup("delete-empty-dirs").Value.String() == "true"
	}
	
	if pflag.Lookup("log-file").Changed {
		config.LogFile = pflag.Lookup("log-file").Value.String()
	}
	
	if pflag.Lookup("jobs").Changed {
		val := pflag.Lookup("jobs").Value.String()
		if intVal, err := strconv.Atoi(val); err == nil {
			config.ConcurrentJobs = intVal
		}
	}

	if pflag.Lookup("scheme").Changed {
		config.OrganizationScheme = OrganizationScheme(schemeFlag)
	}

	// Validate config
	if config.SourceDir == "" {
		return nil, &ConfigError{"source directory is required"}
	}

	if !IsValidScheme(string(config.OrganizationScheme)) {
		return nil, &ConfigError{fmt.Sprintf("invalid organization scheme: %s (valid: extension_first, date_first)", config.OrganizationScheme)}
	}

	// Convert relative paths to absolute paths
	var err error
	config.SourceDir, err = filepath.Abs(config.SourceDir)
	if err != nil {
		return nil, err
	}

	if config.Destination != "" {
		config.Destination, err = filepath.Abs(config.Destination)
		if err != nil {
			return nil, err
		}
		logrus.Debugf("Final unified destination path: %s", config.Destination)
	}

	for mediaType, destDir := range config.DestDirs {
		config.DestDirs[mediaType], err = filepath.Abs(destDir)
		if err != nil {
			return nil, err
		}
		logrus.Debugf("Final destination path for %s: %s", mediaType, config.DestDirs[mediaType])
	}

	// Configure logger
	setupLogger(config)

	return config, nil
}

func setupLogger(config *Config) {
	if config.Verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	if config.LogFile != "" {
		// Create a multi-writer to send output to both the log file and stdout
		file, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			// Create a hook that sends logs to the file
			fileHook := &FileHook{
				Writer: file,
				LogLevels: []logrus.Level{
					logrus.PanicLevel,
					logrus.FatalLevel,
					logrus.ErrorLevel,
					logrus.WarnLevel,
					logrus.InfoLevel,
					logrus.DebugLevel,
				},
			}
			
			// Add the hook - this way logs go to both stdout and the file
			logrus.AddHook(fileHook)
			
			logrus.Infof("Logging to file: %s", config.LogFile)
		} else {
			logrus.Errorf("Failed to log to file: %v", err)
		}
	}
}

// FileHook sends logs to a file while maintaining stdout output
type FileHook struct {
	Writer    io.Writer
	LogLevels []logrus.Level
}

// Fire writes the log entry to the file
func (hook *FileHook) Fire(entry *logrus.Entry) error {
	line, err := entry.String()
	if err != nil {
		return err
	}
	_, err = hook.Writer.Write([]byte(line))
	return err
}

// Levels returns the log levels this hook is enabled for
func (hook *FileHook) Levels() []logrus.Level {
	return hook.LogLevels
}

type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return e.Message
}