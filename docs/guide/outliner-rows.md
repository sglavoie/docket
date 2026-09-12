# Rows, checklists, and Peek

What a task row says, in what order it sheds those signals when the width runs
out, the checklist rows that fold open beneath a task, and the detail Peek adds.
The keys are in [`outliner-keys.md`](outliner-keys.md).

## Inline checklist rows

A task that carries checklist items folds them open beneath itself, as `[ ]` and
`[x]` rows indented one level past the task. There is no new key: `l` and `h`
walk a single ladder, so on a task holding both children and a checklist `l`
opens the children first and then the items, and `h` closes the items first and
then the children. `h` on a checklist row returns to the owning task rather than
folding anything, since the item is one rung below it. Checklists start
collapsed, and the task row keeps its `1/3` progress badge either way.

A checklist row is not a task row. Every task action refuses there with one
sentence — "move the cursor to a task row to use this action" — instead of
quietly applying to the owner.

What a checklist row does own is ten operations of its own. Nine of them
answer a key:

| Key | Operation |
| --- | --- |
| `x` | Toggle Item — tick or untick it |
| `e` | Edit Item — rewrite its text |
| `J` | Move Item Down — swap it with the item below |
| `K` | Move Item Up — swap it with the item above |
| `Ctrl+J` | Move Item to Bottom — send it past every item below |
| `Ctrl+K` | Move Item to Top — send it past every item above |
| `c` | Add Item — append an item to this task's checklist |
| `S` | Make Subtask — convert it into a subtask of the task that owns it |
| `Space` | Checklist Actions — the item menu: Edit Item and Delete Item |

Delete Item is the tenth and answers no key. It is reachable only from the
`Space` menu and confirms before it removes anything.

Add Item is the one of the ten that also answers from the owner task's own row,
because a task with no checklist yet has no item row to press a key on. `c`
therefore adds the first item and every item after it, from the task or from any
of its items, and the prompt is the same single line either way: `Enter` saves,
`Esc` cancels, and a blank line is refused. Once the item is written the
checklist opens and the cursor lands on it, so pressing `c` again adds the next
one. A refused write leaves the fold and the cursor exactly as they were and
hands the item back: the prompt reopens on the text you typed, with the reason
the save was refused, so `Enter` retries it once the problem is fixed.

`S` promotes the item under the cursor: the item's own text becomes a new
subtask of the task that owned it, and the item itself is gone. It asks nothing
and confirms nothing, because the text you are looking at is the whole of the
new task. An unticked item becomes a Backlog subtask; a ticked one becomes a
subtask that is already done. Nothing else is copied down from the owner — no
dates, no tags, no priority — and one press is one undo entry, so `u` puts the
item back where it was.

Once the promotion commits, the cursor moves to the new subtask, opening the
branch it hangs under if that branch was folded. Nothing else about the view
moves: the mode, the search, the filters, the project scope and every other fold
stay exactly as they were, so a view that does not admit the new subtask simply
does not show it. When that happens the cursor returns to the owner task and the
status line names the subtask you cannot see — including when a ticked item
became a done subtask and the view is hiding done work.

Pressing `S` twice in a row never converts the neighbor. The whole checklist is
frozen at the keypress and checked again at the commit, so the second press is
measured against a list it no longer matches and is refused rather than applied
to whatever item slid up into the row.

Visual selection is one of the actions a checklist row refuses, because a
selection names task rows. Entering it from a task row hides any open checklist
items, and `Esc` brings back exactly the rows that were open.

## Peek and multi-select

The Outliner uses the full terminal width at every supported size. Task rows
show workflow state, priority, due and defer dates, and
checklist progress; short task IDs are hidden.

The bold `[context]` segment is reserved like the relation marker: it never
drops when a row narrows. Its value uses at most 12 columns in the row, while
Peek shows the complete value when truncation was necessary. Tags remain a
separate, lower-salience segment and may drop first.

A deferral reads as `defer <date>` only while it is still pending, and the date
carries a clock whenever the task comes back partway through a day. A deferral
to a whole day stays a bare `defer Sep 2`; one that returns later today reads as
`defer 15:04`, and one on another day as `defer Sep 2 15:04`, with the year
added when it falls outside this one. Once the deferral lapses the row says
`woke <age>` for three days, because a task that came back is work again and a
past-dated `defer` would read as still hidden. After that the marker disappears
and the task is ordinary work.

A row that wakes while the Outliner is open returns on its own within a minute,
without a key press. The return waits if an overlay, picker, inline row editor,
visual selection, or the flat ranked list is open, because rebuilding the rows
underneath one of those would move the ground the user is standing on. A query
that is only narrowing the mode's own lanes is not one of those states: it is
re-answered on every rebuild, so the woken task is matched against it like any
other row and returns with them.

A Waiting task reads as `waiting <age>`, measured from the moment it entered
Waiting rather than from the last time anything touched it, so recording a chase
in the comments does not reset how long the work has been blocked. If it also
carries a due date, that date is the check-back, so the row replaces the plain
date with `check <age>` counting down to chasing it, `check today`, or an
overdue `check +<age>`. Moving a task to Waiting with `ww` therefore asks for the
check-back date immediately; `Esc` there declines the date and the Waiting move
still stands.

Due dates and check-backs both render as a filled chip rather than as colored
text, so a date reads as one bounded object beside the titles, tags, and context
badges that share the row. All three tiers are chips — overdue, today, and
upcoming — and they share one hue that no section header uses, so a date is
never mistaken for a section; urgency is carried by how saturated the fill is
rather than by its hue. Under the cursor the chip
flattens into the selection colors like every other colored segment, and under
`--no-color` the fill drops away and the `⊡` glyph marks the date on its own.

A Doing task similarly reads as `doing <age>`, measured from the moment it
entered Doing rather than from its last touch. The neutral age remains beside a
due date when both apply; it reports how long work has stayed active without
turning age alone into another urgency badge.

From the third deferral onward, a row also reads `deferred ×N`. Moving the date
of a still-pending deferral does not add to the count; deferring the task again
after it woke does. This supporting count drops before every other row indicator
under width pressure, while Peek keeps the same token on its existing dates line.

A task holding soft dependencies ends its indicators with `~S/N`: `N` counts the
references it stores and `S` how many of them are settled. `N` is a count of what
the task says, so a reference to finished, deleted, or unloaded work counts
exactly like a reference to open work. `S` is the reading of what those targets
are doing, and it counts a finished task and a task no longer in a loaded project
alike, since neither has anything left to wait on. A set where the two agree
reads as the bare `~N` instead, because a fraction whose halves match says only
that nothing is left, and a trailing `?` says at least one of the references
points into a project this view did not load. Both readings stop at one hop: a
target's own references are its business, and following them would report on
projects this view may never have loaded.

The color says which kind of remainder the fraction has. Amber means at least one
referenced task is still open. Green means everything it points at is finished —
or is no longer in the workspace at all. Muted means nothing readable is left,
but at least one reference points into a project this view did not load, such as
an archived or parked one; the row stays muted rather than green there because it
cannot see whether that work is done, and saying green would claim otherwise.
Peek is still where the targets themselves are named.

A row that cannot fit every indicator sheds them in a fixed order rather than
squeezing the title: the repeated-deferral count goes first, then checklist,
comment, and attachment counts because the checklist rows and Peek still carry
them, then the dependency count because Peek still names it, then the recency
and defer markers, then the waiting age, due date, and Doing age; the check-back
date goes last.

A task shown under a section it did not root — a child promoted into Doing,
Today, Pinned, or a tag facet — ends its title with a bare `‹`, which says only
that the row has a parent the outline above it does not show. Nested rows carry
nothing, because their tree connector already states parentage.

The mirror mark reads the other way: a row whose open children are all placed
somewhere else ends with `›N`, naming how many are hidden — a parent that
started work and sits in Doing while its children wait in Backlog. Only open
children count, so a task whose children are all done or canceled stays quiet,
and a row that shows even one child carries nothing because its connectors and
fold indicator already say it is a parent. A row that is both a promoted child
and a detached parent reads `‹›N`.

Both marks share one segment of fixed columns reserved before the title, so
they neither eat the title on a wide row nor disappear on a crowded one.

The pin marker reads the same way. A pinned task drawn anywhere but the Pinned
lane — Inbox, Next, Backlog, Due, Recent, the flat ranked list, or a filter that
admits a terminal or deferred pinned row into All or Today — wears a bold `*`
before its context badge and title. Rows inside the Pinned lane wear nothing,
because the section header already says it. The marker is reserved columns like
the relation mark and is never dropped.

A pinned project wears the same `*`, so one glyph means pinned everywhere: on
the Outliner project group header, where it sits in the column a pinned task's
mark occupies one row below, on the Select Project rows, and in the `%` Move to
Project list. It is never dropped there either — a crowded project header sheds
its `today` and `overdue` counts first. An archived project keeps the marker,
because the pin is real state that survives archiving, but `*` itself does not
work there and the footer stops offering it: the marker shows the state, the key
hides because the act is unavailable.

`Enter` focuses the cursor task's descendants and `Esc` restores the exact
parent cursor and scroll position. A leaf reports `No children to focus`. While
focused, `a` captures a direct child of the focus root rather than a root task
elsewhere. Pressed on a result in the flat ranked list, `Enter` dismisses the
query first and lands the ordinary projection on that task, so `Esc` unwinds onto
its own row; a leaf still reports `No children to focus`, with the jump already
made. A query that is only narrowing the mode's own lanes has nothing to leave,
so `Enter` there is the ordinary zoom.
`Enter` is that same zoom-in act on the two header rows. On a project sub-header
it scopes the Outliner to that project — except on the `[slug]` header a focused
project leads with, which names the scope you are already in and so takes no
`Enter` and no fold. On a section header it narrows the mode
to that one section: `Enter` on `Next` in All shows the Next lane for every
project, and in a single-project scope shows only that project's. A collapsed
section zooms open without disturbing its fold, and the title carries a
`Section: Next` segment for as long as the zoom holds. `Esc` restores the whole
mode on the header the zoom was entered from, unwinding after Focus and before
the project scope. Zoom and project scope never compose: `Enter` on a project
sub-header inside a zoom scopes and drops the zoom, and the `Esc` that pops that
scope brings the zoom back with it. An emptied zoom stays put and reports
`No tasks are left in this section. Press Esc to return.` rather than unwinding
under you.
`Shift+Enter` widens in the other direction. Outside Focus it opens Focus on the
cursor task's parent, so the task's own siblings come into view; a root task
reports `No parent to focus`. Inside Focus it climbs one level from the focus
root, again keeping the cursor where it was, until the focus root is a root task
and the next press leaves Focus entirely. A press after that is an ordinary
entry once more, focusing the cursor task's parent.
`b` toggles a session-only Peek directly beneath the cursor row, whether that row
is a task or a project header. On a task, Peek
follows navigation and reads, in this order: the resume note, the description, a
dates line reading
`created 12d · updated 2d · due 2026-08-14 · woke 1d · deferred ×3` with empty fields omitted,
attachment names, a comment count, a `Dependencies 1/3:` list, a `Dependents 2:`
list, the task's context, and its tags. Every one of those is shown whole. Long
text wraps rather than being cut off, and nothing is dropped to make room for
something else: Peek shows five rows at a time and `Ctrl+D` and `Ctrl+U` scroll
through the rest, two rows a press, while `Ctrl+F` and `Ctrl+B` move a whole page
at a time. Every move keeps at least one row of what you were reading, the two
step sizes mix freely, and the last one stops at the end of the detail rather
than running past it. While there is more than a window's worth, a row at the
bottom reads `12 above · 7 below` so you always know how much is out of sight.
The footer offers a key only when that direction can actually move, but all four
belong to an open Peek: pressing one at the end of the detail does nothing rather
than moving the outline underneath. `PgDn` and `PgUp` still move the outline,
open Peek or not. Where you scrolled to is remembered while you stay on the same
row and resets by itself when you move to another task or project, the detail
changes, or the terminal is resized.

On a project header — the group header a cross-project mode draws above each
project's tasks, and the single header a project-scoped or focused view leads
with — the same key opens the same pane over the project instead. It reads the
project's full name above the body, then its description, its tags, and every
comment it has, newest first, each under a `[2026-03-03 09:30] 6 days ago` stamp.
A project with no comments yet says `No comments yet.` rather than showing
nothing, so the key always answers on a header. Everything is shown whole and the
same four keys page through it, exactly as on a task. A project shown under
several headers at once — All draws one per state section — is one detail behind
all of them, so where you scrolled to under one is where you are under the next.
A task's Peek is unchanged: its comments stay a count, because `;` is the panel
that reads them.

Both reference lists are read-only wherever Peek is open, and
both carry their own count. `Dependencies` names what this task stores, in the
order it stores it, and its fraction is the row's very numbers; that outgoing set
is edited from the row itself with `~`, never from inside Peek. `Dependents` is
computed from the projects currently loaded, so its count is of the tasks that
point back from there and no key edits it at all. Each entry takes a row of its
own, reading as its project, short ID, title, and
state; a reference whose project is not loaded keeps its stored
`project-id:task-id` locator and reads `(unavailable)` rather than being dropped.
Comments stay a count, because `;` is the panel that reads them, and checklist
items are absent because they are already rows of their own beneath the task. A
promoted row's
ancestor chain is named above the body in full, root-first, as
`Parent: Launch platform › User authentication`; it and a clipped title's
continuation sit outside the five rows rather than spending
them, so a page is always five rows of detail. The `Resume:` line is the first of
those, because it answers the question Peek is opened to ask. `Esc` closes Peek
before unwinding Outliner navigation.

State, Today/Next, priority, due, tags, defer, pin, and Move to Project actions
apply atomically to the complete project-qualified selection, including hidden
members. `y`, `Y`, and `Ctrl+Y` copy that same complete selection in one
clipboard write: the text reads in the order the rows are drawn, with the
members the view is not showing after them, ordered by project and then by each
project's own task order. Only what you selected is copied, so a selected parent
brings no unselected child along, and the status line names the total and how
many of them were hidden. A missing task or a clipboard that refuses the write
copies nothing rather than part of the selection. `v`
toggles the cursor task in or out of the selection, so a selection can name rows
that are not adjacent; movement never changes what `v` picked. `V` starts a
fresh inclusive range at the cursor; movement recalculates it over task rows
only, and `V` or `Esc` freezes it. `v` during a live range freezes it and drops
the cursor row. `Ctrl+A` replaces the selection with all visible task rows. The
selection survives at zero members until `Esc` leaves the mode. Entering
multi-select closes Peek. Structural, edit, Peek, and
relation operations do not fall back silently to the cursor while a selection
exists, and neither do the clipboard keys: they copy the selection, and an empty
one copies nothing. Successful Delete clears selection; other successes,
cancellations, and failures preserve it.
