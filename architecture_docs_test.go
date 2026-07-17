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
	content := string(data)
	required := []string{
		"## Workflow state model",
		"Dependency blocking is derived",
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
	content := string(data)
	for _, needle := range []string{
		"local `~/.tt/` workspace by default",
		"iCloud only after explicit selection",
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
