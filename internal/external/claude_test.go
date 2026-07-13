package external

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestClaudeAsk_CanceledContextReturnsCtxErr verifies that when the caller
// cancels the context, ClaudeAsk surfaces ctx.Err() (context.Canceled)
// instead of an opaque "claude exited with code -1: signal: killed" error
// from cmd.Output(). Callers rely on errors.Is(err, context.Canceled) to
// distinguish user-driven cancellation from a real claude failure.
func TestClaudeAsk_CanceledContextReturnsCtxErr(t *testing.T) {
	// Stage a fake claude that just sleeps so we can kill it via context.
	dir := t.TempDir()
	fakeClaude := filepath.Join(dir, "claude")
	script := "#!/bin/sh\nsleep 5\n"
	if err := os.WriteFile(fakeClaude, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", dir); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}
	claudeCache = cachedPath{}
	t.Cleanup(func() { claudeCache = cachedPath{} })

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel before invocation so the child is killed almost immediately.
	cancel()

	_, err := ClaudeAsk(ctx, "hi", "")
	if err == nil {
		t.Fatal("expected error from canceled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected errors.Is(err, context.Canceled), got %v", err)
	}
	if strings.Contains(err.Error(), "claude exited with code") {
		t.Errorf("regression: surfaced misleading exit-code error for cancellation: %v", err)
	}
}

func TestClaudeAsk_UsesCachedAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	fakeClaude := filepath.Join(dir, "claude")
	script := "#!/bin/sh\necho cached claude\n"
	if err := os.WriteFile(fakeClaude, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", ""); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	claudeCache = cachedPath{path: fakeClaude, checkedAt: time.Now()}
	t.Cleanup(func() { claudeCache = cachedPath{} })

	got, err := ClaudeAsk(context.Background(), "hi", "")
	if err != nil {
		t.Fatalf("ClaudeAsk returned error: %v", err)
	}
	if got != "cached claude\n" {
		t.Fatalf("expected fake claude output, got %q", got)
	}
}

func TestExtractTTCommands_FencedBlock(t *testing.T) {
	response := "Sure, run these:\n\n```\ntt task add \"Hello\"\ntt task start 1\n```\n"
	cmds := ExtractTTCommands(response)
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}
	if cmds[0] != "tt task add \"Hello\"" {
		t.Errorf("unexpected first command: %q", cmds[0])
	}
	if cmds[1] != "tt task start 1" {
		t.Errorf("unexpected second command: %q", cmds[1])
	}
}

func TestExtractTTCommands_StandaloneLine(t *testing.T) {
	response := "Run: tt task complete 5\nMore text"
	cmds := ExtractTTCommands(response)
	if len(cmds) != 0 {
		t.Errorf("expected commands prefixed with 'tt ' at the start of a line to be matched, got %v", cmds)
	}
	response = "tt task complete 5"
	cmds = ExtractTTCommands(response)
	if len(cmds) != 1 || cmds[0] != "tt task complete 5" {
		t.Errorf("expected one command, got %v", cmds)
	}
}

func TestExtractTTCommands_SkipsPipedLines(t *testing.T) {
	// Standalone lines containing ' | ' (e.g. table rows) should not be
	// treated as commands.
	response := "tt task complete 1 | piped to something"
	cmds := ExtractTTCommands(response)
	if len(cmds) != 0 {
		t.Errorf("expected piped standalone line to be skipped, got %v", cmds)
	}
}

func TestBuildTaskContext_IncludesProjectName(t *testing.T) {
	project := &models.Project{Name: "demo", Tasks: nil}
	out := BuildTaskContext(project)
	if !strings.Contains(out, `"demo"`) {
		t.Errorf("expected project name in context, got %q", out)
	}
	if !strings.Contains(out, "tt task add") {
		t.Error("expected command reference in context")
	}
}

func TestBuildTaskContext_IncludesTaskMetadata(t *testing.T) {
	task, err := models.NewTask("uid", 7, models.TaskTypeBug, "broken")
	if err != nil {
		t.Fatal(err)
	}
	// The section is now derived from the task's canonical state, which is
	// case-sensitive (SectionForState always returns the canonical spelling), so
	// the fixture uses the canonical Next section rather than lowercase "next".
	task.SetSection(models.SectionNext)
	task.Tags = []string{"urgent", "ui"}
	project := &models.Project{Name: "p", Tasks: []*models.Task{task}}
	out := BuildTaskContext(project)
	if !strings.Contains(out, "#7") {
		t.Errorf("expected task short id in context, got %q", out)
	}
	if !strings.Contains(out, "broken") {
		t.Errorf("expected task title in context, got %q", out)
	}
	if !strings.Contains(out, "section: Next") {
		t.Errorf("expected section in context, got %q", out)
	}
	if !strings.Contains(out, "tags: urgent,ui") {
		t.Errorf("expected tags in context, got %q", out)
	}
}
