# Presentation grammar

What the Outliner draws around the task list: the title row and its workspace
pulse, the menu bar, contextual guidance, the status strip and the advisories it
carries, the conflict browser, how the cursor row is marked, and the colors a
tag wears.

In `?` help, `/` searches within the selected category tab. Each search word
may match the key, description, category, or an alias of the same entry, so a
category and an action can be searched together. Every word must match that
entry; words never combine across different entries. While searching,
each tab shows the number of matches selecting it would reveal, including its
own case-insensitive fallback. If that category has
no matches but others do, the empty view names the category and counts the
matches available elsewhere. Choose the All tab to see them with the same
query; the search never switches categories on its own. Help uses one column in narrow
windows and two when each description has at least 24 columns of room. Search
highlighting follows matches across wrapped description lines. When a search
matches an alias, that alias stays beside the description and wraps as needed
so the reason for the match remains readable.
An exact key, description, or alias match appears first under `Exact matches`;
the other results retain their category order. Clearing the search restores the
ordinary grouped reference, and category tabs still constrain both kinds of match.

Help remembers its query, category, and scroll position separately for each
surface until you quit `tt`. Press `?` to close a filtered reference and reopen
it later in the same place; `Backspace` clears the query and returns to All.
Available actions refresh on every opening. If a category has disappeared,
Help returns to All, and shorter results or a resized window clamp the scroll.

Every full-screen presentation keeps one operational title row. It names the
surface, scope, and mode once on the left, and reports the workspace pulse on the
right as `workspace: inbox 3 · today 2 · overdue 1 · done 4`. A component at zero
is left out, and a pulse with nothing to report is left out whole rather than
leaving a bare `workspace:` behind. Narrow terminals drop Done, Inbox, overdue,
then Today before shortening required interaction context.

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

A menu lists only what the cursor could do right now: an entry that does not
apply is removed rather than greyed out, so the Task menu is shorter under a
visual selection than without one. The Task menu offers **Add Checklist Item**
(`c`) when the cursor names a task or one of its checklist items. Modes and
toggles carry a checkmark — the
active mode, whether the bar is kept — so a menu reads the state back instead
of only offering to change it, and every row spells the key it is bound to.
Those keys keep working while the menu is up: pressing one the open menu does
not list closes the menu and does what the key does, so nothing is trapped
behind a dropdown.

The Task menu also offers Undo and Redo with the next recorded action in their
labels, such as `Undo: Set Priority: Ship release`. These describe the same
workspace-wide history that `u` and `Ctrl+R` restore, including after a restart.
The descriptions refresh each time the menu opens; reading them changes nothing.
An action with no available history is omitted from the menu.

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

Terminals that do not report enhanced keyboard events encode `Ctrl+M` as
`Enter`, the way they encode `Ctrl+I` as `Tab`. Use `F10` to open and close the
menu in those terminals. `M` still shows the bar, and `Ctrl+M` over an open menu
runs the highlighted entry rather than closing it.

Whether the bar is drawn is remembered for this machine, the way the light or
dark theme is: it survives quitting and reopening `tt`, and it does not follow
you to another computer that opens the same workspace. Chrome belongs to the
terminal you are sitting at. The View menu lists the toggle itself, checked when
`M` is keeping the bar rather than when a menu is only borrowing its row,
because the key that shows the bar is the one thing you cannot read off the bar
once you have found it by accident.

Today mode also compares its open load with the workspace's 30-day completion
pace in the middle region, for example `9 open · median 3 done/day (30d)`. A fresh
workspace with no completion history shows no comparison. Under pressure the
workspace pulse disappears first; the comparison then sheds `(30d)`, then
`median`, and finally disappears before required interaction context shortens.

Contextual guidance is made of complete `key action` entries. Browse states use
one row; editing, adding, confirming, autocomplete, and expanded detail may use
two, and a pending chord may use three, which is what lets `w` name all ten of
its destinations, Done included, at the narrowest terminal. Required commit and cancellation controls survive before
optional hints, and they survive whole at the narrowest terminal `tt` runs in,
so the way out of a state is never half a sentence. Transient pickers and action
sheets own their local guidance while leaving the host title and workspace pulse
visible.

Only one set of shortcuts is on screen at a time, and it always belongs to
whatever your next key press reaches. Open a picker, an action sheet, a lens
list, the check-in editor, or the conflict browser over the outline and the
outline's own row of shortcuts goes quiet: it is describing keys that picker has
taken. The same is true of a confirmation, and of a picker opened inside the
expanded capture form. The row does not collapse
while it is quiet — the space stays reserved, so nothing on screen jumps when
the picker opens — and closing the picker brings the outline's shortcuts back
exactly as they were.

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

The `Esc` entry names what the next press does, not what you are in. Standing in
a Focus inside a lens it reads `Esc leave Focus`, because that is the layer the
key takes first; once the Focus is gone it reads `Esc leave lens`. Open Peek and
one entry covers both keys that close it, `b/Esc close Peek`. Start a range and
it reads `Esc freeze range`, since freezing is what that press does and clearing
is the one after. Search typed under a lens reads `Esc restore lens` rather than
`Esc clear search`, because the lens is what comes back. The empty-state
sentences use the same words — `Press Esc to unzoom`, `Press Esc to leave
Focus` — so the footer and the explanation never disagree.

Anywhere you can type, that entry answers for your draft. An untouched composer,
form, comment box, or editor reads `Esc cancel` and leaves at once. Type
something and it becomes `Esc discard`, and the press asks first: `Y discard ·
N/Esc keep editing`. Declining keeps every character and leaves you in the
editor, which is why that question is worded apart from the `y confirm ·
n/Esc cancel` used to confirm a deletion.

Pending chord hints dim both the key and label when an action would currently
do nothing or be refused: for example, conflicts when none are outstanding,
an already-selected mode or priority, an unavailable workflow transition, or
clearing empty filters or sort settings. Emphasis is recomputed on every render;
dimmed shortcuts remain usable. Toggles and Recent's refresh stay prominent.

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

Surfaces that draw their own footer — Help, Notes, the Hub, Recent messages —
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
depending on text length; undo feedback keeps its six-second window. The strip
clears on a
mode change, so a message naming a mode can never misreport the one you switched
to. A live visual selection is no exception, because the strip never carries one
in the first place: the selection states itself on the second header row, once,
and the strip is left free for the feedback your next bulk gesture writes there.

`F10` → Help → Recent messages opens the latest feedback in a read-only,
scrollable viewer, even after it has expired or a mode change has cleared it.
The viewer keeps the latest 20 messages for this session, including operation
errors and transient view feedback. Left visits older messages and Right returns
to newer ones; `Message 2 of 8` identifies your position, with 1 the newest.
Each entry shows its original local time (`14:32`), including the date when it
is from another day, and whether it is feedback or an error. Adding details
preserves that time. Up/down scroll the displayed
message; browsing away and back restores its scroll position, clamped to the
current window size. `y` copies the message text in full; `Y` copies all retained
messages newest first, with full local timestamps (including UTC offsets) and
message kinds. `Esc` returns to the outline. Ambient advisories and copy notices
do not enter the history.
Press `/` to filter messages by text or kind, case-insensitively. Matching
characters are highlighted in the message and its kind; copied text stays plain.
`Enter` leaves
the input so Left/Right can browse just the matches. `Esc` clears the filter and
restores the message and scroll position you started from; while typing, another
`Esc` leaves the input. `Y` still copies the whole retained history, including
messages outside the filter. While filtered, its hint names the full count
(`Y copy all 20`), and successful copying reports how many copied messages were
hidden by the filter. Searching changes neither timestamps nor messages.
After accepting a nonempty search with `Enter`, `Ctrl+Y` copies only its matching
messages, newest first, with the same full timestamps and kinds as `Y`. The hint
appears only when there are matches, and the result reports how many were copied.
Details added to an existing message update that entry. Recovery hints in the
recalled text are historical text, not controls in this viewer.

The strip carries one line that is not feedback at all: while the workspace
holds unresolved merge conflicts, an indicator counting them leads the strip and
names `tt conflict list`. It is workspace state rather than a message, so it
neither expires nor clears on a mode change; it leads the transient lines so
they cannot shift it, and it disappears only once the last bundle is resolved.
See [`storage.md`](storage.md) for what a bundle is and how to resolve one.

That indicator is an *advisory*: a line the workspace puts on screen because of
what it holds, not because of a key you pressed. The launch-time
`N projects failed to load` notice is the other one. Advisories can be turned
off, together: launch with `tt --no-advisories` for one session, or tick
`View → Advisories` off to make the choice stick on this machine. A hidden
advisory is absent rather than dimmed, and the row it occupied goes back to the
task list. Nothing else goes quiet — feedback from your own operations, the `Q`
readout, and the message that explains a workspace where *every* project failed
to load all still speak, because none of them is ambient. The conflicts
themselves are untouched: hiding the count resolves nothing, and `gc` and
`tt conflict list` still find every bundle.

### While tt is in the background

tt names its window after the workspace pulse — `tt · inbox 2 · today 3 ·
overdue 1`, nonzero counts only — so a terminal tab or title bar reads it
without switching to tt; a view that names its own title keeps it, and quitting
clears it.

In a terminal that reports focus, tt notes the workspace on disk when it loses
focus and compares on return. If another machine's sync, or another `tt`,
changed it meanwhile, tt reloads the catalog, planning, and every project, and
says `Reloaded: the workspace changed on disk`. It waits while you are inside a
picker, an editor, a visual selection, a ranked search, or another view, and
reloads on the next minute's tick after you leave it, so nothing you are acting
on is renumbered under you. A reload writes nothing.

`tt --notify` also sends a desktop notification (OSC 777) when work arrives
while the session runs: a check-in whose day comes, a timed deadline that
passes, or a deferral that ends. One arrival names its task; several are
counted, `2 check-ins · 1 deferral ended`. It reports only what crossed over
since the last minute's tick, so launching reports nothing already standing and
waking from sleep reports the gap once. A date-only deadline is not an arrival:
its day is the Today lane's and the overdue count's. It is opt-in because a
terminal that honors the escape shows it whether or not tt is in view.

`tt --control PATH` serves a Unix socket at `PATH`, readable by your user
alone, for a host app such as tt.app. Over it the session reports the pulse
whenever it changes and each tick's arrivals with the identity of every task,
and it takes six requests: show a task, acknowledge one as `R` does, defer one
by some minutes, capture a line as the `a` composer would read it, refusals
included, ask what that composer would suggest for a draft up to its caret,
and preview the project, plan, status, and deadline of a full capture draft.
Each is answered as though you had pressed the key in tt: through the same
commit and the same undo entry, so `u` takes it back. A request that changes
something is refused, never queued, while you are inside a picker, an editor, a
selection, or another view — completions and previews change nothing, so they
are answered even then — and a session that lost focus first adopts any change made on disk since,
so what it commits is never against a stale read. tt.app starts tt this way, and
falls back to `--notify` for a tt too old to have the flag.

The strip is also where `Q` expands the narrowing readout: one numbered row per
narrowing the view is under, then an `Also in view` block for the axes nothing
peels — your selection, the effective sort, a focus, a section zoom — with the
conflict indicator keeping the lead above it all. It grows the strip rather than
floating over the list, so the rows it pushes off are rows the strip was already
entitled to, and it never takes the last one.
[`search-and-lenses.md`](search-and-lenses.md) has the readout and its peels.

`gc` opens the conflict browser in the TUI and desktop app. It lists recorded
bundles newest first, followed by sync conflicts. `Enter` inspects the selected
conflict; `d` toggles a Git-style diff against the current content (recorded
remote content for project bundles). `b` switches to or from the base comparison. Project bundles offer
`l` to keep local and `r` to keep the current remote; sync conflicts use `[`/`]`
to select a version, `v` to keep it, and `c` to keep the current content. `e`
opens the in-app merge editor where text replacement is supported. `Ctrl+S`
reviews the merged result, and `Enter` confirms a resolution. Related files
and retry requirements are shown before confirmation. Errors retain the draft.

The pane scrolls with the ordinary list keys. `Esc` backs out one level at a
time and asks before discarding edits. With no unresolved conflicts, `gc`
says so in the strip instead of opening an empty overlay. See
[conflict resolution](storage.md) for the complete workflow.

After a commit, recovery feedback advertises `u undo` only when the newest
workspace-ledger entry is that exact transaction. A task relocation adds
`Ctrl+O origin` only after its origin was recorded; `Ctrl+I` returns to the
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
session-only: after restarting `tt`, undo still restores the same work but never
moves the cursor.

The Outliner marks its cursor row in the margins rather than over the row. One
bright column at the far left says where the cursor is, and the empty columns
past the row's text carry a dim recess saying how far the row reaches. Nothing
between them changes: the multi-select box, the tree connectors and the fold
arrow keep the colors they wear on every other row, and so do the state badge,
the priority badge, the due tier, the project badge and the
search-match highlight — all of which read the same under the cursor as beside
it, leaving an overdue row visibly overdue while you stand on it. The bar is the
row's leftmost column on every row type: a blank
margin, except on a section header, where it reverses the fold arrow. A cursor
flash floods the whole gutter rather than changing the bar's color, so a pulse
reads as more of the same mark. Every other list — the pickers, the tag and
parent selectors, notes, the project hub — still inverts the whole row, because a
flat list row has no margins to paint.

A list scrolled into the middle of a lane says which lane it is showing. When
the lane's header is above the window, the first list row becomes a pinned
context row naming the lane, the project the window opens in, and the lane's
count:

```text
▾  Waiting › [release-catalog]  (212) ──────────────────────
```

It is muted, closed with a rule, and never the cursor's: it is not a row, so
`k` from the row under it scrolls the list rather than landing on it, and the
real header is still where `gg` or `N` finds it. It costs one list row while it
is drawn and none otherwise — a window that opens on a lane header, or one that
has not scrolled, shows the header itself. The flat `/` result list and a view
scoped to one project draw none.

Every tag carries a color, and it is the same color everywhere a tag is rendered
as itself: the `Tags:` line in Peek, and every row of
the tag list the composer's expanded detail opens. Nothing needs configuring — a
tag earns its color from its own name the first time you type it and keeps it
for good, and `T` re-resolves the whole set for the new theme so a tag stays
recognizably itself in dark and light.

A row inside `#`, `f#`, or `F#` is an offer rather than a label, so it shows you
the color it is about to paint. Most of those rows paint nothing else and simply
wear their own chip: the descriptive tag rows, and the tag rows that `#` collects
under `CONTEXTS` because the same value is also in use as a context. A tag that does
something louder shows that instead — a row under `SEMANTIC` wears the color the
six-name vocabulary reserves for it, and a `parked` project row wears the warning
badge it hangs on the project. So the same tag can read as one color in a picker
and another as a chip; when they disagree, the picker is naming what the row is
about rather than repainting the chip. Two rows stay plain on purpose: the
`##name` radio rows in `#`, which set a field rather than name a tag, and `F#`'s
`Untagged` row, which names the projects carrying no tag at all. `fc`'s rows are
plain for the same reason: a context is a field, not a chip. A Select
Project row is the one place a tag is named without its color, because its tags
share the one secondary line that carries the counts beside them.

Read a chip color as a mnemonic, not a legend. It tells you `#billing` is
reliably that one color; it never tells you what that color means. Six chip
colors cover an unlimited tag vocabulary, so two unrelated tags will sometimes
share one, and nothing in `tt` ever decides anything from a chip color. A `bug`,
`question`, `epic`, `story`, `feature`, or `idea` tag is a tag like any other:
it wears its chip in Peek and the pickers, and the strongest one present — by
the precedence `bug`, `question`, `epic`, `story`, `feature`, `idea` — also
tints the task's state glyph. The task title stays ordinary text
whatever a task is tagged, how urgent it is, or when it is due.

A task row shows no chips at all. Its columns go to the title, and Peek names
the whole tag set on one `Tags:` line, in an order that does not depend on how
you typed them: descriptive tags first in alphabetical order, then the semantic
ones, also alphabetical. A task tagged `ux`, `bug`, and `billing` reads
`#billing #ux #bug` however it was captured — the six semantic names are a
vocabulary every task shares, so the tags you cannot guess lead. Nothing is
dropped and nothing is counted away, because Peek wraps rather than clips: the
order decides reading sequence, not survival.
