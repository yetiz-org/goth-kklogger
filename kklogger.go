package kklogger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultEnvironment = "default"
	defaultChannelSize = 10000
)

// ContextExtractor is a function type that extracts Fields from a context.Context
// Users can provide a custom extractor to add context-specific information to logs
// Example:
//
//	func MyExtractor(ctx context.Context) Fields {
//		return Fields{
//			"request_id": ctx.Value("request_id"),
//			"user_id":    ctx.Value("user_id"),
//		}
//	}
type ContextExtractor func(ctx context.Context) Fields

// KKLogger is an instance-based logger with independent configuration
type KKLogger struct {
	logger       *internalLogger
	file         *os.File
	environment  string
	loggerPath   string
	level        Level
	hooks        []LoggerHook
	asyncWrite   bool
	reportCaller bool
	handOver     bool
	mu           sync.Mutex
	once         sync.Once
	pendingLogs  int64 // atomic counter for pending logs
	closed       int32 // atomic flag for closed state

	// Archive related fields
	archiveConfig  *ArchiveConfig
	currentLogFile string        // Current log file path
	fileCreatedAt  time.Time     // When current file was created
	fileSizeBytes  int64         // Current file size (atomic)
	archiveTicker  *time.Ticker  // Ticker for time-based archiving
	archiveStop    chan struct{} // Signal to stop archive ticker
	archiving      int32         // Atomic flag: 1 if archive in progress

	// Context extractor for context-aware logging
	contextExtractor ContextExtractor
}

// ArchiveConfig holds archive-related configuration
type ArchiveConfig struct {
	MaxSizeBytes     int64         // Archive when file reaches this size (0 = disabled)
	RotationInterval time.Duration // Rotate log at aligned time intervals (e.g., 24h = daily at 00:00, 1h = hourly at :00, 0 = disabled)
	ArchiveDir       string        // Directory for archived files (relative to LoggerPath)
	FilenamePattern  string        // Time format pattern for archived filename (Go time format)
	Compression      string        // Compression type ("gzip" or "none")
}

// Config holds configuration for creating a new KKLogger instance
type Config struct {
	Environment  string
	LoggerPath   string
	AsyncWrite   bool
	ReportCaller bool
	Level        Level
	Hooks        []LoggerHook
	Archive      *ArchiveConfig
}

// DefaultConfig returns the default configuration
// It automatically loads from logger.yaml if it exists,
// or creates one with default values if it doesn't exist
func DefaultConfig() *Config {
	configPath := getConfigPath()

	// Try to load from YAML file
	cfg, err := loadConfigFromYAML(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kklogger: failed to load config from %s: %v, using defaults\n", configPath, err)
		cfg = nil
	}

	if cfg == nil {
		var archiveCfg *ArchiveConfig
		if ArchiveMaxSizeBytes > 0 || ArchiveRotationInterval > 0 {
			archiveCfg = &ArchiveConfig{
				MaxSizeBytes:     ArchiveMaxSizeBytes,
				RotationInterval: ArchiveRotationInterval,
				ArchiveDir:       ArchiveDir,
				FilenamePattern:  ArchiveFilenamePattern,
				Compression:      ArchiveCompression,
			}
		}

		cfg = &Config{
			Environment:  Environment,
			LoggerPath:   LoggerPath,
			AsyncWrite:   AsyncWrite,
			ReportCaller: ReportCaller,
			Level:        TraceLevel,
			Hooks:        nil,
			Archive:      archiveCfg,
		}

		if err := saveConfigToYAML(configPath, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "kklogger: failed to create default config file %s: %v\n", configPath, err)
		}
	}

	return cfg
}

// Global async worker system
var (
	globalAsyncChan    chan *asyncLogTask
	globalShutdownChan chan struct{}
	globalWorkerWg     sync.WaitGroup
	globalWorkerOnce   sync.Once
	globalShutdownOnce sync.Once  // Protects shutdown from multiple calls
	globalWorkerMu     sync.Mutex // Protects worker restart
	globalWorkerActive int32      // atomic flag
	asyncBlobPool      = sync.Pool{
		New: func() interface{} {
			return &asyncLogTask{}
		},
	}
)

// Global variables for backward compatibility
var (
	defaultLogger     *KKLogger
	defaultLoggerOnce sync.Once
)

// Deprecated global variables - kept for backward compatibility
var LoggerPath = defaultLoggerPath
var Environment = defaultEnvironment
var AsyncWrite = true
var ReportCaller = true

// Global archive configuration variables
var (
	ArchiveMaxSizeBytes     int64         = 0
	ArchiveRotationInterval time.Duration = 0
	ArchiveDir                            = "archived"
	ArchiveFilenamePattern                = time.RFC3339
	ArchiveCompression                    = "gzip"
)

type Level uint32

const (
	TraceLevel Level = iota
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
)

func (l Level) String() string {
	switch l {
	case TraceLevel:
		return "TRACE"
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	default:
		return ""
	}
}

// Lower returns lowercase level name for JSON output
func (l Level) Lower() string {
	switch l {
	case TraceLevel:
		return "trace"
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warning"
	case ErrorLevel:
		return "error"
	default:
		return "unknown"
	}
}

// New creates a new KKLogger instance with default configuration
func New() *KKLogger {
	return NewWithConfig(DefaultConfig())
}

// NewWithConfig creates a new KKLogger instance with the provided configuration
func NewWithConfig(cfg *Config) *KKLogger {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	loggerPath := cfg.LoggerPath
	if p := os.Getenv("GOTH_LOGGER_PATH"); p != "" {
		loggerPath = p
	}

	kk := &KKLogger{
		logger:        newInternalLogger(),
		environment:   cfg.Environment,
		loggerPath:    loggerPath,
		level:         cfg.Level,
		hooks:         cfg.Hooks,
		asyncWrite:    cfg.AsyncWrite,
		reportCaller:  cfg.ReportCaller,
		pendingLogs:   0,
		closed:        0,
		archiveConfig: cfg.Archive,
		archiveStop:   make(chan struct{}),
	}

	kk.init()

	return kk
}

// init initializes the logger instance
func (kk *KKLogger) init() {
	kk.once.Do(func() {
		kk.logger.SetLevel(kk.level)
		kk.logger.SetReportCaller(kk.reportCaller)

		kk.logger.onWrite = func(n int) {
			kk.updateFileSize(n)
		}

		e := os.MkdirAll(kk.loggerPath, 0755)
		if e == nil {
			logFilePath := path.Join(kk.loggerPath, "current.log")
			logFile, e := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0755)
			if e != nil {
				fmt.Fprintf(os.Stderr, "kklogger: failed to open log file: %v\n", e)
				kk.logger.SetOutput(os.Stdout)
			} else {
				kk.file = logFile
				kk.logger.SetOutput(logFile)
				kk.currentLogFile = logFilePath
				kk.fileCreatedAt = time.Now()

				if stat, err := logFile.Stat(); err == nil {
					atomic.StoreInt64(&kk.fileSizeBytes, stat.Size())
				}
			}
		} else {
			fmt.Fprintf(os.Stderr, "kklogger: can't create logger dir at %s: %v\n", kk.loggerPath, e)
			kk.logger.SetOutput(os.Stdout)
		}

		if kk.asyncWrite {
			kk.logger.SetNoLock()
			startGlobalAsyncWorker()
		}

		kk.startArchiveMonitor()

		kk.handOver = false
	})
}

// Close closes the logger and releases all resources
// It waits for all pending async logs to be written before closing
func (kk *KKLogger) Close() error {
	if !atomic.CompareAndSwapInt32(&kk.closed, 0, 1) {
		return nil
	}

	kk.stopArchiveMonitor()

	if kk.asyncWrite {
		maxWait := 5 * time.Minute // Very long timeout for real deadlock detection
		timeout := time.After(maxWait)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		startWait := time.Now()
		lastPending := atomic.LoadInt64(&kk.pendingLogs)
		noProgressCount := 0

		for {
			pending := atomic.LoadInt64(&kk.pendingLogs)
			if pending <= 0 {
				break
			}

			if pending == lastPending {
				noProgressCount++
				if noProgressCount%(10*100) == 0 {
					fmt.Fprintf(os.Stderr, "kklogger: close waiting for %d pending logs (waited %v)\n",
						pending, time.Since(startWait))
				}
			} else {
				noProgressCount = 0
				lastPending = pending
			}

			select {
			case <-timeout:
				fmt.Fprintf(os.Stderr, "CRITICAL: close timeout after %v waiting for %d pending logs - possible deadlock\n",
					maxWait, pending)
				goto closeFile
			case <-ticker.C:
			}
		}

		time.Sleep(10 * time.Millisecond)
	}

closeFile:

	if kk.file != nil {
		return kk.file.Close()
	}

	return nil
}

// Shutdown is deprecated, use Close instead
// Kept for backward compatibility
func (kk *KKLogger) Shutdown() {
	_ = kk.Close()
}

// asyncLogTask represents a log task to be processed by the global worker
type asyncLogTask struct {
	logger        *KKLogger
	logLevel      Level
	args          interface{}
	contextFields Fields // Optional context-extracted fields for *Context methods
}

// Deprecated: use asyncLogTask instead
type AsyncBlob struct {
	logLevel Level
	args     interface{}
}

// Reload reloads the logger configuration
func (kk *KKLogger) Reload() error {
	kk.mu.Lock()
	defer kk.mu.Unlock()

	kk.handOver = true
	defer func() { kk.handOver = false }()

	if kk.file != nil {
		if err := kk.file.Close(); err != nil {
			return fmt.Errorf("failed to close old log file: %w", err)
		}
	}

	kk.once = sync.Once{}
	kk.init()

	return nil
}

func LoggerFilePath() string {
	if p := os.Getenv("GOTH_LOGGER_PATH"); p == "" {
		return LoggerPath
	} else {
		return p
	}
}

// SetLoggerHooks sets the hooks for this logger instance
func (kk *KKLogger) SetLoggerHooks(hooks []LoggerHook) {
	kk.mu.Lock()
	defer kk.mu.Unlock()
	kk.hooks = hooks
}

// SetContextExtractor sets the context extractor for this logger instance
// The extractor will be called by all *Context logging methods to extract
// additional fields from the provided context
func (kk *KKLogger) SetContextExtractor(extractor ContextExtractor) {
	kk.mu.Lock()
	defer kk.mu.Unlock()
	kk.contextExtractor = extractor
}

// getLogEntry creates a new log entry for the given severity
// Each call creates a new entry to avoid race conditions in concurrent logging
func (kk *KKLogger) getLogEntry(severity string) *internalEntry {
	entry := kk.logger.WithFields(Fields{
		"env":      kk.environment,
		"severity": severity,
	})

	return entry
}

// getLogEntryWithContext creates a new log entry with context-extracted fields
// It extracts fields from the context using the configured ContextExtractor
func (kk *KKLogger) getLogEntryWithContext(ctx context.Context, severity string) *internalEntry {
	fields := Fields{
		"env":      kk.environment,
		"severity": severity,
	}

	kk.mu.Lock()
	extractor := kk.contextExtractor
	kk.mu.Unlock()

	if extractor != nil {
		if ctxFields := extractor(ctx); ctxFields != nil {
			for k, v := range ctxFields {
				fields[k] = v
			}
		}
	}

	return kk.logger.WithFields(fields)
}

// getLogEntryWithContextFields creates a new log entry with pre-extracted context fields
// This is used by async logging to avoid context expiration issues
func (kk *KKLogger) getLogEntryWithContextFields(severity string, ctxFields Fields) *internalEntry {
	fields := Fields{
		"env":      kk.environment,
		"severity": severity,
	}

	// Merge pre-extracted context fields
	if ctxFields != nil {
		for k, v := range ctxFields {
			fields[k] = v
		}
	}

	return kk.logger.WithFields(fields)
}

// LogJ logs a structured JSON message at the specified log level
func (kk *KKLogger) LogJ(logLevel Level, typeName string, obj interface{}) {
	kk.Log(logLevel, NewJsonMsg(typeName, obj))
}

// Log logs a message at the specified log level
func (kk *KKLogger) Log(logLevel Level, args ...interface{}) {
	if !kk.logger.IsLevelEnabled(logLevel) {
		return
	}

	var finalArg interface{}

	if len(args) == 0 {
		finalArg = []interface{}{""}
	} else if len(args) == 1 {
		arg := args[0]

		if str, ok := arg.(string); ok {
			jsonMsg := SimpleJsonMsg(str)
			if marshal := jsonMsg.Marshal(); marshal != nil {
				finalArg = []interface{}{string(marshal)}
			} else {
				finalArg = []interface{}{str}
			}
		} else if jsonMsg, ok := arg.(*JsonMsg); ok {
			if marshal := jsonMsg.Marshal(); marshal != nil {
				finalArg = []interface{}{string(marshal)}
			} else {
				finalArg = []interface{}{""}
			}
		} else if slice, ok := arg.([]interface{}); ok {
			finalArg = slice
		} else {
			finalArg = []interface{}{fmt.Sprint(arg)}
		}
	} else {
		finalArg = []interface{}{args}
	}

	if kk.asyncWrite {
		if atomic.LoadInt32(&kk.closed) == 1 {
			return
		}

		task := asyncBlobPool.Get().(*asyncLogTask)
		task.logger = kk
		task.logLevel = logLevel
		task.args = finalArg

		atomic.AddInt64(&kk.pendingLogs, 1)

		select {
		case globalAsyncChan <- task:
		default:
			atomic.AddInt64(&kk.pendingLogs, -1)
			asyncBlobPool.Put(task)
			kk.getLogEntry(getSeverity(logLevel)).Log(logLevel, finalArg)
			kk.runHooks(logLevel, args...)
		}
	} else {
		kk.getLogEntry(getSeverity(logLevel)).Log(logLevel, finalArg)
		kk.runHooks(logLevel, args...)
	}
}

// logWithContext logs a message with context-extracted fields at the specified log level
// This is the internal implementation used by all *Context methods
func (kk *KKLogger) logWithContext(ctx context.Context, logLevel Level, args ...interface{}) {
	if !kk.logger.IsLevelEnabled(logLevel) {
		return
	}

	var finalArg interface{}

	if len(args) == 0 {
		finalArg = []interface{}{""}
	} else if len(args) == 1 {
		arg := args[0]

		if str, ok := arg.(string); ok {
			jsonMsg := SimpleJsonMsg(str)
			if marshal := jsonMsg.Marshal(); marshal != nil {
				finalArg = []interface{}{string(marshal)}
			} else {
				finalArg = []interface{}{str}
			}
		} else if jsonMsg, ok := arg.(*JsonMsg); ok {
			if marshal := jsonMsg.Marshal(); marshal != nil {
				finalArg = []interface{}{string(marshal)}
			} else {
				finalArg = []interface{}{""}
			}
		} else if slice, ok := arg.([]interface{}); ok {
			finalArg = slice
		} else {
			finalArg = []interface{}{fmt.Sprint(arg)}
		}
	} else {
		finalArg = []interface{}{args}
	}

	var ctxFields Fields
	kk.mu.Lock()
	extractor := kk.contextExtractor
	kk.mu.Unlock()

	if extractor != nil {
		ctxFields = extractor(ctx)
	}

	if kk.asyncWrite {
		if atomic.LoadInt32(&kk.closed) == 1 {
			return
		}

		task := asyncBlobPool.Get().(*asyncLogTask)
		task.logger = kk
		task.logLevel = logLevel
		task.args = finalArg
		task.contextFields = ctxFields

		atomic.AddInt64(&kk.pendingLogs, 1)

		select {
		case globalAsyncChan <- task:
		default:
			atomic.AddInt64(&kk.pendingLogs, -1)
			asyncBlobPool.Put(task)
			kk.getLogEntryWithContextFields(getSeverity(logLevel), ctxFields).Log(logLevel, finalArg)
			kk.runHooks(logLevel, args...)
		}
	} else {
		kk.getLogEntryWithContextFields(getSeverity(logLevel), ctxFields).Log(logLevel, finalArg)
		kk.runHooks(logLevel, args...)
	}
}

// Trace logs a trace message
func (kk *KKLogger) Trace(args ...interface{}) {
	kk.Log(TraceLevel, args...)
}

// TraceJ logs a structured trace message
func (kk *KKLogger) TraceJ(typeName string, obj interface{}) {
	kk.LogJ(TraceLevel, typeName, obj)
}

// TraceF logs a trace message from a function
// Deprecated: No longer executes asynchronously to avoid goroutine leaks. Use Trace() directly.
func (kk *KKLogger) TraceF(f func() interface{}) {
	kk.Trace(f())
}

// Tracef logs a formatted trace message
func (kk *KKLogger) Tracef(format string, args ...interface{}) {
	kk.Trace(fmt.Sprintf(format, args...))
}

// TraceContext logs a trace message with context
// If a ContextExtractor is configured, it will extract additional fields from the context
func (kk *KKLogger) TraceContext(ctx context.Context, args ...interface{}) {
	kk.logWithContext(ctx, TraceLevel, args...)
}

// Debug logs a debug message
func (kk *KKLogger) Debug(args ...interface{}) {
	kk.Log(DebugLevel, args...)
}

// DebugJ logs a structured debug message
func (kk *KKLogger) DebugJ(typeName string, obj interface{}) {
	kk.LogJ(DebugLevel, typeName, obj)
}

// DebugF logs a debug message from a function
// Deprecated: No longer executes asynchronously to avoid goroutine leaks. Use Debug() directly.
func (kk *KKLogger) DebugF(f func() interface{}) {
	kk.Debug(f())
}

// Debugf logs a formatted debug message
func (kk *KKLogger) Debugf(format string, args ...interface{}) {
	kk.Debug(fmt.Sprintf(format, args...))
}

// DebugContext logs a debug message with context
// If a ContextExtractor is configured, it will extract additional fields from the context
func (kk *KKLogger) DebugContext(ctx context.Context, args ...interface{}) {
	kk.logWithContext(ctx, DebugLevel, args...)
}

// Info logs an info message
func (kk *KKLogger) Info(args ...interface{}) {
	kk.Log(InfoLevel, args...)
}

// InfoJ logs a structured info message
func (kk *KKLogger) InfoJ(typeName string, obj interface{}) {
	kk.LogJ(InfoLevel, typeName, obj)
}

// InfoF logs an info message from a function
// Deprecated: No longer executes asynchronously to avoid goroutine leaks. Use Info() directly.
func (kk *KKLogger) InfoF(f func() interface{}) {
	kk.Info(f())
}

// Infof logs a formatted info message
func (kk *KKLogger) Infof(format string, args ...interface{}) {
	kk.Info(fmt.Sprintf(format, args...))
}

// InfoContext logs an info message with context
// If a ContextExtractor is configured, it will extract additional fields from the context
func (kk *KKLogger) InfoContext(ctx context.Context, args ...interface{}) {
	kk.logWithContext(ctx, InfoLevel, args...)
}

// Warn logs a warning message
func (kk *KKLogger) Warn(args ...interface{}) {
	kk.Log(WarnLevel, args...)
}

// WarnJ logs a structured warning message
func (kk *KKLogger) WarnJ(typeName string, obj interface{}) {
	kk.LogJ(WarnLevel, typeName, obj)
}

// WarnF logs a warning message from a function
// Deprecated: No longer executes asynchronously to avoid goroutine leaks. Use Warn() directly.
func (kk *KKLogger) WarnF(f func() interface{}) {
	kk.Warn(f())
}

// Warnf logs a formatted warning message
func (kk *KKLogger) Warnf(format string, args ...interface{}) {
	kk.Warn(fmt.Sprintf(format, args...))
}

// WarnContext logs a warning message with context
// If a ContextExtractor is configured, it will extract additional fields from the context
func (kk *KKLogger) WarnContext(ctx context.Context, args ...interface{}) {
	kk.logWithContext(ctx, WarnLevel, args...)
}

// Error logs an error message
func (kk *KKLogger) Error(args ...interface{}) {
	kk.Log(ErrorLevel, args...)
}

// ErrorJ logs a structured error message
func (kk *KKLogger) ErrorJ(typeName string, obj interface{}) {
	kk.LogJ(ErrorLevel, typeName, obj)
}

// ErrorF logs an error message from a function
// Deprecated: No longer executes asynchronously to avoid goroutine leaks. Use Error() directly.
func (kk *KKLogger) ErrorF(f func() interface{}) {
	kk.Error(f())
}

// Errorf logs a formatted error message
func (kk *KKLogger) Errorf(format string, args ...interface{}) {
	kk.Error(fmt.Sprintf(format, args...))
}

// ErrorContext logs an error message with context
// If a ContextExtractor is configured, it will extract additional fields from the context
func (kk *KKLogger) ErrorContext(ctx context.Context, args ...interface{}) {
	kk.logWithContext(ctx, ErrorLevel, args...)
}

// SetLogLevel sets the log level for this logger instance
func (kk *KKLogger) SetLogLevel(logLevel string) {
	kk.mu.Lock()
	defer kk.mu.Unlock()

	kk.level = TraceLevel
	switch strings.ToUpper(logLevel) {
	case "DEBUG":
		kk.level = DebugLevel
	case "INFO":
		kk.level = InfoLevel
	case "WARN":
		kk.level = WarnLevel
	case "ERROR":
		kk.level = ErrorLevel
	}

	kk.logger.SetLevel(kk.level)
}

// GetLogLevel returns the current log level
func (kk *KKLogger) GetLogLevel() Level {
	return kk.level
}

// runHooks executes all registered hooks for the given log level
func (kk *KKLogger) runHooks(logLevel Level, args ...interface{}) {
	defer func() {
		if e := recover(); e != nil {
			if err, ok := e.(error); ok {
				fmt.Fprintf(os.Stderr, "kklogger: hook panic: %v\n", err)
			}
		}
	}()

	kk.mu.Lock()
	hooks := kk.hooks
	kk.mu.Unlock()

	for _, hook := range hooks {
		switch logLevel {
		case TraceLevel:
			hook.Trace(args...)
		case DebugLevel:
			hook.Debug(args...)
		case InfoLevel:
			hook.Info(args...)
		case WarnLevel:
			hook.Warn(args...)
		case ErrorLevel:
			hook.Error(args...)
		}
	}
}

// getSeverity converts log level to severity string
func getSeverity(logLevel Level) string {
	switch logLevel {
	case TraceLevel:
		return "TRACE"
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARNING"
	case ErrorLevel:
		return "ERROR"
	default:
		return "DEFAULT"
	}
}

type JsonMsg struct {
	Type string      `json:"type,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

func (j *JsonMsg) Marshal() []byte {
	if jsonMsgBytes, e := json.Marshal(j); e == nil {
		return jsonMsgBytes
	}

	return nil
}

func NewJsonMsg(typeName string, data interface{}) *JsonMsg {
	return &JsonMsg{
		Type: typeName,
		Data: data,
	}
}

func SimpleJsonMsg(data interface{}) *JsonMsg {
	return &JsonMsg{
		Data: data,
	}
}

// startGlobalAsyncWorker starts the global async worker
// Can be called multiple times - will restart worker if it was shut down
func startGlobalAsyncWorker() {
	if atomic.LoadInt32(&globalWorkerActive) == 1 {
		return
	}

	globalWorkerMu.Lock()
	defer globalWorkerMu.Unlock()

	if atomic.LoadInt32(&globalWorkerActive) == 1 {
		return
	}

	globalWorkerOnce = sync.Once{}

	globalWorkerOnce.Do(func() {
		globalAsyncChan = make(chan *asyncLogTask, defaultChannelSize)
		globalShutdownChan = make(chan struct{})
		atomic.StoreInt32(&globalWorkerActive, 1)

		started := make(chan struct{})
		globalWorkerWg.Add(1)
		go func() {
			close(started)
			globalAsyncWorker()
		}()
		<-started
	})
}

// globalAsyncWorker is the single goroutine that processes all async logs
func globalAsyncWorker() {
	defer globalWorkerWg.Done()

	for {
		select {
		case <-globalShutdownChan:
			for {
				select {
				case task := <-globalAsyncChan:
					processAsyncLogTask(task)
				default:
					return
				}
			}
		case task := <-globalAsyncChan:
			processAsyncLogTask(task)
		}
	}
}

// processAsyncLogTask processes a single async log task
func processAsyncLogTask(task *asyncLogTask) {
	if task == nil || task.logger == nil {
		return
	}

	logger := task.logger

	defer func() {
		atomic.AddInt64(&logger.pendingLogs, -1)
		asyncBlobPool.Put(task)
	}()

	if atomic.LoadInt32(&logger.closed) == 1 {
	}

	if task.contextFields != nil {
		logger.getLogEntryWithContextFields(getSeverity(task.logLevel), task.contextFields).Log(task.logLevel, task.args)
	} else {
		logger.getLogEntry(getSeverity(task.logLevel)).Log(task.logLevel, task.args)
	}

	if cast, ok := task.args.([]interface{}); ok {
		logger.runHooks(task.logLevel, cast...)
	} else {
		logger.runHooks(task.logLevel, task.args)
	}
}

// Package-level functions for backward compatibility

// getDefaultLogger returns the default logger instance
func getDefaultLogger() *KKLogger {
	defaultLoggerOnce.Do(func() {
		var archiveCfg *ArchiveConfig
		if ArchiveMaxSizeBytes > 0 || ArchiveRotationInterval > 0 {
			archiveCfg = &ArchiveConfig{
				MaxSizeBytes:     ArchiveMaxSizeBytes,
				RotationInterval: ArchiveRotationInterval,
				ArchiveDir:       ArchiveDir,
				FilenamePattern:  ArchiveFilenamePattern,
				Compression:      ArchiveCompression,
			}
		}

		cfg := &Config{
			Environment:  Environment,
			LoggerPath:   LoggerPath,
			AsyncWrite:   AsyncWrite,
			ReportCaller: ReportCaller,
			Level:        TraceLevel,
			Archive:      archiveCfg,
		}
		defaultLogger = NewWithConfig(cfg)
	})
	return defaultLogger
}

// Deprecated package-level functions - use instance methods instead

func SetLoggerHooks(hooks []LoggerHook) {
	getDefaultLogger().SetLoggerHooks(hooks)
}

func LogJ(logLevel Level, typeName string, obj interface{}) {
	getDefaultLogger().LogJ(logLevel, typeName, obj)
}

func Log(logLevel Level, args ...interface{}) {
	getDefaultLogger().Log(logLevel, args...)
}

func Trace(args ...interface{}) {
	getDefaultLogger().Trace(args...)
}

func TraceJ(typeName string, obj interface{}) {
	getDefaultLogger().TraceJ(typeName, obj)
}

// Deprecated: No longer executes asynchronously to avoid goroutine leaks
func TraceF(f func() interface{}) {
	getDefaultLogger().TraceF(f)
}

func Debug(args ...interface{}) {
	getDefaultLogger().Debug(args...)
}

func DebugJ(typeName string, obj interface{}) {
	getDefaultLogger().DebugJ(typeName, obj)
}

// Deprecated: No longer executes asynchronously to avoid goroutine leaks
func DebugF(f func() interface{}) {
	getDefaultLogger().DebugF(f)
}

func Info(args ...interface{}) {
	getDefaultLogger().Info(args...)
}

func InfoJ(typeName string, obj interface{}) {
	getDefaultLogger().InfoJ(typeName, obj)
}

// Deprecated: No longer executes asynchronously to avoid goroutine leaks
func InfoF(f func() interface{}) {
	getDefaultLogger().InfoF(f)
}

func Warn(args ...interface{}) {
	getDefaultLogger().Warn(args...)
}

func WarnJ(typeName string, obj interface{}) {
	getDefaultLogger().WarnJ(typeName, obj)
}

// Deprecated: No longer executes asynchronously to avoid goroutine leaks
func WarnF(f func() interface{}) {
	getDefaultLogger().WarnF(f)
}

func Error(args ...interface{}) {
	getDefaultLogger().Error(args...)
}

func ErrorJ(typeName string, obj interface{}) {
	getDefaultLogger().ErrorJ(typeName, obj)
}

// Deprecated: No longer executes asynchronously to avoid goroutine leaks
func ErrorF(f func() interface{}) {
	getDefaultLogger().ErrorF(f)
}

func SetLogLevel(logLevel string) {
	getDefaultLogger().SetLogLevel(logLevel)
}

func GetLogLevel() Level {
	return getDefaultLogger().GetLogLevel()
}

// Shutdown closes the default logger and shuts down the global async worker
// This should be called when the application is shutting down to ensure all pending logs are written
// Safe to call multiple times - subsequent calls will be no-op
func Shutdown() {
	// Close default logger first
	if defaultLogger != nil {
		defaultLogger.Shutdown()
	}

	// Shutdown global async worker (protected by Once)
	if atomic.LoadInt32(&globalWorkerActive) == 0 {
		return
	}

	globalShutdownOnce.Do(func() {
		// Signal shutdown
		close(globalShutdownChan)

		// Wait for worker to finish
		globalWorkerWg.Wait()

		atomic.StoreInt32(&globalWorkerActive, 0)
	})
}

func Reload() {
	if defaultLogger != nil {
		_ = defaultLogger.Reload()
	}
}
