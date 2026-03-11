package logging

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLogDir(t *testing.T) {
	original := logDirPath
	defer func() { logDirPath = original }()

	logDirPath = "/some/test/path"
	if got := LogDir(); got != "/some/test/path" {
		t.Errorf("LogDir() = %q, want %q", got, "/some/test/path")
	}

	logDirPath = ""
	if got := LogDir(); got != "" {
		t.Errorf("LogDir() = %q, want empty string", got)
	}
}

func TestWarn(t *testing.T) {
	var buf bytes.Buffer
	originalOutput := log.Writer()
	originalFlags := log.Flags()
	defer func() {
		log.SetOutput(originalOutput)
		log.SetFlags(originalFlags)
	}()

	log.SetOutput(&buf)
	log.SetFlags(0)

	Warn("something %s %d", "bad", 42)

	got := buf.String()
	if expected := "WARN: something bad 42\n"; got != expected {
		t.Errorf("Warn() logged %q, want %q", got, expected)
	}
}

func TestCloseNilLogFile(t *testing.T) {
	original := logFile
	defer func() { logFile = original }()

	logFile = nil
	Close() // should not panic
}

func TestCloseOpenFile(t *testing.T) {
	original := logFile
	defer func() { logFile = original }()

	tmp := t.TempDir()
	f, err := os.CreateTemp(tmp, "test-*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	logFile = f
	Close()

	// Writing to a closed file should fail.
	_, err = f.Write([]byte("test"))
	if err == nil {
		t.Error("expected error writing to closed file, got nil")
	}
}

func TestPruneOldLogs(t *testing.T) {
	original := logDirPath
	defer func() { logDirPath = original }()

	tmp := t.TempDir()
	logDirPath = tmp

	// Create an old log file (modified 60 days ago).
	oldLog := filepath.Join(tmp, "tt-old.log")
	if err := os.WriteFile(oldLog, []byte("old"), 0o644); err != nil {
		t.Fatalf("failed to create old log: %v", err)
	}
	oldTime := time.Now().AddDate(0, 0, -60)
	if err := os.Chtimes(oldLog, oldTime, oldTime); err != nil {
		t.Fatalf("failed to set old mtime: %v", err)
	}

	// Create a recent log file (today).
	recentLog := filepath.Join(tmp, "tt-recent.log")
	if err := os.WriteFile(recentLog, []byte("recent"), 0o644); err != nil {
		t.Fatalf("failed to create recent log: %v", err)
	}

	// Create a non-log file that is old (should not be removed).
	oldTxt := filepath.Join(tmp, "notes.txt")
	if err := os.WriteFile(oldTxt, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("failed to create old txt: %v", err)
	}
	if err := os.Chtimes(oldTxt, oldTime, oldTime); err != nil {
		t.Fatalf("failed to set old txt mtime: %v", err)
	}

	// Create a subdirectory (should be skipped).
	subDir := filepath.Join(tmp, "subdir.log")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	pruneOldLogs()

	// Old log should be removed.
	if _, err := os.Stat(oldLog); !os.IsNotExist(err) {
		t.Error("expected old log file to be removed")
	}

	// Recent log should remain.
	if _, err := os.Stat(recentLog); err != nil {
		t.Error("expected recent log file to remain")
	}

	// Non-log file should remain.
	if _, err := os.Stat(oldTxt); err != nil {
		t.Error("expected non-log file to remain")
	}

	// Subdirectory should remain.
	if _, err := os.Stat(subDir); err != nil {
		t.Error("expected subdirectory to remain")
	}
}

func TestPruneOldLogsInvalidDir(t *testing.T) {
	original := logDirPath
	defer func() { logDirPath = original }()

	logDirPath = "/nonexistent/path/that/does/not/exist"
	pruneOldLogs() // should not panic
}
