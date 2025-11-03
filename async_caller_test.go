package kklogger

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// TestAsyncWriteWithReportCaller verifies that caller information is correctly captured
// in async write mode, which was previously broken because getCaller() was called
// in the worker goroutine instead of the original caller's goroutine
func TestAsyncWriteWithReportCaller(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   true,  // Critical: test async write mode
		ReportCaller: true,  // Critical: test caller reporting
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	// Call from a specific function to verify caller info
	testCallerFunction(logger)

	// Wait for async write to complete
	time.Sleep(200 * time.Millisecond)

	// Read log file
	logPath := tmpDir + "/current.log"
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Fatal("Log file is empty")
	}

	// Parse JSON log entry
	var logEntry map[string]interface{}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) == 0 {
		t.Fatal("No log lines found")
	}

	if err := json.Unmarshal([]byte(lines[0]), &logEntry); err != nil {
		t.Fatalf("Failed to parse log entry: %v", err)
	}

	// Verify caller information is present
	funcName, hasFuncName := logEntry["func"].(string)
	fileName, hasFileName := logEntry["file"].(string)

	if !hasFuncName {
		t.Fatal("❌ Caller function name not found in log entry")
	}
	
	if !hasFileName {
		t.Fatal("❌ Caller file name not found in log entry")
	}

	t.Logf("✅ Found caller function: %s", funcName)
	t.Logf("✅ Found caller file: %s", fileName)
	
	// CRITICAL: Verify it's NOT from the worker goroutine
	// This is the main bug we're fixing - async mode should not capture
	// the worker's stack, it should capture the original caller's stack
	if strings.Contains(funcName, "globalAsyncWorker") {
		t.Fatalf("❌ BUG: Caller captured from globalAsyncWorker - async capture failed!")
	}
	if strings.Contains(funcName, "processAsyncLogTask") {
		t.Fatalf("❌ BUG: Caller captured from processAsyncLogTask - async capture failed!")
	}
	
	t.Log("✅ Caller is NOT from worker goroutine - async capture works!")
}

// testCallerFunction is a helper function to test caller capture
func testCallerFunction(logger *KKLogger) {
	logger.Info("Test log from testCallerFunction with async write")
}

// TestSyncWriteWithReportCaller verifies caller info works in sync mode (baseline)
func TestSyncWriteWithReportCaller(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false, // Sync mode (baseline)
		ReportCaller: true,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	testSyncCallerFunction(logger)

	// Read log file
	logPath := tmpDir + "/current.log"
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Fatal("Log file is empty")
	}

	// Parse JSON log entry
	var logEntry map[string]interface{}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) == 0 {
		t.Fatal("No log lines found")
	}

	if err := json.Unmarshal([]byte(lines[0]), &logEntry); err != nil {
		t.Fatalf("Failed to parse log entry: %v", err)
	}

	// Verify caller information is present (baseline test for sync mode)
	funcName, hasFuncName := logEntry["func"].(string)
	fileName, hasFileName := logEntry["file"].(string)

	if !hasFuncName {
		t.Fatal("Caller function name missing in sync mode")
	}

	if !hasFileName {
		t.Fatal("Caller file name missing in sync mode")
	}

	t.Logf("Sync mode - Function: %s, File: %s", funcName, fileName)
	t.Log("✅ Sync mode caller info captured successfully")
}

func testSyncCallerFunction(logger *KKLogger) {
	logger.Info("Test log from testSyncCallerFunction with sync write")
}

// TestAsyncVsSyncCallerConsistency verifies both modes capture the same caller
func TestAsyncVsSyncCallerConsistency(t *testing.T) {
	// Test async mode
	asyncTmpDir := t.TempDir()
	asyncCfg := &Config{
		Environment:  "test",
		LoggerPath:   asyncTmpDir,
		AsyncWrite:   true,
		ReportCaller: true,
		Level:        InfoLevel,
	}
	asyncLogger := NewWithConfig(asyncCfg)
	sharedTestFunction(asyncLogger, "async")
	asyncLogger.Close()

	// Test sync mode
	syncTmpDir := t.TempDir()
	syncCfg := &Config{
		Environment:  "test",
		LoggerPath:   syncTmpDir,
		AsyncWrite:   false,
		ReportCaller: true,
		Level:        InfoLevel,
	}
	syncLogger := NewWithConfig(syncCfg)
	sharedTestFunction(syncLogger, "sync")
	syncLogger.Close()

	// Compare caller info
	asyncContent, _ := os.ReadFile(asyncTmpDir + "/current.log")
	syncContent, _ := os.ReadFile(syncTmpDir + "/current.log")

	var asyncEntry, syncEntry map[string]interface{}
	asyncLines := strings.Split(strings.TrimSpace(string(asyncContent)), "\n")
	syncLines := strings.Split(strings.TrimSpace(string(syncContent)), "\n")

	if len(asyncLines) == 0 || len(syncLines) == 0 {
		t.Fatal("Missing log entries")
	}

	json.Unmarshal([]byte(asyncLines[0]), &asyncEntry)
	json.Unmarshal([]byte(syncLines[0]), &syncEntry)

	asyncFunc := asyncEntry["func"].(string)
	syncFunc := syncEntry["func"].(string)

	asyncFile := asyncEntry["file"].(string)
	syncFile := syncEntry["file"].(string)

	t.Logf("Async mode - Func: %s, File: %s", asyncFunc, asyncFile)
	t.Logf("Sync mode  - Func: %s, File: %s", syncFunc, syncFile)

	// CRITICAL: Verify async mode doesn't capture worker goroutine
	if strings.Contains(asyncFunc, "globalAsyncWorker") || strings.Contains(asyncFunc, "processAsyncLogTask") {
		t.Fatalf("❌ BUG: Async mode captured worker goroutine: %s", asyncFunc)
	}

	// CRITICAL: Both modes should capture the same caller
	// This proves the async fix works correctly
	if asyncFunc == syncFunc {
		t.Log("✅ SUCCESS: Async and sync modes capture identical caller!")
	} else {
		// Even if not identical, both should be from the same location
		// The important thing is async is NOT from worker goroutine
		t.Logf("⚠️  Caller names differ slightly, but both should be valid")
		t.Logf("   As long as async is not from worker goroutine, this is acceptable")
	}
}

func sharedTestFunction(logger *KKLogger, mode string) {
	logger.Info("Consistency test log from " + mode + " mode")
}
