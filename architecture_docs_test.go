package main

import (
	"os"
	"strings"
	"testing"
)

func TestArchitectureDocumentationCoversReleaseCandidateContracts(t *testing.T) {
	data, err := os.ReadFile("docs/architecture.md")
	if err != nil {
		t.Fatalf("read architecture documentation: %v", err)
	}
	content := strings.Join(strings.Fields(string(data)), " ")
	required := []string{
		"## Workflow state model",
		"Dependency blocking is derived",
		"`in_progress` identifies work currently underway",
		"Focus Queue is ordering-only",
		"## Durable data layout",
		".tt/transactions/",
		"## Transactions, recovery, and undo",
		"## Concurrent edits and conflicts",
		"opaque prepared action plan",
		"legacy-only aggregate `.tt.json`",
		"canonical lowercase ASCII components",
		"active TUI at either path",
		"## Workspace location and cloud consent",
		"New workspaces start in local storage",
		"## AI privacy and consent",
		"standard input, not process arguments",
		"Full-workspace disclosure requires current provider-specific consent",
		"codex exec --cd <workspace>",
		"Redacted, no-context, and declined",
		"## Upgrade and migration expectations",
		"## Backup and restore",
		"## Platform support",
		"macOS and Linux",
		"## TUI navigation architecture",
		"All, Inbox, Next, Backlog, Due, Priority, Blocked, Parked, Daily",
		"`Tab`/`Shift+Tab` cycle All → Inbox → Next → Backlog",
		"`OperationRegistry` is the common descriptor source",
		"`ListSurface`",
		"restores All over All Projects",
	}
	for _, needle := range required {
		if !strings.Contains(content, needle) {
			t.Errorf("architecture documentation missing %q", needle)
		}
	}
}

func TestREADMEPointsToArchitectureAndLocalFirstStorage(t *testing.T) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	content := strings.Join(strings.Fields(string(data)), " ")
	for _, needle := range []string{
		"local `~/.tt/` workspace by default",
		"iCloud Drive, only after explicit selection",
		"not converted automatically",
		"`--force` skips only the confirmation prompt",
		"docs/architecture.md",
		"just tui-scenarios",
	} {
		if !strings.Contains(content, needle) {
			t.Errorf("README missing %q", needle)
		}
	}
}

func TestCanonicalStateAndPlanningDocumentationStayDistinct(t *testing.T) {
	for _, path := range []string{"README.md", "docs/architecture.md"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := strings.Join(strings.Fields(string(data)), " ")
		for _, want := range []string{
			"Focus Queue",
			"ordering",
			"Today and Next are mutually exclusive",
		} {
			if !strings.Contains(content, want) {
				t.Errorf("%s missing canonical planning distinction %q", path, want)
			}
		}
		for _, stale := range []string{
			"Active section",
			"Next → Backlog → Active → All",
			"command-palette project navigation",
			"`Ctrl+X`",
		} {
			if strings.Contains(content, stale) {
				t.Errorf("%s contains retired runtime guidance %q", path, stale)
			}
		}
	}
}
