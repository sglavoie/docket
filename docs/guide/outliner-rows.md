# Rows, checklists, and Peek

What a task row says, in what order it sheds those signals when the width runs
out, the checklist rows that fold open beneath a task, and the detail Peek adds.
The keys are in [`outliner-keys.md`](outliner-keys.md).

## Common actions

| Goal | Keys | Example |
| --- | --- | --- |
| Read a task’s full details | `b` | Read its resume note or unclipped title |
| Give details more room | `B` | Expand Peek, then press again to return to compact |
| Scroll details | `Ctrl+D` / `Ctrl+U`, `Ctrl+F` / `Ctrl+B` | Move half a page or a full page |
| Open children or checklist | `l` / `→` | Open children first, then checklist items |
| Select work together | `v`, `V`, `Ctrl+A` | Select several tasks, then `wn` to plan them for Next |
| Put a task out of sight | `wh`, then `fH` to see it | Hide a task and its children from every view; `u` takes it back |

Task titles carry the visual emphasis. Project labels keep their identity color
without bold text, lane headings have no filled background, and `‹` / `›N`
family markers are muted. Selection and urgent dates retain stronger emphasis.

Crowded rows give a recognizable phrase of the title room before keeping review
and reference badges. Deadlines yield only when even a short title would not
fit. `b` shows the complete title and every detail the row omits.

Peek's status summary includes **Overdue** and **Check-in owed** when applicable,
before priority and flags, so attention is visible beside the resume note.
Under `fH`, a hidden task's status ends with the bare flag `(hidden)`, which a
child hidden only through its parent wears too. A `Dependencies` or
`Dependents` entry naming a hidden task ends with that same `(hidden)` after
its state; the entry and the `~S/N` count stay.
The full dates and repeat schedule remain in the scrollable details.

**Before a bulk action:** read the selection count. Hidden selected tasks still
belong to the batch. Opening Peek does not change selection.


## Inline checklist rows

A task that carries checklist items folds them open beneath itself, as `[ ]` and
`[x]` rows indented one level past the task. There is no new key: `l` and `h`
walk a single ladder, so on a task holding both children and a checklist `l`
opens the children first and then the items, and `h` closes the items first and
then the children. `h` on a checklist row returns to the owning task rather than
folding anything, since the item is one rung below it. Checklists start
collapsed; Peek (`b`) shows checklist progress, such as `1/3`, either way.

A checklist row is not a task row. Task mutations refuse there with one
sentence — "move the cursor to a task row to use this action" — instead of
quietly applying to the owner.

Press `b` on an item to read its complete text, Done or Not done state, position
in the checklist, and owning task. Long text wraps and scrolls with the usual
Peek controls. Moving between tasks and checklist items keeps the dock the same
size; each item's reading position is separate from its owner's.

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

### Read a task row

The Outliner uses the full terminal width at every supported size. A task row is
its title first: what the work is, in as many columns as the row can spare.
Beside it the row carries only what you would act on without opening anything —
workflow state, priority, whether it is pinned, when it is due, whether it owes
a review, and how many other tasks it points at. Short task IDs are hidden, and
so now are tags, the neutral Waiting and Doing ages, deferral markers,
and the checklist, comment, and attachment counts. None of those is gone: `b`
opens Peek on the same row and shows every one of them at full value, unclipped,
rather than in a fixed-width badge the row had to buy out of the title.

A paperclip (`📎`) after the title means the task has one or more attachments,
including files, URLs, and code references. It stays visible on narrow rows.
Press `&` to open the attachments panel; Peek shows the count and details.

A document glyph (`▤`) after the title means the task has a description.
It precedes the paperclip when both are present and stays visible on narrow
rows. Empty or whitespace-only descriptions show no glyph. Open Peek with `b`
to read the description.

Context appears on the task row immediately after the project label, as
`[project] [context]`. When the project is named by a group header, the task
shows its context before the title. Long contexts and combined project/context
labels are shortened to leave room for the task title and its indicators.
Grouped rows use only the space their labels need: a task without context
leaves no empty context column. Cross-project ranked results align their labels.
Peek (`b`) shows the full context; `#` opens the picker to change it.

An overdue deadline includes `overdue` in its badge, such as `Oct 15 overdue` or
`Today 09:00 overdue`, so it remains identifiable without color. Peek's Due line
shows the date, any time, and `overdue`. A date-only deadline becomes overdue
the following day; a timed deadline becomes overdue after its time passes.

A title too wide for its row is display-compacted before it is cut: a link
draws as its host and first path segment, so
`https://www.slack.com/archives/C024BE91L/p17…` reads `slack.com/archives/…`,
and a path under your home directory draws as `~/…/<last segment>`. Only the
row is compacted. Peek, the inline editor, the form, and exports keep the whole
title. If ordinary shortening would hide a search match, the row shows an excerpt
around it, keeping an issue number or directory name visible. `…` marks omitted
text; Peek keeps the whole title.

A deferral is one of those, and it is read in two places depending on which way
it points. Peek's schedule line reads `Defer: <date>` while it is still pending,
and the date carries a clock whenever the task comes back partway
through a day: a deferral to a whole day stays a bare `Defer: Sep 2`, one that
returns later today reads as `Defer: 15:04`, and one on another day as
`Defer: Sep 2 15:04`, with the year added when it falls outside this one. Once
the deferral lapses it stops being something the task owes, so it moves to
Peek's history line, which says `Woke: <age>` for three days: a task that
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
footer says so — `No check-in: <task> · F10: Set up check-ins` — naming the
keyless Task menu entry, which `F10` opens, that schedules one rather than
asking for a date there and then. It is a reminder, not a refrain: the first such
commit in a session says it, and later ones stay quiet.

Waiting and Doing ages appear in Peek. Their rows become prominent only when
there is something to act on, such as a deadline, priority, or owed review.

Due-date chips include the year when the date is outside the current year,
including overdue dates. Today and Tomorrow keep their relative labels.

Due dates and arrived check-ins use filled chips. Today and overdue dates are
bold; upcoming dates are quieter. In `--no-color`, bold and the `⊡` date marker
remain, so urgency does not depend on color.

A Doing task similarly reads `doing <age>` in Peek, measured from the moment it
entered Doing rather than from its last touch. It reports how long work has
stayed active without spending a row column on an age that is never itself the
thing you act on.

From the third deferral onward, Peek's history line also reads `Deferred: ×N`.
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

Fold arrows and task titles stay aligned as you navigate or open a subtree.

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

`Alt+↑` widens in the other direction. Outside Focus it opens Focus on the
cursor task's parent, so the task's own siblings come into view; a root task
reports `No parent to focus`. Inside Focus it climbs one level from the focus
root, again keeping the cursor where it was, until the focus root is a root task
and the next press leaves Focus entirely. A press after that is an ordinary
entry once more, focusing the cursor task's parent.

Project labels keep their distinguishing suffix when shortened names would
otherwise collide. Peek always names the full project slug.

### Open, expand, and scroll Peek

`B` switches an open Peek between compact and expanded sizes. Expanded Peek
uses roughly two thirds of the outline's height while preserving list rows.
The choice lasts for the session, including after closing and reopening Peek;
cursor movement never changes it.

`b` toggles a session-only Peek in a pane docked below the list, whether the
cursor is on a task or on a project header. Compact Peek uses about a quarter
of the outline's height, normally six to eight rows. Moving between short and
long tasks keeps the list in place. Capture can temporarily shrink the dock;
closing capture restores it. A checklist item shows its full text, completion
state, and owning task. A section heading displays `No detail to show for this row.`;
`b` closes Peek from any row.

Task Peek ordinarily leads with the full title, status, and resume note.
When compact Peek cannot show two lines of the resume note on its first page,
it moves the note ahead of status, or ahead of a title that would fill the
page. Nothing is shortened or removed: scrolling retains the full document,
and expanded Peek restores title-first order while preserving the reading
passage. For example,
`Plan: Today · Status: Backlog · Priority: 1 high` distinguishes the planning
collection from the lifecycle state. Doing and Waiting include time in that
state. The project slug and parent chain follow, then the schedule, description,
checklist progress, attachments, comment count, dependencies, tags,
and history. Empty fields are omitted.

| Detail | Example | Meaning |
| --- | --- | --- |
| Schedule | `Check-in: Thu Aug 20 (in 10 days) · Repeat: every 7d` | What is still owed; an arrived check-in adds `review owed` |
| History | `Created: 12d ago · Updated: 2d ago · Woke: 1d ago` | Past activity; Updated is omitted when it repeats Created |
| Checklist | `Checklist: 1/4 done` | Progress; open the task's checklist rows to edit items |
| Search | `Match: comment · …` | An excerpt explaining a match outside the title |

URL and `path:line` attachments are clickable in terminals that support OSC 8
links (`Cmd`-click in Ghostty and tt.app). Wrapped links still open the complete
address. Open stored files through the `&` panel.

Long details wrap and remain accessible by scrolling:

| Keys | Action |
| --- | --- |
| `Ctrl+D` / `Ctrl+U` | Scroll half a pane down / up |
| `Ctrl+F` / `Ctrl+B` | Scroll a full page with one row of overlap |
| `PgDn` / `PgUp` | Move the outline, even with Peek open |
| `b` / `Esc` | Close Peek before unwinding navigation |

The bottom line reports hidden content, such as `12 above · 7 below`.
At either end, Peek's scrolling keys stop rather than moving the outline.
The footer offers only directions that can move.

Peek remembers the most recently scrolled task or project during the session.
Moving away and back preserves that position unless you scrolled another owner.
Resizing, changing the theme, appending text, and edits before an unchanged
passage preserve the passage being read. Replacing that passage returns to the
start of its section; removing the section or changing the search query returns
to the top. A shorter document clamps the position to its end.

On a project header, Peek shows the full project name, a task summary such as
`Tasks: 12 · Doing 2 · Backlog 7 · Done 3`, description, tags, and every comment
newest first. Counts include all stored tasks regardless of the current view.
An empty project says `0 · no sections`; an unread project says `Not loaded`.
With no comments, it says `No comments yet. Press C to add one.`

All headers for the same project share its reading position. A group with one
task has no project header: fold it with `h` to reach the header and its Peek.
Task Peek shows a comment count; `;` opens the full conversation.

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

A selected task wears a `✓` in its leftmost column, the one the cursor bar
already uses, and every other row leaves that column blank. The mark takes no
column of its own, so no title moves when a selection starts or ends. It is a
glyph rather than a color: under the cursor it is drawn reversed, and a terminal
without color still shows it.

How many tasks you are holding is stated in exactly one place: the second header
row, as `Multi-select: 5 selected`, with `(2 hidden)` after it whenever the view
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
