package kklogger

import (
	"os"
	"strings"
	"testing"
)

type testExtendedHook struct {
	traces []string
	debugs []string
	infos  []string
	warns  []string
	errors []string
}

func (h *testExtendedHook) Trace(args ...interface{}) {
	h.traces = append(h.traces, "basic")
}

func (h *testExtendedHook) Debug(args ...interface{}) {
	h.debugs = append(h.debugs, "basic")
}

func (h *testExtendedHook) Info(args ...interface{}) {
	h.infos = append(h.infos, "basic")
}

func (h *testExtendedHook) Warn(args ...interface{}) {
	h.warns = append(h.warns, "basic")
}

func (h *testExtendedHook) Error(args ...interface{}) {
	h.errors = append(h.errors, "basic")
}

func (h *testExtendedHook) TraceWithCaller(funcName, file string, line int, args ...interface{}) {
	h.traces = append(h.traces, funcName+":"+file)
}

func (h *testExtendedHook) DebugWithCaller(funcName, file string, line int, args ...interface{}) {
	h.debugs = append(h.debugs, funcName+":"+file)
}

func (h *testExtendedHook) InfoWithCaller(funcName, file string, line int, args ...interface{}) {
	h.infos = append(h.infos, funcName+":"+file)
}

func (h *testExtendedHook) WarnWithCaller(funcName, file string, line int, args ...interface{}) {
	h.warns = append(h.warns, funcName+":"+file)
}

func (h *testExtendedHook) ErrorWithCaller(funcName, file string, line int, args ...interface{}) {
	h.errors = append(h.errors, funcName+":"+file)
}

func TestExtendedHookWithCaller(t *testing.T) {
	tmpDir := t.TempDir()
	hook := &testExtendedHook{}

	logger := NewWithConfig(&Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: true,
		Level:        TraceLevel,
		Hooks:        []LoggerHook{hook},
	})
	defer logger.Close()

	logger.Trace("trace message")
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	if len(hook.traces) != 1 {
		t.Errorf("Expected 1 trace, got %d", len(hook.traces))
	}
	if len(hook.debugs) != 1 {
		t.Errorf("Expected 1 debug, got %d", len(hook.debugs))
	}
	if len(hook.infos) != 1 {
		t.Errorf("Expected 1 info, got %d", len(hook.infos))
	}
	if len(hook.warns) != 1 {
		t.Errorf("Expected 1 warn, got %d", len(hook.warns))
	}
	if len(hook.errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(hook.errors))
	}

	if !strings.Contains(hook.traces[0], ":") {
		t.Errorf("Expected trace to contain caller info with separator, got: %s", hook.traces[0])
	}
	parts := strings.Split(hook.traces[0], ":")
	if len(parts) < 2 {
		t.Errorf("Expected trace to have function:file format, got: %s", hook.traces[0])
	}
	if parts[0] == "" || parts[1] == "" {
		t.Errorf("Expected both function and file to be non-empty, got: %s", hook.traces[0])
	}
}

func TestExtendedHookWithoutReportCaller(t *testing.T) {
	tmpDir := t.TempDir()
	hook := &testExtendedHook{}

	logger := NewWithConfig(&Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
		Hooks:        []LoggerHook{hook},
	})
	defer logger.Close()

	logger.Info("info message")

	if len(hook.infos) != 1 {
		t.Errorf("Expected 1 info, got %d", len(hook.infos))
	}

	if hook.infos[0] != "basic" {
		t.Errorf("Expected basic hook to be called when ReportCaller is false, got: %s", hook.infos[0])
	}
}

type basicHook struct {
	called bool
}

func (h *basicHook) Trace(args ...interface{}) { h.called = true }
func (h *basicHook) Debug(args ...interface{}) { h.called = true }
func (h *basicHook) Info(args ...interface{})  { h.called = true }
func (h *basicHook) Warn(args ...interface{})  { h.called = true }
func (h *basicHook) Error(args ...interface{}) { h.called = true }

func TestBasicHookBackwardCompatibility(t *testing.T) {
	tmpDir := t.TempDir()

	hook := &basicHook{}

	logger := NewWithConfig(&Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: true,
		Level:        InfoLevel,
		Hooks:        []LoggerHook{hook},
	})
	defer logger.Close()

	logger.Info("test")

	if !hook.called {
		t.Error("Basic hook should still work when ReportCaller is enabled")
	}
}

func TestExtendedHookAsync(t *testing.T) {
	tmpDir := t.TempDir()
	hook := &testExtendedHook{}

	logger := NewWithConfig(&Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   true,
		ReportCaller: true,
		Level:        TraceLevel,
		Hooks:        []LoggerHook{hook},
	})
	defer logger.Close()

	logger.Info("async test")

	if err := logger.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if len(hook.infos) != 1 {
		t.Errorf("Expected 1 info in async mode, got %d", len(hook.infos))
	}

	if !strings.Contains(hook.infos[0], ":") {
		t.Errorf("Expected async hook to contain caller info with separator, got: %s", hook.infos[0])
	}
	parts := strings.Split(hook.infos[0], ":")
	if len(parts) < 2 {
		t.Errorf("Expected async hook to have function:file format, got: %s", hook.infos[0])
	}
	if parts[0] == "" || parts[1] == "" {
		t.Errorf("Expected both function and file to be non-empty in async mode, got: %s", hook.infos[0])
	}
}

func init() {
	os.Setenv("GOTH_LOGGER_PATH", "/tmp/kklogger-test")
}
