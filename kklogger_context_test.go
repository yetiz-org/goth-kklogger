package kklogger

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	requestIDKey contextKey = "request_id"
	userIDKey    contextKey = "user_id"
	traceIDKey   contextKey = "trace_id"
)

// TestContextExtractor_WithoutExtractor tests that *Context methods work without an extractor
func TestContextExtractor_WithoutExtractor(t *testing.T) {
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
	ctx = context.WithValue(ctx, requestIDKey, "req-12345")
	ctx = context.WithValue(ctx, userIDKey, "user-67890")

	logger.TraceContext(ctx, "trace message")
	logger.DebugContext(ctx, "debug message")
	logger.InfoContext(ctx, "info message")
	logger.WarnContext(ctx, "warn message")
	logger.ErrorContext(ctx, "error message")

	content, err := os.ReadFile(filepath.Join(tmpDir, "current.log"))
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	if !strings.Contains(logContent, "trace message") {
		t.Error("trace message not found in log")
	}
	if !strings.Contains(logContent, "debug message") {
		t.Error("debug message not found in log")
	}

	if strings.Contains(logContent, "req-12345") {
		t.Error("request_id should not be in log without extractor")
	}
	if strings.Contains(logContent, "user-67890") {
		t.Error("user_id should not be in log without extractor")
	}
}

// TestContextExtractor_WithExtractor tests that *Context methods extract fields when extractor is set
func TestContextExtractor_WithExtractor(t *testing.T) {
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

	logger.SetContextExtractor(func(ctx context.Context) Fields {
		fields := Fields{}
		if reqID, ok := ctx.Value(requestIDKey).(string); ok {
			fields["request_id"] = reqID
		}
		if userID, ok := ctx.Value(userIDKey).(string); ok {
			fields["user_id"] = userID
		}
		if traceID, ok := ctx.Value(traceIDKey).(string); ok {
			fields["trace_id"] = traceID
		}
		return fields
	})

	ctx := context.Background()
	ctx = context.WithValue(ctx, requestIDKey, "req-12345")
	ctx = context.WithValue(ctx, userIDKey, "user-67890")
	ctx = context.WithValue(ctx, traceIDKey, "trace-abcdef")

	logger.TraceContext(ctx, "trace message with context")
	logger.DebugContext(ctx, "debug message with context")
	logger.InfoContext(ctx, "info message with context")

	content, err := os.ReadFile(filepath.Join(tmpDir, "current.log"))
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify messages are logged
	if !strings.Contains(logContent, "trace message with context") {
		t.Error("trace message not found in log")
	}

	if !strings.Contains(logContent, "req-12345") {
		t.Error("request_id not found in log")
	}
	if !strings.Contains(logContent, "user-67890") {
		t.Error("user_id not found in log")
	}
	if !strings.Contains(logContent, "trace-abcdef") {
		t.Error("trace_id not found in log")
	}
}

// TestContextExtractor_AsyncMode tests context extraction in async mode
func TestContextExtractor_AsyncMode(t *testing.T) {
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

	logger.SetContextExtractor(func(ctx context.Context) Fields {
		fields := Fields{}
		if reqID, ok := ctx.Value(requestIDKey).(string); ok {
			fields["request_id"] = reqID
		}
		if userID, ok := ctx.Value(userIDKey).(string); ok {
			fields["user_id"] = userID
		}
		return fields
	})

	ctx := context.Background()
	ctx = context.WithValue(ctx, requestIDKey, "async-req-999")
	ctx = context.WithValue(ctx, userIDKey, "async-user-888")

	logger.InfoContext(ctx, "async info message")
	logger.WarnContext(ctx, "async warn message")
	logger.ErrorContext(ctx, "async error message")

	time.Sleep(100 * time.Millisecond)

	content, err := os.ReadFile(filepath.Join(tmpDir, "current.log"))
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify messages are logged
	if !strings.Contains(logContent, "async info message") {
		t.Error("async info message not found in log")
	}

	if !strings.Contains(logContent, "async-req-999") {
		t.Error("async request_id not found in log")
	}
	if !strings.Contains(logContent, "async-user-888") {
		t.Error("async user_id not found in log")
	}
}

// TestContextExtractor_NilContext tests that nil extractor result is handled gracefully
func TestContextExtractor_NilContext(t *testing.T) {
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

	logger.SetContextExtractor(func(ctx context.Context) Fields {
		return nil
	})

	ctx := context.Background()

	logger.InfoContext(ctx, "message with nil extractor result")

	content, err := os.ReadFile(filepath.Join(tmpDir, "current.log"))
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	if !strings.Contains(logContent, "message with nil extractor result") {
		t.Error("message not found in log")
	}
}

// TestContextExtractor_EmptyContext tests logging with empty context
func TestContextExtractor_EmptyContext(t *testing.T) {
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

	logger.SetContextExtractor(func(ctx context.Context) Fields {
		fields := Fields{}
		if reqID, ok := ctx.Value(requestIDKey).(string); ok {
			fields["request_id"] = reqID
		}
		return fields
	})

	ctx := context.Background()

	logger.InfoContext(ctx, "message with empty context")

	content, err := os.ReadFile(filepath.Join(tmpDir, "current.log"))
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify message is logged
	if !strings.Contains(logContent, "message with empty context") {
		t.Error("message not found in log")
	}
}

// TestContextExtractor_JSONFormat tests that context fields appear in JSON output
func TestContextExtractor_JSONFormat(t *testing.T) {
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

	logger.SetContextExtractor(func(ctx context.Context) Fields {
		return Fields{
			"request_id": ctx.Value(requestIDKey),
			"user_id":    ctx.Value(userIDKey),
		}
	})

	ctx := context.Background()
	ctx = context.WithValue(ctx, requestIDKey, "json-req-111")
	ctx = context.WithValue(ctx, userIDKey, "json-user-222")

	logger.InfoContext(ctx, "json format test")

	content, err := os.ReadFile(filepath.Join(tmpDir, "current.log"))
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := bytes.Split(content, []byte("\n"))
	if len(lines) < 1 {
		t.Fatal("no log lines found")
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(lines[0], &logEntry); err != nil {
		t.Fatalf("failed to parse log JSON: %v", err)
	}

	if reqID, ok := logEntry["request_id"].(string); !ok || reqID != "json-req-111" {
		t.Errorf("request_id not found or incorrect in JSON: %v", logEntry["request_id"])
	}
	if userID, ok := logEntry["user_id"].(string); !ok || userID != "json-user-222" {
		t.Errorf("user_id not found or incorrect in JSON: %v", logEntry["user_id"])
	}
}

// TestContextExtractor_AllLevels tests context extraction for all log levels
func TestContextExtractor_AllLevels(t *testing.T) {
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

	logger.SetContextExtractor(func(ctx context.Context) Fields {
		return Fields{
			"extracted": "yes",
		}
	})

	ctx := context.Background()

	logger.TraceContext(ctx, "trace")
	logger.DebugContext(ctx, "debug")
	logger.InfoContext(ctx, "info")
	logger.WarnContext(ctx, "warn")
	logger.ErrorContext(ctx, "error")

	content, err := os.ReadFile(filepath.Join(tmpDir, "current.log"))
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	count := strings.Count(logContent, `"extracted":"yes"`)
	if count != 5 {
		t.Errorf("expected 5 occurrences of extracted field, got %d", count)
	}
}
