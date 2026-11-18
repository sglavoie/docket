# Docket v2

A keyboard-only terminal outliner for one person with dozens of projects.
Every row is edited in place, metadata is typed as sigils, and a sidebar shows
which projects need attention. This file is the contract: a change that breaks
a rule here changes this file first.

## Rules

1. **Nothing asks a question.** No confirmations and no dialogs. Delete is
   instant; undo is the safety net. A key that cannot act says so on the
   status line and names the way that works, as `this row has no link: L
   makes one`.
2. **Rows never move by themselves.** Changing a row's state leaves it where
   it is. Only the user reorders: by moving a row, or by pinning it with `^`.
   Today and a flat filter draw closed rows after the rest, but they read
   that order only when opened, so a row closed there stays put until then.
3. **One way to do each thing.** No menu or palette repeats a key. Reaching
   the same rows by a count, a selection or a motion is the grammar, not a
   second route.
4. **Keys follow a grammar.** A key in the outline is a motion, an operator,
   or an act on rows; a count goes before any of them. A key that is none
   of these is not added. The six characters that open the stamp line
   (`#` `@` `$` `*` `-` `~`) are the row grammar typed on a row. The word
   motions `w` `b` `e` step into a row's text, where the same grammar reads
   characters instead of rows; `gn` and `gN` step in on what the last
   filter found, and `k` on the first row steps into the body of the row
   the pane opens, where it reads lines as well. The navigation aliases are the arrows,
   `Ctrl+F` `Ctrl+B`, `PgDn` `PgUp` and `Home` `End`. `zt` `zz` `zb` say
   where in the pane the cursor's row is drawn and change nothing else;
   `Ctrl+E` `Ctrl+Y` scroll the pane under the cursor, moving it only to
   keep it on the pane.
5. **Every keystroke paints within one frame.** The tree lives in memory;
   SQLite is written when a change is committed, never while typing. What
   the sidebar states is derived again only when a row has changed or the
   minute has turned, so a typed letter never walks the outline. A test
   holds moving and typing to 16 ms on an outline of 10,000 rows.
6. **Chrome is the sidebar, one header line and one status line.**

## Model

- One kind of row, in a tree. The root is the empty ID; top-level rows are
  projects. A row with a state is a task; a row without one is a note or a
  heading. Any row may carry a body: free text, read and written at the top
  of the pane when the row is opened (see Body).
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
| `tags` | 2 | Space-separated, normalised; a tag may carry a value, as `est=2h` |
| `changed` | 2 | When `state` last changed, for Waiting and Stale ages |
| `today` | 3 | The day the row was picked for Today |
| `todaypos` | 7 | Fractional position in Today; read only while the row is in Today |

| `start` | 5 | Not before this day, a calendar day |
| `repeat` | 5 | An interval, as in `2w`: the row comes back after it is closed |
| `link` | 13 | The row this one points at |
| `color` | 19 | A project's colour: a name, or a hex value as `#rrggbb` |
| `context` | 20 | Where the work is done: one name, normalised as a tag without a value |
| `pin` | 21 | The row is drawn before its unpinned siblings |
| `body` | 23 | Free text under the row's line, lines and blanks kept as typed; never read for sigils |

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

**Not a text file, by choice.** TaskPaper keeps the outline in a plain file
any editor, `grep` or `git` can read. Docket does not: a row needs an
identity that survives an edit for undo across restarts, per-field sync,
links and Today's order, and a text file loses it whenever someone else's
editor saves. The text stays one command away instead: `:export` and
`docket ls` write the row grammar, `:import`, paste and `docket add` read it
back, and the daily backups are whole copies. Editing the outline in an
outside editor and reconciling it back is not planned.

**A save that fails changes nothing.** If a change cannot be written, it is
taken back out of memory and the status line says so; the screen never shows
what the disk does not hold.

**Undo is kept.** The undo and redo stacks, 200 steps deep, are stored on
this machine beside the folds and survive a restart. They are not logged and
will not sync: each step undoes a change made here.

**Prompt lines are kept.** The filter, stamp and command lines each
remember the last 50 lines they were given, on this machine beside the
folds, across restarts. They are not logged and do not sync.

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
  Inbox                1 │ docket-rewrite  · 3 to do
  Today                3 │   ▾ Storage: 1/3
  Waiting              2 │ ▌   ○ Decide op-log format ! @fri
  Stale                9 │       · per-field, clock-stamped
                         │     ● Prototype row parser
▌ Projects               │     ✓ Audit grammar reuse
  docket-rewrite         │   ▾ TUI:
  blog               12d │     ○ Sidebar counts
  tax-2026         @ 3d  │
  homelab              ∅ │
                         │
  31 parked projects     │
Inbox › Buy milk %home
```

After the place’s name, the header states the open, doing and waiting tasks
as `14 to do`, followed by their readable `#est` sum, as `14 to do · 6h30m`.
An outline counts every such row under its root, folded or not; a list counts
its held rows and a filter its held matches, excluding context ancestors.
Zero tasks says nothing. Today is the list that gets finished, so it alone
goes on to count its done rows, as `3 to do · 2 done`, and says `all 5 done`
when nothing is left. None of Waiting is the user's to do, so it counts its
rows still waiting instead, as `270 waiting`. The Inbox is emptied by
filing, so it counts what the sidebar counts beside it, every row in it not
yet done or canceled, notes included, as `24 to file`; one number is never
shown two ways side by side. The review list holds projects, not tasks, so it
counts those, as `Review · 43 projects`. These totals and heading progress are derived in
the existing scan, never stored.

The header reads in three weights. The place is in the text colour, and the
rows above it in a breadcrumb are dim, as is everything said about it. A
project with a colour is named in it, as the place or above it. A
project that is parked, done or canceled has no entry in the sidebar, so the
header says which before its totals, as `tax-2026 · parked · 50 to do`. A held
filter is in the accent colour, since it hides rows. When the rows overflow
the pane, the header's right edge says where the pane is among them, as
`Top`, `40%` or `Bot`; it says nothing while every row is on screen, and
gives way to the place when the line is too narrow for both. A line still
too narrow gives up the rows above the place, from the project down, behind
a `…`, as `… › Sidebar  · 3 to do`: the place is what the pane shows, so it
is the last thing cut. A heading in the header drops its colon, as it does
in the pickers and after a flat row.

On the status line a message is in the text colour and a standing hint
about keys is dim, so a message is seen to be one. A message that something
was not written (a save, a backup, a sync pass) is in the alert colour, so
it does not pass for a confirmation.

The screen uses named text, dim, accent, warning and alert roles in two
built-in true-colour palettes. It asks the terminal for its background at
launch, when the window has focus again, once a minute, and when the
terminal reports that the system turned light or dark (mode 2031, which it
turns on at launch and off on exit), at once and again 300 ms later, so a
terminal that changes its colours (Docket.app does, with macOS's
appearance) is followed, focused or not: light selects the light palette, dark selects dark, and dark is
used until an answer arrives or if none does. The cursor, selection, flash
and header backgrounds are mixed from the background the terminal names,
so they sit on any theme's: the cursor a step toward the text colour, the
selection and a copy's flash toward the accent, the header toward an
indigo. Each is eased back toward the pane where the contrasts below would
fail, as far as the pane itself allows them, and never past a third of its
step. Bubble Tea steps colours
down to the terminal's colour profile and removes them for `NO_COLOR`.
There is no theme command or configuration file.

Beside the five roles, each palette holds a shade of every colour name
`~colour` reads, lighter on dark and deeper on light so each stays
readable. None stands near the alert, the warning or a step of the age
gradient, so a project named in red is not taken for one overdue: `red`
is drawn as a crimson, `orange` an amber and `yellow` a gold. A hex colour
is drawn as it is, whichever palette is in use. Tags
are told apart by a ramp of eight colours, kept clear of the alert and
warning colours. A tag's colour is fixed by its name, without its value,
hashed by FNV-1a, so a tag keeps it on every machine and a tag added later
repaints none of the others; two tags sharing a colour is expected.

The focused cursor row keeps its `▌` mark and has a background across the
pane's full width, on every wrapped line. Selected rows keep `┃` and use
a selection background of their own, a teal, including when the cursor is
on one. It stands further from the cursor's background than that does from
the pane, so a selected row is not taken for the cursor, and a copy's
flash stands as far from it in turn. On that background dim text is drawn
a step nearer the text colour, so it keeps a contrast of 4.5 to 1 there in
both palettes.
An age is drawn on a smooth gradient against the oldest beside it: the
text colour for one just begun, warming through pale yellow, yellow and
orange to red for the oldest, so however young the oldest is, it is red.
The sidebar and review draw a project's age against the oldest active
project; Waiting and Stale draw a row's against the oldest row shown.
The focused pane shades its header in a muted indigo of its own, so focus
is plain even in a pane with no rows and the shade is not taken for the
cursor on the first row; dim text keeps 4.5 to 1 on it in both palettes.
With sidebar focus its entry is shaded instead; the pane’s header and
cursor lose their tint, while selections remain shaded. Indentation stays blank.

### Sidebar

Drawn from 72 columns up; narrower, it takes the pane only while focused.
It is as wide as its widest entry needs, from 24 columns up to 40, so each
count stays beside its name and each mark in its column. Beside the pane
it widens only while the pane keeps the 45 columns it has at 72, so a
wider window gives the room to a long name before the pane.
Everything in it is derived by one pass over the tree. Nothing is stored.

- **Four lists, and they stay four.** Inbox counts its rows not yet done or
  canceled, so it empties as they are filed or finished; Today counts
  picks not yet closed, and shows a dimmed `✓` once every pick is closed,
  so a finished day is told from one with nothing picked. A change the
  user makes that closes the last open pick says so on the status line,
  once, as `today is done: every pick is closed`, unless the key has
  something of its own to say; an undo, a sync pass or the day turning
  over says nothing. Waiting counts
  rows in that state; Stale counts open
  and doing rows untouched for 14 days, leaving out rows whose start day is
  still ahead.
- **Saved filters** follow the four lists, each as `/name`, in the order
  they were saved, set apart from the lists by a blank line, since they are
  the user's places and the lists are every outline's. They carry no count,
  so the scan stays one pass.
- **Projects** shows every top-level row as one outline. It is where a
  project is added, renamed, reordered, parked or closed, with the ordinary
  keys.
- **Active projects** follow, in the order of the top-level rows: the order
  is the user's, so it never shuffles as work happens. A project with a
  colour is named in it. Each says `@`, the
  sigil a deadline is written with, in
  the warning colour for a deadline within a week or the alert colour and
  bold for one overdue, so the two are not told apart by colour alone, then
  `∅` when
  it holds no task to do, else the whole days since anything inside changed:
  dim, and in the text colour from 14 days, the Stale rule, so the project
  left alone is the one the eye finds. The two stand in columns at the
  sidebar's right edge, as review's do: the mark in one of its own whenever
  any project carries one, left blank on the rest, and the age or `∅`
  right-aligned in the other, so a mark is read down one column whether or
  not an age follows it. The columns never cost a name: a name runs into
  its own blank cells, the sidebar widens for one they would still cut,
  and where it cannot, that name has its marks drawn side by side against
  the edge instead.
- **Parked** projects are a count, as `31 parked projects`: it counts
  projects only, never the parked rows inside an active one. Done and
  canceled projects are not listed. While the pane is inside a parked
  project the count carries the dimmed selection mark, since no entry can;
  it is still not a destination. `/$parked $project` in Projects
  lists the projects it counts.
- **A sidebar taller than its pane** scrolls to keep its selection in view,
  and counts the destinations scrolled out at each edge, dimmed, as `↑ 6`
  on its first row and `↓ 13` on its last. A count is never drawn over the
  selection. The sync line does not scroll with them: from 8 rows up it
  stays at the sidebar's foot, since a pass that failed is an alarm and an
  alarm out of sight is none.

`Tab` gives the sidebar focus, and the status line then says, before its
keys, what the marks on the chosen entry stand for, each led by the mark as
it is drawn: `@ a deadline is overdue · 97d since anything changed`,
`∅ nothing left to do`, `✓ every pick is closed`, and for Stale the rule
that fills it. `j` and `k` show each destination at
once, as do `gg` and `G` or `Home` and `End` (first, last), `Ctrl+D` and
`Ctrl+U` (half a page) and `Ctrl+F`, `Ctrl+B`, `PgDn` and `PgUp` (a page),
where a page is the pane's usable row height in entries, stopping at either
end. `Enter`, `Tab` or `Esc` returns to the pane. `:` returns focus to
the pane and opens its command line, so `:tutor` works from the sidebar too.

A project's entry is its top-level row, reached by the sidebar's `j` and
`k`, so `dd` on it deletes the project with everything in it, as `dd` on
that row in Projects does: one undo step, and the selection takes the entry
now in its place. On any other entry `dd` changes nothing and names what
works: `:forget name` for a saved filter, `Enter` and then `dd` for rows.
`u` and `Ctrl+R` undo and redo with the focus kept, and a project that
comes back is chosen again.

The same row is renamed in its entry. `A` and `I` type at the end and the
start of its line, `cc` retypes it keeping its fields, `c` with a motion
or a text object (`cw`, `ce`, `c$`, `ciw`) changes that much from its
start, and `w` `b` `e` `W` `B` `E` step into its text, as each does on
that row in Projects. The line takes the entry's place, wrapped to the
sidebar's width, with the caret in it; the pane stays where it is. It is
one row, so nothing carries the typing out of it: `Tab` does not indent,
`Enter` does not split, `Backspace` at the start does not join, and `j`
`k` `n` inside it stay in it. `Enter` or `Esc` ends the typing, as one undo
step, with the focus kept; a name typed out of the list, by `$parked` or
`$done`, gives the selection to the entry now in its place. On any other
entry the keys change nothing and say what works. The status line offers
`A I cc rename it · dd delete it` on a project's entry. No other operator,
and no count, acts in the sidebar.

### Pane

The pane shows the rows under one row, or a list.

- **A project, the Inbox, or Projects** is the outline, edited where it sits.
  The Inbox is an ordinary row with the fixed identity `inbox`, kept first
  among the top-level rows and never drawn among them. The `filters` row is
  the other such row.
- **Today, Waiting and Stale** are flat. A list is read when it is opened and
  holds those rows until it is opened again, so closing or rescheduling a row
  never pulls it from under the cursor. Each row names its project; Waiting
  and Stale lead with the longest wait and say how many days,
  drawn as the sidebar draws an age against the oldest row shown, so a
  long wait stands out. The age stands in a column at the pane's right
  edge, on the row's first line, right-aligned and as wide as the widest
  age the list holds, as the sidebar's ages stand: a list sorted by age is
  then read down one column, and an age is not taken for the project named
  beside it. A row wraps short of the column, so the column stays clear on
  every line and nothing is cut. A pane that would leave a row's text
  fewer than 16 columns beside it says the age after the text instead,
  led by its dot, and the row being typed gives up its column while it is.
  A list read by its ages names a project dim, whatever its colour, so no
  hue beside a row competes with the heat of the column; Today, review and
  a flat filter name it in its colour.
  Each thing
  said after a row is led by a dot that stays with it when the row wraps.
  Nothing folds in a flat view, so a note there shows `·` whatever it holds
  and counts the rows under it (`+3`) as a task does, unless it is a heading
  that says its progress. A row unpicked in Today stays, as a held row
  does, and is dimmed, so the pick is seen to be gone. Today alone
  has an editable order: `J` and `K` move the cursor's row down and up,
  keeping the cursor on it without moving it in the outline. They refuse
  selections; Waiting, Stale and review refuse both keys. `o`, `O`, `>` and
  `<` are refused in every list. `Enter` works inside a row, from which
  `Esc` returns to the same list. `h` opens the cursor's row among its
  siblings in its parent's outline; `Esc` returns to the held list on that
  row. In review, a top-level row opens Projects. The row stays visible
  even with `:closed` on. `h` refuses a selection, and `l` does nothing;
  neither changes a fold.
- **Launch** on a database with no project and no Inbox row adds and opens
  a `Welcome` tutorial in one undo step, with three saved filters when none
  is saved yet: `/next` (`$next`), `/week` (`@week`) and `/done`
  (`$done ~7d`), each flat over Projects.
  Otherwise it opens Today when
  something is picked for the day, else Projects. `docket add` and `docket ls`
  never add the tutorial.
- **A row wraps** under its own text at the pane's width, each word keeping
  its colour, so nothing is cut; a word wider than the pane breaks across
  lines. The cursor and selection marks run the row's height, and scrolling
  keeps the cursor's row whole when it fits. A row wraps as it is typed;
  when it is taller than the pane, scrolling keeps the caret's line visible.
- **A row's body heads the pane that opens it.** When the pane shows the
  rows under one row, a project, the Inbox or a zoom, with no filter on,
  that row's body is drawn first, folded to its first line unless it was
  opened, with a dimmed rule between it and the rows when there are any.
  Stepped into, it takes the whole pane. Projects and the lists have no row
  of their own, so no body. See Body.
- **Each place remembers its cursor** for the session, so coming back to a
  project lands on the row you left.
- **A row reached from afar is read in context.** When `n` or `N`, a jump,
  the trail, a filter dropped with `Esc` or coming back to a place puts the
  cursor on a row the pane was not showing, that row is drawn in the middle
  of the pane, with what is above and below it. A row already on screen is
  not scrolled, and the other motions scroll no further than they must.
- **Closed rows can be left out.** `:closed` hides done and canceled rows,
  and what is under them, from outlines; the header says so and the setting
  is kept on this machine. A row closed while on screen stays until the
  place is left. Lists and filters name what they show, so they still show
  closed rows.
- **An empty pane says why, and names the key that changes it**, dimmed
  on its first line: `o` in an outline, `a` in the Inbox, `t` for Today,
  `$waiting` for Waiting, `:closed` when only closed rows are hidden, and
  `Esc` under a held filter that matches nothing.
- **The screen follows the clock.** It is read again once a minute, so a day
  that turns over while the app is open updates Today, deadlines and ages
  without a key press.
- **Rows can say how far they are.** `:numbers` shows, before each row, how
  many shown rows it is from the cursor, so a count is read off the screen:
  the row marked 3 is `3j` away and `d3j` takes through it. The cursor's row
  shows its own place instead, counted from 1, as Vim's `number` and
  `relativenumber` together do, so `12G` goes back to where 12 stood. The
  column widens to the digits of the last row's place. It works in outlines, lists and filters, and the setting is
  kept on this machine.

Today is hand-picked, and a pick stays until it is dealt with: a row is in
Today when its `today` field is the current day, or an earlier day while the
row is still open, doing or waiting. So what a day left undone is still in
Today the next morning, saying when it was picked as `· since Nov 15`, and
nothing goes out of sight by itself (rule 2); the sidebar counts it, so a
day with leftovers is never shown as finished. A pick from an earlier day
leaves when it is closed or when `t` unpicks it; one closed on an earlier
day has left already. `t` reads a carried pick as picked, so on a mixed
selection it keeps that pick's day and place and adds the others after it.
Nothing is written when the day turns. A
task whose start day is today is in Today too, which is how a row is made to
come back on a date. Today names that day already, so a start day of today
is not drawn there, as the pick's `★` is not. Picks are kept in the order made: `t` assigns a fresh
`todaypos` after the last placed row, overwriting any earlier day's place
in the same undo step. Unpicking leaves that place stored. Rows with a place
come first, sorted by it; rows without one follow in outline order. Before
`J` or `K` moves a row, any missing places are filled in the order shown,
in the same undo step. Ordering is kept across reopening and restarting.
Done and canceled rows come after the open ones, each part keeping that
order. Which rows are closed is read when Today is opened, as which are
carried is, so a row closed or reopened there stays put until Today is
opened again. `J` and `K` keep a row with its part and say so, as at the
rule.

Today reads in two parts. The day's own rows, picked today or starting
today, come first, as if pinned; the picks carried from an earlier day
follow, below a dim rule across the pane. The rule is a line of the pane,
not a row: the cursor, counts and motions pass over it, and it is drawn
only when the day has rows of its own above carried ones. Each part keeps
its own order by the rule above. Which rows are carried is read when Today
is opened, so a carried row unpicked there keeps its place, dimmed; `t`
picking a row there for today moves it to the end of the day's own rows,
since the act is the user's. `J` and `K` do not cross the rule and say so,
as `carried picks stay below the line: t t picks it for today`.

### Bottom line

The status line becomes a prompt for three acts, named at its left edge:

- `a` **Inbox ›** files a typed line in the Inbox. A `%name` word files it in
  the project or heading that name matches instead; the line names where it
  will land, and a blank line names `%name`, the one word no other line
  reads. One matching nothing stays in the text. `Enter` keeps the line
  and asks for the next, saying where that one went, as `added to Alpha ›
  TUI`; a blank `Enter` or `Esc` closes, and `Esc` keeps what was typed.
- `f` **Go to ›** opens a project, a heading, a list or a saved filter by
  name. A heading opens inside its project: the sidebar stays on the
  project and `Esc` backs out to it. Once something is typed it reaches
  every other row too, by words of its text: the row is named with where
  it lives, and opens among its siblings with the cursor on it, inside its
  project as a heading does, shown even when it is closed and `:closed` is
  on. A row must hold what is typed, at its start, at the start of a word or
  inside; only a place answers to its letters in order. Places come before
  rows on equal match ranks. A blank `f` `Enter` opens the Inbox.
- `L` **Link to ›** points the cursor's row or selection at a row found the
  same way, as described under Links.
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

**Home.** `gf` opens the whole project the cursor's row is filed in, or the
Inbox, from a list, a filter or a zoom inside the project. The cursor lands
on the row: the rows above it are unfolded, and a closed row is shown even
when `:closed` is on. The sidebar moves to the project, and it is a jump, so
`Ctrl+O` comes back. On a project's own row, or already in the whole
project, it says so and stays. It is not a second route to `h` (rule 3):
`h` looks at the row among its siblings and `Esc` returns, while `gf`
travels.

**The change list.** `g;` goes back to the row the last change was made on,
and `g,` forward again; a count goes that many changes. Each saved change
notes its row, a run of changes to one row once, up to 100, for this session
only; a saved filter is not one. A change on the cursor's row is passed over,
so `g;` always moves. A row since deleted stands for its nearest live
ancestor. A row on screen is reached as a motion is; elsewhere it opens where
it lives, as `f` opens a row, and that is a jump, so `Ctrl+O` comes back.
The status line says `change 4 of 9`, or that the walk is at either end.
The trail goes where jumps left and the change list where work was done, so
neither repeats the other (rule 3), and marks are still not needed.

**The trail.** `Ctrl+O` goes back to the place the last jump left, and
`Ctrl+I` forward again; a count goes that many places. A jump is a picker
destination opened with `f` `Enter`, a project reached with `gf`, a link
followed with `gl`, a change
reached with `g;` or `g,` in another place, or a sidebar
visit ending in a different place; walking several sidebar entries counts
once, from where `Tab` was pressed. `Enter` zooming, `Esc` backing out, `h`
and the trail keys themselves are not jumps. Going back restores the place's
held rows, filter, zoom, sidebar entry and cursor. A new jump drops the
places `Ctrl+I` would have reached. A place whose root row is no longer live
is passed over. The trail holds 100 places, for this session only; at either
end the status line says so. The picker offers no way back: that would
repeat a key (rule 3). `Ctrl+I` is told from `Tab` by the keyboard protocol
the app asks every terminal for, which Kitty, Ghostty, WezTerm, foot,
Alacritty, iTerm2 and xterm answer, and tmux passes on with `extended-keys
on`. A terminal that answers nothing sends `Tab` for both, so there `Ctrl+I`
gives the sidebar focus and the trail only goes back.

**A line that stays open says why.** While the bottom line is a prompt it
is the only place left to say anything, so what an `Enter` did or could not
do is said there, after what was typed and in place of the hint, until the
next key: where a captured line went, the word a stamp cannot read, that a
picker matches nothing. One saying that something was not written is in the
alert colour, as on the status line.

The pickers list their matches as you type, best first. `Down` and `Up`
walk them, and `Enter` takes the one at the front. A name matches by its start, the start of a word, anywhere
inside, or its letters in order. `Esc` leaves a picker without acting.

**Recall.** On the filter, stamp and command lines `Up` brings back the
line last kept there, and again the one before; `Down` comes forward, and
past the newest to the line as it was typed. Only lines beginning with what
was typed come back, so `/#h` `Up` walks the filters that began that way.
The filter narrows the pane to each line as it is shown. A line is kept
when it is used: a filter with `Enter`, a stamp when it is applied, a
command with `Enter`; one given twice is kept once, at the newest end.
`Ctrl+P` and `Ctrl+N` are aliases. The command line recalls once its name
is finished, so `:sort ` `Up` walks the sorts given before; until then the
same keys complete the name. The pickers and the capture line keep
nothing: there the arrows walk the matches.

### Links

A row can point at one other row. `L` asks for it by name, as `f` does but
without the lists, the Inbox and the saved filters, and sets it on the
cursor's row, a count of rows or the selection as one undo step. The row
then shows `→` and the text of the row it points at, as that row reads now,
or `→ gone` once that row is deleted. `gl` follows the link: it is a jump,
so `Ctrl+O` comes back. On a row that has a link, a blank query offers
`none` at the front, which takes the link off. A row cannot point at itself.
The link is the field `link`, so it is undone, backed up and synced like any
other change; copy and export do not carry it, since a pasted row is a new
row.

With no row link, `gl` opens the first web address in the row's text,
dropping trailing sentence punctuation as the drawn hyperlink does. It
starts the system opener (`open` on macOS, `xdg-open` elsewhere) without
waiting; the status line says `opened <address>` or the error starting the
opener. This changes no row, makes no undo step or log entry, leaves `.`
unchanged and is not a jump. It works in lists and filters; a selection
refuses it. A row link wins even when its destination is gone.

A row that live rows link to shows `←` and the row that links to it, as
that row reads now, or how many do, as `← 3`; the answer comes from the
scan, so drawing it walks nothing. With neither a link nor an address,
`gl` on such a row goes back to the one row that links to it, as a jump.
When several do, the status line says `3 rows link here: /&. lists them`.
With none of these, it says `this row has no link: L makes one`.

Three more acts use the same line:

- `/` **Filter ›** narrows the pane as the query is typed. Words match a
  row's text or its body, ignoring case. A leading `=` holds a word or
  phrase to the row's own line, body left out: `=milk` is the rows whose
  line says milk, `=milk $` the tasks among them, and `milk -=milk` the
  rows whose body alone does. It goes after `-` and any of `^` `_` `<` `>`
  and before the word, and is read back as `"milk" on the row's line`; a
  bare `=` is skipped while typing and `==` starts literal text. A match
  found only in the body draws its `≡` in the accent colour, underlined.
  Double quotes match a phrase, blanks included:
  `"per the spec"`. A quote not closed yet reads to the end of the line;
  a quote inside a word (`5"`) stays part of that word. A bare quote or empty
  phrase is passed over, including with prefixes or inside alternatives,
  until it holds text. Words inside an open or closed phrase stay plain,
  without sigil colours or completions. Phrases take the same prefixes as
  words: `-"per the spec"` excludes, and `^`, `_`, `<`,
  `>` test the phrase on related rows. `!`, `#tag`, `$state` and `@date`
  match fields; a leading `-` excludes. `#name` holds whatever value the tag
  carries and `#name=value` only that one; `#name<value` holds for a value at most that
  and `#name>value` for one at least that, compared as numbers when both
  are and else as text ignoring case, so `#est<3` and `#who>m` both read.
  `##name` holds a row with that context, `##` alone any context, and `-##`
  a row with none; `##home|town` reads either.
  `#` alone is any tag, `$` alone is any state (a task),
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
  bar standing alone, with a blank on each side, splits the terms around it
  into alternatives, and brackets standing alone set a part off as one
  term: `#home $open | #work !` is the open rows at home or the flagged
  ones at work, `fix ( #docs | cache )` needs `fix` and either of the two,
  and `-(` opens a part that must not hold (`#bug -( cache | ! )`). Parts
  nest. Terms side by side bind before a bar, so brackets are only needed
  around a bar that is not the whole line. A part left empty, a bracket
  not closed yet and one closed too often are passed over, so the pane
  follows the line while it is typed. `(`, `)` and `|` touching a word
  belong to that word.
  A bar inside a word offers alternatives, and the word holds when any does:
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
  A leading `_` tests the term on the rows under the row, at any depth,
  excluding the row itself: `_@overdue` finds every row with an overdue row
  somewhere beneath it, `_#bug alpha` the rows named alpha that hold a bug,
  and `-_$open|doing` the rows with nothing left to do under them. It
  reads as `^` does: once, before the alternatives, with `-` before it; a
  bare `_` is skipped while typing and `__` starts literal text. Rows under
  a row count whether or not a fold or `:closed` hides them.
  A leading `<` tests the term on the row's parent alone, and a leading `>`
  on the rows directly under it: `<#work` is the rows whose parent is
  tagged, `>$waiting` the rows holding a waiting row of their own, and
  `->$` the rows with no task directly under them. They read as `^` and `_`
  do: once, before the alternatives, with `-` before them; a bare `<` or
  `>` is skipped while typing and `<<` or `>>` starts literal text. A
  top-level row has no parent, so `<term` never holds for it.
  `$next` holds for each active project's next action: its first row, in
  outline order, that is open or doing, has no start day still ahead, and is
  not parked, under a parked row, or in a closed project, so the order of a
  project's rows is its priority. It is one row per project, not per
  heading; the Inbox has none.
  It combines like any state word (`$next #home`, `-$next`,
  `$next|waiting`), so `/$next`, `Tab` and
  `:save next` keep the daily list of what could be done.
  `$first` holds for the first workable sibling under each parent: open
  or doing, no start day still ahead, not parked or under a parked row,
  and not in a closed project. It includes the Inbox and combines like
  any state word (`$first #home`, `$first|waiting`). `$first -<$` gives
  the next action under each heading and project, leaving out children
  of tasks. Both phrases and `$first` work in saved filters and with `n`
  and `N`, just as they do on the filter line.
  `$today` holds for a row in Today, picked or starting on the day, and is
  read like `$next`: a filter word, not a state.
  `$missed` holds for a pick carried into Today from an earlier day, a
  task still open, doing or waiting, so `$missed #work` is what the work
  days left undone. It is read like `$next`.
  `$project` holds for a project, a top-level row other than the Inbox,
  and is read like `$next`, so `$parked $project` is the parked projects
  the sidebar counts. `$pinned` holds for a row pinned with `^` (see Pins)
  and is read like `$next`: a leading `^` reads the ancestors, so the pin
  is a word here. `$p` still reads as `parked`, since only a state
  is read from a prefix; `Tab` after it offers both.
  `&` holds for a row that links to another (see Links): `&` alone for any
  link, `&word` for a link to a row whose text holds the word, and `&.` for
  a link to the row the cursor was on when the filter was opened, which is
  how the rows pointing at a row are found. It reads like any sigil
  (`-&`, `&ann|bob`, `^&.`). A filter reads the pane, so in Projects `&.`
  finds every such row. A saved filter and `n` read `&.` as the cursor's
  row when the filter was opened. In an
  outline a match keeps its
  ancestors, dimmed, so it is read where it lives; `Tab` switches to a flat
  list of matches and back. While the line ends in a sigil that has values
  to take (`$`, `$d`, `#ho`, `@to`, `$next|w`), `Tab` completes it instead:
  it writes the first value the suggestion offers, again the next, and past
  the last the sigil as typed; `Shift+Tab` walks back. A sigil only one
  value finishes is finished at once, and then `Tab` switches the shape
  again. While the walk lasts, every value is listed one above another over
  the rows, under the letters it completes or at the pane's left edge when
  those stand under the sidebar, with the one written reversed;
  a list longer than ten, or than the pane, scrolls to keep it in view, and
  `Down`, `Ctrl+N`, `Up` and `Ctrl+P` walk the list as `Tab` and
  `Shift+Tab` do. Any other key ends the walk and closes the list. The status line says `Tab completes`
  or `Tab switches`, whichever `Tab` will do. A match is drawn with what was found in it:
  each word of its text that a looked-for word or phrase touches, and each
  tag the query names, in the accent colour and underlined. A term that
  excludes, one read on other rows (`^ _ < >`) and one inside `-(` mark
  nothing. While the line is typed the status line reads it back in words,
  after the count of matches (`no match`, `1 match`, `12 matches`), so its
  marks need not be remembered. A blank line hides nothing, so it counts
  nothing and names what the line reads, as `words, #tag, $state, @date, !
  · - excludes`:
  `-_$open|doing` says `no row under it: open or doing`, `$done ~7d` says
  `done and changed since Nov 8`, and a bar or brackets standing alone read
  as `, or` and brackets. A word that starts like a sigil and cannot be
  read is looked for as text, and says so with what a word of its kind
  reads as, as the stamp line does: `"@tomorow" as text: a day reads as
  @fri, @+3d, @dec4 or @2026-12-04`. A tag being typed names the tags in use instead.
  When the query has an un-negated `~` term on the row itself, each match
  says the day its state last changed, or it was made, as `~today` or
  `~Nov 12`, after its fields; a flat list of such matches reads newest
  first, ties in outline order, unless a deadline term orders it.
  When the query has an un-negated deadline term
  (`@`, `@date`, `@overdue`, `@week`, `@<`, `@>`), the flat list reads
  soonest deadline first, ties in outline order; in context, and in Today,
  Waiting and Stale, the order is the outline's or the list's. A flat list
  then puts its done and canceled rows last, each part keeping its order,
  so what is left to do reads first. The order is
  set when the query is read, and nothing in the outline moves: a row
  closed while the filter shows stays where it is. The cursor
  goes to the first match as the line is typed: in context the rows above
  it are only there to place it, and no key is meant for them. `Enter`
  keeps the filter and returns to the rows, where the header names it, and
  the status line says what is held and the keys that are new while it is,
  as `12 matches kept · n N step through them · Esc drops the filter`,
  since the count leaves with the line; `Esc` drops it. Like a list, a filter holds the
  rows it found until the query changes, and refuses the keys that
  restructure the tree. Changing the filter clears the selection. A filter
  shows every match: the outline's folds never hide one. In context it
  folds on its own instead: `h`, `l`, `zc`, `zo`, `zM`, `zR` and `:fold`
  fold and unfold as in the outline, a folded row counts the rows the
  filter found under it (`+1`), and `h` on a folded row or one with nothing
  under it steps to the parent shown. These folds are held with the filter,
  never saved, and dropped when the query changes. In a flat filter, `h` opens the
  cursor's row among its siblings in its parent's outline, keeping it
  visible even with `:closed` on; `Esc` returns to the held filter with
  its query and cursor intact. `h` refuses a selection and `l` does
  nothing there. Neither changes a fold. A flat row names its project, as
  a list's does; when the pane is inside that project, which the header
  names already, it names the nearest heading over the row instead, colon
  dropped, and nothing when there is none.
- `#` `@` `$` `*` `-` `~` **Stamp ›** opens holding that character, and lists
  what that sigil could become. The line is
  sigils only: `#tag`, `##context`, `@date`, `@@date`, `*interval`, `$state`, `~colour`, `!` and `^`
  set; `-#tag`, `-##`, `-@`, `-@@`, `-*`, `-~`, `-!` and `-^` remove; `-##name`
  takes the context off only the rows that have that one. `#name=value` sets the value,
  replacing any the name had; `-#name` takes the tag off whatever its value
  and `-#name=value` only that one. `@++1w` and `@--3d` move a deadline on
  or back from the day the row has, not from today, by a count of `d`, `w`,
  `m` or `y`, a month keeping its day as a repeat does; `@@++1w` moves a
  start day. A row with no such day is left as it is, so a selection of
  rows each slips from its own date, and `.` slips them again. These two
  forms are read on the stamp line only. `Enter` applies them to the selection, else to the cursor's row, as
  one undo step. A word it cannot read is named on the line, with what a
  word of its kind reads as (`cannot read @firday: a day reads as @fri,
  @+3d, @dec4 or @2026-12-04`); nothing is applied and the line stays open
  to be put right.
- `:` runs a command. The line names the commands it could become, dimmed:
  all of them by name alone while it is blank, so more fit and are scanned
  sooner, those that start as typed with what each takes after that, and
  the one that is left with what it does, as `sort [keys]: order siblings
  once: @ deadline, ! flag, $ state, else text`; a line that starts none
  says `no such command`. A name is read in any case, so `:Q` is `:q` when
Shift or Caps Lock was held. `Enter` on a name that only one command starts, as
  `hel`, runs that command; one given anyway is named with the list. While
  the line holds no more than a name, `Tab` writes the first command it
  could become and `Tab` again the next, each with the space or slash that
  what it takes is typed after, and past the last the line as it was typed;
  `Shift+Tab` walks back. The commands are listed one above another over the
  rows, as a filter value is, with the one written reversed, and the status
  line says what it does. A name only one command starts is finished at once. `Down`, `Ctrl+N` and
  `Ctrl+J` are aliases of `Tab`, and `Up`, `Ctrl+P` and `Ctrl+K` of
  `Shift+Tab`; once the name is finished the same keys do what they do on
  any line. `q` quits; `help` opens help; `tutor` adds a fresh
  `Welcome` tutorial as the last project and opens it, from any place, in
  one undo step; `export [file]` writes
  the whole outline as tab-indented lines in the row grammar, to `~/docket-YYYY-MM-DD.txt` unless
  a file is named; `import file` reads such a file back, as described below;
  `closed` hides or shows closed rows; `numbers` shows or
  hides each row's distance from the cursor, and the cursor's own place; `all` selects every
  row the pane shows, as described under Selection; `s/old/new` changes
  text across rows, as described below; `archive` moves
  closed rows under an `Archive:` heading, as described below; `review`
  opens the review list; `fold level` shows that many levels, as described below;
  `sort [keys]` orders siblings once, as described below;
  `save name` and `forget name` keep and drop a filter; `sync
  folder` and `sync off` are described under Sync.

`zM` folds every row with children under the pane's root and `zR` unfolds
them all, as in Vim; a count and the selection play no part. `:fold N`
shows N levels: rows above level N are unfolded and rows at level N are
folded, with the root's children at level 1. Deeper folds stay as they were.
`:fold` takes a level, since folding all is `zM`; without one, or with an
invalid one, it says so on the status line and changes nothing, and
`:unfold` names `zR`. A filter in context folds
its own rows, as described under Filter; lists and flat filters say there is
nothing to fold. The cursor stays
on its row, or moves to its nearest visible ancestor. Folds are saved together
as this machine's view state, never logged or synced.

`zm` folds one level more and `zr` opens one, as in Vim: `zm` folds the
deepest shown rows whose rows are shown under them, and `zr` opens the
shallowest folded rows shown. A count takes that many levels, `2zm` two,
stopping when there is no level left; with none at all `zm` says every row
is folded and `zr` that nothing is. The selection plays no part.

`zc` folds the cursor's row and every row under it that has rows of its
own, and `zo` unfolds them all, so a branch is shut or laid open whole
where `h` and `l` take one level. They act on the selection, or with a
count on that many shown rows, and a row with nothing under it says so.
`zM`, `zR` and `:fold N` work from the pane's root, `zm` and `zr` from
the folds it already shows, and `zc`, `zo` from a row, so none repeats
another (rule 3): `:fold N` names a level outright, `zm` and `zr` move from
wherever the pane stands.

`:s/old/new` puts `new` wherever `old` stands in the text of the cursor's
row, or of every selected row, as one undo step: `/old`, `:all`,
`:s/old/new` renames across a project. It reads the text as typed, capitals
included, and changes every place in a row; fields are not text, so a tag
is renamed with a stamp. The mark after the `s` may be any character that
is not a letter, a digit or a blank, for text that holds a slash
(`:s,a/b,c`). A last mark may close the line, which is how `new` ends in a
blank, and `new` may be nothing. The status line counts the rows changed,
or says no row holds `old`; a line that is not whole changes nothing.

`:sort [keys]` sorts the cursor's row and all its siblings, or a selected
block of adjacent siblings under the Selection rule. Selected descendants
travel with their selected ancestor. Children travel with their rows and
are not sorted. Keys are space-separated: `@` puts soonest deadlines first
and missing deadlines last; `!` puts flagged rows first; `$` orders doing,
open, waiting, notes, parked, done, canceled. Each later key breaks ties;
without keys, text is compared ignoring case. Ties keep their original
order. Pinned rows stay first: each group is sorted on its own (see Pins).
Rows stay inside their original sibling span, in one undo step,
with the cursor on its row and the selection kept. The status says how many
rows were sorted, or that they are already in order (with no undo step).
Lists, filters and incompatible selections refuse the command; an unknown
key is named and nothing changes. Rule 2 holds: nothing stays sorted;
sorting only happens when the command is given.

`:import file` reads a file of indented lines as paste reads them, so what
`:export` wrote, a copy made with `yy`, or any indented list or checklist,
and adds its rows after the cursor's row in one undo step, the cursor on
the last. In Projects an export's lines are projects again, and the rows
under its `Inbox` line go back to the Inbox; anywhere else every line is a
row. The rows are new: importing twice adds them twice, and links are not
carried, as with copy. Lists, filters and selections refuse it; a file that
cannot be read is said and nothing changes.

A file from another outliner is read as what its name says it is, and then
as above. A `.taskpaper` file: a line starting with a dash is a task, one
ending in a colon a heading, any other a note, at the depth of its tabs;
`@done` and `@cancelled` close the row, `@flagged` flags it, `@due(day)`
and `@start(day)` or `@defer(day)` set its days, and every other `@name` or
`@name(value)` becomes `#name` or `#name=value`, blanks in a value written
as dashes. A name that cannot be a tag stays in the text. An `.opml` file:
each row is a note, or a task where the file says checked or not, and a
row's note becomes rows under it. A `.bike` file: a task is a task, done
when it says so, a heading gains the colon a heading is read by, and every
other kind of row is a note, its inline formatting dropped. Every state is
spelled, so the rows read the same wherever they land. Nothing is written
in these formats: `:export` stays the row grammar.

`Ctrl+D` and `Ctrl+U` move down and up by half the pane's usable row height
(excluding header and status), at least one row, stopping at either end;
`Ctrl+F` and `Ctrl+B`, or `PgDn` and `PgUp`, move by the whole of it.
They work in outlines, lists and filters and keep the cursor visible. While
typing they retain their existing behavior (`Ctrl+U` cuts before the caret);
in help they retain half-page scrolling.

### Help

`:help` opens a reference in eight sections, in the tutorial's order. The
first screen gives the essentials; `]` and `[` go to the next and previous
heading. `j` and `k`, paging keys and `gg` and `G` scroll; any other key
closes it. `/` opens a `Search ›` line on the status row: the page goes, as
each key is typed, to the first line at or below where it stood that holds
the text, read without case, with every match on screen reversed and the
line stood on tinted; the line counts the matches. `Enter` keeps the
search, `Esc` drops it and returns the page to where it stood. While one is
kept, `n` and `N` go to the next and previous match, round past either end,
the status line says which of how many is stood on, and `Esc` drops the
search before it closes help. It is drawn as the app is (rule 6): the sections stand where the
sidebar does, from 72 columns up, with the one being read marked; a header
names that section; the status line gives the keys, a waiting `g` and where
the page is, as `Top`, `40%` or `Bot`. Each line is keys in the accent
colour in one column and what they do beside them, wrapped under itself at
the pane's width and no wider than 88 columns; keys named inside a
description are bold and a note on the line above is dim. A section groups
its lines under bold sub-headings. Narrower than 48 columns the keys sit
above what they do. `:tutor` teaches on editable outline rows, read through the same
reader as `:import`. `dd` on its `Welcome` project removes it; another
`:tutor` makes a fresh copy with new row identities. Exercises that act on
the next row have a sibling task to receive the action. The split-and-join
exercise names the space inserted by a join; sidebar travel starts with
`k` from Welcome's last entry.

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

### Pins

`^` pins the cursor's row, a count of rows or the selection, as one undo
step, and again unpins it; on a selection it pins every row unless all are
pinned already, as `!` flags. A pinned row is drawn first among its
siblings, after any pinned before it in the outline's order, and the cursor
goes with it, so a pinned project leads Projects and the sidebar's active
projects, after the Inbox, and a pinned task becomes its project's `$next`.
The pin is the field `pin` and no row's `pos` changes: siblings are ordered
pinned first, each group by its own `pos`, and a position is only ever
read inside one group. So an unpinned row goes back to where it was among
the rest, undo is one `set`, and a sync merges it like any field. It is the
user's act, so rule 2 holds.

`J` and `K` reorder inside a group and do not cross into the other, saying
`pinned rows stay above the rest: ^ unpins it`, or `^ pins it` from below.
`:sort` sorts each group on its own. `o` and `O` open a row with the pin of
the sibling they open beside, so it is drawn where it was opened, whatever
its line says; a put or pasted line says for itself, by `^`, and lands as
near the spot as its group allows. A row keeps its pin when it moves, by
`m`, `>>`, `<<`, `here` or `:archive`, and is drawn first under its new
parent. Today, Waiting, Stale, review and a held filter keep their own
orders: `^` sets the field there and nothing moves. `^` does nothing in the
sidebar, and no rule is drawn under the pinned rows: the mark says it.

### Saved filters and review

`:save name` keeps the pane's filter, with the row it was made under and
whether it was flat. It then appears in the `f` picker as `/name`, and
opens with the cursor on its first match unless the place remembers a row
the filter still shows.
`:forget name` drops it. It is also an entry in the sidebar, after the four
lists, reached with the sidebar's ordinary keys: a place, as a list is, and
not a second route to a key (rule 3). Each is a row
under a hidden top-level row with the fixed identity `filters`, so saving
and forgetting are undone, backed up and synced like any other change.

`:review` is a list of the active projects, the longest untouched first,
each with how many tasks are left to do, or `∅`, and its age, in two
columns at the pane's right edge as Waiting's ages stand, the age at the
edge: `2 to do  12d`. The age is drawn as the sidebar draws it: none for a
project touched today, its column left blank, and otherwise against the
oldest active project. Every row there is a project, so the glyph column
leaves out a note's `·`, which would say the same of every row; a project
parked or closed during the review keeps its glyph and has no columns
left. It is a list
like the others, so the ordinary keys do the review: moving on keeps a
project, `Enter` looks inside, and a stamp parks (`$p`) or closes (`$done`)
it.

### Selection

`v` selects the cursor's row and steps down; again on a selected row it
deselects. While rows are selected, `x`, `!`, `^`, `t`, `dd`, `m`, `L`, `>>`, `<<`,
`J`, `K` and a stamp act on the selection, in one undo step. `x`, `!`, `^` and `t` set
their value on every row unless all already hold it, in which case they
clear it. The keys that work on one row (`A`, `I`, `cc`, `o`, `O`, `Enter`,
`gf`, `gl`, `gn`, `gN` and the word motions that step into a row) are refused and say so. The status line counts the selection, in the accent colour its
marks are drawn in since the keys now act on those rows, and `Esc` clears it.

`V` starts a sweep at the cursor's row. Each motion, with or without a
count, selects the shown span from that anchor to the cursor, both included;
moving back gives rows up again. Picks made before the sweep stay selected.
While it lasts the status line says so beside the count, as `3 selected ·
V keeps · Esc clears`.
`V` ends the sweep and keeps the rows; `v` ends it, then toggles the cursor's
row and steps down as usual. `o` swaps its ends, as in Vim: the cursor goes
to the anchor's row and the row it left becomes the anchor, so the other
end moves next; it opens no row. `zt`, `zz` and `zb` leave it as it is. Any
other non-motion key ends the sweep before acting: `VGdd` deletes through the end in one undo step, and `V2jx` closes
three rows. `Esc` ends the sweep and clears the selection (a pending count
is still canceled first). Sweeps work in lists and filters, passing over
dimmed ancestors in a contextual filter. Changing the filter, jumping or
opening a place clears both the selection and the sweep.

`gv` selects again the rows the selection held when it was last cleared,
by `Esc`, a filter, a jump or a place opened, as Vim's `gv` does: those of
them shown here, matches only in a filter in context. With a selection
standing, the two change places, so `gv` again returns to it. With nothing
cleared yet, or none of its rows shown, it says so. It drops a count.

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

For `>>`, `<<`, `J` and `K`, a selected row with a selected ancestor travels
with that ancestor. The remaining rows must be adjacent children of one
parent; otherwise the move is refused and nothing changes. `>>` appends the
block under the sibling above it and unfolds that sibling; `<<` places it
just after its parent. The rows `>` or `<` takes with a motion or a count
are a block under the same rule. `J` moves it past the sibling below; `K` moves it
before the sibling above. Order within the block, the selection and the
cursor's row stay the same, so the key can be pressed again. Lists and
filters still refuse these keys.

`Esc` undoes the most recent narrowing: the selection, then the filter, then
the zoom.

**Repeat.** `.` makes the last change to rows again, on the selection, else
on the cursor's row, as one undo step. What it remembers is the last of:

- a stamp, remembered as typed and read again against today (`@+3d` counts
  from the day it is repeated);
- a named move, which remembers the destination row and its label: a
  project, Inbox or heading, and appends the rows as its last children,
  saying `moved to <label>`;
- a link, to the same row or to none;
- a substitution, with the same `old` and `new`;
- `x`, `!`, `^`, `t`, `J`, `K`, `p`, `P`, `gp` or `gP`, pressed again as the key
  it is, so `x` on a done row opens it;
- `d`, `>` or `<` with what completed it: `dd` takes rows again, `d}`
  reads its motion again from the cursor and `dic` its object;
- text typed after `o`, `O`, `A`, `I` or `cc`, when adding text where the
  typing began is all the typing did: it is typed again into a new row, or
  into each row at the place the key would have put the caret. A line that
  was split, joined, or edited elsewhere is not remembered.

A key or operator is repeated with the count it was given; a count before
`.` replaces it, and rows selected now are what it lands on. A copy, a
motion, an unreadable stamp, a move to `here`, a key that changed nothing
and `u` leave the remembered act unchanged. It lasts only for this session.
A destination that is no longer live is named on the status line and
nothing changes; moving a row under itself is refused as with `m`. With
nothing remembered, `.` says `nothing to repeat`. It works wherever the
remembered act works, including lists and filters; with sidebar focus it
does nothing. Changes made inside a row have their own `.`, described under
Inside a row.

## Keys

**Grammar.** Motions are `j` `k` `gg` `G` `Ctrl+D` `Ctrl+U` `Ctrl+F`
`Ctrl+B`, the structural motions `}` `{` `)` `(` `H` `]` `[` `zj` `zk` and the find
motions `n` `N`; `h` and `l` are fold keys, not motions. The operators are
`d`, `y`, `>` and `<`. In the outline, digits
`1` to `9` start a count and `0` continues it; `0` alone does nothing.
The status line shows, at its right and as typed, the keys that wait for
more: a count, an operator, a `g`, `z` or `i`, so `2d3` and `3g` read as
they were pressed. It does so in the outline, the sidebar, inside a row and
in help, beside whatever the line already says, and a line too narrow for
both keeps the pending keys. In the outline, a waiting `g`, `z`, operator
or operator with `g` or `i` also says at the line's left, dimmed, what may
follow it (`gg top · gf project · gl follow link · …`, `dd the row · dj } ) H ] n G…
through a motion · dic dis objects`), so the grammar teaches itself; a
count alone says nothing. While rows are selected an operator offers only
what it can take, as `dd the 3 selected rows · no motion or object while
rows are selected`, and `c` says it retypes one row. `Esc`
drops it without clearing a selection, filter or zoom. Counts do not apply
in the sidebar, help, prompts or while typing.

A count repeats `j`, `k`, the paging motions and their arrow and page
aliases, and `}`, `{`, `H`, `]`, `[`, `zj`, `zk`, `n` and `N`, stopping at either end.
`gg`, `G`, `Home` and `End` take it as the shown row to go to, counted
from 1 as `:numbers` draws the cursor's, and stop at the last: `12G` goes to
the twelfth row and `d3G` deletes through the third. `)` and `(` drop it.
`Ctrl+E` and `Ctrl+Y` take it as rows to scroll.
It also repeats `J`, `K`, `u` and `Ctrl+R`: `3J` is one undo step, while
`3u` undoes three steps. `J` and `K` stop when the row can go no further
and keep the moves already made, including in Today; at the end they say
the row is already first or last of its siblings, and that `<<`, `>>` or `m`
move it elsewhere.

`u` and `Ctrl+R` say on the status line what they took back or made
again, since undo is the only safety net (rule 1): rows that came back,
rows gone, rows moved, and which fields changed on how many rows, as
`undo: 3 rows back` or `redo: text and tags changed on 2 rows`. A row that came back or went is
counted with the rows under it, as a delete counts them. A counted
undo says how many steps it took, as `undo: 3 steps`.

**Structural motions** read the shown rows by depth and change no fold.
`}` and `{` go to the next and previous sibling, passing over the rows under
each; `)` and `(` go to the last and first sibling. `H` goes to the parent,
and a count climbs that many levels, stopping at the pane's top level. `]`
and `[` go to the next and previous shown row ending in a colon, at any
depth. `zj` and `zk` go to the next and previous shown row with rows under
it, folded or open, at any depth, as Vim's go to the next fold. A motion with nowhere to go stays put. In a list or a flat filter
every row is at one level, so `}` and `{` step as `j` and `k`, `)` and `(`
reach the ends and `H` stays. Like every motion they take a count, sweep
with `V` and complete an operator: `d}` deletes this branch and the next,
`y)` copies through the last sibling, `VH` selects up to the parent,
`dzj` deletes through the next row with rows under it.

**Into the match.** `gn` steps into the text of the cursor's row with the
caret on the first place the filter's words stand, and `gN` on the last; a
row that holds none hands over to the next or previous shown match that
does. The places are those a match is drawn with: each looked-for word or
phrase, ignoring case, and each tag the query names, in the line `A` opens.
Inside the row `n` and `N` go on to the next and previous place, keeping
the row and stepping into the next shown match when this one has no more,
so `/old` `Enter` `Esc` `gn` `cw` `new` `Esc` `n` `.` changes a word across
rows without leaving the text. Each row is its own undo step. After an
operator `n` and `N` stay in the row (`dn` deletes up to the next place).
A filter that names no word or tag says so and names `w`; with no filter
kept, both say there is nothing to find. `gn` and `gN` drop a count and
refuse a selection.

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

An operator waits for a doubled key or a motion. `dd`, `yy`, `>>` and `<<`
act on the selection, else the cursor's row; a count takes shown rows down
from there.
With a motion they take every shown row from the cursor to its destination,
both included, leaving dimmed ancestors out. Counts before the operator
and before the motion multiply: `d3j` and `3dj` both delete four rows.
An operator with a motion is refused while rows are selected: double the
key instead. `Esc` or any other key cancels the pending operator and does
nothing. `y` alone waits; a completed copy keeps the cursor where it was.
Every copy and delete writes whole subtrees to the session's text register
and system clipboard; delete writes before deleting and is one undo step.
`u` restores the deleted rows without changing the register.

**Row objects.** An operator also takes a part of the tree named from the
cursor's row: `ic` is the rows under it, and `is` is the row with all its
siblings. Each row goes with everything under it, shown or folded. `dic`
empties a row and `yic` copies what it holds; `<ic` dissolves a group,
leaving its rows after it; `yis` copies a whole level and `>is` is refused
as `>>` on a first row is. They are objects, as `iw` is inside a row, and
not motions: the cursor does not move, a count is dropped, and `V` does not
sweep with them. A row with nothing under it says so. A selection, a list
and a filter refuse them. No `ac` or `as` exists: the row with what is
under it is the doubled key, and the siblings with their parent is `H`
then the doubled key (rule 3).

Before `x`, `!`, `^`, `t`, `dd`, `yy`, `>>`, `<<`, `m`, `L`, `.`, `zc`, `zo`, or a stamp character
(`#` `@` `$` `*` `-` `~`), a count takes that many shown rows from the cursor
down, or as many as remain, without changing the selection. In a filter
in context it passes over dimmed ancestors, taking matches only, as `:all`
does. The act is one undo step; copy changes no rows. `>` and `<`, counted
or with a motion, follow the Selection rule for a block. `:numbers` shows
the count each row is at. `5v` selects five rows as five
presses of `v` would, stopping at the last row. With rows already selected,
a count before an act on rows (including `v`) is refused and says so.
The word motions that step into a row take the count as theirs, and
`Ctrl+O` and `Ctrl+I` go that many places. Every other key, including `o`,
`O`, `A`, `I`, `cc`, `Enter`, `h`, `l`, `f`, `gf`, `gl`, `gn`, `gN`, `p`, `P`, `gp`, `gP`,
`zt`, `zz`, `zb`, `a`, `/`, `:`, `gv`, `gi` and `Tab`, drops the count and acts as usual, as do `zM` and `zR`.
`zm` and `zr` take the count as levels, and `Ctrl+A` and `Ctrl+X` as days.

**Moving a day.** `Ctrl+A` moves the deadline of the cursor's row, or of
each selected row, a day later and `Ctrl+X` a day earlier, as Vim adds to
a number; a row with no deadline moves its start instead, and a row with
neither is passed over. The count is days, not rows: `7 Ctrl+A` is a week.
It is one undo step, says how far it moved, and `.` moves as far again,
on the selection too. Rows with no day at all say `no day to move`. It is
the stamp `@++Nd` or `@@++Nd` made a key, since putting a day off is the
commonest change a list asks for; the stamp line keeps weeks, months and
years and setting a day outright.

**Placing the row.** `zt` draws the cursor's row at the top of the pane,
`zz` in the middle and `zb` at the bottom, as far as the ends of the rows
allow. The cursor stays on its row, nothing is selected or changed, and
the pane stays there until a motion has to scroll it.

**Scrolling the pane.** `Ctrl+E` scrolls the rows up a row, showing one
more below, and `Ctrl+Y` down a row; a count scrolls that many, as far as
the ends of the rows allow. The cursor stays on its row while the row is on
the pane and otherwise moves to the nearest row still drawn, as in Vim. A
sweep follows the cursor it moves; nothing is selected or changed
otherwise.

| Keys | Act | Milestone |
| --- | --- | --- |
| `j` `k` `gg` `G` | Move; `Home` `End` as `gg` `G`; a count before `gg` `G` names the row; `k` on the first row steps into the body | 1, 10, 23, 25 |
| `Ctrl+D` `Ctrl+U` | Move half a pane down, up; `Ctrl+F` `Ctrl+B` `PgDn` `PgUp` a whole pane | 7, 10 |
| `}` `{` `)` `(` | Next, previous sibling; last, first sibling | 12 |
| `H` `]` `[` | Parent; next, previous heading | 12 |
| `n` `N` | Next, previous match of the last filter | 12 |
| `gn` `gN` | Step into the text on what the filter found, first or last place | 18 |
| `h` `l` | Fold, unfold; step to parent, first child; in lists and flat filters, `h` shows the row where it lives (`Esc` returns), `l` does nothing | 1, 10 |
| `zc` `zo` | Fold, unfold the row's whole branch | 15 |
| `zM` `zR` | Fold, unfold every row in the pane | 15 |
| `zm` `zr` | Fold, unfold one level more of the pane | 24 |
| `zj` `zk` | Next, previous row with rows under it | 24 |
| `zt` `zz` `zb` | Draw the cursor's row at the top, middle, bottom of the pane | 15 |
| `Ctrl+E` `Ctrl+Y` | Scroll the pane a row, or the count, under the cursor | 25 |
| `Enter` `Esc` | Zoom in, back out | 1 |
| `o` `O` `A` `I` `cc` | New row below, above; type at end, at start; retype keeping fields | 1, 11 |
| `gi` | Type again where typing last ended, in that row | 24 |
| `w` `b` `e` `W` `B` `E` | Step into the row's text, on that word | 13 |
| `J` `K` | Reorder | 1 |
| `d` `y` `>` `<` | Delete, copy, indent, outdent: doubled they take rows; a motion takes an inclusive range; `ic` `is` take the rows under the row, the row and its siblings | 1, 7, 11, 13, 14 |
| `p` `P` | Put the register after or before the row, as new rows | 11 |
| `gp` `gP` | Put the last copy, whatever was deleted since | 14 |
| `u` `Ctrl+R` | Undo, redo | 1 |
| `.` | Repeat the last change to rows | 10, 13 |
| `x` `!` | Done and back, flag | 2 |
| `Ctrl+A` `Ctrl+X` | Move the row's deadline, else its start, a day or the count later, earlier | 24 |
| `^` | Pin, and unpin: drawn first among its siblings | 21 |
| `t` | Pick for Today, and unpick, a pick carried from an earlier day too | 3, 19 |
| `Tab` `f` | Sidebar focus, where `A` `I` `cc` `cw` rename the chosen project, `dd` deletes it and `u` `Ctrl+R` undo and redo; go to a project, heading, list or row by name | 3, 13 |
| `Ctrl+O` `Ctrl+I` | Back along the places jumps left, and forward | 13 |
| `g;` `g,` | Back to the rows changed this session, and on again | 17 |
| `a` `m` | Capture to the Inbox from anywhere; move to a project or heading | 3 |
| `gf` | Go to the whole project the row is filed in, the cursor on the row; a jump | 22 |
| `L` `gl` | Link the row to another; follow the link, else open its first web address, else go back to the row linking here | 13, 16, 19 |
| `v` `V` | Pick a row; sweep with motions; `o` while sweeping swaps its ends | 4, 11, 24 |
| `gv` | Select again the rows last cleared | 24 |
| `/` `:` | Filter, command | 4 |
| `#` `@` `$` `*` `-` `~` | Stamp sigils on rows; `~` a colour, so selected projects take one at once | 4, 19 |

A Vim key the outline does not bind (`i`, `s`, `S`, `C`, `r`, `R`, `X`,
`D`, `Y`, `U`, `q`) changes nothing and names the outline's way to do
what it means, as `i` says `I types at the start, A at the end; w steps
into the text`. So do the keys other programs taught: `?` names `:help`,
`Z` says every change is saved as it is made and that `:q` quits, `'` and
`` ` `` name the trail and the change list that stand in for marks, and
`Space` names `x`, `v` and `t`. `:w`, `:write`, `:wq` and `:x` answer as `Z`
does, while typed and on `Enter`, and do not quit. Each is a pointer, not a
second route (rule 3).

`A` opens the cursor's row for typing at the end of its canonical line; `I`
opens it at the start. `cc` removes its text and keeps its fields as sigils,
with a space before them and the caret at the start. Its state is explicit
so replacement text cannot change it by gaining or losing a trailing colon.
Keeping the line saves the new text in one undo step; leaving it blank keeps
the existing row with no text. All three drop counts, refuse a selection,
and work in lists and filters. `c` waits only for another `c`; `Esc` or any
other key drops it and does nothing.

`gi` opens typing in the row where typing last ended this session, with the
caret where it stood, as Vim's `gi`: the cursor goes to that row, and `Esc`
keeps it and returns to the outline. A body and a project's name in the
sidebar are not counted. A row since gone, or not shown here, says so and
names `u` or `g;`. It drops a count and refuses a selection.

### Inside a row

A word motion typed on a row steps into its text, as if a caret had been
waiting before the line: `w` lands on the first word and `3w` on the third,
`e` on the end of the first word, and `b` on the last word, counted from the
end. `W`, `B` and `E` count anything but a blank as one word, so a sigil such
as `@2026-11-20` is one. The line is the canonical one `A` opens, sigils
included; the caret is a block on a character, and the status line says
`in the row`. A selection refuses the keys; lists and filters do not.

There the keys are the same grammar over characters, and none of them is a
key of the outline. A waiting `d`, `c` or `y` says what may follow it, as
an operator on the outline does (`dw de d$ dt, dn through a motion · diw
di" di( objects`):

- **Motions.** `h` `l` by character and `0` `^` `$` to the ends; `w` `b` `e`
  and `W` `B` `E` by word, and `ge` `gE` back to the end of the word
  before; `%` to the bracket, `(` `[` or `{`, that pairs with the one at or
  after the caret, across a body's lines; `f` `F` to a character and `t` `T` to just before
  it, with `;` again and `,` the other way; `n` `N` to the next and
  previous place the filter's words stand, as described under Into the
  match; `j` `k` by drawn line of a wrapped row, and from its last or first
  line on into the row drawn below or above, keeping this one and the
  column, as `Down` and `Up` do while typing. Each row left is its own undo
  step; a selection stays in its row, and at either end of the pane the
  caret stays put. `Left`, `Right`, `Home` and `End` are aliases.
- **Operators.** `d`, `c` and `y` take a motion or a text object: `dw`,
  `c2e`, `d$`, `ct,`, `yiw`. The objects are a word (`iw`, `aw` with the
  blank beside it, `iW`, `aW`), what is between quotes or backticks (`i"`
  `a"`), and what is between brackets (`i(` or `ib`, `i[`, `i{` or `iB`,
  `i<`, and `a` for each with its brackets), and a field: `i@` a deadline
  or start, `i#` a tag or context, `i$` a state, `i~` a colour and `i*` a
  repeat. A field object is the field the caret is on, else the first
  after it, else the last before it, so `ci@` retypes the row's date from
  anywhere in the line; `i` leaves out what starts it (`@`, `@@`, `#`,
  `##`, `$`, `~`, `*`) and `a` takes the whole word with the blank after
  it, else before, as `aw` does. A word that is not read as that field,
  as `a@b` or `\#5`, is text and passed over, and a line with none says so
  and names the stamp line. A body reads no sigils, so it has none.
  `cw` on a word leaves the blank
  after it, as `ce` does. A doubled operator belongs to the outline, so
  here it cancels. `gu`, `gU` and `g~` lower, raise and switch the case of
  what a motion or an object names, as `gUiw`; doubled, as `guu`, `gUU`,
  `g~~` or `gUgU`, they take the whole line, since no outline key is
  theirs. A motion that takes the character it lands on, as `e`, `ge` and
  `%` do, takes the caret's too when it goes back.
- **Acts on characters.** `x` deletes the character and `X` the one before;
  `D` deletes and `C` changes to the end; `s` changes the character; `r`
  replaces it with the next key; `~` switches its case. `Ctrl+A` adds one,
  or the count, to the number at or after the caret, and `Ctrl+X` takes it
  away, as in Vim: `#est=2h` becomes `#est=3h`. A minus is the number's
  sign only where it starts a word, so `2026-11-09` holds three numbers, and
  leading zeros keep their width: `08` becomes `09`, while `10` less one is `9`. A
  line with no number from the caret on says so. Moving a row's dates by
  their meaning is `@++1w` on the stamp line (rule 3). `p` and `P` put
  what `d`, `c`, `y` or `x` last took after or before the caret: that is a
  text register of its own, kept for the session, and `y` also writes the
  system clipboard and draws what it took in reverse for a moment.
- **Typing.** `i` and `a` type at the caret and after it, `I` and `A` at the
  start and the end, and `c`, `C` and `s` type where they cut. `Esc` ends
  the typing and puts the caret back on a character, the last one typed,
  so the next change is made from there: `cw` `new` `Esc` `w` `.` changes
  two words on one visit.
- **Selecting.** `v` starts a selection at the caret, drawn in reverse.
  Motions and counts stretch it to wherever they go, both ends included,
  and `iw`, `a"` and the other text objects make it that object. `d` or
  `x` deletes it, `c` or `s` changes it, `y` copies it, `~` switches
  its case and `u` and `U` lower and raise it; `o` puts the caret at the
  other end, so that end moves, as it does in a sweep; the key that acts ends the selection, as do `v` and `Esc`,
  which stay in the row. The other keys do nothing meanwhile. `.` after
  such a change takes as many characters again from the caret.
- **Filtering.** `*` leaves the row, keeping it, and narrows the pane to
  the word under the caret as if it had been typed after `/` and kept with
  `Enter`: a sigil whole (`#work`, `$waiting`, `@2026-11-20`), else the
  word without what touches it. The cursor stays on the row, `n` and `N`
  walk the matches, and the filter line recalls it. It types nothing, so
  it is not a second route to `/` (rule 3). On a blank it says so.
- **Counts** go before a motion, an operator or an act, and multiply as in
  the outline: `3w`, `d2w`, `2dw`, `3x`, `2fa`.
- `u` and `Ctrl+R` undo and redo the changes made in this row since
  stepping in. `.` repeats the last change made inside a row at the caret,
  typed text included, and stays in the row; it is remembered for the
  session and apart from the outline's `.`.

`Esc` drops a pending count or operator, then ends a selection; otherwise
it keeps the row and returns to the outline. Typing ends where it was
opened: `Esc` after `i`, `a`, `c` or `s` pressed here goes back to the
caret, and `Esc` after `A`, `I`, `o` or `cc` pressed on the outline goes
back to the outline. The other keys of typing are the same in both: `Enter`
splits or keeps the row, and `Up` and `Down` go on typing in the next.
Everything done between stepping in and leaving is one undo step, and
nothing is written when nothing changed. `u` inside the row takes back one
change at a time, what was typed included.
A sync pass and another window's changes wait, as they do for typing.

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
`copied N rows`. The rows it took, those beneath them included, are lit for a
moment (200 ms) in a colour of their own, over the selection and the
cursor, so the copy can be checked against the screen; a copy made sooner
takes the light over. Copy works in lists and filters, using the chosen rows
without adding dimmed ancestors. It keeps the selection and cursor and makes
no undo step. `y` with a motion copies the inclusive shown range, excluding
dimmed ancestors, with the same message; `y` alone waits, and `Esc` or an unrelated key cancels it. Counts
before the operator and motion multiply. A motion with selected rows is
refused; use `yy` (or `dd` to delete) on the selection. `dd` and `d` with a
motion write the same subtree text before deleting, as one undo step, and
say `deleted N rows`, counting the rows under each as copy does: delete
asks nothing (rule 1), so a folded row says how much went with it. Undo
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
formatted line beginning with `- `, `* `, `+ `, `• `, `[ ] `, `[x] `, `[X] `,
`>` or a backslash, since a line starting `>` is a body line (see Body). After reading indentation, paste removes this protective
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

A delete replaces the register, so after `yy` and a `dd` on the way `p`
puts the deleted rows. `gp` and `gP` put the last copy instead, what `y`
last took, after or before the cursor's row, under every rule of `p` and
`P`. With nothing copied yet they say so. Until a delete follows a copy
the two registers hold the same rows.

`o` on an open parent adds its first child; otherwise it adds the next
sibling.

### Body

A row's body is the text that does not fit on its line: notes, a draft,
what was agreed. It is read where the row is opened, at the head of the
pane, so it has the pane's whole width and nothing beside it (rule 6).

**Drawn.** Plain text in the text colour, each line wrapped at the pane's
width as a row wraps, blank lines kept, a tab drawn to the next stop of
four. Nothing in it is coloured: no sigil, no address, no formatting. A
dimmed rule sets it apart from the rows below when there are any. In the
outline, lists and filters its row shows `≡` (see On the row).

**Folded.** Above the rows a body is folded to one line, its first line
with text and how many lines follow it, dimmed (`Plan the cutover +12`),
so it never pushes the rows off the screen. Inside the body `zo` keeps it
open above the rows, whole and wrapped, `zc` folds it again and `za` turns
it over; the status line says which, and it shows once the caret leaves.
Which bodies stay open is kept on this machine beside the folds, and is not
logged. An open body taller than the room above the cursor's row gives up
its first lines, so the cursor is always seen.

**Focused.** While the caret is in the body, folded or open, it takes the
whole pane: the rows wait below it, and it scrolls to keep the caret's line
in view.

**Reached.** `k` on the first row of the pane, or in a pane with no rows,
steps into the body, on its last line, and the status line says `in the
body`. An empty body takes no lines; `k` still reaches it, drawn as one
dimmed line, `no body · A writes one`, while the caret is there. The cursor
never lands in a body by itself: a place opens on its rows. `j` from the
body's last line, and `Esc`, return
to the first row. A selection refuses the step and says so. Nothing on screen shows
the way in, so while the cursor is on the first row of a pane whose row
has a body, with nothing pending or selected, the status line says `k
edits its body`. A pane with no body says nothing there: a hint on every
first row would soon go unread. `:help` and the tutorial's zoom step,
whose row has a body, teach the key.

**Edited.** Inside the body the keys are the in-row grammar, over lines as
well as characters, and none of them is a key of the outline:

- **Motions.** Every motion of a row; `j` `k` by drawn line, staying in the
  body; `}` `{` to the next and previous blank line, `)` `(` to the next and
  previous sentence; `gg` and `G` to the first and last line.
- **Operators.** `d`, `c` and `y` take those motions and the row's objects,
  and `ip` and `ap` for a paragraph, without or with the blank lines after
  it. Doubled, as `dd`, `cc` and `yy`, they take whole lines here, since
  the body has no rows to give them to; `gu`, `gU` and `g~` take lines
  doubled and with `j` `k` as `d` does.
- **Acts and typing.** Those of a row, by the caret's line where a row
  goes to its end: `0` `^` `$`, `I` `A`, `D` `C`, `x` and `X` keep to the
  line. `o` and `O` open a line below and above, `J` joins the next line on
  with one blank, and `p` and `P` put lines that `dd` or `yy` took below
  and above the caret's line. What a body gives a row joins into one line. While
  typing, `Enter` is a newline, `Tab` types a tab, and `Up` and `Down` stay
  in the body. A paste is typed text, every line and blank kept, with no
  bullets or checkboxes read: a body never makes rows.
- `u`, `Ctrl+R` and `.` work as in a row. Everything done between stepping
  in and leaving is one undo step; nothing is written when nothing changed.

**Found.** The filter reads the body (see Filter), and the row it found
there draws its `≡` as a match. Inside the body, `n` and `N` step through
the places the filter's words stand in it and stay in it; `*` filters
nothing there and says so. `:s/old/new` changes the row's line only.

**Carried.** The body travels with its row: a move, archive, delete, copy
and put take it, and undo brings it back. Copy, `:export` and `docket ls`
with no filter write each line of it under the row, one tab deeper, after `>` and a space,
or `>` alone for a blank line. Paste and `:import` read such lines as the
body of the row line above them, and a `>` line with no row above stays
text. `docket ls` with a filter keeps one line per row and leaves bodies
out, so a script reads what it read before. The body is one field, so when two machines change it, the later
change wins whole.

**Fast.** Only the lines on screen are wrapped and drawn, so a body of
5,000 lines keeps typing in it within rule 5's 16 ms.

## Sigils

A sigil is a whole word, so `R&D`, `a@b` and `wow!` stay prose. One the
grammar cannot read also stays prose (`@alice`, `#123`, `$5`). `\#tag` keeps a readable sigil literal.

| Sigil | Field | Accepts |
| --- | --- | --- |
| `!` | `flag` | The word `!` alone |
| `^` | `pin` | The word `^` alone |
| `#tag` | `tags` | A word starting with a letter; stored lowercase, once. `#name=value` carries a value, kept as typed: a name is held once, so a later value replaces the earlier where it sits |
| `@date` | `due` | `today`, `tomorrow`, a weekday (the next one, never today), `+3d`, `+2w`, `MM-DD` or a month named as the row shows it, `dec4` or `dec-4` (the next such day), `jan5-2027`, `YYYY-MM-DD`, and `eom`, the last day of this month |
| `@@date` | `start` | The same forms as `@date` |
| `*interval` | `repeat` | A count and `d`, `w`, `m` or `y` (`*3d`, `*2w`), or `daily`, `weekly`, `monthly`, `yearly` |
| `$state` | `state` | `open`, `doing`, `waiting`, `done`, `canceled`, `parked`, `note`, or an unambiguous prefix (`$w`, `$c`, `$p`; `$d` is not one). `$next`, `$first`, `$today`, `$missed` and `$project` are read by the filter only: they are not states |
| `##context` | `context` | A tag's name, without a value: `##home`, `##phone`. A row has one; a later one replaces it. `##` alone and `##1` stay prose |
| `~colour` | `color` | A colour name: `red`, `orange`, `yellow`, `lime`, `green`, `teal`, `cyan`, `blue`, `indigo`, `purple`, `magenta`, `pink`, `brown`, `gray` (also `grey`, `violet`); or a hex value, `#rgb` or `#rrggbb`, stored as `#rrggbb`. Read on any row, drawn on projects only. The filter reads `~` as a time instead |
| `%name` | | On the capture line only: the project or heading to file the row in |

`#est` values are numbers with `m`, `h` or `d` (a day is 8 hours); a bare
number means hours. Unreadable or negative values add nothing. The header
sums estimates only for tasks left to do, writing hours and minutes, never
days, rounded to the nearest minute after adding.

Typed in a row, sigils become fields when the edit ends and may sit anywhere
in the line; a repeated `@` or `$` takes the last. `A` opens a row showing its
canonical line: text, `!`, `^`, `@YYYY-MM-DD`, tags, context, state. Deleting a sigil from
that line clears its field. Typed on a row outside an edit, they open the
stamp line.

**While typing.** On every line that reads sigils (a row, the capture line,
the stamp line, the filter) a word the line reads as a sigil is drawn in the
accent colour, and a word that starts like one and cannot be read in the
warning colour, so a mistyped `@firday` is seen before `Enter`. A
`~colour` is drawn in the colour it names instead, so it is seen before
it is kept. At the end of
the line a sigil being typed is completed in dim text: a `#tag` or a
`##context` already in use, most used first; a `$state`; a date word; a colour name. The right arrow takes the
suggestion and types nothing otherwise. The capture line also names where
the row will land. The row being typed shows the glyph of what its line
makes it so far: `○` under a heading, `·` once its text ends in a colon,
`◔` when `$w` is typed, so what `Enter` keeps is seen first.

While a `#tag` is typed at the end of a row, the capture, stamp or filter
line, the status line names every tag in use that it could become, most
used first, each with how many rows carry it: `#home 42 · #house 3`. A tag
with values counts as its name. `#` alone lists them all, so the tags there
are can be read without a list of their own (rule 6); one that starts no
tag in use says `a new tag`. A `##context` names the contexts in use the
same way, `##phone 4 · ##town 2`, and one that starts none says `a new
context`; on the filter `Tab` walks them as it walks tags. On the prompts the list follows the count of
matches or rows, in place of the hint about keys. On the filter a word
that names the one tag or context it could become lists nothing: the rows
carrying it, beside the count of matches, would read as a contradiction
(`1 match · #work 254`), so the line is read back in words instead. A `~colour` typed there,
except on the filter, lists the colour names it could become in the same
place, each drawn in itself, as `~teal`; a `~#` asks for a hex value.
Where the values are listed over the rows, as below, that list is the one
that names them and the status line does not repeat it: the row keeps its
hint about keys and a prompt its hint after the count. A word with nothing
listed, as one typed in full on the stamp line or one that starts no tag,
is still named on the line.

Every sigil with values to take lists them one above another as the filter
lists its values, from the moment its mark ends the line on a row, in the
pane or a sidebar entry, or on the capture or stamp line: under the word on
a row, or over it when there is more room above, and above the line on a
prompt, keeping to the pane so it never covers the sidebar's names.
`#` lists the tags in use, each in its colour, `##` the contexts, each
with how many rows carry it, dim and right-aligned beside its name, and a
tag with values one line for each value; `$` the states, `@` and `@@` the
date words, and `~` the colour names, each drawn in itself. On the stamp line `-#` and `-##` list what could be taken
off; a removal that takes no value lists nothing. `Tab`, `Down` and `Ctrl+N` write the next value, `Shift+Tab`,
`Up` and `Ctrl+P` the one before, and past either end the word is as it was
typed; the one written is reversed. While the list is shown those keys walk
it, so `Tab` does not indent and `Down` does not go on into the next row;
any other key ends the walk and keeps what is written. A value typed in
full, or a hex value, lists nothing.

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
children and a dimmed `·` otherwise, so its depth is read off the same column. A folded row
counts the rows it hides after its text (`+3`): a task, which keeps its
state glyph, and a note, unless it is a heading that says its progress. A row ending in a colon shows
dimmed `done/total` after its text when it has tasks beneath it, at any depth,
folded or not. Canceled and parked rows are left out of the total.
A row with a body shows a dimmed `≡` right after its text, before any count
or progress, in the outline, lists and filters alike; how long the body is
is not said.
The outline's shape is read at a glance: a row ending in a colon is drawn
bold in the accent colour, a top-level row bold, unless it is closed,
parked or not yet started, which dims it as any row. A heading with every
task under it done stays bold and is dimmed, so what is finished does not
ask to be read.
A project with a colour is drawn bold in it, in its row, its sidebar entry,
the header and the project named after a row in a flat view; the colour is
not written out after the text, since the colour says it. Closed, parked
or not yet started, it is dimmed as any row.
Fields follow the text, each in
the sigil that writes it, and then `→` with the row a link points at. Each
`#tag` is drawn in its own colour; a closed or parked row dims its tags
with the rest. A context leads the text in brackets, `[phone] Call Ann`,
so where the work is done is read first, in the colour its name hashes
to as a tag's does; a filter that asks for it marks it as it marks a tag. A pinned row
shows `^` in the accent colour after its text and flag, so the row says
which way `^` will go. A row
picked for Today shows `★` in the accent colour after its text and flag,
wherever it is drawn but in Today: `t` toggles the pick, so the row says
which way the key will go. The deadline reads `@today`, `@tomorrow`, `@fri`,
`@Nov 20`. A deadline or start day outside the current year names the year,
as `@Jan 5 2027` or `@@Jan 5 2027`, even across a nearby year boundary.
`A` still opens the canonical `@2027-01-05` or `@@2027-01-05` for editing.
An overdue deadline takes the alert colour and bold, and one within a week
the warning colour, as the sidebar marks the row's project, so the row
behind a mark is found; a later one is dim. They hold until the row is
done or canceled, which dims the whole row. The flag `!` is bold in the
warning colour: the alert colour is kept for an overdue deadline and what
was not written, so a list of flagged rows raises no alarm. A word starting `http://` or
`https://` is written as a terminal hyperlink, without the punctuation a
sentence leaves after it; every piece of an address broken across lines
carries the whole of it, so it opens wherever it is clicked. `gl` opens the
first address from the keyboard when the row has no row link.

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
Every done row under it, at any depth, reopens with `changed` stamped in
the same undo step. Canceled, parked, waiting, doing and open rows and notes
stay as they are, and no date under it changes. One `u` restores the parent's
dates and every reopened row; `.` repeats the close.

## Outside the app

`docket add LINE` captures without opening the outline. The line is read as
the capture line reads it: sigils become fields and a `%name` word files it
in a project or heading, else it goes to the Inbox. With no words, each line
of standard input is one row. It prints where each row went, and exits
non-zero with nothing added when a line holds nothing to add. It is one
short-lived window on the database, under the rules of Two windows: an open
window sees the row at its next key, and a sync pass runs before it ends.

`docket ls FILTER` prints the rows a filter matches, read as `/` reads it
across every project and the Inbox, so the outline answers a script, a
status bar or a pipe. Each row is one line: the row in the row grammar, a
task always spelling its state since no outline is there to read a default
from, then a tab and where the row lives (`Alpha › TUI`). The Inbox's rows
come first, and the order is a flat filter's: the outline's, or soonest
deadline first when the filter asks about deadlines, closed rows last. `&.` has no cursor to
read and matches nothing. With no filter it prints the whole outline as
`:export` writes it, tab-indented, which `:import` and paste read back. It
changes nothing, prints nothing when nothing matches, and does not sync.
For example, `docket ls '"per the spec"'` finds a phrase, and
`docket ls '$first -<$'` prints the next action under each heading and project.

`docket --version` prints `docket version V (commit: C, built: T)`, the
parentheses left out of a plain `go build`.

These are the only commands; everything else is done in the outline.

### Docket.app

`macos/` builds Docket.app: its own window, Dock and menu-bar icons, and a
global shortcut that shows or hides it. It runs the ordinary `docket` on a
pty inside an embedded Ghostty terminal and changes nothing in the outline
by itself; [`macos/README.md`](macos/README.md) is its contract. Quick capture
is `docket add`, and the overdue count it shows is `docket ls '@overdue'`.
A `docket://capture` link, the Services and Share menus, and `⌘K` in quick
capture (the browser tab, Mail messages or Finder selection in front) only
fill quick capture's line: Enter still sends it, so nothing from outside adds
a row unseen.

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
   `here`, where `o` would open a row. *Built.*

10. **Repeat and reach.** `h` in a list or flat filter shows the cursor's
   row where it lives; `Esc` returns to the held list or filter on that row.
   A jump leaves a place to come back to, with its zoom and cursor, and a
   sidebar visit counts as one jump; since milestone 13 `Ctrl+O` goes back
   and a blank `f` `Enter` opens the Inbox. Filters read bare `#`
   and `$` as any tag and any state, and alternatives may mix sigils
   (`#home|!`, `$waiting|@overdue`, `milk|#food`). `^term` tests ancestors,
   including above the pane root; `-^term` excludes such ancestry.
   `:sort [keys]` stably orders siblings or a selected block by deadline,
   flag, state, or text, within its span and in one undo step.
   `.` repeats the last stamp or named move; `:help` (or `:h`) opens help.
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

13. **Modal.** `w` `b` `e` step into a row's text, where motions, the
   operators `d` `c` `y` with text objects, and acts on characters edit it
   in one undo step. `>` and `<` are operators, doubled as `>>` and `<<`.
   `.` repeats every change to rows, typed text included, and has a
   counterpart inside a row. `f` reaches any row by its text. `Ctrl+O` and
   `Ctrl+I` walk the trail of jumps, replacing the picker's `back`. `L`
   links a row to another and `gl` follows it. `:numbers` shows each row's
   count from the cursor. Filters test the rows below with `_term`, and a
   tag carries a value (`#est=2h`, `#est<3`). *Built.*

14. **Recall.** `:import file` reads back what `:export` wrote. `@++1w` and
   `@--3d` on the stamp line move a day from where the row has it. The
   filter, stamp and command lines recall earlier lines with `Up` and
   `Down`, across restarts. `&` filters by link, and `&.` finds the rows
   that point at the cursor's row. `gp` and `gP` put the last copy after a
   delete has replaced the register. Operators take the row objects `ic`
   and `is`. A row reached from afar is drawn mid-pane. *Built.*

15. **Flow.** Typing opened inside a row ends inside it, so a row takes
   several changes on one visit; `v` selects there and `*` filters by the
   word under the caret. A filter groups terms with a bar and brackets
   standing alone, tests the parent with `<term` and the children with
   `>term`, and reads `$today`. `zc` and `zo` fold and unfold a whole
   branch; `zt` `zz` `zb` place the cursor's row in the pane.
   `:s/old/new` changes text across rows, and `.` repeats it. `docket ls`
   answers a filter from a shell. *Built.*

16. **Arrival.** Closing a repeating row reopens every done row under it,
   at any depth, preserving their dates and undoing together. `gl` opens
   the first web address on a row with no row link. The header counts tasks
   left to do and sums their `#est` values; headings show done/total across
   all descendants. Filters read quoted phrases and `$first`, the first
   workable sibling under each parent. Quoted filter phrases stay plain while
   typing, without sigil colours or completions; empty phrases are passed over.
   Dark and light palettes follow the terminal background; cursor and
   selection backgrounds span wrapped rows,
   and dates outside this year name their year. An empty interactive launch
   opens a Welcome tutorial; `:tutor` adds a fresh copy in one undo step.
   Tutorial repeat and sweep steps include sibling tasks to act on, and
   its join and sidebar instructions describe the keys' actual outcomes.
   Help has eight sections walked with `]` and `[`. *Built.*

17. **Feedback.** `u` and `Ctrl+R` say what they took back. A filter draws
   what it found on its matches. A waiting prefix or operator says what may
   follow it. Headings and projects are drawn with weight. Dates are typed
   as they read (`@dec4`, `@jan5-2027`, `@eom`). Typing a `#tag` names
   the tags in use with their counts. A new outline comes with `/next`,
   `/week` and `/done`. `g;` and `g,` walk back through the rows changed
   this session. A Vim key the outline does not bind names what does, and
   every refusal names the way that works. *Built.*

18. **Seam.** `gn` and `gN` step into a row's text on what the filter
   found, and `n` and `N` inside a row go on to the next place, row to
   row. `j` and `k` inside a row go on into the row below and above. The
   status line reads a filter back in words as it is typed. A filter with
   a `~` term says when each match changed and reads newest first when
   flat. Saved filters follow the four lists in the sidebar. `:import`
   reads TaskPaper, OPML and Bike files. *Built.*

19. **Return.** A row that others link to shows `←` and the way back, and
   `gl` takes it when the row has no link or address of its own. A pick
   left undone stays in Today, saying since when, until it is closed or
   unpicked, below a rule that sets the day's own rows apart, and
   `$missed` finds those picks. Tags are told apart by colour, and `~colour` gives a
   project its own, by name or hex. *Built.*

20. **Context.** `##context` gives a row one context, drawn in brackets
   before its text in the colour its name hashes to. Typed, it is
   completed and listed as a tag is; the filter reads `##name`, `##` and
   `-##`, and the stamp line sets and takes it off. *Built.*

21. **Pins.** `^` pins a row, or a project, so it is drawn first among its
   siblings, and unpinning puts it back where it was. `J` and `K` keep to
   their group, `:sort` sorts each group, new rows take the pin of where
   they open, and the filter reads `$pinned`. *Built.*

22. **Home.** `gf` goes from any row to the whole project it is filed in,
   or the Inbox, with the cursor on the row, the rows above it unfolded and
   the sidebar on the project. It is a jump, so `Ctrl+O` returns to Today
   or wherever it was pressed. *Built.*

23. **Body.** A row carries a `body` of plain text, marked `≡` after its
   text and drawn at the head of the pane that opens it, folded to its
   first line unless `zo` opened it. `k` from the first row steps into it,
   and it takes the whole pane, where the in-row grammar edits lines, with
   paragraphs, sentences, line operators and `o` `O` `J`; `Enter` there is
   a newline and nothing in it is read as a sigil. The filter reads
   bodies, and `=` holds a word to the row's own line. Copy, export,
   paste, `:import` and `docket ls` carry the body as `>` lines under its
   row. *Built.*

24. **Reach.** `zj` and `zk` go to the next and previous row with rows
   under it, and `zm` and `zr` fold and open a level at a time. `gv`
   selects again the rows last cleared, `o` swaps a sweep's ends, and `gi`
   types again where typing ended, and `Ctrl+A` and `Ctrl+X` put a
   row's day off or bring it forward. Inside a row `ge` and `gE` go back to a
   word's end, `%` to the paired bracket, `gu` `gU` `g~` change case
   through a motion, and `Ctrl+A` and `Ctrl+X` add to a number. *Built.*

25. **Fields in reach.** Inside a row `i@` `i#` `i$` `i~` `i*` and their
   `a` forms name a field, so `ci@` retypes a date and `da#` drops a tag
   from anywhere in the line, and `o` swaps the ends of a selection there.
   `Ctrl+E` and `Ctrl+Y` scroll the pane under the cursor. `:numbers`
   draws the cursor's own place, which a count before `G` or `gg` goes to.
   *Built.*

26. **Settle.** Done and canceled rows sink below the open ones in Today
   and in a flat filter, read when the view is opened, so nothing moves
   under the cursor. Outlines keep their order. *Built.*

Not planned: reminders, attachments, a details panel (a body is read in the
pane, not beside it), import from v1, any
command-line interface beyond `docket add`, `docket ls` and `--version`, macros (`.`,
`:all` and `:s` repeat what a recording would), marks (the trail, the change list, links and each
place's remembered cursor reach the same rows), inline formatting.

## Tests

Behaviour is tested by pressing keys against the model and comparing the
drawn rows, in `internal/tui/model_test.go`. A new behaviour arrives with a
scenario; a test that reaches into the model's fields is the exception.
