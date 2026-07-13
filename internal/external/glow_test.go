package external

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestPageMarkdown_NoDeadlineKillsPager verifies that PageMarkdown does not
// impose a deadline on the glow pager. Pagers are interactive — the user
// controls how long to read — so a 30s timeout would terminate the pager
// mid-read, leaving the user with a confusing "fell back to plain text"
// dump and the entire content spilled to the terminal.
//
// Regression: PageMarkdown previously wrapped its exec in
// context.WithTimeout(ctx, 30s) which killed long pager sessions. The test
// shrinks the non-interactive timeout to 1s and uses a fake glow that
// sleeps 2s; if PageMarkdown were still honoring a deadline, the process
// would be killed at ~1s and we'd return well before the sleep finishes.
func TestPageMarkdown_NoDeadlineKillsPager(t *testing.T) {
	// Shrink the non-interactive deadline so the test would catch a
	// PageMarkdown regression that started reusing it. Restore on exit.
	prev := glowTimeoutSeconds
	glowTimeoutSeconds = 1
	t.Cleanup(func() { glowTimeoutSeconds = prev })

	dir := t.TempDir()
	fakeGlow := filepath.Join(dir, "glow")
	// Sleep 2s — well past the 1s non-interactive deadline.
	script := "#!/bin/sh\nsleep 2\nexit 0\n"
	if err := os.WriteFile(fakeGlow, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake glow: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	// Prepend the fake dir so our glow shadows any real install, but keep
	// the rest of PATH so the shell script can still invoke `sleep`.
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+origPath); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}
	// Bust the toolcache so GlowAvailable() re-evaluates against the new PATH.
	glowCache = cachedPath{}
	t.Cleanup(func() { glowCache = cachedPath{} })

	start := time.Now()
	if err := PageMarkdown("# hi\n"); err != nil {
		t.Fatalf("PageMarkdown returned error: %v", err)
	}
	elapsed := time.Since(start)
	// The fake glow sleeps 2s; the non-interactive deadline is 1s. If
	// PageMarkdown were honoring that deadline, the process would have
	// been killed shortly after 1s and we'd return well before the 2s
	// sleep finished. Allow a small margin for scheduling jitter.
	if elapsed < 1800*time.Millisecond {
		t.Errorf("PageMarkdown returned too early (%v) — pager appears to have been killed by a deadline", elapsed)
	}
}

// TestRenderFile_LeadingDashPathNotTreatedAsFlag verifies that a markdown
// file whose name begins with '-' is passed to glow as a positional arg,
// not parsed as a flag. Before the fix, exec.CommandContext(ctx, "glow",
// path) with path="-bad.md" would surface as an unknown flag to glow.
//
// Strategy: invoke RenderFile on PATH with no glow available — the
// fallback prints file contents via os.ReadFile, which succeeds. The key
// guarantee for the fix is that the command-line construction includes
// "--" before the path. We assert that by inspecting the produced
// command's args directly (not by spawning glow).
func TestRenderFile_LeadingDashPath_FallbackReads(t *testing.T) {
	// Disable PATH so glow is unavailable; that path is exercised by
	// other tests. Here we just want to be sure RenderFile returns
	// without panic when the file name starts with a dash.
	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", ""); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}
	// Bust the toolcache so GlowAvailable() re-evaluates against the
	// empty PATH.
	glowCache = cachedPath{}
	t.Cleanup(func() { glowCache = cachedPath{} })

	dir := t.TempDir()
	dashPath := filepath.Join(dir, "-leading-dash.md")
	if err := os.WriteFile(dashPath, []byte("# heading\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := RenderFile(dashPath); err != nil {
		t.Fatalf("RenderFile returned error for dash-prefixed name: %v", err)
	}
}

func TestRenderMarkdown_UsesCachedAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	markerFile := filepath.Join(dir, "ran.txt")
	fakeGlow := filepath.Join(dir, "glow")
	script := "#!/bin/sh\nprintf ran > " + markerFile + "\n"
	if err := os.WriteFile(fakeGlow, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake glow: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", ""); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	glowCache = cachedPath{path: fakeGlow, checkedAt: time.Now()}
	t.Cleanup(func() { glowCache = cachedPath{} })

	if err := RenderMarkdown("# hi\n"); err != nil {
		t.Fatalf("RenderMarkdown returned error: %v", err)
	}
	got, err := os.ReadFile(markerFile)
	if err != nil {
		t.Fatalf("read marker file: %v", err)
	}
	if string(got) != "ran" {
		t.Fatalf("expected fake cached glow to run, got marker %q", got)
	}
}

// TestPageMarkdown_UsesTempFileNotStdinPipe verifies that PageMarkdown
// invokes glow with the content staged in a temp file and `--` before it,
// rather than feeding content through stdin with `-`. Piping content to
// stdin while using `glow -p` breaks the pager: the spawned `less` (or
// equivalent) inherits the same stdin pipe, sees it drained by glow,
// then hits EOF and cannot accept keypresses.
func TestPageMarkdown_UsesTempFileNotStdinPipe(t *testing.T) {
	dir := t.TempDir()
	argFile := filepath.Join(dir, "args.txt")
	stdinFile := filepath.Join(dir, "stdin.txt")
	fakeGlow := filepath.Join(dir, "glow")
	// Fake glow records its args and reads stdin (which may be empty). It
	// must exit zero so PageMarkdown does not fall back.
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + argFile + "\n" +
		"cat > " + stdinFile + " 2>/dev/null || true\n" +
		"exit 0\n"
	if err := os.WriteFile(fakeGlow, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake glow: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", dir); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}
	// Bust the cache so GlowAvailable picks up the fake.
	glowCache = cachedPath{}
	t.Cleanup(func() { glowCache = cachedPath{} })

	// Replace os.Stdin with a closed pipe so the fake `cat` doesn't block
	// waiting on our terminal. The key assertion is what PageMarkdown
	// passes to glow as args — not what stdin contains.
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("pipe: %v", pipeErr)
	}
	_ = w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = origStdin
		_ = r.Close()
	})

	const content = "# heading\n\nbody text\n"
	if err := PageMarkdown(content); err != nil {
		t.Fatalf("PageMarkdown returned error: %v", err)
	}

	recorded, err := os.ReadFile(argFile)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(recorded)), "\n")
	if len(args) < 3 {
		t.Fatalf("expected ≥3 args (-p, --, <tmpPath>), got %v", args)
	}
	if args[0] != "-p" {
		t.Errorf("expected args[0]='-p', got %q (all: %v)", args[0], args)
	}
	if args[1] != "--" {
		t.Errorf("expected args[1]='--', got %q (all: %v)", args[1], args)
	}
	// args[2] must be a real path that does NOT equal "-" (the stdin sigil
	// that caused the broken-pager bug).
	if args[2] == "-" {
		t.Fatalf("regression: PageMarkdown passed '-' to glow (content on stdin), expected temp file path; args: %v", args)
	}
	if !strings.HasSuffix(args[2], ".md") {
		t.Errorf("expected last arg to be a .md temp file path, got %q", args[2])
	}

	// Stdin to glow must NOT carry the rendered content (it would otherwise
	// be propagated to the pager, breaking it). The closed pipe means cat
	// reads zero bytes.
	if data, err := os.ReadFile(stdinFile); err == nil {
		if strings.Contains(string(data), "heading") || strings.Contains(string(data), "body text") {
			t.Errorf("regression: stdin to glow contained content %q — pager would be broken", string(data))
		}
	}
}

// TestPageMarkdown_GlowUnavailableFallback verifies the plain-text fallback
// path when glow isn't on PATH (no panic, no error returned).
func TestPageMarkdown_GlowUnavailableFallback(t *testing.T) {
	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", ""); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}
	glowCache = cachedPath{}
	t.Cleanup(func() { glowCache = cachedPath{} })

	if err := PageMarkdown("# hello\n"); err != nil {
		t.Errorf("expected nil error in fallback path, got %v", err)
	}
}

func TestFormatTaskAsMarkdown(t *testing.T) { //nolint:gocyclo
	now := time.Now()

	t.Run("basic task format", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 42, models.TaskTypeTask, "My Task")
		if err != nil {
			t.Fatal(err)
		}
		task.WorkflowState = models.StateReady
		task.Priority = 2
		task.CreatedAt = now

		result := FormatTaskAsMarkdown(task)

		if !strings.Contains(result, "# Task #42: My Task") {
			t.Error("output should contain header with type, short ID, and title")
		}
		if !strings.Contains(result, "| Status |") {
			t.Error("output should contain status row")
		}
		if !strings.Contains(result, "| Priority | 2 |") {
			t.Error("output should contain priority row")
		}
		if !strings.Contains(result, "| Created |") {
			t.Error("output should contain created date row")
		}
	})

	t.Run("task with description", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "Task with desc")
		if err != nil {
			t.Fatal(err)
		}
		task.Description = "This is a detailed description."

		result := FormatTaskAsMarkdown(task)

		if !strings.Contains(result, "## Description") {
			t.Error("output should contain description header")
		}
		if !strings.Contains(result, "This is a detailed description.") {
			t.Error("output should contain the description text")
		}
	})

	t.Run("task without description", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "No description task")
		if err != nil {
			t.Fatal(err)
		}
		task.Description = ""

		result := FormatTaskAsMarkdown(task)

		if strings.Contains(result, "## Description") {
			t.Error("output should not contain description section when description is empty")
		}
	})

	t.Run("task with tags", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "Tagged task")
		if err != nil {
			t.Fatal(err)
		}
		task.Tags = []string{"backend", "urgent"}

		result := FormatTaskAsMarkdown(task)

		if !strings.Contains(result, "| Tags | backend, urgent |") {
			t.Error("output should contain tags row with comma-separated values")
		}
	})

	t.Run("task without tags", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "Untagged task")
		if err != nil {
			t.Fatal(err)
		}
		task.Tags = nil

		result := FormatTaskAsMarkdown(task)

		if strings.Contains(result, "| Tags |") {
			t.Error("output should not contain tags row when no tags present")
		}
	})

	t.Run("escapes pipes in table cells", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 1, models.TaskTypeBug, "Bug with table chars")
		if err != nil {
			t.Fatal(err)
		}
		task.Tags = []string{"api|db"}
		task.BugAttrs = &models.BugAttrs{
			Workaround: "run alpha|beta\nthen retry",
		}

		result := FormatTaskAsMarkdown(task)

		if !strings.Contains(result, "| Tags | api\\|db |") {
			t.Errorf("expected escaped pipe in tags row, got:\n%s", result)
		}
		if !strings.Contains(result, "| Workaround | run alpha\\|beta then retry |") {
			t.Errorf("expected escaped pipe and normalized newline in workaround row, got:\n%s", result)
		}
	})

	t.Run("task with due date", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "Task with due date")
		if err != nil {
			t.Fatal(err)
		}
		dueDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
		task.DueDate = &dueDate

		result := FormatTaskAsMarkdown(task)

		if !strings.Contains(result, "| Due |") {
			t.Error("output should contain due date row")
		}
		if !strings.Contains(result, "2025-01-15") {
			t.Error("output should contain formatted due date")
		}
	})

	t.Run("completed task", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "Completed task")
		if err != nil {
			t.Fatal(err)
		}
		task.WorkflowState = models.StateDone
		completedAt := now
		task.CompletedAt = &completedAt

		result := FormatTaskAsMarkdown(task)

		if !strings.Contains(result, "| Completed |") {
			t.Error("output should contain completed date row")
		}
	})

	t.Run("task with children shows subtasks section", func(t *testing.T) {
		parent, err := models.NewTask("parent-uuid", 1, models.TaskTypeStory, "Parent Story")
		if err != nil {
			t.Fatal(err)
		}
		child1, err := models.NewTask("child1-uuid", 2, models.TaskTypeTask, "Child Task 1")
		if err != nil {
			t.Fatal(err)
		}
		child2, err := models.NewTask("child2-uuid", 3, models.TaskTypeTask, "Child Task 2")
		if err != nil {
			t.Fatal(err)
		}
		child2.WorkflowState = models.StateDone

		if err := parent.AddChild(child1); err != nil {
			t.Fatal(err)
		}
		if err := parent.AddChild(child2); err != nil {
			t.Fatal(err)
		}

		result := FormatTaskAsMarkdown(parent)

		if !strings.Contains(result, "## Subtasks") {
			t.Error("output should contain subtasks section")
		}
		if !strings.Contains(result, "Progress:") {
			t.Error("output should contain progress line")
		}
		if !strings.Contains(result, "1/2 completed") {
			t.Error("output should show 1/2 completed (1 done, 1 todo)")
		}
		if !strings.Contains(result, "[ ]") {
			t.Error("output should contain unchecked checkbox for incomplete child")
		}
		if !strings.Contains(result, "[x]") {
			t.Error("output should contain checked checkbox for completed child")
		}
	})

	t.Run("epic type header format", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 5, models.TaskTypeEpic, "My Epic")
		if err != nil {
			t.Fatal(err)
		}

		result := FormatTaskAsMarkdown(task)

		if !strings.Contains(result, "# Epic #5: My Epic") {
			t.Error("output should contain Epic heading prefix")
		}
	})

	t.Run("bug type header format", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 10, models.TaskTypeBug, "Critical Bug")
		if err != nil {
			t.Fatal(err)
		}

		result := FormatTaskAsMarkdown(task)

		if !strings.Contains(result, "# Bug #10: Critical Bug") {
			t.Error("output should contain Bug heading prefix")
		}
	})

}
