package kklogger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestInit(t *testing.T) {
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

	if logger.environment != "test" {
		t.Errorf("Environment = %s, want test", logger.environment)
	}

	if logger.loggerPath != tmpDir {
		t.Errorf("LoggerPath = %s, want %s", logger.loggerPath, tmpDir)
	}

	if logger.level != TraceLevel {
		t.Errorf("Level = %v, want TraceLevel", logger.level)
	}

	logFile := filepath.Join(tmpDir, "current.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Log file should be created during init")
	}

	logger.Info("init test")

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "init test") {
		t.Error("Should be able to write logs after init")
	}
}

func TestReload(t *testing.T) {
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

	logger.Info("before reload")

	if err := logger.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	logger.Info("after reload")

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)

	if !strings.Contains(contentStr, "before reload") {
		t.Error("Should contain 'before reload'")
	}

	if !strings.Contains(contentStr, "after reload") {
		t.Error("Should contain 'after reload'")
	}
}

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   true,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)

	for i := 0; i < 100; i++ {
		logger.Info(fmt.Sprintf("close test %d", i))
	}

	start := time.Now()
	err := logger.Close()
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	if duration > 5*time.Second {
		t.Errorf("Close took too long: %v", duration)
	}

	if atomic.LoadInt32(&logger.closed) != 1 {
		t.Error("Closed flag should be set")
	}

	if atomic.LoadInt64(&logger.pendingLogs) != 0 {
		t.Errorf("Pending logs should be 0 after close, got %d", logger.pendingLogs)
	}

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	count := 0
	for _, line := range lines {
		if strings.Contains(line, "close test") {
			count++
		}
	}

	if count != 100 {
		t.Errorf("Expected 100 logs, got %d", count)
	}
}

func TestDoubleClose(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)

	err1 := logger.Close()
	if err1 != nil {
		t.Errorf("First close returned error: %v", err1)
	}

	err2 := logger.Close()
	if err2 != nil {
		t.Errorf("Second close returned error: %v", err2)
	}

}

func TestShutdown(t *testing.T) {

	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   true,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)

	logger.Info("before shutdown")

	logger.Close()

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "before shutdown") {
		t.Error("Should contain 'before shutdown'")
	}
}

func TestDeprecatedShutdownMethod(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)

	logger.Info("test message")

	logger.Shutdown()

	if atomic.LoadInt32(&logger.closed) != 1 {
		t.Error("Logger should be closed after Shutdown")
	}
}

func TestCloseWaitsForAllLogs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping wait test in short mode")
	}

	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   true,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)

	totalLogs := 10000
	for i := 0; i < totalLogs; i++ {
		logger.Info(fmt.Sprintf("wait test %d", i))
	}

	start := time.Now()
	logger.Close()
	duration := time.Since(start)

	t.Logf("Close duration: %v (waited for all %d logs)", duration, totalLogs)

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	count := 0
	for _, line := range lines {
		if strings.Contains(line, "wait test") {
			count++
		}
	}

	if count != totalLogs {
		t.Errorf("Data loss! Expected %d logs, found %d", totalLogs, count)
	} else {
		t.Logf("✅ All %d logs preserved", totalLogs)
	}
}

func TestReloadDuringWrite(t *testing.T) {
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

	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			logger.Info(fmt.Sprintf("concurrent write %d", i))
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	time.Sleep(50 * time.Millisecond)
	if err := logger.Reload(); err != nil {
		t.Errorf("Reload failed: %v", err)
	}

	<-done

}

func TestInitWithInvalidPath(t *testing.T) {
	invalidPath := "/invalid/readonly/path"

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   invalidPath,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	logger.Info("test with invalid path")

	if logger == nil {
		t.Error("Logger should not be nil even with invalid path")
	}
}

func TestMultipleReloads(t *testing.T) {
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

	for i := 0; i < 5; i++ {
		logger.Info(fmt.Sprintf("before reload %d", i))

		if err := logger.Reload(); err != nil {
			t.Fatalf("Reload %d failed: %v", i, err)
		}

		logger.Info(fmt.Sprintf("after reload %d", i))
	}

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)

	for i := 0; i < 5; i++ {
		if !strings.Contains(contentStr, fmt.Sprintf("before reload %d", i)) {
			t.Errorf("Should contain 'before reload %d'", i)
		}
		if !strings.Contains(contentStr, fmt.Sprintf("after reload %d", i)) {
			t.Errorf("Should contain 'after reload %d'", i)
		}
	}
}

func TestCloseWithPendingLogs(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   true,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)

	count := 5000
	for i := 0; i < count; i++ {
		logger.Info(fmt.Sprintf("pending log %d", i))
	}

	logger.Close()

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	actualCount := 0
	for _, line := range lines {
		if strings.Contains(line, "pending log") {
			actualCount++
		}
	}

	if actualCount != count {
		t.Errorf("Expected %d logs, got %d (some logs were lost)", count, actualCount)
	}
}

func TestInitOnce(t *testing.T) {
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

	logger.Info("first message")

	logger.Reload()
	logger.Info("after first reload")

	logger.Reload()
	logger.Info("after second reload")

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)

	if !strings.Contains(contentStr, "first message") {
		t.Error("Should contain 'first message'")
	}
	if !strings.Contains(contentStr, "after first reload") {
		t.Error("Should contain 'after first reload'")
	}
	if !strings.Contains(contentStr, "after second reload") {
		t.Error("Should contain 'after second reload'")
	}
}

func TestFileHandleManagement(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)

	if logger.file == nil {
		t.Error("File handle should not be nil after init")
	}

	logger.Info("test message")

	oldFile := logger.file
	logger.Reload()

	if logger.file == oldFile {
		t.Error("File handle should be different after reload")
	}

	logger.Close()

	logger.Info("after close")
}

func TestArchiveMonitorLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping archive monitor test in short mode")
	}

	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
		Archive: &ArchiveConfig{
			MaxSizeBytes:     1024,
			RotationInterval: 1 * time.Second,
			ArchiveDir:       "archived",
			FilenamePattern:  time.RFC3339,
			Compression:      "gzip",
		},
	}

	logger := NewWithConfig(cfg)

	logger.Info("test message")

	time.Sleep(100 * time.Millisecond)

	logger.Close()

	select {
	case <-logger.archiveStop:
	default:
		t.Error("archiveStop channel should be closed after Close")
	}
}

func TestGracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   true,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)

	for i := 0; i < 1000; i++ {
		logger.Info(fmt.Sprintf("graceful shutdown test %d", i))
	}

	start := time.Now()
	logger.Close()
	duration := time.Since(start)

	t.Logf("Graceful shutdown took: %v", duration)

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	count := 0
	for _, line := range lines {
		if strings.Contains(line, "graceful shutdown test") {
			count++
		}
	}

	if count != 1000 {
		t.Errorf("Expected 1000 logs after graceful shutdown, got %d", count)
	}
}
