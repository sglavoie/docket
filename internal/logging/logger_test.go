package logging

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// This file covers the parts of the package that stand alone: the one exported
// write, the retention sweep and what it must leave behind, and the file naming
// that keeps two log files from colliding. The lifecycle is beside it, in
// logger_init_test.go and logger_close_test.go.

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

func TestPruneOldLogsKeepsActiveLogFile(t *testing.T) {
	origDir := logDirPath
	origFile := logFile
	origPath := logFilePath
	defer func() {
		logDirPath = origDir
		logFile = origFile
		logFilePath = origPath
	}()

	tmp := t.TempDir()
	logDirPath = tmp

	activeLog := filepath.Join(tmp, "tt-active.log")
	f, err := os.OpenFile(activeLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("failed to create active log: %v", err)
	}
	defer func() { _ = f.Close() }()

	oldTime := time.Now().AddDate(0, 0, -60)
	if err := os.Chtimes(activeLog, oldTime, oldTime); err != nil {
		t.Fatalf("failed to set active log mtime: %v", err)
	}

	staleLog := filepath.Join(tmp, "tt-stale.log")
	if err := os.WriteFile(staleLog, []byte("stale"), 0o644); err != nil {
		t.Fatalf("failed to create stale log: %v", err)
	}
	if err := os.Chtimes(staleLog, oldTime, oldTime); err != nil {
		t.Fatalf("failed to set stale log mtime: %v", err)
	}

	logFile = f
	logFilePath = activeLog

	pruneOldLogs()

	if _, err := os.Stat(activeLog); err != nil {
		t.Fatalf("expected active log file to remain: %v", err)
	}
	if _, err := os.Stat(staleLog); !os.IsNotExist(err) {
		t.Fatal("expected inactive stale log file to be removed")
	}
}

func TestPruneOldLogsInvalidDir(t *testing.T) {
	original := logDirPath
	defer func() { logDirPath = original }()

	logDirPath = "/nonexistent/path/that/does/not/exist"
	pruneOldLogs() // should not panic
}

func TestLogFileNameUsesNanosecondPrecision(t *testing.T) {
	base := time.Date(2025, time.January, 2, 3, 4, 5, 123456780, time.UTC)
	next := base.Add(time.Nanosecond)

	first := logFileName(base)
	second := logFileName(next)

	if first == second {
		t.Fatalf("expected distinct filenames for sub-second timestamps, got %q", first)
	}
}
