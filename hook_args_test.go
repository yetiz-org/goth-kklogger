package kklogger

import (
	"strings"
	"testing"
)

// mockHook implements LoggerHook to verify argument passing
type mockHook struct {
	receivedArgs []interface{}
	callCount    int
}

func (m *mockHook) Trace(args ...interface{}) {
	m.callCount++
	m.receivedArgs = args
}

func (m *mockHook) Debug(args ...interface{}) {
	m.callCount++
	m.receivedArgs = args
}

func (m *mockHook) Info(args ...interface{}) {
	m.callCount++
	m.receivedArgs = args
}

func (m *mockHook) Warn(args ...interface{}) {
	m.callCount++
	m.receivedArgs = args
}

func (m *mockHook) Error(args ...interface{}) {
	m.callCount++
	m.receivedArgs = args
}

// TestHookArgumentPassing verifies that hooks receive arguments in the expected format
// This test ensures compatibility with hooks like GCP Logging that expect args[0] to be []interface{}
// Note: Hooks receive finalArg (processed arguments), not raw args
func TestHookArgumentPassing(t *testing.T) {
	logger := NewWithConfig(&Config{
		Environment: "test",
		LoggerPath:  "",
		AsyncWrite:  false,
		Level:       TraceLevel,
	})
	defer logger.Shutdown()

	hook := &mockHook{}
	logger.SetLoggerHooks([]LoggerHook{hook})

	tests := []struct {
		name         string
		logFunc      func()
		validateFunc func(t *testing.T, argsSlice []interface{})
	}{
		{
			name: "single string argument",
			logFunc: func() {
				logger.Info("test message")
			},
			validateFunc: func(t *testing.T, argsSlice []interface{}) {
				// Single string is wrapped in JSON format: {"data":"test message"}
				if len(argsSlice) != 1 {
					t.Errorf("Expected 1 arg, got %d", len(argsSlice))
					return
				}
				str, ok := argsSlice[0].(string)
				if !ok {
					t.Errorf("Expected string, got %T", argsSlice[0])
					return
				}
				// Should contain the message in JSON format
				if !strings.Contains(str, "test message") {
					t.Errorf("Expected message to contain 'test message', got %s", str)
				}
			},
		},
		{
			name: "formatted string with args",
			logFunc: func() {
				logger.Info("test %s %d", "message", 123)
			},
			validateFunc: func(t *testing.T, argsSlice []interface{}) {
				// Multiple args are wrapped as []interface{}{args}
				if len(argsSlice) != 1 {
					t.Errorf("Expected 1 arg (wrapped slice), got %d", len(argsSlice))
					return
				}
				innerSlice, ok := argsSlice[0].([]interface{})
				if !ok {
					t.Errorf("Expected []interface{}, got %T", argsSlice[0])
					return
				}
				if len(innerSlice) != 3 {
					t.Errorf("Expected 3 inner args, got %d", len(innerSlice))
				}
			},
		},
		{
			name: "multiple arguments",
			logFunc: func() {
				logger.Info("arg1", "arg2", "arg3")
			},
			validateFunc: func(t *testing.T, argsSlice []interface{}) {
				// Multiple args are wrapped as []interface{}{args}
				if len(argsSlice) != 1 {
					t.Errorf("Expected 1 arg (wrapped slice), got %d", len(argsSlice))
					return
				}
				innerSlice, ok := argsSlice[0].([]interface{})
				if !ok {
					t.Errorf("Expected []interface{}, got %T", argsSlice[0])
					return
				}
				if len(innerSlice) != 3 {
					t.Errorf("Expected 3 inner args, got %d", len(innerSlice))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook.receivedArgs = nil
			hook.callCount = 0

			tt.logFunc()

			if hook.callCount != 1 {
				t.Errorf("Expected hook to be called once, got %d", hook.callCount)
			}

			if len(hook.receivedArgs) != 1 {
				t.Errorf("Expected hook to receive 1 argument (the finalArg slice), got %d", len(hook.receivedArgs))
				return
			}

			// The hook should receive a single argument which is the []interface{} (finalArg)
			argsSlice, ok := hook.receivedArgs[0].([]interface{})
			if !ok {
				t.Errorf("Expected hook to receive []interface{} as first argument, got %T", hook.receivedArgs[0])
				return
			}

			tt.validateFunc(t, argsSlice)
		})
	}
}

// TestHookArgumentPassingAsync verifies async mode has same behavior
func TestHookArgumentPassingAsync(t *testing.T) {
	logger := NewWithConfig(&Config{
		Environment: "test",
		LoggerPath:  "",
		AsyncWrite:  true,
		Level:       TraceLevel,
	})
	defer logger.Shutdown()

	hook := &mockHook{}
	logger.SetLoggerHooks([]LoggerHook{hook})

	logger.Info("async test %s", "message")
	
	// Wait for async processing
	logger.Shutdown()

	if hook.callCount != 1 {
		t.Errorf("Expected hook to be called once, got %d", hook.callCount)
	}

	if len(hook.receivedArgs) != 1 {
		t.Errorf("Expected hook to receive 1 argument, got %d", len(hook.receivedArgs))
		return
	}

	argsSlice, ok := hook.receivedArgs[0].([]interface{})
	if !ok {
		t.Errorf("Expected hook to receive []interface{} as first argument, got %T", hook.receivedArgs[0])
		return
	}

	// Multiple args are wrapped as []interface{}{args}
	if len(argsSlice) != 1 {
		t.Errorf("Expected 1 arg (wrapped slice), got %d", len(argsSlice))
		return
	}
	
	innerSlice, ok := argsSlice[0].([]interface{})
	if !ok {
		t.Errorf("Expected []interface{}, got %T", argsSlice[0])
		return
	}
	
	if len(innerSlice) != 2 {
		t.Errorf("Expected 2 inner args, got %d", len(innerSlice))
	}
}
