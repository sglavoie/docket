package logging

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Init either takes the standard logger over completely or leaves no trace: a
// failure part-way through must clear the paths it had already chosen, since
// code downstream reads them as "a log file exists". This file covers the happy
// path, a second Init over the first, each failure point, and the two of them
// racing. Close is in logger_close_test.go.

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
