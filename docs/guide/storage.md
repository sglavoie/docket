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


## Schema 15 storage and migration

The live portable layout is deliberately small:

```text
<data>/
├── .tt/
│   ├── workspace.json
│   ├── planning.json
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

Project task files use strict schema-14 Markdown and generic `Task` headings.
`workspace.json` contains the lean project catalog, project tags, and the
protected Unfiled ID. `planning.json` contains Today and
Next ordering only.
Each task may carry an optional non-negative `defer_count`; zero is omitted.
Each task may also carry an optional `due_time`, a canonical `HH:MM` wall-clock
in 00:00–23:59 that pairs with the calendar `due` date; midnight is reserved for
date-only due dates, so a date-only task omits it.
Each task may finally carry an optional `depends` list of soft references to
other tasks, each a `project-id:task-id` pair; an empty list is omitted. A
reference is durable and one-directional: only the referring task stores it, so
the dependents you see are computed at read time and never written down.
Machine-local `~/.tt/runtime/<workspace-id>/machine.json` contains only the
dark/light theme choice, the saved named-lens definitions, and a pointer to the
lens that was last open — the three seeded starters, anything you saved
yourself, and anything you added from the built-in gallery, which are all one
list. An empty lens list
persists as `[]` and is honored, which is what makes deletion durable.
The active-lens pointer is a hint rather than a
record: it is checked for shape alone when the file loads, so one naming a lens
that is no longer there costs you a stale hint and never a failed launch.

Startup migrates supported schema-14 workspaces to schema 15, supported
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
`pre-due-time-v14-<UTC>.tar.gz` for v13→v14, and
`pre-dependencies-v15-<UTC>.tar.gz` for v14→v15.

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

Invalid or future-schema input aborts without changing workspace data. An
existing `unfiled` project or directory also aborts before snapshot publication
with rename/move guidance. Interrupted activation is recovered before ordinary
loading, and repeated startup is idempotent.

There is no down-migration. An older `tt` refuses a migrated workspace as newer
than its supported schema, and no build converts a workspace back. Recovery is
manual: extract the matching archive — `pre-due-time-v14-<UTC>.tar.gz` for the
schema-14 boundary or `pre-dependencies-v15-<UTC>.tar.gz` for schema 15 — into a
separate directory and open that copy with a compatible build. Migration
snapshots themselves are never modified.

Run `tt doctor` for read-only integrity checks or `tt doctor --fix` for
explicitly supported repairs.

