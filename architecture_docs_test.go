package main

import (
	"os"
	"strings"
	"testing"
)

func normalizedDocumentation(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return strings.Join(strings.Fields(string(data)), " ")
}

func TestArchitectureDocumentationCoversFinalRuntimeContracts(t *testing.T) {
	content := normalizedDocumentation(t, "docs/architecture.md")
	required := []string{
		"## Runtime layers",
		"strict schema-5 Markdown",
		"## Task model",
		"Blocked is derived from unresolved dependencies",
		"Today and Next are mutually exclusive",
		"resulting subtree depth above six",
		"## TUI architecture",
		"The five modes are All, Inbox, Next, Backlog, and Due",
		"### One operation grammar",
		"`SimplifiedInputStrategy` is the only Outliner key grammar",
		"the exact nine `Space` actions",
		"The Outliner remains full-width at every supported terminal size",
		"`Enter` toggles a session-only, project-qualified Peek below the cursor task",
		"Entering visual selection disables Peek",
		"The bulk `Space` menu contains Delete only",
		"The combined Notes tool carries an explicit location with every row",
		"## Persistence layout",
		"workspace.json",
		"planning.json",
		"## Transactions and recovery",
		"at most 50 transactions",
		"## Concurrent edits and conflicts",
		"## Snapshot-first v5 migration",
		"pre-simplification-v5-<UTC>.tar.gz",
		"prepared activation journal",
		"## Workspace location and cloud consent",
		"## Maintenance CLI and platform support",
		"macOS and Linux",
	}
	for _, needle := range required {
		if !strings.Contains(content, strings.Join(strings.Fields(needle), " ")) {
			t.Errorf("architecture documentation missing %q", needle)
		}
	}
}

func TestArchitectureDocumentationRejectsRetiredLiveProduct(t *testing.T) {
	content := normalizedDocumentation(t, "docs/architecture.md")
	for _, stale := range []string{
		"Review mode",
		"Finder",
		"Command Palette",
		"Task Type",
		"Inspector",
		"live schema 4",
		"task mutation CLI",
		"Ctrl+Esc reset",
	} {
		if strings.Contains(content, stale) {
			t.Errorf("architecture documentation contains retired runtime guidance %q", stale)
		}
	}
}

func TestArchitectureContributorGuidanceMatchesFinalGrammar(t *testing.T) {
	content := normalizedDocumentation(t, "CLAUDE.md")
	for _, required := range []string{
		"nine-state, five-mode, Outliner-only product",
		"keys `m`, `:`, `R`, `!`, and `Ctrl+Esc` must remain unbound",
		"The `Space` menu is exactly",
		"!1-5 #tag @date ~interval $state %project >parent",
		"Every JSON file in `docs/tui-testing/scenarios/` is a committed final-product contract",
		"focused snapshot-first migration and interrupted activation recovery tests",
	} {
		if !strings.Contains(content, strings.Join(strings.Fields(required), " ")) {
			t.Errorf("CLAUDE.md missing %q", required)
		}
	}
}
