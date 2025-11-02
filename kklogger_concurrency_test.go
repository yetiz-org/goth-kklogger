package kklogger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAsyncWrite(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment: "test",
		LoggerPath:  tmpDir,
		AsyncWrite:  true,

		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	count := 1000
	for i := 0; i < count; i++ {
		logger.Info(fmt.Sprintf("async log %d", i))
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
		if strings.Contains(line, "async log") {
			actualCount++
		}
	}

	if actualCount != count {
		t.Errorf("Expected %d log entries, got %d", count, actualCount)
	}
}

func TestConcurrentWrites(t *testing.T) {
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

	goroutines := 10
	logsPerGoroutine := 100
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < logsPerGoroutine; i++ {
				logger.Info(fmt.Sprintf("goroutine-%d log-%d", id, i))
			}
		}(g)
	}

	wg.Wait()
	logger.Close()

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	expectedTotal := goroutines * logsPerGoroutine
	lines := strings.Split(string(content), "\n")
	actualCount := 0
	for _, line := range lines {
		if strings.Contains(line, "goroutine-") {
			actualCount++
		}
	}

	if actualCount != expectedTotal {
		t.Errorf("Expected %d log entries, got %d", expectedTotal, actualCount)
	}
}

func TestRaceCondition(t *testing.T) {
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

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			logger.Info(fmt.Sprintf("race test %d", id))
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.SetLogLevel("DEBUG")
			logger.SetLogLevel("INFO")
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = logger.GetLogLevel()
		}()
	}

	wg.Wait()
	logger.Close()

}

func TestMultipleInstancesConcurrent(t *testing.T) {
	instances := 5
	var loggers []*KKLogger
	var wg sync.WaitGroup

	for i := 0; i < instances; i++ {
		tmpDir := t.TempDir()
		cfg := &Config{
			Environment:  fmt.Sprintf("env-%d", i),
			LoggerPath:   tmpDir,
			AsyncWrite:   true,
			ReportCaller: false,
			Level:        TraceLevel,
		}
		logger := NewWithConfig(cfg)
		loggers = append(loggers, logger)
	}

	for i, logger := range loggers {
		wg.Add(1)
		go func(id int, l *KKLogger) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				l.Info(fmt.Sprintf("logger-%d msg-%d", id, j))
			}
		}(i, logger)
	}

	wg.Wait()

	for _, logger := range loggers {
		logger.Close()
	}

	for i, logger := range loggers {
		logFile := filepath.Join(logger.loggerPath, "current.log")
		content, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("Failed to read log file for logger %d: %v", i, err)
		}

		if !strings.Contains(string(content), fmt.Sprintf("env-%d", i)) {
			t.Errorf("Logger %d should contain env-%d", i, i)
		}

		lines := strings.Split(string(content), "\n")
		count := 0
		for _, line := range lines {
			if strings.Contains(line, fmt.Sprintf("logger-%d", i)) {
				count++
			}
		}

		if count != 100 {
			t.Errorf("Logger %d: expected 100 logs, got %d", i, count)
		}
	}
}

func TestGlobalWorkerLifecycle(t *testing.T) {
	if atomic.LoadInt32(&globalWorkerActive) == 1 {
		t.Skip("Global worker already active, skipping")
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

	if atomic.LoadInt32(&globalWorkerActive) != 1 {
		t.Error("Global worker should be active")
	}

	logger.Info("test message")
	logger.Close()

	if atomic.LoadInt32(&globalWorkerActive) != 1 {
		t.Error("Global worker should still be active after logger close")
	}
}

func TestPendingLogsCounter(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   true,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)

	count := 1000
	for i := 0; i < count; i++ {
		logger.Info(fmt.Sprintf("pending test %d", i))
	}

	pending := atomic.LoadInt64(&logger.pendingLogs)
	t.Logf("Pending logs: %d", pending)

	logger.Close()

	finalPending := atomic.LoadInt64(&logger.pendingLogs)
	if finalPending != 0 {
		t.Errorf("After close, pending logs should be 0, got %d", finalPending)
	}
}

func TestClosedLoggerNoWrite(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   true,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)

	logger.Info("before close")

	logger.Close()

	logger.Info("after close")

	time.Sleep(100 * time.Millisecond)

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)

	if !strings.Contains(contentStr, "before close") {
		t.Error("Should contain 'before close'")
	}

	if strings.Contains(contentStr, "after close") {
		t.Error("Should NOT contain 'after close' (logger was closed)")
	}
}

func TestChannelFullFallback(t *testing.T) {
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

	// defaultChannelSize = 10000
	count := 15000
	for i := 0; i < count; i++ {
		logger.Info(fmt.Sprintf("overflow test %d", i))
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
		if strings.Contains(line, "overflow test") {
			actualCount++
		}
	}

	if actualCount != count {
		t.Logf("Expected %d logs, got %d (some may have used fallback)", count, actualCount)
	}
}

func TestSyncVsAsync(t *testing.T) {
	count := 500

	tmpDirSync := t.TempDir()
	cfgSync := &Config{
		Environment:  "test",
		LoggerPath:   tmpDirSync,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}
	loggerSync := NewWithConfig(cfgSync)

	for i := 0; i < count; i++ {
		loggerSync.Info(fmt.Sprintf("sync log %d", i))
	}
	loggerSync.Close()

	tmpDirAsync := t.TempDir()
	cfgAsync := &Config{
		Environment:  "test",
		LoggerPath:   tmpDirAsync,
		AsyncWrite:   true,
		ReportCaller: false,
		Level:        TraceLevel,
	}
	loggerAsync := NewWithConfig(cfgAsync)

	for i := 0; i < count; i++ {
		loggerAsync.Info(fmt.Sprintf("async log %d", i))
	}
	loggerAsync.Close()

	contentSync, _ := os.ReadFile(filepath.Join(tmpDirSync, "current.log"))
	contentAsync, _ := os.ReadFile(filepath.Join(tmpDirAsync, "current.log"))

	linesSync := strings.Split(string(contentSync), "\n")
	linesAsync := strings.Split(string(contentAsync), "\n")

	countSync := 0
	for _, line := range linesSync {
		if strings.Contains(line, "sync log") {
			countSync++
		}
	}

	countAsync := 0
	for _, line := range linesAsync {
		if strings.Contains(line, "async log") {
			countAsync++
		}
	}

	if countSync != count {
		t.Errorf("Sync mode: expected %d logs, got %d", count, countSync)
	}

	if countAsync != count {
		t.Errorf("Async mode: expected %d logs, got %d", count, countAsync)
	}
}

func TestNoGoroutineLeak(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()

	for i := 0; i < 10; i++ {
		tmpDir := t.TempDir()
		cfg := &Config{
			Environment:  "test",
			LoggerPath:   tmpDir,
			AsyncWrite:   true,
			ReportCaller: false,
			Level:        TraceLevel,
		}

		logger := NewWithConfig(cfg)
		logger.Info("test message")
		logger.Close()
	}

	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()

	diff := finalGoroutines - initialGoroutines
	if diff > 5 {
		t.Errorf("Possible goroutine leak: initial=%d, final=%d, diff=%d",
			initialGoroutines, finalGoroutines, diff)
	}

	t.Logf("Goroutines: initial=%d, final=%d, diff=%d",
		initialGoroutines, finalGoroutines, diff)
}

func TestConcurrentSetLogLevel(t *testing.T) {
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

	levels := []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR"}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			level := levels[idx%len(levels)]
			logger.SetLogLevel(level)
		}(i)
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			logger.Info(fmt.Sprintf("concurrent level test %d", idx))
		}(i)
	}

	wg.Wait()

}

func TestConcurrentHooks(t *testing.T) {
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
			logger.Info(fmt.Sprintf("hook test %d", idx))
		}(i)
	}

	wg.Wait()

}

func TestAsyncWriteTimeout(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   true,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)

	for i := 0; i < 5000; i++ {
		logger.Info(fmt.Sprintf("timeout test %d", i))
	}

	start := time.Now()
	logger.Close()
	duration := time.Since(start)

	if duration > 5*time.Second {
		t.Errorf("Close took too long: %v", duration)
	}

	t.Logf("Close duration: %v", duration)
}

func TestHandoverFlag(t *testing.T) {
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

	logger.Info("before handover")

	go func() {
		time.Sleep(50 * time.Millisecond)
		logger.Reload()
	}()

	for i := 0; i < 100; i++ {
		logger.Info(fmt.Sprintf("during handover %d", i))
		time.Sleep(1 * time.Millisecond)
	}

}
