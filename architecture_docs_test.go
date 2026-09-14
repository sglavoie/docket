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

// architectureDocumentChapters is the architecture corpus in reading order: the
// index page first, then every chapter it links. The order is declared here
// rather than derived from the directory listing so a chapter added, removed, or
// resequenced is a reviewable change rather than a silent one.
var architectureDocumentChapters = []string{
	"docs/architecture.md",
	"docs/architecture/runtime-and-task-model.md",
	"docs/architecture/canonical-query.md",
	"docs/architecture/tui-architecture.md",
	"docs/architecture/presentation-chrome.md",
	"docs/architecture/row-indicators.md",
	"docs/architecture/operation-grammar.md",
	"docs/architecture/capture-and-outline-editor.md",
	"docs/architecture/search-filters-and-sorting.md",
	"docs/architecture/named-lenses.md",
	"docs/architecture/selection-and-project-tags.md",
	"docs/architecture/persistence-and-transactions.md",
	"docs/architecture/migration.md",
	"docs/architecture/workspace-and-maintenance.md",
}

// normalizedArchitectureDocumentation joins the whole architecture corpus so a
// contract assertion names the mechanism, not the file the chapter happens to
// live in.
func normalizedArchitectureDocumentation(t *testing.T) string {
	t.Helper()
	chapters := make([]string, 0, len(architectureDocumentChapters))
	for _, path := range architectureDocumentChapters {
		chapters = append(chapters, normalizedDocumentation(t, path))
	}
	return strings.Join(chapters, " ")
}

func TestArchitectureDocumentationCoversFinalRuntimeContracts(t *testing.T) {
	content := normalizedArchitectureDocumentation(t)
	required := []string{
		"## Contents",
		"## Runtime layers",
		"strict schema-17 Markdown",
		"Schema 17 adds an optional `PROJECT-COMMENTS` block between the schema marker and the " +
			"first task heading",
		"`storage.WriteProjectMarkdown` renders a complete `*models.Project` and is the only " +
			"production rewrite entry point",
		"`mergeProjectComments` walks the union of the comment ids across the three sides",
		"`CreatedAt` is treated as identity metadata rather than content",
		"The id lives in a field of its own rather than in `TaskTitle`",
		"## Task model",
		"| Lifecycle | created, updated, `state_changed`, completed timestamps |",
		"deferral count (`defer_count`)",
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
		"`n`/`N` step the cursor between grouping headers, and their stop set is `IsHeaderRow()`",
		"There are three fold axes and therefore three bulk keys: `s` for section headers, " +
			"`p` for project group headers, `t` for task subtrees",
		"if any member of the set is folded the press expands them all, otherwise it folds them all",
		"they separate by scanning row kind, `IsSectionHeader` against `IsProjectHeader`, " +
			"rather than by matching a key prefix",
		"`t` owns `ExpandedNodes` alone, and its walk covers every loaded project",
		"`s` refuses under a section zoom and `p` does not, which is the one place the family diverges",
		"The walk wraps at either end",
		"The checklist operation catalog holds eleven operations",
		"Ten of them answer a key \u2014 `x`, `e`, `J`, `K`, `Ctrl+J`, `Ctrl+K`, `c`, `S`, `S`, and " +
			"`Space`. Some of those reuse the key their task-row counterpart holds",
		"one predicate, `operationRefusedOnChecklistRow`, read by `OperationDescriptor.Applicable`",
		"Recent is the activity review projection",
		"Stale is an explicit cleanup filter and lens, with no automatic lane",
		"The lowercase `fs` facet applies that same predicate as a row refinement except in Recent, where it " +
			"is unavailable",
		"Entering Recent with it active clears only Stale and reports " +
			"`Stale filter cleared: Recent only includes the last 30 days`",
		"The `fs` facet never reloads the project population",
		"`Ctrl+L` is the one key that reaches the newest task from anywhere",
		"#### Row indicators",
		"#### Status flash",
		"### One operation grammar",
		"Five surfaces hold entities, so five `InputStrategy` values exist, and `surfaceOperationGates()` is " +
			"the whole inventory: there is no sixth grammar and no unlisted one",
		"| `SimplifiedInputStrategy` | `taskOperationManifest` + `taskDirectOperationManifest` + " +
			"`outlinerProjectOperationManifest` | Set Check-In and Delete on a task row; " +
			"Edit, Archive, Delete on a project header |",
		"A bulk editor starts blank and requires either a date or explicit Clear",
		"| `ProjectHubInputStrategy` | `ProjectOperationManifest` | Archive or Unarchive, bulk tags |",
		"| `NotesInputStrategy` | `NotesOperationManifest` | Delete |",
		"| `AttachmentsInputStrategy` | `attachmentOperationManifest` | three origin acts, Unlink, Delete |",
		"| `CommentsInputStrategy` | `commentOperationManifest` | Delete |",
		"`DirectOnly` is the field that splits it",
		"What the surface is for stays direct; what is destructive or rare goes behind `Space`",
		"The Outliner is the one surface whose strategy reads more than one manifest, because its rows " +
			"name more than one kind of entity",
		"`validateOutlinerOperationOnePath` therefore reads the manifests concatenated, and each sheet " +
			"projects only its own",
		"an Outliner project header is drawn from `TaskWorkspaceProjects()`, which is non-archived by " +
			"definition, so applicability could never admit the row",
		"Footers do not follow the strategy everywhere, and the difference is deliberate",
		"Only the gated keymap beside those strategies can say that, so it is their single footer source " +
			"and their bindings carry no ranks or labels at all",
		"A footer has one source per surface, never two ranked descriptions that can drift apart",
		"`registerSurfaceInputStrategies` walks `surfaceOperationGates()` and runs each surface's check " +
			"before registering its strategy",
		"The five named wrappers each delegate to one `operation.ValidateOperationOnePath`",
		"`SimplifiedInputStrategy` is the only Outliner key grammar",
		"the one `Space` action",
		"`taskDirectOperationManifest` holds Comments and Attachments as `DirectOnly` entries instead, " +
			"bound to `;` and `&`",
		"The checklist is absent because its rows own the whole mutation set",
		"`AttachmentsInputStrategy` is the third peer grammar and splits the same way",
		"hosted as `ContextActionHostAttachments`",
		"The retirement that matters most is `u`, which meant unlink here while meaning undo everywhere else",
		"`validateAttachmentOperationOnePath` holds them to the same direct-or-menu rule",
		"`CommentsInputStrategy` is the fourth peer grammar and splits the same way",
		"holds Delete alone behind `Space`, hosted as `ContextActionHostComments`",
		"`validateCommentOperationOnePath` holds them to the same direct-or-menu rule",
		"Pin's key is its sigil",
		"`ValueProviderDate` is the one date picker a task operation reaches",
		"which is why `instantEntry()` cannot be a `ValueProviderID` method",
		"`deferRefusalReason` on its defer rows only",
		"Optional check-in setup follows committed `TaskStateEffect` evidence",
		// The vocabulary rename is a third nested grammar on the value picker and
		// the one place a picker's key commits a workspace transaction, so the
		// split that keeps it safe — steps on the picker, effects on the shell —
		// is the mechanism a contributor has to be able to read.
		"`vocabularyRenameProvider` is the whole set",
		"The picker owns the steps and none of the effects",
		"`vocabularyRenameAxis` is the one place a provider is bound to the application namespace",
		"releases it on every exit from the confirmation, accepted or not",
		"reports a clause when it would admit exactly one of the two names",
		"### Capture and inline-edit grammar",
		"`##context` is the singleton task-context sigil",
		"`*pin`'s value set is closed to `pin` and `unpin`",
		"That is the tag rejection behavior rather than priority's",
		"deleting that token is what unpins it",
		"`<child` is the one repeatable structural sigil",
		"Adoption is additive and reaches only the effective project",
		"which is what makes `<0` an ordinary unresolvable reference rather than a detach",
		"`~` is the fourth of the set and the only one that preselects",
		"both halves of the diff resolve against one snapshot and land in one transaction " +
			"with one undo entry",
		"A task Peek names the\nchecklist as a count and never as its items",
		"Nothing sits outside that window",
		"The dropped tail is never reassembled from a head and a tail that\ntwo different measurements agreed on",
		"`inlinePeekDocument` is that split",
		"the height the viewport reserves and the height the renderer draws are the same number",
		"The Outliner remains full-width at every supported terminal size",
		"The task context badge is reserved beside the ancestor and pin markers",
		"`taskPinnedMarker` renders a bold `*` when `task.Pinned && !row.IsPinnedSection`",
		"`zt`, `zz`, and `zb` move the window under the cursor and leave the cursor alone",
		"The `z` prefix is scroll alone: the bulk fold that once shared it moved to `t`",
		"`Enter` pushes a project-qualified Focus frame",
		"`Esc` restores one frame at a time",
		"`Shift+Enter` widens by exactly one level",
		"`b` toggles a session-only, project-qualified Peek below the cursor row",
		"`Ctrl+D`/`Ctrl+U` travel `inlinePeekHalfPageRows`\n(2) and `Ctrl+F`/`Ctrl+B` travel `inlinePeekFullPageRows` (3)",
		"Entering visual selection disables Peek",
		"### Sorting axis",
		"The `o` chord is the third axis beside `f` and `F`",
		"Repeating a field key reverses that field",
		"Manual order is never touched",
		"Group order never changes",
		"### Named lenses",
		"persisted as `config.NamedLens` in the portable `lenses.json` domain, while the theme, the\n" +
			"menu bar's visibility, and the active-lens pointer stay machine-local",
		"The\nmenu-bar flag carries `omitempty` and `NewMachineState` never seeds it, so the\nomitted key " +
			"is the default",
		"`Manager.AdoptPortableLenses` is the one-time-per-machine step",
		"`MergeNamedLenses` never deletes",
		"`CaptureLensState` reads the query axis through\n`capturedLensQuery`",
		"its binding carries `DelegateTransaction`",
		"`OperationHistory` drops the entry, so `u`\nnever takes a lens back",
		"a TUI effect is published only after\nthe commit returns nil",
		"before it writes a single axis",
		"`HandleLensProjectsLoaded` publishes only when its\nown generation comes back",
		"Scope is the one axis\na lens may decline to own",
		"A lens that owns a scope therefore installs it *before* the load decision",
		"`LensModified` carves\nscope out of \"navigating\"",
		"`SetPersistActiveLens`",
		"`activeLensFrame` is the one live lens; there is never a stack",
		"switching lenses\nreplaces the definition while keeping the baseline",
		"Under an active lens `/` calls `beginLensQueryEdit`",
		"bound directly in `SimplifiedInputStrategy` with no\n`FooterRank`",
		"`\\` stays a literal character everywhere text is being typed",
		"`ResetToGlobalAll` calls\n`ClearActiveLensWithoutRestore`",
		"`NewAppModel` reads `Machine().ActiveNamedLens()` into\n`lenses.pendingLaunch` before any message is dispatched",
		"drained in\nthe `CrossProjectsLoadedMsg` arm",
		"`restorePendingLaunchLens`\ntakes the value exactly once",
		"`ActiveNamedLens()` validates the record it finds for shape alone and never\nreferentially",
		"`v` toggles the cursor task, so a selection can name rows that are not adjacent",
		"`V` starts a fresh inclusive task-row range",
		"`Ctrl+A` replaces the selection with every visible task row",
		"Only a committed Delete clears selection",
		"`P` opens Select Project on a live cursor",
		"Administering a project is a direct key rather than a menu row",
		"`ProjectHubInputStrategy` is that grammar, and it is a peer of `SimplifiedInputStrategy` rather than a copy",
		"a surface earns its own vocabulary by declaring bindings, not by adding a key router",
		"`ShouldCaptureEsc` names the unwind rungs in order — the action sheet, the query, then the selection",
		"`v` toggles the row under the cursor, `Ctrl+A` selects every match",
		"The workspace-wide Notes manager carries an explicit `NoteLocation` plus filename identity with every row",
		"opens in All Notes unless the Outliner cursor names one active project header, which opens that project's scope",
		"`Runtime.AvailableProjects()`; archived projects are not owners, but a parked project remains one",
		"### Project tags",
		"`models.ParkedProjectTag` (`parked`) is the sole behavioral project tag",
		"`Runtime.TaskWorkspaceProjects()` returns non-archived projects that are not parked",
		"the uppercase `F` facets — `F#` project tags, `Fp`/`FP` parked, `Fa`/`FA` archived, `Fx` clear — decide which projects the snapshot contains at all",
		"Each axis is exclude, include, or only, the lowercase key owns include and the uppercase key owns only",
		"`Fp`, `FP`, and the `parked` row inside the `F#` picker write the same `ParkedMode` field",
		"`Space` on a tag row opens a sheet closed over Rename everywhere, Remove everywhere, Park all members, and Unpark all members",
		"filtered at read time instead of being cleared",
		"`p` opens the searchable scope picker",
		"`/` edits the accepted search",
		"`%` moves one parsed note",
		"hosted as `ContextActionHostNotes` and holding Delete alone",
		"delegates its confirmation back to the manager that owns the row, which is why Notes gains a sheet without gaining a selection",
		"`ValidateNotesOperationOnePath` holds them to the same direct-or-menu rule",
		"Malformed notes remain available for raw editing and deletion",
		"Note tags, task links, pins, bulk actions, and archived owners remain outside the product",
		"`renderNotesContent` asks `ListSurfacePreviewGeometry` for the split",
		"the list renderer is handed the resolved pane width instead of reading the terminal's",
		"keyed on note identity, the Notes scope, the active query with the case mode that produced the visible rows, the revision, and that width together",
		"while the whole key holds, the offset is the reader's and ordinary re-renders leave it alone",
		"`notesPreviewAnchor` decides where a rebuilt cache opens",
		"only if no term is found that way does a per-line fuzzy pass run",
		"they are claimed ahead of the search text input while a query is being edited",
		"The render owns the tail clamp",
		"`ListPreviewScrollWindow` converges on a window that reserves a row for each " +
			"`RenderListContinuation` it needs",
		"consecutive pages overlap by one line rather than drifting apart",
		"renders that reason as a warning and never as Markdown",
		"paging is gated on `notesPreviewPageable` rather than on `notesPreviewVisible`",
		"## Persistence layout",
		"workspace.json",
		"planning.json",
		"## Transactions and recovery",
		"at most 50 transactions",
		"A delete op is `os.Remove`: one entry, and never a non-empty directory",
		"`PlanProjectDelete` therefore enumerates the project tree and emits one op per path in reverse " +
			"`WalkDir` order, which is post-order",
		"a project can never be absent from disk while still named in either file",
		"restore holds one advisory lock per outermost directory rather than one per image, or creating " +
			"the nested lock's parent would recreate the directory preflight is about to find absent",
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
		"pre-defer-count-v13-<UTC>.tar.gz",
		"pre-due-time-v14-<UTC>.tar.gz",
		"Repeated deferral starts at `DeferCount >= 3`",
		"Task.ApplyDeferAt",
		"backfilling each task's `state_changed` clock from its `updated` timestamp",
		"renaming each task's durable deferral metadata key from `snoozed` to `deferred`",
		"rewriting each task's durable deferral value from a `YYYY-MM-DD` calendar day",
		"folds the two retired per-project grouping fields",
		"restore-and-reactivate path",
		"prepared activation journal",
		"## Workspace location and cloud consent",
		"## Maintenance CLI and platform support",
		"There are no release artifacts",
		"developed and used daily on macOS",
		"Windows is not supported",
		"`resolveBuildInfo` reads `debug.ReadBuildInfo` when the compiled-in version is still " +
			"the `dev` placeholder",
	}
	for _, needle := range required {
		if !strings.Contains(content, strings.Join(strings.Fields(needle), " ")) {
			t.Errorf("architecture documentation missing %q", needle)
		}
	}
}

func TestArchitectureDocumentationRejectsRetiredLiveProduct(t *testing.T) {
	content := normalizedArchitectureDocumentation(t)
	for _, stale := range []string{
		"Review mode",
		"Finder",
		"Command Palette",
		"Task Type",
		"Inspector",
		"live schema 4",
		"Current release builds target",
		"not a supported release target",
		"task mutation CLI",
		"Ctrl+Esc reset",
		"focused project's notes",
		"Global notes plus the focused project",
		"strict schema-5 Markdown",
		"protected Inbox ID",
		"Select Project with search focused",
		"`a`/`A` select or clear every match",
		"### Project boundaries",
		"Boundary Manager",
		"BoundaryID",
		"`zM` and `zR` share that prefix",
		"`S` keeps its own narrower `visibleSectionHeaders()` scan",
		"the screen-relative jumps",
	} {
		if strings.Contains(content, stale) {
			t.Errorf("architecture documentation contains retired runtime guidance %q", stale)
		}
	}
}
