package external

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/aiharness"
)

func TestHarnessRunner_AdaptersUseShellFreeStdinContract(t *testing.T) {
	tests := []struct {
		harness aiharness.Name
		args    []string
		envKey  string
	}{
		{
			harness: aiharness.Claude,
			args: []string{
				"--print",
				"--output-format", "text",
				"--permission-mode", "dontAsk",
				"--tools", "",
				"--no-session-persistence",
			},
			envKey: "CLAUDE_CONFIG_DIR",
		},
		{
			harness: aiharness.Codex,
			args: []string{
				"exec",
				"--ephemeral",
				"--sandbox", "read-only",
				"--skip-git-repo-check",
				"--color", "never",
				"-",
			},
			envKey: "CODEX_HOME",
		},
		{
			harness: aiharness.Copilot,
			args: []string{
				"--silent",
				"--no-ask-user",
				"--no-color",
				"--disable-builtin-mcps",
				"--deny-tool", "shell",
				"--deny-tool", "write",
			},
			envKey: "COPILOT_HOME",
		},
	}

	for _, test := range tests {
		t.Run(test.harness.String(), func(t *testing.T) {
			recordDir := t.TempDir()
			configDir := t.TempDir()
			workspaceDir := t.TempDir()
			executable := writeFakeHarness(t, `
printf 'cwd=%s\n' "$PWD" > "$RECORD_DIR/meta"
if [ -z "$(ls -A .)" ]; then
  printf 'empty=yes\n' >> "$RECORD_DIR/meta"
else
  printf 'empty=no\n' >> "$RECORD_DIR/meta"
fi
printf 'CLAUDE_CONFIG_DIR=%s\n' "${CLAUDE_CONFIG_DIR-}" >> "$RECORD_DIR/meta"
printf 'CODEX_HOME=%s\n' "${CODEX_HOME-}" >> "$RECORD_DIR/meta"
printf 'COPILOT_HOME=%s\n' "${COPILOT_HOME-}" >> "$RECORD_DIR/meta"
printf 'TT_DATA_DIR=%s\n' "${TT_DATA_DIR-}" >> "$RECORD_DIR/meta"
for arg in "$@"; do
  printf 'arg=<%s>\n' "$arg" >> "$RECORD_DIR/meta"
done
cat > "$RECORD_DIR/stdin"
printf '{"version":2,"summary":"ok","actions":[]}'
`)

			var workDirs []string
			runner := newHarnessRunner(runnerDeps{
				lookup: func(harness aiharness.Name) (string, error) {
					if harness != test.harness {
						t.Fatalf("lookup harness = %q, want %q", harness, test.harness)
					}
					return executable, nil
				},
				makeTempDir: recordingTempDirFactory(t, &workDirs),
				environ: func() []string {
					return []string{
						"PATH=/bin:/usr/bin",
						"RECORD_DIR=" + recordDir,
						"CLAUDE_CONFIG_DIR=inherited-claude",
						"CODEX_HOME=inherited-codex",
						"COPILOT_HOME=inherited-copilot",
						"TT_DATA_DIR=inherited-tt-data",
					}
				},
			})

			response, err := runner.Run(context.Background(), Request{
				Harness:      test.harness,
				ConfigDir:    configDir,
				WorkspaceDir: workspaceDir,
				Query:        "private query",
				SystemPrompt: "private system prompt",
			})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if response != `{"version":2,"summary":"ok","actions":[]}` {
				t.Fatalf("response = %q", response)
			}
			if _, err := ParseActionPlan(response); err != nil {
				t.Fatalf("shared raw response did not satisfy the strict parser: %v", err)
			}

			meta := readTestFile(t, filepath.Join(recordDir, "meta"))
			if strings.Contains(meta, "private query") || strings.Contains(meta, "private system prompt") {
				t.Fatalf("provider argv leaked prompt text: %s", meta)
			}
			if !strings.Contains(meta, "empty=yes\n") {
				t.Fatalf("provider working directory was not fresh and empty: %s", meta)
			}
			wantArgs := test.args
			if test.harness == aiharness.Codex {
				wantArgs = append([]string{"exec", "--cd", workspaceDir}, test.args[1:]...)
			}
			if got := extractArgs(meta); !slices.Equal(got, wantArgs) {
				t.Fatalf("provider argv = %#v, want %#v", got, wantArgs)
			}
			if !strings.Contains(meta, test.envKey+"="+configDir+"\n") {
				t.Fatalf("selected config environment was not overridden: %s", meta)
			}
			for key, inherited := range map[string]string{
				"CLAUDE_CONFIG_DIR": "inherited-claude",
				"CODEX_HOME":        "inherited-codex",
				"COPILOT_HOME":      "inherited-copilot",
			} {
				if key == test.envKey {
					continue
				}
				if !strings.Contains(meta, key+"="+inherited+"\n") {
					t.Fatalf("unselected provider environment changed: %s", meta)
				}
			}
			wantWorkspace := "inherited-tt-data"
			if test.harness == aiharness.Codex {
				wantWorkspace = workspaceDir
			}
			if !strings.Contains(meta, "TT_DATA_DIR="+wantWorkspace+"\n") {
				t.Fatalf("TT_DATA_DIR = %q, want %q: %s", meta, wantWorkspace, meta)
			}

			stdin := readTestFile(t, filepath.Join(recordDir, "stdin"))
			if stdin != providerPrompt("private query", "private system prompt") {
				t.Fatalf("provider stdin = %q", stdin)
			}
			assertWorkDirsRemoved(t, workDirs)
		})
	}
}

func TestHarnessRunner_EmptyConfigDirPreservesInheritedEnvironment(t *testing.T) {
	recordDir := t.TempDir()
	executable := writeFakeHarness(t, `
printf '%s' "${CODEX_HOME-}" > "$RECORD_DIR/config"
printf '%s' "${TT_DATA_DIR-}" > "$RECORD_DIR/workspace"
for arg in "$@"; do
  printf 'arg=<%s>\n' "$arg" >> "$RECORD_DIR/args"
done
printf ok
`)
	var workDirs []string
	runner := newHarnessRunner(runnerDeps{
		lookup:      func(aiharness.Name) (string, error) { return executable, nil },
		makeTempDir: recordingTempDirFactory(t, &workDirs),
		environ: func() []string {
			return []string{"PATH=/bin:/usr/bin", "RECORD_DIR=" + recordDir, "CODEX_HOME=inherited", "TT_DATA_DIR=inherited-workspace"}
		},
	})

	response, err := runner.Run(context.Background(), Request{Harness: aiharness.Codex, Query: "query"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if response != "ok" {
		t.Fatalf("response = %q", response)
	}
	if got := readTestFile(t, filepath.Join(recordDir, "config")); got != "inherited" {
		t.Fatalf("CODEX_HOME = %q, want inherited", got)
	}
	if got := readTestFile(t, filepath.Join(recordDir, "workspace")); got != "inherited-workspace" {
		t.Fatalf("TT_DATA_DIR = %q, want inherited workspace", got)
	}
	if got := extractArgs(readTestFile(t, filepath.Join(recordDir, "args"))); !slices.Equal(got, []string{"exec", "--ephemeral", "--sandbox", "read-only", "--skip-git-repo-check", "--color", "never", "-"}) {
		t.Fatalf("empty workspace Codex argv = %#v", got)
	}
	assertWorkDirsRemoved(t, workDirs)
}

func TestHarnessRunner_CancellationWinsAndCleansUp(t *testing.T) {
	recordDir := t.TempDir()
	executable := writeFakeHarness(t, `
printf '%s' "$PWD" > "$RECORD_DIR/cwd"
while :; do :; done
`)
	var workDirs []string
	runner := newHarnessRunner(runnerDeps{
		lookup:      func(aiharness.Name) (string, error) { return executable, nil },
		makeTempDir: recordingTempDirFactory(t, &workDirs),
		environ:     func() []string { return []string{"PATH=/bin:/usr/bin", "RECORD_DIR=" + recordDir} },
	})

	stream, err := runner.Start(context.Background(), Request{Harness: aiharness.Claude, Query: "query"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForFile(t, filepath.Join(recordDir, "cwd"))
	stream.Cancel()
	started := time.Now()
	if err := stream.Wait(); !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("cancellation took %s", elapsed)
	}
	_, _ = io.ReadAll(stream)
	if err := stream.Wait(); !errors.Is(err, context.Canceled) {
		t.Fatalf("repeated Wait error = %v", err)
	}
	assertWorkDirsRemoved(t, workDirs)
}

func TestHarnessRunner_CloseTreatsBareCancellationAsSuccess(t *testing.T) {
	recordDir := t.TempDir()
	executable := writeFakeHarness(t, `
printf ready > "$RECORD_DIR/ready"
while :; do :; done
`)
	var workDirs []string
	runner := newHarnessRunner(runnerDeps{
		lookup:      func(aiharness.Name) (string, error) { return executable, nil },
		makeTempDir: recordingTempDirFactory(t, &workDirs),
		environ:     func() []string { return []string{"PATH=/bin:/usr/bin", "RECORD_DIR=" + recordDir} },
	})

	stream, err := runner.Start(context.Background(), Request{Harness: aiharness.Claude, Query: "query"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForFile(t, filepath.Join(recordDir, "ready"))
	if err := stream.Close(); err != nil {
		t.Fatalf("Close error = %v, want nil", err)
	}
	if err := stream.Wait(); !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait error = %v, want repeatable context.Canceled result", err)
	}
	assertWorkDirsRemoved(t, workDirs)
}

func TestHarnessRunner_ClosePreservesCleanupFailureAfterCancel(t *testing.T) {
	recordDir := t.TempDir()
	executable := writeFakeHarness(t, `
printf ready > "$RECORD_DIR/ready"
while :; do :; done
`)
	cleanupErr := errors.New("synthetic cleanup failure")
	runner := newHarnessRunner(runnerDeps{
		lookup:      func(aiharness.Name) (string, error) { return executable, nil },
		makeTempDir: func() (string, error) { return os.MkdirTemp(t.TempDir(), "run-") },
		removeAll:   func(string) error { return cleanupErr },
		environ:     func() []string { return []string{"PATH=/bin:/usr/bin", "RECORD_DIR=" + recordDir} },
	})

	stream, err := runner.Start(context.Background(), Request{Harness: aiharness.Claude, Query: "query"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForFile(t, filepath.Join(recordDir, "ready"))
	stream.Cancel()
	err = stream.Close()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Close error = %v, want context.Canceled in error chain", err)
	}
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("Close error = %v, want cleanup failure in error chain", err)
	}
	if waitErr := stream.Wait(); !errors.Is(waitErr, context.Canceled) || !errors.Is(waitErr, cleanupErr) {
		t.Fatalf("Wait error = %v, want repeatable cancellation and cleanup failures", waitErr)
	}
}

func TestHarnessRunner_ClosePreservesParentDeadline(t *testing.T) {
	recordDir := t.TempDir()
	executable := writeFakeHarness(t, `
printf ready > "$RECORD_DIR/ready"
while :; do :; done
`)
	var workDirs []string
	runner := newHarnessRunner(runnerDeps{
		lookup:      func(aiharness.Name) (string, error) { return executable, nil },
		makeTempDir: recordingTempDirFactory(t, &workDirs),
		environ:     func() []string { return []string{"PATH=/bin:/usr/bin", "RECORD_DIR=" + recordDir} },
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stream, err := runner.Start(ctx, Request{Harness: aiharness.Claude, Query: "query"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForFile(t, filepath.Join(recordDir, "ready"))
	<-ctx.Done()
	if err := stream.Close(); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close error = %v, want context.DeadlineExceeded", err)
	}
	assertWorkDirsRemoved(t, workDirs)
}

func TestHarnessRunner_WaitIsRepeatableAndConcurrent(t *testing.T) {
	executable := writeFakeHarness(t, `printf response`)
	var workDirs []string
	runner := newHarnessRunner(runnerDeps{
		lookup:      func(aiharness.Name) (string, error) { return executable, nil },
		makeTempDir: recordingTempDirFactory(t, &workDirs),
		environ:     func() []string { return []string{"PATH=/bin:/usr/bin"} },
	})
	stream, err := runner.Start(context.Background(), Request{Harness: aiharness.Copilot, Query: "query"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got, err := io.ReadAll(stream); err != nil || string(got) != "response" {
		t.Fatalf("ReadAll = %q, %v", got, err)
	}

	const waiters = 12
	errorsSeen := make(chan error, waiters)
	var waitGroup sync.WaitGroup
	for range waiters {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			errorsSeen <- stream.Wait()
		}()
	}
	waitGroup.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if err != nil {
			t.Fatalf("Wait: %v", err)
		}
	}
	assertWorkDirsRemoved(t, workDirs)
}

func TestHarnessRunner_NonzeroExitBoundsAndSanitizesStderr(t *testing.T) {
	executable := writeFakeHarness(t, `
printf '\033[2Jprovider failed\n' >&2
head -c 70000 /dev/zero | tr '\000' x >&2
exit 7
`)
	for _, harness := range aiharness.Names() {
		t.Run(harness.String(), func(t *testing.T) {
			var workDirs []string
			runner := newHarnessRunner(runnerDeps{
				lookup:      func(aiharness.Name) (string, error) { return executable, nil },
				makeTempDir: recordingTempDirFactory(t, &workDirs),
				environ:     func() []string { return []string{"PATH=/bin:/usr/bin"} },
			})
			stream, err := runner.Start(context.Background(), Request{Harness: harness, Query: "query"})
			if err != nil {
				t.Fatalf("Start: %v", err)
			}
			if _, readErr := io.ReadAll(stream); readErr != nil {
				t.Fatalf("ReadAll: %v", readErr)
			}
			err = stream.Wait()
			if err == nil || !strings.Contains(err.Error(), harness.String()+" exited with code 7") {
				t.Fatalf("Wait error = %v", err)
			}
			if !strings.Contains(err.Error(), "stderr truncated at 64 KiB") || !stream.StderrTruncated() {
				t.Fatalf("stderr truncation was not marked: %v", err)
			}
			if strings.Contains(err.Error(), "\x1b") {
				t.Fatalf("stderr error retained a raw escape byte: %q", err)
			}
			if !strings.Contains(err.Error(), `\x1b[2Jprovider failed`) {
				t.Fatalf("stderr control was not rendered safely: %q", err)
			}
			if closeErr := stream.Close(); closeErr == nil || !strings.Contains(closeErr.Error(), harness.String()+" exited with code 7") {
				t.Fatalf("Close error = %v, want repeatable provider failure", closeErr)
			}
			assertWorkDirsRemoved(t, workDirs)
		})
	}
}

func TestHarnessRunner_OutputLimitCancelsAndCleansUp(t *testing.T) {
	executable := writeFakeHarness(t, fmt.Sprintf(`head -c %d /dev/zero`, MaxHarnessOutputBytes+1))
	for _, harness := range aiharness.Names() {
		t.Run(harness.String(), func(t *testing.T) {
			var workDirs []string
			runner := newHarnessRunner(runnerDeps{
				lookup:      func(aiharness.Name) (string, error) { return executable, nil },
				makeTempDir: recordingTempDirFactory(t, &workDirs),
				environ:     func() []string { return []string{"PATH=/bin:/usr/bin"} },
			})
			stream, err := runner.Start(context.Background(), Request{Harness: harness, Query: "query"})
			if err != nil {
				t.Fatalf("Start: %v", err)
			}
			response, readErr := io.ReadAll(stream)
			if !errors.Is(readErr, ErrHarnessOutputLimit) {
				t.Fatalf("ReadAll error = %v, want ErrHarnessOutputLimit", readErr)
			}
			if len(response) != MaxHarnessOutputBytes {
				t.Fatalf("retained output = %d bytes, want %d", len(response), MaxHarnessOutputBytes)
			}
			if err := stream.Wait(); !errors.Is(err, ErrHarnessOutputLimit) {
				t.Fatalf("Wait error = %v, want ErrHarnessOutputLimit", err)
			}
			if err := stream.Close(); !errors.Is(err, ErrHarnessOutputLimit) {
				t.Fatalf("Close error = %v, want repeatable ErrHarnessOutputLimit", err)
			}
			assertWorkDirsRemoved(t, workDirs)
		})
	}
}

func TestHarnessRunner_StartFailureCleansTemporaryDirectory(t *testing.T) {
	notExecutable := filepath.Join(t.TempDir(), "not-executable")
	if err := os.WriteFile(notExecutable, []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}
	var workDirs []string
	runner := newHarnessRunner(runnerDeps{
		lookup:      func(aiharness.Name) (string, error) { return notExecutable, nil },
		makeTempDir: recordingTempDirFactory(t, &workDirs),
		environ:     func() []string { return []string{"PATH=/bin:/usr/bin"} },
	})
	if _, err := runner.Start(context.Background(), Request{Harness: aiharness.Claude}); err == nil {
		t.Fatal("Start unexpectedly succeeded")
	}
	assertWorkDirsRemoved(t, workDirs)
}

func TestHarnessRunner_InvalidConfigDirStopsBeforeInvocation(t *testing.T) {
	invoked := filepath.Join(t.TempDir(), "invoked")
	executable := writeFakeHarness(t, `printf invoked > "$INVOKED"`)
	tempDirCalled := false
	runner := newHarnessRunner(runnerDeps{
		lookup: func(aiharness.Name) (string, error) { return executable, nil },
		makeTempDir: func() (string, error) {
			tempDirCalled = true
			return os.MkdirTemp(t.TempDir(), "run-")
		},
		environ: func() []string { return []string{"PATH=/bin:/usr/bin", "INVOKED=" + invoked} },
	})

	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := runner.Run(context.Background(), Request{
		Harness:   aiharness.Copilot,
		ConfigDir: missing,
		Query:     "query",
	}); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("Run error = %v, want invalid config directory", err)
	}
	if tempDirCalled {
		t.Fatal("runner created a working directory for an invalid config override")
	}
	if _, err := os.Stat(invoked); !os.IsNotExist(err) {
		t.Fatalf("provider was invoked despite invalid config: %v", err)
	}
}

func TestHarnessRunner_InvalidCodexWorkspaceDirStopsBeforeInvocation(t *testing.T) {
	invoked := filepath.Join(t.TempDir(), "invoked")
	executable := writeFakeHarness(t, `printf invoked > "$INVOKED"`)
	tempDirCalled := false
	runner := newHarnessRunner(runnerDeps{
		lookup: func(aiharness.Name) (string, error) { return executable, nil },
		makeTempDir: func() (string, error) {
			tempDirCalled = true
			return os.MkdirTemp(t.TempDir(), "run-")
		},
		environ: func() []string { return []string{"PATH=/bin:/usr/bin", "INVOKED=" + invoked} },
	})

	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := runner.Run(context.Background(), Request{
		Harness:      aiharness.Codex,
		WorkspaceDir: missing,
		Query:        "query",
	}); err == nil || !strings.Contains(err.Error(), "AI workspace directory does not exist") {
		t.Fatalf("Run error = %v, want invalid Codex workspace directory", err)
	}
	if tempDirCalled {
		t.Fatal("runner created a temporary directory for an invalid Codex workspace")
	}
	if _, err := os.Stat(invoked); !os.IsNotExist(err) {
		t.Fatalf("provider was invoked despite invalid Codex workspace: %v", err)
	}
}

func TestHarnessRunner_MissingExecutableStopsBeforeTemporaryDirectory(t *testing.T) {
	tempDirCalled := false
	runner := newHarnessRunner(runnerDeps{
		lookup: func(aiharness.Name) (string, error) {
			return "", errors.New("executable not found")
		},
		makeTempDir: func() (string, error) {
			tempDirCalled = true
			return "", errors.New("should not be called")
		},
		environ: func() []string { return nil },
	})
	if _, err := runner.Start(context.Background(), Request{Harness: aiharness.Codex}); err == nil || !strings.Contains(err.Error(), "codex not found in PATH") {
		t.Fatalf("Start error = %v, want missing executable", err)
	}
	if tempDirCalled {
		t.Fatal("runner created a working directory without an executable")
	}
}

func TestCachedHarnessPath_UsesProviderSpecificCaches(t *testing.T) {
	now := time.Now()
	claudeCache = cachedPath{path: "/cached/claude", checkedAt: now}
	codexCache = cachedPath{path: "/cached/codex", checkedAt: now}
	copilotCache = cachedPath{path: "/cached/copilot", checkedAt: now}
	t.Cleanup(func() {
		claudeCache = cachedPath{}
		codexCache = cachedPath{}
		copilotCache = cachedPath{}
	})

	for harness, want := range map[aiharness.Name]string{
		aiharness.Claude:  "/cached/claude",
		aiharness.Codex:   "/cached/codex",
		aiharness.Copilot: "/cached/copilot",
	} {
		got, err := CachedHarnessPath(harness)
		if err != nil || got != want {
			t.Fatalf("CachedHarnessPath(%q) = %q, %v; want %q", harness, got, err, want)
		}
	}
}

func writeFakeHarness(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-harness")
	script := "#!/bin/sh\nset -eu\n" + strings.TrimSpace(body) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func recordingTempDirFactory(t *testing.T, workDirs *[]string) func() (string, error) {
	t.Helper()
	base := t.TempDir()
	return func() (string, error) {
		path, err := os.MkdirTemp(base, "run-")
		if err == nil {
			*workDirs = append(*workDirs, path)
		}
		return path, err
	}
}

func assertWorkDirsRemoved(t *testing.T, workDirs []string) {
	t.Helper()
	if len(workDirs) == 0 {
		t.Fatal("runner did not create a temporary working directory")
	}
	for _, path := range workDirs {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("temporary working directory still exists: %s (%v)", path, err)
		}
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func extractArgs(meta string) []string {
	var args []string
	for _, line := range strings.Split(meta, "\n") {
		if strings.HasPrefix(line, "arg=<") && strings.HasSuffix(line, ">") {
			args = append(args, strings.TrimSuffix(strings.TrimPrefix(line, "arg=<"), ">"))
		}
	}
	return args
}
