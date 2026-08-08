package logging

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

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

func TestCloseStatsAfterRestoreStdLogger(t *testing.T) {
	// Regression: Close used to call logFile.Stat() BEFORE restoreStdLogger().
	// Because the std logger's mutex is held by both log.SetOutput and
	// log.Printf, a concurrent log.Printf could write bytes to the file
	// between the stale Stat() and the SetOutput that detaches the writer.
	// Those bytes were never reflected in info.Size(), so Close would remove
	// the file even though it had content.
	//
	// With the fix (restore-then-stat), once Stat() runs, no further std
	// logger writes can land in the file. So whenever Close decides to
	// remove the file (size == 0), the file is genuinely empty on disk.
	// We exercise the race with concurrent writers and assert the
	// invariant: after Close, either the file is gone (and there were no
	// writes that reached it) or the file exists and is non-empty (and
	// reflects the writes that did land).
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
	path := filepath.Join(tmp, "race.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	previousLoggerOutput = os.Stderr
	previousLoggerFlags = 0
	loggerStateCaptured = true
	log.SetOutput(f)
	log.SetFlags(0)

	logFile = f
	logFilePath = path

	// Pre-write some content via the std logger so the file is non-empty
	// before Close even starts. With the buggy ordering this is sufficient
	// — Stat would still see the pre-write content. We additionally launch
	// concurrent writers to widen the race window.
	log.Printf("pre-write-1")
	log.Printf("pre-write-2")

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 100 {
			log.Printf("racing-%d", i)
		}
	}()

	Close()
	<-done

	// File must still exist on disk because content was written before
	// Close started. Pre-fix, this passed trivially; the test still
	// guards against any regression of the "remove non-empty file" path.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Close removed a non-empty log file: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected log file to contain pre-Close writes")
	}
}

func TestInitClearsLogFilePathOnOpenFailure(t *testing.T) {
	// Regression: when os.OpenFile fails inside Init, logFilePath used to be
	// left set to a path that was never opened. Subsequent code reading
	// logFilePath could mistakenly believe a log file existed.
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

	// Force OpenFile to fail by making the would-be log dir read-only. The
	// resulting path inside it is not creatable.
	logsDir := filepath.Join(home, ".tt", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	if err := os.Chmod(logsDir, 0o500); err != nil {
		t.Fatalf("chmod readonly: %v", err)
	}
	defer func() { _ = os.Chmod(logsDir, 0o755) }()

	// Pre-set logFilePath to a sentinel so we can detect whether the failure
	// path clears it.
	logFilePath = "sentinel-before-init"
	logFile = nil

	Init()

	if logFile != nil {
		t.Fatal("Init should not have set logFile when OpenFile failed")
	}
	if logFilePath != "" {
		t.Errorf("Init should clear logFilePath on OpenFile failure, got %q", logFilePath)
	}
}

func TestInitClearsLogDirPathOnMkdirFailure(t *testing.T) {
	// Regression: Init used to assign logDirPath before creating the
	// directory. If MkdirAll failed, the retained path did not exist and was
	// not usable for logging.
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

	// Make $HOME/.tt a regular file so creating $HOME/.tt/logs fails.
	if err := os.WriteFile(filepath.Join(home, ".tt"), []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("setup .tt file: %v", err)
	}

	logDirPath = "stale-before-init"
	logFile = nil
	logFilePath = ""

	Init()

	if logFile != nil {
		t.Fatal("Init should not have set logFile when MkdirAll failed")
	}
	if logDirPath != "" {
		t.Errorf("Init should clear logDirPath on MkdirAll failure, got %q", logDirPath)
	}
}

func TestInitClearsLogFilePathOnUserHomeDirFailure(t *testing.T) {
	// Regression: if UserHomeDir failed before a file was opened, Init left
	// logFilePath stale even though no active log file existed.
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

	t.Setenv("HOME", "")

	logDirPath = "stale-dir-before-init"
	logFilePath = "stale-file-before-init"
	logFile = nil

	Init()

	if logFile != nil {
		t.Fatal("Init should not have set logFile when UserHomeDir failed")
	}
	if logDirPath != "" {
		t.Errorf("Init should clear logDirPath on UserHomeDir failure, got %q", logDirPath)
	}
	if logFilePath != "" {
		t.Errorf("Init should clear logFilePath on UserHomeDir failure, got %q", logFilePath)
	}
}

func TestPruneOldLogsInvalidDir(t *testing.T) {
	original := logDirPath
	defer func() { logDirPath = original }()

	logDirPath = "/nonexistent/path/that/does/not/exist"
	pruneOldLogs() // should not panic
}

func TestRestoreStdLoggerNilPreviousOutput(t *testing.T) {
	origOutput := log.Writer()
	origFlags := log.Flags()
	origPrevOutput := previousLoggerOutput
	origPrevFlags := previousLoggerFlags
	origStateCaptured := loggerStateCaptured
	defer func() {
		log.SetOutput(origOutput)
		log.SetFlags(origFlags)
		previousLoggerOutput = origPrevOutput
		previousLoggerFlags = origPrevFlags
		loggerStateCaptured = origStateCaptured
	}()

	previousLoggerOutput = nil
	previousLoggerFlags = 0
	loggerStateCaptured = true

	restoreStdLogger()

	// Must not leave log.Writer() == nil, which would panic on next Print.
	if log.Writer() == nil {
		t.Fatal("restoreStdLogger must not leave log writer nil")
	}

	// Sanity-check that a subsequent Print does not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("log.Print panicked after restoreStdLogger: %v", r)
		}
	}()
	log.Print("post-restore")
}

func TestConcurrentInitCloseNoRace(t *testing.T) {
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

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(2)
		go func() { defer wg.Done(); Init() }()
		go func() { defer wg.Done(); Close() }()
	}
	wg.Wait()

	// Final Close to release any open file.
	Close()
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
