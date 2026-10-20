package main

import (
	"os"
	"strings"
	"testing"
)

func TestContributorGuidanceStaysRulesOnly(t *testing.T) {
	data, err := os.ReadFile("CLAUDE.md")
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	// The budget polices prose, not rule count. Move mechanism and behavior to
	// their architecture and guide documents instead of raising it for explanation.
	// Historical budget decisions are preserved in
	// docs/decisions/contributor-guidance-budget-{early,middle,recent}.md.
	// Record future increases there with their rationale.
	const budget = 77480
	if len(data) > budget {
		t.Errorf("CLAUDE.md is %d bytes, over the %d-byte rules-only budget; "+
			"move explanatory prose to README.md or docs/architecture.md", len(data), budget)
	}
}

func TestArchitectureContributorGuidanceMatchesFinalGrammar(t *testing.T) {
	content := normalizedDocumentation(t, "CLAUDE.md")
	for _, required := range []string{
		"This file is the invariant contract the implementation and its tests are held to",
		"if you want to know how `tt` behaves, read [`docs/guide/`](docs/guide/README.md) instead",
		"Contributor rules only: invariants, forbidden patterns, forwarding addresses",
		"[`docs/decisions/behavioral-signals-contracts.md`](docs/decisions/behavioral-signals-contracts.md) " +
			"is the locked behavioral-signal authority",
		"The lifecycle states are:",
		"eight-state, seven-mode, Outliner-only product",
		"Do not recreate cross-project behavior with independent `SaveProject` calls",
		"`StateChangedAt` is the state clock and `UpdatedAt` is the touch time",
		"A task reaching `WriteV8ProjectDocument` without a clock is rejected, never backfilled",
		"do not add a parallel key router, palette, or view-specific copy",
		"An entity operation is direct-only or menu-only, never both, in that surface's " +
			"entity action sheet, enforced by `ValidateOperationOnePath` over its manifest",
		"The menu bar is not a sheet but a projection of the registry, the class `?` help and " +
			"the footer ranks belong to, so it names directly-bound operations freely",
		"a menu-only operation's `TargetSource` is target-free or view-scoped, never a task or project",
		"The task-row `Space` menu holds Set Check-In under `Schedule` and Delete, identical to the bulk menu: " +
			"the editor saves with `Ctrl+S`; Delete confirms first",
		"bulk use starts blank and labels the replacement",
		"The Outliner project-header sheet is Archive and Delete, and never Unarchive: " +
			"that header is drawn from the active population",
		"`*`, `#`, `C`, `;`, `e`, and `Space` are the shared keys, the cursor deciding: task or " +
			"project pin, task or project tags, task or project comment, task or project comment panel, " +
			"checklist-item or task or project edit, and checklist, project, or task actions",
		"Edit project, Add project comment, and Project comments narrow to an Outliner project " +
			"header through `outlinerProjectOperationTargetsHeader`",
		"Add Item also answers from the owner task's row, since an empty checklist has no item row",
		"Its reveal rides `TransactionResultMsg.RevealChecklistFor`, so a refused append moves nothing",
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
		"`ComputeOutlinerViewport` is the single window authority",
		"Never recompute the window in a key handler",
		"Stored, planning, and pin order only break ties; Due and Recent break theirs by priority.",
		"Arming stays in the one deferred closure in `AppModel.Update`",
		"The interactive TUI ignores an inherited `NO_COLOR`",
		"A value provider is exactly one of `TextEntry()`",
		"`inlinePeekDocument` separates the whole detail from the window of it on screen",
		"`peekOffsetForRow` is pure and `scrollPeek` the only writer, so no render moves\nthe window",
		"`TruncateToVisualWidth` has no counterpart: its dropped tail is not\n  recoverable",
		"Nothing sits outside that window and the document has no header",
		"Checklist and comments stay counts; their items are rows and their panel",
		"A task row promotes three indicators and no more \u2014 the deadline, an owed\n  review, the reference census \u2014 and `indicatorDropLadder` is that list in\n  reverse",
		"The trailing `~S/N` counts stored locators in `N` regardless of target",
		"amber blocked, green settled, muted unreadable",
		"an addition naming a task inside the selection",
		"A reference target is named nowhere else",
		"`project export` shares that fallback and writes no dependents",
		"`~task` is the one repeatable reference crossing a project boundary",
		"`DependencyPatch`'s zero value is no intent, so a save that never mentions a reference " +
			"preserves the set",
		"Relation, pin, attachment, and description bypass the ladder",
		"`taskRowSegments` reserves them at every width",
		"`taskRowSegments.titlePrefix()` is where the inline row editor gets its indent\n  and its width",
		"`quickEditMandatoryMetadata` serves it first and shrinks the\n  route to `quickEditRouteFloor` beneath it",
		"`taskRelationMarker` is one segment both ways",
		"The four-step pulse ladder drops done, inbox, overdue, then today",
		"pace outranks the workspace pulse: exhaust that ladder first, then shorten and remove pace before " +
			"truncating required interaction context",
		"All has no automatic Stale lane: old work stays in its lifecycle group",
		"The 60-second periodic tick invalidates a projection when its frozen local day is no longer current",
		"`fs` is unavailable in Recent; entry clears only Stale: " +
			"`Stale filter cleared: Recent only includes the last 30 days`",
		"There is no `$delete` sigil",
		"The old `d`, `m`, `$`, `:`, `Ctrl+Esc`, `H`, `L`, and the " +
			"comma state prefix stay unbound, as does `SortByStatus`",
		"`b` took Peek from the retired `Ctrl+Enter`; no alias returns it",
		"`;` opens the panel that reads, edits, and deletes comments and `C` " +
			"only appends one from the list; on a task both commit through `Service.AddComment` " +
			"under the one limit `taskdetail.CommentCharLimit` states",
		"On a project header both keys answer for the project, through a second `CommentOwner` " +
			"over that same panel and never a second panel",
		"A project comment is addressed by its stored id and the text the panel read, never by row",
		"The adapter resolves its project by identity on every reload",
		"That popup is the multiline `TextEntry()` provider, so `Enter` writes a newline, " +
			"`Ctrl+S` saves, `Esc` confirms a discard the prompt draws in its own box, and " +
			"the prompt emits one submission until a refusal restores its text",
		"Done is `wd` alone and no `d` alias may return",
		"Root `w`, `z`, and `g` are chord prefixes, not direct keys: `z` never means snooze or defer, " +
			"and cursor-top is `gg` or `Home`",
		"`w` is the workflow chord and the one keyboard route that does not spell its sigil: " +
			"`$` is typed and pressed nowhere, and `f$` is retired with it",
		"Every other facet carrying a sigil answers to that sigil, so pinned is `f*`, " +
			"references are `f~`, review is `f=`; states follow the chord to `fw`, and the " +
			"letters `fp` and `fr` stay retired",
		"`s`, `p`, and `t` fold the section, project, and task axes; `P` is Select Project",
		"`>` and `<` are Set Parent and Adopt Children on task rows, and move the preview split " +
			"in Select Project; indent and outdent are `]` and `[`, which own no sigil",
		"`@` is the one date picker over Due, Defer, and Check-in; the cursor's Defer section, " +
			"not a provider method, lets `+Nh` parse",
		"`Tab` moves forward through those sections and wraps; `Shift+Tab` moves backward",
		"Task workflow state, project tags, and project archive lifecycle are three independent axes",
		"`parked` is the one behavioral project tag",
		"Keep the two populations named apart: `Runtime.TaskWorkspaceProjects()` is non-archived and not " +
			"parked and is the only one the base Outliner reads",
		"!1-3 #tag ##context @date @@defer $destination *pin =review %project >parent <child ~task &link ;comment;",
		"`registry` is parse order and `guideOrder` teach order",
		"Due's last alternative is `[^\\s@]\\S*` so `@` can never eat `@@`",
		"`*unpin` belongs to edit and search",
		"Slash search is the fourth surface of the same registry, not a parallel query language",
		"`FieldChild` declares no `Clears`, which makes `<0` an ordinary unresolvable reference " +
			"rather than a detach and the `<` picker additive: unchecking never detaches",
		"Review is not a lifecycle state. It is a stored boolean axis beside `Pinned`, " +
			"written by the sigil `=` and narrowed by the facet `f=`",
		"`##0` clears it and `@@0` a deferral on the edit surfaces",
		"Children resolve only in the effective project",
		"When changing the grammar, update parser definitions, completion, syntax guide, scenarios, and parity tests together",
		"v4→v5→v6→v7→v8→v9→v10→v11→v12→v13",
		"v4/v5/v6/v11 decoders are migration-only",
		"Every JSON file in `docs/tui-testing/scenarios/` is a committed final-product contract",
		"focused snapshot-first migration and interrupted activation recovery tests",
		"App timers go through `status.TickCmd`, never `tea.Tick`",
		"## Where the details live",
	} {
		if !strings.Contains(content, strings.Join(strings.Fields(required), " ")) {
			t.Errorf("CLAUDE.md missing %q", required)
		}
	}
}
