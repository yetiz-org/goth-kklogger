package kklogger

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// isArchiveEnabled checks if archive functionality is enabled
// Archive is enabled if MaxSizeBytes > 0 or RotationInterval > 0
func (cfg *ArchiveConfig) isArchiveEnabled() bool {
	if cfg == nil {
		return false
	}
	return cfg.MaxSizeBytes > 0 || cfg.RotationInterval > 0
}

// calculateNextRotationTime calculates the next aligned rotation time based on the interval
func calculateNextRotationTime(now time.Time, interval time.Duration) time.Time {
	if interval <= 0 {
		return now
	}

	if interval == 24*time.Hour {
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		return next
	}

	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	elapsed := now.Sub(midnight)

	intervalsSinceMidnight := elapsed / interval
	nextIntervalStart := midnight.Add((intervalsSinceMidnight + 1) * interval)

	return nextIntervalStart
}

// startArchiveMonitor starts monitoring for archive triggers
// This should be called after the log file is opened
func (kk *KKLogger) startArchiveMonitor() {
	if !kk.archiveConfig.isArchiveEnabled() {
		return
	}

	if kk.archiveConfig.RotationInterval > 0 {
		go kk.archiveTimeMonitor()
	}
}

// stopArchiveMonitor stops the archive monitoring
func (kk *KKLogger) stopArchiveMonitor() {
	if kk.archiveTicker != nil {
		kk.archiveTicker.Stop()
	}

	select {
	case <-kk.archiveStop:
	default:
		close(kk.archiveStop)
	}
}

// archiveTimeMonitor monitors time-based archiving with aligned rotation times
func (kk *KKLogger) archiveTimeMonitor() {
	interval := kk.archiveConfig.RotationInterval

	for {
		now := time.Now()
		nextRotation := calculateNextRotationTime(now, interval)
		waitDuration := nextRotation.Sub(now)

		timer := time.NewTimer(waitDuration)

		select {
		case <-kk.archiveStop:
			timer.Stop()
			return
		case <-timer.C:
			if err := kk.performArchive(); err != nil {
				fmt.Fprintf(os.Stderr, "kklogger: archive failed: %v\n", err)
			}
		}
	}
}

// checkSizeBasedArchive checks if size-based archiving should be triggered
// This is called after each write (from write callback)
// Uses non-blocking trigger to avoid deadlock
func (kk *KKLogger) checkSizeBasedArchive() {
	if kk.archiveConfig == nil || kk.archiveConfig.MaxSizeBytes <= 0 {
		return
	}

	currentSize := atomic.LoadInt64(&kk.fileSizeBytes)
	if currentSize >= kk.archiveConfig.MaxSizeBytes {
		go func() {
			if err := kk.performArchive(); err != nil {
				fmt.Fprintf(os.Stderr, "kklogger: archive failed: %v\n", err)
			}
		}()
	}
}

// performArchive performs the actual archiving process
// This method ensures no logs are lost during archiving
func (kk *KKLogger) performArchive() error {
	if !atomic.CompareAndSwapInt32(&kk.archiving, 0, 1) {
		return nil
	}
	defer atomic.StoreInt32(&kk.archiving, 0)

	if kk.asyncWrite {
		maxWait := 2 * time.Minute
		timeout := time.After(maxWait)
		ticker := time.NewTicker(50 * time.Millisecond)
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
				if noProgressCount%(10*20) == 0 {
					fmt.Fprintf(os.Stderr, "kklogger: archive waiting for %d pending logs (waited %v)\n",
						pending, time.Since(startWait))
				}
			} else {
				noProgressCount = 0
				lastPending = pending
			}

			select {
			case <-timeout:
				return fmt.Errorf("CRITICAL: archive timeout after %v waiting for %d pending logs",
					maxWait, pending)
			case <-ticker.C:
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	kk.mu.Lock()
	defer kk.mu.Unlock()

	if kk.file != nil {
		if err := kk.file.Close(); err != nil {
			if !strings.Contains(err.Error(), "already closed") {
				return fmt.Errorf("failed to close current log file: %w", err)
			}
		}
		kk.file = nil
	}

	if err := kk.archiveCurrentFile(); err != nil {
		_ = kk.reopenLogFile()
		return err
	}

	if err := kk.reopenLogFile(); err != nil {
		return fmt.Errorf("failed to reopen log file after archive: %w", err)
	}

	return nil
}

// archiveCurrentFile archives the current log file
func (kk *KKLogger) archiveCurrentFile() error {
	currentLogPath := filepath.Join(kk.loggerPath, "current.log")

	if _, err := os.Stat(currentLogPath); os.IsNotExist(err) {
		return nil
	}

	archiveDir := filepath.Join(kk.loggerPath, kk.archiveConfig.ArchiveDir)
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	timestamp := time.Now()
	archiveFilename := timestamp.Format(kk.archiveConfig.FilenamePattern)

	if kk.archiveConfig.Compression == "gzip" {
		archiveFilename += ".gz"
	} else {
		archiveFilename += ".log"
	}

	archivePath := filepath.Join(archiveDir, archiveFilename)

	if kk.archiveConfig.Compression == "gzip" {
		if err := compressFile(currentLogPath, archivePath); err != nil {
			return fmt.Errorf("failed to compress log file: %w", err)
		}
	} else {
		if err := os.Rename(currentLogPath, archivePath); err != nil {
			return fmt.Errorf("failed to move log file: %w", err)
		}
	}

	if kk.archiveConfig.Compression == "gzip" {
		if err := os.Remove(currentLogPath); err != nil {
			fmt.Fprintf(os.Stderr, "kklogger: warning: failed to remove original log file: %v\n", err)
		}
	}

	return nil
}

// reopenLogFile reopens the log file after archiving
func (kk *KKLogger) reopenLogFile() error {
	logFile, err := os.OpenFile(
		filepath.Join(kk.loggerPath, "current.log"),
		os.O_CREATE|os.O_APPEND|os.O_RDWR,
		0755,
	)
	if err != nil {
		return fmt.Errorf("failed to open new log file: %w", err)
	}

	kk.file = logFile
	kk.logger.SetOutput(logFile)
	kk.fileCreatedAt = time.Now()
	atomic.StoreInt64(&kk.fileSizeBytes, 0)

	return nil
}

// compressFile compresses a file using gzip
func compressFile(srcPath, dstPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	gzWriter := gzip.NewWriter(dstFile)
	defer gzWriter.Close()

	if _, err := io.Copy(gzWriter, srcFile); err != nil {
		return fmt.Errorf("failed to compress data: %w", err)
	}

	return nil
}

// updateFileSize updates the file size counter after a write
// This should be called after each log write
func (kk *KKLogger) updateFileSize(bytesWritten int) {
	if !kk.archiveConfig.isArchiveEnabled() || kk.archiveConfig.MaxSizeBytes <= 0 {
		return
	}

	newSize := atomic.AddInt64(&kk.fileSizeBytes, int64(bytesWritten))

	var checkInterval int64 = 10240
	if kk.archiveConfig.MaxSizeBytes < 10240 {
		checkInterval = 1024
	}

	if newSize%checkInterval < int64(bytesWritten) {
		kk.checkSizeBasedArchive()
	}
}
