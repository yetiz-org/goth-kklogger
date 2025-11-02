package kklogger

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestArchiveSizeBased(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
		Archive: &ArchiveConfig{
			MaxSizeBytes:     1024, // 1KB
			RotationInterval: 0,
			ArchiveDir:       "archived",
			FilenamePattern:  time.RFC3339,
			Compression:      "gzip",
		},
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	for i := 0; i < 100; i++ {
		logger.Info(fmt.Sprintf("size test message %d with some padding to increase size", i))
	}

	time.Sleep(500 * time.Millisecond)

	archiveDir := filepath.Join(tmpDir, "archived")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		t.Fatalf("Failed to read archive directory: %v", err)
	}

	if len(entries) == 0 {
		t.Error("Should have at least one archived file")
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".gz") {
			t.Errorf("Archive file should have .gz extension: %s", entry.Name())
		}

		archivePath := filepath.Join(archiveDir, entry.Name())
		if err := verifyGzipFile(archivePath); err != nil {
			t.Errorf("Failed to verify gzip file %s: %v", entry.Name(), err)
		}
	}

	currentLog := filepath.Join(tmpDir, "current.log")
	stat, err := os.Stat(currentLog)
	if err != nil {
		t.Fatalf("current.log should exist: %v", err)
	}

	t.Logf("Current log size: %d bytes", stat.Size())
	t.Logf("Archived files: %d", len(entries))
}

func TestArchiveTimeBased(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping time-based test in short mode")
	}

	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
		Archive: &ArchiveConfig{
			MaxSizeBytes:     0,
			RotationInterval: 2 * time.Second,

			ArchiveDir:      "archived",
			FilenamePattern: time.RFC3339,
			Compression:     "gzip",
		},
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	logger.Info("before rotation")

	time.Sleep(3 * time.Second)

	logger.Info("after rotation")

	time.Sleep(500 * time.Millisecond)

	archiveDir := filepath.Join(tmpDir, "archived")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		t.Fatalf("Failed to read archive directory: %v", err)
	}

	if len(entries) == 0 {
		t.Error("Should have at least one archived file after time-based rotation")
	}

	t.Logf("Archived files after time rotation: %d", len(entries))
}

func TestArchiveNoCompression(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
		Archive: &ArchiveConfig{
			MaxSizeBytes:     512, // 512 bytes
			RotationInterval: 0,
			ArchiveDir:       "archived",
			FilenamePattern:  time.RFC3339,
			Compression:      "none",
		},
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	for i := 0; i < 50; i++ {
		logger.Info(fmt.Sprintf("no compression test %d", i))
	}

	time.Sleep(500 * time.Millisecond)

	archiveDir := filepath.Join(tmpDir, "archived")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		t.Fatalf("Failed to read archive directory: %v", err)
	}

	if len(entries) == 0 {
		t.Error("Should have archived files")
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".log") {
			t.Errorf("Uncompressed archive should have .log extension: %s", entry.Name())
		}

		archivePath := filepath.Join(archiveDir, entry.Name())
		content, err := os.ReadFile(archivePath)
		if err != nil {
			t.Errorf("Failed to read uncompressed archive: %v", err)
		}

		if len(content) == 0 {
			t.Error("Archive file should not be empty")
		}
	}
}

func TestCalculateNextRotationTime(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		interval time.Duration
		wantHour int
		wantMin  int
	}{
		{
			name:     "daily rotation at 13:29",
			now:      time.Date(2024, 1, 15, 13, 29, 0, 0, time.UTC),
			interval: 24 * time.Hour,
			wantHour: 0,
			wantMin:  0,
		},
		{
			name:     "hourly rotation at 13:29",
			now:      time.Date(2024, 1, 15, 13, 29, 0, 0, time.UTC),
			interval: 1 * time.Hour,
			wantHour: 14,
			wantMin:  0,
		},
		{
			name:     "30min rotation at 13:29",
			now:      time.Date(2024, 1, 15, 13, 29, 0, 0, time.UTC),
			interval: 30 * time.Minute,
			wantHour: 13,
			wantMin:  30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := calculateNextRotationTime(tt.now, tt.interval)

			if next.Hour() != tt.wantHour {
				t.Errorf("Hour = %d, want %d", next.Hour(), tt.wantHour)
			}

			if next.Minute() != tt.wantMin {
				t.Errorf("Minute = %d, want %d", next.Minute(), tt.wantMin)
			}

			if next.Second() != 0 {
				t.Errorf("Second should be 0, got %d", next.Second())
			}

			t.Logf("%s: now=%v, next=%v", tt.name, tt.now, next)
		})
	}
}

func TestArchiveNoDataLoss(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment: "test",
		LoggerPath:  tmpDir,
		AsyncWrite:  true,

		ReportCaller: false,
		Level:        TraceLevel,
		Archive: &ArchiveConfig{
			MaxSizeBytes: 1500,

			RotationInterval: 0,
			ArchiveDir:       "archived",
			FilenamePattern:  time.RFC3339,
			Compression:      "gzip",
		},
	}

	logger := NewWithConfig(cfg)

	totalLogs := 100
	for i := 0; i < totalLogs; i++ {
		logger.Info(fmt.Sprintf("no loss test %d", i))
	}

	logger.Close()

	allLogs := []string{}

	currentLog := filepath.Join(tmpDir, "current.log")
	if content, err := os.ReadFile(currentLog); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.Contains(line, "no loss test") {
				allLogs = append(allLogs, line)
			}
		}
	}

	archiveDir := filepath.Join(tmpDir, "archived")
	if entries, err := os.ReadDir(archiveDir); err == nil {
		for _, entry := range entries {
			archivePath := filepath.Join(archiveDir, entry.Name())
			content, err := readGzipFile(archivePath)
			if err != nil {
				t.Errorf("Failed to read archive %s: %v", entry.Name(), err)
				continue
			}

			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				if strings.Contains(line, "no loss test") {
					allLogs = append(allLogs, line)
				}
			}
		}
	}

	if len(allLogs) != totalLogs {
		t.Errorf("Data loss detected! Expected %d logs, found %d (loss: %d)",
			totalLogs, len(allLogs), totalLogs-len(allLogs))

		foundMap := make(map[int]bool)
		for _, line := range allLogs {
			for i := 0; i < totalLogs; i++ {
				if strings.Contains(line, fmt.Sprintf("no loss test %d", i)) {
					foundMap[i] = true
					break
				}
			}
		}

		missing := []int{}
		for i := 0; i < totalLogs; i++ {
			if !foundMap[i] {
				missing = append(missing, i)
			}
		}

		if len(missing) > 0 && len(missing) <= 10 {
			t.Logf("Missing log entries: %v", missing)
		} else if len(missing) > 10 {
			t.Logf("Missing %d log entries (first 10): %v", len(missing), missing[:10])
		}
	} else {
		t.Logf("✅ Perfect! All %d logs preserved (no data loss)", totalLogs)
	}
}

func TestArchiveCustomPattern(t *testing.T) {
	tmpDir := t.TempDir()

	customPattern := "2006-01-02_15-04-05"

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
		Archive: &ArchiveConfig{
			MaxSizeBytes:     512,
			RotationInterval: 0,
			ArchiveDir:       "archived",
			FilenamePattern:  customPattern,
			Compression:      "gzip",
		},
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	for i := 0; i < 50; i++ {
		logger.Info(fmt.Sprintf("custom pattern test %d", i))
	}

	time.Sleep(500 * time.Millisecond)

	archiveDir := filepath.Join(tmpDir, "archived")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		t.Fatalf("Failed to read archive directory: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("Should have archived files")
	}

	for _, entry := range entries {
		name := entry.Name()
		nameWithoutExt := strings.TrimSuffix(name, ".gz")

		if _, err := time.Parse(customPattern, nameWithoutExt); err != nil {
			t.Errorf("Archive filename %s doesn't match pattern %s: %v",
				name, customPattern, err)
		}
	}
}

func TestArchiveCustomDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	customArchiveDir := "my_archives"

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
		Archive: &ArchiveConfig{
			MaxSizeBytes:     512,
			RotationInterval: 0,
			ArchiveDir:       customArchiveDir,
			FilenamePattern:  time.RFC3339,
			Compression:      "gzip",
		},
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	for i := 0; i < 50; i++ {
		logger.Info(fmt.Sprintf("custom dir test %d", i))
	}

	time.Sleep(500 * time.Millisecond)

	archiveDir := filepath.Join(tmpDir, customArchiveDir)
	if _, err := os.Stat(archiveDir); os.IsNotExist(err) {
		t.Errorf("Custom archive directory %s should exist", customArchiveDir)
	}

	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		t.Fatalf("Failed to read custom archive directory: %v", err)
	}

	if len(entries) == 0 {
		t.Error("Should have files in custom archive directory")
	}
}

func TestArchiveDisabled(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
		Archive:      nil,
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	for i := 0; i < 1000; i++ {
		logger.Info(fmt.Sprintf("no archive test %d", i))
	}

	time.Sleep(500 * time.Millisecond)

	archiveDir := filepath.Join(tmpDir, "archived")
	if _, err := os.Stat(archiveDir); !os.IsNotExist(err) {
		entries, _ := os.ReadDir(archiveDir)
		if len(entries) > 0 {
			t.Error("Should not have archived files when archive is disabled")
		}
	}

	currentLog := filepath.Join(tmpDir, "current.log")
	content, err := os.ReadFile(currentLog)
	if err != nil {
		t.Fatalf("Failed to read current.log: %v", err)
	}

	if len(content) == 0 {
		t.Error("current.log should contain logs")
	}
}

func TestArchiveConcurrent(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   true,
		ReportCaller: false,
		Level:        TraceLevel,
		Archive: &ArchiveConfig{
			MaxSizeBytes:     1024,
			RotationInterval: 0,
			ArchiveDir:       "archived",
			FilenamePattern:  time.RFC3339,
			Compression:      "gzip",
		},
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	var wg sync.WaitGroup
	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				logger.Info(fmt.Sprintf("concurrent archive test g%d-i%d", id, i))
			}
		}(g)
	}

	wg.Wait()
	logger.Close()

	archiveDir := filepath.Join(tmpDir, "archived")
	if entries, err := os.ReadDir(archiveDir); err == nil {
		t.Logf("Archived files created: %d", len(entries))
	}
}

func TestUpdateFileSize(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Environment:  "test",
		LoggerPath:   tmpDir,
		AsyncWrite:   false,
		ReportCaller: false,
		Level:        TraceLevel,
		Archive: &ArchiveConfig{
			MaxSizeBytes:     10240, // 10KB
			RotationInterval: 0,
			ArchiveDir:       "archived",
			FilenamePattern:  time.RFC3339,
			Compression:      "gzip",
		},
	}

	logger := NewWithConfig(cfg)
	defer logger.Close()

	for i := 0; i < 10; i++ {
		logger.Info(fmt.Sprintf("size tracking test %d", i))
	}

	trackedSize := atomic.LoadInt64(&logger.fileSizeBytes)

	currentLog := filepath.Join(tmpDir, "current.log")
	stat, err := os.Stat(currentLog)
	if err != nil {
		t.Fatalf("Failed to stat current.log: %v", err)
	}

	actualSize := stat.Size()

	if trackedSize == 0 {
		t.Error("Tracked size should not be zero")
	}

	t.Logf("Tracked size: %d, Actual size: %d", trackedSize, actualSize)

	diff := trackedSize - actualSize
	if diff < 0 {
		diff = -diff
	}

	if diff > 100 {
		t.Errorf("Size tracking mismatch: tracked=%d, actual=%d, diff=%d",
			trackedSize, actualSize, diff)
	}
}

// Helper functions

func verifyGzipFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	buf := make([]byte, 1024)
	_, err = gz.Read(buf)
	if err != nil && err != io.EOF {
		return err
	}

	return nil
}

func readGzipFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	return io.ReadAll(gz)
}
