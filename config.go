package kklogger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

var ConfigFileName = "logger.yaml"

// YAMLConfig represents the YAML configuration structure
type YAMLConfig struct {
	Logger struct {
		LoggerPath   string `yaml:"logger_path"`
		LogLevel     string `yaml:"log_level"`
		AsyncWrite   *bool  `yaml:"async_write"`
		ReportCaller *bool  `yaml:"report_caller"`
		Environment  string `yaml:"environment"`
		Archive      struct {
			MaxSizeBytes     int64  `yaml:"max_size_bytes"`
			RotationInterval string `yaml:"rotation_interval"`
			ArchiveDir       string `yaml:"archive_dir"`
			FilenamePattern  string `yaml:"filename_pattern"`
			Compression      string `yaml:"compression"`
		} `yaml:"archive"`
	} `yaml:"logger"`
}

// loadConfigFromYAML loads configuration from logger.yaml file
// Returns nil if file doesn't exist (not an error)
func loadConfigFromYAML(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var yamlCfg YAMLConfig
	if err := yaml.Unmarshal(data, &yamlCfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	cfg := &Config{
		LoggerPath:  yamlCfg.Logger.LoggerPath,
		Environment: yamlCfg.Logger.Environment,
		Level:       parseLogLevel(yamlCfg.Logger.LogLevel),
		Archive:     parseArchiveConfig(&yamlCfg.Logger.Archive),
	}

	// Fill in missing values from global variables
	if cfg.LoggerPath == "" {
		cfg.LoggerPath = LoggerPath
	}
	if cfg.Environment == "" {
		cfg.Environment = Environment
	}
	// Use YAML value if set, otherwise use global variable
	if yamlCfg.Logger.AsyncWrite != nil {
		cfg.AsyncWrite = *yamlCfg.Logger.AsyncWrite
	} else {
		cfg.AsyncWrite = AsyncWrite
	}
	if yamlCfg.Logger.ReportCaller != nil {
		cfg.ReportCaller = *yamlCfg.Logger.ReportCaller
	} else {
		cfg.ReportCaller = ReportCaller
	}

	return cfg, nil
}

// saveConfigToYAML saves the current configuration to logger.yaml
func saveConfigToYAML(configPath string, cfg *Config) error {
	yamlCfg := YAMLConfig{}
	yamlCfg.Logger.LoggerPath = cfg.LoggerPath
	yamlCfg.Logger.LogLevel = cfg.Level.String()
	asyncWrite := cfg.AsyncWrite
	yamlCfg.Logger.AsyncWrite = &asyncWrite
	reportCaller := cfg.ReportCaller
	yamlCfg.Logger.ReportCaller = &reportCaller
	yamlCfg.Logger.Environment = cfg.Environment

	if cfg.Archive != nil {
		yamlCfg.Logger.Archive.MaxSizeBytes = cfg.Archive.MaxSizeBytes
		yamlCfg.Logger.Archive.RotationInterval = cfg.Archive.RotationInterval.String()
		yamlCfg.Logger.Archive.ArchiveDir = cfg.Archive.ArchiveDir
		yamlCfg.Logger.Archive.FilenamePattern = cfg.Archive.FilenamePattern
		yamlCfg.Logger.Archive.Compression = cfg.Archive.Compression
	}

	data, err := yaml.Marshal(&yamlCfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// parseLogLevel converts string log level to Level type
func parseLogLevel(level string) Level {
	switch level {
	case "trace", "TRACE":
		return TraceLevel
	case "debug", "DEBUG":
		return DebugLevel
	case "info", "INFO":
		return InfoLevel
	case "warn", "WARN":
		return WarnLevel
	case "error", "ERROR":
		return ErrorLevel
	default:
		return TraceLevel
	}
}

// getConfigPath returns the path to the logger.yaml file
// Looks in current working directory
func getConfigPath() string {
	wd, err := os.Getwd()
	if err != nil {
		return ConfigFileName
	}
	return filepath.Join(wd, ConfigFileName)
}

// parseArchiveConfig converts YAML archive config to ArchiveConfig
func parseArchiveConfig(yamlArchive *struct {
	MaxSizeBytes     int64  `yaml:"max_size_bytes"`
	RotationInterval string `yaml:"rotation_interval"`
	ArchiveDir       string `yaml:"archive_dir"`
	FilenamePattern  string `yaml:"filename_pattern"`
	Compression      string `yaml:"compression"`
}) *ArchiveConfig {
	if yamlArchive == nil {
		return nil
	}

	maxSize := yamlArchive.MaxSizeBytes
	if maxSize == 0 {
		maxSize = ArchiveMaxSizeBytes
	}

	rotationInterval := ArchiveRotationInterval
	if yamlArchive.RotationInterval != "" {
		if d, err := time.ParseDuration(yamlArchive.RotationInterval); err == nil {
			rotationInterval = d
		}
	}

	archiveDir := yamlArchive.ArchiveDir
	if archiveDir == "" {
		archiveDir = ArchiveDir
	}

	filenamePattern := yamlArchive.FilenamePattern
	if filenamePattern == "" {
		filenamePattern = ArchiveFilenamePattern
	}

	compression := yamlArchive.Compression
	if compression == "" {
		compression = ArchiveCompression
	}

	cfg := &ArchiveConfig{
		MaxSizeBytes:     maxSize,
		RotationInterval: rotationInterval,
		ArchiveDir:       archiveDir,
		FilenamePattern:  filenamePattern,
		Compression:      compression,
	}

	return cfg
}
