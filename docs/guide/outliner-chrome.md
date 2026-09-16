# Presentation grammar

What the Outliner draws around the task list: the title row and its workspace
pulse, the menu bar, contextual guidance, the status strip and the advisories it
carries, the conflict browser, how the cursor row is marked, and the colors a
tag wears.

Every full-screen presentation keeps one operational title row. Its left side
names the surface, scope, and mode once; active selection, search, filters,
focus, or editor context occupies the middle; and the right side reports the
workspace pulse as `workspace: inbox 3 · today 0 · overdue 1 · done 4`. Today
and the current local day's Done count are shown even at zero whenever the pulse
region fits; zero Inbox and overdue counts disappear. Narrow terminals drop
Done, Inbox, overdue, then Today before shortening required interaction context.

`M` puts a menu bar above that title row: a single band reading
`Task  View  Go  Find  Help`, one label per menu. `M` again takes it away. The
bar costs exactly one row of the task list while it is shown, which is the whole
reason it is off by default and the whole reason it is a key rather than a
permanent fixture. A terminal too narrow for every label drops whole menus from
the right — Help first, then Find, then Go — rather than cutting a word in half,
because a half-word is a lie about what the bar offers while a dropped menu is
still reachable by the keys its entries carry.

`Ctrl+M` opens the leftmost menu, borrowing a row for the bar if it was hidden,
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
highlighted entry and closes the menu, and `Esc` or `Ctrl+M` closes it without
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
`Enter`, the way they encode `Ctrl+I` as `Tab`. There `M` still shows the bar,
and `Ctrl+M` over an open menu runs the highlighted entry rather than closing
it.

Whether the bar is drawn is remembered for this machine, the way the light or
dark theme is: it survives quitting and reopening `tt`, and it does not follow
you to another computer that opens the same workspace. Chrome belongs to the
terminal you are sitting at. The View menu lists the toggle itself, checked when
`M` is keeping the bar rather than when a menu is only borrowing its row,
because the key that shows the bar is the one thing you cannot read off the bar
once you have found it by accident.

Today mode also compares its open load with the workspace's 30-day completion
pace in the middle region, for example `today 9 · median 3/day (30d)`. A fresh
workspace with no completion history shows no comparison. Under pressure the
workspace pulse disappears first; the comparison then sheds `(30d)`, then
`median`, and finally disappears before required interaction context shortens.

Contextual guidance is made of complete `key action` entries. Browse states use
one row; editing, adding, confirming, autocomplete, expanded detail, and pending
chords may use two. Required commit and cancellation controls survive before
optional hints, and they survive whole at the narrowest terminal `tt` runs in,
so the way out of a state is never half a sentence. Transient pickers and action
sheets own their local guidance while leaving the host title and workspace pulse
visible.

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

The Outliner's own feedback — a focus notice, or a line such as
`Latest activity: …` — renders in a status strip pinned directly above the
footer, introduced by a dim thin divider. The strip never floats mid-screen on a
short list and never leaves blank padding under itself: it takes its rows from
the task list, and in a narrow terminal it wraps rather than pushing the footer
off screen. It clears itself after a couple of seconds, and immediately on a
mode change, so a message naming a mode can never misreport the one you switched
to. A live visual selection is the exception: its summary stays for as long as
the selection does.

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

The strip is also where `Q` expands the narrowing readout: one numbered row per
narrowing the view is under, with the conflict indicator keeping the lead above
it. It grows the strip rather than floating over the list, so the rows it pushes
off are rows the strip was already entitled to.
[`search-and-lenses.md`](search-and-lenses.md) has the readout and its peels.

`gc` answers the question that indicator raises. It opens a read-only overlay
listing every recorded bundle newest first — its short id, the project, how many
conflicts it holds, the action that hit it, and when it was recorded. `Enter`
inspects one, showing exactly what `tt conflict show` prints: the revisions on
each side, whether the original operation still has to be retried, and every
conflicting field with its base, local, and remote values. The pane scrolls with
the ordinary list keys, `Esc` steps back to the list, and a second `Esc` closes
the overlay. Nothing here resolves anything: the browser reads the store and the
record it inspects is reread at the moment you open it, so a bundle another
terminal resolved in the meantime refuses rather than painting a stale record.
Resolving stays in `tt conflict resolve`. With no unresolved conflicts, `gc`
says so in the strip instead of opening an empty overlay.

After a commit, recovery feedback advertises `u undo` only when the newest
workspace-ledger entry is that exact transaction. A task relocation adds
`Ctrl+O origin` only after its origin was recorded; `Ctrl+I` returns to the
relocated task.

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
it wears its chip in Peek and the pickers and nothing else. The task title stays ordinary text
whatever a task is tagged, how urgent it is, or when it is due.

A task row shows no chips at all. Its columns go to the title, and Peek names
the whole tag set on one `Tags:` line, in an order that does not depend on how
you typed them: descriptive tags first in alphabetical order, then the semantic
ones, also alphabetical. A task tagged `ux`, `bug`, and `billing` reads
`#billing #ux #bug` however it was captured — the six semantic names are a
vocabulary every task shares, so the tags you cannot guess lead. Nothing is
dropped and nothing is counted away, because Peek wraps rather than clips: the
order decides reading sequence, not survival.
