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
		"strict schema-13 Markdown",
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
		"`n`/`N` step the cursor between grouping headers, and their stop set is `isHeaderRow()`",
		"There are three fold axes and therefore three bulk keys: `s` for section headers, " +
			"`p` for project group headers, `t` for task subtrees",
		"if any member of the set is folded the press expands them all, otherwise it folds them all",
		"they separate by scanning row kind, `IsSectionHeader` against `IsProjectHeader`, " +
			"rather than by matching a key prefix",
		"`t` owns `expandedNodes` alone, and its walk covers every loaded project",
		"`s` refuses under a section zoom and `p` does not, which is the one place the family diverges",
		"The walk wraps at either end",
		"A checklist row owns nine operations",
		"Eight of them reuse the key their task-row counterpart holds",
		"one predicate, `operationRefusedOnChecklistRow`, read by `OperationDescriptor.Applicable`",
		"Recent is the activity review projection",
		"Stale draws immediately after Review",
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
			"`outlinerProjectOperationManifest` | Delete alone on a task row; " +
			"Edit, Archive, Delete on a project header |",
		"| `ProjectHubInputStrategy` | `projectOperationManifest` | Archive or Unarchive, bulk tags |",
		"| `NotesInputStrategy` | `notesOperationManifest` | Delete |",
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
		"The five named wrappers each delegate to one `validateOperationOnePath`",
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
		"That request names `ValueProviderDueDate` rather than the combined list `@` opens",
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
		"The task context badge is reserved beside the ancestor and pin markers",
		"`taskPinnedMarker` renders a bold `*` when `task.Pinned && !row.IsPinnedSection`",
		"`H`, `M`, and `L` are the screen-relative jumps",
		"`zt`, `zz`, and `zb` are the inverse family",
		"The `z` prefix is scroll alone: the bulk fold that once shared it moved to `t`",
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
		"### Named lenses",
		"persisted as `config.NamedLens` in the\nmachine domain beside the theme",
		"`captureLensState` reads the query axis through\n`capturedLensQuery`",
		"its binding carries `DelegateTransaction`",
		"`OperationHistory` drops the entry, so `u`\nnever takes a lens back",
		"a TUI effect is published only after\nthe commit returns nil",
		"before it writes a single axis",
		"`handleLensProjectsLoaded` publishes only when its\nown generation comes back",
		"A lens therefore never writes scope",
		"`activeLensFrame` is the one live lens; there is never a stack",
		"switching lenses\nreplaces the definition while keeping the baseline",
		"Under an active lens `/` calls `beginLensQueryEdit`",
		"bound directly in `SimplifiedInputStrategy` with no\n`FooterRank`",
		"`\\` stays a literal character everywhere text is being typed",
		"`resetToGlobalAll` calls\n`clearActiveLensWithoutRestore`",
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
		"`validateNotesOperationOnePath` holds them to the same direct-or-menu rule",
		"Malformed notes remain available for raw editing and deletion",
		"Note tags, task links, pins, bulk actions, and archived owners remain outside the product",
		"`renderNotesContent` asks `listSurfacePreviewGeometry` for the split",
		"the list renderer is handed the resolved pane width instead of reading the terminal's",
		"keyed on note identity, the Notes scope, the active query with the case mode that produced the visible rows, the revision, and that width together",
		"while the whole key holds, the offset is the reader's and ordinary re-renders leave it alone",
		"`notesPreviewAnchor` decides where a rebuilt cache opens",
		"only if no term is found that way does a per-line fuzzy pass run",
		"they are claimed ahead of the search text input while a query is being edited",
		"The render owns the tail clamp",
		"`listPreviewScrollWindow` converges on a window that reserves a row for each " +
			"`renderListContinuation` it needs",
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
		"Select Project with search focused",
		"`a`/`A` select or clear every match",
		"### Project boundaries",
		"Boundary Manager",
		"BoundaryID",
		"`zM` and `zR` share that prefix",
		"`S` keeps its own narrower `visibleSectionHeaders()` scan",
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
	// The budget polices prose, not rule count: it is what forces mechanism into
	// docs/architecture.md and behavior into README.md. It moves only when a
	// feature adds invariants that cannot live anywhere else — the Outliner
	// project sheet, the enumerated project delete, the restore lock scope, the
	// cursor row's gutter/content seam, Select Project's preview pane owning a
	// second scroll, the parse-order/teach-order registry split that the
	// attachment sigil forced, the paired-delimiter comment sigil that leads
	// parse order, and the Notes preview pane — a second read-only scroll whose
	// cache key and anchor rule nothing else states — are the eight that raised
	// it from 16200 — and never to absorb explanation.
	const budget = 19100
	if len(data) > budget {
		t.Errorf("CLAUDE.md is %d bytes, over the %d-byte rules-only budget; "+
			"move explanatory prose to README.md or docs/architecture.md", len(data), budget)
	}
}

func TestArchitectureContributorGuidanceMatchesFinalGrammar(t *testing.T) {
	content := normalizedDocumentation(t, "CLAUDE.md")
	for _, required := range []string{
		"Contributor rules only: invariants, forbidden patterns, forwarding addresses",
		"[`docs/decisions/behavioral-signals-contracts.md`](docs/decisions/behavioral-signals-contracts.md) " +
			"is the locked behavioral-signal authority",
		"The lifecycle states are:",
		"eight-state, seven-mode, Outliner-only product",
		"Do not recreate cross-project behavior with independent `SaveProject` calls",
		"`StateChangedAt` is the state clock and `UpdatedAt` is the touch time",
		"A task reaching `WriteV8ProjectDocument` without a clock is rejected, never backfilled",
		"do not add a parallel key router, palette, or view-specific copy",
		"An entity operation is direct-only or menu-only, never both, on every surface, " +
			"enforced by `validateOperationOnePath` over that surface's manifest",
		"The task-row `Space` menu is Delete alone, identical to the bulk menu: Delete has no " +
			"direct key and confirms first",
		"The Outliner project-header sheet is Edit, Archive, Delete, and never Unarchive: " +
			"that header is drawn from the active population",
		"`*`, `#`, and `Space` are the shared keys, the cursor deciding: task or project pin, " +
			"task or project tags, and checklist, project, or task actions",
		"`OpDelete` is `os.Remove`, so a project delete enumerates its tree post-order and commits it " +
			"with the catalog and planning rewrites",
		"It confirms, names the cascade, and refuses the system Unfiled project",
		"Restore locks outermost directories only; a nested lock recreates the directory the entry removed",
		"One key is one act on every entity surface",
		"| `Space` | menu: destructive and rare operations only |",
		"Transient input overlays are exempt: composer, project editor/creator, text editor, " +
			"operation text prompt, tag/date add modes, list picker adapter, and the outline error and help viewers",
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
		"the relation marker, the task context segment, and the pin marker stay out of it",
		"`taskRowSegments` reserves all three at every width",
		"`taskRelationMarker` is one segment both ways",
		"The four-step pulse ladder drops done, inbox, overdue, then today",
		"pace outranks the workspace pulse: exhaust that ladder first, then shorten and remove pace before " +
			"truncating required interaction context",
		"All's Stale lane follows Review; below Pinned/Today/Next/Review/Doing",
		"`fs` is unavailable in Recent; entry clears only Stale: " +
			"`Stale filter cleared: Recent only includes the last 30 days`",
		"There is no `$delete` sigil",
		"The old `b`, `c`, `d`, `m`, `w`, `:`, `C`, `R`, `Ctrl+Esc`, and the " +
			"comma state prefix stay unbound, as does `SortByStatus`",
		"Done is `$d` alone and no `d` alias may return",
		"Root `$`, `z`, and `g` are chord prefixes, not direct keys: `z` never means snooze or defer, " +
			"and cursor-top is `gg` or `Home`",
		"`s`, `p`, and `t` fold the section, project, and task axes; `P` is Select Project",
		"`>` and `<` are Set Parent and Adopt Children; indent and outdent are `]` and `[`, " +
			"which own no sigil",
		"`@` is the one date picker over both fields, so the cursor's defer section, " +
			"not a provider method, lets `+Nh` parse",
		"Task workflow state, project tags, and project archive lifecycle are three independent axes",
		"`parked` is the one behavioral project tag",
		"Keep the two populations named apart: `Runtime.TaskWorkspaceProjects()` is non-archived and not " +
			"parked and is the only one the base Outliner reads",
		"!1-3 #tag ##context @date @@defer $destination *pin %project >parent <child &link ;comment;",
		"`registry` is parse order and `guideOrder` teach order",
		"Due's last alternative is `[^\\s@]\\S*` so `@` can never eat `@@`",
		"`*unpin` belongs to edit and search",
		"Slash search is the fourth surface of the same registry, not a parallel query language",
		"`FieldChild` declares no `Clears`, which makes `<0` an ordinary unresolvable reference " +
			"rather than a detach and the `<` picker additive: unchecking never detaches",
		"Review is not a lifecycle state and has no key of its own",
		"`##0` clears it and `@@0` a deferral on the edit surfaces",
		"Children resolve only in the effective project",
		"When changing the grammar, update parser definitions, completion, syntax guide, scenarios, and parity tests together",
		"v4→v5→v6→v7→v8→v9→v10→v11→v12→v13",
		"v4/v5/v6/v11 decoders are migration-only",
		"Every JSON file in `docs/tui-testing/scenarios/` is a committed final-product contract",
		"focused snapshot-first migration and interrupted activation recovery tests",
		"App timers go through `tickCmd`, never `tea.Tick`",
		"## Where the details live",
	} {
		if !strings.Contains(content, strings.Join(strings.Fields(required), " ")) {
			t.Errorf("CLAUDE.md missing %q", required)
		}
	}
}
