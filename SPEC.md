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
3. **One way to do each thing.** No menu or palette repeats a key.
4. **Thirty keys at most** in the outline. A new key evicts an old one. The
   five characters that open the stamp line (`#` `@` `$` `*` `-`) are the
   row grammar typed on a row, and are not counted.
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
once; `Enter`, `Tab` or `Esc` returns to the pane.

### Pane

The pane shows the rows under one row, or a list.

- **A project, the Inbox, or Projects** is the outline, edited where it sits.
  The Inbox is an ordinary row with the fixed identity `inbox`, kept first
  among the top-level rows and never drawn among them. The `filters` row is
  the other such row.
- **Today, Waiting and Stale** are flat. A list is read when it is opened and
  holds those rows until it is opened again, so closing or rescheduling a row
  never pulls it from under the cursor. Each row names its project; Waiting
  and Stale lead with the longest wait and say how many days. A list has no
  order to edit: `o`, `O`, `>`, `<`, `J` and `K` are refused there, and
  `Enter` works inside a row, from which `Esc` returns to the same list.
- **Launch** opens Today when something is picked for the day, else Projects.
- **A row wraps** under its own text at the pane's width, each word keeping
  its colour, so nothing is cut; a word wider than the pane breaks across
  lines. The cursor and selection marks run the row's height, and scrolling
  keeps the cursor's row whole. The row being typed is the exception: it
  stays on one line and scrolls sideways.
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
come back on a date.

### Bottom line

The status line becomes a prompt for three acts, named at its left edge:

- `a` **Inbox ›** files a typed line in the Inbox. A `%name` word files it in
  the project that name matches instead; one matching nothing stays in the
  text. `Enter` keeps the line and asks for the next; a blank `Enter` or
  `Esc` closes, and `Esc` keeps what was typed.
- `p` **Go to ›** opens a project or a list by name.
- `m` **Move to ›** makes the cursor's row the last child of a project or the
  Inbox.

The two pickers list their matches as you type, best first. `Down` and `Up`
walk them, and `Enter` takes the one at the front. A name matches by its start, the start of a word, anywhere
inside, or its letters in order. `Esc` leaves a picker without acting.

Three more acts use the same line:

- `/` **Filter ›** narrows the pane as the query is typed. Words match a
  row's text; `!`, `#tag`, `$state` and `@date` match its fields; a leading
  `-` excludes. `@` alone is any deadline, and `@overdue` and `@week` read
  the calendar. Every term must hold. In an outline a match keeps its
  ancestors, dimmed, so it is read where it lives; `Tab` switches to a flat
  list of matches and back. `Enter` keeps the filter and returns to the rows,
  where the header names it; `Esc` drops it. Like a list, a filter holds the
  rows it found until the query changes, and refuses the keys that
  restructure the tree. Changing the filter clears the selection.
- `#` `@` `$` `*` `-` **Stamp ›** opens holding that character. The line is
  sigils only: `#tag`, `@date`, `@@date`, `*interval`, `$state` and `!` set;
  `-#tag`, `-@`, `-@@`, `-*` and `-!` remove. `Enter` applies them to the selection, else to the cursor's row, as
  one undo step. A word it cannot read is named and nothing is applied.
- `:` runs a command: `q` quits; `export [file]` writes the whole outline as
  tab-indented lines in the row grammar, to `~/docket-YYYY-MM-DD.txt` unless
  a file is named; `closed` hides or shows closed rows; `review` opens the
  review list; `save name` and `forget name` keep and drop a filter; `sync
  folder` and `sync off` are described under Sync.

### Saved filters and review

`:save name` keeps the pane's filter, with the row it was made under and
whether it was flat. It then appears in the `p` picker as `/name`.
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
deselects. While rows are selected, `x`, `!`, `t`, `dd`, `m` and a stamp act
on all of them, in one undo step. `x`, `!` and `t` set their value on every
row unless all already hold it, in which case they clear it. The keys that
work on one row (`e`, `o`, `O`, `>`, `<`, `J`, `K`, `Enter`) are refused and
say so. The status line counts the selection, and `Esc` clears it.

`Esc` undoes the most recent narrowing: the selection, then the filter, then
the zoom.

## Keys

| Keys | Act | Milestone |
| --- | --- | --- |
| `j` `k` `gg` `G` | Move | 1 |
| `h` `l` | Fold, unfold; step to parent, first child | 1 |
| `Enter` `Esc` | Zoom in, back out | 1 |
| `o` `O` `e` | New row below, above; edit | 1 |
| `>` `<` `J` `K` | Indent, outdent, reorder | 1 |
| `dd` | Delete | 1 |
| `u` `Ctrl+R` | Undo, redo | 1 |
| `?` | Help | 1 |
| `x` `!` | Done and back, flag | 2 |
| `t` | Pick for Today, and unpick | 3 |
| `Tab` `p` | Sidebar focus; go to a project or list by name | 3 |
| `a` `m` | Capture to the Inbox from anywhere; move to a project | 3 |
| `v` | Select | 4 |
| `/` `:` | Filter, command | 4 |
| `#` `@` `$` `*` `-` | Stamp sigils on rows (not counted) | 4 |

While typing: `Enter` keeps the row and opens the next, `Esc` keeps the row,
`Tab` and `Shift+Tab` indent and outdent. There is no cancel: `u` restores
the line. `Alt+Left`, `Alt+Right` and `Alt+Backspace` work by word.

**Paste.** One pasted line is typed text. Several become rows: the first
joins the row being typed and the rest follow it, or all follow the cursor's
row when nothing is being typed. Indentation of any unit is depth; `-`, `*`
and `+` bullets are dropped; `[x]` closes its row. The whole paste, with the
row it was typed into, is one undo step. Rows cannot be pasted into a list, a
filter, or over a selection. Pasted into a prompt, lines join into one. A new row left
blank disappears.

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
| `$state` | `state` | `open`, `doing`, `waiting`, `done`, `canceled`, `parked`, `note`, or an unambiguous prefix (`$w`, `$c`, `$p`; `$d` is not one) |
| `%name` | | On the capture line only: the project to file the row in |

Typed in a row, sigils become fields when the edit ends and may sit anywhere
in the line; a repeated `@` or `$` takes the last. Editing a row shows its
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

**Default state.** A row whose line names no state is an open task, unless it
is top-level (a project) or its text ends in a colon (a heading); those are
notes. The line spells `$state` only where the row differs from that default,
so most lines carry none. A row keeps its state when it moves.

**On the row.** One glyph column says what the row is: `○` open, `●` doing,
`◔` waiting, `✓` done, `✕` canceled, `◌` parked; a note shows `▾` or `▸` when it has
children and nothing otherwise. A folded task keeps its state glyph and
counts its children after its text (`+3`). Fields follow the text, each in
the sigil that writes it; the deadline reads `@today`, `@tomorrow`, `@fri`,
`@Nov 20`. An overdue deadline is red and today's is yellow until the row is
done or canceled, which dims the whole row.

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

## Milestones

1. **Outline editor.** Rows on SQLite, in-place editing, indent, reorder,
   fold, zoom, undo. *Built.*
2. **Tasks.** Sigil parsing, states, flag, deadline, row rendering. *Built.*
3. **Sidebar.** The four lists, projects, parking, `t`, `a`, `m` and `p`.
   *Built.*
4. **Filter line.** `/`, stamping, selection, `:` commands. *Built.*

5. **After the trial.** Typing polish, backups, wrapping, paste, persistent
   undo, start dates, repeating rows, saved filters, review, sync. *Built.*

6. **Before daily use.** A copied database takes its own sync name, a stuck
   sync file is named, a session left open keeps taking backups, sync works
   in the folder away from the keys and reads only what is new, and rule 5
   is held by a test. *Built.*

Not planned: reminders, attachments, a details panel, import from v1, any
command-line interface.

## Tests

Behaviour is tested by pressing keys against the model and comparing the
drawn rows, in `internal/tui/model_test.go`. A new behaviour arrives with a
scenario; a test that reaches into the model's fields is the exception.
