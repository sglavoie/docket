# TUI review harness

`tt tui-test` is a hidden, repository-native interface for automated and
LLM-assisted review of the real Bubble Tea application. It runs the same
`AppModel`, operation registry, typed navigation, persistence, and rendering as
the interactive TUI, but drives them in-process and returns normalized JSON.

The harness is intended for tests and review tooling. It is hidden from normal
CLI help and from the generated CLI reference.

## Safety

Built-in fixtures are created in isolated temporary workspaces. Scenario
mutations never touch the user's workspace.

Copied-workspace mode first copies the selected workspace into a temporary
directory, assigns the copy a fresh workspace identity, and runs against that
copy. The source remains read-only. Use `--keep-workspace` only when you need to
inspect the disposable copy after a run.

External integrations such as `fzf` and `glow` are disabled in harness
workspaces, so runs stay headless and deterministic.

## Fixtures

List fixtures with:

```bash
tt tui-test fixtures
tt tui-test fixtures --json
```

| Fixture | Purpose |
| --- | --- |
| `empty` | Empty workspace behavior. |
| `fresh` | First-run onboarding and System Inbox capture. |
| `rich` | Hierarchy, all canonical workflow states, dates, tags, and relationships. |
| `alerts` | Overdue, blocked, waiting, and multi-item Inbox behavior. |
| `workflow` | Workflow rules and automation editors. |
| `narrow` | Long content for responsive-layout audits. |
| `contexts` | Regular/custom contexts plus active, archived, and trashed projects. |

## Run a scenario

```bash
tt tui-test run \
  --scenario docs/tui-testing/scenarios/daily-loop.json \
  --json
```

Useful flags:

```text
--fixture <name>       override the scenario fixture
--size <WxH>           override the scenario size
--observations         include every per-action observation
--ansi                 include styled output as well as plain output
--drain-timeout <dur>  override the per-command async budget
--keep-workspace       retain the temporary workspace and print its path
```

A passing JSON result includes `"ok": true`, the scenario name, the action
count, and the final observation. A failed result identifies the action index,
its optional label, the reason, and the failure-point observation. Process exit
codes are:

| Code | Meaning |
| --- | --- |
| `0` | Every action and assertion passed. |
| `1` | A scenario action or assertion failed. |
| `2` | CLI input or scenario JSON was invalid. |

## Scenario format

```json
{
  "name": "finder-search",
  "fixture": "rich",
  "size": "100x35",
  "actions": [
    { "type": "key", "label": "open Finder", "key": ":" },
    { "type": "text", "label": "find Today", "text": "today" },
    { "type": "assert", "assert": "view", "value": "Finder" },
    { "type": "assert", "assert": "no_layout_issues" }
  ]
}
```

Unknown fields, malformed sizes, invalid keys, empty text actions, unknown
assertions, and trailing JSON are rejected before execution.

### Actions

| Type | Fields | Behavior |
| --- | --- | --- |
| `key` | `key` | Send one parsed key press. |
| `text` | `text` | Type literal runes into the active input. |
| `resize` | `width`, `height` | Resize the simulated terminal. |
| `wait` | none | Drain pending asynchronous work with a bounded wait. |
| `command` | `command` | Execute an applicable operation by stable ID or label. |
| `assert` | `assert`, optional `value` | Check the current normalized observation. |

`label` is optional on every action and should describe user intent, such as
`"triage the first Inbox task to Next"`. Failure output repeats it.

Key names are case-sensitive where the TUI contract is case-sensitive:

```text
a e s d b t n v
N R T !
tab shift+tab enter space esc
up down left right
ctrl+r ctrl+esc ctrl+c
pgup pgdown home end
```

Prefer real public keys for user workflow coverage. A `command` action uses the
same `OperationRegistry` and `OperationExecutor` as production. Explicit
scenario commands gain `CapabilityHarness`, which permits setup-only deep links
that are intentionally absent from Finder, help, and `available_commands`.

### Assertions

| Assertion | Value | Meaning |
| --- | --- | --- |
| `contains` | substring | Plain screen contains the value. |
| `not_contains` | substring | Plain screen does not contain the value. |
| `view` | view name | `current_view` equals the value. |
| `status_contains` | substring | The current informational/status line contains the value. |
| `no_layout_issues` | none | No overflow, invalid UTF-8, or height audit issue exists. |

Assertions always observe freshly rendered state. Use `wait` after mutations
when the scenario needs the committed reload or the status timeout rather than
the immediate optimistic frame.

## Committed final-application contracts

Every JSON file in `docs/tui-testing/scenarios/` is replayed by
`TestExampleScenariosPass` and `./scripts/run_tui_scenarios.sh`.

The primary daily-loop contracts are:

| Behavior | Scenarios |
| --- | --- |
| All over All Projects and responsive root | `open-all.json`, `baseline-layout.json`, `outliner-alerts-minimum-layout.json`, `resize-stress.json` |
| Capture → Inbox | `fresh-install-capture.json`, `task-composer-minimum-layout.json` |
| Inbox auto-advance and `n`/`b`/`t`/`s`/`d` loop | `daily-loop.json` |
| Exact ten-mode catalog and Tab modes | `mode-catalog.json`, `navigate-outliner.json` |
| Contextual action sheet and value picker | `date-input-minimum-layout.json`, `snooze-input-minimum-layout.json` |
| Multi-select and bulk action sheet | `outliner-multiselect-minimum-layout.json`, `outliner-multiselect-narrow-layout.json` |
| Scope Hub browse, search, mutation, and narrow layout | `context-hub-open.json`, `scope-hub-search-commit.json`, `context-hub-*.json` |
| Empty Finder shortlist and typed search | `finder-shortlist.json`, `finder-search.json`, `finder-task-actions-narrow-layout.json` |
| Promoted Notes, Weekly Review, Tags, and Notifications | `promoted-tools.json`, `global-notes-resize-minimum-layout.json`, `notifications-minimum-layout.json` |
| Global/project Inbox responsive layouts | `global-inbox-mode-*.json`, `project-inbox-mode-*.json` |
| Visible search and narrow task detail | `visible-search-minimum-layout.json`, `task-detail-narrow-layout.json` |

Additional committed scenarios cover Settings, project creation/editing,
templates, workflow editors, Roadmap, Metrics, messages, reference content, and
other retained secondary tools. These are typed children or overlays of the
Outliner; they are not alternate task roots.

## Interactive NDJSON session

Session mode keeps one driver alive and reads one request per line:

```bash
tt tui-test session --fixture rich --size 100x35
```

Action request:

```json
{"id":"1","action":{"type":"key","key":":"}}
```

Control requests:

```json
{"id":"2","command":"observe"}
{"id":"3","command":"reset"}
{"id":"4","command":"quit"}
```

Each response repeats `id` and returns either an observation or a structured
error. Standard output is pure NDJSON; diagnostics go to standard error.

## Observation contract

An observation contains:

| Field | Meaning |
| --- | --- |
| `current_view` | Active presentation identity, such as `CrossProjectOutliner`, `Finder`, or `TaskDetail`. |
| `view_stack` | Current typed presentation path. |
| `width`, `height` | Simulated terminal dimensions. |
| `plain_screen` | ANSI-free rendered screen. |
| `ansi_screen` | Optional styled screen when `--ansi` is set. |
| `status`, `status_is_info`, `error` | Shell feedback state. |
| `layout_issues` | Structured render audit failures. |
| `pending_async` | Commands still pending after the bounded drain. |
| `snapshot` | Small view-specific semantic state. |
| `available_commands` | Applicable production operations sorted by name. |

Snapshots expose high-signal values only. The Outliner snapshot includes its
mode, cursor, row count, scope/search state, and selected task when applicable.
Finder and list surfaces expose bounded semantic state rather than entire task
or project objects.

`available_commands` is derived from the active input-strategy projection of
`OperationRegistry`; dormant harness-only deep links are excluded.

## Add or update a scenario

1. Start from the smallest fixture that contains the required behavior.
2. Use public keys for acceptance behavior and harness commands only for setup.
3. Assert stable semantic labels, not color or exact full-screen snapshots.
4. Include `no_layout_issues` at every important narrow/resized state.
5. Give mutation steps intent labels and wait only when committed state is
   required.
6. Run the file directly, then replay the complete committed set:

```bash
tt tui-test run --scenario docs/tui-testing/scenarios/<name>.json --json
./scripts/run_tui_scenarios.sh
```
