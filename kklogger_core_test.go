package kklogger

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogLevels(t *testing.T) {
	tests := []struct {
		name         string
		setLevel     Level
		logLevel     Level
		shouldOutput bool
	}{
		{"TRACE logs at TRACE level", TraceLevel, TraceLevel, true},
		{"DEBUG logs at TRACE level", TraceLevel, DebugLevel, true},
		{"TRACE filtered at DEBUG level", DebugLevel, TraceLevel, false},
		{"DEBUG logs at DEBUG level", DebugLevel, DebugLevel, true},
		{"INFO filtered at ERROR level", ErrorLevel, InfoLevel, false},
		{"ERROR logs at ERROR level", ErrorLevel, ErrorLevel, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			cfg := &Config{
				Environment: "test",
				LoggerPath:  tmpDir,
				AsyncWrite:  false,

				ReportCaller: false,
				Level:        tt.setLevel,
			}

			logger := NewWithConfig(cfg)
			defer logger.Close()

			testMsg := "test message"
			logger.Log(tt.logLevel, testMsg)

			logFile := filepath.Join(tmpDir, "current.log")
			content, err := os.ReadFile(logFile)
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}

			hasOutput := len(content) > 0 && strings.Contains(string(content), testMsg)

			if hasOutput != tt.shouldOutput {
				t.Errorf("Expected output=%v, got=%v. Content: %s",
					tt.shouldOutput, hasOutput, string(content))
			}
		})
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
		lower    string
	}{
		{TraceLevel, "TRACE", "trace"},
		{DebugLevel, "DEBUG", "debug"},
		{InfoLevel, "INFO", "info"},
		{WarnLevel, "WARN", "warning"},
		{ErrorLevel, "ERROR", "error"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
			if got := tt.level.Lower(); got != tt.lower {
				t.Errorf("Lower() = %v, want %v", got, tt.lower)
			}
		})
	}
}

func TestSetLogLevel(t *testing.T) {
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

	logger.Debug("debug message 1")

	logger.SetLogLevel("ERROR")
	logger.Debug("debug message 2")

	logger.Error("error message")

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)

	if !strings.Contains(contentStr, "debug message 1") {
		t.Error("Should contain 'debug message 1'")
	}
	if strings.Contains(contentStr, "debug message 2") {
		t.Error("Should NOT contain 'debug message 2'")
	}
	if !strings.Contains(contentStr, "error message") {
		t.Error("Should contain 'error message'")
	}
}

func TestFormattedLogging(t *testing.T) {
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

	logger.Tracef("trace: %s %d", "test", 123)
	logger.Debugf("debug: %s %d", "test", 456)
	logger.Infof("info: %s %d", "test", 789)
	logger.Warnf("warn: %s %d", "test", 101)
	logger.Errorf("error: %s %d", "test", 202)

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)

	expectedStrings := []string{
		"trace: test 123",
		"debug: test 456",
		"info: test 789",
		"warn: test 101",
		"error: test 202",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(contentStr, expected) {
			t.Errorf("Should contain '%s'", expected)
		}
	}
}

func TestStructuredLogging(t *testing.T) {
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

	testData := map[string]interface{}{
		"user_id": 12345,
		"action":  "login",
		"status":  "success",
	}

	logger.InfoJ("user_action", testData)

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse log JSON: %v", err)
	}

	if logEntry["level"] != "info" {
		t.Errorf("Expected level=info, got %v", logEntry["level"])
	}

	if logEntry["env"] != "test" {
		t.Errorf("Expected env=test, got %v", logEntry["env"])
	}

	msgStr := logEntry["msg"].(string)
	if !strings.Contains(msgStr, "user_action") {
		t.Error("msg should contain 'user_action'")
	}
	if !strings.Contains(msgStr, "12345") {
		t.Error("msg should contain user_id")
	}
}

func TestJsonMsg(t *testing.T) {
	tests := []struct {
		name     string
		jsonMsg  *JsonMsg
		wantType string
		wantData interface{}
	}{
		{
			name:     "with type and data",
			jsonMsg:  NewJsonMsg("test_type", "test_data"),
			wantType: "test_type",
			wantData: "test_data",
		},
		{
			name:     "simple message",
			jsonMsg:  SimpleJsonMsg("simple_data"),
			wantType: "",
			wantData: "simple_data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.jsonMsg.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", tt.jsonMsg.Type, tt.wantType)
			}
			if tt.jsonMsg.Data != tt.wantData {
				t.Errorf("Data = %v, want %v", tt.jsonMsg.Data, tt.wantData)
			}

			marshaled := tt.jsonMsg.Marshal()
			if marshaled == nil {
				t.Error("Marshal() should not return nil")
			}

			var unmarshaled JsonMsg
			if err := json.Unmarshal(marshaled, &unmarshaled); err != nil {
				t.Errorf("Failed to unmarshal: %v", err)
			}
		})
	}
}

func TestMultipleInstances(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	logger1 := NewWithConfig(&Config{
		Environment:  "env1",
		LoggerPath:   tmpDir1,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        DebugLevel,
	})
	defer logger1.Close()

	logger2 := NewWithConfig(&Config{
		Environment:  "env2",
		LoggerPath:   tmpDir2,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        ErrorLevel,
	})
	defer logger2.Close()

	logger1.Debug("logger1 debug")
	logger1.Error("logger1 error")
	logger2.Debug("logger2 debug")

	logger2.Error("logger2 error")

	content1, _ := os.ReadFile(filepath.Join(tmpDir1, "current.log"))
	if !strings.Contains(string(content1), "logger1 debug") {
		t.Error("logger1 should contain debug message")
	}
	if !strings.Contains(string(content1), "env1") {
		t.Error("logger1 should have env1")
	}

	content2, _ := os.ReadFile(filepath.Join(tmpDir2, "current.log"))
	if strings.Contains(string(content2), "logger2 debug") {
		t.Error("logger2 should NOT contain debug message")
	}
	if !strings.Contains(string(content2), "logger2 error") {
		t.Error("logger2 should contain error message")
	}
	if !strings.Contains(string(content2), "env2") {
		t.Error("logger2 should have env2")
	}
}

func TestReportCaller(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: true,

		Level: TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	logger.Info("test with caller")

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse log JSON: %v", err)
	}

	if _, ok := logEntry["func"]; !ok {
		t.Error("Should contain 'func' field")
	}
	if _, ok := logEntry["file"]; !ok {
		t.Error("Should contain 'file' field")
	}

	fileStr := logEntry["file"].(string)
	if !strings.Contains(fileStr, ":") {
		t.Errorf("file should contain line number (format: path:line), got: %s", fileStr)
	}
}

func TestOutputToStdout(t *testing.T) {
	invalidPath := "/invalid/readonly/path"

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   invalidPath,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	logger := NewWithConfig(cfg)
	defer func() {
		logger.Close()
		os.Stderr = oldStderr
	}()

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)

	stderrOutput := buf.String()

	if !strings.Contains(stderrOutput, "can't create logger dir") {
		t.Logf("stderr output: %s", stderrOutput)
	}
}

func TestGetLogLevel(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        InfoLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	if logger.GetLogLevel() != InfoLevel {
		t.Errorf("Expected InfoLevel, got %v", logger.GetLogLevel())
	}

	logger.SetLogLevel("ERROR")

	if logger.GetLogLevel() != ErrorLevel {
		t.Errorf("Expected ErrorLevel after SetLogLevel, got %v", logger.GetLogLevel())
	}
}

func TestEmptyLog(t *testing.T) {
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

	logger.Info()
	logger.Debug("")

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Should have some output even for empty logs")
	}
}

func TestLogWithMultipleArgs(t *testing.T) {
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

	logger.Info("arg1", "arg2", "arg3")
	logger.Debug(123, 456, 789)

	logFile := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Should have output for multi-arg logs")
	}
}

func TestDefaultConfig(t *testing.T) {
	wd, _ := os.Getwd()
	configPath := filepath.Join(wd, "logger.yaml")

	var backupContent []byte
	if content, err := os.ReadFile(configPath); err == nil {
		backupContent = content
		defer os.WriteFile(configPath, backupContent, 0644)
	}

	os.Remove(configPath)

	cfg := DefaultConfig()

	if cfg.Environment != defaultEnvironment {
		t.Errorf("Expected default environment, got %s", cfg.Environment)
	}

	if cfg.LoggerPath != defaultLoggerPath {
		t.Errorf("Expected default logger path, got %s", cfg.LoggerPath)
	}

	if !cfg.AsyncWrite {
		t.Error("AsyncWrite should be true by default")
	}

	if !cfg.ReportCaller {
		t.Error("ReportCaller should be true by default")
	}

	if cfg.Level != TraceLevel {
		t.Errorf("Expected TraceLevel by default, got %v", cfg.Level)
	}

	os.Remove(configPath)
}

func TestNew(t *testing.T) {
	wd, _ := os.Getwd()
	configPath := filepath.Join(wd, "logger.yaml")
	os.Remove(configPath)

	logger := New()
	defer func() {
		logger.Close()
		os.Remove(configPath)
	}()

	if logger == nil {
		t.Fatal("New() should return a valid logger")
	}

	logger.Info("test message")

	time.Sleep(100 * time.Millisecond)
}
