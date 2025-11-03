package kklogger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigFromYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "logger.yaml")

	yamlContent := `logger:
  logger_path: /tmp/test_logs
  log_level: DEBUG
  async_write: true
  report_caller: false
  environment: production
  archive:
    max_size_bytes: 10485760
    rotation_interval: 24h
    archive_dir: old_logs
    filename_pattern: "2006-01-02"
    compression: gzip
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := loadConfigFromYAML(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.LoggerPath != "/tmp/test_logs" {
		t.Errorf("LoggerPath = %s, want /tmp/test_logs", cfg.LoggerPath)
	}

	if cfg.Level != DebugLevel {
		t.Errorf("Level = %v, want DebugLevel", cfg.Level)
	}

	if !cfg.AsyncWrite {
		t.Error("AsyncWrite should be true")
	}

	if cfg.ReportCaller {
		t.Error("ReportCaller should be false")
	}

	if cfg.Environment != "production" {
		t.Errorf("Environment = %s, want production", cfg.Environment)
	}

	if cfg.Archive == nil {
		t.Fatal("Archive config should not be nil")
	}

	if cfg.Archive.MaxSizeBytes != 10485760 {
		t.Errorf("MaxSizeBytes = %d, want 10485760", cfg.Archive.MaxSizeBytes)
	}

	if cfg.Archive.RotationInterval != 24*time.Hour {
		t.Errorf("RotationInterval = %v, want 24h", cfg.Archive.RotationInterval)
	}

	if cfg.Archive.ArchiveDir != "old_logs" {
		t.Errorf("ArchiveDir = %s, want old_logs", cfg.Archive.ArchiveDir)
	}

	if cfg.Archive.FilenamePattern != "2006-01-02" {
		t.Errorf("FilenamePattern = %s, want 2006-01-02", cfg.Archive.FilenamePattern)
	}

	if cfg.Archive.Compression != "gzip" {
		t.Errorf("Compression = %s, want gzip", cfg.Archive.Compression)
	}
}

func TestLoadConfigNonExistent(t *testing.T) {
	cfg, err := loadConfigFromYAML("/nonexistent/path/logger.yaml")

	if err != nil {
		t.Errorf("Should not return error for non-existent file, got: %v", err)
	}

	if cfg != nil {
		t.Error("Should return nil config for non-existent file")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "logger.yaml")

	invalidYAML := `logger:
  logger_path: /tmp/test
  invalid yaml content [[[
`

	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := loadConfigFromYAML(configPath)

	if err == nil {
		t.Error("Should return error for invalid YAML")
	}

	if cfg != nil {
		t.Error("Should return nil config for invalid YAML")
	}
}

func TestSaveConfigToYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "logger.yaml")

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   "/tmp/test_logs",
		AsyncWrite:   true,
		ReportCaller: false,
		Level:        InfoLevel,
		Archive: &ArchiveConfig{
			MaxSizeBytes:     5242880,
			RotationInterval: 12 * time.Hour,
			ArchiveDir:       "archives",
			FilenamePattern:  time.RFC3339,
			Compression:      "gzip",
		},
	}

	if err := saveConfigToYAML(configPath, cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file should exist after save")
	}

	loadedCfg, err := loadConfigFromYAML(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loadedCfg.LoggerPath != cfg.LoggerPath {
		t.Errorf("LoggerPath mismatch: got %s, want %s", loadedCfg.LoggerPath, cfg.LoggerPath)
	}

	if loadedCfg.Level != cfg.Level {
		t.Errorf("Level mismatch: got %v, want %v", loadedCfg.Level, cfg.Level)
	}

	if loadedCfg.Archive.MaxSizeBytes != cfg.Archive.MaxSizeBytes {
		t.Errorf("MaxSizeBytes mismatch: got %d, want %d",
			loadedCfg.Archive.MaxSizeBytes, cfg.Archive.MaxSizeBytes)
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"trace", TraceLevel},
		{"TRACE", TraceLevel},
		{"debug", DebugLevel},
		{"DEBUG", DebugLevel},
		{"info", InfoLevel},
		{"INFO", InfoLevel},
		{"warn", WarnLevel},
		{"WARN", WarnLevel},
		{"error", ErrorLevel},
		{"ERROR", ErrorLevel},
		{"invalid", TraceLevel},

		{"", TraceLevel},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseLogLevel(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEnvironmentVariableOverride(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env_override")

	os.Setenv("GOTH_LOGGER_PATH", envPath)
	defer os.Unsetenv("GOTH_LOGGER_PATH")

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   "/should/be/overridden",
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	if logger.loggerPath != envPath {
		t.Errorf("LoggerPath = %s, want %s (from env var)", logger.loggerPath, envPath)
	}

	logger.Info("env test")

	logFile := filepath.Join(envPath, "current.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Log file should exist at env var path")
	}
}

func TestConfigDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "logger.yaml")

	minimalYAML := `logger:
  logger_path: /tmp/minimal
`

	if err := os.WriteFile(configPath, []byte(minimalYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := loadConfigFromYAML(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Environment != defaultEnvironment {
		t.Errorf("Environment should default to %s, got %s", defaultEnvironment, cfg.Environment)
	}

	// When no level is specified in YAML, parseLogLevel returns TraceLevel (default case)
	// This means all log levels will be recorded by default
	if cfg.Level != TraceLevel {
		t.Errorf("Level should default to TraceLevel, got %v", cfg.Level)
	}
}

func TestArchiveConfigDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "logger.yaml")

	yamlContent := `logger:
  logger_path: /tmp/test
  archive:
    max_size_bytes: 1048576
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := loadConfigFromYAML(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Archive == nil {
		t.Fatal("Archive config should not be nil")
	}

	if cfg.Archive.ArchiveDir != "archived" {
		t.Errorf("ArchiveDir should default to 'archived', got %s", cfg.Archive.ArchiveDir)
	}

	if cfg.Archive.FilenamePattern != time.RFC3339 {
		t.Errorf("FilenamePattern should default to RFC3339, got %s", cfg.Archive.FilenamePattern)
	}

	if cfg.Archive.Compression != "gzip" {
		t.Errorf("Compression should default to 'gzip', got %s", cfg.Archive.Compression)
	}
}

func TestConfigWithoutArchive(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "logger.yaml")

	yamlContent := `logger:
  logger_path: /tmp/test
  log_level: INFO
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := loadConfigFromYAML(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Archive != nil {
		if cfg.Archive.ArchiveDir != "archived" {
			t.Errorf("Default ArchiveDir should be 'archived', got %s", cfg.Archive.ArchiveDir)
		}
	}
}

func TestGetConfigPath(t *testing.T) {
	path := getConfigPath()

	if path == "" {
		t.Error("Config path should not be empty")
	}

	if !strings.HasSuffix(path, "logger.yaml") {
		t.Errorf("Config path should end with logger.yaml, got: %s", path)
	}
}

func TestConfigFileCreation(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	os.Chdir(tmpDir)

	configPath := filepath.Join(tmpDir, "logger.yaml")

	os.Remove(configPath)

	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig should return a valid config")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file should be created by DefaultConfig")
	}

	os.Remove(configPath)
}

func TestRotationIntervalParsing(t *testing.T) {
	tests := []struct {
		name     string
		interval string
		expected time.Duration
	}{
		{"1 hour", "1h", 1 * time.Hour},
		{"24 hours", "24h", 24 * time.Hour},
		{"30 minutes", "30m", 30 * time.Minute},
		{"1 hour 30 minutes", "1h30m", 90 * time.Minute},
		{"invalid", "invalid", 0},
		{"empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "logger.yaml")

			yamlContent := fmt.Sprintf(`logger:
  logger_path: /tmp/test
  archive:
    rotation_interval: %s
`, tt.interval)

			if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			cfg, err := loadConfigFromYAML(configPath)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			if cfg.Archive == nil {
				t.Fatal("Archive config should not be nil")
			}

			if cfg.Archive.RotationInterval != tt.expected {
				t.Errorf("RotationInterval = %v, want %v",
					cfg.Archive.RotationInterval, tt.expected)
			}
		})
	}
}

func TestConfigReload(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        DebugLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	logger.Debug("initial debug")
	logger.Info("initial info")

	logger.SetLogLevel("ERROR")
	if err := logger.Reload(); err != nil {
		t.Fatalf("Failed to reload: %v", err)
	}

	logger.Debug("after reload debug")

	logger.Error("after reload error")

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)

	if !strings.Contains(contentStr, "initial debug") {
		t.Error("Should contain 'initial debug'")
	}

	if strings.Contains(contentStr, "after reload debug") {
		t.Error("Should NOT contain 'after reload debug' (filtered by level)")
	}

	if !strings.Contains(contentStr, "after reload error") {
		t.Error("Should contain 'after reload error'")
	}
}

func TestMultipleConfigFiles(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	configPath1 := filepath.Join(tmpDir1, "logger.yaml")
	configPath2 := filepath.Join(tmpDir2, "logger.yaml")

	yaml1 := `logger:
  logger_path: /tmp/test1
  log_level: DEBUG
  environment: env1
`

	yaml2 := `logger:
  logger_path: /tmp/test2
  log_level: ERROR
  environment: env2
`

	os.WriteFile(configPath1, []byte(yaml1), 0644)
	os.WriteFile(configPath2, []byte(yaml2), 0644)

	cfg1, err1 := loadConfigFromYAML(configPath1)
	cfg2, err2 := loadConfigFromYAML(configPath2)

	if err1 != nil || err2 != nil {
		t.Fatalf("Failed to load configs: %v, %v", err1, err2)
	}

	if cfg1.Environment == cfg2.Environment {
		t.Error("Configs should have different environments")
	}

	if cfg1.Level == cfg2.Level {
		t.Error("Configs should have different levels")
	}

	if cfg1.Environment != "env1" {
		t.Errorf("cfg1 environment = %s, want env1", cfg1.Environment)
	}

	if cfg2.Environment != "env2" {
		t.Errorf("cfg2 environment = %s, want env2", cfg2.Environment)
	}
}
