# Contributor guidance budget history 59680 to 77480

Historical rationale for the `CLAUDE.md` byte budget, preserved from
`contributor_guidance_docs_test.go`. These entries describe decisions at the
time they were made; `CLAUDE.md` remains the current invariant contract.

[Early history](contributor-guidance-budget-early.md) · [Middle history](contributor-guidance-budget-middle.md)

Forty-seventh raise, 59680 to 60180, for a view-local modal standing on its
own dimmed host. 500 bytes buy the rule the old code read as the obvious
one: a dropdown that returns a frame instead of a box is a one-file change
that looks tidier and blanks the view behind it, and once the host is gone
the row budget is the modal's to spend, which is how a menu press came to
move the window it was pressed over. Naming composeOutlinerModal as the one
seam is what keeps the second modal from growing its own compositor, and
naming the anchor is what keeps the box off the bar and the operational
title, whose workspace pulse is the one thing a menu may never take. That
the box is bounded by what the anchor leaves, rather than by the frame, is
stated because the natural rewrite fits it to the whole frame and quietly
turns a long menu's scroll into interior rows dropped out of its middle.
The mechanism lives in docs/architecture/presentation-chrome.md.

Forty-eighth raise, 60180 to 60560, for a recovery that walks its captured
neighbors instead of probing one. 380 bytes buy the distinction the old
sentence left to chance: "the following unselected task" reads as one
identity, and one identity is exactly what a subtree move invalidates,
because the row nearest the cursor is usually the child the commit carried
into another project. The failure is silent and backward — the sweep lands
on work the user already read — so it looks like a cursor that merely
stayed put. Saying both runs are frozen whole is what forbids the natural
shrink back to a single successor and the equally natural cap on how far
each run reaches, since a truncated run reads as nothing surviving ahead
and fails the same way.
The mechanism lives in docs/architecture/tui-architecture.md.

Forty-ninth raise, 60560 to 60820, for one dock reservation both render
paths measure against. 260 bytes buy the exception the old sentence granted
outright: an empty projection has no list under the dock, so appending it
after the explanation and clamping it to whatever height is left is the
obvious way to write that path, and the result is a dock at the top of the
frame on one projection and at the bottom on the other. Naming
outlinerListRows as the one window, and saying the empty path pads to it and
draws the dock whole, is what forbids both halves of that rewrite; saying it
only skips computing a window is what keeps the next reader from concluding
the path owes a viewport and a scroll offset it has no rows to spend.
The mechanism lives in docs/architecture/presentation-chrome.md.

Fiftieth raise, 60820 to 61200, for a footer that moves only when something
is wrong. 380 bytes buy the difference between quiet and broken, which no
test can state on its own: a flash that never arrives reads as a bug, so the
obvious repair is to arm on any status change again, and the obvious
tidy-up is to let the severity switch fall through to "anything that is not
an error". Naming animates() as a closed table is what makes a later
severity declare itself rather than inherit silence. Saying quiet feedback
retires rather than abstains is the other half: leaving the warning's
animation running under the line that replaced it is what the abstaining
version does, and it says the new line is the alert. And separating whether
a status changed from whether that change animates is stated because the two
read as one condition — collapse them and either a tick retires the very
animation it was stepping, or a message expiring cancels frames its text
already earned.
The mechanism lives in docs/architecture/row-indicators.md.

Fifty-first raise, 61200 to 62000, for one instruction set at a time. 800
bytes buy the five things a reader would otherwise have to rediscover by
breaking one of them. Naming the resolution order is what stops the obvious
per-overlay repair: a footer speaking under a picker looks like the
picker's bug, so the natural fix is a check at the picker's own render
site, and the next overlay gets its own. Naming what the last rung asks —
is a host still drawn under this presentation, not did this presentation
draw its own acts — is the one distinction the whole rule turns on, and it
is the one a reader gets backwards: a fullscreen panel that draws its acts
in its content looks like a presentation owner, and blanking the footer
under it takes away the only statement of its breadcrumb and its `? help`
rather than a duplicate. Saying the occupant is blanked at its own height
rather than dropped is the measurement the layout depends on, and it is
counter-intuitive enough that the tidy version — return the empty string —
reads as correct while it moves every row above the footer twice per
overlay. Saying an unclassified presentation publishes states the safe
direction of the default, since the symmetric-looking choice blanks the
only instructions a new view has. And saying a view-local modal answers
through its own surface is what keeps the shell resolver from growing a
second list of things it cannot see.
The mechanism lives in docs/architecture/presentation-chrome.md.

Fifty-second raise, 62000 to 63400, for a footer that says what happened
and what the keys do at once. 1400 bytes buy four rules a reader would
otherwise have to break to find. The composition order is one of them: the
obvious insert puts feedback below the controls, where it is the terminal's
last row and the controls move every time a message arrives, so naming the
order also names which block is anchored. The surface-owned half is the
rule that replaces a deleted one — the old behavior was to take a
self-drawn footer away and put the message in its place, which looked
correct because the message was visible, and the guidance it silently
spent was the only statement of that surface's way out. Purity is stated
because the measurement and the paint read the same blocks a moment apart:
a one-shot consumed by the measuring pass disappears from the frame that
reserved a row for it, and the bug reads as a render fault rather than as
a measurement that had a side effect. And the single-recipient rule is the
double reservation itself: a bounded panel and its host are both on screen
and both plausibly owe the footer rows, so telling both is the local fix a
reader reaches for, and the frame then ends in a blank band nobody can
trace to either view. The clause about the compensating guard is the other
direction of the same defect, and its exception is the one case the
measurement does not simply replace: the Peek dock's fixed extra row
existed to absorb a footer height nothing measured, and the number that
replaced the guess turned out to be one the dock's own cursor moves,
because the dock advertises paging keys only over a document that can page.
So the guard stays as a floor, and the rule has to say when a floor is
legitimate rather than leave the next reader to delete it and hand a fixed
dock back to the cursor. The mechanism lives in
docs/architecture/presentation-chrome.md; the geometry examples are in
docs/guide/outliner-chrome.md. No rule elsewhere was trimmed to pay for it.

Fifty-third raise, 63400 to 65100, for the context row and the bounded
readout it forwards to. 1700 bytes buy five rules a reader would otherwise
have to break to find. The first is the shared structured description: a
second row and a `Q` block naming the same axes are two renderers, and the
tidy way to keep them in step is to have one read the other's text, which
works until a compact segment is what the readout quotes back. The second
is that the row's height is presence-only — the obvious signature takes a
width, and `ContentHeight` has none, so a width-dependent row is charged
from a guessed width in one place and a real one in another and the list
ends up a row short at exactly the sizes nobody tests. The third is the one
writer: `renderOutlinerEmptyState` builds its own frame, so a header
appended only to the populated path is missing from every empty projection,
which reads as an intermittent row rather than a missed branch. The fourth
is that `queryReadoutBodyRows` may not call `StatusStripHeight` — asking it
is the obvious way to find out what the strip costs, and it is the height
this function is deciding, so the call is either infinite or answered with
a stale number. Saying the clamp is pure and runs at render as well as at
every write is the same rule from the other side: a peel shortens the
document under a window that is already past its end, and clamping only on
the next scroll makes that key look inert. The fifth is that the digits are
the peel vocabulary and not a numbering of the window — renumbering what is
on screen reads as the helpful thing and silently makes `1` remove a
different axis depending on where the reader scrolled. The scroll keys
being consumed at both bounds is stated with them for the reason Peek's
are: a shared binding that declines falls through and scrolls the outline
under a readout the user was reading. The mechanism lives in
docs/architecture/presentation-chrome.md. No rule elsewhere was trimmed to
pay for it.

Fifty-fourth raise, 65100 to 65500, for splitting Peek's one dates line
into what the task still owes and what it already spent. 400 bytes buy
three rules. The first is the classification itself: the body order alone
names two sections and says nothing about which one a new field joins, and
every field here is a date, so the obvious reading is that any date goes on
whichever line is nearer — which is how a deadline ends up under the
history a reader scrolled past it to reach. The second is that history
closes the document, which the order states and the reason does not: the
order is what a diff changes and the reason is what stops the change. The
third is the deferral instant keying both sections — a deferral renders as
a pending field on one line or a lapsed wake on the other, never both, so
keying it once looks correct against whichever half the author had open and
drops it from the fingerprint entirely in the other case, which is a paging
window that survives the value it was keyed on changing. The mechanism
lives in docs/architecture/row-indicators.md. No rule elsewhere was trimmed
to pay for it.

Fifty-fifth raise, 65500 to 66500, for making Peek's dock a share of the
frame. 1000 bytes buy four rules. The first is that one value measures the
dock: the height it replaced was a constant four call sites each read
independently, and the failure a derived body row count invites is exactly
the one that constant made impossible — a render, a clamp, a page distance,
and an availability hint each computing the window from the frame and
arriving at different numbers, which reads as a dock that scrolls past its
own end. The second is the two halves of the clamp and the order between
them: a preferred share nobody capped overflows the frame the moment capture
takes rows, and a cap nobody floored clips live controls instead of putting
the dock down, so the rule has to say both that the cap wins and that a
total under the minimum draws nothing. The third is that the geometry reads
the frame alone — the shape of this bug is a height that quietly depends on
the cursor's own document, which makes moving between two rows move the
list, and the cheapest way to write it is to ask the document how tall it
wants to be. The fourth is the anti-circularity note on
`queryReadoutBodyRows`: the readout is measured inside the same strip the
dock is capped against, so reading the settled geometry there is a recursion
the compiler cannot refuse, and the reason it charges the preferred half is
not derivable from either function alone. The mechanism lives in
docs/architecture/search-filters-and-sorting.md and
docs/architecture/presentation-chrome.md. The retired constants
`inlinePeekWindowRows`, `inlinePeekBodyRows`, `inlinePeekHalfPageRows`, and
`inlinePeekFullPageRows` were struck from the rules they appeared in rather
than left beside their replacements.

Fifty-sixth raise, 66500 to 68000, for moving project identity into a
trailing aligned column. 1500 bytes buy three rules. The first is where the
field now sits, which is not derivable from the old rule: "reserved at every
width" said nothing about order, and the two plausible readings — after the
indicators, or between the title and its cues — are both wrong for reasons
the row cannot show. After the indicators puts the column behind the row's
most volatile field, so it moves whenever a deadline arrives; between the
title and its cues separates a search cue from the title it explains. The
second is that the geometry is measured over the projection and never over a
window or a cursor. That is the whole of why the column holds still, and it
is exactly the rule a later reader is tempted to undo, because measuring the
visible slice is cheaper and looks identical until the list scrolls. The
third is who answers affordability. The column is the right edge of the
title region rather than a position rows are stretched to, so a row that
cannot reach it gets as near as its own width allows and never cuts its
title further to arrive. The two moves a reader reaches for first are both
worse: pulling the column left until the deepest indent and the longest
deadline in view both fit is one row buying every other row's title
shorter, which at the app's own minimum width is most of the row, and
letting the row wrap breaks the one-line invariant every row consumer
depends on. The width budget rising with the row is stated in the same rule
because `projectLabelWidth` is now a floor rather than the value, and a rule
naming only the constant would read as a cap. The mechanism lives in
docs/architecture/row-indicators.md. No rule elsewhere was trimmed to pay
for it.

Fifty-seventh raise, 68000 to 68500, for the band under the current task
row. 500 bytes buy three rules. The first is which rows wear it: the band
says "this is the work you are on", so it is a task row's alone and a
section or project header keeps the bar by itself, which is not derivable
from a rule that only said the cursor is marked. The second is background
precedence, and it has to be written down because two backgrounds already
existed and neither reading is obviously wrong — the overdue tint over the
band loses the cursor on exactly the rows a reader is hunting through, and
the tint under it is invisible, so the rule states that the band owns the
marked row and that the deadline keeps saying so through the badge, the
hero emphasis, and its own text color. The third is why the mark is an
attribute rather than a color: everything else the row wears is a hue, and
a profile that drops every hue would otherwise leave the cursor's column an
ordinary blank, which is the one failure a band makes more likely rather
than less. `textfit.PaintBackground` is named in the same rule because "the
foregrounds survive" is a claim about how the band is laid down, not a
preference: a style applied over assembled text ends at that text's first
reset, so any implementation that does not re-arm silently drops the band
after the first badge. The mechanism lives in
docs/architecture/presentation-chrome.md. No rule elsewhere was trimmed to
pay for it.

Fifty-eighth raise, 68500 to 68700, for the one selection summary. It is a
net raise of 200 bytes against a rule it replaces: the retired sentence said
a view status is transient except the multi-select summary, and deleting it
pays for part of what follows. 200 bytes buy two rules. The first is where a
live selection is stated and where it is not — the context row's entry, one
place, derived from the live rows rather than refreshed by a seam — and the
negative half has to be a rule because the old exception is exactly what a
reader would otherwise restore the moment a count looked stale. The second
is what that one statement may shed: a compact row drops the word
`selected`, never the hidden count, because the number a bulk gesture is
about to act on is the one a narrow terminal must not quietly stop saying,
and the visible half stays in `Q` where a nonzero hidden one gives it
something to be read against. The clamp clause in the same paragraph is a
third, much smaller, spend: `reseatQueryReadoutAfterPeel` running after the
status its peel publishes is not derivable from "the clamp is pure and runs
at every write", since the clamp only ever reduces and the strip is a row
taller while it holds a message, so the ordering is what keeps a reader's
window. The mechanism lives in docs/architecture/presentation-chrome.md.

Fifty-ninth raise, 68700 to 69300, for the Plan/Status division. The
retired half of the Peek sentence — `planning membership` listed fifth
among the status line's fields — pays for part of it, because that
placement was the bug: `today` trailing `priority 1` reads as a tenth
state, which is exactly what a reader restores if the rule only says the
field is present. 600 bytes buy three rules. The first is the vocabulary
itself, and it has to be a rule because it is a negative one: four
surfaces spell these two words and nothing in any one of them stops a
fifth from minting `Lane` or `Collection`, after which the division is
noise rather than a distinction. The second is that grouping is metadata
over the existing binding — the whole point of the session was that no
operation identity, sequence, or dispatch path moved, and a contributor
reading only the rendered row would reasonably conclude the chord gained a
second key list. The third is that a heading rides its run's first entry,
which is not derivable from the packing rules already stated: those say
required entries are reserved first and optional ones added by priority
until one does not fit, and under exactly that policy a heading modeled
as its own entry outlives every member beneath it. The mechanism lives in
docs/architecture/presentation-chrome.md and
docs/architecture/operation-grammar.md.

Sixtieth raise, 69300 to 69900, for the hosted-panel budget. The retired
clause — `HostRows` being pure *because* `OutlinerModel.SetSize` drops
`ViewScrollOffset` — pays for a little of it, and it had to go: it was the
justification for the loss, so a reader who kept it would reasonably
conclude that a panel costing its host a scroll window is by design. 600
bytes buy three rules. The first is that the rows a panel takes are a loan
rather than a resize, and which call is which, because the shell has one
act to perform and the two spellings are indistinguishable from a host's
signature alone — a surface told `SetSize` for a budget cannot tell a
terminal apart from a panel, which is the whole defect. The second is the
anchor's ladder, top task row then cursor then the raw offset: a
contributor who knows only that the window is preserved will preserve the
number, and the number is wrong precisely in the case the panel exists for,
since capture commits work into the projection underneath it. The third is
the negative one — the seam writes `ViewScrollOffset` and nothing else — so
that keeping the cursor visible stays `ComputeOutlinerViewport`'s, which is
the rule this paragraph would otherwise quietly carve an exception out of.
The mechanism lives in docs/architecture/capture-and-outline-editor.md and
docs/architecture/presentation-chrome.md.

Sixty-first raise, 69900 to 70500, for the content-sized capture panel.
The retired clause — `HostRows` being a pure function of the frame, never
of the draft — pays for most of it, and it had to go: it is now false, and
a contributor holding it would read the measurement as the defect rather
than as the rule. 600 bytes buy three rules. The first is what the height
now is, floor and ceiling included, because "sized by its content" alone
does not say that a blank panel still keeps its input row or that a long
draft stops growing, and either omission is a panel that eats its host.
The second is that every demand term reads settled state: a measurement
that re-parsed the line, or read the panel's own height, is a feedback
loop no test of one frame catches, and the pane-replacement case is the
one that already bit — measuring an open syntax guide as nothing shrinks
the share and costs the host the window session 60 bought. The third is
that the divider names the act alone, since the destination it used to
spell is the route summary's and a second copy goes stale the moment
`Ctrl+P` rewrites that row. The mechanism lives in
docs/architecture/capture-and-outline-editor.md.

Sixty-second raise, 70500 to 71500, for the applied-first tag picker. 1000
bytes buy three rules. The first is when the order may be computed: the
obvious place is the source that builds the rows, and a source cannot know
what a bulk selection actually holds, so a picker ordered there promotes
whatever the first task carries and reads as correct on every single-task
test. Naming the call site as after `ConfigureValueMembership` is the whole
of it. The second is that promotion is presentation and not identity:
moving a row by rewriting its `CategoryID` is one field and needs no new
seam, and it silently makes `s`, `c`, and `t` walk past the very values the
user already applied — a category jump that skips applied rows is a bug
nobody reports, because the section it lands in still has rows in it. The
third is the freeze, and it is a negative rule: recomputing the order on
every membership change is what a reader assumes a "sorted by applied"
picker does, and it moves the row under the cursor on the keypress that
toggled it, so ticking three tags in a row requires re-finding two of them.
The clear-context exclusion rides along because `TaskContextMembership`
marks that row all for every task with no context, so the honest reading of
"promote every all value" puts `No context` at the top of a picker whose
whole job is the values you already chose. The mechanism lives in
docs/architecture/selection-and-project-tags.md. No rule elsewhere was
trimmed to pay for it.

Sixty-third raise, 71500 to 72400, for non-interrupting check-in feedback.
900 bytes buy four rules, and the first is the negative one the whole
session exists for: no commit opens the scheduling draft. That cannot be
read off the code, because what a contributor finds there is committed
evidence with a consequence attached, and reattaching the editor to it is a
two-line change that every existing test still passes — the offer was
removed as a design decision, not as a defect. The second is what the note
may name: the keyless menu entry and never `@` or `Space`, since those read
the cursor and a committed task has usually moved lanes by the time the
line is read, so a helpful-looking key hint is a lie about the target. The
third is where suppression lives: once per transaction ID, so that a reload
or a redraw is not a second offer — the obvious implementation, keyed on
task identity or on the projection, re-announces on every rebuild and looks
correct on any single-commit test. The fourth is that riding an existing
line advances no `seq`, which is what keeps the handler's own severity and
flash decision intact; a note that published itself would retire the very
warning it appended to. The clause paid for it is the one saying an
explicit save attaches a strict read-set precondition, which is now false:
that guard existed only to keep an automatic save from replacing a schedule
made by hand, and a concurrent write is still refused by the project's own
`ExpectedRevision` contract. The mechanism lives in
docs/architecture/operation-grammar.md.

Sixty-fourth raise, 72400 to 73300, for the first-task launch position.
900 bytes buy four rules a reader cannot derive from the code. The first is
where the intent is armed: the launch message and not `ResetToGlobalAll`,
which that message calls and which every reload, mode change, and Esc
restoration also calls — arming it there is the obvious one-line change,
it passes every startup test, and it silently drags the cursor back to the
top of the list on every refresh. The second is that both population flags
are read: `SnapshotReady` alone cannot tell an empty workspace from a read
that has not landed, and `Loading` alone is what a saved lens still reading
its own projects leaves set, so dropping either half either answers for a
projection that does not exist yet or pre-empts the lens that is about to
replace it. The third is the keystroke drop, which is the whole of "never
override newer navigation": there is no other seam where the shell learns
the user has taken the cursor before a slow startup lands. The fourth is
the negative one — `FocusFirstLaunchRow` writes `m.Cursor` alone — because
seating a cursor on a row a fold hides reads as an invitation to open that
fold, and `ComputeOutlinerViewport` is still the one window authority. No
rule elsewhere was trimmed to pay for it.

Sixty-fifth raise, 73300 to 74200, for the footer's rank ladder. 900 bytes
buy three rules that `FooterRank: 4` in a descriptor cannot state. The
first is that the ladder is one ordering over the whole grammar rather
than a per-surface opinion, so a surface that wants a key forward has to
move it for every surface or leave it alone; without that, the obvious fix
for "my view should lead with X" is a second ranked description, which is
exactly the drift `A footer has one source` forbids. The second is the
sizing, and it is the one nothing in the code says out loud: the six
leading entries and their separators come to exactly
`chrome.MinTerminalWidth`, so a contributor who lengthens `e edit` to `e
edit line`, or slips a seventh act in ahead of `? help`, silently drops
help off the smallest supported terminal and no wide-terminal test
notices. The third is the negative one: rank zero excludes a binding from
the footer and from nothing else, because the menu bar and the generated
`?` help project from the registry regardless — a reader who takes zero as
"hidden" either ranks keys that do not need it or believes an unranked
operation is unreachable. The mechanism lives in
docs/architecture/operation-grammar.md. No rule elsewhere was trimmed to
pay for it.

Sixty-sixth raise, 74200 to 74334, for the macOS app's recipes. 134 bytes
buy one rule: `just mac-test` and `just mac-install` stay outside `just
ci`, as `just cross-machine` does, because they need Xcode and Zig. A
contributor who reads only the build list would otherwise fold them into
the gate and make `just ci` unrunnable on Linux and on any Mac without
Xcode. How the app builds lives in macos/README.md. No rule elsewhere was
trimmed to pay for it.
Caret-aware capture/help and bounded feedback recall add two rules;
their mechanisms remain in the architecture guide.

Raise from 74488 to 74835 for the footer's shed order. 347 bytes buy two
rules nothing in the code says out loud. A pending chord is the one spec
asking for the packer's third row, and `HintKeep` ranks `wd` with the head
of its own run rather than first, since an entry kept ahead of the one
carrying `Status:` would be drawn under `Plan:`. And capture's footer
sheds by priority while drawing in declaration order, so a contributor
reordering the hints to change what a narrow row keeps moves entries and
changes nothing about which one goes. The mechanism lives in
docs/architecture/presentation-chrome.md. No rule elsewhere was trimmed to
pay for it.

Raise from 74835 to 75108 for what a short frame gives up first. 273 bytes
buy two rules of sacrifice a contributor would otherwise get backwards. A
feedback line wider than its row loses its head and keeps `u undo` and
`Ctrl+O origin`, which one helper spells, so a new ending added inline at
a splice site would be the one the fit cuts. And a tag picker short of its
list's rows drops its legends before a row and keeps the jump hint, the
reverse of the reservation it replaces. The mechanisms live in
docs/architecture/operation-grammar.md and
docs/architecture/selection-and-project-tags.md. No rule elsewhere was
trimmed to pay for it.

Raise from 75108 to 75280 for the row's fold column. 172 bytes buy two
facts about the one row geometry: a projection holding anything foldable
keeps that column blank on every other task row and a nil layout keeps
none, so a renderer padding a leaf itself would misalign the inline
editor, and the ranked label's slug cap has one source, which is what
stops a second default returning to the badge measurement. The mechanism
lives in docs/architecture/row-indicators.md. No rule elsewhere was
trimmed to pay for it.

Raise from 75280 to 75413 for Peek's one label grammar. 133 bytes buy the
rule a new Peek line is held to: every labeled line is `Label: value`
joined by a middot, the two boolean axes are bare capitalized flags, and
a list's count follows its colon rather than preceding it, which is what
stops a fourth label style returning beside the three this replaced. The
mechanism lives in docs/architecture/search-filters-and-sorting.md. No
rule elsewhere was trimmed to pay for it.

Raise from 75413 to 75662 for capture saying its destination once. 249
bytes buy the rule the pane under the route row is held to: the `To:`
counts belong to a batch spanning projects and one predicate answers the
measurement and the render, and the preview's Route group is omitted only
by a host that draws the route row, so the shared preview cannot repeat
that row in the composer or lose it elsewhere. The mechanism lives in
docs/architecture/capture-and-outline-editor.md. No rule elsewhere was
trimmed to pay for it.

Raise from 75662 to 76209 for the pinned lane context row. 547 bytes buy
the rule the one change inside the window authority is held to: the
screen-row count is unchanged while the reported window is one row
shorter, so a reader of the window that recomputed either would seat the
cursor under the pinned row, and the lane is found from the row's group
and a recorded index rather than by the upward scan the group invariant
forbids. The mechanism lives in docs/architecture/presentation-chrome.md.
No rule elsewhere was trimmed to pay for it.

Raise from 76209 to 76496 for one footer on fullscreen surfaces. 287
bytes buy the rule a surface replacing its host is held to: it publishes
its keys through the guidance provider and draws no footer of its own, so
the frame has one divider, the breadcrumb's exit and help become the
surface's own entries, and the per-frame override is left to the expanded
form's inline overlay alone. The mechanism lives in
docs/architecture/presentation-chrome.md. No rule elsewhere was trimmed
to pay for it.

Raise from 76496 to 76544 for row columns that hold across folds. 48
bytes buy the clause the row layout is now held to: the measurement
covers rows a fold hides, so a builder adding a collapsed branch owes the
accumulator what it declined to emit. The mechanism lives in
docs/architecture/row-indicators.md. No rule elsewhere was trimmed to pay
for it.

Raise from 76544 to 76680 for one-task project groups merging into their
row. 136 bytes buy the exception to "every group opens with its header":
which groups still draw one, that the merged row leads with the ranked
list's label, and that project keys wait for the header. The mechanism
lives in docs/architecture/tui-architecture.md. No rule elsewhere was
trimmed to pay for it.

Raise from 76680 to 76980 for capture creating its missing project. 300
bytes buy the rule the offer is held to: only the composer asks, only
when every refusal is an unknown slug already canonical and unheld, and
a yes is one transaction and one undo entry covering the projects, the
catalog, and the tasks, so no surface may create the project in a commit
of its own first. The mechanism lives in
docs/architecture/capture-and-outline-editor.md. No rule elsewhere was
trimmed to pay for it.

Raise from 76980 to 77480 for desktop draft ownership, content-aware
quit, and the explicit Peek size preference. Detailed protocol and
history behavior lives in docs/architecture/tui-architecture.md.

Raise from 77480 to 78320 for the stored Hidden flag. 840 bytes buy the
rules the flag is held to: it is orthogonal to every other axis and writes
`UpdatedAt` alone, it is inherited through one predicate while only the
toggled task stores it, it is excluded at read time unless a positive
hidden clause opens it, attention readers never admit it, and no edit
surface may drop it because it has no sigil; plus where `fH` and `wh` sit
in their chords. Behavior lives in docs/guide/search-and-lenses.md and
docs/guide/storage.md. No rule elsewhere was trimmed to pay for it.
