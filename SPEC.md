# Docket v2

A keyboard-only terminal outliner for one person with dozens of projects.
Every row is edited in place, metadata is typed as sigils, and a sidebar shows
which projects need attention. This file is the contract: a change that breaks
a rule here changes this file first.

## Rules

1. **Nothing asks a question.** No confirmations and no dialogs. Delete is
   instant; undo is the safety net.
2. **Rows never move by themselves.** Changing a row's state leaves it where
   it is. Only the user reorders.
3. **One way to do each thing.** No menu or palette repeats a key. Reaching
   the same rows by a count, a selection or a motion is the grammar, not a
   second route.
4. **Keys follow a grammar.** A key in the outline is a motion, an operator,
   or an act on rows; a count goes before any of them. A key that is none
   of these is not added. The five characters that open the stamp line
   (`#` `@` `$` `*` `-`) are the row grammar typed on a row. The navigation
   aliases are the arrows, `Ctrl+F` `Ctrl+B`, `PgDn` `PgUp` and `Home` `End`.
5. **Every keystroke paints within one frame.** The tree lives in memory;
   SQLite is written when a change is committed, never while typing. What
   the sidebar states is derived again only when a row has changed or the
   minute has turned, so a typed letter never walks the outline. A test
   holds moving and typing to 16 ms on an outline of 10,000 rows.
6. **Chrome is the sidebar, one header line and one status line.**

## Model

- One kind of row, in a tree. The root is the empty ID; top-level rows are
  projects. A row with a state is a task; a row without one is a note or a
  heading.
- A row is `id`, `parent`, `pos`, and a set of named string fields. Fields are
  the truth; the sigil line on screen is rendered from them and parsed back
  into them.
- Every change is one of three operations: `insert`, `set` (one field of one
  row), `move` (parent and position). There is no delete operation: a row is
  retired by setting its `deleted` field, so every operation is undone by
  another operation.
- `pos` is a fractional string key, so a row is placed between two siblings
  without renumbering either.
- A move under the row itself or one of its descendants is refused.

### Fields

| Field | Milestone | Meaning |
| --- | --- | --- |
| `text` | 1 | The row's title |
| `deleted` | 1 | Tombstone |
| `state` | 2 | `open`, `doing`, `waiting`, `done`, `canceled`, `parked`; unset is a note |
| `flag` | 2 | The single priority mark `!` |
| `due` | 2 | A hard deadline, a calendar day. Never "when I plan to do it" |
| `tags` | 2 | Space-separated, normalised |
| `changed` | 2 | When `state` last changed, for Waiting and Stale ages |
| `today` | 3 | The day the row was picked for Today |
| `todaypos` | 7 | Fractional position in Today; read only while the row is in Today |

| `start` | 5 | Not before this day, a calendar day |
| `repeat` | 5 | An interval, as in `2w`: the row comes back after it is closed |

## Storage

SQLite, one file, local to the machine. It must never sit in a folder a file
sync tool copies.

- `ops` is the append-only log: every operation with a stamp.
- `rows` and `fields` are the current state, written in the same transaction
  as the log entry. `fields` keeps one stamp per field and `rows` one for the
  last move, which is what a later merge needs to resolve conflicts per field.
- A stamp is `milliseconds-counter-device`, issued by a clock that never runs
  backwards, and sorts bytewise in issue order.
- `folds` and `meta` are this machine's view state and settings. They are
  not logged.
- Adding a field needs no schema change.

**A save that fails changes nothing.** If a change cannot be written, it is
taken back out of memory and the status line says so; the screen never shows
what the disk does not hold.

**Undo is kept.** The undo and redo stacks, 200 steps deep, are stored on
this machine beside the folds and survive a restart. They are not logged and
will not sync: each step undoes a change made here.

**Backups.** The first launch of each day writes a whole copy of the
database to `backups/docket-YYYY-MM-DD.db` beside it, before the outline is
read or changed, and keeps the newest 14. A session left open takes each
later day's copy when the day turns over. Restoring is copying one over the
live file. A failed backup is said on the status line and does not stop the
work.

**A copy is another machine.** A database remembers the machine and the
path it was named on. Opened anywhere else, as a file copied to a second
machine or to a second place on this one, it takes a new sync name and says
so once. Two databases never share a name, since each would write the same
sync file and pass over the other's lines as its own. A backup restored
over the file it was taken from is not a copy and keeps its name.

### Sync

`:sync folder` shares the outline with other machines through a folder that
any file syncing tool carries. The database never goes in that folder.

- Each machine appends its own operations to `<device>.ops` there, one JSON
  object a line, and reads every file. No file has two writers. A last line
  the machine never finished writing is cut before the next is added.
- A remote operation is applied by its stamp, per field: the newest write to
  a field wins, and the newest move of a row. Renaming a row on one machine
  and closing it on another keeps both. The same field written on both keeps
  the later one; the earlier stays in the log.
- Applying is idempotent and does not depend on arrival order, so machines
  that have seen the same operations hold the same outline.
- An operation on a row, or under a parent, that has not arrived yet waits
  until it has. A last line still being copied is left for the next pass.
- Two machines can each move a row under the other's. Every machine breaks
  that loop the same way when it loads: the row in it that moved last is
  shown at the top level. Nothing is lost.
- After reading, the clock takes the newest stamp seen, so what is written
  next is newer than everything received even if the wall clock is behind.
- A machine reads its own file too, so a database restored from a backup
  gets back what the backup lacks.
- A copy that took a new name writes out, once, everything it held when it
  was copied, whoever made it: the machine it came from may never sync.
- Each file is read from where the last pass stopped, so a pass costs what
  is new and not the length of the log. A file found replaced, or a folder
  not used before, is read from the top.
- A whole line that cannot be read stops that file there, since a tool
  copying in pieces may yet fill it in; nothing after it is applied. If the
  next pass finds it unchanged the sidebar says `sync stuck` in the alarm
  colour and the status line names the file and the line, until the file is
  whole again.
- Sync runs at launch, once a minute, on quit, and on `:sync`. The work in
  the folder is done away from the keys, so a slow folder never holds up a
  keystroke; only the pass on quit is waited for. A pass does not start
  while a line is being typed, and one that ends then is applied when the
  line is kept. `:sync off` stops it.

Undo on one machine is a new operation, so it reaches the others. A row
added under a row that another machine deleted meanwhile is hidden with it,
and returns if the delete is undone. Folds, settings and the undo stacks
stay on their machine. The log is never compacted.

The sidebar's last line says `synced HH:MM`, with a count of operations
still waiting for a row, or `sync failed` or `sync stuck` in the alarm
colour; the status line gives the reason. Work goes on either way.

Convergence is tested by three machines making random changes, moves that
loop and deletes among them, while their files reach one another late, out
of order and cut short. Once every file has arrived whole, all three must
hold the same outline, with every live row reachable.

### Two windows

Two windows may be open on one database. Each notices the other's writes
at its next key, at each save, and once a minute, and reads the outline
again; it waits while a row is being typed, since that row exists only
there. A write passes every stamp already in the log, so no two stamps are
alike and the latest write is the newest whichever window made it. Each
window keeps its own undo history for the session; the stored one is
whichever was written last.

## Screen

```
  Inbox                1 │ docket-rewrite
  Today                3 │   ▾ Storage:
  Waiting              2 │ ▌   ○ Decide op-log format ! @fri
  Stale                9 │         per-field, clock-stamped
                         │     ● Prototype row parser
▌ Projects               │     ✓ Audit grammar reuse
  docket-rewrite         │   ▾ TUI:
  blog               12d │     ○ Sidebar counts
  tax-2026         ! 3d  │
  homelab              ∅ │
                         │
  parked 31              │
Inbox › Buy milk %home
```

### Sidebar

Drawn from 72 columns up; narrower, it takes the pane only while focused.
Everything in it is derived by one pass over the tree. Nothing is stored.

- **Four lists, and they stay four.** Inbox counts its rows; Today counts
  picks not yet closed; Waiting counts rows in that state; Stale counts open
  and doing rows untouched for 14 days, leaving out rows whose start day is
  still ahead.
- **Projects** shows every top-level row as one outline. It is where a
  project is added, renamed, reordered, parked or closed, with the ordinary
  keys.
- **Active projects** follow, in the order of the top-level rows: the order
  is the user's, so it never shuffles as work happens. Each says `!` in
  yellow for a deadline within a week or red for one overdue, then `∅` when
  it holds no task to do, else the whole days since anything inside changed.
- **Parked** projects are a count. Done and canceled projects are not listed.

`Tab` gives the sidebar focus. `j` and `k` then show each destination at
once, as do `gg` and `G` or `Home` and `End` (first, last), `Ctrl+D` and
`Ctrl+U` (half a page) and `Ctrl+F`, `Ctrl+B`, `PgDn` and `PgUp` (a page),
where a page is the pane's usable row height in entries, stopping at either
end. `Enter`, `Tab` or `Esc` returns to the pane.

### Pane

The pane shows the rows under one row, or a list.

- **A project, the Inbox, or Projects** is the outline, edited where it sits.
  The Inbox is an ordinary row with the fixed identity `inbox`, kept first
  among the top-level rows and never drawn among them. The `filters` row is
  the other such row.
- **Today, Waiting and Stale** are flat. A list is read when it is opened and
  holds those rows until it is opened again, so closing or rescheduling a row
  never pulls it from under the cursor. Each row names its project; Waiting
  and Stale lead with the longest wait and say how many days. Today alone
  has an editable order: `J` and `K` move the cursor's row down and up,
  keeping the cursor on it without moving it in the outline. They refuse
  selections; Waiting, Stale and review refuse both keys. `o`, `O`, `>` and
  `<` are refused in every list. `Enter` works inside a row, from which
  `Esc` returns to the same list. `h` opens the cursor's row among its
  siblings in its parent's outline; `Esc` returns to the held list on that
  row. In review, a top-level row opens Projects. The row stays visible
  even with `:closed` on. `h` refuses a selection, and `l` does nothing;
  neither changes a fold.
- **Launch** opens Today when something is picked for the day, else Projects.
- **A row wraps** under its own text at the pane's width, each word keeping
  its colour, so nothing is cut; a word wider than the pane breaks across
  lines. The cursor and selection marks run the row's height, and scrolling
  keeps the cursor's row whole when it fits. A row wraps as it is typed;
  when it is taller than the pane, scrolling keeps the caret's line visible.
- **Each place remembers its cursor** for the session, so coming back to a
  project lands on the row you left.
- **Closed rows can be left out.** `:closed` hides done and canceled rows,
  and what is under them, from outlines; the header says so and the setting
  is kept on this machine. A row closed while on screen stays until the
  place is left. Lists and filters name what they show, so they still show
  closed rows.
- **The screen follows the clock.** It is read again once a minute, so a day
  that turns over while the app is open updates Today, deadlines and ages
  without a key press.

Today is hand-picked and rebuilt daily: a row is in Today when its `today`
field is the current day, so yesterday's picks drop out without a write. A
task whose start day is today is in Today too, which is how a row is made to
come back on a date. Picks are kept in the order made: `t` assigns a fresh
`todaypos` after the last placed row, overwriting any earlier day's place
in the same undo step. Unpicking leaves that place stored. Rows with a place
come first, sorted by it; rows without one follow in outline order. Before
`J` or `K` moves a row, any missing places are filled in the order shown,
in the same undo step. Ordering is kept across reopening and restarting.

### Bottom line

The status line becomes a prompt for three acts, named at its left edge:

- `a` **Inbox ›** files a typed line in the Inbox. A `%name` word files it in
  the project or heading that name matches instead; the line names where it
  will land. One matching nothing stays in the text. `Enter` keeps the line
  and asks for the next; a blank `Enter` or `Esc` closes, and `Esc` keeps
  what was typed.
- `f` **Go to ›** opens a project, a heading, a list or a saved filter by
  name. A heading opens inside its project: the sidebar stays on the
  project and `Esc` backs out to it. After a jump, a blank query offers
  `back` and the name of the place left first; typing a name drops the
  offer. Taking it restores that place's held rows, filter, zoom, sidebar
  entry and cursor. It is itself a jump, so blank `f` `Enter` alternates
  between two places. A jump is a picker destination opened with `f`
  `Enter`, or a sidebar visit ending in a different place; walking several
  sidebar entries counts once, from where `Tab` was pressed. `Enter`
  zooming, `Esc` backing out and `h` are not jumps. One place is remembered
  for this session only, and is not offered if its root row is no longer
  live. Until a jump, blank `f` `Enter` opens the Inbox as before.
- `m` **Move to ›** makes the cursor's row or selection the last children of
  a project, the Inbox or a heading, in screen order. A heading is any live
  row ending in a colon under a project or the Inbox, at any depth; rows
  under a deleted row are excluded. Headings are labelled by their path,
  with colons dropped: `project › TUI › Sidebar`. Projects precede headings
  on equal match ranks. Moving under oneself or a descendant is refused.
  With rows selected and the cursor on a row that is neither selected nor
  under a selected row, a blank query offers `here` at the front, so `m`
  `Enter` moves the selection where `o` would open a row, as described under
  Selection. Typing a name drops the offer.

The two pickers list their matches as you type, best first. `Down` and `Up`
walk them, and `Enter` takes the one at the front. A name matches by its start, the start of a word, anywhere
inside, or its letters in order. `Esc` leaves a picker without acting.

Three more acts use the same line:

- `/` **Filter ›** narrows the pane as the query is typed. Words match a
  row's text; `!`, `#tag`, `$state` and `@date` match its fields; a leading
  `-` excludes. `#` alone is any tag, `$` alone is any state (a task),
  and `@` alone is any deadline; `-#` is untagged and `-$` is notes or
  headings. `@overdue` and `@week` read the calendar. `@<date` holds for an
  open row due on or before that day and `@>date` for one due on or after it,
  in any form `@date` takes
  (`@<+30d`, `@<fri`, `@>12-31`); like `@overdue` they leave out done and
  canceled rows, so `-@<+30d` is everything else, rows with no deadline
  included. `~7d`, `~2w`, `~1m`, `~1y` and `~today` match a row whose
  state last changed that recently, or which was made that recently if it
  never had one, so `$done ~7d` is what was finished this week and
  `$waiting -~2w` what has waited longer than two. Every term must hold. A
  bar inside a word offers alternatives, and the word holds when any does:
  `$open|doing`, `#home|errands`, `milk|eggs`. Each side may carry its own
  sigil (`#home|!`, `$waiting|@overdue`, `milk|#food`); a side without one
  inherits the first side's sigil. If a side cannot be read as its own or
  inherited sigil, the whole word is text. Each side is coloured and
  completed according to its sigil. A leading `^` tests the term on any
  ancestor, from the parent up to the project, excluding the row itself:
  `^#work` finds rows under a tagged row, `^docket $next` a project's next
  action, and `^$waiting` rows under a waiting row. `-^#work` means no
  ancestor has that tag; `^#work|home` accepts either tag. Write `^` once,
  before the alternatives. Ancestors above the pane's root count too,
  including in lists. A bare `^` is skipped while typing; `^^` starts
  literal text. Colour and completion use the term after `^`.
  `$next` holds for each active project's next action: its first row, in
  outline order, that is open or doing, has no start day still ahead, and is
  not parked, under a parked row, or in a closed project, so the order of a
  project's rows is its priority. It is one row per project, not per
  heading; the Inbox has none.
  It combines like any state word (`$next #home`, `-$next`,
  `$next|waiting`), so `/$next`, `Tab` and
  `:save next` keep the daily list of what could be done. In an
  outline a match keeps its
  ancestors, dimmed, so it is read where it lives; `Tab` switches to a flat
  list of matches and back. When the query has an un-negated deadline term
  (`@`, `@date`, `@overdue`, `@week`, `@<`, `@>`), the flat list reads
  soonest deadline first, ties in outline order; in context, and in Today,
  Waiting and Stale, the order is the outline's or the list's. The order is
  set when the query is read, and nothing in the outline moves. `Enter` keeps the filter and returns to the rows,
  where the header names it; `Esc` drops it. Like a list, a filter holds the
  rows it found until the query changes, and refuses the keys that
  restructure the tree. Changing the filter clears the selection. A filter
  shows every match: a fold never hides one. In context, `h` and `l` only
  step to the parent and the first child. In a flat filter, `h` opens the
  cursor's row among its siblings in its parent's outline, keeping it
  visible even with `:closed` on; `Esc` returns to the held filter with
  its query and cursor intact. `h` refuses a selection and `l` does
  nothing there. Neither changes a fold.
- `#` `@` `$` `*` `-` **Stamp ›** opens holding that character. The line is
  sigils only: `#tag`, `@date`, `@@date`, `*interval`, `$state` and `!` set;
  `-#tag`, `-@`, `-@@`, `-*` and `-!` remove. `Enter` applies them to the selection, else to the cursor's row, as
  one undo step. A word it cannot read is named and nothing is applied.
- `:` runs a command: `q` quits; `help` opens help; `export [file]` writes
  the whole outline as tab-indented lines in the row grammar, to `~/docket-YYYY-MM-DD.txt` unless
  a file is named; `closed` hides or shows closed rows; `all` selects every
  row the pane shows, as described under Selection; `archive` moves
  closed rows under an `Archive:` heading, as described below; `review`
  opens the review list; `fold [level]` and `unfold` control folds as described below;
  `sort [keys]` orders siblings once, as described below;
  `save name` and `forget name` keep and drop a filter; `sync
  folder` and `sync off` are described under Sync.

`:fold` folds every row with children under the pane's root. `:fold N`
shows N levels: rows above level N are unfolded and rows at level N are
folded, with the root's children at level 1. Deeper folds stay as they were.
`:unfold` unfolds everything under the pane's root. An invalid level is
named on the status line and changes nothing. Lists and filters say
there is nothing to fold. The cursor stays
on its row, or moves to its nearest visible ancestor. Folds are saved together
as this machine's view state, never logged or synced.

`:sort [keys]` sorts the cursor's row and all its siblings, or a selected
block of adjacent siblings under the Selection rule. Selected descendants
travel with their selected ancestor. Children travel with their rows and
are not sorted. Keys are space-separated: `@` puts soonest deadlines first
and missing deadlines last; `!` puts flagged rows first; `$` orders doing,
open, waiting, notes, parked, done, canceled. Each later key breaks ties;
without keys, text is compared ignoring case. Ties keep their original
order. Rows stay inside their original sibling span, in one undo step,
with the cursor on its row and the selection kept. The status says how many
rows were sorted, or that they are already in order (with no undo step).
Lists, filters and incompatible selections refuse the command; an unknown
key is named and nothing changes. Rule 2 holds: nothing stays sorted;
sorting only happens when the command is given.

`Ctrl+D` and `Ctrl+U` move down and up by half the pane's usable row height
(excluding header and status), at least one row, stopping at either end;
`Ctrl+F` and `Ctrl+B`, or `PgDn` and `PgUp`, move by the whole of it.
They work in outlines, lists and filters and keep the cursor visible. While
typing they retain their existing behavior (`Ctrl+U` cuts before the caret);
in help they retain half-page scrolling.

### Archive

`:archive` moves the closed work under the pane's root to the end of its
project, as the last children of a top-level heading there named
`Archive:`, made when there is none. A done or canceled row goes with
everything under it, and stays where it is while a row under it is still
open, doing or waiting. Rows already under the heading are left alone. In
Projects every project is swept into its own heading. The whole act is one
undo step and says how many rows moved; the cursor stays on its row. Lists,
filters and the Inbox refuse it. Nothing is archived unless the command is
given, so rule 2 holds, and the heading is an ordinary row: it folds, moves
and is deleted like any other.

### Saved filters and review

`:save name` keeps the pane's filter, with the row it was made under and
whether it was flat. It then appears in the `f` picker as `/name`.
`:forget name` drops it. Saved filters are not in the sidebar. Each is a row
under a hidden top-level row with the fixed identity `filters`, so saving
and forgetting are undone, backed up and synced like any other change.

`:review` is a list of the active projects, the longest untouched first,
each with its age and how many tasks are left to do, or `∅`. It is a list
like the others, so the ordinary keys do the review: moving on keeps a
project, `Enter` looks inside, and a stamp parks (`$p`) or closes (`$done`)
it.

### Selection

`v` selects the cursor's row and steps down; again on a selected row it
deselects. While rows are selected, `x`, `!`, `t`, `dd`, `m`, `>`, `<`, `J`,
`K` and a stamp act on the selection, in one undo step. `x`, `!` and `t` set
their value on every row unless all already hold it, in which case they
clear it. The keys that work on one row (`A`, `I`, `cc`, `o`, `O`, `Enter`)
are refused and say so. The status line counts the selection, and `Esc` clears it.

`V` starts a sweep at the cursor's row. Each motion, with or without a
count, selects the shown span from that anchor to the cursor, both included;
moving back gives rows up again. Picks made before the sweep stay selected.
`V` ends the sweep and keeps the rows; `v` ends it, then toggles the cursor's
row and steps down as usual. Any other non-motion key ends the sweep before
acting: `VGdd` deletes through the end in one undo step, and `V2jx` closes
three rows. `Esc` ends the sweep and clears the selection (a pending count
is still canceled first). Sweeps work in lists and filters, passing over
dimmed ancestors in a contextual filter. Changing the filter, jumping or
opening a place clears both the selection and the sweep.

`m` then `Enter` on a blank query moves the selection `here`: where `o`
would open a row at the cursor, as the first children of its row when that
row has children and is unfolded, else just after it. The rows keep their
screen order and their identity, a selected row under a selected row travels
with it, and it is one undo step; the cursor and the selection stay. It is
offered only in an outline (not a list or a filter) and only when the
cursor's row is neither selected nor under a selected row. Selection does not
survive leaving a place: to reach an exact spot in a project not on screen,
`m` to the project, then `here`.

`:all` replaces the selection with every row the pane shows. Under a filter
in context it takes the matches only, never the dimmed ancestors; rows inside
a fold are not shown and so not selected. An empty pane says there is
nothing to select. It is not a second route to `v` (rule 3): `v` selects one
row, `:all` the pane. Filter, `:all`, then one stamp or key acts on every
match: `/#old`, `:all`, `-#old #new` retags.

For `>`, `<`, `J` and `K`, a selected row with a selected ancestor travels
with that ancestor. The remaining rows must be adjacent children of one
parent; otherwise the move is refused and nothing changes. `>` appends the
block under the sibling above it and unfolds that sibling; `<` places it
just after its parent. `J` moves it past the sibling below; `K` moves it
before the sibling above. Order within the block, the selection and the
cursor's row stay the same, so the key can be pressed again. Lists and
filters still refuse these keys.

`Esc` undoes the most recent narrowing: the selection, then the filter, then
the zoom.

**Repeat.** `.` applies the last stamp or named move to the selection, else
to the cursor's row, as one undo step. The stamp line is remembered as typed
and read again against today (`@+3d` counts from the day it is repeated).
A move remembers the destination row and its label: a project, Inbox or
heading, and appends the rows as its last children, saying `moved to <label>`.
An unreadable stamp, a move to `here`, every other key and `u` leave the
remembered act unchanged. It lasts only for this session. A destination
that is no longer live is named on the status line and nothing changes;
moving a row under itself is refused as with `m`. With nothing remembered,
`.` says `nothing to repeat`. It works wherever the remembered act works,
including lists and filters; with sidebar focus it does nothing.

## Keys

**Grammar.** Motions are `j` `k` `gg` `G` `Ctrl+D` `Ctrl+U` `Ctrl+F`
`Ctrl+B`, the structural motions `}` `{` `)` `(` `H` `]` `[` and the find
motions `n` `N`; `h` and `l` are fold keys, not motions. In the outline, digits
`1` to `9` start a count and `0` continues it; `0` alone does nothing.
The status line shows the pending count, then any pending operator. `Esc`
drops it without clearing a selection, filter or zoom. Counts do not apply
in the sidebar, help, prompts or while typing.

A count repeats `j`, `k`, the paging motions and their arrow and page
aliases, and `}`, `{`, `H`, `]`, `[`, `n` and `N`, stopping at either end.
`gg`, `G`, `Home`, `End`, `)` and `(` drop it.
It also repeats `J`, `K`, `u` and `Ctrl+R`: `3J` is one undo step, while
`3u` undoes three steps. `J` and `K` stop when the row can go no further
and keep the moves already made, including in Today.

**Structural motions** read the shown rows by depth and change no fold.
`}` and `{` go to the next and previous sibling, passing over the rows under
each; `)` and `(` go to the last and first sibling. `H` goes to the parent,
and a count climbs that many levels, stopping at the pane's top level. `]`
and `[` go to the next and previous shown row ending in a colon, at any
depth. A motion with nowhere to go stays put. In a list or a flat filter
every row is at one level, so `}` and `{` step as `j` and `k`, `)` and `(`
reach the ends and `H` stays. Like every motion they take a count, sweep
with `V` and complete `d` and `y`: `d}` deletes this branch and the next,
`y)` copies through the last sibling, `VH` selects up to the parent.

**Find motions.** `n` and `N` go to the next and previous shown row matching
a filter, stopping at the last and saying so; they do not wrap. While a
filter is on screen that is its matches, so in context they pass over the
dimmed ancestors. Otherwise it is the filter last kept with `Enter` or
dropped from the rows with `Esc`, read again against today and the whole
grammar of `/`, ancestors included: `/milk` `Enter` `Esc` leaves the cursor
on the first match in the full outline, and `n` walks the rest. A filter
dropped while still being typed is not remembered. The query lasts for the
session and is one for every place; with none, `n` says there is nothing to
find. A row inside a fold is not shown and so not found. `/` narrows the
pane and `n` moves within it, so neither is a second route to the other
(rule 3).

`d` and `y` wait for a doubled key or a motion. `dd` and `yy` act on the
selection, else the cursor's row; a count takes shown rows down from there.
With a motion they take every shown row from the cursor to its destination,
both included, leaving dimmed ancestors out. Counts before the operator
and before the motion multiply: `d3j` and `3dj` both delete four rows.
An operator with a motion is refused while rows are selected: use `dd` or
`yy` instead. `Esc` or any other key cancels the pending operator and does
nothing. `y` alone waits; a completed copy keeps the cursor where it was.
Every copy and delete writes whole subtrees to the session's text register
and system clipboard; delete writes before deleting and is one undo step.
`u` restores the deleted rows without changing the register.

Before `x`, `!`, `t`, `dd`, `yy`, `m`, `>`, `<`, `.`, or a stamp character
(`#` `@` `$` `*` `-`), a count takes that many shown rows from the cursor
down, or as many as remain, without changing the selection. In a filter
in context it passes over dimmed ancestors, taking matches only, as `:all`
does. The act is one undo step; copy changes no rows. Counted `>` and `<`
follow the Selection rule for a block. `5v` selects five rows as five
presses of `v` would, stopping at the last row. With rows already selected,
a count before an act on rows (including `v`) is refused and says so.
Every other key, including `o`, `O`, `A`, `I`, `cc`, `Enter`, `h`, `l`, `f`,
`p`, `P`, `a`, `/`, `:` and `Tab`, drops the count and acts as usual.

| Keys | Act | Milestone |
| --- | --- | --- |
| `j` `k` `gg` `G` | Move; `Home` `End` as `gg` `G` | 1, 10 |
| `Ctrl+D` `Ctrl+U` | Move half a pane down, up; `Ctrl+F` `Ctrl+B` `PgDn` `PgUp` a whole pane | 7, 10 |
| `}` `{` `)` `(` | Next, previous sibling; last, first sibling | 12 |
| `H` `]` `[` | Parent; next, previous heading | 12 |
| `n` `N` | Next, previous match of the last filter | 12 |
| `h` `l` | Fold, unfold; step to parent, first child; in lists and flat filters, `h` shows the row where it lives (`Esc` returns), `l` does nothing | 1, 10 |
| `Enter` `Esc` | Zoom in, back out | 1 |
| `o` `O` `A` `I` `cc` | New row below, above; type at end, at start; retype keeping fields | 1, 11 |
| `>` `<` `J` `K` | Indent, outdent, reorder | 1 |
| `d` `y` | Delete or copy: `dd` / `yy` take rows; a motion takes an inclusive range | 1, 7, 11 |
| `p` `P` | Put the register after or before the row, as new rows | 11 |
| `u` `Ctrl+R` | Undo, redo | 1 |
| `.` | Repeat the last stamp or move | 10 |
| `x` `!` | Done and back, flag | 2 |
| `t` | Pick for Today, and unpick | 3 |
| `Tab` `f` | Sidebar focus; go to a project, heading or list by name | 3 |
| `a` `m` | Capture to the Inbox from anywhere; move to a project or heading | 3 |
| `v` `V` | Pick a row; sweep with motions | 4, 11 |
| `/` `:` | Filter, command | 4 |
| `#` `@` `$` `*` `-` | Stamp sigils on rows | 4 |

`A` opens the cursor's row for typing at the end of its canonical line; `I`
opens it at the start. `cc` removes its text and keeps its fields as sigils,
with a space before them and the caret at the start. Its state is explicit
so replacement text cannot change it by gaining or losing a trailing colon.
Keeping the line saves the new text in one undo step; leaving it blank keeps
the existing row with no text. All three drop counts, refuse a selection,
and work in lists and filters. `c` waits only for another `c`; `Esc` or any
other key drops it and does nothing. `e` does nothing in the outline.

While typing: `Enter` inside a line splits it literally at the caret,
sigils included, and keeps typing at the start of the new row. It goes
where `o` would: first child of an open parent, otherwise next sibling.
At either end, `Enter` keeps the row and opens the next only if it was fresh.
`Backspace` at the start joins into the row drawn above, with a space
between their lines and the caret at the seam; a row with children or no
row above is left alone. A blank fresh row still disappears instead.
Each split or join is one undo step after typing ends. In lists and filters,
`Enter` only keeps the row and `Backspace` at the start does nothing.
`Up` and `Down` move one drawn line within the typed row, keeping the
column. From its first line `Up` keeps the row and goes on typing in the row
above; from its last line `Down` does the same in the row below, so a run of
rows is revised without leaving the text. Each row is its own undo step;
a fresh row left blank disappears on the way, and at either end the line
stays as it is. They
work in lists and filters too. `Home`, `End`, `Ctrl+A` and `Ctrl+E` still
mean the start or end of the whole typed row, not the drawn line.
`Esc` keeps the row; `Tab` and `Shift+Tab` indent and outdent. There is no
cancel: `u` restores the line. `Alt+Left`, `Alt+Right` and `Alt+Backspace`
work by word.

**Copy.** `yy` puts the cursor's row or selection, with whole subtrees, on the
system clipboard and one unnamed register as tab-indented row-grammar lines.
Depth is relative to the copied roots; selected descendants are not written
twice. Deleted rows are omitted; folded and hidden closed descendants are included. The status says
`copied N rows`. Copy works in lists and filters, using the chosen rows
without adding dimmed ancestors. It keeps the selection and cursor and makes
no undo step. `y` with a motion copies the inclusive shown range, excluding
dimmed ancestors, with the same message; `y` alone waits, and `Esc` or an unrelated key cancels it. Counts
before the operator and motion multiply. A motion with selected rows is
refused; use `yy` (or `dd` to delete) on the selection. `dd` and `d` with a
motion write the same subtree text before deleting, as one undo step. Undo
restores the rows and leaves the register intact. The register lasts for the
session only and is never stored.

Copy and export write each line against its own parent's default. Pasting
under the same kind of parent round-trips the text, state, row-grammar fields,
sibling order and hierarchy. An otherwise empty row is written with its state
sigil (`$open` for an open task, `$note` for a note), at its indentation, and
kept through editor insertion until parsing; it counts as a row and keeps
its children. Whitespace-only external lines may still be ignored. There is
no row-count limit; the entire paste is one undo step, including more than
2000 rows.

Two escapes preserve literal text. First, the sigil formatter adds one
backslash to any text token with zero or more backslashes before a recognized
sigil, anywhere in a line. The sigil parser removes exactly one and keeps the
rest literal; unescaped sigils remain metadata, and backslashes before
non-sigils stay unchanged. Second, copy/export prefix one backslash to a
formatted line beginning with `- `, `* `, `+ `, `• `, `[ ] `, `[x] `, `[X] ` or
a backslash. After reading indentation, paste removes this protective
backslash only before one of those prefixes or another backslash, then skips
bullet/checkbox normalization, including in a single-line row editor.
Transport encoding follows sigil formatting; transport decoding precedes
sigil parsing. Thus stored `\#tag` has three leading backslashes on the
clipboard, two after transport decoding, and one after parsing. Prompts keep
typed text. Defaults are not preserved when pasting under a different kind
of parent.

**Paste.** One pasted line is typed text, with spaces preserved, except an
unescaped checkbox line (optionally after a list bullet): its normalized text
is inserted at the caret with `$open` for `[ ]` or `$done` for `[x]`, and editing
continues. Several lines become rows: the first joins the row being typed
and the rest follow it. When nothing is being typed, all pasted lines become
rows after the cursor. Indentation of any unit is depth; `-`, `*`,
`+` and `•` bullets are dropped; `[ ]` adds `$open` and `[x]` adds `$done`,
so checkboxes stay tasks wherever pasted. The appended state wins over any
state sigil in the pasted text. The whole paste, with the row it was typed
into, is one undo step. Rows cannot be pasted into a list, a
filter, or over a selection. Pasted into a prompt, lines stay typed text and
join into one. A new row left blank disappears. Newly pasted or put closed
rows stay visible until the place is left, so the cursor can land on the
last row even with `:closed` on.

**Put.** `p` puts the session register after the cursor's row and `P` before
it, as new rows at the cursor's level. Depth, states and fields are read as
with terminal paste; the whole put is one undo step. The cursor lands on the
last row put and the status says `put N rows`. Each put makes fresh rows and
leaves the register unchanged: `yyp` duplicates a row with its subtree,
`ddp` swaps a row with the one below, and `p` twice puts twice. An empty
register says `nothing to put`; in an empty place either key adds children
of that place. Lists, filters and selections refuse put; with sidebar focus
both keys do nothing. A count before either key is dropped. Put does not
keep a row's identity; `m`, `J` and `K` do.

`o` on an open parent adds its first child; otherwise it adds the next
sibling.

## Sigils

A sigil is a whole word, so `R&D`, `a@b` and `wow!` stay prose. One the
grammar cannot read also stays prose (`@alice`, `#123`, `$5`). `\#tag` keeps a readable sigil literal.

| Sigil | Field | Accepts |
| --- | --- | --- |
| `!` | `flag` | The word `!` alone |
| `#tag` | `tags` | A word starting with a letter; stored lowercase, once |
| `@date` | `due` | `today`, `tomorrow`, a weekday (the next one, never today), `+3d`, `+2w`, `MM-DD` (the next such day), `YYYY-MM-DD` |
| `@@date` | `start` | The same forms as `@date` |
| `*interval` | `repeat` | A count and `d`, `w`, `m` or `y` (`*3d`, `*2w`), or `daily`, `weekly`, `monthly`, `yearly` |
| `$state` | `state` | `open`, `doing`, `waiting`, `done`, `canceled`, `parked`, `note`, or an unambiguous prefix (`$w`, `$c`, `$p`; `$d` is not one). `$next` is read by the filter only: it is not a state |
| `%name` | | On the capture line only: the project or heading to file the row in |

Typed in a row, sigils become fields when the edit ends and may sit anywhere
in the line; a repeated `@` or `$` takes the last. `A` opens a row showing its
canonical line: text, `!`, `@YYYY-MM-DD`, tags, state. Deleting a sigil from
that line clears its field. Typed on a row outside an edit, they open the
stamp line.

**While typing.** On every line that reads sigils (a row, the capture line,
the stamp line, the filter) a word the line reads as a sigil is drawn in the
accent colour, and a word that starts like one and cannot be read in the
warning colour, so a mistyped `@firday` is seen before `Enter`. At the end of
the line a sigil being typed is completed in dim text: a `#tag` already in
use, most used first; a `$state`; a date word. The right arrow takes the
suggestion and types nothing otherwise. The capture line also names where
the row will land.

**Default state.** A row whose line names no state is a note when it is
top-level, its text ends in a colon, or its parent has any state (is a task).
Otherwise it is an open task: children of a project, the Inbox, a heading or
another note default to tasks. With `A` or `I`, the line spells `$state` only
where the row differs from that default, so a subtask needs `$open`; `cc`
always spells it to preserve the state while replacing text. Stored fields remain
the truth; existing rows keep their states without migration. A row keeps
its state when it moves, including while editing. A fresh row takes the
default of where it ends up when the edit is kept.

**On the row.** One glyph column says what the row is: `○` open, `●` doing,
`◔` waiting, `✓` done, `✕` canceled, `◌` parked; a note shows `▾` or `▸` when it has
children and nothing otherwise. A folded task keeps its state glyph and
counts its children after its text (`+3`). Fields follow the text, each in
the sigil that writes it; the deadline reads `@today`, `@tomorrow`, `@fri`,
`@Nov 20`. An overdue deadline is red and today's is yellow until the row is
done or canceled, which dims the whole row. A word starting `http://` or
`https://` is written as a terminal hyperlink, without the punctuation a
sentence leaves after it; every piece of an address broken across lines
carries the whole of it, so it opens wherever it is clicked.

**Parked** is "not now" for any row: a project leaves the active list, and a
parked row and everything under it is in no computed list however long it
sits. It is the one place for work that is kept and not being done. Today
still shows a parked row that was picked, since a pick is explicit.

`x` toggles done and open; `!` toggles the flag. Every other state is typed
or stamped.
Any change of state stamps `changed`.

**Start.** A row whose start day is ahead is dimmed like a parked one and is
never stale. On the day it reads as live again and is in Today. The filter
reads `@@` (has a start day) and `@@later` (one still ahead).

**Repeat.** A repeating row is never done. Closing it, by `x`, a stamp or its
line, moves it on and leaves it open: its deadline goes to the next one after
today, counted from the deadline so the weekday or day of month holds, and
its start day moves with it; with no deadline, its start day goes one
interval on from today. A month step keeps the day, landing on the last day
of a shorter month. The status line says when it is next. The row is changed
in place, never copied, so two machines closing it cannot make two.

## Outside the app

`docket add LINE` captures without opening the outline. The line is read as
the capture line reads it: sigils become fields and a `%name` word files it
in a project or heading, else it goes to the Inbox. With no words, each line
of standard input is one row. It prints where each row went, and exits
non-zero with nothing added when a line holds nothing to add. It is one
short-lived window on the database, under the rules of Two windows: an open
window sees the row at its next key, and a sync pass runs before it ends.
This is the only command; everything else is done in the outline.

## Milestones

1. **Outline editor.** Rows on SQLite, in-place editing, indent, reorder,
   fold, zoom, undo. *Built.*
2. **Tasks.** Sigil parsing, states, flag, deadline, row rendering. *Built.*
3. **Sidebar.** The four lists, projects, parking, `t`, `a`, `m` and `f`.
   *Built.*
4. **Filter line.** `/`, stamping, selection, `:` commands. *Built.*

5. **After the trial.** Typing polish, backups, wrapping, paste, persistent
   undo, start dates, repeating rows, saved filters, review, sync. *Built.*

6. **Before daily use.** A copied database takes its own sync name, a stuck
   sync file is named, a session left open keeps taking backups, sync works
   in the folder away from the keys and reads only what is new, and rule 5
   is held by a test. *Built.*

7. **Outliner.** Indent, outdent and reorder a selected block of adjacent
   siblings in one undo step. Rows under tasks default to notes; pasted
   checkboxes explicitly keep their task state. `yy` copies rows and their
   subtrees for lossless paste under the same kind of parent. `m` and capture
   `%name` file rows under headings as well as projects. While typing,
   `Enter` splits a line and `Backspace` at its start joins it into the row
   above, each in one undo step, in outlines only. `:fold [level]` and `:unfold`
   control the pane's folds; `Ctrl+D` and `Ctrl+U` move half a pane. Today
   keeps pick order; `J` and `K` reorder it without moving outline rows. *Built.*

8. **Fluid.** `Up` and `Down` go on typing in the next row. A filter shows
   matches inside folded rows, reads alternatives (`$open|doing`) and when
   a state changed (`~7d`). `f` reaches headings. `:archive` shelves closed
   work under `Archive:`. Web addresses are links. `docket add` captures
   from a shell. *Built.*

9. **Queries.** `$next` filters each project's next action; `@<date` and
   `@>date` filter deadline ranges, and a flat deadline filter reads soonest
   first. `:all` selects every row the pane shows. `m` `Enter` moves the selection
   `here`, where `o` would open a row. *In progress.*

10. **Repeat and reach.** `h` in a list or flat filter shows the cursor's
   row where it lives; `Esc` returns to the held list or filter on that row.
   Blank `f` `Enter` returns to the place the last jump left, with its zoom
   and cursor; a sidebar visit counts as one jump. Filters read bare `#`
   and `$` as any tag and any state, and alternatives may mix sigils
   (`#home|!`, `$waiting|@overdue`, `milk|#food`). `^term` tests ancestors,
   including above the pane root; `-^term` excludes such ancestry.
   `:sort [keys]` stably orders siblings or a selected block by deadline,
   flag, state, or text, within its span and in one undo step.
   `.` repeats the last stamp or named move; `:help` opens help.
   *Built.*

11. **Grammar.** Counts repeat motions, reorder, undo and redo, or take
   shown rows for an act; each counted act is one undo step. `V` sweeps a
   selection with motions, preserving earlier picks. `d` and `y` take doubled
   keys or counted motions and fill the session register and clipboard;
   each delete is one undo step. `p` and `P` put the register after or before
   the cursor as new rows in one undo step; Go to moved from `p` to `f`.
   `A` and `I` open typing at the end and start; `cc` retypes text while
   keeping fields, as one undo step. The typed row wraps, preserving spaces
   and sigil colours; arrows move by drawn line and scrolling follows the caret.
   *Built.*

12. **Motions.** `}` `{` `)` `(` move among siblings, `H` to the parent and
   `]` `[` between headings, by the shown rows and without folding. `n` and
   `N` find the next and previous match of the filter on screen or the one
   last kept or dropped. All take counts, sweep with `V` and complete `d`
   and `y`. *Built.*

Not planned: reminders, attachments, a details panel, import from v1, any
command-line interface beyond `docket add`.

## Tests

Behaviour is tested by pressing keys against the model and comparing the
drawn rows, in `internal/tui/model_test.go`. A new behaviour arrives with a
scenario; a test that reaches into the model's fields is the exception.
