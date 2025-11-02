package kklogger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Fields map[string]interface{}

// entryPool reduces allocations by reusing entry objects
var entryPool = sync.Pool{
	New: func() interface{} {
		return &internalEntry{
			Data: make(Fields, 4),
		}
	},
}

// bufferPool reduces allocations for JSON encoding
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

type internalLogger struct {
	out          io.Writer
	level        Level
	reportCaller bool
	noLock       bool
	mu           sync.Mutex
	onWrite      func(int)
}

func newInternalLogger() *internalLogger {
	return &internalLogger{
		out:          nil,
		level:        TraceLevel,
		reportCaller: false,
		noLock:       false,
	}
}

func (l *internalLogger) SetOutput(w io.Writer) {
	if !l.noLock {
		l.mu.Lock()
		defer l.mu.Unlock()
	}
	l.out = w
}

func (l *internalLogger) SetLevel(level Level) {
	if !l.noLock {
		l.mu.Lock()
		defer l.mu.Unlock()
	}
	l.level = level
}

func (l *internalLogger) SetReportCaller(reportCaller bool) {
	if !l.noLock {
		l.mu.Lock()
		defer l.mu.Unlock()
	}
	l.reportCaller = reportCaller
}

func (l *internalLogger) SetNoLock() {
	l.noLock = true
}

func (l *internalLogger) IsLevelEnabled(level Level) bool {
	return l.level <= level
}

func (l *internalLogger) WithFields(fields Fields) *internalEntry {
	entry := entryPool.Get().(*internalEntry)
	entry.logger = l
	entry.Time = time.Now()
	entry.Level = 0
	entry.Message = ""
	entry.Caller = nil
	entry.Buffer = nil

	clear(entry.Data)

	for k, v := range fields {
		entry.Data[k] = v
	}
	return entry
}

type internalEntry struct {
	logger  *internalLogger
	Data    Fields
	Time    time.Time
	Level   Level
	Message string
	Caller  *runtime.Frame
	Buffer  *bytes.Buffer
}

func (e *internalEntry) HasCaller() bool {
	return e.logger.reportCaller
}

func (e *internalEntry) Log(level Level, args ...interface{}) {
	if !e.logger.IsLevelEnabled(level) {
		return
	}

	e.Level = level
	e.Time = time.Now()

	if e.HasCaller() {
		e.Caller = e.getCaller()
	}

	switch len(args) {
	case 0:
		e.Message = ""
	case 1:
		if str, ok := args[0].(string); ok {
			e.Message = str
		} else if v, ok := args[0].(interface{}); ok {
			e.Message = fmt.Sprint(v)
		}
	default:
		if v, ok := args[0].([]interface{}); ok && len(v) > 0 {
			if str, ok := v[0].(string); ok {
				e.Message = str
			} else {
				e.Message = fmt.Sprint(v[0])
			}
		} else {
			e.Message = fmt.Sprint(args...)
		}
	}

	if err := e.formatAndWrite(); err != nil {
		fmt.Fprintf(e.logger.out, "Failed to format log entry: %v\n", err)
	}

	entryPool.Put(e)
}

func (e *internalEntry) formatAndWrite() error {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	data := make(Fields, len(e.Data)+6)

	for k, v := range e.Data {
		switch v := v.(type) {
		case error:
			data[k] = v.Error()
		default:
			data[k] = v
		}
	}

	e.prefixFieldClashes(data)

	data["time"] = e.Time.Format(time.RFC3339)

	msg := strings.TrimPrefix(e.Message, "[[")
	msg = strings.TrimSuffix(msg, "]]")

	if len(msg) > 0 && (msg[0] == '{' || msg[0] == '[') {
		var mapMessage map[string]interface{}
		if err := json.Unmarshal([]byte(msg), &mapMessage); err == nil {
			data["msg"] = mapMessage
		} else {
			data["msg"] = msg
		}
	} else {
		data["msg"] = msg
	}

	data["level"] = e.Level.Lower()

	if e.HasCaller() && e.Caller != nil {
		funcVal := e.Caller.Function
		fileVal := fmt.Sprintf("%s:%d", e.Caller.File, e.Caller.Line)

		if funcVal != "" {
			data["func"] = funcVal
		}
		if fileVal != "" {
			data["file"] = fileVal
		}
	}

	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to marshal fields to JSON: %v", err)
	}

	if !e.logger.noLock {
		e.logger.mu.Lock()
		defer e.logger.mu.Unlock()
	}

	if e.logger.out != nil {
		n, _ := e.logger.out.Write(buf.Bytes())

		if e.logger.onWrite != nil {
			e.logger.onWrite(n)
		}
	}

	return nil
}

func (e *internalEntry) prefixFieldClashes(data Fields) {
	if t, ok := data["time"]; ok {
		data["fields.time"] = t
		delete(data, "time")
	}

	if m, ok := data["msg"]; ok {
		data["fields.msg"] = m
		delete(data, "msg")
	}

	if l, ok := data["level"]; ok {
		data["fields.level"] = l
		delete(data, "level")
	}

	if e.HasCaller() {
		if f, ok := data["func"]; ok {
			data["fields.func"] = f
			delete(data, "func")
		}
		if f, ok := data["file"]; ok {
			data["fields.file"] = f
			delete(data, "file")
		}
	}
}

var (
	kkloggerPackage    string
	minimumCallerDepth = 1
	maximumCallerDepth = 29
	callerInitOnce     sync.Once
)

// callerPCPool reduces allocations for caller stack frames
var callerPCPool = sync.Pool{
	New: func() interface{} {
		pcs := make([]uintptr, maximumCallerDepth)
		return &pcs
	},
}

func (e *internalEntry) getCaller() *runtime.Frame {
	callerInitOnce.Do(func() {
		pcs := make([]uintptr, 8)
		_ = runtime.Callers(0, pcs)
		kkloggerPackage = getPackageName(runtime.FuncForPC(pcs[1]).Name())
	})

	pcsPtr := callerPCPool.Get().(*[]uintptr)
	pcs := *pcsPtr
	defer callerPCPool.Put(pcsPtr)

	depth := runtime.Callers(minimumCallerDepth, pcs)
	frames := runtime.CallersFrames(pcs[:depth])

	for f, again := frames.Next(); again; f, again = frames.Next() {
		pkg := getPackageName(f.Function)

		if pkg != kkloggerPackage {
			return &f
		}
	}

	return nil
}

func getPackageName(f string) string {
	for {
		lastPeriod := strings.LastIndex(f, ".")
		lastSlash := strings.LastIndex(f, "/")
		if lastPeriod > lastSlash {
			f = f[:lastPeriod]
		} else {
			break
		}
	}
	return f
}
