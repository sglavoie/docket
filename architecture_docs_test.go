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
		"## Contents",
		"## Runtime layers",
		"strict schema-11 Markdown",
		"## Task model",
		"| Lifecycle | created, updated, `state_changed`, completed timestamps |",
		"`StateChangedAt` (`state_changed`) is the state clock and `UpdatedAt` is the touch time",
		"a task reaching `WriteV8ProjectDocument` without a clock is rejected, never silently backfilled",
		"Waiting is an explicit lifecycle state with no derived relationship semantics",
		"Today and Next are mutually exclusive",
		"resulting subtree depth above six",
		"## TUI architecture",
		"The seven modes are All, Today, Inbox, Next, Backlog, Due, and Recent",
		"`g` is a chord prefix, not a direct key: cursor-top is `gg` or `Home`",
		"Project grouping is universal, not per-mode",
		"through the shared `appendProjectGroupHeader` kernel",
		"`n`/`N` step the cursor between grouping headers, and their stop set is `isHeaderRow()`",
		"The walk wraps at either end",
		"A checklist row owns six operations",
		"All six reuse the key their task-row counterpart holds",
		"one predicate, `operationRefusedOnChecklistRow`, read by `OperationDescriptor.Applicable`",
		"Recent is the activity review projection",
		"`Ctrl+L` is the one key that reaches the newest task from anywhere",
		"#### Row indicators",
		"#### Status flash",
		"### One operation grammar",
		"`SimplifiedInputStrategy` is the only Outliner key grammar",
		"the exact three `Space` actions",
		"The checklist is absent because its rows own the whole mutation set",
		"Pin has two routes",
		"### Capture and inline-edit grammar",
		"`##context` is the singleton task-context sigil",
		"`*pin`'s value set is closed to `pin` and `unpin`",
		"That is the tag rejection behavior rather than priority's",
		"deleting that token is what unpins it",
		"`<child` is the one repeatable structural sigil",
		"Adoption is additive and reaches only the effective project",
		"which is what makes `<0` an ordinary unresolvable reference rather than a detach",
		"It carries no checklist line",
		"`inlinePeekLines` therefore owns a drop order, not a set",
		"The Outliner remains full-width at every supported terminal size",
		"The task context badge is reserved beside the ancestor marker",
		"`H`, `M`, and `L` are the screen-relative jumps",
		"`zt`, `zz`, and `zb` are the inverse family",
		"`Enter` pushes a project-qualified Focus frame",
		"`Esc` restores one frame at a time",
		"`Shift+Enter` widens by exactly one level",
		"`Ctrl+Enter` toggles a session-only, project-qualified Peek below the cursor task",
		"Entering visual selection disables Peek",
		"### Sorting axis",
		"The `o` chord is the third axis beside `f` and `F`",
		"Repeating a field key reverses that field",
		"Manual order is never touched",
		"Group order never changes",
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
		"`Fp`, `FP`, and the `parked` row inside the `Ft` picker write the same `ParkedMode` field",
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
		"pre-defer-rename-v10-<UTC>.tar.gz",
		"pre-defer-instant-v11-<UTC>.tar.gz",
		"pre-context-v12-<UTC>.tar.gz",
		"backfilling each task's `state_changed` clock from its `updated` timestamp",
		"renaming each task's durable deferral metadata key from `snoozed` to `deferred`",
		"rewriting each task's durable deferral value from a `YYYY-MM-DD` calendar day",
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

func TestContributorGuidanceStaysRulesOnly(t *testing.T) {
	data, err := os.ReadFile("CLAUDE.md")
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	const budget = 15200
	if len(data) > budget {
		t.Errorf("CLAUDE.md is %d bytes, over the %d-byte rules-only budget; "+
			"move explanatory prose to README.md or docs/architecture.md", len(data), budget)
	}
}

func TestArchitectureContributorGuidanceMatchesFinalGrammar(t *testing.T) {
	content := normalizedDocumentation(t, "CLAUDE.md")
	for _, required := range []string{
		"Contributor rules only: invariants, forbidden patterns, forwarding addresses",
		"eight-state, seven-mode, Outliner-only product",
		"Do not recreate cross-project behavior with independent `SaveProject` calls",
		"`StateChangedAt` is the state clock and `UpdatedAt` is the touch time",
		"A task reaching `WriteV8ProjectDocument` without a clock is rejected, never backfilled",
		"do not add a parallel key router, palette, or view-specific copy",
		"A task operation is direct-only or menu-only, never both, enforced by `validateTaskOperationOnePath`",
		"The task-row `Space` menu is exactly Comments, Attachments, Delete; the bulk menu is Delete only; " +
			"Delete has no direct key and confirms first",
		"never branch one descriptor's target source on runtime context",
		"Keep `operationRefusedOnChecklistRow` out of the descriptor's `Applicability` field",
		"Emit project group headers through the shared `appendProjectGroupHeader` kernel, never a per-mode copy",
		"`computeOutlinerViewport` is the single window authority",
		"Never recompute the window in a key handler",
		"Manual order is never touched by the sort axis, and that holds by construction rather than by a guard " +
			"— do not add one. Group order never changes",
		"Arming stays in the one deferred closure in `AppModel.Update`",
		"The interactive TUI ignores an inherited `NO_COLOR`",
		"A value provider is exactly one of `textEntry()`",
		"`inlinePeekLines` owns a drop order, not a set",
		"`visualWidthOverflow` must subtract the same ellipsis column as `truncateToVisualWidth`",
		"`indicatorDropLadder` is a list: a new indicator gets a position, not an append",
		"neither the ancestor marker nor the task context segment belongs in it",
		"The pulse ladder in `formatWorkspacePulseComponents` has exactly three steps",
		"There is no `$delete` sigil",
		"The old standalone `b` and `t` actions and the comma state prefix stay unbound, " +
			"along with `c`, `m`, `:`, `C`, `R`, and `Ctrl+Esc`. `SortByStatus` stays unbound",
		"`d` is Set Defer and never Done — Done stays `wd` alone and no `d`-means-done alias may return. " +
			"Root `z` and `g` are chord prefixes, not direct keys: `z` never means snooze or defer, " +
			"and cursor-top is `gg` or `Home`",
		"Task workflow state, project tags, and project archive lifecycle are three independent axes",
		"`parked` is the one behavioral project tag",
		"Keep the two populations named apart: `Runtime.TaskWorkspaceProjects()` is non-archived and not " +
			"parked and is the only one the base Outliner reads",
		"!1-3 #tag ##context @date $destination *pin %project >parent <child",
		"`*unpin` is edit-only",
		"`FieldChild` declares no `Clears`, which makes `<0` an ordinary unresolvable reference rather than a detach",
		"Children resolve only in the effective project",
		"When changing the grammar, update parser definitions, completion, syntax guide, scenarios, and parity tests together",
		"v4→v5→v6→v7→v8→v9→v10",
		"v4/v5/v6 decoders are migration-only",
		"Every JSON file in `docs/tui-testing/scenarios/` is a committed final-product contract",
		"focused snapshot-first migration and interrupted activation recovery tests",
		"## Where the details live",
	} {
		if !strings.Contains(content, strings.Join(strings.Fields(required), " ")) {
			t.Errorf("CLAUDE.md missing %q", required)
		}
	}
}
