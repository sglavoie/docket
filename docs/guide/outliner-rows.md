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
collapsed; Peek (`b`) shows checklist progress, such as `1/3`, either way.

A checklist row is not a task row. Every task action refuses there with one
sentence — "move the cursor to a task row to use this action" — instead of
quietly applying to the owner.

The checklist operation catalog contains eleven operations. Ten of them answer a key
on a checklist row or child task row:

| Key | Operation |
| --- | --- |
| `x` | Toggle Item — tick or untick it |
| `e` | Edit Item — rewrite its text |
| `J` | Move Item Down — swap it with the item below |
| `K` | Move Item Up — swap it with the item above |
| `Ctrl+J` | Move Item to Bottom — send it past every item below |
| `Ctrl+K` | Move Item to Top — send it past every item above |
| `c` | Add Checklist Item — append an item to this task's checklist |
| `S` | Make Subtask — convert it into a subtask of the task that owns it |
| `S` | Make Checklist Item — convert a child task into an item in its current parent's checklist |
| `Space` | Checklist Actions — the item menu: Edit Item and Delete Item |

Delete Item is the eleventh and answers no key. It is reachable only from the
`Space` menu and confirms before it removes anything.

Add Checklist Item also answers from the owner task's own row,
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

Two `S` presses queued before the first commit never convert a neighboring
item. The whole checklist is frozen at each keypress and checked again at the
commit, so the second request is refused when the first changes that list.
After a completed conversion, another `S` on the selected child converts it
back into a checklist item.

On a child task row, `S` converts that child into an item in its current parent's
checklist. It uses the child's current title; Backlog becomes unticked and Done
becomes ticked. A child created from a checklist item returns to its saved
position if it still has the original parent; other children append. The task
must have no descendants, other task content, Today or Next membership, or
incoming task references. One undo restores the child and removes the item.
After a successful conversion, the parent's checklist opens and the cursor
selects the new item. If filters hide it, the status line reports the result.

Visual selection is one of the actions a checklist row refuses, because a
selection names task rows. Entering it from a task row hides any open checklist
items, and `Esc` brings back exactly the rows that were open.

## Peek and multi-select

The Outliner uses the full terminal width at every supported size. A task row is
its title first: what the work is, in as many columns as the row can spare.
Beside it the row carries only what you would act on without opening anything —
workflow state, priority, whether it is pinned, when it is due, whether it owes
a review, and how many other tasks it points at. Short task IDs are hidden, and
so now are tags, context, the neutral Waiting and Doing ages, deferral markers,
and the checklist, comment, and attachment counts. None of those is gone: `b`
opens Peek on the same row and shows every one of them at full value, unclipped,
rather than in a fixed-width badge the row had to buy out of the title.

The trade is deliberate. A row is a list you scan; Peek is the one you read. A
title that fits is worth more on every row than a badge that only matters on
some, and a value shown whole one keypress away is worth more than an
abbreviation shown always.

A deferral is one of those, and it is read in two places depending on which way
it points. Peek's schedule line reads `defer <date>` while it is still pending,
and the date carries a clock whenever the task comes back partway
through a day: a deferral to a whole day stays a bare `defer Sep 2`, one that
returns later today reads as `defer 15:04`, and one on another day as
`defer Sep 2 15:04`, with the year added when it falls outside this one. Once
the deferral lapses it stops being something the task owes, so it moves to
Peek's history line, which says `woke <age>` for three days: a task that
came back is work again and a past-dated `defer` would read as still hidden.
After that the field disappears and the task is ordinary work.

A row that wakes while the Outliner is open returns on its own within a minute,
without a key press. The return waits if an overlay, picker, inline row editor,
visual selection, or the flat ranked list is open, because rebuilding the rows
underneath one of those would move the ground the user is standing on. A query
that is only narrowing the mode's own lanes is not one of those states: it is
re-answered on every rebuild, so the woken task is matched against it like any
other row and returns with them.

A Waiting task carries its state badge and nothing else about the waiting. Peek
reads `waiting <age>`, measured from the moment it entered Waiting rather than
from the last time anything touched it, so recording a chase in the comments
does not reset how long the work has been blocked. A due date beside it remains
a deadline and keeps the ordinary due chip on the row. A check-in is a separate
schedule: once its day arrives the row says `review today` or `review +<age>`,
unless the task is deferred; before then it appears only in Peek and scheduling
controls. A task you flagged with `=` and owe no date for says a bare `review`,
so the row always names what it is asking for; inside the Review lane that bare
word is left off, since the lane's heading already says it. `review` and
`review today` share the teal of a due-today date, and a late `review +<age>`
turns amber, a warning rather than a missed deadline. Moving a task to Waiting with
`ww` opens nothing: the move commits, and when the task has no check-in the
footer says so — `No check-in: <task> · Ctrl+M: Set up check-ins` — naming the
keyless Task menu entry, which `Ctrl+M` opens, that schedules one rather than
asking for a date there and then.

Waiting by itself is quiet. The badge says where the task is, and that is the
whole signal: no background tint, no age competing for the eye, and nothing that
makes it read as more urgent than the work you could actually do something about
today. Waiting says where a task is, not how loudly it is asking. A Waiting task
still shouts when it earns it — a deadline that has passed, a check-in whose day
has come, a priority you set — on exactly the same terms as any other task.

Due-date chips include the year when the date is outside the current year,
including overdue dates. Today and Tomorrow keep their relative labels.

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

A Doing task similarly reads `doing <age>` in Peek, measured from the moment it
entered Doing rather than from its last touch. It reports how long work has
stayed active without spending a row column on an age that is never itself the
thing you act on.

From the third deferral onward, Peek's history line also reads `deferred ×N`.
Moving the date of a still-pending deferral does not add to the count; deferring
the task again after it woke does.

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

A row that still cannot fit its three indicators sheds them in a fixed order
rather than squeezing the title: the reference census goes first, because Peek
names every target in full and neither the deadline nor the review depends on
it; the owed review follows, because it says the task wants looking at while the
deadline says when something is actually due; the deadline goes last. The order
is short because the budget is short — the counts, ages, and markers that used
to fill the lower rungs did not move down it, they left the row for Peek.

Wherever one task in the list can be folded — it shows a child, or it has a
checklist — every task row keeps the two columns the `▾`/`▸` glyph takes, blank
on a row with nothing to fold, so state glyphs and titles line up down the
list. A list in which nothing folds keeps those columns for its titles, and so
does a terminal under 60 columns. The reference count stands two columns clear
of the title, the gap the deadline and review chips keep.

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
before its title. Rows inside the Pinned lane wear nothing,
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

`Enter` focuses the cursor task's descendants — Focus, in which that task is
the unlisted focus root — and `Esc` restores the exact parent cursor
and scroll position. A leaf reports `No children to focus`. While
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

`b` toggles a session-only Peek in a pane docked below the list, whether the
cursor is on a task or on a project header. Its height is a share of the terminal
— about a third of the outline's own room, never fewer than seven rows and never
more than twelve — counted off the list once when you open it, so a taller
terminal reads more of a long detail at a time. What the pane never reads is the
row you are on: a short task and a long one give the outline exactly the same
room, and moving between them never shifts the rows under your cursor. When
something else needs those rows — a capture panel raised over the pane on a short
terminal — Peek gives them up and comes back to its full height when that panel
closes, keeping your place in the outline; if the terminal is too short even for
the divider, one line of detail, and the position row, the pane stays closed
rather than being drawn cut off. On a row with no detail to show, a section
heading or a checklist item, the pane stays where it is and reads `No detail to
show for this row.`; `b` closes it again from any row. On a task, Peek
follows navigation and reads, in this order: the task's whole title, the ancestor
chain of a promoted row as `Parent: Launch platform › User authentication`, a
status line reading
`plan today · status doing 4h · priority 1 · pinned · review`
with empty fields omitted (only Doing and Waiting carry an age, and it counts
from the last state change rather than the last edit). The plan comes first and
is the collection you committed the task to, Today or Next; the status is the
lifecycle state it holds. They are two answers, not one, which is why a task
planned for Today reads `plan today · status backlog` and a task in no
collection has no plan half at all. Then a schedule line reading
`due 2026-08-14 · defer Sep 2 · check-in 2026-08-20 · repeat 7d`
with empty fields omitted too (a one-off schedule reads `repeat off`), the resume
note, the description, a `Checklist: 1/4 done` count,
attachment names, a comment count, a `Dependencies 1/3:` list, a `Dependents 2:`
list, the task's context, its tags, and last of all a history line reading
`created 12d · updated 2d · last review 3d · woke 1d · deferred ×3`.
A URL or `path:line` attachment name is also a link (OSC 8) in a terminal that
follows them — `Cmd`-click in Ghostty and tt.app — so a shortened
`GH owner/repo#123` opens the issue, and every wrapped row of a long URL opens
the whole URL. A stored file is opened from the `&` panel instead.
The split is what the task still owes against what it has already spent: the
schedule is the part you can act on, so it sits with the status line at the top,
and the history is the part you consult afterwards, so it closes the document
below everything else. Every one of those is shown whole. Long
text wraps rather than being cut off, and nothing is dropped to make room for
something else: Peek shows as many rows as its height allows and `Ctrl+D` and
`Ctrl+U` scroll through the rest half a pane a press, while `Ctrl+F` and `Ctrl+B`
move a whole page, one row short of the pane. Every move keeps at least one row
of what you were reading, the two step sizes mix freely, and the last one stops
at the end of the detail rather than running past it. A further row at the bottom
always reads
`12 above · 7 below`, so you know how much is out of sight and a detail that fits
says so with `0 above · 0 below` rather than changing the pane's height.
The footer offers a key only when that direction can actually move, but all four
belong to an open Peek: pressing one at the end of the detail does nothing rather
than moving the outline underneath. `PgDn` and `PgUp` still move the outline,
open Peek or not. Where you scrolled to is remembered while you stay on the same
row and resets by itself when you move to another task or project, the detail
changes, or the terminal is resized. Future check-ins appear here while the task
row stays quiet; only an arrived, undeferred schedule adds `review today` or
`review +Nd` to the row, and the review flag alone adds a bare `review`.

On a project header — the group header a cross-project mode draws above each
project's tasks, and the single header a project-scoped or focused view leads
with — the same key opens the same pane over the project instead. It leads with the
project's full name, then a task summary such as
`Tasks: 12 · Doing 2 · Backlog 7 · Done 3` — every stored task, children and
finished work included, counted by lifecycle section and the same whatever mode,
search, or folds you are in; an empty project says `0 · no sections`, and one
whose tasks have not been read yet says `Not loaded` rather than zero — then its
description, its tags, and every
comment it has, newest first, each under a `[2026-03-03 09:30] 6 days ago` stamp.
A project with no comments yet says `No comments yet. Press C to add one.` rather than showing
nothing, so the key always answers on a header. Everything is shown whole and the
same four keys page through it, exactly as on a task. A project shown under
several headers at once — All draws one per state section — is one detail behind
all of them, so where you scrolled to under one is where you are under the next.
A task's Peek keeps its comment count; `;` is the panel that reads the full
conversation. During search, a matching comment can also appear in the short
`Match:` excerpt below the title and parent chain.

Both reference lists are read-only wherever Peek is open, and
both carry their own count. `Dependencies` names what this task stores, in the
order it stores it, and its fraction is the row's very numbers; that outgoing set
is edited from the row itself with `~`, never from inside Peek. `Dependents` is
computed from the projects currently loaded, so its count is of the tasks that
point back from there and no key edits it at all. Each entry takes a row of its
own, reading as its project, short ID, title, and
state; a reference whose project is not loaded keeps its stored
`project-id:task-id` locator and reads `(unavailable)` rather than being dropped.
Comments stay a count below any search excerpt, because `;` is the panel that reads them, and the
checklist stays a count because its items are already rows of their own beneath
the task, where you tick, edit, and promote them. Nothing in Peek sits outside
the pane: the whole title leads the detail, so a title the row had to clip
with `…` is finished by scrolling rather than read from two places at once, and
the ancestor chain follows it. `Esc` closes Peek
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
selection survives at zero members until `Esc` leaves the mode.

How many tasks you are holding is stated in exactly one place: the second header
row, as `MULTI-SELECT: 5 selected`, with `(2 hidden)` after it whenever the view
is not showing all of them. Nothing repeats it lower down, which is what keeps
the status strip free for what your next bulk gesture has to say. A narrow
terminal shortens that segment before it drops it, and the hidden count is the
one thing it never gives up — a gesture about to reach rows you cannot see
should say so. Press `Q` and, whenever something is hidden, all three numbers
are written out there: total, visible, and hidden. Entering
multi-select closes Peek. Structural, edit, Peek, and
relation operations do not fall back silently to the cursor while a selection
exists, and neither do the clipboard keys: they copy the selection, and an empty
one copies nothing. Successful Delete clears selection; other successes,
cancellations, and failures preserve it.
