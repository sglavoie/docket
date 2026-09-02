# Storage, undo, and migration

The durable layout on disk, the one undo ledger every surface shares, conflict
recovery, and the schema chain a startup migrates through.

## Undo, recovery, and conflicts

`u` and `Ctrl+R` use one workspace-wide ledger capped at 50 committed
transactions. Task, project, note, and attachment changes share that history.
A new forward mutation clears redo; a failed operation creates no entry. The
Outliner, Select Project, Notes, and the Comments and Attachments panels all
answer both keys against that one ledger, and each refreshes what it is showing
rather than leaving a reversed row on screen. No surface offers the `u undo` a
recovery toast advertises and then consumes the key in silence.

Workspace commits validate paths and revisions, stage a durable manifest, and
publish every affected artifact atomically. Startup rolls an interrupted
prepared transaction forward before loading the workspace. Concurrent edits
that cannot be merged produce durable conflict bundles:

```bash
tt conflict list
tt conflict show <id>
tt conflict resolve <id> --use local
tt conflict resolve <id> --use remote
tt conflict resolve <id> --use file --file merged.md
```

Startup can also meet a transaction that is not interrupted work but another
machine's live commit, arriving mid-flight from a sync service. Each transaction
records the machine driving it and keeps a heartbeat while it runs, so this
machine recognizes one that is not its own, leaves it entirely alone, and opens
the workspace normally. Nothing is rolled back and nothing is blocked: the
machine that started it is the one that finishes it. Each transaction skipped
that way is written to the log with its owner and heartbeat, so an edit that
exists on the other machine and has not landed here yet has a recorded reason;
`tt doctor` names the same transactions while they are still live. If that machine never does —
it crashed, or you stopped syncing it — the heartbeat goes stale within ten
minutes and the transaction becomes ordinary interrupted work the next launch
recovers. That window is the only clock difference between machines `tt`
tolerates, which is why the runbook below asks you not to run `tt` on two
machines at the same time rather than describing it as a supported way to work.

A recorded bundle no longer waits to be discovered. While the TUI is open, the
status strip above the footer carries a state indicator counting the workspace's
unresolved merge conflicts and naming the command that reviews them. It is
recounted at launch and once a minute, so a conflict another machine recorded
while you were working appears on its own, and the line disappears once the last
bundle is resolved. `gc` opens a read-only overlay over the same store, listing
the recorded bundles and showing one in the form `tt conflict show` prints; see
[`outliner-chrome.md`](outliner-chrome.md). Resolving stays in the CLI. The count can be hidden
with `tt --no-advisories` or `View → Advisories`, which changes nothing about the
bundles themselves.

A bundle stores its three documents as the markdown of the schema that was live
the day it was recorded, and migrations never rewrite them. So a bundle that
outlived a schema bump holds documents this version cannot read. It stays
resolvable: `tt conflict list` and `tt conflict show` still describe it — `show`
names each document it could not read and why — `--use remote` still adopts what
is on disk and clears the record, and `--use file` still commits a merge you
made by hand from the files in the bundle directory. Only `--use local` refuses,
because the document it would commit is the unreadable one, and it says so by
naming the schema that wrote it.


## Schema 17 storage and migration

The live portable layout is deliberately small:

```text
<data>/
├── .tt/
│   ├── workspace.json
│   ├── planning.json
│   ├── lenses.json
│   ├── global-notes/
│   ├── transactions/
│   ├── undo/
│   ├── conflicts/
│   └── backups/
└── projects/<slug>/
    ├── tasks.md
    ├── notes/
    └── .attachments/
```

Project task files use strict schema-15 Markdown and generic `Task` headings.
`workspace.json` contains the lean project catalog, project tags, and the
protected Unfiled ID. `planning.json` contains Today and
Next ordering only.
Each task may carry an optional non-negative `defer_count`; zero is omitted.
Each task may also carry an optional `due_time`, a canonical `HH:MM` wall-clock
in 00:00–23:59 that pairs with the calendar `due` date; midnight is reserved for
date-only due dates, so a date-only task omits it.
Each task may carry an optional `review` flag; false is the flag's absence, so
an unflagged task omits it.
Each task may finally carry an optional `depends` list of soft references to
other tasks, each a `project-id:task-id` pair; an empty list is omitted. A
reference is durable and one-directional: only the referring task stores it, so
the dependents you see are computed at read time and never written down.

`lenses.json` contains your saved named-lens definitions — the three seeded
starters, anything you saved yourself, and anything you added from the built-in
gallery, which are all one list. It travels with the workspace, so a lens you
save on one machine is a lens every machine that opens that workspace has. An
empty lens list persists as `[]` and is honored, which is what makes deletion
durable, on every machine rather than on the one you deleted from. Each record
names its mode, its project scope, its sort, and one query holding every facet
it narrows by, so what a lens selects is one line you can read.

Machine-local `~/.tt/runtime/<workspace-id>/machine.json` contains only the
dark/light theme choice, whether the Outliner draws its menu bar, whether it
draws ambient advisories, and a pointer to the lens that was last open. All four
are answers about this machine rather than about the workspace, which is why they
stay behind while the definitions travel: a second machine opens on its own
theme, with no menu bar, showing advisories, and on no lens at all. A machine
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
it one definition short. Nothing on disk changes, and no list a `tt` session
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

Startup migrates supported schema-16 workspaces to schema 17, supported
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
`pre-dependencies-v15-<UTC>.tar.gz` for v14→v15, and
`pre-portable-lenses-v16-<UTC>.tar.gz` for v15→v16, and
`pre-stored-review-v17-<UTC>.tar.gz` for v16→v17.

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

Invalid or future-schema input aborts without changing workspace data. An
existing `unfiled` project or directory also aborts before snapshot publication
with rename/move guidance. Interrupted activation is recovered before ordinary
loading, and repeated startup is idempotent.

There is no down-migration. An older `tt` refuses a migrated workspace as newer
than its supported schema, and no build converts a workspace back. Recovery is
manual: extract the matching archive — `pre-due-time-v14-<UTC>.tar.gz` for the
schema-14 boundary, `pre-dependencies-v15-<UTC>.tar.gz` for schema 15,
`pre-portable-lenses-v16-<UTC>.tar.gz` for schema 16, or
`pre-stored-review-v17-<UTC>.tar.gz` for schema 17 — into a
separate directory and open that copy with a compatible build. Migration
snapshots themselves are never modified.

Because migration is one-way, keep your own copy of the workspace directory
before a version jump. The published archive is a migration record rather than a
backup you are meant to rely on, and a copy you made yourself is the only thing
that lets you go back to the build you were running.

Run `tt doctor` for read-only integrity checks or `tt doctor --fix` for
explicitly supported repairs.

## Running one workspace on more than one machine

A `tt` workspace is an ordinary directory, so putting it on iCloud Drive,
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
| `.tt/workspace.json`, `.tt/planning.json` | The project catalog and Today/Next ordering. Without them the other machine sees an empty or partial workspace |
| `.tt/lenses.json` | Your saved lenses on the machines that did not write them |
| `.tt/global-notes/`, `projects/<slug>/notes/` | Note bodies, silently. The catalog still names the collection |
| `projects/<slug>/tasks.md` | The tasks themselves |
| `projects/<slug>/.attachments/` | Attachment bytes, while the tasks keep pointing at them. A leading dot makes this the one most likely to be skipped by a rule you did not write for `tt` |
| `.tt/transactions/` | A commit that was interrupted on the other machine can no longer be finished or cleaned up, which is the one exclusion that can leave a half-written file behind |
| `.tt/undo/` | Undo history, which is per-workspace rather than per-machine |
| `.tt/conflicts/` | Recorded conflict bundles, so the machine that has to resolve one may not be the machine that can see it |
| `.tt/backups/` | The migration archives, which are the only route back to an older build |

**Do not run `tt` on two machines at the same time.** The safeguards are real —
revision checks refuse a stale write, lens lists merge instead of overwriting,
and another machine's live commit is left for its owner to finish — but they are
there to survive an overlap, not to make one a way of working. Sync services
deliver files whenever they get to them, so two open sessions see each other's
writes at unpredictable moments; the ten-minute heartbeat window above is the
only clock difference between machines the design accounts for. Quit on one
machine, let the sync settle, then open on the other.

**A `.conflicted copy` file is your sync client's, not `tt`'s.** When two
machines write `tasks.md` or `lenses.json` before either has seen the other's
version, the sync client may keep both, leaving something like
`tasks (Case Conflict).md`, `tasks.md.sync-conflict-20250831-…`, or
`lenses (conflicted copy).json` beside the original. `tt` never writes those
names and never reads them. Its own conflict record is a bundle under
`.tt/conflicts/`, which is what `tt conflict list` shows and the status strip
counts. So:

- A file with a client-invented suffix is a copy you resolve yourself. Compare
  it with the original, keep the content you want under the original name, and
  delete the copy. `tt doctor` will not touch it.
- A bundle in `.tt/conflicts/` is resolved with `tt conflict resolve`.

Provider-specific notes:

- **iCloud Drive** may evict a file's contents while leaving its name in place,
  which reads to `tt` as an unreadable file rather than a missing one. Turn off
  "Optimize Mac Storage" for the workspace, or open the directory in Finder and
  download it before launching. Attachments and notes are the usual victims,
  because they are the largest and the least recently touched.
- **Dropbox** creates conflicted copies as described above, and its Selective
  Sync is the exclusion trap the table warns about: a folder you excluded on one
  machine still exists on the service, so the workspace looks intact from the
  other machine and incomplete from this one.
- **Syncthing** keeps its own file versions under a `.stversions` directory. That
  is a Syncthing archive, not `tt` state; restoring from it means copying a file
  back over the original by hand. Syncthing also does not synchronize clocks, and
  a machine whose clock is well behind the others will write timestamps that read
  as older than they are — which is what the state clock, the check-back
  interval, and the Stale lane are all measured against.

**After a sync problem, run `tt doctor`.** Its checks are ordered so the ones
that block anything else come first: saved lenses, the transaction journal and
anything sitting in quarantine, recorded conflict bundles, stale lock files left
behind by a session that never exited, and the migration archives. Then the
per-project checks, of which one is the expected cross-machine skew: Today and
Next membership disagreeing with a task's own state, because two machines can
write the planning file and the task file in either order. That one is fixable,
so `tt doctor --fix` repairs it.

A quarantined transaction is the one state that stops the workspace outright,
because a manifest `tt` cannot safely finish or undo must not be worked around.
Mutations are refused until it is cleared, and there is no CLI command that
clears it — the recovery step is manual and deliberately so:

1. Quit `tt` on every machine and let the sync settle.
2. Run `tt doctor`. It names each quarantined transaction, where its manifest
   sits and what state it stopped in, the machine that wrote it, whether the
   manifest is readable, and the recovery step itself — the same sentence a
   blocked startup prints, so the two never disagree.
3. Back up the whole data directory before touching anything.
4. Inspect the manifests under `.tt/transactions/quarantine/`. If a file one of
   them names has wrong contents, restore that file from `.tt/backups` first.
5. Move the quarantine directory aside — `.tt/transactions/quarantine.bak` is
   enough — which unblocks mutations. This abandons those transactions; the work
   they carried is lost, which is why the backup comes first.
6. Run `tt doctor` again. It should report no quarantine, and mutations work
   again.

If the same transaction quarantines repeatedly, the cause is usually a partial
sync: the manifest arrived and its staged bytes did not. Wait for the sync to
complete on the machine that wrote it before clearing anything.

