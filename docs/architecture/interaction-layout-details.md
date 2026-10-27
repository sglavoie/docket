# Interaction layout details

Layout contracts for capture forms, contextual guidance, and task rows.
User-facing operations are documented in [the guide](../guide/README.md).

## Expanded form viewport

Every field on every tab is reachable at any supported height. When a tab holds
more than the terminal can show — a long checklist, a dozen attachments, a
comments list — the form scrolls instead of cutting: `Tab` and `Shift+Tab` move
the focus as usual and the window follows it, and the last row reads `3 above · 1
below` so what is off screen is named rather than lost. A tab that already fits
shows no such row. A refused save keeps its message on screen and moves the focus
to the field it names, so the two stay readable together, and a taller terminal
simply gives the rows back.

## Expanded form width

Every field is reachable at any supported width too. The expanded composer opens
wherever the rest of Docket opens — down to the same 60-column minimum every other
surface accepts — rather than replacing itself with a "terminal too small"
notice on anything under 68 columns. A narrow terminal shortens what a row can
spare and nothing else: the grey key hints beside a focused row go before its
value does, and a value too long for the row is shortened there and read whole by
pressing `Enter` on it, which opens it in its own editor. The nested pickers —
priority, state, flags, due date, the check-in editor, and the checklist,
subtask, attachment, and comment editors — take the whole screen while they are
open, so at every width they keep a complete frame and a complete list; `Esc`
hands the form back exactly as it was.

## Guidance ownership

Only one set of shortcuts is on screen at a time, and it always belongs to
whatever your next key press reaches. Open a picker, an action sheet, a lens
list, the check-in editor, or the conflict browser over the outline and the
outline's own row of shortcuts goes quiet: it is describing keys that picker has
taken. The same is true of a confirmation, and of a picker opened inside the
expanded capture form. The row does not collapse
while it is quiet — the space stays reserved, so nothing on screen jumps when
the picker opens — and closing the picker brings the outline's shortcuts back
exactly as they were.

## Chord guidance

Contextual guidance is made of complete `key action` entries. Browse states use
one row; editing, adding, confirming, autocomplete, and expanded detail may use
two, and a pending chord may use three, which is what lets `w` name all ten of
its destinations, Done included, at the narrowest terminal. The `f` and `g`
rows are divided the way `w` is, by a heading riding the first entry of each
run: `f` reads `Field:` over the values a task stores and `Derived:` over the
facts worked out from it, with `x clear all` and `% projects` closing the row
outside both, and `g` reads `Mode:`, `Task:`, and `Sync:` behind `g top`. A row
too narrow for all of it never draws a heading without its first entry. At the
narrowest terminal `f` keeps `x clear all` and gives up `a completion age` and
`% projects`, and `g` over a task row gives up the `Sync:` pair whole; every
shed key still works and `?` help still lists it. Required commit and cancellation controls survive before
optional hints, and they survive whole at the narrowest terminal Docket runs in,
so the way out of a state is never half a sentence. Transient pickers and action
sheets own their local guidance while leaving the host title and workspace pulse
visible.

## Fold-column alignment

Wherever one task in the list can be folded — it shows a child, or it has a
checklist — every task row keeps the two columns the `▾`/`▸` glyph takes, blank
on a row with nothing to fold, so state glyphs and titles line up down the
list. A list in which nothing folds keeps those columns for its titles, and so
does a terminal under 60 columns. The reference count stands two columns clear
of the title, the gap the deadline and review chips keep.


## Chrome layout and interaction contracts

The following details are maintained here rather than in the everyday guide.

Every full-screen presentation keeps one operational title row. It names the
surface, scope, and mode once on the left, and reports the workspace pulse on the
right as `workspace: inbox 3 · today 2 · overdue 1 · done 4`. A component at zero
is left out, and a pulse with nothing to report is left out whole rather than
leaving a bare `workspace:` behind. Narrow terminals drop Done, Inbox, overdue,
then Today before shortening required interaction context. A terminal wide
enough to hold all of that whole follows the mode with its two Tab neighbours,
muted — `Inbox  ‹ Today  Next ›` — so the title shows where `Shift+Tab` and
`Tab` go; anywhere narrower the active mode stands alone.

What is narrowing the view gets a second row, immediately under that one and
above the list. It appears only while some axis is live — a selection, a lens, a
search, filters, a sort you chose, a focus, or a section zoom — so a resting view
spends no row saying that nothing is happening. Each axis is one segment, and a
terminal too narrow for all of them drops whole segments from the right rather
than cutting one in half, ending in `+2 more · Q details`: the count is what it
could not draw and `Q` is where the whole of it is written out. Press `Q` and
every one of those values is there in full, wrapped rather than cut, in a
readout you can scroll.

`M` puts a menu bar above that title row: a single band reading
`Task  View  Go  Find  Help`, one label per menu. `M` again takes it away. The
bar costs exactly one row of the task list while it is shown, which is the whole
reason it is off by default and the whole reason it is a key rather than a
permanent fixture. A terminal too narrow for every label drops whole menus from
the right — Help first, then Find, then Go — rather than cutting a word in half,
because a half-word is a lie about what the bar offers while a dropped menu is
still reachable by the keys its entries carry.

`F10` or `Ctrl+M` opens the leftmost menu, borrowing a row for the bar if it was hidden,
so reaching a menu is never two presses. The reveal lasts as long as the menu
does: close the menu and the bar goes back to whatever `M` last left it as, up
if it was up and away if it was away. The dropdown falls inline under the label
it belongs to, and the view stays underneath it: the bar stays on its row, the
title and its workspace pulse stay on theirs, and the task list — with an open
Peek dock, a multi-select summary, whatever you were looking at — is still there,
dimmed, everywhere the box does not reach. The box covers those rows rather than
pushing them anywhere, so nothing shifts down and back up per press and the view
you return to is the one you left, scrolled where you left it. A menu too tall
for the room under its label scrolls inside its own box. `h`/`l` or left/right move along the bar, `j`/`k` or up/down move within
the open menu and step over the rules between groups, `Enter` runs the
highlighted entry and closes the menu, and `Esc`, `F10`, or `Ctrl+M` closes it without
running anything, leaving the bar exactly as it was.

A row with no key beside it has none anywhere, and the menu is the only place it
exists. The View menu's **Clear All Folds** is the first of those: `s`, `p`, and
`t` each toggle one fold axis and each reverses, so opening a tree that is folded
in three different ways takes three presses that may each go the wrong way, while
this opens all of them once and never folds anything. It says `All folds cleared`
or `Nothing is folded`, since the dropdown closes over the rows it changed.
**Advisories** is the other, ticked while advisories are shown; it is described
with the status strip below. An
operation reachable only this way never touches a task or a project — it acts on
the view or on nothing at all — so the bar can stay off by default without hiding
anything that edits your work.

Today mode also compares its open load with the workspace's 30-day completion
pace in the middle region, for example `9 open · median 3 done/day (30d)`. A fresh
workspace with no completion history shows no comparison. Under pressure the
workspace pulse disappears first; the comparison then sheds `(30d)`, then
`median`, and finally disappears before required interaction context shortens.

Press a chord prefix such as `w`, `f`, or `g` to see its next keys.
`w` separates Plan from Status; `?` lists all shortcuts, including those a
narrow footer cannot show.

The footer always describes the keys for the current interaction. Closing a
picker restores the outline’s shortcuts.

A panel that takes over the whole screen is a different case, because nothing is
left underneath it to go quiet. Comments, attachments, the project creator and
editor, and the expanded capture form each have one footer: a single divider
with that panel's own shortcuts under it, on the last row of the terminal. The
way out is among them, and on Comments and Attachments so is `? help`. On a
narrow terminal the shortcuts wrap onto a second row rather than drop the ones
you cannot reach any other way, and the panel gives up a row of its content to
make room.

A message is not a set of shortcuts, so a status line or an error still reaches
that row with a picker open: it is telling you what just happened rather than
what to press next. An unfinished chord, likewise, keeps its `w …` row, since
those are the keys really waiting for you.

When an operation finishes, its result and the keys never trade places. The
footer keeps its controls on the bottom row of the terminal and grows one row
upward to carry the message, so `✓ Copied title to clipboard` sits directly under
the divider with `Enter create · … · Esc cancel` still beneath it. Nothing that
was usable a moment ago goes unnamed because something succeeded or failed. A
refused save is the case that matters most: the draft is still there, the error
says why, and the keys that would fix or abandon it are still spelled out at the
smallest supported terminal.

The row a message costs comes out of the view underneath, once. A 35-row terminal
resting on the outline spends one row on the divider and one on the controls,
leaving 33 for the list; a message standing leaves 32. Open capture over that
same outline and it is the panel, not the outline under it, that gives the row
back — so the list behind the panel does not shuffle when a message arrives, and
the panel never overlaps the footer it grew. With Peek open the list has already
set a row aside for a footer the dock's own paging keys can wrap, so an ordinary
one-line message costs it nothing further and the dock stays exactly where it
was: no message and no cursor ever moves it.

Surfaces that draw their own footer — Help, Notes, the Hub, Recent Messages —
keep it. The message is placed directly above that footer's divider rather than
over its keys, and the frame stays exactly as tall as the terminal. Everywhere
else, the comments panel, the project editor, and expanded capture included, the
message sits between the one divider and the shortcuts.

The Outliner's own feedback — a focus notice, or a line such as
`Latest activity: …` — renders in a status strip pinned directly above the
footer, introduced by a dim thin divider. The strip never floats mid-screen on a
short list and never leaves blank padding under itself: it takes its rows from
the task list, and in a narrow terminal it wraps rather than pushing the footer
off screen. Ordinary shell and view feedback stays for two to six seconds,
depending on text length; undo feedback keeps its six-second window. A save
failure with a retained draft stays visible until that field is edited or the
draft is saved or discarded. Cursor movement, timers, and typing in covering
Help do not dismiss the editor's failure. The strip
clears on a
mode change, so a message naming a mode can never misreport the one you switched
to. A live visual selection is no exception, because the strip never carries one
in the first place: the selection states itself on the second header row, once,
and the strip is left free for the feedback your next bulk gesture writes there.

`docket --control PATH` serves a Unix socket at `PATH`, readable by your user
alone, for a host app such as Docket.app. Over it the session reports the pulse
whenever it changes, the agenda — the overdue and Today tasks behind those
counts, overdue first — whenever that changes, the schedule — the arrivals still
to come within the next eight days, each with the instant it will happen, so a
host can book them with the system ahead of time — whenever that changes, and
each tick's arrivals, with the identity of every task it names, and it takes
seven requests: show a task, acknowledge one as `R` does, mark
one Done as `wd` does, defer one by some minutes, capture a line as the `a`
composer would read it, refusals included, ask what that composer would suggest for a draft up to its caret,
and preview the project, plan, status, and deadline of a full capture draft.
Each is answered as though you had pressed the key in Docket: through the same
commit and the same undo entry, so `u` takes it back. A request that changes
something is refused, never queued, while you are inside a picker, an editor, a
selection, or another view — completions and previews change nothing, so they
are answered even then — and a session that lost focus first adopts any change made on disk since,
so what it commits is never against a stale read. Docket.app starts Docket this way, and
falls back to `--notify` for a Docket too old to have the flag. `--headless`
runs that same session with no input, no renderer, and a fixed 120×40 size,
blurred from the start so every request adopts what changed on disk first;
it is how Docket.app keeps answering after `q`, and it quits when its parent
process changes, since an orphan would hold the lock the next session needs.

The strip is also where `Q` expands the narrowing readout: one numbered row per
narrowing the view is under, then an `Also in view` block for the axes nothing
peels — your selection, the effective sort, a focus, a section zoom — with the
conflict indicator keeping the lead above it all. It grows the strip rather than
floating over the list, so the rows it pushes off are rows the strip was already
entitled to, and it never takes the last one.
[`search-and-lenses.md`](../guide/search-and-lenses.md) has the readout and its peels.

After a commit, recovery feedback advertises `u undo` only when the newest
workspace-ledger entry is that exact transaction. A task relocation adds
`Ctrl+O origin` only after its origin was recorded; `Alt+→` returns to the
relocated task. When that line is wider than the terminal, the sentence is
shortened with `…` and the keys stay whole at its end, so a narrow window never
hides the way back.

Undoing or redoing tells you where the work went. When the entry you took back
was aimed at one task — a state change, a reorder, an indent, a row edit, a
delete — and the restored task is drawn in the view you are standing in, the
cursor moves onto it and the message ends in `Ctrl+O origin`, so one key returns
to where you were. When the view does not draw it, nothing moves: your mode,
scope, search, filters, and folds stay exactly as you left them and the message
names the destination instead, as `now in Design (Done)`. An entry that was aimed
at many tasks at once, or at a project, says neither and moves nothing, so a
selection you are still holding survives its own undo. The correlation is
session-only: after restarting Docket, undo still restores the same work but never
moves the cursor.

It is muted, closed with a rule, and never the cursor's: it is not a row, so
`k` from the row under it scrolls the list rather than landing on it, and the
real header is still where `gg` or `N` finds it. It costs one list row while it
is drawn and none otherwise — a window that opens on a lane header, or one that
has not scrolled, shows the header itself. The flat `/` result list and a view
scoped to one project draw none.


## Date chip emphasis

Due dates and arrived check-ins both render as a filled chip rather than as colored
text, so a date reads as one bounded object at the end of a row the title now
owns. All three tiers are chips — overdue, today, and
upcoming — and they share one hue that no section header uses, so a date is
never mistaken for a section; urgency is carried by how saturated the fill is
rather than by its hue. The two tiers that demand an answer — overdue and today
— are also emphasized: they are bold, and their text is pushed away from the
fill rather than toward it, so it is lighter than the chip in the dark theme and
darker than it in the light one. An upcoming date stays quieter on both. Under
the cursor the chip keeps its own colors like every other colored segment, and
under `--no-color` the fill drops away, the bold survives, and the `⊡` glyph
marks the date on its own.



## Compact capture row allocation

Capture opens as a panel across the bottom of the screen rather than a view that
replaces what you were looking at. The outline stays above it, live and scrolled
where you left it, so the row you were on — and its Peek dock, if one was open —
is still readable while you type. The panel leads with a divider naming the act
(`── Capture ─`), states the effective route on one row, and holds the input.
A blank capture is exactly those three rows; the panel then grows a row at a
time as the draft, the preview of what `Enter` would create, a suggestion list,
or a refusal needs one, and shrinks again as they go away, up to a ceiling of
fourteen rows and never below the four the outline keeps.
Once the panel is at its ceiling — or on a short terminal, where the ceiling is
whatever the outline can spare — it keeps its shape and shortens the preview
instead of its chrome, and a refusal keeps its rows ahead of the draft's, so the
message that says what went wrong is never the thing that scrolls away. Rather
than show the first few labelled groups and silently drop the
rest, it falls back to one wrapped run — `Panel capture · ◌ $inbox · !2 · #panel`
— so nothing the parser recognized goes unnamed. At the 60×20 floor the outline
keeps enough rows to show the row the cursor is on; capture never hides it.


The capture footer has two rows, and a narrow terminal cannot hold every entry
in them. `Enter create` and the `Esc` entry always stay. The rest are kept in
this order, whatever order they are drawn in: `Ctrl+N create another`,
`Ctrl+E create & edit`, `? syntax`, `Ctrl+P project` when there is a second
project to switch to, the `Ctrl+F` entry, `Alt+Enter new line`, and last
`Ctrl+V paste`, which is the first to go because the terminal's own paste
already works. A key the footer shed still answers.
