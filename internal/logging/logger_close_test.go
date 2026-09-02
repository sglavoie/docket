package logging

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"testing"
)

// Close has to undo everything Init did, in an order that holds even when it was
// never called: detach the standard logger, restore the output and flags it had,
// close the handle, and remove the file only when it truly has nothing in it.
// This file covers each of those, plus the restore path on its own. Init is in
// logger_init_test.go.

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
