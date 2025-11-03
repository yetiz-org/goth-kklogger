package kklogger

import (
	"fmt"
	"strings"
	"testing"
)

type recordingHook struct {
	basicCalls    []string
	extendedCalls []callInfo
}

type callInfo struct {
	level    Level
	funcName string
	file     string
	line     int
	msg      string
}

func (h *recordingHook) Trace(args ...interface{}) {
	h.basicCalls = append(h.basicCalls, "basic:trace")
}

func (h *recordingHook) Debug(args ...interface{}) {
	h.basicCalls = append(h.basicCalls, "basic:debug")
}

func (h *recordingHook) Info(args ...interface{}) {
	h.basicCalls = append(h.basicCalls, "basic:info")
}

func (h *recordingHook) Warn(args ...interface{}) {
	h.basicCalls = append(h.basicCalls, "basic:warn")
}

func (h *recordingHook) Error(args ...interface{}) {
	h.basicCalls = append(h.basicCalls, "basic:error")
}

func (h *recordingHook) TraceWithCaller(funcName, file string, line int, args ...interface{}) {
	h.extendedCalls = append(h.extendedCalls, callInfo{TraceLevel, funcName, file, line, fmt.Sprint(args...)})
}

func (h *recordingHook) DebugWithCaller(funcName, file string, line int, args ...interface{}) {
	h.extendedCalls = append(h.extendedCalls, callInfo{DebugLevel, funcName, file, line, fmt.Sprint(args...)})
}

func (h *recordingHook) InfoWithCaller(funcName, file string, line int, args ...interface{}) {
	h.extendedCalls = append(h.extendedCalls, callInfo{InfoLevel, funcName, file, line, fmt.Sprint(args...)})
}

func (h *recordingHook) WarnWithCaller(funcName, file string, line int, args ...interface{}) {
	h.extendedCalls = append(h.extendedCalls, callInfo{WarnLevel, funcName, file, line, fmt.Sprint(args...)})
}

func (h *recordingHook) ErrorWithCaller(funcName, file string, line int, args ...interface{}) {
	h.extendedCalls = append(h.extendedCalls, callInfo{ErrorLevel, funcName, file, line, fmt.Sprint(args...)})
}

func TestHookIntegrationWithCaller(t *testing.T) {
	tmpDir := t.TempDir()
	hook := &recordingHook{}

	logger := NewWithConfig(&Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: true,
		Level:        TraceLevel,
		Hooks:        []LoggerHook{hook},
	})
	defer logger.Close()

	logger.Info("test message")

	if len(hook.basicCalls) > 0 {
		t.Error("Basic hook methods should not be called when ExtendedLoggerHook is implemented and ReportCaller is true")
	}

	if len(hook.extendedCalls) != 1 {
		t.Errorf("Expected 1 extended call, got %d", len(hook.extendedCalls))
	}

	call := hook.extendedCalls[0]
	if call.level != InfoLevel {
		t.Errorf("Expected InfoLevel, got %v", call.level)
	}

	if call.funcName == "" {
		t.Error("Function name should not be empty")
	}

	if call.file == "" {
		t.Error("File should not be empty")
	}

	if call.line == 0 {
		t.Error("Line should not be zero")
	}

	if strings.Contains(call.funcName, "globalAsyncWorker") || strings.Contains(call.funcName, "processAsyncLogTask") {
		t.Errorf("Caller should not be from worker goroutine, got: %s", call.funcName)
	}
}

func TestHookIntegrationWithoutCaller(t *testing.T) {
	tmpDir := t.TempDir()
	hook := &recordingHook{}

	logger := NewWithConfig(&Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
		Hooks:        []LoggerHook{hook},
	})
	defer logger.Close()

	logger.Info("test message")

	if len(hook.extendedCalls) > 0 {
		t.Error("Extended hook methods should not be called when ReportCaller is false")
	}

	if len(hook.basicCalls) != 1 {
		t.Errorf("Expected 1 basic call, got %d", len(hook.basicCalls))
	}

	if hook.basicCalls[0] != "basic:info" {
		t.Errorf("Expected basic:info, got %s", hook.basicCalls[0])
	}
}

func TestHookIntegrationAllLevels(t *testing.T) {
	tmpDir := t.TempDir()
	hook := &recordingHook{}

	logger := NewWithConfig(&Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: true,
		Level:        TraceLevel,
		Hooks:        []LoggerHook{hook},
	})
	defer logger.Close()

	logger.Trace("trace")
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	if len(hook.extendedCalls) != 5 {
		t.Errorf("Expected 5 extended calls, got %d", len(hook.extendedCalls))
	}

	levels := []Level{TraceLevel, DebugLevel, InfoLevel, WarnLevel, ErrorLevel}
	for i, expected := range levels {
		if hook.extendedCalls[i].level != expected {
			t.Errorf("Call %d: expected %v, got %v", i, expected, hook.extendedCalls[i].level)
		}
	}

	for _, call := range hook.extendedCalls {
		if call.funcName == "" || call.file == "" || call.line == 0 {
			t.Error("All calls should have complete caller information")
		}
	}
}
