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
	// The budget polices prose, not rule count: it is what forces mechanism into
	// docs/architecture.md and behavior into README.md. It moves only when a
	// feature adds invariants that cannot live anywhere else — the Outliner
	// project sheet, the enumerated project delete, the restore lock scope, the
	// cursor row's gutter/content seam, Select Project's preview pane owning a
	// second scroll, the parse-order/teach-order registry split that the
	// attachment sigil forced, the paired-delimiter comment sigil that leads
	// parse order, the Notes preview pane — a second read-only scroll whose
	// cache key and anchor rule nothing else states — the named lens a launch
	// reopens, which is the one thing layered over the clean-root reset and so
	// cannot be stated by the launch invariant it qualifies, and the search's
	// notes tier, whose read-only admission rule belongs to neither the Notes
	// surface nor the search grammar alone, and the soft task reference, whose
	// preserve-by-default patch and count-what-is-stored indicator are rules no
	// behavior or mechanism document can state for the surfaces that must obey
	// them, are the eleven that raised it from 16200 — and never to absorb
	// explanation. The last raise is the one exception and is not an invariant at
	// all: publishing the repository made this file readable by strangers, so it
	// opens with a paragraph saying what it is and pointing at docs/guide/ for
	// behavior. That framing is bounded and one-time; the budget still polices
	// every rule below it.
	//
	// This raise is taken once, before the work rather than after it, because
	// three queued features each need contract text and would otherwise race for
	// the same headroom — the first to land would take it and the next two would
	// each argue their own raise. It is earmarked for exactly three additions and
	// nothing else. The bulk move to another project: its destination project
	// must be committed even when it holds none of the selection, which is the
	// rule that separates it from every other bulk attribute and binds every
	// surface offering it. The machine-local adoption and merge of named lenses:
	// the migration chain is guarded by the synced catalog version while lenses
	// live outside every migration snapshot, so this step is keyed off
	// machine-local state and runs after the schema is already current — a
	// sequencing rule the persistence layout cannot state for it. And the
	// conflict signal: the status strip carries it and a read-only overlay
	// browses it, because the workspace pulse ladder is closed and there is no
	// sixth grammar, so both placements are rules stated against standing
	// refusals. Spending it on anything else is a new raise, argued on its own.
	//
	// That earmark is spent, so this is the new raise, argued on its own and for
	// one feature: the search grammar's uniform negation. Three rules come with
	// it and none can be stated anywhere else. `-` now means removal on every
	// surface, which is a claim about the whole registry rather than about any
	// one sigil, and the ten spellings it covers only stay coherent if the
	// registration order and the twin-resolved vocabulary are contract rather
	// than convention — a reviewer adding an eleventh has nowhere else to read
	// that the negation must sit immediately before its twin, longest trigger
	// first, and must own no values of its own. `?` opens a second kind of
	// predicate, one computed rather than stored, and its rule is a layering
	// refusal: the names and the answers live in `internal/models` because
	// `internal/tui` production code may not name `blocked`, so the contract
	// that already bans the word has to say where the word does belong. `!N+`
	// is one line and rides along because it is the same claim about where
	// acceptance lives. The prose that explains any of it stayed in
	// docs/architecture.md; what landed here is the rule set alone, in bullets.
	//
	// That raise is spent too, so this is the next one, argued on its own and for
	// one feature: the query readout and the peel. Its rules are refusals about
	// placement and authority that no other document can hold. The readout is a
	// fourth line in a strip whose height is reserved from the viewport, so it has
	// to be pure or the reservation and the paint disagree — a rule about
	// `StatusStripHeight`, not about a readout. It may name only what it can
	// prove, which is why a mode row names the mode rather than clauses the
	// catalog never declared, and it is the standing ban on inferred chrome that
	// makes that a contract rather than a design note. Peeling routes every axis
	// through the key's own call, so the one thing the model cannot do for itself
	// — reload the population — is delegated to the shell instead of reimplemented
	// under the strip, and the ranked-list refusal is the same standing refusal
	// the `F` chord already answers to. Two grammar rules ride along because they
	// are the same claim about vocabulary rather than about this feature: a
	// sigil-carrying facet answers to its sigil, which is what binds pinned to
	// `f*` and keeps the retired letter `fp` retired, and a picker row is the
	// canonical token it writes, enforced at registration, which is what stops a
	// picker minting a private spelling beside the grammar. The prose is in
	// docs/architecture.md and docs/guide/search-and-lenses.md; the rules alone
	// landed here.
	//
	// That raise is spent as well, so this is the next one, argued on its own and
	// for one feature: the soft reference read as a signal. Its rules are the kind
	// only this file can hold, because each one is a refusal binding several
	// surfaces at once. The reference reading is one hop and never transitive, a
	// ban that has to be stated where the row, Peek, the facet, search, and the
	// doctor census can all read it — no one of them owns it, and a closure would
	// look locally reasonable in any of them. `blocked` is live vocabulary now but
	// may never be rendered bare, which amends a rule already in this file rather
	// than adding a document elsewhere. And the `~` picker is tri-state, which is
	// what lets one picker take a selection: the row nobody agreed on is emitted
	// by neither half, and an addition naming a task inside the selection refuses
	// by name — a rule about what a gesture may not mean, not about how it is
	// implemented. The row shape, the facet's carve-out from the row predicate,
	// and the drop-ladder position ride along as the same claim. The prose is in
	// docs/architecture.md and the two guides.
	//
	// That raise is spent, so this is the next one, argued on its own and for one
	// feature: review as a stored axis. It is the rare change that rewrites this
	// file rather than appending to it, because the sentence it replaces was a
	// contract — review had no key and rode an ordinary tag — and a stale contract
	// is worse than an absent one. What takes its place is the same shape as the
	// pin and has to be stated once for four surfaces at a time: `=` writes the
	// flag on capture, edit, outline, and search, `=unreview` clears it on the two
	// surfaces that may clear anything, `-=` narrows and owns no vocabulary, and
	// `f=` is the facet, with `fr` retired the way `fp` was. None of that can live
	// in a guide, because a guide says what the sigil does rather than what a
	// reviewer adding the next one may not do. The schema pair moves with it — a
	// v17 catalog over schema-15 Markdown, and the migration that lifts the tag
	// onto the flag — and that pairing sentence already lived here because nothing
	// else states which task grammar a catalog version implies. The prose is in
	// docs/architecture.md and the guides; the rules alone landed here.
	// That raise is spent too. This one buys two rules the cross-machine story
	// cannot state anywhere else. The first is a forbidden pattern with teeth:
	// `.tt/transactions/` is portable, so a reviewer touching recovery has to know
	// that another machine's live manifest is left alone rather than rolled either
	// way — the plausible reading, "an unfinished transaction is interrupted work",
	// is the one that makes one machine roll back a commit another is publishing.
	// The second is that `just cross-machine` is deliberately outside `just ci`,
	// which is a rule about the gate rather than about the code, and a contributor
	// who does not know it will either delete the recipe as dead or wire it into CI
	// where it forks processes and checks out tags. Both are invariants; neither is
	// derivable from the tests, because the tests that could state them are the
	// two-process ones no in-process suite runs.
	// That raise is spent as well. This one buys the menu bar, and it is bought
	// twice over because the bar breaks two defaults a contributor would otherwise
	// reach for. The first is where a new chrome flag goes: machine.json's "only
	// those two" was a closed list, and closing it again at three without saying
	// why would read as an accident, so the rule now names the test the list is
	// closed by — chrome follows the terminal you are sitting at — and the
	// omitempty-and-never-seeded shape that makes an absent key mean the default.
	// A reviewer who does not have that test will either put the next flag in the
	// portable workspace, where it follows the user to a terminal that cannot draw
	// it, or seed it and hand every existing machine a surprise. The second is that
	// the bar is a view-local gated sub-state rather than a presentation. Every
	// instinct here says otherwise: it is persistent, it is visible chrome, and it
	// will grow dropdowns, all of which look like reasons to mint a
	// `PresentationID` and a manifest entry — and doing so silently changes what
	// `surfaceOperationGates()` and `ValidateOperationOnePath` cover. The one-row
	// cost belongs beside it, because the failure it prevents is the quiet one: a
	// second subtraction at a call site does not crash, it just draws a row short.
	// Neither rule is derivable from the code, since the code that would state them
	// is the code a contributor is about to write.
	// That raise is spent as well, and this last one is the smallest yet: it buys
	// the sentence the finished menu bar makes necessary and rewrites a contract
	// rather than appending to it, which is the one case this file pays for
	// willingly. "On every surface" was true when the only menu was an entity
	// action sheet. Once the bar exists it reads as a ban on the bar naming any
	// operation that has a key, which would empty it — and a contributor obeying a
	// stale contract does more damage than one with no contract at all, because the
	// obedience looks like care. So the sentence is narrowed to the sheet it always
	// meant, the bar is classified as a projection beside `?` help and the footer
	// ranks, and the one genuinely new rule lands beside it: the bar may make an
	// operation keyless, but not an entity operation keyless. That last clause is
	// the only place the boundary is written down. `ValidateOperationOnePath`
	// cannot state it — it never sees the bar — and the bar's own specs cannot,
	// because the defect it prevents is a descriptor that looks locally reasonable
	// in core_input_operations.go and only becomes wrong when a menu row names it.
	// That raise is spent as well, and this one buys two rules that are both
	// categories rather than features, which is the only kind of prose this file
	// has ever paid for. The first is the advisory gate. A contributor adding the
	// next ambient row — a sync warning, a quota line — will reach for the strip
	// directly, and the row will then be unhideable while `--no-advisories`
	// silently claims to cover it. So the rule names the category, names the two
	// members, and forbids gating at the render site, because the plausible local
	// fix (an `if hidden` beside the paint) is exactly what breaks the single
	// `StatusStripHeight` measurement the strip's whole height contract rests on.
	// The second is conflict-bundle decay: `.tt/conflicts/` is portable and
	// unmigrated, so a bundle outlives its schema, and the default instinct — a
	// document that will not parse is a fatal error — is what stranded a real
	// record where adopting remote was the only legal resolution. "Undecodable is
	// allowed, unresolvable is not" is a rule about which paths may read a stored
	// document, and neither the merge code nor the store can state it, since both
	// are what a contributor is about to change.
	// That raise is spent as well, and this one is the first that buys no rule at
	// all. The package split gives a named thing a longer name: `tickCmd` became
	// `status.TickCmd` when Session 2.6 moved it, and the sentence naming it grew
	// by the seven bytes of the qualifier. Absorbing that by trimming an unrelated
	// rule would be the worse trade, because it makes the diff claim an edit the
	// author did not intend to make. So a requalification is paid for at cost, in
	// exactly the bytes the qualifier adds, and says so here; the ratchet still
	// refuses prose, which is what it was built to refuse. Expect one of these per
	// remaining extraction that touches a name this file spells out.
	//
	// That raise is spent as well, and this one is the second that buys no rule
	// either: it pays for the layer-ownership sentence becoming true. Phase 1 of
	// the package split turned `internal/tui` from one package into twenty-two,
	// so the clause calling it "the sole Outliner root and operation
	// registry/executor" stopped naming anything a contributor could act on — and
	// a forwarding address with no destination is worse than none, because the
	// reader concludes the layering is prose rather than a graph and puts the
	// next concern wherever it compiles. What replaces it is the same rule at the
	// same altitude, spelled at the granularity that now exists: the eight
	// layers, which packages sit in each, and the table plus fitness test that
	// refuse an edge nobody declared. It is bought at cost, in the bytes the
	// enumeration takes, and no rule elsewhere was trimmed to pay for it. Phase 2
	// folds `outliner` out of the shell and rewrites this list once more; that is
	// the same kind of raise and is argued then rather than pre-approved here.
	//
	// That is this raise, and it is the one the sentence above pre-declared.
	// Phase 2 landed `outliner` as its own L7 package and decomposed the shell,
	// so the enumeration written for twenty-two packages named a graph that no
	// longer existed. It was replaced twice over: once by prose forwarding to
	// `scripts/tui_package_graph.tsv`, which was accurate but nameless — a
	// contributor holding a new concern learned that a table decides, not which
	// row their concern belongs in — and now by the nine-layer block that names
	// all twenty-three packages and the shell. The block is the cheapest form
	// the rule has: one line per layer, package names and nothing else, with the
	// whitelist, the fitness test, and every edge's reason left in the table
	// where they already live. It costs 227 bytes and no rule elsewhere was
	// trimmed to pay for it. A package added or a layer moved rewrites the block
	// and is a raise of the same kind, argued then.
	//
	// A third raise, of 171 bytes, for the menu bar's two visibility sources.
	// While `Ctrl+M` revealed a hidden bar by committing the machine-local flag,
	// there was one source and "`M` toggles the Outliner menu bar" covered every
	// question a contributor could have: the bar was shown because someone chose
	// it. The reveal is now the open menu's own, so a frame draws the band for
	// either of two reasons, and which one the View checkbox and the close read
	// is exactly the kind of fact that gets guessed wrong once and then copied.
	// The sentence names the disjunction, its one writer, and the projection the
	// checkbox takes, and nothing else; the mechanism is in
	// docs/architecture/presentation-chrome.md, where the rest of the bar's
	// implementation already lives. No rule elsewhere was trimmed to pay for it.
	//
	// A fourth raise, of 54 bytes, for the review key. Both edits correct a
	// standing enumeration rather than add a rule: the review sentence named the
	// axis's whole keyboard surface and would otherwise still name it as the
	// sigil and the facet alone, and the one-key-one-act table is an inventory of
	// the keys that rule governs, so a key it omits is a key nobody is holding to
	// it. Fourteen of those bytes are the table cell; the rest is the clause that
	// says the row key is the sigil's own character rather than a new one, which
	// is the fact a contributor would otherwise have to infer from the absence of
	// a second spelling. No rule elsewhere was trimmed to pay for it.
	//
	// A fifth raise, of 367 bytes, for `g~`'s return trail. It extends the
	// standing session-only sentence rather than opening a section: the trail is
	// the second thing in the Outliner that survives no relaunch, and the four
	// facts a contributor cannot get from the code without reading three files
	// are exactly the ones a later change would break silently — what a frame may
	// hold (no task or project pointer, which is what lets a return resolve
	// against current data), where `Esc` spends it in a cascade whose order is
	// otherwise only implicit in a handler, that `0` drops it whole, and that a
	// hop suspends a live lens through the one seam that touches no persistence.
	// The mechanism is in docs/architecture/tui-architecture.md, where the rest
	// of the revelation path already lives. No rule elsewhere was trimmed to pay
	// for it.
	//
	// A sixth raise, of 403 bytes, for selection-aware copy. It corrects a
	// standing enumeration and adds the one rule that correction creates. The
	// clause listing clipboard beside structural, edit, Peek, and relation
	// operations became false the moment the three projections started taking a
	// selection, and a false refusal is the worst kind of rule here: a
	// contributor reads it and adds the guard back. What replaces it is the
	// contract that made the refusal unnecessary — one snapshot, one
	// `external.CopyText` call, so partial success has nowhere to live — plus
	// the ordering seam, which is the fact no code path advertises: copied text
	// is the one place target order is the result, so `ClipboardSelectionOrder`
	// exists beside `NormalizedTaskIdentities` rather than replacing it, and a
	// later change that folds the two together would silently reorder every
	// copy. The mechanism is in docs/architecture/selection-and-project-tags.md,
	// where the rest of the selection contract already lives. No rule elsewhere
	// was trimmed to pay for it.
	//
	// A seventh raise, of 341 bytes, for the resume note. Two of the three edits
	// are digit swaps a schema boundary always forces; the cost is the rule the
	// new field creates. `ResumeNote` is the first task field with a shape a
	// caller can get wrong — trimmed, single-line, bounded in runes — and the
	// only thing standing between a bad value and unrepresentable bytes is that
	// every surface routes through one validator. A second gate written beside a
	// future editing surface would pass what the canonical writer refuses, and
	// the refusal would surface as a failed save rather than a rejected edit, so
	// naming `models.ValidateResumeNote` as the single gate is the rule. The
	// v18 clause pays for itself by keeping the pairing paragraph honest: it is
	// the boundary that rewrites every `tasks.md` and changes nothing in it, and
	// a contributor who assumes otherwise looks for the lens restage that is
	// deliberately absent. The mechanism is in docs/architecture/migration.md.
	// No rule elsewhere was trimmed to pay for it.
	const budget = 30960
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
		"`ComputeOutlinerViewport` is the single window authority",
		"Never recompute the window in a key handler",
		"Manual order is never touched by the sort axis, and that holds by construction rather than by a guard " +
			"— do not add one. Group order never changes",
		"Arming stays in the one deferred closure in `AppModel.Update`",
		"The interactive TUI ignores an inherited `NO_COLOR`",
		"A value provider is exactly one of `TextEntry()`",
		"`inlinePeekLines` owns a drop order, not a set",
		"`VisualWidthOverflow` must subtract the same ellipsis column as `TruncateToVisualWidth`",
		"`indicatorDropLadder` is a list: a new indicator gets a position, not an append",
		"The trailing `~S/N` counts stored locators in `N` regardless of target",
		"amber blocked, green settled, muted unreadable",
		"an addition naming a task inside the selection",
		"Its read-only `Dependencies`/`Dependents` lines sit above tags",
		"`project export` shares that fallback and writes no dependents",
		"`~task` is the one repeatable reference crossing a project boundary",
		"`DependencyPatch`'s zero value is no intent, so a save that never mentions a reference " +
			"preserves the set",
		"The relation marker, the task context segment, and the pin marker stay out of it",
		"`taskRowSegments` reserves all three at every width",
		"`taskRelationMarker` is one segment both ways",
		"The four-step pulse ladder drops done, inbox, overdue, then today",
		"pace outranks the workspace pulse: exhaust that ladder first, then shorten and remove pace before " +
			"truncating required interaction context",
		"All's Stale lane follows Review; below Pinned/Today/Next/Review/Doing",
		"`fs` is unavailable in Recent; entry clears only Stale: " +
			"`Stale filter cleared: Recent only includes the last 30 days`",
		"There is no `$delete` sigil",
		"The old `b`, `c`, `d`, `m`, `w`, `:`, `C`, `R`, `Ctrl+Esc`, `H`, `L`, and the " +
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
