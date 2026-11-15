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
   four characters that open the stamp line (`#` `@` `$` `-`) are the row
   grammar typed on a row, and are not counted.
5. **Every keystroke paints within one frame.** The tree lives in memory;
   SQLite is written when a change is committed, never while typing.
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

Reserved, not built: `start` (a not-before date) and `repeat` (regenerate in
place on completion, never as spawned instances).

## Storage

SQLite, one file, local to the machine. It must never sit in a folder a file
sync tool copies.

- `ops` is the append-only log: every operation with a stamp.
- `rows` and `fields` are the current state, written in the same transaction
  as the log entry. `fields` keeps one stamp per field and `rows` one for the
  last move, which is what a later merge needs to resolve conflicts per field.
- A stamp is `milliseconds-counter-device`, issued by a clock that never runs
  backwards, and sorts bytewise in issue order.
- `folds` is view state for this machine. It is not logged.
- Adding a field needs no schema change.

Sync is not built. When it is, machines exchange `ops` and never the database
file, and conflicts resolve per field by stamp.

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
  and doing rows untouched for 14 days.
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
  among the top-level rows and never drawn among them.
- **Today, Waiting and Stale** are flat. A list is read when it is opened and
  holds those rows until it is opened again, so closing or rescheduling a row
  never pulls it from under the cursor. Each row names its project; Waiting
  and Stale lead with the longest wait and say how many days. A list has no
  order to edit: `o`, `O`, `>`, `<`, `J` and `K` are refused there, and
  `Enter` works inside a row, from which `Esc` returns to the same list.
- **Launch** opens Today when something is picked for the day, else Projects.

Today is hand-picked and rebuilt daily: a row is in Today when its `today`
field is the current day, so yesterday's picks drop out without a write.

### Bottom line

The status line becomes a prompt for three acts, named at its left edge:

- `a` **Inbox ›** files a typed line in the Inbox. A `%name` word files it in
  the project that name matches instead; one matching nothing stays in the
  text. `Enter` keeps the line and asks for the next; a blank `Enter` or
  `Esc` closes, and `Esc` keeps what was typed.
- `p` **Go to ›** opens a project or a list by name.
- `m` **Move to ›** makes the cursor's row the last child of a project or the
  Inbox.

The two pickers list their matches as you type, best first, and `Enter`
takes the first. A name matches by its start, the start of a word, anywhere
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
- `#` `@` `$` `-` **Stamp ›** opens holding that character. The line is
  sigils only: `#tag`, `@date`, `$state` and `!` set, `-#tag`, `-@` and `-!`
  remove. `Enter` applies them to the selection, else to the cursor's row, as
  one undo step. A word it cannot read is named and nothing is applied.
- `:` runs a command: `q` quits, and `export [file]` writes the whole
  outline as tab-indented lines in the row grammar, to
  `~/docket-YYYY-MM-DD.txt` unless a file is named.

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
| `#` `@` `$` `-` | Stamp sigils on rows (not counted) | 4 |

While typing: `Enter` keeps the row and opens the next, `Esc` keeps the row,
`Tab` and `Shift+Tab` indent and outdent. There is no cancel: `u` restores
the line. A new row left
blank disappears.

`o` on an open parent adds its first child; otherwise it adds the next
sibling.

## Sigils

A sigil is a whole word, so `R&D`, `a@b` and `wow!` stay prose. One the
grammar cannot read also stays prose (`@alice`, `#123`, `$5`), which is the
only feedback a mistyped sigil gets. `\#tag` keeps a readable sigil literal.

| Sigil | Field | Accepts |
| --- | --- | --- |
| `!` | `flag` | The word `!` alone |
| `#tag` | `tags` | A word starting with a letter; stored lowercase, once |
| `@date` | `due` | `today`, `tomorrow`, a weekday (the next one, never today), `+3d`, `+2w`, `MM-DD` (the next such day), `YYYY-MM-DD` |
| `$state` | `state` | `open`, `doing`, `waiting`, `done`, `canceled`, `parked`, `note`, or an unambiguous prefix (`$w`, `$c`, `$p`; `$d` is not one) |
| `%name` | | On the capture line only: the project to file the row in |

Typed in a row, sigils become fields when the edit ends and may sit anywhere
in the line; a repeated `@` or `$` takes the last. Editing a row shows its
canonical line: text, `!`, `@YYYY-MM-DD`, tags, state. Deleting a sigil from
that line clears its field. Typed on a row outside an edit, they open the
stamp line.

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

## Milestones

1. **Outline editor.** Rows on SQLite, in-place editing, indent, reorder,
   fold, zoom, undo. *Built.*
2. **Tasks.** Sigil parsing, states, flag, deadline, row rendering. *Built.*
3. **Sidebar.** The four lists, projects, parking, `t`, `a`, `m` and `p`.
   *Built.*
4. **Filter line.** `/`, stamping, selection, `:` commands. *Built.*

Then two weeks of daily use before anything else is added. Not planned for
v0: sync, recurrence, start dates, reminders, attachments, saved filters, a
review walk, a details panel, import from v1, any command-line interface.

## Tests

Behaviour is tested by pressing keys against the model and comparing the
drawn rows, in `internal/tui/model_test.go`. A new behaviour arrives with a
scenario; a test that reaches into the model's fields is the exception.
