package kklogger

import "context"

// Logger defines the basic logging interface
// This is the minimal interface that all loggers must implement
type Logger interface {
	Trace(args ...interface{})
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
}

// FormattedLogger defines the formatted logging interface
// Supports fmt.Sprintf-style formatting
type FormattedLogger interface {
	Tracef(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// StructuredLogger defines the structured logging interface
// Supports JSON-formatted structured logging with type and data
type StructuredLogger interface {
	TraceJ(typeName string, obj interface{})
	DebugJ(typeName string, obj interface{})
	InfoJ(typeName string, obj interface{})
	WarnJ(typeName string, obj interface{})
	ErrorJ(typeName string, obj interface{})
}

// ContextLogger defines the context-aware logging interface
// Supports logging with context.Context for tracing and metadata
type ContextLogger interface {
	TraceContext(ctx context.Context, args ...interface{})
	DebugContext(ctx context.Context, args ...interface{})
	InfoContext(ctx context.Context, args ...interface{})
	WarnContext(ctx context.Context, args ...interface{})
	ErrorContext(ctx context.Context, args ...interface{})
}

// ExtendedLogger defines the complete logging interface with all features
// Combines basic logging, formatted logging, structured logging, and context support
type ExtendedLogger interface {
	Logger
	FormattedLogger
	StructuredLogger
	ContextLogger
}

type LoggerHook interface {
	Logger
}

type ExtendedLoggerHook interface {
	TraceWithCaller(funcName, file string, line int, args ...interface{})
	DebugWithCaller(funcName, file string, line int, args ...interface{})
	InfoWithCaller(funcName, file string, line int, args ...interface{})
	WarnWithCaller(funcName, file string, line int, args ...interface{})
	ErrorWithCaller(funcName, file string, line int, args ...interface{})
}

type DefaultLoggerHook struct {
}

func (h *DefaultLoggerHook) Trace(args ...interface{}) {
}

func (h *DefaultLoggerHook) Debug(args ...interface{}) {
}

func (h *DefaultLoggerHook) Info(args ...interface{}) {
}

func (h *DefaultLoggerHook) Warn(args ...interface{}) {
}

func (h *DefaultLoggerHook) Error(args ...interface{}) {
}
