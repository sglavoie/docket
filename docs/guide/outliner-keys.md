# Outliner grammar

Every key the Outliner answers, the pickers those keys open, the `Space` action
sheets, and the two edit surfaces. What the modes themselves list is in
[`outliner-modes.md`](outliner-modes.md).

| Area | Keys |
| --- | --- |
| Sync | `gs` exchanges with the configured sync folder now; **Go → Sync Now** does the same, with progress and results in the footer |
| Modes | `Tab` / `Shift+Tab` cycle the seven modes; `g` opens the go-to chord: `ga` All, `gi` Inbox, `gt` Today, `gn` Next, `gb` Backlog, `g@` Due, `gr` Recent, `gp` the cursor task's project, `gP` the immediate parent task (or a checklist item's owner task), `g~` a task the cursor task references, `gc` the merge-conflict browser and resolver; `1`–`7` are the same seven jumps in `Tab`-cycle order — `1` All, `2` Today, `3` Inbox, `4` Next, `5` Backlog, `6` Due, `7` Recent — and `0` resets the view to the launch view |
| Navigation | `j`/`k` or arrows move; `n`/`N` jump to the next/previous block — a section, a project group, or a folded group's summary — wrapping at either end; `h`/`l` or left/right walk one fold ladder over sections, child tasks, and checklist items, and `h` on a direct child of a focus root — the task `Enter` zoomed into, whose descendants Focus lists (see [`outliner-rows.md`](outliner-rows.md)) — steps out of that Focus frame onto the root; a task with no checklist, no children, and no parent left to step to folds its own project group, and `l` on the summary that replaces it puts the cursor back on the task the fold covered, or on the group's first task when there is none; `s`, `p`, and `t` are the three bulk fold toggles — every visible section header, every project group header, every task subtree — and each expands its whole set if any part of it is folded and otherwise folds the set; `gg`/`Home`, `G`/`End`, `Ctrl+U`/`PgUp`, `Ctrl+D`/`PgDn`; `z` opens the scroll chord, which moves the window under the cursor instead: `zt` puts the cursor's row at the top, `zz` centers it, `zb` puts it at the bottom; `Ctrl+O`/`Alt+→` jump backward/forward after a task relocation; `Ctrl+L` enters Recent on the newest activity; `gp` opens the cursor task's project in All and keeps the cursor on it; `gP` reveals the parent task and leaves the cursor on its row, stepping out of Focus when that parent is its hidden root; on a checklist item it jumps to the owner task; `g~` picks one of the cursor task's references and opens that task the same way, and `Esc` walks the hops back one at a time; `Enter` zooms in on whatever the cursor names — descendants on a task row, that project on a project sub-header, that section alone on a section header; `Alt+↑` widens focus one level; `b` opens and closes Peek's docked pane below the list, from any row, and `Ctrl+D`/`Ctrl+U` scroll an open one two rows at a time while `Ctrl+F`/`Ctrl+B` page it; all four belong to the open dock, and `PgUp`/`PgDn` keep moving the outline regardless |
| Capture and edit | `a` capture, `A` child, `c` add a checklist item to the cursor's task, `S` turn a checklist item into a child task or a child task back into a checklist item in its current parent, `D` repeat the cursor task as a new draft, `e` edits whatever the cursor names — the task inline, the checklist item, or the project a group header names — `E` description, `Ctrl+E` external editor, whose title line speaks the same edit grammar as `e`, `O` outline edit the cursor's project |
| Workflow | `w` opens the workflow chord over the ten destinations the `$destination` capture sigil names, under the two headings that say which kind of act each one is — Plan: `wn` toggle Next, `wt` toggle Today; Status: `wi` Inbox, `wb` Backlog, `wg` Doing, `ww` Waiting, `wp` Paused, `wm` Someday, `wd` Done, `wc` Canceled, and last `wh` hide/unhide, which is no destination: it toggles the stored hidden flag on the cursor task or the whole selection, hiding them all unless every one is already hidden, and the toast says `Hidden 1 task · fH to view · u undo` because the rows have just left every view. A plan is the collection you committed the task to and a status is the lifecycle state it holds, so planning a task for Today leaves it in Backlog and Peek says `Plan: Today · Status: Backlog`; entering Doing or Waiting without a schedule interrupts nothing and says so in the footer — `No check-in: <task> · F10: Set up check-ins` — naming the Task menu entry `F10` opens rather than a key that acts on a row, since the task it names may no longer be under the cursor, and only the first time in a session. The chord is the one place the keyboard spells a sigil differently: `$` stays what you type, `w` is what you press |
| Metadata | `!0` clears priority; `!1` High, `!2` Medium, `!3` Low; `@` due date, defer, and check-in schedule; `-` and `+` step the deadline one day earlier and one day later; `#` tags and context; `*` pin — those two read the cursor, so on a project group header they set that project's tags and toggle its pin; `=` toggles the review flag and `R` marks the cursor task reviewed and moves to the next candidate, and both name a task only |
| Structure | `[` outdent, `]` indent, `>` Set Parent, `<` Adopt Children, `%` Move to Project, `~` Set Dependencies, `J`/`K` reorder one step, `Ctrl+J`/`Ctrl+K` reorder to the far end of the visible peers |
| Sorting | `o` opens the sort chord: `o@` due date, `o!` priority, `oc` created, `ou` updated, `ot` title, `or` switch the held query between the flat ranked list slash opens and a narrowing of the mode, `ox` clear; repeat a field key to reverse it |
| Content | `&` opens the cursor task's Attachments panel, and `;` opens the Comments panel of whatever the cursor names — the task, or the project a group header names — the two capture sigils reaching, as keys, the panels that own add and delete, with reorder and unlink in Attachments alone; `C` appends one comment to the same two things without opening that panel, in a multiline popup where `Enter` starts a new line and `Ctrl+S` saves; `r` writes the cursor task's one-line resume note, prefilled with what it already holds — `Enter` saves, an emptied line clears it, `Esc` cancels, and unchanged text writes nothing |
| Clipboard | `y` title, `Y` description, `Ctrl+Y` both; each copies the whole visual selection, in displayed order, as one write |
| Selection | `v` toggles the cursor task; `V` starts/freezes an inclusive visible-task range; `Ctrl+A` replaces it with every visible task |
| Global | `Ctrl+P` Select Pinned Project, `P` Select Project, `Q` the narrowing readout, `\` Named Lenses, `Ctrl+N` Notes, `T` theme, `M` menu bar, `/` search, `?` help, `u` undo, `Ctrl+R` redo, `Esc` unwind/clear, `q`/`Ctrl+C` quit (`q` on the Outliner itself; inside a text input or picker it is just a letter) |

`@` opens one scheduling picker holding three sections. Its DUE section lists
Clear, Today, Tomorrow, This weekend, Start of next week, In one week, and In 30 days;
its DEFER section lists Clear defer, In 1 hour, In 4 hours, In 8 hours,
Tomorrow morning, Until tomorrow, Until the weekend, Until next week, For one
week, and For 30 days. This weekend is the coming Saturday and Start of next week
the coming Monday, each strictly after today: on a Saturday or a Sunday the
weekend row names the next Saturday, and on a Monday the next-week row is a week
away. It is not the `next mon` you can type, which skips the coming Monday. CHECK-IN contains one Set check-in
row that opens the same Next check-in and Repeat editor as the task action menu;
it is a typed editor action, never a deadline value. The row you commit decides
which field or editor is reached, and the cursor always opens in the due section, so a due date
is one `Enter` away. Where in that section it opens is the due date's business
alone: on the preset the task's own due date names; on Today when there is no
due date, so a reflexive `Enter` sets one; or — when a selection disagrees about
it, or when it is a date no preset reproduces — on DUE's own Clear row. A field
nothing holds keeps its Clear row listed but disabled with `no date set`, so `0`
there refuses instead of clearing nothing. The line under the title names what
is stored now — `Now: due none · defer none · check-in Thu Oct 22 every 7d` — the
pending deferral alone, and `mixed` for a field a selection disagrees about; it
wraps rather than being cut, the list gives up the rows it takes, and it is left
out when none of the three holds anything. A deferral never moves that cursor, so a
selection sharing a deadline but not a hide-until still opens on the deadline
they share. Opening the picker writes nothing either way.

`Tab` moves Due → Defer → Check-in and wraps; `Shift+Tab` moves backward. Each
skips sections with no search matches and lands on the next section's first
enabled matching row, or on its first matching row when every one is
disabled. When only one section has matches, the cursor and query stay put
without a message. The
footer names the field you are in — `a set due date` or `a set defer date` —
which is the field `a` types into and the one `Enter` commits.

Tomorrow morning is 09:00 on the local wall clock, on daylight-saving transition
days too. A preset is highlighted as the current value only when it is
reproducible from the clock alone, so the three hour rows never highlight. On a
terminal task the defer rows are disabled and say so rather than disappearing,
and the due rows stay reachable beside them; `Tab` still crosses onto those
disabled rows, which is how their reason gets read, and they keep refusing `a`
and `Enter`.

Entering Doing or Waiting without a schedule commits the state
change and opens nothing; the footer names what it left unscheduled and the
keyless `Set up check-ins` entry that schedules it. That entry, and the
`Set Check-In` entry on one task's `Space` menu, open the same editor: type a
date and optional repeat, save with `Ctrl+S`, and `Esc` leaves. Saving a
check-in has its own undo entry, separate from the state change.

Every preset row prints the key that applies it immediately, in a muted column
before its label, and pressing that key commits the row exactly as `Enter` on it
would. The two fields keep separate keyboards, because a shortcut means whatever
the section you are in says it means. DUE binds `0` Clear, `t` Today, `m`
Tomorrow, `s` This weekend, `o` Start of next week, `w` In one week, `n` In 30 days.
DEFER binds `0` Clear defer, `1`, `4`, and `8` for those hour offsets, `9`
Tomorrow morning, `m` Until tomorrow, `s` Until the weekend, `o` Until next
Monday, `w` For one week, `n` For 30 days. So `@m` is tomorrow's deadline, `@` `Tab`
`4` hides the task for four hours, and `t` in the defer section reports
`t is a Due shortcut` without changing anything. Hour keys in Due similarly
name Defer. On Check-in, preset keys say `Press Enter to edit check-in`. Where a row sits in the list never changes what
its key means, so a search that reorders the rows leaves the keyboard alone; a
search that filters a preset off screen says it is filtered out rather than
applying whatever is left, and a disabled row gives the same reason it gives
`Enter`. While you are typing a search or a specific date, the keys are text.

Inside the `@` picker, `a` opens a typed-date input for any date the relative
presets do not cover: `2026-01-15`, `tomorrow`, `+3d`, `+1w`, `fri`, `next mon`,
or `dec 15`. The due input also accepts an optional trailing `HH:MM` clock time
(`fri 15:30`, `2026-01-15 09:00`) to set an explicit time-of-day; a task with a
time compares at that instant, so it is overdue once the time passes rather than
at day's end. `+4h` and the other `+Nh` hour offsets parse only when the cursor
sits in the defer section, because a deferral is an instant and a due date is a
calendar day; the due section refuses them by name. The custom input names its
field: **Set due date** or **Defer until**. Valid input previews the resolved
weekday and date before Enter, including the clock for deferrals. Incomplete
input stays quiet until submission; editing rejected text clears its old error.
Inside the `#` picker the
same key adds a new tag, or a new context when the cursor sits on a context row.

`-` and `+` move an existing deadline instead of opening anything. They walk one
line — earlier dates, yesterday, no deadline at all, today, tomorrow, later dates
— one calendar day per press, weekends and month ends included. No deadline sits
between yesterday and today, so `+` on a task with no due date lands on today and
`-` lands on yesterday, and you arrive back at no deadline from exactly one side:
`+` off yesterday, `-` off today. A step onto today adds the task to Today the
way `wt` would. A deadline that carries a clock time keeps it; a deadline you
step off the line loses its clock, and pressing back toward it puts that clock
back, because the time is remembered for the rest of the session. Setting a due
date outright with `@` or an edit sigil replaces that memory, moving around the
outline keeps it, and quitting drops it.

Both keys take the whole visual selection, hidden members included, and commit it
as one transaction and one undo entry per press: a burst of four presses is four
steps to undo, in the order you typed them. Each press is frozen on the rows it
named, so moving the cursor, changing the query, or watching a row relocate out
of the filter while the save runs never redirects a step onto something else, and
the presses apply one at a time, each from the deadline the previous one actually
committed. One task reports the date it landed on; a selection reports how many
deadlines were written, how many were cleared, and how many of them you could not
see. A refused step stops the presses queued behind it and says how many never
ran, rather than retrying them against whatever is on screen by then. On a
section header, a project header, or a checklist row the keys refuse by name, and
while you are typing anywhere they are ordinary text.

`?` opens the generated key reference for the surface you are on, sectioned by
the same categories the operations declare, with a **Grammar** section holding
the sigils — `@@defer` among them — that no key spells. Its tabs fit one row at
80 columns and wrap, aligned, on a narrower terminal. `\`
jumps to a section, `/` filters the rows, and the filter reads vocabulary the
labels hide: `defer` finds `@` **Set Due, Defer, or Check-In** and `hidden` finds `f@`
**Filter by Due or Deferred**, each row naming the word that matched it.


Task updates that move the focused task—such as `wg` moving it into In
Progress—add a session-only jump entry. `Ctrl+O` returns to the task that
followed its old position (or the preceding/nearest surviving row), and
`Alt+→` returns to the relocated task. Changing mode, scope, search, filters,
sorting, folds, or focus clears this history. `Ctrl+I` remains an alias on
terminals that report enhanced keyboard events.

`D` repeats the cursor task: it opens the expanded composer, titled **Repeat
Task**, over a fresh draft seeded from that task. Pressing it writes nothing at
all — the draft is a starting point, and the ordinary save is what creates a
task, with its own identifiers, timestamps, and undo entry.

The draft keeps what describes the work: title, description, priority, tags,
context, and the checklist in its stored order with every item unticked. It
starts in the source task's own project, at the project root, in Backlog.
Everything that described the particular occurrence is cleared: parent, due
date, deferral, Today or Next membership, pin, review flag, resume note,
comments, attachments, and dependencies. Children are not copied either.
Nothing links the new task to the old one afterwards, and no recurrence is
recorded — a repetition is another task shaped like the one you were looking
at, and nothing more.

The title is seeded literally rather than re-read as capture syntax, so a task
called `Fix bug #123` repeats as `Fix bug #123` instead of picking up a `123`
tag. Terminal tasks repeat too, since finishing something is the usual reason
to want another like it, and a parked project you have explicitly scoped into
repeats like any other. Once the draft is open it is yours: editing or deleting
the source afterwards leaves it untouched, and `Esc` discards it having created
nothing.

`R` marks the cursor task reviewed and moves to the next task that still wants
looking at. It answers on one task at a time and refuses under a visual
selection, because the move to the next candidate is half of what it does and a
selection has no next.

A candidate is an open task you raised yourself: one flagged for review with `=`,
or one whose check-in day has arrived. Done and canceled tasks are never
candidates, and a deferred task is one only if you flagged it yourself — hiding
something until next week is already an answer to "is this still relevant". A
task that has merely sat untouched, or that is Waiting on someone else, is not a
candidate: that is cleanup you go looking for with `fs` and `?stale`, not work
that asked for you. Pressing `R` on anything else refuses and says why.

You may also press `R` on a task whose check-in is still ahead, and answer it
early. That is the one way the two differ: naming a task yourself admits any
scheduled check-in, while the pass below only ever walks onto one that is owed.

Marking clears the review flag, records when the review happened, and updates the
task's timestamp. A repeating check-in moves on from the moment you answered, not
from the day it was owed, so answering three days late does not make the next one
three days early; a one-off check-in is done and clears. Nothing else changes:
the state and how long it has been in it, deadlines, deferral, Today or Next
membership, pin, resume note, comments, and body all survive. A Waiting task is
still Waiting afterwards, keeps its separate deadline, and still counts the
days it has been waiting — acknowledging that you have seen it is not the same as
hearing back. There is no review queue accumulating anywhere.

The cursor then moves down to the next candidate visible right where you are: it
skips headers, checklist rows, and tasks that are not candidates, and it stays
inside the view you are in. Folds stay folded, the lens and filters stay as they
are, no other project is loaded, and the pass does not wrap around to the top.
If a task that was going to be next is gone by the time the cursor gets there —
completed in another window, filtered away by what you just did — it is skipped
too. When nothing is left below, the cursor stays where it is and the status
line says there are no further visible review candidates, which is how a pass
ends. A refused `R` moves nothing at all, and `u` takes the mark back while
leaving the cursor where the pass had carried it.

`=` is unchanged: it flags and unflags, on any task, and never moves the cursor.

`#` opens Set Tags & Context for the current task or visual selection. Inside
the overlay, `Space` toggles tags, `a` creates and selects a new tag, and
`Enter` applies the pending changes. A task carries any number of tags but at
most one context, so the `CONTEXTS` section is a radio group answering `#`
rather than `Space`: the row you press it on becomes the only checked context,
and the leading `No context` row clears the field the way `##0` does. Tags and
context apply together, in one transaction that undoes in one step.

The overlay opens on what the task already carries. Every applied tag and its
context are promoted to an `APPLIED` section at the top, in the same semantic,
context, descriptive order the sections below keep, and the cursor starts on the
first of them — so the values you are most likely to remove are the ones you
land on. `No context` is not one of them: a task with no context agrees with
that row without carrying a value, so it stays in `CONTEXTS`. That order is
fixed for as long as the overlay is open. Checking an unapplied value marks it
where it stands rather than moving it up, searching and clearing the search
leave it where it was, and a tag you create with `a` lands in the section its
own name belongs to. Reopening the overlay is what recomputes the order, out of
whatever you just applied.

Over a visual selection those checkboxes are tri-state, because the tasks need
not agree. A tag row is checked when every selected task carries it, `[-]` when
only some do, and empty when none do; applying adds every row you checked,
removes every row you unchecked, and leaves every `[-]` row exactly as it is, so
a tag only part of the selection carries survives an edit to the tags beside it.
`Space` on a mixed row checks it for the whole selection: a mixed row can be
left but not re-entered, and canceling the overlay is how you restore an
untouched set. The `CONTEXTS` rows read `[-]` the same way, marking a context
only some of the selection already holds, and they write nothing unless you pick
one; picking `No context` clears the field across the whole selection the way
`##0` does. A context named `0` is refused, because that name is what clears the
field. `APPLIED` leads there too and holds both kinds together: a value every
selected task carries and one only some of them carry are equally worth
reaching, and each keeps its own mark, so the section says what the selection
has without saying the two are the same.

`[` and `]` move the cursor task one step along the tree it is already in: `[`
outdents it to its parent's level, `]` nests it under the peer above. They are
the two structural keys that answer no sigil, because a relative nudge has no
capture form to name. `>` opens
Set Parent, the picker form of the `>parent` edit sigil. It lists every open
task in the cursor task's project, above a leading `No parent (project root)`
row that does what `>0` does. A parent the move would reject — the task itself,
a task inside its own subtree, or one that would push the subtree past six
levels — is still listed, disabled and carrying the reason, so the refusal is
visible before you press `Enter`. Set Parent never crosses a project boundary;
`%` is what moves a subtree between projects.

`<` opens Adopt Children, the picker form of the `<child` edit sigil, and points
the other way: it hands the cursor task new children instead of choosing its
parent. Because `<child` is repeatable, the list is multi-select; because it has
no clearing form, the list is strictly additive. Nothing is preselected, `Space`
checks the tasks to adopt, and `Enter` adopts the whole checked set in one
transaction that undoes in one step. Tasks the cursor task already parents are
absent from the list, so unchecking can never mean detaching — a child leaves
its parent through `>parent` or `>0` and nowhere else. Candidates come from the
cursor task's own project only, and one the adoption would reject is listed
disabled with the reason.

`%` opens Move to Project, the picker form of the `%project` edit sigil, and is
the one key that carries a task out of its project. It moves the whole subtree
in one transaction that undoes in one step, and the destination mints its own
`#N` numbers for every task it receives. The list names each project by slug and
skips the one the task already lives in, since a move to it is not a move. It is
drawn from every non-archived project rather than the ones the Outliner shows,
so a parked project is still a destination — the task simply follows it out of
view. `P` remains Select Project, which changes what you are looking at rather
than where a task lives.

Over a visual selection the same `%` moves every selected subtree into one
destination, in one transaction with one undo entry, however many projects the
selection spanned. The picker names the selection rather than a title
(`Move to Project: 3 selected task(s)`) and withholds a project only when the
whole selection already lives in it, since for a selection spanning several
projects every project is a real destination for the tasks not already there.
A task selected together with one of its own ancestors moves once, as part of
that ancestor's subtree, so the count the status strip reports
(`Move to Project: 3 task(s) → client-alpha`) is subtrees moved, not rows
ticked. A committed move clears the selection, because every address it held
names the project the tasks just left.

`~` opens Set Dependencies, the picker form of the `~task` edit sigil, and edits
the soft references the cursor task stores. It is multi-select and, unlike Adopt
Children, it arrives with the stored set already checked: the edit sigil owns
both `-~task` and `~0`, so unchecking a row has a meaning to carry. `Space`
toggles a row, and `Enter` commits checking as adding and unchecking as removing
— both halves in one transaction that undoes in one step. Closing the picker
having changed nothing commits nothing and reports `Dependencies unchanged`.

Over a visual selection it edits every selected task at once, the way the tags
editor does. A row every selected task stores arrives checked, one only some of
them store arrives in a third state that is neither, and leaving that row alone
is what keeps the tasks that disagree disagreeing: only the rows you actually
move are applied, checking one as an addition everywhere and unchecking one as a
removal everywhere. No selected task is offered as a row, because naming one
would be a reference inside the selection, and adding one by any other route
refuses by name.

The list is two halves. Every reference the task already stores gets a row,
whatever state its target is in and even when the target cannot be read from
here — a reference into a project the Outliner has not loaded keeps its stored
`project-id:task-id` locator and reads `unavailable`, and it stays checkable, so
a stranded reference can always be dropped. The other half is every open task in
every loaded project except the tasks being edited, grouped under its project's
name, which is the whole population an addition may name. References cross
project boundaries, so that grouping is what keeps a filtered list unambiguous.
Dependents are never listed: they are computed from what other tasks store, and
no key edits them.


## Task actions

`Space` contains exactly:

1. Set Check-In
2. Delete

Set Check-In is listed under **Schedule** and Delete under **Danger**. The sheet,
for one task or a selection, opens directly under the cursor's row, or just
above it when the row is too near the bottom of the screen.

`j`/`k` move through the list and `/` opens its search, as in every other
action sheet: typing filters only once the search is open, `Enter` leaves the
search with its matches standing, and `Esc` peels one layer per press — the
query, then the search, then the sheet. Frequent direct operations do not repeat
in this menu. Set Check-In opens one draft with **Next check-in** and **Repeat**
fields. A single task starts with its stored schedule; a bulk
selection starts blank and says that saving replaces every selected schedule.
Use `Tab`/`Shift+Tab` between fields, `Enter` for the current field's presets,
and `Ctrl+S` to save. Dates accept the relative spellings used by the date
controls or a custom calendar date. Repeat offers Off, 1, 3, 7, 14, and 30 days,
or a custom positive day count. Date presets and valid typed dates show their
resolved weekday and calendar date; `+30 days` means exactly 30 calendar days.
Repeat presets open on the current interval, including a custom interval shown
above the standard choices. A repeating draft explains that the next reminder
falls that many days after you acknowledge it. Clear is an explicit date choice;
an empty bulk draft never clears schedules. `Esc` closes a nested preset list first and
cancels the editor from its field view. Validation and save failures keep the
draft open for correction. Delete always asks for confirmation. The checklist
is absent because it is edited on its own rows, and a second route to the same
mutations is the shape this menu exists to avoid. Comments and Attachments left
this menu when they gained `;` and `&`, which is that same rule: a panel reached
by a key is not also a row. The single-task sheet remains identical to the bulk
one, with the latter applying one explicit schedule replacement atomically.

For example, keep a vendor deadline on Friday and set a repeating check-in for
Tuesday every 3 days. Reviewing on Monday schedules the next check-in for
Thursday; reviewing late on Wednesday schedules it for Saturday. The Friday
deadline stays put, and neither check-in adds the task to Today. Changing the
check-in date alone keeps any manual review flag and the previous review time.
Deferral hides scheduled reminders until wake; completion or cancellation clears
the schedule, so reopening retains review history without restarting it.

To schedule existing active work, press `F10` or `Ctrl+M` and choose **Task** →
**Set up check-ins**. `M` only keeps the bar on screen; `F10` opens the menu even
on terminals that send `Ctrl+M` as `Enter` (see
[`outliner-chrome.md`](outliner-chrome.md)). The action also appears in `?` help.
It walks unscheduled Doing and Waiting tasks
across every unparked, unarchived project, including tasks hidden by the current
lens, query, scope, or folds. Projects sort by slug and tasks by local number;
the initial list stays fixed for the pass. Each editor shows the project, task,
position, remaining count, and saved/skipped results.

`Ctrl+S` saves one schedule and advances; each save has its own undo entry.
`Ctrl+N` skips without writing, and `Esc` exits setup even from a preset list.
Clear followed by Save also skips an unscheduled task. If a save is already in
flight, Escape exits after its outcome is known. Save failures retain the draft
for correction or retry. Tasks deleted, moved, completed, parked, archived, or
scheduled elsewhere are rechecked and counted as no longer eligible rather
than overwritten. An unreadable active project prevents a complete workspace
read and is reported as a failure, never silently omitted.

The original view is restored on exit. Saved schedules remain, so invoking the
action again starts a fresh pass over work still unscheduled; there is no saved
queue or automatic reminder to finish setup.

The same key on a project group header opens a different sheet, because the row
names a project rather than a task. Project Actions holds Archive project and
Delete project under Danger, and nothing else. Editing left this menu when the
header gained `e`, and pinning left it when the header gained `*`, the same rule
that took Comments and Attachments out of the task menu. Opening the project and
opening its notes are absent for the reason above: `Enter` and `Ctrl+N` already
reach both from that row.

The Attachments panel that `&` opens carries the same split. `Enter` opens
the attachment under the cursor, `a` adds one, `E` opens it in the external
editor, `y` and `Y` copy its path and its `path:line` form, and `J`/`K` reorder
it. Its own `Space` sheet holds the five acts a direct key would have spent for
nothing:

1. Reveal original
2. Open original
3. Open attachments folder
4. Unlink
5. Delete

Unlink and Delete both confirm, and neither has a direct key: removing an
attachment is now a deliberate menu row rather than a single press. `u` and
`Ctrl+R` reach the same workspace ledger they reach everywhere else, so an
attachment added or removed by mistake comes back without leaving the panel, and
the status strip names the transaction that moved.

The Comments panel that `;` opens splits the same way. `Enter` expands the comment under the
cursor, `a` adds one, `e` edits it inline, `E` opens it in the external editor,
and `y` and `Y` copy that comment and every comment as Markdown. Its own `Space`
sheet holds Delete alone, which confirms and has no direct key, so a comment
leaves the list only through a deliberate menu row. `u` and `Ctrl+R` reach the
same workspace ledger here too, so the `u undo` the strip offers after a comment
is added, edited, or removed is answerable without leaving the panel, and the
list rebuilds around what came back. `?` lists that grammar
rather than the Outliner's, and both it and `u` stay literal characters while a
comment is being typed.

The expanded comment is a reader, not a wall of text: it shows as much of the
body as the terminal has room for and scrolls the rest. `j`/`k` and the arrow
keys move a line, `Ctrl+D`/`Ctrl+U` and `PgDn`/`PgUp` move half a screen,
`Ctrl+F`/`Ctrl+B` move a screen, and `gg`/`Home` and `G`/`End` go to the ends. A
line under the body counts where you are whenever there is more comment than
window, and a comment short enough to fit shows no counter at all. `Enter` and
`Esc` both close the reader and leave the cursor on the comment you were
reading, which reopens at the top.

`;` on a project group header opens that same panel over the project's own
comments. Everything the panel does for a task it does here — expand, add, edit
inline or in the external editor, copy one or all, and the `Space` sheet's
confirming Delete — and `u` and `Ctrl+R` still answer for the mutation. Two
things read differently. The list runs newest first, because a project's
comments are a running log and the last line is the one you opened the panel
for. And a comment you edit or delete is the one you were looking at rather than
the row it sat on: if it was rewritten or removed from somewhere else while your
editor was open, the save is refused, your text stays in front of you, and
nothing else in the list is touched. Editing keeps the comment's original
timestamp. The panel follows the project itself, so renaming it, parking it, or
committing a change from another surface leaves the panel reading the same
project rather than a stale copy; archiving it leaves the comments readable and
refuses the change, naming the reason.

`C` on a project group header writes a comment onto the project instead, in the
same popup under the same 2,000-character bound: `Enter` starts a new line,
`Ctrl+S` saves, a terminal paste arrives whole, and `Esc` on a non-empty buffer
asks before discarding it. The subtitle line names the project so the two
meanings of the key are never a guess. The project the popup names is fixed when
it opens, so a refresh, a rename, or a reordering underneath it cannot redirect
what you typed onto a different project — and if that project turns out to be
gone, archived, or unwritable when you save, the popup comes back with your text
and the reason rather than dropping either. A parked project takes comments like
any other, and so does Unfiled. Saving leaves Peek exactly as you left it, open
or closed, and `u` takes the comment back the way it takes back anything else.

Saving is not instantaneous, and the composer stays in front of you while it
runs: your text is held there until the transaction answers, so a refusal has
something to give back. For that moment the box is read-only. A key that would
change it is turned away and says so, because nothing typed after `Ctrl+S` could
survive either answer — a confirmed save closes the composer, and a refused one
puts back exactly what was sent.

Text that comes back from the external editor can be longer than the composer
will hold: the box stops at 2,000 characters and writes tabs as spaces, so when
a save is refused the panel keeps the editor's exact result beside it and shows
the end of that result under the box. `Ctrl+X` writes the complete draft — every
character, interior tabs included — to a file of its own and names the path on
screen. It asks the comment's owner for nothing, so it answers just as well when
the comment was deleted or the project is gone, and it writes nothing else: the
refusal, your text, and the stored comments are all exactly where they were.

A short terminal cannot hold all of that at once, and what it drops is chosen
rather than cropped. The excerpt under the box shrinks and then goes, and the
sentence explaining it goes after that; what stays at every size is the refusal,
the line saying the box is a preview and that `Ctrl+X` writes the whole draft,
the keys along the bottom, and the path a `Ctrl+X` already spent. None of it is
about the draft itself — the complete text is kept whatever the terminal is doing
— and `Esc` still discards it and nothing else does.

A comment already in the workspace can outrun the box the same way, so opening
one with `e` is treated the same. If it is longer than 2,000 characters or
carries interior tabs, the box becomes a read-only preview of the stored comment
rather than a buffer for it: the whole comment is kept beside it, the end of it
is shown under the box, and `Ctrl+S`, `Ctrl+E`, and `Ctrl+X` each carry every
character of it. Saving without typing anything writes the comment back exactly
as it was. A press that would have changed the preview is refused and says so —
the part of the comment you cannot see is not something a keystroke over the
part you can see is asking to discard — so `Ctrl+E` is the way to edit one, and
it opens the whole comment in the external editor. A comment the box does hold
whole is none of this: it edits inline the way it always has.

Both panels and All Notes move the way every other list does: `j`/`k` or arrows
step one row, and `gg`/`Home` and `G`/`End` reach the first and last. A `g` you
do not complete is abandoned silently and does not swallow the key after it, so
a mistyped prefix costs one press rather than the act the next key names.

Press `e` to edit the selected task in place. Append `%project` to move its
subtree, `>parent` to reparent it, `>0` to move it to the project root, or
`<child` once per task you want to adopt; these structural changes commit
atomically with title and metadata edits. `~task` adds a soft dependency here
too, `-~task` removes one, and `~0` clears them all; a line mentioning none of
the three leaves the stored set exactly as it was, which is what keeps an
external-editor title edit from quietly dropping references it never displayed. `Ctrl+E` opens the same task in an
external editor and reads its title line with the same grammar, so the title,
its sigils, and the body below commit as one transaction and one undo step. The
edit surface is also the only one that accepts `*unpin`: capture has nothing to
clear, and the outline buffer unpins by dropping the `*pin` token from the line.

A title `Ctrl+E` returns is parsed only when it differs from the one the buffer
was opened with, so a literal `Fix bug #123` saved untouched keeps its `#123`. A
title whose value the grammar recognizes but cannot honor — an unknown
`$destination`, a `%project` that does not exist, a reparenting that would make
a cycle — commits nothing and reopens the same buffer with the refusal in its
instruction comment and the description you already typed still in place. Saving
that buffer again unchanged abandons the edit and reports the refusal.
