# Contributor guidance budget history 37340 to 59680

Historical rationale for the `CLAUDE.md` byte budget, preserved from
`contributor_guidance_docs_test.go`. These entries describe decisions at the
time they were made; `CLAUDE.md` remains the current invariant contract.

[Early history](contributor-guidance-budget-early.md) · [Recent history](contributor-guidance-budget-recent.md)

Eighteenth raise, 37340 to 38284, for the one header a focused project
leads with. 944 bytes buy four rules a reader would otherwise have to
rediscover from a bug. The first is that this row is not a group header:
giving it a collapse key folds the whole projection out from under the
cursor, and `IsCollapsibleHeader` exists so `h`/`l`, `p`, the fold walk,
and the glyph cannot disagree about which headers own a key. The second is
the finalization seam: the four `buildCanonicalModeRows` exits used to each
finish their own rows, so a header appended to the normal tail alone is
missing from ranked search and from the two early returns, which reads as
an intermittent header rather than a missed branch. The third is that the
insert happens after the body, because `pinnedBoundary` and the narrowing
attribution are absolute indices into the rows already emitted. The fourth
is that an empty projection is decided by `HasContentRows` and not by the
row count, which is now never zero where the header exists — the same
mistake would silently retire every empty-state explanation. The mechanism
lives in docs/architecture/tui-architecture.md.

Nineteenth raise, 38284 to 38596, for the two step sizes Peek now scrolls
by. 312 bytes buy three rules whose absence is silent rather than loud. The
first is that the remembered position is a content-row offset and not a
page ordinal: two distances share one window, and a reader who restores the
ordinal reintroduces boundaries the document imposed, which is exactly the
state where mixing the two keys cannot reach some lines. The second is that
consumption and availability are different questions — a visible Peek eats
all four keys, including at both ends, while only the footer reads what can
move. Deriving applicability from `CanPage*` instead reads as the tidier
rule and is a bug: a shared binding that declines falls through to the next
owner, so the outline scrolls under a Peek the user was reading. The third
is the corollary that keeps that fallthrough available where it is wanted —
`PgDn`/`PgUp` are sole-owner paths and stay the cursor's. The owner key
being typed rather than a bare task identity is stated with them because it
is the seam a project-header Peek takes. The mechanism lives in
docs/architecture/search-filters-and-sorting.md.

Twentieth raise, 38596 to 39371, for Peek's second document kind. 775 bytes
buy three rules that are silent when guessed wrong. The first is how `b` is
shared, which is not how any other shared key is shared: `*`, `#`, `C`, and
`;` are two descriptors branching on the cursor, and copying that pattern
here mints a second Peek descriptor, a second footer label, and two windows
over one pane. Peek is one act over `OperationTargetCurrentEntity` instead,
and the gate that keeps it off a section, note, or checklist row has to be
named or the next reader reintroduces the task-only requirement that made
the key silently dead on a header. The second is that the document resolves
the row's own `ProjectID` and not the view's: a cross-project group header
names a project the scoped model is not, and resolving through the scope
draws the wrong project's comments under the right project's header, which
commits nothing and is therefore never caught. The third is that the owner
is the project identity rather than the header's collapse key — All emits
one header per state section, so keying on the row makes a page turn under
one of them a different window under the next, and only a reader who
scrolled and then moved would ever see it. The mechanism lives in
docs/architecture/tui-architecture.md.

Twenty-first raise, 39371 to 39561, for the workflow chord's move off `$`.
190 bytes buy three rules that are silent when guessed wrong. The first is
that `w` is the one keyboard route not spelled like its sigil: every other
route spells one, so a reader who finds the correspondence broken fixes it
by binding `$` back, and the exception has to be stated where the rule is.
The second is that `$` is typed and pressed nowhere — a compatibility alias
is the obvious kindness and it is what keeps two vocabularies alive, which
is the same reason `d` and `m` are named above. The third is that the states
facet follows the chord to `fw` rather than the sigil to `f$`: the facet rule
in the next sentence makes `fw` look like the violation to repair, and
repairing it splits one axis across a letter and a sigil. The mechanism lives
in docs/architecture/operation-grammar.md.

Twenty-second raise, 39561 to 40095, for the combined date picker's two
fields. 534 bytes buy four rules that are silent when guessed wrong. The
first is that the active field is the cursor's row and nothing else: a
section index kept beside the cursor is the obvious implementation and it
drifts, so the guidance names one field while `a` writes the other and
nothing refuses. The second is that the opening highlight reads due alone —
coupling it to deferral agreement reads as symmetry and instead withholds a
highlight the selection does share, since the cursor can only sit in the one
section the picker always opens in. The third is that reading a row's field
and committing it are different questions, which is what keeps a terminal
task's disabled defer rows reachable: fusing them makes `Tab` skip the rows
whose stated reason is the only thing explaining the refusal. The fourth is
that `Tab` is consumed on every picker — a picker with one section that
declines instead leaks the key to the Outliner's mode change under an open
overlay. The measured guidance budget is named with them because it is what
lets the field name grow the footer without eating a terminal row. The
mechanism lives in docs/architecture/operation-grammar.md.

Twenty-third raise, 40095 to 40638, for the date picker's preset keys. 543
bytes buy four rules that are silent when guessed wrong. The first is that
the keyboard table lives one layer below both surfaces that read it: a key
declared beside the rows it decorates is the obvious placement, and it lets
the creation form and the Outliner picker disagree about what `m` means with
nothing to fail. The second is the dispatch position — the browse grammar's
default arm is where a shortcut belongs by precedent, and `t` and `w` never
arrive there because the semantic-tag category jumps claim them above it, so
two of the five due keys are simply dead and only on one provider. The third
is that an unbound key is claimed anyway: letting it fall through reads as
harmless and quietly hands `1` to whatever the browse grammar binds next.
The fourth is that the hint rides `PrimaryMarker`, which is the one row field
query and ranking skip — spelling the key into the label instead makes typing
`m` match every row `m` applies. The mechanism lives in
docs/architecture/operation-grammar.md.

Twenty-fourth raise, 40638 to 42146, for deadline stepping's two keys. 1508
bytes buy four rules that are silent when guessed wrong. The first is that
the remembered clock is read at dispatch and not at the keypress: reading it
where the targets are frozen is the obvious symmetry, and it makes the second
press of a burst compute from the deadline the first press displaced rather
than from the one it committed. The second is that a gesture is never
coalesced — folding a burst into one transaction is the efficient-looking
move and it silently turns four presses into one undo. The third is that the
supersede reads `Edits`/`Removed` and never `Steps`: superseding on every
committed due effect reads as the conservative choice and has the queue
cancel its own successor on the first commit, so a burst applies once. The
fourth is that a detached result still reloads with its `DueEffect` blanked —
dropping the message instead leaves the shell rendering rows the transaction
already changed, and delivering it whole lets a late step write back clock
memory an explicit edit or a ledger restore has replaced. The fifth is that
the toast names the outcome rather than the ledger's context — a step's
ledger entry says `+1 day`, which is the right thing for the undo browser
and no answer at all to the press, and the re-render delayed origin feedback
runs would otherwise put the gesture back. The mechanism lives in
docs/architecture/operation-grammar.md.

Twenty-fifth raise, 42146 to 42859, for promoting a checklist item on `S`.
713 bytes buy three rules that are silent when guessed wrong. The first is
that the frozen checklist is what makes a repeated press safe: the obvious
reading of "one item at a time" is a queue like the deadline keys', and
building one here is both unnecessary and wrong, because the commit already
refuses a list it no longer matches and a queue would instead retarget the
second press onto whatever slid up into the row. The second is that the
committed child reaches the view only through `RevealTaskFor` — every other
reveal in the file resolves a row that still exists, so a reader following
that precedent looks for the promoted item in a reloaded projection and finds
the one thing the transaction deleted. The third is the shape of the reveal
itself, which is the inverse of `revealTask` sitting next to it: that one
rewrites mode and filters until the target is visible, and copying it here
would throw away the user's view every time a done subtask lands under a
query that hides done work. Naming what the reveal may not write, and that a
hidden child is reported rather than chased, is what keeps the next reader
from "fixing" the narrower one into the wider one. The key table cell and the
direct-only clause ride along because the table is the inventory the one-act
rule is enforced from. The mechanism lives in
docs/architecture/tui-architecture.md.

Twenty-sixth raise, 42859 to 43073, for splitting the deadline detach in
two. 214 bytes buy two rules that are silent when guessed wrong. The first
is that pending work and committed evidence are separate invalidations: the
previous guidance said the tail drops and the `DueEffect` is blanked in one
breath, which reads as one act, and treating it as one is what made an
explicit deadline typed on any task discard the wall clock some other task's
already-committed step had just cleared — a loss nothing reports, since the
user only sees a date-only deadline several presses later. The second is
which half of the effect a supersession may touch: narrowing `Steps` alone
looks arbitrary beside `Edits`, `Removed`, and `Moves`, and narrowing those
too strands memory at an address the task has left or keeps it alive for a
task that is gone, because only a step ever writes a clock back. The two
entry points are named because the difference between them is the whole
rule: a mover that can name its targets must, and one that cannot — a ledger
restore, or a delivery the queue never armed — must name all of them. The
mechanism lives in docs/architecture/operation-grammar.md.

Twenty-seventh raise, 43073 to 43405, for naming where remembered clocks
are reconciled. 332 bytes buy the rule that the three reconciliation
moments are a closed set and that the third one is an adoption, not a
message. The guidance already said a deadline written on another machine
retires the clock it superseded, and left the reader to assume the reload
that delivers it does the retiring; it did not, so an externally set
deadline could sit on screen while a later `+` handed back a wall clock the
workspace had already replaced — again a loss nothing reports. Naming
`adoptCrossProjectSnapshot` rather than `CrossProjectsLoadedMsg` is the
whole rule: the published lens reaches the same seam, so a future third
population path inherits the reconciliation instead of needing a second
copy of the check, and hanging it off acceptance is what keeps a superseded
load from observing a workspace it never installed. The mechanism lives in
docs/architecture/operation-grammar.md.

Twenty-eighth raise, 43405 to 45435, for the check-in schedule. 2030 bytes
buy three rules and one correction. The first is that the three stored
values are one unit with one gate: split across surfaces they become a
repeat with no occurrence, a calendar date silently carrying an hour, or a
cadence counted from the date that was owed instead of the day the review
happened, and each of those reads as a working schedule that fires on the
wrong day. The second is that a reschedule is not an acknowledgement and a
review instant is history: `SetCheckIn` leaving the state clock alone and
`LastReviewedAt` surviving completion are what keep the workspace from ever
claiming a task was answered on a day nobody answered it, which is also why
the v21 seeding mints a date and nothing else. The third is that the merge
takes the three together — the one exception to field-by-field merging in
the whole document — because acknowledging and rescheduling differ only in
the review instant, so a per-field merge composes a schedule neither machine
wrote and no audit downstream can tell it from a real one. The correction is
the schema pairing, which had drifted two boundaries behind the code. The
mechanism lives in docs/architecture/migration.md and
docs/architecture/persistence-and-transactions.md.

Twenty-ninth raise, 45435 to 45763, for what `R` may be pressed on. 328
bytes buy one rule and one retirement. The rule is that admission and
navigation are two predicates rather than one: a person who opens a task
early and answers it is doing the thing the feature is for, while a pass
that walked onto a check-in whose day has not come would keep handing back
work nobody owes yet, and a single predicate cannot be both permissive
enough for the first and strict enough for the second. The retirement is
staleness and Waiting, which the previous rule admitted: a task that has
merely sat still never asked for anything, so every pass began with
cleanup material that buried the tasks that did ask, and the two remaining
routes — the flag and a date someone chose — are exactly the ones a person
raised deliberately. Naming which predicate `internal/app` rechecks is the
rest of it, since rechecking the narrower one would refuse the early review
the wider one just admitted. The mechanism lives in
docs/architecture/tui-architecture.md.

Thirtieth raise, 45763 to 46263, for time-driven attention presentation.
Five hundred bytes buy the contracts that a future check-in stays in Peek,
an arrived one joins rows and Review without displacing scheduled-only
Doing work, and the periodic invalidation waits out any interaction that
owns row identities before restoring the cursor. The same rule retires the
automatic Stale lane without retiring its explicit filter or lens. The
mechanism lives in docs/architecture/row-indicators.md and
docs/architecture/presentation-chrome.md.

Thirty-first raise, 46263 to 46602, for the reusable check-in editor. 339
bytes buy the one draft contract every scheduling entry point must share:
bulk starts without a borrowed default, clearing is explicit, date and
cadence remain paired until save, and a commit refusal restores both over
the frozen qualified targets. The mechanism lives in
docs/architecture/operation-grammar.md.

Thirty-second raise, 46602 to 47447, for neutral titles and one attention
ladder. 845 bytes buy two rules and one retirement. The first is that a
title carries no signal: every consumer of a task title shares one style
authority, so a rule written for the Outliner alone would leave previews,
autocomplete, and pickers recoloring the same text on their own terms, and
a title that means priority here and a semantic tag there means nothing
anywhere. The second is that the ladder is a closed precedence over badges
rather than a set of independent claims, which is what keeps one saturated
mark per row when overdue, review, due today, and priority 1 are all true
at once. The retirement is Waiting, as both hero and row tint: a lifecycle
state says what a row is rather than how loudly it is asking, and tinting
the quietest work in the workspace made it read as the most urgent. Naming
the state badge as identity is the rest of it, since a badge that is never
a hero would otherwise be demoted by every hero above it. The mechanism
lives in docs/architecture/row-indicators.md and
docs/decisions/behavioral-signals-contracts.md.

Thirty-third raise, 47447 to 47922, for Peek as the complete document. 475
bytes buy the rule that nothing in it sits outside the scrolling window.
A pinned header made the document two heights at once — one reserved, one
drawn — and pinned exactly the section a reader opens Peek to finish
reading, so the whole title and the ancestor chain are now ordinary body
rows. Spelling the new order is what makes "place a new section
deliberately" enforceable, and naming status and dates as the lines that
carry what the row stopped showing is what keeps a future contributor from
re-adding a badge to the row instead. The counterpart rule is the
retirement beneath it: a truncation has no inverse, so the surface that
needs the whole text renders the whole text. The mechanism lives in
docs/architecture/search-filters-and-sorting.md.

Thirty-fourth raise, 47922 to 48761, for the docked Peek. 839 bytes buy the
rule that the reservation is a constant: the pane moved out from under its
row and into a fixed block below the list, so the height reads neither the
cursor nor the document and moving between a one-line task and a forty-line
one leaves every list row where it was. A pane whose height was its
document's height re-laid-out the list on every cursor move, which is the
thing this retires, and stating the constant is what keeps a future
contributor from trimming the block back to what the current row happens to
need. The three consequences are the rest of it: a row with nothing to show
draws a placeholder rather than closing the gap, the position row is drawn
always so paging cannot change the height either, and consumption is the
dock being open rather than the cursor owning a document, which is what lets
the four keys stay eaten and `b` stay able to close what it opened. The one
exception, the empty projection, is named because it is the one path with no
list beneath the dock to protect. The mechanism lives in
docs/architecture/search-filters-and-sorting.md.

Thirty-fifth raise, 48761 to 49636, for the simplified row budget. 875
bytes buy the rule that the row promotes three indicators and no more, and
that indicatorDropLadder is exactly that list reversed. The old ladder had
ten rungs, so "a new indicator gets a position" read as an invitation to
add an eleventh; naming the three and naming where everything else went is
what makes the ceiling enforceable rather than advisory, and what keeps a
contributor from re-adding a tag chip or an age the row deliberately stopped
buying out of its title. The counterpart is the editor: the title budget a
row reclaims is worth nothing if the caret is laid out against a second copy
of the row's arithmetic, so titlePrefix() is named as the one source, and
the refusal-over-route precedence is named because a validation error the
user cannot read is the one way widening the title makes the editor worse.
The mechanism lives in docs/architecture/row-indicators.md and
docs/decisions/behavioral-signals-contracts.md.

Thirty-sixth raise, 49636 to 50484, for the group as a value. 848 bytes buy
the rule that a project group is an identity every row of a block carries,
not something read back off the header above it. Four separate helpers used
to reconstruct a block by scanning upward and then re-testing lane markers to
tell a Pinned row from the row that merely follows one, which is exactly the
inference a projection that stops drawing a header would break; naming the
value, naming its two keys as the folds that already exist, and forbidding
the scan is what keeps a fifth one from being written. The one predicate is
named because the emitter and the metadata disagreeing is the failure mode
that produces a fold nothing can open. The last sentence is the boundary:
All's Today lane and the Today mode spell one project fold alike, so a group
carried across a mode transition seats the cursor on the destination's
header, and modeCursorAnchor holding none is what keeps a mode switch landing
where its own arithmetic put it. The mechanism lives in
docs/architecture/tui-architecture.md.

Thirty-seventh raise, 50484 to 51328, for folding and navigation that no
longer read the heading. 844 bytes buy the rule that a group is reachable
whether or not its heading is drawn: the stop set n/N walk, the two bulk
folds' populations, h's last rung, l's landing, and the lifetime of the
memory that landing reads. Every one of those is a place a contributor would
otherwise reach for the header row again, which is the inference session 6
removed and this session's presentation switch makes false. The memory's
lifetime is stated because a group focus that outlived its fold would seat
the cursor on a task a restored checkpoint left behind rather than on one the
expanded group holds. The mechanism lives in
docs/architecture/tui-architecture.md.

Thirty-eighth raise, 51328 to 51904, for the compact project label. 576
bytes buy the rule that replaces the expanded heading rather than merely
deleting it: a group states its project exactly once, and which row carries
that statement depends only on whether the group is folded. Naming the
carrier on both sides is what keeps a contributor from restoring the heading
the moment a row looks anonymous, and from letting the label join
indicatorDropLadder, where a narrow row would shed the one segment that says
which of two identically titled tasks this is. The width constant is named
because a truncation with no counterpart is forbidden here, so the folded
summary is stated to be that counterpart. The two silences are the boundary:
both single-project forms already name the project in their chrome, and a
nested row's connectors already answer, so a label there would spend the
columns the row can least afford on a repetition. The mechanism lives in
docs/architecture/row-indicators.md and docs/architecture/tui-architecture.md.

Thirty-ninth raise, 51904 to 52637, for the form that windows instead of
clipping. 733 bytes buy the rule that the full task form has a viewport at
all, and the four consequences a contributor would otherwise undo one at a
time. Naming the field bounds is what keeps the next renderer from writing
rows the window cannot find — a tab that declares no field is silently
unscrollable, and the symptom is a control that will not come on screen
rather than an error. The offset being pure and the position row being paid
for only by a body that overflows are a pair: the first forbids a second
writer, and the second is why the rule is not "always reserve a row", which
is what a reader copying Peek's dock would assume. The banner staying chrome
is stated because moving it into the body is the natural simplification and
it hides the one message the focus was seated to explain. The demotion of
clampFormBody is the last: a helper that used to be the height policy reads
like one forever unless the file says it is now the defensive cut, and the
help column's own cut is what stops it from spending the body's last row.
The width guard is named as unchanged so session 10 still owns it. The
mechanism lives in docs/architecture/capture-and-outline-editor.md.

Fortieth raise, 52637 to 53633, for the form that fits the app's own floor.
996 bytes retire "the width guard is unchanged" and buy the guard that
replaced it. The floor being the app's rather than a second literal is the
rule a reader would otherwise re-derive from whichever renderer they opened,
and the preferred width being a ceiling is the one fact that stops the next
contributor from reading 68 as a minimum again and reinstating the notice.
One bodyWidth reaching every renderer and one file owning the chrome
arithmetic are a pair: the first makes a control's budget knowable, the
second makes it correct, and without both the fix decays into a per-file
guess the moment a tab is added. Charging the focused pad at every width is
counterintuitive enough to be undone as dead arithmetic, and the symptom —
a row that changes length when focus lands on it — is cosmetic enough to
survive review. The hint-before-value order and the Enter counterpart are
stated because this file forbids a truncation with no counterpart, so a
renderer that shortens a value has to say where the whole one is read. The
overlay owning the view is last because shrinking it is the natural repair
and it is the one that loses a picker's bottom border. The mechanism lives
in docs/architecture/capture-and-outline-editor.md.

Forty-first raise, 53633 to 55137, for capture drawn over a live host. 1504
bytes buy the rule that a view may own part of the content area rather than
all of it, and the consequences that are invisible from either side alone.
Naming the seam and the two fields is what stops the next bounded surface
from overlaying its host or registering a second view: HostRows and
HostDrawn are separate because the syntax guide needs the rows without the
paint, and collapsing them is the natural simplification. HostRows being a
pure function of the frame is the one that decays silently — a height read
off the cursor or the draft resizes the host mid-interaction, and SetSize
drops the scroll offset, so the symptom is an outline that jumps while the
contributor is typing somewhere else. The once-per-Update resize and the
blank rows above a declined host are each a bug this session fixed and would
otherwise be refixed by moving the arithmetic back into the render. The
pulse and the discard dialog are stated as the host's and the shell's
because a panel that paints either duplicates it, and duplication reads as
correctness. The bounded preview is last because clipping is what a row
budget suggests and it is the one outcome a capture surface may not produce.
The mechanism lives in docs/architecture/capture-and-outline-editor.md.

Forty-second raise, 55137 to 55580, for slash search opening the ranked
list. 443 bytes buy the one rule a reader cannot recover from the code: the
presentation is chosen by the seam the caller took, not by the query, so
there are two opens and SetSearchQuery is deliberately not the ranked one.
Collapsing them is the natural simplification — both install the same
compiled query — and it would silently rewrite every programmatic and test
caller's projection into a flat list that has no lanes, no folds, and no
mode order. The repeated-slash half is stated because it decays the other
way: making the second press restate the presentation reads as consistency
and costs the user both a deliberate `or` and the return frame `Esc` spends.
The mechanism lives in docs/architecture/search-filters-and-sorting.md.

Forty-third raise, 55580 to 56600, for a temporary search suspending the
lens it was pressed under. 1020 bytes buy the rules that survive no reading
of the code. The suspension living in searchState is the one a refactor
undoes first — outlinerLensState is where a lens frame obviously belongs —
and moving it there strands the frame the moment a relation hop or a nested
lens application carries the search's own checkpoint somewhere else. The
restore order is stated because it is invisible and wrong in exactly one
direction: the checkpoint carries the narrowing query, so installing it
after the frame is the correct order and installing it before silently
erases the lens just put back. Scope and facets staying standing is the
difference between a search and a reset, and forgetActiveLensFrame being
the put-down is what keeps a visit from writing a definition or the
machine-local pointer — a persistence call added here would make every
search a lens edit. HasSuspendedLens not being HasActiveLens is last
because the natural simplification is to keep the lens "active underneath",
which would put two narrowing queries on one projection; what is owed
instead is honest advertising, which is why one SearchExitHint answers for
the footer, the guidance, and the empty state.
The mechanism lives in docs/architecture/named-lenses.md.

Forty-fourth raise, 56600 to 57620, for mutation cursor recovery measured
against the row group. 1020 bytes buy the one rule the code reads as an
accident: rowIndexForTaskInGroup rather than RowIndexForTask. Bare
visibility is the obvious simplification — the identity is right there and
the projection already holds it — and it silently reintroduces the defect
this replaced, because All draws a state change one lane down and a move
draws it under another project, so "still on screen" and "still where the
user was pointing" are different questions with the same answer most of the
time. The recovery order is stated because it has no other home: the child
is looked for inside the captured group and the neighbors anywhere, which is
what makes a sweep cross a boundary only when the block it swept has run
out. The capture being frozen at the keypress is what a refactor undoes
first — a folded parent renders no child rows, so asking the rebuilt rows is
the natural rewrite and it silently drops every promotion. And no mode may
declare any of it: an OutlinerMode field gating recovery is exactly what was
removed here.
The mechanism lives in docs/architecture/tui-architecture.md.

Forty-fifth raise, 57620 to 58920, for undo focus correlated to the exact
committed transaction. 1300 bytes buy three refusals the code reads as
incidental. The stacks being a subsequence of the ledger's is the whole
design: it is what makes a restart, a bulk gesture, and an unrelated
transaction one uncorrelated case, and the obvious "improvement" — a map
keyed by transaction ID, so a correlation survives out of order — silently
resurrects a stale target and moves the cursor on an undo that restored
something else. Which seam may record is stated because CommittedDueEffect
is right there and already names addresses: electing one out of Moves or
Removed looks like reuse and would follow a descendant nobody pointed at,
and a bulk gesture would acquire a target it never had. And the refocus
being refusal-shaped — only a task already drawn, no expansion, no load, no
mode, scope, query, or fold write, and nothing at all under a visual
selection — is the rule a later session undoes first, because "reveal the
restored task" reads as helpful and would let undo rewrite a projection the
user chose. The ordering against refreshTaskViewsAfterSave rides along
because it is invisible and wrong in one direction only.
The mechanism lives in docs/architecture/tui-architecture.md.

Forty-sixth raise, 58920 to 59680, for Esc having one authority. 760 bytes
buy the rule a footer breaks by looking correct: an entry spelling its own
recovery — "Esc clear search" under a suspended lens, "Esc unzoom" inside a
Focus — reads fine at its own site and names an act the next press does not
perform, and nothing but the cascade knows which layer is on top. Saying
the Action rides on the step is what forbids the natural rewrite, a switch
in the profile; saying SearchExitAction answers for itself is what keeps
the one step whose act varies from being flattened back to a constant. The
draft pair is stated because one key genuinely does two things and the
discard question is not the delete confirmation: reusing
ConfirmationGuidanceSpec there is a one-line "cleanup" that tells the user
declining cancels their editing. Two gated rows rather than a branched
label is the same rule the footer sentence above already states, spelled
where a panel would otherwise reach for a conditional. And the width floor
is here because a required entry that does not fit is truncated rather than
dropped, so the failure is a half sentence at 60 columns and nothing else.
The mechanism lives in docs/architecture/presentation-chrome.md.
