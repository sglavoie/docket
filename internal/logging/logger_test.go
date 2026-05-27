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

func TestCloseNilLogFilePreservesStdLoggerWhenNeverInitialized(t *testing.T) {
	origFile := logFile
	origOutput := log.Writer()
	origFlags := log.Flags()
	origPrevOutput := previousLoggerOutput
	origPrevFlags := previousLoggerFlags
	origStateCaptured := loggerStateCaptured
	defer func() {
		logFile = origFile
		log.SetOutput(origOutput)
		log.SetFlags(origFlags)
		previousLoggerOutput = origPrevOutput
		previousLoggerFlags = origPrevFlags
		loggerStateCaptured = origStateCaptured
	}()

	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(log.Lshortfile)
	logFile = nil
	previousLoggerOutput = nil
	previousLoggerFlags = 0
	loggerStateCaptured = false

	Close()

	if log.Writer() != &buf {
		t.Fatal("Close should leave the standard logger output unchanged when Init never ran")
	}
	if log.Flags() != log.Lshortfile {
		t.Fatalf("Close should preserve log flags, got %d", log.Flags())
	}
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

func TestCloseRemovesEmptyFile(t *testing.T) {
	origFile := logFile
	origPath := logFilePath
	defer func() { logFile = origFile; logFilePath = origPath }()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "empty.log")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	logFile = f
	logFilePath = path
	Close()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected empty log file to be removed")
	}
}

func TestCloseKeepsNonEmptyFile(t *testing.T) {
	origFile := logFile
	origPath := logFilePath
	defer func() { logFile = origFile; logFilePath = origPath }()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "nonempty.log")
	if err := os.WriteFile(path, []byte("some log content"), 0o644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open temp file: %v", err)
	}

	logFile = f
	logFilePath = path
	Close()

	if _, err := os.Stat(path); err != nil {
		t.Error("expected non-empty log file to remain")
	}
}

func TestCloseDetachesStdLogger(t *testing.T) {
	origFile := logFile
	origPath := logFilePath
	origOutput := log.Writer()
	origFlags := log.Flags()
	origPrevOutput := previousLoggerOutput
	origPrevFlags := previousLoggerFlags
	origStateCaptured := loggerStateCaptured
	defer func() {
		logFile = origFile
		logFilePath = origPath
		log.SetOutput(origOutput)
		log.SetFlags(origFlags)
		previousLoggerOutput = origPrevOutput
		previousLoggerFlags = origPrevFlags
		loggerStateCaptured = origStateCaptured
	}()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "writes.log")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	logFile = f
	logFilePath = path
	log.SetOutput(f)

	// Write a real entry so the file is non-empty (so Close keeps it on disk).
	log.Printf("warmup")

	Close()

	// After Close, the standard logger must not write to the closed file.
	// If the bug is present (output not detached), this call would attempt
	// to write to the closed FD and could fail or panic, but more importantly
	// the closed FD's bytes would never reach the file. Verify the file size
	// is unchanged after additional logging.
	infoBefore, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after close: %v", err)
	}

	// Should not panic and should not write to closed file.
	log.Printf("after-close")
	Warn("post close warning")

	infoAfter, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after extra logs: %v", err)
	}
	if infoAfter.Size() != infoBefore.Size() {
		t.Errorf("log file grew after Close (before=%d, after=%d)",
			infoBefore.Size(), infoAfter.Size())
	}
}

func TestInitCloseRestoresStdLoggerState(t *testing.T) {
	origDir := logDirPath
	origFile := logFile
	origPath := logFilePath
	origOutput := log.Writer()
	origFlags := log.Flags()
	origPrevOutput := previousLoggerOutput
	origPrevFlags := previousLoggerFlags
	origStateCaptured := loggerStateCaptured
	defer func() {
		logDirPath = origDir
		logFile = origFile
		logFilePath = origPath
		log.SetOutput(origOutput)
		log.SetFlags(origFlags)
		previousLoggerOutput = origPrevOutput
		previousLoggerFlags = origPrevFlags
		loggerStateCaptured = origStateCaptured
	}()

	home := t.TempDir()
	t.Setenv("HOME", home)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(log.Lshortfile)

	Init()
	Close()

	if log.Writer() != &buf {
		t.Fatal("Close should restore the previous standard logger output")
	}
	if log.Flags() != log.Lshortfile {
		t.Fatalf("Close should restore previous log flags, got %d", log.Flags())
	}

	log.Print("restored")
	if !bytes.Contains(buf.Bytes(), []byte("restored")) {
		t.Fatal("restored logger output should receive post-Close log entries")
	}
}

func TestInitTwiceClosesPreviousLogFile(t *testing.T) {
	origDir := logDirPath
	origFile := logFile
	origPath := logFilePath
	origOutput := log.Writer()
	origFlags := log.Flags()
	origPrevOutput := previousLoggerOutput
	origPrevFlags := previousLoggerFlags
	origStateCaptured := loggerStateCaptured
	defer func() {
		logDirPath = origDir
		logFile = origFile
		logFilePath = origPath
		log.SetOutput(origOutput)
		log.SetFlags(origFlags)
		previousLoggerOutput = origPrevOutput
		previousLoggerFlags = origPrevFlags
		loggerStateCaptured = origStateCaptured
	}()

	home := t.TempDir()
	t.Setenv("HOME", home)

	Init()
	firstLogFile := logFile
	if firstLogFile == nil {
		t.Fatal("Init should open a log file")
	}

	Init()

	if _, err := firstLogFile.Write([]byte("x")); err == nil {
		t.Fatal("second Init should close the previous log file handle")
	}

	Close()
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

func TestLogFileNameUsesNanosecondPrecision(t *testing.T) {
	base := time.Date(2025, time.January, 2, 3, 4, 5, 123456780, time.UTC)
	next := base.Add(time.Nanosecond)

	first := logFileName(base)
	second := logFileName(next)

	if first == second {
		t.Fatalf("expected distinct filenames for sub-second timestamps, got %q", first)
	}
}
