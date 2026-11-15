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
4. **Thirty keys at most** in the outline. A new key evicts an old one.
5. **Every keystroke paints within one frame.** The tree lives in memory;
   SQLite is written when a change is committed, never while typing.
6. **Chrome is one header line and one status line.**

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
| `state` | 2 | `open`, `doing`, `waiting`, `done`, `canceled`; unset is a note |
| `flag` | 2 | The single priority mark `!` |
| `due` | 2 | A hard deadline, a calendar day. Never "when I plan to do it" |
| `tags` | 2 | Space-separated, normalised |
| `changed` | 2 | When `state` last changed, for Waiting and Stale ages |
| `today` | 3 | The day the row was picked for Today |
| `parked` | 3 | Project rows only: hidden from the active list |

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
 Inbox         4 │ docket-rewrite
 Today         3 │ ▾ Storage
 Waiting       2 │   ○ Decide op-log format  !  due Fri
 Stale         9 │     per-field, clock-stamped
─────────────────│   ● Prototype row parser
 ACTIVE          │   ✓ Audit grammar reuse
▸docket-rewrite  │ ▾ TUI
 blog        12d │   ○ Sidebar counts
 tax-2026  ! 3d  │
 homelab     ∅   │
 PARKED (31)     │
```

- **Sidebar** (milestone 3): four fixed lists, then active projects by last
  touch, each with days since touched, `!` for a near deadline and `∅` for no
  open task, then parked projects as a count. The four lists stay four.
- **Outline:** the selected project or list. Rows are edited where they sit.
- **Bottom line** (milestone 4): appears only when invoked, for `/` filter,
  a sigil stamp, or a `:` command, and shows which of the three it is.

Today is hand-picked and rebuilt daily: a row is in Today when its `today`
field is the current day, so yesterday's picks drop out without a write.
Stale is an open row untouched for 14 days.

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
| `x` `t` `!` | Done, Today, flag | 2, 3 |
| `Tab` `p` | Sidebar focus, fuzzy project jump | 3 |
| `a` `m` | Add to Inbox from anywhere; move to a project | 3 |
| `v` | Select | 4 |
| `/` `:` | Filter, command | 4 |

While typing: `Enter` keeps the row and opens the next, `Esc` keeps the row,
`Tab` and `Shift+Tab` indent and outdent. There is no cancel. A new row left
blank disappears.

`o` on an open parent adds its first child; otherwise it adds the next
sibling.

## Sigils

`!` flag, `#tag`, `@date` deadline, `$state`, `%project` to route a captured
row. Typed in a row they become fields when the edit ends; typed alone on the
bottom line they stamp the selected rows, and `-#tag` removes. A sigil needs
whitespace before it, so `R&D` and `a@b` stay prose.

## Milestones

1. **Outline editor.** Rows on SQLite, in-place editing, indent, reorder,
   fold, zoom, undo. *Built.*
2. **Tasks.** Sigil parsing, states, flag, deadline, row rendering.
3. **Sidebar.** The four lists, project ordering, parking, `a` and `m`.
4. **Filter line.** `/`, stamping, selection, `:` commands.

Then two weeks of daily use before anything else is added. Not planned for
v0: sync, recurrence, start dates, reminders, attachments, saved filters, a
review walk, a details panel, import from v1, any command-line interface.

## Tests

Behaviour is tested by pressing keys against the model and comparing the
drawn rows, in `internal/tui/model_test.go`. A new behaviour arrives with a
scenario; a test that reaches into the model's fields is the exception.
