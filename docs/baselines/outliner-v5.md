# Outliner-only v5 contract and baseline

This is the Session 1 acceptance record for the outliner-only v5
simplification. The destination inventories are machine-readable through
`testutil.OutlinerV5TargetContract`; the current-state inventory is preserved by
`testutil.OutlinerV5CurrentBaseline`. Neither fixture configures production, so
recording the destination does not change current behavior.

Baseline commit: `549400c3`.

## Target contract

The Outliner is the only primary presentation. Every run begins in All Projects
and All with no restored navigation, filters, search, or selection. Search and
filters survive mode and scope changes only for the current run. Frequent
actions have one direct binding, occasional task actions live only under
`Space`, and help plus footer hints derive from the same operation descriptors.

### Retained inventories

| Inventory | Exact v5 contents |
| --- | --- |
| Modes and Tab cycle | All → Inbox → Next → Backlog → Due |
| States | Inbox, Ready, Next, In Progress, Waiting, Paused, Someday, Done, Canceled |
| Task `Space` menu | Recurrence, Move Project, Set Parent, Dependencies, Checklist, Comments, Decisions, Attachments, Delete |
| Configuration domains | `workspace` (lean catalog and protected System Inbox ID), `planning` (Today and Next order), `machine` (custom contexts and dark/light theme) |
| Public CLI | `tt`, `doctor`, `conflict`, `workspace`, `completion`, `project export`, help, `--version` |
| Global flags | `--data-dir`, `--no-color` |

The direct interaction grammar is:

| Area | Keys |
| --- | --- |
| Modes | `Tab`, `Shift+Tab` |
| Navigation | `j`/Down, `k`/Up, `h`/Left, `l`/Right, Home, End, PageUp, PageDown, `Enter` |
| Capture and edit | `a`, `A`, `e`, `E`, `Ctrl+E` |
| State and planning chord | `si`, `sr`, `sn`, `sg`, `sw`, `sp`, `sm`, `sd`, `sc`, `st` |
| Metadata | `!0`, `!1`–`!5`, `@`, `#`, `z`, `P` |
| Structure | `<`, `>`, `J`, `K` |
| Clipboard | `y`, `Y`, `Ctrl+Y` |
| Selection and task menu | `v`, `V`, `Space` |
| Global | `c`, `C`, `N`, `T`, `/`, `?`, `u`, `Ctrl+R`, `Esc`, `Ctrl+C` |

Filters use `fp0`, `fp1`–`fp5`, `fs`, `ft`, `fd`, `fb`, `fz`, and `fx`.
Quick capture accepts only `!1-5`, `#tag`, `@date`, `~interval`, `$state`,
`%project`, and `>parent`; colon aliases and `^type` are not part of v5.

The removal contract covers alternate primary views and duplicate commands;
mode picker, command palette, Finder, and recent destinations; AI and Ask;
workflow automation, templates, and scoring; reviews, analytics, metrics,
roadmap, alerts, notifications, onboarding, and activity history; related links,
standalone blockers, park/dim, status icons, and tag catalogs; archived-task
storage; project trash/delete/purge, Sandbox, and Internal projects; Settings
TUI and custom theme editing; rich note metadata; retired task metadata; and all
non-maintenance CLI commands.

## Current-state baseline

These facts characterize the starting point. They are historical observations,
not gates that later sessions must preserve.

| Area | Commit `549400c3` behavior |
| --- | --- |
| Clean install | Domain bootstrap creates synced `workspace`, `planning`, `settings`, and `automation` files plus machine-local state. TUI startup ensures the visible System Inbox and hidden Sandbox. Settings default to context `personal`; machine editor defaults to `code`. |
| Daily loop | Ten workflow states include Review. Ten modes are All, Inbox, Next, Backlog, Due, Priority, Blocked, Parked, Daily Review, and Activity. Tab cycles the first four. The mode picker, Finder, weekly review, tag manager, and notifications remain available. |
| Navigation | The cross-project Outliner is already the sole root, but startup reapplies machine `StartupRoute` state, including mode, search, and filters. `Ctrl+Esc` performs a broad reset. |
| Storage | Five configuration domains are `workspace`, `planning`, `settings`, `automation`, and `machine`. Active project Markdown coexists with completed archives and indexes, notes, attachments, transaction manifests, undo ledgers, and conflict bundles. |
| CLI | The top-level command tree still exposes `add`, `ask`, `attach`, `attachment`, `attachments`, `blocker`, `completion`, `conflict`, `context`, `dash`, `doctor`, `epic`, `focus`, `help`, `note`, `project`, `redo`, `roadmap`, `sandbox`, `score`, `search`, `select`, `tag`, `task`, `template`, `theme`, `undo`, `workflow`, and `workspace`. |

The prior daily-loop fixture remains a record of the pre-v5 acceptance surface.
The v5 fixture is separate so later sessions can remove behavior deliberately
without rewriting this baseline into a fictional starting state.
