package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	logDirPath  string
	logFile     *os.File
	logFilePath string
)

// Init creates the log directory and opens a timestamped log file.
// All subsequent log.Printf calls (and Warn) write to that file.
func Init() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	logDirPath = filepath.Join(home, ".tt", "logs")

	if err = os.MkdirAll(logDirPath, 0o755); err != nil {
		return
	}
	name := fmt.Sprintf("tt-%s.log", time.Now().Format("2006-01-02-150405"))
	logFilePath = filepath.Join(logDirPath, name)
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	logFile = f
	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime)
	pruneOldLogs()
}

// pruneOldLogs removes .log files older than 30 days from the log directory.
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
func LogDir() string { return logDirPath }

// Warn writes a formatted warning to the log file.
func Warn(format string, args ...any) {
	log.Printf("WARN: "+format, args...)
}

// Close closes the log file and removes it if empty.
func Close() {
	if logFile == nil {
		return
	}
	path := logFilePath
	info, statErr := logFile.Stat()
	_ = logFile.Close()
	logFile = nil
	if statErr == nil && info.Size() == 0 && path != "" {
		_ = os.Remove(path)
	}
}
