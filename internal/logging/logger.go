package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu                   sync.Mutex
	logDirPath           string
	logFile              *os.File
	logFilePath          string
	previousLoggerOutput io.Writer
	previousLoggerFlags  int
	loggerStateCaptured  bool
)

// Init creates the log directory and opens a timestamped log file.
// All subsequent log.Printf calls (and Warn) write to that file.
func Init() {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		closeLocked()
	}
	logDirPath = ""
	logFilePath = ""

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".tt", "logs")

	if err = os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	logDirPath = dir
	name := logFileName(time.Now())
	logFilePath = filepath.Join(logDirPath, name)
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		logFilePath = ""
		return
	}

	if !loggerStateCaptured {
		previousLoggerOutput = log.Writer()
		previousLoggerFlags = log.Flags()
		loggerStateCaptured = true
	}
	logFile = f
	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime)
	pruneOldLogs()
}

// pruneOldLogs removes .log files older than 30 days from the log directory.
// Caller must hold mu.
func pruneOldLogs() {
	cutoff := time.Now().AddDate(0, 0, -30)
	entries, err := os.ReadDir(logDirPath)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".log" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(logDirPath, entry.Name()))
		}
	}
}

// LogDir returns the log directory path.
func LogDir() string {
	mu.Lock()
	defer mu.Unlock()
	return logDirPath
}

func logFileName(now time.Time) string {
	return fmt.Sprintf("tt-%s.log", now.Format("2006-01-02-150405.000000000"))
}

// Warn writes a formatted warning to the log file.
func Warn(format string, args ...any) {
	log.Printf("WARN: "+format, args...)
}

// Close closes the log file and removes it if empty.
// It also detaches the standard logger from the file so subsequent
// log.Printf or Warn calls do not write to a closed descriptor.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	closeLocked()
}

// closeLocked performs Close logic; caller must hold mu.
func closeLocked() {
	if logFile == nil {
		restoreStdLogger()
		return
	}
	path := logFilePath
	restoreStdLogger()
	info, statErr := logFile.Stat()
	_ = logFile.Close()
	logFile = nil
	logFilePath = ""
	if statErr == nil && info.Size() == 0 && path != "" {
		_ = os.Remove(path)
	}
}

// restoreStdLogger restores the standard logger output and flags.
// Caller must hold mu.
func restoreStdLogger() {
	if !loggerStateCaptured {
		return
	}
	out := previousLoggerOutput
	if out == nil {
		out = os.Stderr
	}
	log.SetOutput(out)
	log.SetFlags(previousLoggerFlags)
	previousLoggerOutput = nil
	previousLoggerFlags = 0
	loggerStateCaptured = false
}
