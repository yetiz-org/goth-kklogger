package kklogger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHookPanicRecovery(t *testing.T) {
	tmpDir := t.TempDir()

	panicHook := &PanicHook{}

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
		Hooks:        []LoggerHook{panicHook},
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	logger.Info("test with panic hook")

	logger.Info("after panic hook")

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "test with panic hook") {
		t.Error("Should contain first log message")
	}

	if !strings.Contains(string(content), "after panic hook") {
		t.Error("Should contain second log message")
	}
}

type PanicHook struct{}

func (h *PanicHook) Trace(args ...interface{}) { panic("trace panic") }
func (h *PanicHook) Debug(args ...interface{}) { panic("debug panic") }
func (h *PanicHook) Info(args ...interface{})  { panic("info panic") }
func (h *PanicHook) Warn(args ...interface{})  { panic("warn panic") }
func (h *PanicHook) Error(args ...interface{}) { panic("error panic") }

func TestNilConfig(t *testing.T) {
	logger := NewWithConfig(nil)
	defer logger.Close()

	if logger == nil {
		t.Fatal("Logger should not be nil even with nil config")
	}

	logger.Info("test with nil config")

}

func TestEmptyEnvironment(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment: "",

		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	logger.Info("test message")

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse log JSON: %v", err)
	}

	if _, ok := logEntry["env"]; !ok {
		t.Error("env field should exist even if empty")
	}
}

func TestVeryLongMessage(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	longMessage := strings.Repeat("A", 10240)
	logger.Info(longMessage)

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Should have written long message")
	}

	if !strings.Contains(string(content), longMessage) {
		t.Error("Long message should be complete")
	}
}

func TestSpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	specialChars := []string{
		"newline\ntest",
		"tab\ttest",
		"quote\"test",
		"backslash\\test",
		"unicode: Chinese test 🎉",
		"null\x00byte",
	}

	for _, msg := range specialChars {
		logger.Info(msg)
	}

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Should have written special character messages")
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
			t.Errorf("Failed to parse log line as JSON: %v\nLine: %s", err, line)
		}
	}
}

func TestNilArgs(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	logger.Info(nil)
	logger.Debug(nil, nil)
	logger.Error(nil, "test", nil)

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Should have some output even with nil args")
	}
}

func TestContextMethods(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	ctx := context.Background()

	logger.TraceContext(ctx, "trace with context")
	logger.DebugContext(ctx, "debug with context")
	logger.InfoContext(ctx, "info with context")
	logger.WarnContext(ctx, "warn with context")
	logger.ErrorContext(ctx, "error with context")

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	expectedMessages := []string{
		"trace with context",
		"debug with context",
		"info with context",
		"warn with context",
		"error with context",
	}

	for _, msg := range expectedMessages {
		if !strings.Contains(string(content), msg) {
			t.Errorf("Should contain '%s'", msg)
		}
	}
}

func TestDeprecatedFMethods(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	logger.TraceF(func() interface{} { return "trace from func" })
	logger.DebugF(func() interface{} { return "debug from func" })
	logger.InfoF(func() interface{} { return "info from func" })
	logger.WarnF(func() interface{} { return "warn from func" })
	logger.ErrorF(func() interface{} { return "error from func" })

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	expectedMessages := []string{
		"trace from func",
		"debug from func",
		"info from func",
		"warn from func",
		"error from func",
	}

	for _, msg := range expectedMessages {
		if !strings.Contains(string(content), msg) {
			t.Errorf("Should contain '%s'", msg)
		}
	}
}

func TestFieldClashes(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	clashMsg := map[string]interface{}{
		"time":     "custom time",
		"msg":      "custom msg",
		"level":    "custom level",
		"func":     "custom func",
		"file":     "custom file",
		"severity": "custom severity",
	}

	logger.InfoJ("clash_test", clashMsg)

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse log JSON: %v", err)
	}

	if _, ok := logEntry["time"]; !ok {
		t.Error("time field should exist")
	}

	if _, ok := logEntry["level"]; !ok {
		t.Error("level field should exist")
	}

}

func TestInvalidLogLevel(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	logger.SetLogLevel("INVALID")
	logger.SetLogLevel("")
	logger.SetLogLevel("123")

	if logger.GetLogLevel() != TraceLevel {
		t.Errorf("Invalid log level should fallback to TraceLevel, got %v", logger.GetLogLevel())
	}

	logger.Info("test after invalid level")

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "test after invalid level") {
		t.Error("Should still be able to log after invalid level")
	}
}

func TestConcurrentHookModification(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   true,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.SetLoggerHooks([]LoggerHook{&DefaultLoggerHook{}})
		}()
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			logger.Info(fmt.Sprintf("concurrent hook mod %d", idx))
		}(i)
	}

	wg.Wait()

}

func TestZeroSizeArchive(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
		Archive: &ArchiveConfig{
			MaxSizeBytes: 0,

			RotationInterval: 0,

			ArchiveDir:      "archived",
			FilenamePattern: time.RFC3339,
			Compression:     "gzip",
		},
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	for i := 0; i < 1000; i++ {
		logger.Info(fmt.Sprintf("zero size archive test %d", i))
	}

	time.Sleep(500 * time.Millisecond)

	archiveDir := filepath.Join(tmpDir, "archived")
	if entries, err := os.ReadDir(archiveDir); err == nil {
		if len(entries) > 0 {
			t.Error("Should not have archived files with zero size config")
		}
	}
}

func TestNegativeArchiveSize(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
		Archive: &ArchiveConfig{
			MaxSizeBytes: -1024,

			RotationInterval: 0,
			ArchiveDir:       "archived",
			FilenamePattern:  time.RFC3339,
			Compression:      "gzip",
		},
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	for i := 0; i < 100; i++ {
		logger.Info(fmt.Sprintf("negative size test %d", i))
	}

}

func TestEmptyArchiveDir(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
		Archive: &ArchiveConfig{
			MaxSizeBytes:     512,
			RotationInterval: 0,
			ArchiveDir:       "",

			FilenamePattern: time.RFC3339,
			Compression:     "gzip",
		},
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	for i := 0; i < 50; i++ {
		logger.Info(fmt.Sprintf("empty dir test %d", i))
	}

	time.Sleep(500 * time.Millisecond)

	archiveDir := filepath.Join(tmpDir, "archived")
	if _, err := os.Stat(archiveDir); err == nil {
		entries, _ := os.ReadDir(archiveDir)
		t.Logf("Archive directory exists with %d files", len(entries))
	} else {
		t.Logf("Archive directory not created (archive may not have triggered)")
	}
}

func TestInvalidCompressionType(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
		Archive: &ArchiveConfig{
			MaxSizeBytes:     512,
			RotationInterval: 0,
			ArchiveDir:       "archived",
			FilenamePattern:  time.RFC3339,
			Compression:      "invalid_compression",
		},
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	for i := 0; i < 50; i++ {
		logger.Info(fmt.Sprintf("invalid compression test %d", i))
	}

	time.Sleep(500 * time.Millisecond)

}

func TestRapidReload(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	for i := 0; i < 10; i++ {
		if err := logger.Reload(); err != nil {
			t.Errorf("Reload %d failed: %v", i, err)
		}
	}

	logger.Info("after rapid reload")

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "after rapid reload") {
		t.Error("Should be able to log after rapid reload")
	}
}

func TestPackageLevelFunctions(t *testing.T) {
	wd, _ := os.Getwd()
	configPath := filepath.Join(wd, "logger.yaml")
	os.Remove(configPath)
	defer os.Remove(configPath)

	SetLogLevel("INFO")

	Info("package level info")
	Debug("package level debug")

	Error("package level error")

	InfoJ("test_type", map[string]interface{}{"key": "value"})

	Trace("package trace")
	Warn("package warn")

}

func TestLoggerFilePath(t *testing.T) {
	os.Unsetenv("GOTH_LOGGER_PATH")
	path1 := LoggerFilePath()
	if path1 != LoggerPath {
		t.Errorf("LoggerFilePath() = %s, want %s", path1, LoggerPath)
	}

	testPath := "/tmp/test_logger_path"
	os.Setenv("GOTH_LOGGER_PATH", testPath)
	defer os.Unsetenv("GOTH_LOGGER_PATH")

	path2 := LoggerFilePath()
	if path2 != testPath {
		t.Errorf("LoggerFilePath() = %s, want %s", path2, testPath)
	}
}

func TestGetSeverity(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{TraceLevel, "TRACE"},
		{DebugLevel, "DEBUG"},
		{InfoLevel, "INFO"},
		{WarnLevel, "WARNING"},
		{ErrorLevel, "ERROR"},
		{Level(999), "DEFAULT"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := getSeverity(tt.level)
			if result != tt.expected {
				t.Errorf("getSeverity(%v) = %s, want %s", tt.level, result, tt.expected)
			}
		})
	}
}

func TestSimpleJsonMsg(t *testing.T) {
	msg := SimpleJsonMsg("test data")

	if msg.Type != "" {
		t.Errorf("SimpleJsonMsg should have empty Type, got %s", msg.Type)
	}

	if msg.Data != "test data" {
		t.Errorf("Data = %v, want 'test data'", msg.Data)
	}

	marshaled := msg.Marshal()
	if marshaled == nil {
		t.Error("Marshal should not return nil")
	}

	var unmarshaled JsonMsg
	if err := json.Unmarshal(marshaled, &unmarshaled); err != nil {
		t.Errorf("Failed to unmarshal: %v", err)
	}
}

func TestMarshalFailure(t *testing.T) {
	invalidData := make(chan int)
	msg := NewJsonMsg("test", invalidData)

	marshaled := msg.Marshal()
	if marshaled != nil {
		t.Error("Marshal should return nil for invalid data")
	}
}

func TestExtendedLoggerInterface(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	var _ ExtendedLogger = logger

	logger.Trace("trace")
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	logger.Tracef("tracef %d", 1)
	logger.Debugf("debugf %d", 2)
	logger.Infof("infof %d", 3)
	logger.Warnf("warnf %d", 4)
	logger.Errorf("errorf %d", 5)

	logger.TraceJ("type", "data")
	logger.DebugJ("type", "data")
	logger.InfoJ("type", "data")
	logger.WarnJ("type", "data")
	logger.ErrorJ("type", "data")

	ctx := context.Background()
	logger.TraceContext(ctx, "trace ctx")
	logger.DebugContext(ctx, "debug ctx")
	logger.InfoContext(ctx, "info ctx")
	logger.WarnContext(ctx, "warn ctx")
	logger.ErrorContext(ctx, "error ctx")

}
