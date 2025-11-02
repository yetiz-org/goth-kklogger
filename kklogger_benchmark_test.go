package kklogger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func BenchmarkSyncWrite(b *testing.B) {
	tmpDir := b.TempDir()

	cfg := &Config{
		Environment:  "bench",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Info("benchmark sync write message")
	}
}

func BenchmarkAsyncWrite(b *testing.B) {
	tmpDir := b.TempDir()

	cfg := &Config{
		Environment:  "bench",
		LoggerPath:   tmpDir,
		AsyncWrite:   true,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Info("benchmark async write message")
	}
}

func BenchmarkFormattedLog(b *testing.B) {
	tmpDir := b.TempDir()

	cfg := &Config{
		Environment:  "bench",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Infof("benchmark formatted log %d", i)
	}
}

func BenchmarkStructuredLog(b *testing.B) {
	tmpDir := b.TempDir()

	cfg := &Config{
		Environment:  "bench",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	data := map[string]interface{}{
		"user_id": 12345,
		"action":  "test",
		"status":  "success",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.InfoJ("benchmark", data)
	}
}

func BenchmarkWithCaller(b *testing.B) {
	tmpDir := b.TempDir()

	cfg := &Config{
		Environment:  "bench",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: true,

		Level: TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Info("benchmark with caller")
	}
}

func BenchmarkWithoutCaller(b *testing.B) {
	tmpDir := b.TempDir()

	cfg := &Config{
		Environment:  "bench",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,

		Level: TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Info("benchmark without caller")
	}
}

func BenchmarkLevelFiltering(b *testing.B) {
	tmpDir := b.TempDir()

	cfg := &Config{
		Environment:  "bench",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        ErrorLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Debug("filtered debug message")

	}
}

func BenchmarkConcurrentWrites(b *testing.B) {
	tmpDir := b.TempDir()

	cfg := &Config{
		Environment:  "bench",
		LoggerPath:   tmpDir,
		AsyncWrite:   true,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("concurrent benchmark message")
		}
	})
}

func BenchmarkJsonMsgMarshal(b *testing.B) {
	msg := NewJsonMsg("test_type", map[string]interface{}{
		"key1": "value1",
		"key2": 12345,
		"key3": true,
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = msg.Marshal()
	}
}

func BenchmarkSimpleJsonMsg(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msg := SimpleJsonMsg("test data")
		_ = msg.Marshal()
	}
}

func BenchmarkMultipleInstances(b *testing.B) {
	instances := 10
	var loggers []*KKLogger

	for i := 0; i < instances; i++ {
		tmpDir := b.TempDir()
		cfg := &Config{
			Environment:  fmt.Sprintf("bench-%d", i),
			LoggerPath:   tmpDir,
			AsyncWrite:   true,
			ReportCaller: false,
			Level:        TraceLevel,
		}
		logger := NewWithConfig(cfg)
		loggers = append(loggers, logger)
	}

	defer func() {
		for _, logger := range loggers {
			logger.Close()
		}
	}()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		loggers[i%instances].Info(fmt.Sprintf("multi-instance bench %d", i))
	}
}

func BenchmarkWithHooks(b *testing.B) {
	tmpDir := b.TempDir()

	cfg := &Config{
		Environment:  "bench",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
		Hooks:        []LoggerHook{&DefaultLoggerHook{}},
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Info("benchmark with hooks")
	}
}

func BenchmarkLongMessage(b *testing.B) {
	tmpDir := b.TempDir()

	cfg := &Config{
		Environment:  "bench",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	longMsg := string(make([]byte, 1024))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Info(longMsg)
	}
}

func BenchmarkArchiveCheck(b *testing.B) {
	tmpDir := b.TempDir()

	cfg := &Config{
		Environment:  "bench",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
		Archive: &ArchiveConfig{
			MaxSizeBytes: 10485760,

			RotationInterval: 0,
			ArchiveDir:       "archived",
			FilenamePattern:  time.RFC3339,
			Compression:      "gzip",
		},
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Info("benchmark with archive check")
	}
}

func TestMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   true,
		ReportCaller: false,
		Level:        TraceLevel,
	}

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	logger := NewWithConfig(cfg)

	for i := 0; i < 10000; i++ {
		logger.Info(fmt.Sprintf("memory test %d", i))
	}

	logger.Close()

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	allocDiff := m2.TotalAlloc - m1.TotalAlloc

	t.Logf("Memory allocated: %d bytes (%.2f MB)", allocDiff, float64(allocDiff)/(1024*1024))
	t.Logf("Heap objects: %d", m2.HeapObjects)

	if allocDiff > 50*1024*1024 {
		t.Errorf("Memory usage too high: %.2f MB", float64(allocDiff)/(1024*1024))
	}
}

func TestMemoryLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	for i := 0; i < 100; i++ {
		tmpDir := t.TempDir()
		cfg := &Config{
			Environment:  "test",
			LoggerPath:   tmpDir,
			AsyncWrite:   true,
			ReportCaller: false,
			Level:        TraceLevel,
		}

		logger := NewWithConfig(cfg)
		logger.Info("leak test")
		logger.Close()
	}

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()

	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	heapDiff := int64(m2.HeapAlloc) - int64(m1.HeapAlloc)

	t.Logf("Heap difference: %d bytes (%.2f MB)", heapDiff, float64(heapDiff)/(1024*1024))

	if heapDiff > 10*1024*1024 {
		t.Errorf("Possible memory leak: %.2f MB", float64(heapDiff)/(1024*1024))
	}
}

func TestPoolEfficiency(t *testing.T) {
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

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	for i := 0; i < 10000; i++ {
		logger.Info(fmt.Sprintf("pool test %d", i))
	}

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	allocDiff := m2.TotalAlloc - m1.TotalAlloc
	mallocsDiff := m2.Mallocs - m1.Mallocs

	t.Logf("Total allocations: %d bytes", allocDiff)
	t.Logf("Number of mallocs: %d", mallocsDiff)
	t.Logf("Average bytes per malloc: %.2f", float64(allocDiff)/float64(mallocsDiff))

	if mallocsDiff > 50000 {
		t.Logf("Warning: High malloc count, pool may not be effective")
	}
}

func TestThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping throughput test in short mode")
	}

	tmpDir := t.TempDir()

	tests := []struct {
		name       string
		asyncWrite bool
	}{
		{"Sync", false},
		{"Async", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Environment:  "test",
				LoggerPath:   tmpDir,
				AsyncWrite:   tt.asyncWrite,
				ReportCaller: false,
				Level:        TraceLevel,
			}

			logger := NewWithConfig(cfg)

			count := 100000
			start := time.Now()

			for i := 0; i < count; i++ {
				logger.Info(fmt.Sprintf("throughput test %d", i))
			}

			logger.Close()
			duration := time.Since(start)

			throughput := float64(count) / duration.Seconds()

			t.Logf("%s mode: %d logs in %v (%.0f logs/sec)",
				tt.name, count, duration, throughput)

			if throughput < 10000 {
				t.Logf("Warning: Low throughput: %.0f logs/sec", throughput)
			}
		})
	}
}

func TestConcurrentThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent throughput test in short mode")
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
	defer logger.Close()

	goroutines := 10
	logsPerGoroutine := 10000
	totalLogs := goroutines * logsPerGoroutine

	start := time.Now()

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < logsPerGoroutine; i++ {
				logger.Info(fmt.Sprintf("concurrent throughput g%d-i%d", id, i))
			}
		}(g)
	}

	wg.Wait()
	logger.Close()

	duration := time.Since(start)
	throughput := float64(totalLogs) / duration.Seconds()

	t.Logf("Concurrent: %d logs from %d goroutines in %v (%.0f logs/sec)",
		totalLogs, goroutines, duration, throughput)

	if throughput < 20000 {
		t.Logf("Warning: Low concurrent throughput: %.0f logs/sec", throughput)
	}
}

func TestLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping latency test in short mode")
	}

	tmpDir := t.TempDir()

	tests := []struct {
		name       string
		asyncWrite bool
	}{
		{"Sync", false},
		{"Async", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Environment:  "test",
				LoggerPath:   tmpDir,
				AsyncWrite:   tt.asyncWrite,
				ReportCaller: false,
				Level:        TraceLevel,
			}

			logger := NewWithConfig(cfg)
			defer logger.Close()

			iterations := 1000
			var totalDuration time.Duration

			for i := 0; i < iterations; i++ {
				start := time.Now()
				logger.Info("latency test")
				duration := time.Since(start)
				totalDuration += duration
			}

			avgLatency := totalDuration / time.Duration(iterations)

			t.Logf("%s mode average latency: %v", tt.name, avgLatency)

			if !tt.asyncWrite && avgLatency > 1*time.Millisecond {
				t.Logf("Warning: High sync latency: %v", avgLatency)
			}

			if tt.asyncWrite && avgLatency > 100*time.Microsecond {
				t.Logf("Warning: High async latency: %v", avgLatency)
			}
		})
	}
}

func TestScalability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scalability test in short mode")
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
	defer logger.Close()

	concurrencyLevels := []int{1, 2, 4, 8, 16}
	logsPerGoroutine := 10000

	for _, concurrency := range concurrencyLevels {
		start := time.Now()

		var wg sync.WaitGroup
		for g := 0; g < concurrency; g++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for i := 0; i < logsPerGoroutine; i++ {
					logger.Info(fmt.Sprintf("scalability test g%d-i%d", id, i))
				}
			}(g)
		}

		wg.Wait()
		duration := time.Since(start)

		totalLogs := concurrency * logsPerGoroutine
		throughput := float64(totalLogs) / duration.Seconds()

		t.Logf("Concurrency %d: %.0f logs/sec", concurrency, throughput)
	}
}

func TestFileIOPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping file I/O test in short mode")
	}

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

	count := 50000
	start := time.Now()

	for i := 0; i < count; i++ {
		logger.Info(fmt.Sprintf("file io test %d", i))
	}

	duration := time.Since(start)

	logFile := filepath.Join(tmpDir, "current.log")
	stat, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("Failed to stat log file: %v", err)
	}

	fileSize := stat.Size()
	writeThroughput := float64(fileSize) / duration.Seconds() / (1024 * 1024) // MB/s

	t.Logf("File size: %.2f MB", float64(fileSize)/(1024*1024))
	t.Logf("Write throughput: %.2f MB/s", writeThroughput)
	t.Logf("Duration: %v", duration)
}
