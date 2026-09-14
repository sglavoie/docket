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
	//
	// An eighth raise, of 532 bytes, for the two keys that stopped naming one
	// entity. `e` joined `*`, `#`, and `Space` as a shared key, so the sentence
	// that enumerates them had to grow a third member and, with it, the one
	// rule the widening creates: Edit project's Outliner route is narrowed to a
	// project header, because `e` is a root key there and the sole-scope
	// fallback that is right on the Hub would otherwise edit the scoped project
	// from a section header. `c` cost a checklist bullet for the same reason in
	// the other direction — Add Item is the one member of that family answering
	// from a row it does not own, and the fold and cursor it moves are a
	// property of the commit rather than of the press, which is the fact a
	// future handler that reveals at dispatch time would get wrong. The
	// explanatory halves live in docs/architecture/operation-grammar.md and
	// docs/architecture/tui-architecture.md. Retiring the start-only operation
	// and the Edit project sheet row paid part of the raise back in the same
	// edit; no rule elsewhere was trimmed for the rest.
	//
	// Ninth raise, 31492 to 31942, for quick Add Comment on `C`. 450 bytes buy
	// three rules that nothing else in the file implies. The first is which of
	// the two comment routes may do what: `;` still owns reading, editing,
	// reordering, and deleting, `C` only appends, and the only reason that is
	// not a second route to an existing act is that they share one mutation and
	// one limit — a reader who does not know that would either duplicate the
	// mutation or refuse the key. The second is that a multiline `TextEntry()`
	// provider moves apply off `Enter`, which the value-provider rule above
	// cannot say for it, since Enter-as-newline is a property of the value and
	// not of the provider kind. The third is the pair of properties that make
	// the popup safe rather than merely convenient — one submission per open,
	// and text that survives a refusal — because both are easy to drop in a
	// later refactor and neither shows up in a passing render. Nothing was
	// trimmed to pay for it: the retired-key list gave back only the five bytes
	// `C` itself occupied.
	//
	// Tenth raise, 31942 to 32732, for a Peek that pages. 790 bytes buy the two
	// rules the old five-line cap used to make unnecessary. The first is that
	// there are now two heights — the document's and the page's — and exactly
	// one function returns either, which is the whole reason a render and a
	// viewport measurement taken at different widths still agree; a reader who
	// reached for a second line count would reserve a height nothing draws. The
	// second is that a remembered page is state, and state in a render path
	// needs its reset rule stated where the render rule is: keyed on identity,
	// content, and size, read purely, written from one place. Neither is
	// derivable from the drop-order bullet they replace, which said the
	// opposite — that Peek sheds content to fit. That bullet, the body order it
	// carried, and `Ctrl+Enter`'s line in the retired-key list were all trimmed
	// in the same edit; the section order survives because it is still a rule,
	// just about placement rather than about what gets dropped. The
	// explanatory half lives in docs/architecture/search-filters-and-sorting.md.
	//
	// Eleventh raise, 32732 to 33543, for repeating a task on `D`. 811 bytes buy
	// two rules and one prohibition, none of them implied by anything above. The
	// first is the seam: a draft is prepared read-only in the application layer
	// and reaches the composer as copied values through a typed seed rather than
	// as `InitialText`. The preset already carries a text field that the compact
	// surface parses, so a reader following the nearest precedent would route the
	// title through the capture grammar and silently turn a literal `#123` into a
	// tag — the file has to say which channel this is, because both exist and
	// only one is correct. The second is the retain/clear split itself, which is
	// a policy decision rather than a derivation: nothing in the task invariants
	// says whether a repetition keeps a due date or a pin, and a later reader
	// adding one field to the seed would have no line to check against. The
	// prohibition — no recurrence field, no repeat relationship, no copy-options
	// screen — is the boundary that keeps this one key from growing into a
	// scheduling feature, and it is exactly the kind of thing a passing test
	// cannot state. Nothing was trimmed to pay for it; the key table cell is
	// fourteen of the bytes and the rest is the paragraph. The behavioral half
	// lives in docs/guide/outliner-keys.md.
	//
	// Twelfth raise, 33543 to 34557, for acknowledging a review candidate on `R`.
	// 1014 bytes buy one policy, one prohibition, and one cursor contract, and the
	// prohibition is the reason the other two cannot wait. This file already says
	// that only the lifecycle transitions write the state clock, which is exactly
	// the sentence that sends a reader implementing "mark it reviewed" to
	// `ApplyStateTransitionAt` — and that call clears deferral and completion data
	// on the way past. So the file has to say what acknowledging is instead: a
	// flag clear and a touch, no transition, no second timestamp, no stored queue.
	// The policy is the admission set. Flagged, stale, or Waiting is a choice
	// nothing above implies — review is a stored axis and staleness is a derived
	// one, and joining them is this operation's own decision — and a later reader
	// adding a fourth clause has no other line to check against. The cursor
	// contract is the half that is not about the mutation at all: the successors
	// are read before the commit because the list that answers "who is next" stops
	// existing once the acknowledgement lands, and the refusals beside it — no
	// fold expanded, no lens changed, no project loaded, no wrap — are what keep a
	// triage key from quietly becoming a navigation feature. `=` keeping both
	// halves off is one clause and rides along, because the two keys are otherwise
	// indistinguishable in a descriptor list. The behavioral half lives in
	// docs/guide/outliner-keys.md and the mechanism in
	// docs/architecture/tui-architecture.md; the retired-key list gave back the
	// five bytes `R` occupied.
	//
	// Thirteenth raise, 34557 to 35063, for what a merge owes the canonical
	// writer. 506 bytes buy one rule and the one worked example that makes it
	// checkable. The rule is not discoverable from the code: auditMergedDocument
	// reads as a duplicate of models.AuditProject, so a reader adding an invariant
	// to the writer has no reason to look at it, and the failure that follows is
	// silent in the only way that matters — the merge succeeds field by field, the
	// commit fails while rendering, and the losing machine holds an error that
	// names no bundle, so `tt conflict` lists nothing and the retry path that
	// already exists is never reached. Concurrent creation is the example because
	// it is the case the file otherwise implies is handled: ShortID is
	// project-local and minted next-free, which two machines do independently
	// against documents neither has seen, and nothing above says where that
	// collision is caught. Naming the audit rather than the symptom is what keeps
	// the next invariant — not this one — from landing in the same blind spot.
	// The mechanism lives in docs/architecture/persistence-and-transactions.md.
	//
	// Fourteenth raise, 35063 to 36119, for the project-comment collection. 1056
	// bytes buy three rules a reader cannot derive. The first is where the
	// `PROJECT-COMMENTS` block sits and what fixes its bytes: placement, escaping,
	// UTC, and an order derived from stored values alone, because the canonical
	// rendering is what ComputeRevision hashes and two machines disagreeing about
	// any of those would trade revisions that describe one collection. The second
	// is that `tasks.md` is now the whole project, so `WriteProjectMarkdown` is the
	// only production renderer and the name-plus-task-list one is gone rather than
	// deprecated — a rewrite site that could still spell a name and a task list
	// would drop comments silently, and rename, task edit, outline edit, and
	// cross-project move all rewrite documents they did not otherwise change. The
	// third is that comments merge as one collection rather than by id, which
	// reads as an oversight beside the field-level task merge and is instead the
	// conservative seam an id-keyed merge replaces once comments can be written.
	// The mechanism for all three lives in
	// docs/architecture/persistence-and-transactions.md.
	//
	// Fifteenth raise, 36119 to 36516, for the identity-keyed comment merge that
	// replaced the conservative seam the fourteenth raise's third rule named. 397
	// bytes buy the rules that seam did not need. Merging by id rather than
	// position is the one a reader would otherwise have to infer from the value
	// carrying a UUID, and inferring it wrong is silent: an index-keyed merge
	// reconciles two unrelated comments after the other side inserts one above.
	// CreatedAt being refused even when both sides moved it alike is the opposite
	// of every other field beside it, where agreement coalesces, so it has to be
	// stated or the next reader will "fix" it. The conflict shape — CommentID plus
	// one of three field names, never TaskTitle, one entry per diverging id — is
	// what a bundle is addressable by, and borrowing TaskTitle would send an
	// operator looking for a task that does not exist. The mechanism lives in
	// docs/architecture/persistence-and-transactions.md.
	//
	// Sixteenth raise, 36516 to 36574, for `C` joining the shared keys. 58 bytes
	// buy two rules that are cheaper stated than inferred. The first is that the
	// key is shared at all: a reader who knows `*` and `#` branch on the cursor
	// has no way to tell whether the comment key does, and guessing wrong writes a
	// project comment where a task comment was meant. The second is the name of
	// the guard, which stopped being about one operation: the narrowing is now a
	// set, so a third root key aimed at a project header declares itself in
	// `outlinerProjectOperationTargetsHeader` instead of restating the check. The
	// mechanism lives in docs/architecture/operation-grammar.md.
	//
	// Seventeenth raise, 36574 to 37340, for `;` joining `C` on the project header
	// and for what the project half of that panel is held to. 766 bytes buy three
	// rules that are silent when guessed wrong. The first is that the second owner
	// is an adapter over the one panel rather than a second panel, which is the
	// decision a reader is most likely to undo: a project comments surface reads
	// like a sibling of the task one, and building it that way forks the grammar,
	// the footer, and the delete confirmation. The second is that a project
	// comment is addressed by id and by the text the panel read, and that the list
	// runs newest first — those two are a pair, because reversing the display is
	// exactly what makes an index-addressed mutation rewrite the wrong comment,
	// and the failure is invisible: it commits. The third is that the adapter
	// re-resolves its project by identity on every reload, which a reader holding
	// a `*models.Project` has no reason to expect until a commit replaces the
	// object and the panel is silently commenting on a discarded one. Preserving
	// the UUID and CreatedAt across an edit is stated with them because an edit
	// that mints either is a different comment to the merge. The mechanism lives
	// in docs/architecture/operation-grammar.md.
	//
	// Eighteenth raise, 37340 to 38284, for the one header a focused project
	// leads with. 944 bytes buy four rules a reader would otherwise have to
	// rediscover from a bug. The first is that this row is not a group header:
	// giving it a collapse key folds the whole projection out from under the
	// cursor, and `IsCollapsibleHeader` exists so `h`/`l`, `p`, the fold walk,
	// and the glyph cannot disagree about which headers own a key. The second is
	// the finalization seam: the four `buildCanonicalModeRows` exits used to each
	// finish their own rows, so a header appended to the normal tail alone is
	// missing from ranked search and from the two early returns, which reads as
	// an intermittent header rather than a missed branch. The third is that the
	// insert happens after the body, because `pinnedBoundary` and the narrowing
	// attribution are absolute indices into the rows already emitted. The fourth
	// is that an empty projection is decided by `HasContentRows` and not by the
	// row count, which is now never zero where the header exists — the same
	// mistake would silently retire every empty-state explanation. The mechanism
	// lives in docs/architecture/tui-architecture.md.
	//
	// Nineteenth raise, 38284 to 38596, for the two step sizes Peek now scrolls
	// by. 312 bytes buy three rules whose absence is silent rather than loud. The
	// first is that the remembered position is a content-row offset and not a
	// page ordinal: two distances share one window, and a reader who restores the
	// ordinal reintroduces boundaries the document imposed, which is exactly the
	// state where mixing the two keys cannot reach some lines. The second is that
	// consumption and availability are different questions — a visible Peek eats
	// all four keys, including at both ends, while only the footer reads what can
	// move. Deriving applicability from `CanPage*` instead reads as the tidier
	// rule and is a bug: a shared binding that declines falls through to the next
	// owner, so the outline scrolls under a Peek the user was reading. The third
	// is the corollary that keeps that fallthrough available where it is wanted —
	// `PgDn`/`PgUp` are sole-owner paths and stay the cursor's. The owner key
	// being typed rather than a bare task identity is stated with them because it
	// is the seam a project-header Peek takes. The mechanism lives in
	// docs/architecture/search-filters-and-sorting.md.
	// Twentieth raise, 38596 to 39371, for Peek's second document kind. 775 bytes
	// buy three rules that are silent when guessed wrong. The first is how `b` is
	// shared, which is not how any other shared key is shared: `*`, `#`, `C`, and
	// `;` are two descriptors branching on the cursor, and copying that pattern
	// here mints a second Peek descriptor, a second footer label, and two windows
	// over one pane. Peek is one act over `OperationTargetCurrentEntity` instead,
	// and the gate that keeps it off a section, note, or checklist row has to be
	// named or the next reader reintroduces the task-only requirement that made
	// the key silently dead on a header. The second is that the document resolves
	// the row's own `ProjectID` and not the view's: a cross-project group header
	// names a project the scoped model is not, and resolving through the scope
	// draws the wrong project's comments under the right project's header, which
	// commits nothing and is therefore never caught. The third is that the owner
	// is the project identity rather than the header's collapse key — All emits
	// one header per state section, so keying on the row makes a page turn under
	// one of them a different window under the next, and only a reader who
	// scrolled and then moved would ever see it. The mechanism lives in
	// docs/architecture/tui-architecture.md.
	// Twenty-first raise, 39371 to 39561, for the workflow chord's move off `$`.
	// 190 bytes buy three rules that are silent when guessed wrong. The first is
	// that `w` is the one keyboard route not spelled like its sigil: every other
	// route spells one, so a reader who finds the correspondence broken fixes it
	// by binding `$` back, and the exception has to be stated where the rule is.
	// The second is that `$` is typed and pressed nowhere — a compatibility alias
	// is the obvious kindness and it is what keeps two vocabularies alive, which
	// is the same reason `d` and `m` are named above. The third is that the states
	// facet follows the chord to `fw` rather than the sigil to `f$`: the facet rule
	// in the next sentence makes `fw` look like the violation to repair, and
	// repairing it splits one axis across a letter and a sigil. The mechanism lives
	// in docs/architecture/operation-grammar.md.
	// Twenty-second raise, 39561 to 40095, for the combined date picker's two
	// fields. 534 bytes buy four rules that are silent when guessed wrong. The
	// first is that the active field is the cursor's row and nothing else: a
	// section index kept beside the cursor is the obvious implementation and it
	// drifts, so the guidance names one field while `a` writes the other and
	// nothing refuses. The second is that the opening highlight reads due alone —
	// coupling it to deferral agreement reads as symmetry and instead withholds a
	// highlight the selection does share, since the cursor can only sit in the one
	// section the picker always opens in. The third is that reading a row's field
	// and committing it are different questions, which is what keeps a terminal
	// task's disabled defer rows reachable: fusing them makes `Tab` skip the rows
	// whose stated reason is the only thing explaining the refusal. The fourth is
	// that `Tab` is consumed on every picker — a picker with one section that
	// declines instead leaks the key to the Outliner's mode change under an open
	// overlay. The measured guidance budget is named with them because it is what
	// lets the field name grow the footer without eating a terminal row. The
	// mechanism lives in docs/architecture/operation-grammar.md.
	// Twenty-third raise, 40095 to 40638, for the date picker's preset keys. 543
	// bytes buy four rules that are silent when guessed wrong. The first is that
	// the keyboard table lives one layer below both surfaces that read it: a key
	// declared beside the rows it decorates is the obvious placement, and it lets
	// the creation form and the Outliner picker disagree about what `m` means with
	// nothing to fail. The second is the dispatch position — the browse grammar's
	// default arm is where a shortcut belongs by precedent, and `t` and `w` never
	// arrive there because the semantic-tag category jumps claim them above it, so
	// two of the five due keys are simply dead and only on one provider. The third
	// is that an unbound key is claimed anyway: letting it fall through reads as
	// harmless and quietly hands `1` to whatever the browse grammar binds next.
	// The fourth is that the hint rides `PrimaryMarker`, which is the one row field
	// query and ranking skip — spelling the key into the label instead makes typing
	// `m` match every row `m` applies. The mechanism lives in
	// docs/architecture/operation-grammar.md.
	// Twenty-fourth raise, 40638 to 42146, for deadline stepping's two keys. 1508
	// bytes buy four rules that are silent when guessed wrong. The first is that
	// the remembered clock is read at dispatch and not at the keypress: reading it
	// where the targets are frozen is the obvious symmetry, and it makes the second
	// press of a burst compute from the deadline the first press displaced rather
	// than from the one it committed. The second is that a gesture is never
	// coalesced — folding a burst into one transaction is the efficient-looking
	// move and it silently turns four presses into one undo. The third is that the
	// supersede reads `Edits`/`Removed` and never `Steps`: superseding on every
	// committed due effect reads as the conservative choice and has the queue
	// cancel its own successor on the first commit, so a burst applies once. The
	// fourth is that a detached result still reloads with its `DueEffect` blanked —
	// dropping the message instead leaves the shell rendering rows the transaction
	// already changed, and delivering it whole lets a late step write back clock
	// memory an explicit edit or a ledger restore has replaced. The fifth is that
	// the toast names the outcome rather than the ledger's context — a step's
	// ledger entry says `+1 day`, which is the right thing for the undo browser
	// and no answer at all to the press, and the re-render delayed origin feedback
	// runs would otherwise put the gesture back. The mechanism lives in
	// docs/architecture/operation-grammar.md.
	// Twenty-fifth raise, 42146 to 42859, for promoting a checklist item on `S`.
	// 713 bytes buy three rules that are silent when guessed wrong. The first is
	// that the frozen checklist is what makes a repeated press safe: the obvious
	// reading of "one item at a time" is a queue like the deadline keys', and
	// building one here is both unnecessary and wrong, because the commit already
	// refuses a list it no longer matches and a queue would instead retarget the
	// second press onto whatever slid up into the row. The second is that the
	// committed child reaches the view only through `RevealTaskFor` — every other
	// reveal in the file resolves a row that still exists, so a reader following
	// that precedent looks for the promoted item in a reloaded projection and finds
	// the one thing the transaction deleted. The third is the shape of the reveal
	// itself, which is the inverse of `revealTask` sitting next to it: that one
	// rewrites mode and filters until the target is visible, and copying it here
	// would throw away the user's view every time a done subtask lands under a
	// query that hides done work. Naming what the reveal may not write, and that a
	// hidden child is reported rather than chased, is what keeps the next reader
	// from "fixing" the narrower one into the wider one. The key table cell and the
	// direct-only clause ride along because the table is the inventory the one-act
	// rule is enforced from. The mechanism lives in
	// docs/architecture/tui-architecture.md.
	// Twenty-sixth raise, 42859 to 43073, for splitting the deadline detach in
	// two. 214 bytes buy two rules that are silent when guessed wrong. The first
	// is that pending work and committed evidence are separate invalidations: the
	// previous guidance said the tail drops and the `DueEffect` is blanked in one
	// breath, which reads as one act, and treating it as one is what made an
	// explicit deadline typed on any task discard the wall clock some other task's
	// already-committed step had just cleared — a loss nothing reports, since the
	// user only sees a date-only deadline several presses later. The second is
	// which half of the effect a supersession may touch: narrowing `Steps` alone
	// looks arbitrary beside `Edits`, `Removed`, and `Moves`, and narrowing those
	// too strands memory at an address the task has left or keeps it alive for a
	// task that is gone, because only a step ever writes a clock back. The two
	// entry points are named because the difference between them is the whole
	// rule: a mover that can name its targets must, and one that cannot — a ledger
	// restore, or a delivery the queue never armed — must name all of them. The
	// mechanism lives in docs/architecture/operation-grammar.md.
	// Twenty-seventh raise, 43073 to 43405, for naming where remembered clocks
	// are reconciled. 332 bytes buy the rule that the three reconciliation
	// moments are a closed set and that the third one is an adoption, not a
	// message. The guidance already said a deadline written on another machine
	// retires the clock it superseded, and left the reader to assume the reload
	// that delivers it does the retiring; it did not, so an externally set
	// deadline could sit on screen while a later `+` handed back a wall clock the
	// workspace had already replaced — again a loss nothing reports. Naming
	// `adoptCrossProjectSnapshot` rather than `CrossProjectsLoadedMsg` is the
	// whole rule: the published lens reaches the same seam, so a future third
	// population path inherits the reconciliation instead of needing a second
	// copy of the check, and hanging it off acceptance is what keeps a superseded
	// load from observing a workspace it never installed. The mechanism lives in
	// docs/architecture/operation-grammar.md.
	// Twenty-eighth raise, 43405 to 45435, for the check-in schedule. 2030 bytes
	// buy three rules and one correction. The first is that the three stored
	// values are one unit with one gate: split across surfaces they become a
	// repeat with no occurrence, a calendar date silently carrying an hour, or a
	// cadence counted from the date that was owed instead of the day the review
	// happened, and each of those reads as a working schedule that fires on the
	// wrong day. The second is that a reschedule is not an acknowledgement and a
	// review instant is history: `SetCheckIn` leaving the state clock alone and
	// `LastReviewedAt` surviving completion are what keep the workspace from ever
	// claiming a task was answered on a day nobody answered it, which is also why
	// the v21 seeding mints a date and nothing else. The third is that the merge
	// takes the three together — the one exception to field-by-field merging in
	// the whole document — because acknowledging and rescheduling differ only in
	// the review instant, so a per-field merge composes a schedule neither machine
	// wrote and no audit downstream can tell it from a real one. The correction is
	// the schema pairing, which had drifted two boundaries behind the code. The
	// mechanism lives in docs/architecture/migration.md and
	// docs/architecture/persistence-and-transactions.md.
	// Twenty-ninth raise, 45435 to 45763, for what `R` may be pressed on. 328
	// bytes buy one rule and one retirement. The rule is that admission and
	// navigation are two predicates rather than one: a person who opens a task
	// early and answers it is doing the thing the feature is for, while a pass
	// that walked onto a check-in whose day has not come would keep handing back
	// work nobody owes yet, and a single predicate cannot be both permissive
	// enough for the first and strict enough for the second. The retirement is
	// staleness and Waiting, which the previous rule admitted: a task that has
	// merely sat still never asked for anything, so every pass began with
	// cleanup material that buried the tasks that did ask, and the two remaining
	// routes — the flag and a date someone chose — are exactly the ones a person
	// raised deliberately. Naming which predicate `internal/app` rechecks is the
	// rest of it, since rechecking the narrower one would refuse the early review
	// the wider one just admitted. The mechanism lives in
	// docs/architecture/tui-architecture.md.
	// Thirtieth raise, 45763 to 46263, for time-driven attention presentation.
	// Five hundred bytes buy the contracts that a future check-in stays in Peek,
	// an arrived one joins rows and Review without displacing scheduled-only
	// Doing work, and the periodic invalidation waits out any interaction that
	// owns row identities before restoring the cursor. The same rule retires the
	// automatic Stale lane without retiring its explicit filter or lens. The
	// mechanism lives in docs/architecture/row-indicators.md and
	// docs/architecture/presentation-chrome.md.
	// Thirty-first raise, 46263 to 46602, for the reusable check-in editor. 339
	// bytes buy the one draft contract every scheduling entry point must share:
	// bulk starts without a borrowed default, clearing is explicit, date and
	// cadence remain paired until save, and a commit refusal restores both over
	// the frozen qualified targets. The mechanism lives in
	// docs/architecture/operation-grammar.md.
	const budget = 46602
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
		"The task-row `Space` menu holds Set Check-In and Delete, identical to the bulk menu: " +
			"the editor saves with `Ctrl+S`, while Delete confirms first",
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
		"Manual order is never touched by the sort axis, and that holds by construction rather than by a guard " +
			"— do not add one. Group order never changes",
		"Arming stays in the one deferred closure in `AppModel.Update`",
		"The interactive TUI ignores an inherited `NO_COLOR`",
		"A value provider is exactly one of `TextEntry()`",
		"`inlinePeekDocument` separates the whole detail from the window of it on screen",
		"`peekOffsetForRow` is pure and `scrollPeek` the only writer, so no render moves\nthe window",
		"`VisualWidthOverflow` must subtract the same ellipsis column as `TruncateToVisualWidth`",
		"`indicatorDropLadder` is a list: a new indicator gets a position, not an append",
		"The trailing `~S/N` counts stored locators in `N` regardless of target",
		"amber blocked, green settled, muted unreadable",
		"an addition naming a task inside the selection",
		"A reference target is named nowhere else",
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
		"All has no automatic Stale lane: old work stays in its lifecycle group",
		"The 60-second periodic tick invalidates a projection when its frozen local day is no longer current",
		"`fs` is unavailable in Recent; entry clears only Stale: " +
			"`Stale filter cleared: Recent only includes the last 30 days`",
		"There is no `$delete` sigil",
		"The old `d`, `m`, `$`, `:`, `Ctrl+Esc`, `H`, `L`, and the " +
			"comma state prefix stay unbound, as does `SortByStatus`",
		"`b` is live: it took Peek from the retired `Ctrl+Enter`, which no alias returns",
		"`;` opens the panel that reads, edits, reorders, and deletes comments and `C` " +
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
		"`>` and `<` are Set Parent and Adopt Children; indent and outdent are `]` and `[`, " +
			"which own no sigil",
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
