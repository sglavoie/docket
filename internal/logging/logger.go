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

// logRetention is how long a log file survives before pruneOldLogs removes it.
const logRetention = 30 * 24 * time.Hour

// Log files can quote task titles and workspace paths, so the directory and
// every file in it are private to the user.
const (
	logDirMode  os.FileMode = 0o700
	logFileMode os.FileMode = 0o600
)

// openFile is the os.OpenFile seam the open-failure test replaces: Init
// re-tightens the directory mode, so a read-only directory no longer fails it.
var openFile = os.OpenFile

// Dir returns the directory Init writes log files into, ~/.tt/logs. It creates
// nothing.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".tt", "logs"), nil
}

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

	dir, err := Dir()
	if err != nil {
		return
	}

	if err = os.MkdirAll(dir, logDirMode); err != nil {
		return
	}
	// MkdirAll leaves an existing directory's mode alone, so a directory an
	// older release created world-readable is tightened here.
	if err = os.Chmod(dir, logDirMode); err != nil {
		return
	}
	logDirPath = dir
	name := logFileName(time.Now())
	logFilePath = filepath.Join(logDirPath, name)
	f, err := openFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, logFileMode)
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

// pruneOldLogs removes .log files older than logRetention from the log
// directory. Caller must hold mu.
func pruneOldLogs() {
	cutoff := time.Now().Add(-logRetention)
	entries, err := os.ReadDir(logDirPath)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".log" {
			continue
		}
		path := filepath.Join(logDirPath, entry.Name())
		if logFile != nil && path == logFilePath {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
		}
	}
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
