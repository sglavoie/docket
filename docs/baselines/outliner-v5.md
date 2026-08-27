# Historical acceptance record: the outliner-only simplification

This file is history, not contract. It was the Session 1 acceptance record for
the outliner-only simplification and captured the schema-6 surface as it stood
at commit `549400c3`. The live product has moved past it: there are seven modes,
not six; the task `Space` menu is Delete alone; the machine domain also holds
`NamedLenses`; and the global key list below is stale. For the current
contract read [`CLAUDE.md`](../../CLAUDE.md), [`README.md`](../../README.md),
and [`docs/architecture.md`](../architecture.md).

Two earlier fixture names in this document,
`testutil.OutlinerV5TargetContract` and `testutil.OutlinerV5CurrentBaseline`,
never existed in the repository; nothing here is machine-readable.

## Permanent removal contract (still binding)

This is the one section that still governs. Nothing it names may return:

The removal contract covers alternate primary views and duplicate commands;
mode picker, command palette, Finder, and recent destinations; AI and Ask;
workflow automation, templates, and scoring; reviews, analytics, metrics,
roadmap, alerts, notifications, onboarding, and activity history; related links,
standalone blockers, park/dim, status icons, and tag catalogs; archived-task
storage; project trash/delete/purge, Sandbox, and Internal projects; Settings
TUI and custom theme editing; rich note metadata; retired task metadata; and all
non-maintenance CLI commands.

Three further non-goals, rescued from the retired lean-search/notes restoration
plan when that file was deleted, bind alongside it:

- No universal Finder or workspace-wide search overlay returns; `/`, `P`, and
  Notes remain separate discovery surfaces.
- Search autocomplete stays Outliner-only; Project Hub and Notes remain
  ordinary fuzzy text searches.
- Project Hub aliases never include task titles, task descriptions, task tags,
  task comments, or task content.

## Historical schema-6 target contract (superseded)

Baseline commit: `549400c3`. Everything below this line records what was
accepted then, not what is true now.

The Outliner is the only primary presentation. Every run begins in All Projects
and All with no restored navigation, filters, search, or selection. Search and
filters survive mode and scope changes only for the current run. Frequent
actions have one direct binding, occasional task actions live only under
`Space`, and help plus footer hints derive from the same operation descriptors.

### Retained inventories as recorded then

| Inventory | Schema-6 contents as accepted |
| --- | --- |
| Modes and Tab cycle | All → Inbox → Today → Next → Backlog → Due |
| States | Inbox, Backlog, Doing, Waiting, Paused, Someday, Done, Canceled |
| Task `Space` menu | Comments, Attachments, Delete |
| Configuration domains | `workspace` (lean catalog, exclusive regular contexts, and protected Unfiled ID), `planning` (Today and Next order), `machine` (dark/light theme only) |
| Public CLI | `tt`, `doctor`, `conflict`, `workspace`, `completion`, `project export`, help, `--version` |
| Global flags | `--data-dir`, `--no-color` |

The direct interaction grammar as recorded then was:

| Area | Keys |
| --- | --- |
| Modes | `Tab`, `Shift+Tab` |
| Navigation | `j`/Down, `k`/Up, `h`/Left, `l`/Right, Home, End, PageUp, PageDown, `Enter` Focus, `Shift+Enter` Peek |
| Capture and edit | `a`, `A`, `e`, `E`, `Ctrl+E` |
| Workflow chord | `si`, `sb`, `sn`, `st`, `sg`, `sr`, `sw`, `sp`, `sm`, `sd`, `sc` |
| Metadata | `!0`, `!1`–`!3`, `@`, `#`, `z`, `P` |
| Structure | `<`, `>`, `J`, `K` |
| Clipboard | `y`, `Y`, `Ctrl+Y` |
| Selection and task menu | `v`, `Ctrl+A`, `Space`; `V` is unbound |
| Global | `c`, `C`, `N`, `T`, `/`, `?`, `u`, `Ctrl+R`, `Esc`, `Ctrl+C` |

Filters used `fp0`, `fp1`–`fp3`, `fs`, `fc`, `ft`, `fd`, `fz`, and `fx`.
Quick capture accepted `!1-3`, `#tag`, `@date`, `$destination`, `%project`,
`>parent`, and the repeatable `<child`. Destinations were Inbox, Backlog, Today,
Next, Doing, Waiting, Paused, Someday, Done, and Canceled. `#tag` created valid
tags on the fly. Retired type and recurrence sigils, priorities 4–5, `$ready`,
and `$in_progress` were recognized errors with concise replacement guidance.

## Pre-simplification starting point (historical observations)

These facts characterized the starting point. They are historical observations,
not gates that later sessions must preserve.

| Area | Commit `549400c3` behavior |
| --- | --- |
| Clean install | Domain bootstrap creates synced `workspace`, `planning`, `settings`, and `automation` files plus machine-local state. TUI startup ensures the visible System Inbox and hidden Sandbox. Settings default to context `personal`; machine editor defaults to `code`. |
| Daily loop | Ten workflow states include Review. Ten modes are All, Inbox, Next, Backlog, Due, Priority, Blocked, Parked, Daily Review, and Activity. Tab cycles the first four. The mode picker, Finder, weekly review, tag manager, and notifications remain available. |
| Navigation | The cross-project Outliner is already the sole root, but startup reapplies machine `StartupRoute` state, including mode, search, and filters. `Ctrl+Esc` performs a broad reset. |
| Storage | Five configuration domains are `workspace`, `planning`, `settings`, `automation`, and `machine`. Active project Markdown coexists with completed archives and indexes, notes, attachments, transaction manifests, undo ledgers, and conflict bundles. |
| CLI | The top-level command tree still exposes `add`, `ask`, `attach`, `attachment`, `attachments`, `blocker`, `completion`, `conflict`, `context`, `dash`, `doctor`, `epic`, `focus`, `help`, `note`, `project`, `redo`, `roadmap`, `sandbox`, `score`, `search`, `select`, `tag`, `task`, `template`, `theme`, `undo`, `workflow`, and `workspace`. |
