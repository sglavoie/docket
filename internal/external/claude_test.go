package external

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

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
	task.Section = "next"
	task.Tags = []string{"urgent", "ui"}
	project := &models.Project{Name: "p", Tasks: []*models.Task{task}}
	out := BuildTaskContext(project)
	if !strings.Contains(out, "#7") {
		t.Errorf("expected task short id in context, got %q", out)
	}
	if !strings.Contains(out, "broken") {
		t.Errorf("expected task title in context, got %q", out)
	}
	if !strings.Contains(out, "section: next") {
		t.Errorf("expected section in context, got %q", out)
	}
	if !strings.Contains(out, "tags: urgent,ui") {
		t.Errorf("expected tags in context, got %q", out)
	}
}
