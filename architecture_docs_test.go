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
		"strict schema-8 Markdown",
		"## Task model",
		"Waiting is an explicit lifecycle state with no derived relationship semantics",
		"Today and Next are mutually exclusive",
		"resulting subtree depth above six",
		"## TUI architecture",
		"The seven modes are All, Today, Inbox, Next, Backlog, Due, and Recent",
		"### One operation grammar",
		"`SimplifiedInputStrategy` is the only Outliner key grammar",
		"the exact five `Space` actions",
		"The checklist is absent because its rows own the whole mutation set",
		"It carries no checklist line",
		"The Outliner remains full-width at every supported terminal size",
		"`Enter` toggles a session-only, project-qualified Peek below the cursor task",
		"Entering visual selection disables Peek",
		"`v` toggles the cursor task, so a selection can name rows that are not adjacent",
		"`V` starts a fresh inclusive task-row range",
		"`Ctrl+A` replaces the selection with every visible task row",
		"Only a committed Delete clears selection",
		"The workspace-wide Notes manager carries an explicit `NoteLocation` plus filename identity with every row",
		"opens in All Notes unless the Outliner cursor names one active project header, which opens that project's scope",
		"`Runtime.AvailableProjects()`; archived projects are not owners, but a parked project remains one",
		"### Project tags",
		"`models.ParkedProjectTag` (`parked`) is the sole behavioral project tag",
		"`Runtime.TaskWorkspaceProjects()` returns non-archived projects that are not parked",
		"the uppercase `F` facets — `Ft` project tags, `Fp`/`FP` parked, `Fa`/`FA` archived, `Fx` clear — decide which projects the snapshot contains at all",
		"Each axis is exclude, include, or only, the lowercase key owns include and the uppercase key owns only",
		"`Space` on a tag row opens a sheet closed over Rename everywhere, Remove everywhere, Park all members, and Unpark all members",
		"filtered at read time instead of being cleared",
		"`v` opens the searchable scope picker",
		"`s` edits the accepted search",
		"`m` moves one parsed note",
		"Malformed notes remain available for raw editing and deletion",
		"Note tags, task links, pins, bulk actions, previews, and archived owners remain outside the product",
		"## Persistence layout",
		"workspace.json",
		"planning.json",
		"## Transactions and recovery",
		"at most 50 transactions",
		"## Concurrent edits and conflicts",
		"## Snapshot-first schema migration",
		"pre-simplification-v5-<UTC>.tar.gz",
		"pre-consolidation-v6-<UTC>.tar.gz",
		"pre-scope-v7-<UTC>.tar.gz",
		"pre-tag-v8-<UTC>.tar.gz",
		"pre-state-clock-v9-<UTC>.tar.gz",
		"backfilling each task's `state_changed` clock from its `updated` timestamp",
		"folds the two retired per-project grouping fields",
		"restore-and-reactivate path",
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
		"focused project's notes",
		"Global notes plus the focused project",
		"strict schema-5 Markdown",
		"protected Inbox ID",
		"### Project boundaries",
		"Boundary Manager",
		"BoundaryID",
	} {
		if strings.Contains(content, stale) {
			t.Errorf("architecture documentation contains retired runtime guidance %q", stale)
		}
	}
}

func TestArchitectureContributorGuidanceMatchesFinalGrammar(t *testing.T) {
	content := normalizedDocumentation(t, "CLAUDE.md")
	for _, required := range []string{
		"eight-state, seven-mode, Outliner-only product",
		"`g` is a chord prefix, not a direct key: cursor-top is `gg` or `Home`",
		"Recent is the activity-ordered mode",
		"The old standalone `b`, `d`, and `t` actions and comma state prefix must remain unbound",
		"`n`/`N` step the cursor between grouping headers, and their stop set is `isHeaderRow()`",
		"The walk wraps at either end",
		"The `Space` menu on a task row is exactly",
		"A checklist row owns six operations",
		"one predicate, `operationRefusedOnChecklistRow`, read in exactly two places",
		"!1-3 #tag @date $destination %project >parent",
		"Every JSON file in `docs/tui-testing/scenarios/` is a committed final-product contract",
		"focused snapshot-first migration and interrupted activation recovery tests",
		"`Ctrl+N` opens All Notes across Global and every project returned by `Runtime.AvailableProjects()`, except on a project header row, which opens that project's scope",
		"`v` chooses All, Global, or one active project",
		"`s` persistently searches titles and bodies",
		"`m` atomically moves one note between active owners",
		"Task workflow state, project tags, and project archive lifecycle are three independent axes",
		"`Runtime.TaskWorkspaceProjects()` is non-archived and not parked, and is the only population the base Outliner reads",
		"Project filter chords are `Ft`, `Fp`, `FP`, `Fa`, `FA`, and `Fx`",
		"The parked and archived populations are axes, not switches",
		"`Fp`, `FP`, and the `parked` row inside `Ft` write the same `ProjectFilterState.ParkedMode` field",
		"Project grouping is universal, not per-mode",
		"Emit those headers through the shared `appendProjectGroupHeader` kernel; do not add a per-mode copy",
		"The sort axis is the `o` chord: `od` due date, `op` priority, `oc` created, `ou` updated, `ot` title, and `ox` clear",
		"Repeating a field key reverses that field",
		"Manual order is never touched",
		"Group order never changes",
	} {
		if !strings.Contains(content, strings.Join(strings.Fields(required), " ")) {
			t.Errorf("CLAUDE.md missing %q", required)
		}
	}
}
