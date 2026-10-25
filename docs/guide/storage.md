# Storage, undo, and migration

The durable layout on disk, the one undo ledger every surface shares, conflict
recovery, and the schema chain a startup migrates through.

## Start with the recovery action

| Goal | Action | What to expect |
| --- | --- | --- |
| Reverse an accidental change | `u`; `Ctrl+R` to redo | One whole transaction, including its related files |
| Inspect workspace health | `docket doctor` | Read-only diagnostics |
| Inspect a conflict | `docket conflict list`, then `docket conflict show <id>` | Review both versions before choosing one |
| Resolve a conflict | `docket conflict resolve <id> --use local` (or `remote`) | Keep the version you chose |
| Locate or move data | `docket workspace location`; `docket workspace move <destination>` | Inspect the current location before moving |
| Set up device exchange | [Exchange-folder instructions](#syncing-through-an-exchange-folder) | Each device retains its own working copy |

For example, after an accidental bulk move, press `u` once to reverse the whole
move. If undo refuses because a file changed outside that transaction, read the
named file and entry before trying the previous history entry. Before upgrading,
run `docket workspace backup DIR` and keep the verified snapshot: migrations are one-way.


## Undo, recovery, and conflicts

`u` and `Ctrl+R` use one workspace-wide ledger capped at 50 committed
transactions. Task, project, note, and attachment changes share that history.
A new forward mutation clears redo; a failed operation creates no entry. The
Outliner, Select Project, Notes, and the Comments and Attachments panels all
answer both keys against that one ledger, and each refreshes what it is showing
rather than leaving a reversed row on screen. No surface offers the `u undo` a
recovery toast advertises and then consumes the key in silence.

An undo or redo is refused when a file it would restore was changed by something
the ledger did not record — a change synced from another device, or an edit made
outside Docket. Nothing is written, and that entry is dropped from history, since
the file can no longer return to the state the entry expects. The message names
the entry and the file; press the key again to reach the entry before it.

Workspace commits validate paths and revisions, stage a durable manifest, and
publish every affected artifact atomically. Startup rolls an interrupted
prepared transaction forward before loading the workspace. Concurrent edits
that cannot be merged produce durable conflict bundles:

```bash
docket conflict list
docket conflict show <id>
docket conflict resolve <id> --use local
docket conflict resolve <id> --use remote
docket conflict resolve <id> --use file --file merged.md
```

Startup can also meet a transaction that is not interrupted work but another
machine's live commit, arriving mid-flight from a sync service. Each transaction
records the machine driving it and keeps a heartbeat while it runs, so this
machine recognizes one that is not its own, leaves it entirely alone, and opens
the workspace normally. Nothing is rolled back and nothing is blocked: the
machine that started it is the one that finishes it. Each transaction skipped
that way is written to the log with its owner and heartbeat, so an edit that
exists on the other machine and has not landed here yet has a recorded reason;
`docket doctor` names the same transactions while they are still live. If that machine never does —
it crashed, or you stopped syncing it — the heartbeat goes stale within ten
minutes and the transaction becomes ordinary interrupted work the next launch
recovers. That window is the only clock difference between machines Docket
tolerates, which is why the runbook below asks you not to run Docket on two
machines at the same time rather than describing it as a supported way to work.

A recorded bundle no longer waits to be discovered. While the TUI is open, the
status strip above the footer carries a state indicator counting the workspace's
unresolved merge conflicts and naming the command that reviews them. It is
recounted at launch and once a minute, so a conflict another machine recorded
while you were working appears on its own, and the line disappears once the last
bundle is resolved. `gc` opens the conflict browser in both the TUI and desktop
app. `Enter` inspects a conflict and `d` toggles Git-style unified diffs, with
context lines, hunk ranges, red removals, and green additions. Project bundles
show recorded-remote-to-local changes; `b` switches to comparisons against the
base. Press `l` to keep local,
`r` to keep the current remote state, or `e` to edit a merged result in the app.
Retry-only bundles offer `r`, followed by retrying the original operation.

For sync conflicts, `[` and `]` select a competing version; `d` compares it with
the current workspace file, initially selecting a competing version. `b` toggles
comparison against the base. When there is no common base, that comparison
shows the whole file as additions; the current-file comparison shows the actual
differences between versions. Press `v` to keep that version or `c` to keep the current workspace
content. The confirmation lists any other conflicted files that choosing the
version will also resolve. Text conflicts support `e` to compose a merge starting
from the selected version; binary or unavailable content is identified explicitly.

In the merge editor, arrows, Home/End, Page Up/Down, Backspace/Delete, and paste
edit the full text. `Ctrl+S` previews the result as a diff, then `Enter` confirms
and saves. `Esc` cancels confirmation or returns from the editor while retaining
the draft; `e` resumes it. Leaving an edited conflict asks before discarding the
draft. Validation errors and stale conflicts keep the draft available. Every
resolution checks for newer edits before committing and refreshes the conflict
count and workspace views. See [`outliner-chrome.md`](outliner-chrome.md).
The count can be hidden with `docket --no-advisories` or `View → Advisories`, which
changes nothing about the bundles themselves.

A bundle stores its three documents as the markdown of the schema that was live
the day it was recorded, and migrations never rewrite them. So a bundle that
outlived a schema bump holds documents this version cannot read. It stays
resolvable: `docket conflict list` and `docket conflict show` still describe it — `show`
names each document it could not read and why — `--use remote` still adopts what
is on disk and clears the record, and `--use file` still commits a merge you
made by hand from the files in the bundle directory. Only `--use local` refuses,
because the document it would commit is the unreadable one, and it says so by
naming the schema that wrote it.


## Storage layout and migration

The live portable layout is deliberately small:

```text
<data>/
├── .docket/
│   ├── workspace.json
│   ├── planning.json
│   ├── lenses.json
│   ├── global-notes/
│   ├── transactions/
│   ├── undo/
│   ├── conflicts/
│   └── backups/
├── .tt/                  tombstone: workspace.json, planning.json, transactions/commit.lock
├── exports/
└── projects/<slug>/
    ├── tasks.md
    ├── notes/
    └── .attachments/
```

`exports/` is where `docket project export` writes `<slug>.md` by default, a
sibling of `projects/`. Nothing in it is read back: an export is a document for
you, so deleting one costs nothing but the file. Logs are not in the workspace
at all. They are machine-local, under `~/.docket/logs` as timestamped `.log`
files, and any older than 30 days are removed the next time a command logs.
`docket workspace location` prints the data directory, its `workspace.json`
catalog, whether the location is local or synced, the machine-local
active-workspace pointer, and the logs, `backups/`, `exports/`, and
`conflicts/` paths.

Releases named `tt` kept the machine-local root at `~/.tt` and an iCloud Drive
workspace in `iCloud Drive/tt`. The first Docket command that logs, while
`~/.docket` does not exist yet, renames `~/.tt` to `~/.docket` in one step —
runtime state, the active-workspace pointer, logs, and the default workspace
when `~/.tt` held one — and an `iCloud Drive/tt` workspace to
`iCloud Drive/Docket`, rewriting the pointer when it named either. A pointer
naming any other workspace is left as it is. If `iCloud Drive/Docket` already
exists beside a `tt` workspace, or a rename fails, nothing moves and the
command stops, naming both folders; once `~/.docket` exists it never runs
again.

Schema 24 moves the workspace's internal directory from `.tt/` to `.docket/`
and changes nothing else: no task document, catalog field, or conflict bundle
is rewritten, and task Markdown stays at schema 20. Opening a pre-v24 workspace
first publishes `pre-docket-layout-v24-<UTC>.tar.gz` and its audit under
`.docket/backups/`, then builds `.docket/` beside it from the catalog, planning,
lenses, global notes, conflict bundles, sync history, and backups, byte for
byte, checks every copied byte, and renames it into place. That rename is the
point of no return: a crash before it leaves `.tt/` untouched and the next
opening starts again, a crash after it is finished forward. The schema chain
then runs inside `.docket/`, and the catalog's schema number advances to 24
last. Undo history does not move, because its entries name the `.tt/` paths they
restore. `.tt/` is then replaced by a tombstone, a catalog declaring schema 24
beside a planning note and the commit lock, so a version that still reads `.tt/`
refuses the workspace as newer than it supports instead of finding nothing and
creating an empty workspace over synced data. Every workspace created at schema
24 gets one too. Do not delete it. A `.tt/` tombstone with no `.docket/` beside
it means another machine moved the workspace and the move has not synced here
yet: Docket refuses to open it, by name, rather than creating anything. A live
`.tt/` found beside `.docket/` that holds bytes `.docket/` does not is kept,
whole, under `.docket/backups/kept-pre-v24-internal-<UTC>/`.

Project task files use strict schema-20 Markdown and generic `Task` headings.
`workspace.json` contains the lean project catalog, project tags, and the
protected Unfiled ID. `planning.json` contains Today and
Next ordering only.
Each task may carry an optional non-negative `defer_count`; zero is omitted.
Each task may also carry an optional `due_time`, a canonical `HH:MM` wall-clock
in 00:00–23:59 that pairs with the calendar `due` date; midnight is reserved for
date-only due dates, so a date-only task omits it.
Each task may carry an optional `review` flag; false is the flag's absence, so
an unflagged task omits it.
Each task may carry an optional `hidden: true` flag the same way. A hidden task
keeps its state, planning, pin, review flag, check-in, and deadline; it is only
left out of what you see. False is the flag's absence, so a task that is not
hidden writes exactly the bytes schema 19 wrote. Only the task you hid stores
it: its children are hidden through it and keep no flag of their own. See
[hidden tasks](search-and-lenses.md#hidden-tasks).
Each task may finally carry an optional `depends` list of soft references to
other tasks, each a `project-id:task-id` pair; an empty list is omitted. A
reference is durable and one-directional: only the referring task stores it, so
the dependents you see are computed at read time and never written down.

A task may also carry a check-in: an optional `check_in` calendar date saying
when you next mean to look at it, an optional `check_in_repeat` cadence in whole
days saying whether answering it moves the date forward or clears it, and an
optional `reviewed` instant recording when it was last answered. A date with no
repeat is a one-off, a repeat with no date is refused because a cadence needs an
occurrence to count from, and each key is omitted when it is unset. The date has
no time of day, so a check-in arrives when its day arrives wherever you read it.
Finishing or canceling a task drops the date and the cadence, because terminal
work owes nothing; the `reviewed` instant survives, because it records what
happened rather than what is owed.

A task may also hold one resume note, a single line of at most 240 characters
kept in its own `RESUME` block above the description. It is the note you leave
yourself about where you stopped, so a task without one writes no block at all
and an empty note and an absent note are the same thing. `r` on the row is where
you write it and Peek is where you read it back; it is yours to clear, so
pausing, starting, or finishing a task never prompts for one and never throws
one away. `/` searches it beside titles and descriptions, and `project export`
prints it as a labelled `Resume:` paragraph above the description.

A project file may also hold comments that belong to the project itself rather
than to any one task. They live in their own block above the first task, so
reading the file top to bottom gives you the project's own notes before its
work. Each one carries its own identity and the moment it was written, and the
file always lists them oldest first, so the same set of comments looks the same
on every machine that opens the workspace. A project with no comments writes no
block at all. `C` on a project group header in the Outliner is where you add
one.

`lenses.json` contains your saved named-lens definitions — the three seeded
starters, anything you saved yourself, and anything you added from the built-in
gallery, which are all one list. It travels with the workspace, so a lens you
save on one machine is a lens every machine that opens that workspace has. An
empty lens list persists as `[]` and is honored, which is what makes deletion
durable, on every machine rather than on the one you deleted from. Each record
names its mode, its project scope, its sort, and one query holding every facet
it narrows by, so what a lens selects is one line you can read.

Machine-local `~/.docket/runtime/<workspace-id>/machine.json` contains only the
dark/light theme choice, whether the Outliner draws its menu bar, whether it
draws ambient advisories, a pointer to the lens that was last open, and the last
50 submitted search queries for this workspace. These
are answers about this machine rather than about the workspace, which is why they
stay behind while the definitions travel: a second machine opens on its own
theme, with no menu bar, showing advisories, on no lens at all, and with its own
search history. History saves and clears create no task undo entry. A machine
that has never pressed `M` writes no menu-bar key at all, and the absent key is
what says the bar is hidden; the advisory key works the same way, so its absence
says advisories are shown. The active-lens pointer is a
hint rather than a record: it is checked for shape alone when the file loads, so
one naming a lens that is no longer there costs you a stale hint and never a
failed launch.

The first time a machine opens a workspace that has reached schema 16, it joins
the shared list: whatever lenses that machine had saved for itself are merged
in, and nothing is dropped. A lens only one side had survives, and one name held
by two different definitions yields both — as `Name` and `Name (2)` — because
the two lists carry no clock between them and neither can be called the newer.
Which machine joined first does not change the answer: the merge reads only the
definitions themselves, so three machines joining in any order reach the same
list, with the same names on the same lenses. The merge runs once per machine, so
a duplicate you tidy up afterwards stays tidied.

After that, a lens write checks the shared list against the version your session
opened. When another machine has written it since — the usual cause is a sync
service delivering that machine's copy while yours was open — the two lists are
merged by the same rule adoption uses, and the merged list is what is saved. You
keep your change and the other machine keeps theirs; neither write is lost and
neither is refused. The one exception is a list hand-built to collide: if a
single lens id has been made to carry more different definitions than the merge
has identities to hand out, the merge refuses the whole list rather than saving
it one definition short. Nothing on disk changes, and no list a Docket session
wrote can reach that shape. Merging the same pair of lists again adds nothing, so a sync
service delivering the same copy twice cannot leave you with duplicates. If the
merge has to give one of your lenses a new identity — that happens only when two
different definitions arrive under one — the lens you had open follows it, so the
pointer never quietly lands on somebody else's lens. The cost is that a merge
cannot tell a lens you deleted from
a lens the other machine added: if you delete one while another machine writes
the same file, the deletion does not take and the lens comes back. Delete it
again once both machines have seen the same list and it stays gone. The other
two synced files, `workspace.json` and `planning.json`, do not merge — a change
that lands under one of those is refused and reported, because there is no
order-independent way to combine two project catalogs.

Startup migrates schema-23 workspaces to schema 24, supported schema-21
workspaces, and working copies enrolled in sync at schema 22, to schema 23,
supported
schema-20 workspaces to schema 21, supported
schema-19 workspaces to schema 20, supported
schema-18 workspaces to schema 19, supported
schema-17 workspaces to schema 18, supported
schema-16 workspaces to schema 17, supported
schema-15 workspaces to schema 16, supported
schema-14 workspaces to schema 15, supported
schema-13 workspaces to schema 14, supported schema-12 workspaces to schema 13,
supported schema-6 through schema-11 workspaces through the intermediate
boundaries, and supported
schema-4/5 workspaces through the whole chain. Each step validates the complete
candidate before mutation and publishes its own immutable rollback archive and
audit first: `pre-simplification-v5-<UTC>.tar.gz` for v4→v5,
`pre-consolidation-v6-<UTC>.tar.gz` for v5→v6, `pre-scope-v7-<UTC>.tar.gz`
for v6→v7, `pre-tag-v8-<UTC>.tar.gz` for v7→v8,
`pre-state-clock-v9-<UTC>.tar.gz` for v8→v9,
`pre-defer-rename-v10-<UTC>.tar.gz` for v9→v10,
`pre-defer-instant-v11-<UTC>.tar.gz` for v10→v11,
`pre-context-v12-<UTC>.tar.gz` for v11→v12,
`pre-defer-count-v13-<UTC>.tar.gz` for v12→v13,
`pre-due-time-v14-<UTC>.tar.gz` for v13→v14,
`pre-dependencies-v15-<UTC>.tar.gz` for v14→v15,
`pre-portable-lenses-v16-<UTC>.tar.gz` for v15→v16,
`pre-stored-review-v17-<UTC>.tar.gz` for v16→v17,
`pre-resume-note-v18-<UTC>.tar.gz` for v17→v18,
`pre-project-comments-v19-<UTC>.tar.gz` for v18→v19,
`pre-checklist-origin-v20-<UTC>.tar.gz` for v19→v20,
`pre-check-ins-v21-<UTC>.tar.gz` for v20→v21,
`pre-hidden-v23-<UTC>.tar.gz` for v21→v23 and v22→v23, and
`pre-docket-layout-v24-<UTC>.tar.gz` for v23→v24.

The two structural conversions state themselves. The schema-6 conversion
preserves notes and attachment bytes, converts retained decision text into
comments, repairs planning references, renames the protected project to Unfiled,
and deliberately resets the incompatible pre-v6 undo ledger. The schema-7
conversion folds the retired per-project grouping fields and the retired
workspace scope table into ordinary project tags, keeping `parked` as the sole
behavioral project tag.

Every boundary from schema 8 on follows one rule: the step rewrites task
Markdown into the previous schema's shape, existing tasks keep their meaning
under the new optional field, and undo/redo history is cleared because its
retained Markdown carries a schema marker the new reader rejects. The table
records what each boundary adds and whatever it does that the rule does not
already say.

| Boundary | Beyond the rule |
| --- | --- |
| Schema 8 upgrades task Markdown to schema 7 | Canonicalizes every task and project tag to lowercase ASCII kebab form and audits renames and collisions; the cleared history is the bytes predating that canonicalization |
| Schema 9 upgrades task Markdown to schema 8 by giving every task a durable `state_changed` clock, backfilled from `updated` | The audit states that the backfill is an approximation which self-corrects on the task's next state transition |
| Schema 10 upgrades task Markdown to schema 9 by renaming the durable deferral metadata key from `snoozed` to `deferred` | Nothing |
| Schema 11 upgrades task Markdown to schema 10 by rewriting each deferral value from a calendar date to an RFC3339Nano instant | This is what lets a deferral expire later the same day. A bare date always meant the moment that day began where it was read, so each one lifts to exactly that instant and every existing deferral wakes when it always would have; the due date stays a calendar date |
| Schema 12 upgrades task Markdown to schema 11 by adding the optional, single-valued task-level `context` field | Existing tasks receive an empty value. The field is unrelated to the retired v4 project-level context that schema 7 folded into project tags |
| Schema 13 upgrades task Markdown to schema 12 by adding the optional `defer_count` field | Zero is the field's absence, so every existing task begins there |
| Schema 14 upgrades task Markdown to schema 13 by adding the optional `due_time` field | A canonical `HH:MM` wall-clock giving a due date an explicit time-of-day. Existing due dates are calendar-only, so every migrated task keeps its date and receives no `due_time` |
| Schema 15 upgrades task Markdown to schema 14 by adding the optional `depends` list of soft task references | No retired field carried dependency data into this boundary, so every migrated task starts with an empty list — an omitted key |
| Schema 16 leaves task Markdown at schema 14 | It is the one boundary that upgrades no task grammar: it marks the workspace as the home of named lens definitions, so the catalog is the only file it rewrites. Every project document is still read and validated first, so an unreadable workspace is refused rather than half-migrated |
| Schema 17 upgrades task Markdown to schema 15 by moving review off the tag list and onto a stored `review` flag | A task carrying the `review` tag before the migration carries `review: true` after it, and that tag is removed as it is lifted, so the workspace never holds two answers to one question. Every other tag is preserved in place, and a task that was not up for review is written back unchanged, because the key is omitted when the flag is off. Your saved lenses are rewritten in the same step and by the same rule: each one becomes a single query, and a lens that narrowed by the `review` tag narrows by review itself, so it still selects the rows you saved it for |
| Schema 18 upgrades task Markdown to schema 16 by adding the optional `RESUME` block that holds one resume note | No existing task has a note to lift, so every document comes out byte-identical to its schema-15 source apart from the schema marker line. Your saved lenses are not rewritten: a resume note is prose on one task rather than a value a query narrows by, so every lens keeps selecting exactly the rows it selected before |
| Schema 19 upgrades task Markdown to schema 17 by adding the optional `PROJECT-COMMENTS` block that holds a project's own comments | No existing project has a comment to lift, so every document comes out byte-identical to its schema-16 source apart from the schema marker line, and every migrated project — active, parked, or archived — starts with none. Your saved lenses are not rewritten: a project comment is prose on a project rather than a value a task query narrows by, so every lens keeps selecting exactly the rows it selected before |
| Schema 20 upgrades task Markdown to schema 18 by adding the optional checklist-origin line that records where a promoted task came from | No existing task was promoted under a schema that recorded an origin, so every document comes out byte-identical to its schema-17 source apart from the schema marker line. Your saved lenses are not rewritten, because an origin is not something a query narrows by |
| Schema 21 upgrades task Markdown to schema 19 by adding the optional `check_in` date, its `check_in_repeat` cadence, and the `reviewed` instant | This is the one boundary since schema 17 that writes a value rather than only a marker. Every Waiting task that carries a due date is given a one-off check-in on that same date, because the deadline you set on work you are waiting for is the day you meant to look at it again; the date is copied exactly, so the deadline, its `due_time`, the timestamps, the body, and the comments are untouched. No cadence and no review instant are invented, so a seeded check-in fires once and the workspace never claims a task was reviewed on a day nobody reviewed it. Every catalog-owned project is migrated, parked and archived ones included, and a task that already holds a check-in is left alone, so running the step again changes nothing. Your saved lenses are not rewritten |
| Schema 23 upgrades task Markdown to schema 20 by adding the optional `hidden` flag | No existing task is hidden, so every document comes out byte-identical to its schema-19 source apart from the schema marker line. There is no schema-22 step: 22 was the catalog schema of a working copy enrolled in sync, and such a copy migrates to 23 like a schema-21 workspace and keeps its `sync` block, which from schema 23 alone marks enrollment. Your saved lenses are not rewritten, because no lens could name the flag before this schema |
| Schema 24 leaves task Markdown at schema 20 and moves `.tt/` to `.docket/` | It changes the layout alone, described above. The undo history is dropped because its entries name `.tt/` paths, and `.tt/` keeps only the tombstone that makes an older version refuse the workspace |

Invalid or future-schema input aborts without changing workspace data. An
existing `unfiled` project or directory also aborts before snapshot publication
with rename/move guidance. Interrupted activation is recovered before ordinary
loading, and repeated startup is idempotent.

There is no down-migration. An older Docket refuses a migrated workspace as newer
than its supported schema, and no build converts a workspace back. Recovery is
manual: extract the matching archive — `pre-due-time-v14-<UTC>.tar.gz` for the
schema-14 boundary, `pre-dependencies-v15-<UTC>.tar.gz` for schema 15,
`pre-portable-lenses-v16-<UTC>.tar.gz` for schema 16,
`pre-stored-review-v17-<UTC>.tar.gz` for schema 17,
`pre-resume-note-v18-<UTC>.tar.gz` for schema 18,
`pre-project-comments-v19-<UTC>.tar.gz` for schema 19,
`pre-checklist-origin-v20-<UTC>.tar.gz` for schema 20,
`pre-check-ins-v21-<UTC>.tar.gz` for schema 21,
`pre-hidden-v23-<UTC>.tar.gz` for schema 23, or
`pre-docket-layout-v24-<UTC>.tar.gz` for schema 24, whose `workspace/.tt/` is
restored over the tombstone — into a
separate directory and open that copy with a compatible build. Migration
snapshots themselves are never modified.

Because migration is one-way, keep your own copy of the workspace directory
before a version jump. The published archive is a migration record rather than a
backup you are meant to rely on, and a copy you made yourself is the only thing
that lets you go back to the build you were running.

Run `docket doctor` for read-only integrity checks or `docket doctor --fix` for
explicitly supported repairs. `--fix` commits each repair as a journaled
transaction that refuses if the file changed since doctor read it, then runs
every check again, so the verdict describes the workspace after fixing.
`docket doctor --json` prints one JSON report on stdout and nothing else. Doctor
exits 0 when every check passes, 2 when it ran and found issues, and 1 when it
could not run or a repair failed.

## Running one workspace on more than one machine

A Docket workspace is an ordinary directory, so putting it on iCloud Drive,
Dropbox, Syncthing, or any other file sync service works. What follows is what
the design assumes of that service. None of it can be verified in CI: a sync
client's eviction, conflict-copy, and versioning behavior lives in a daemon on a
real machine talking to a real account, and reproducing it would mean shipping a
test that depends on a third-party service being logged in. The two-machine
behavior that *can* be checked mechanically is checked, and this section is the
rest.

**Sync the whole data directory, and exclude nothing inside it.** Every path in
the layout above is live state, including the ones that look like scratch space:

| Directory | Excluding it costs you |
| --- | --- |
| `.docket/workspace.json`, `.docket/planning.json` | The project catalog and Today/Next ordering. Without them the other machine sees an empty or partial workspace |
| `.docket/lenses.json` | Your saved lenses on the machines that did not write them |
| `.docket/global-notes/`, `projects/<slug>/notes/` | Note bodies, silently. The catalog still names the collection |
| `projects/<slug>/tasks.md` | The tasks themselves |
| `projects/<slug>/.attachments/` | Attachment bytes, while the tasks keep pointing at them. A leading dot makes this the one most likely to be skipped by a rule you did not write for Docket |
| `.docket/transactions/` | A commit that was interrupted on the other machine can no longer be finished or cleaned up, which is the one exclusion that can leave a half-written file behind |
| `.docket/undo/` | Undo history, which is per-workspace rather than per-machine |
| `.docket/conflicts/` | Recorded conflict bundles, so the machine that has to resolve one may not be the machine that can see it |
| `.docket/backups/` | The migration archives, the record of what each schema step replaced. They are not a backup to rely on: keep your own copy of the workspace before a version jump |

**Do not run Docket on two machines at the same time.** The safeguards are real —
revision checks refuse a stale write, lens lists merge instead of overwriting,
and another machine's live commit is left for its owner to finish — but they are
there to survive an overlap, not to make one a way of working. Sync services
deliver files whenever they get to them, so two open sessions see each other's
writes at unpredictable moments; the ten-minute heartbeat window above
(`ForeignTransactionGrace` in `internal/transaction/recovery.go`) is the only
clock difference between machines the design accounts for. Quit on one
machine, let the sync settle, then open on the other.

**A `.conflicted copy` file is your sync client's, not Docket's.** When two
machines write `tasks.md` or `lenses.json` before either has seen the other's
version, the sync client may keep both, leaving something like
`tasks (Case Conflict).md`, `tasks.md.sync-conflict-20250831-…`, or
`lenses (conflicted copy).json` beside the original. Docket never writes those
names and never reads them. Its own conflict record is a bundle under
`.docket/conflicts/`, which is what `docket conflict list` shows and the status strip
counts. So:

- A file with a client-invented suffix is a copy you resolve yourself. Compare
  it with the original, keep the content you want under the original name, and
  delete the copy. `docket doctor` will not touch it.
- A bundle in `.docket/conflicts/` is resolved with `docket conflict resolve`.

Provider-specific notes:

- **iCloud Drive** may evict a file's contents while leaving its name in place,
  which reads to Docket as an unreadable file rather than a missing one. Turn off
  "Optimize Mac Storage" for the workspace, or open the directory in Finder and
  download it before launching. Attachments and notes are the usual victims,
  because they are the largest and the least recently touched.
- **Dropbox** creates conflicted copies as described above, and its Selective
  Sync is the exclusion trap the table warns about: a folder you excluded on one
  machine still exists on the service, so the workspace looks intact from the
  other machine and incomplete from this one.
- **Syncthing** keeps its own file versions under a `.stversions` directory. That
  is a Syncthing archive, not Docket state; restoring from it means copying a file
  back over the original by hand. Syncthing also does not synchronize clocks, and
  a machine whose clock is well behind the others will write timestamps that read
  as older than they are — which is what the state clock, the check-in
  schedule, and the explicit Stale filter are all measured against.

**After a sync problem, run `docket doctor`.** Its checks are ordered so the ones
that block anything else come first: saved lenses, the transaction journal and
anything sitting in quarantine, recorded conflict bundles, stale lock files left
behind by a session that never exited, and the migration archives. Then the
per-project checks, of which one is the expected cross-machine skew: Today and
Next membership disagreeing with a task's own state, because two machines can
write the planning file and the task file in either order. That one is fixable,
so `docket doctor --fix` repairs it.

A quarantined transaction is the one state that stops the workspace outright,
because a manifest Docket cannot safely finish or undo must not be worked around.
Mutations are refused until it is cleared, and there is no CLI command that
clears it — the recovery step is manual and deliberately so:

1. Quit Docket on every machine and let the sync settle.
2. Run `docket doctor`. It names each quarantined transaction, where its manifest
   sits and what state it stopped in, the machine that wrote it, whether the
   manifest is readable, and the recovery step itself — the same sentence a
   blocked startup prints, so the two never disagree.
3. Back up the whole data directory before touching anything.
4. Inspect the manifests under `.docket/transactions/quarantine/`. If a file one of
   them names has wrong contents, restore that file from `.docket/backups` first.
5. Move the quarantine directory aside — `.docket/transactions/quarantine.bak` is
   enough — which unblocks mutations. This abandons those transactions; the work
   they carried is lost, which is why the backup comes first.
6. Run `docket doctor` again. It should report no quarantine, and mutations work
   again.

If the same transaction quarantines repeatedly, the cause is usually a partial
sync: the manifest arrived and its staged bytes did not. Wait for the sync to
complete on the machine that wrote it before clearing anything.


## Syncing through an exchange folder

Putting the live workspace in a file sync service is the older way to share it,
and everything above describes its hazards. The newer way keeps every device's
workspace local and shares only immutable history through an *exchange
folder*, typically one in iCloud Drive. It is opt-in, and a workspace that is
never enrolled keeps working exactly as before.

```bash
docket sync enroll --exchange ~/Library/Mobile\ Documents/com~apple~CloudDocs/tt-exchange --working-copy ~/tt-synced
```

Enrolling does not convert your workspace. It keeps the current one untouched,
saves a verified snapshot of it under `~/.docket/enrollments/`, builds a new local
working copy at `--working-copy` — which must be outside iCloud Drive — and makes
that copy the workspace Docket opens from then on. The copy's whole content is
then published to the exchange folder as its first piece of history. If
enrollment is interrupted, run the same command again: it finishes what is left
and copies or publishes nothing twice.

An enrolled working copy uses the current catalog schema, 23, with a `sync`
block, so a Docket from before sync refuses to open it rather than write around
its history; update Docket on every machine that opens it. A copy enrolled by an
older Docket at schema 22 migrates to 23 on first open and stays enrolled. Another desktop joins with
`docket sync join --exchange DIR --working-copy DIR`, which builds its own local
copy from the published history alone.

An enrolled desktop workspace exchanges automatically while Docket is running,
including inside tt.app: at launch, when the terminal regains focus, and every
five seconds. Local saves trigger an exchange immediately; a save or focus event
while a pass is running queues one follow-up pass. The periodic poll also detects
changes another process already applied to the working copy, without requiring
a focus change. Incoming changes refresh the Outliner when no editor, picker,
selection, ranked search, or child view is active. Polling preserves any current
save or undo feedback.

On macOS, an exchange requests iCloud downloads for cloud-only files it finds,
including the workspace identity, incoming history, and its content. The request
runs in the background through the system Foundation API; it requires no extra
helper installation. It does not wait for delivery or treat a request as a
completed download. Files are verified and imported on a later pass, and download
request failures appear in sync status. Other platforms leave file delivery to
the shared-folder provider. Unavailable folders and files are retried on later
passes. Closing tt.app's window keeps its session syncing; quitting Docket stops automatic exchange.
Cloud delivery can take longer than the polling interval.

In the Outliner, press `gs` (or choose **Go → Sync Now**) to exchange immediately.
The footer shows progress, then how much was published and imported, any work
still waiting, and failures or conflicts that need attention. It also refreshes
the workspace from disk, waiting until any active selection or editor is closed.
A pass already in progress is reused. `?` lists the shortcut.

`docket sync exchange` runs the same exchange from the command line, publishing what
you committed and applying what other devices published. The Outliner's status
strip shows a `sync:` line while anything is waiting — work saved here and not yet published, work found and not yet
applied, or a change held until a device updates Docket — and counts open sync
conflicts beside the merge conflicts, which `gc` lists. A file iCloud has not
downloaded yet is reported as unavailable and fetched on a later exchange; a
missing file never deletes anything. `docket sync status` shows the same state in
full, and `docket sync doctor` checks the working copy's placement, the exchange
folder, the history, and which device needs an update.

When two devices change the same thing while apart, both versions are kept as a
sync conflict until you choose; nothing is decided by timestamp. `gc` shows each
one with diff and resolution controls, and `docket sync conflicts ID` prints it in full: the version both
sides started from, every competing version with the device and the kind of
change that wrote it, and what the workspace holds meanwhile. Resolve it in `gc`
as described above, or with
`docket sync resolve ID --version N`, which keeps that version's side of what
conflicted and everything else both devices changed; when the same change also
left conflicts in other files, their versions from that change are kept too, in
one resolution. `--current` keeps what the workspace holds, and `--file PATH`
writes replacement text for tasks or a note, checked the way Docket reads that
file. A resolution is a new change that reaches your other devices at the next
exchange, and the iPhone resolves the same conflicts the same way. If the
conflict changed after you read it — another device resolved it, or an edit
landed — the resolution is refused and writes nothing; read it again and
choose. `docket sync diagnostics --file FILE` writes what to share when sync
misbehaves: status, the last exchange and its error, the unpublished work, and
the conflicts' files, devices, and digests, without task or note content.

### Quick-capture retry receipts

The Mac panel keeps its editable draft and any pending submission together in
`~/Library/Application Support/io.github.sglavoie.tt/pending-capture.json`.
Each edit replaces this local recovery file atomically. Restoring it fills the
field only; capture still requires Enter or Ctrl+N. A successful reply clears
its submission and the matching draft in one write, preserving newer text typed
while the reply was delayed. This file is separate from the workspace and is
never exchanged with another device.

The Mac quick-capture panel gives each submitted draft a unique identity bound
to its workspace. The task transaction also writes an immutable receipt under
`.docket/control-captures/`. Tasks, planning changes, attachments, undo history, and
the receipt recover together after an interruption. A retry returns the original
transaction and titles; the same identity with different text is refused.

Receipts remain after undo, deletion, and undo-history pruning so a delayed
retry cannot recreate work you removed. They are internal working-copy metadata,
excluded from undo and exchange sync; keep them when backing up or relocating a
working copy. They contain the draft's hash, captured titles, and transaction ID.
