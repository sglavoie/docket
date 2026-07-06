# TUI Review Harness (`tt tui-test`)

`tt tui-test` is a hidden, repo-native surface for LLM-driven review of the Bubble
Tea TUI. It drives the real `AppModel` in-process against isolated fixture data
(or a copy of real config/data) and returns normalized JSON observations. It is
meant for automated review, CI, and development — not day-to-day use — so it is
hidden from `tt --help` and the generated CLI reference.

There is no PTY in v1: actions are applied directly to the model and the rendered
screen is captured and normalized. This keeps runs fast and deterministic.

## Fixtures

Every run starts from an isolated, throwaway workspace. Built-in fixtures give you
reliable, pre-populated data to drive against:

| Fixture    | What it exercises                                                       |
| ---------- | ----------------------------------------------------------------------- |
| `empty`    | Zero-task onboarding surface (the dashboard capture hint).              |
| `rich`     | A full task hierarchy: every type, status, priority, and GTD sections.  |
| `alerts`   | Overdue / due-soon / high-priority-blocked work that trips alerts.      |
| `workflow` | Tasks spread across workflow stages for the stage-oriented views.       |
| `narrow`   | Long titles and tags that stress layout at narrow widths.               |

List them at runtime:

```bash
tt tui-test fixtures            # one name per line
tt tui-test fixtures --json     # {"fixtures":[...]}
```

## Running a scenario

A scenario is a JSON file describing an ordered list of actions to drive against
the app. Run one and inspect the result:

```bash
tt tui-test run --fixture rich --scenario scenario.json --json
```

Useful flags:

| Flag               | Purpose                                                                        |
| ------------------ | ------------------------------------------------------------------------------ |
| `--fixture NAME`   | Built-in fixture to load (default `rich`). Mutually exclusive with `--config`. |
| `--size WxH`       | Initial terminal size (default `100x30`). Overrides the scenario's own `size`. |
| `--scenario PATH`  | Path to the scenario JSON file (required).                                      |
| `--json`           | Emit the structured result instead of a human summary.                         |
| `--observations`   | Include the per-action observation list (large payload).                       |
| `--ansi`           | Include the raw styled (ANSI) screen alongside the plain screen.               |
| `--drain-timeout`  | Per-command async drain budget (`0` uses the default).                         |
| `--keep-workspace` | Keep the temp workspace and print its path to stderr.                          |

Exit codes: `0` every action/assertion passed, `1` a scenario action/assertion
failed, `2` invalid CLI input or an unparseable/invalid scenario. On failure the
result JSON still prints, carrying the failing action and the failure-point
screen under `error`.

When grepping the `--json` result for the pass flag, remember that
`run --scenario --json` is **pretty-printed** (`json.MarshalIndent`), so it emits
`"ok": true` with a **space after the colon**. A bare `grep -o '"ok":[a-z]*'`
matches nothing against that output and silently reports a false "clean". Match
`"ok": *true` (allowing the space) or parse the JSON instead:

```bash
tt tui-test run --scenario scenario.json --json | grep -o '"ok": *true'
```

(The compact `"ok":true` form — no space — belongs to the NDJSON interactive
protocol under [Interactive session mode](#interactive-session-mode-ndjson),
not to scenario runs.)

### Scenario format

```json
{
  "name": "open-today",
  "fixture": "rich",
  "size": "100x35",
  "actions": [
    { "type": "resize", "width": 100, "height": 35 },
    { "type": "key", "label": "open Today (;w)", "key": ";" },
    { "type": "key", "key": "w" },
    { "type": "assert", "assert": "view", "value": "Today" },
    { "type": "assert", "assert": "no_layout_issues" }
  ]
}
```

`fixture` and `size` are optional and make a scenario self-contained; the CLI
`--fixture` / `--size` flags override them when passed. Scenarios are validated
strictly before any action runs — unknown fields, unknown action types, malformed
sizes, invalid keys, and empty scenarios all fail up front.

Action types:

| `type`    | Fields                        | Effect                                                              |
| --------- | ----------------------------- | ------------------------------------------------------------------ |
| `key`     | `key`                         | Send one key press (see key syntax below).                         |
| `text`    | `text`                        | Type a literal string as a sequence of key presses.                |
| `command` | `command`                     | Invoke a registered TUI command by name through the driver (see below). |
| `resize`  | `width`, `height`             | Change the simulated terminal size.                                |
| `wait`    | —                             | Drain previously abandoned async work (long-running commands).     |
| `assert`  | `assert`, `value`             | Check an invariant against a fresh observation (see below).        |

Every action also accepts an optional `label`, echoed back in the failure error so
a failing step names the intent an author gave it (e.g. `"open Today (;w)"`)
instead of only the raw action index.

Multi-key shortcuts are sent one key at a time. For `;w` (Open Today), send a
`key` action for `;` then another for `w`. `:` (command palette) is a single key.

The `command` action invokes a registered command directly, without opening the
command-palette UI or replaying its shortcut keys. The `command` field is the
command **name** — the same string returned as `available_commands[].name` for
the active view. Matching is case-insensitive against those names only; shortcuts
and aliases are metadata and are **not** accepted as identifiers. The command must
be exposed by the active view (a view-scoped command is rejected on other views),
and unknown, unavailable, or unhandled commands fail the action. This lets a
review host discover an action via `available_commands` and then run it by name:

```json
{ "type": "command", "label": "invoke help command", "command": "help" }
```

Key syntax (`parseUITestKey`):

- Printable runes: `"a"`, `"A"`, `"1"`, `"!"`.
- Named keys: `enter`/`return`, `esc`/`escape`, `tab`, `space`, `backspace`.
- Arrows: `up`, `down`, `left`, `right`.
- Modifiers joined with `+`: `ctrl+x`, `alt+o`, `shift+tab`, `ctrl+shift+a`.

Assertion kinds (`assert`):

| `assert`           | `value`                         | Passes when                                                |
| ------------------ | ------------------------------- | ---------------------------------------------------------- |
| `contains`         | substring                       | the plain screen contains `value`.                         |
| `not_contains`     | substring                       | the plain screen does not contain `value`.                 |
| `view`             | view name                       | the current view equals `value` (e.g. `Dashboard`, `Today`, `CommandPalette`, `Outliner`, `CrossProjectOutliner`, `Kanban`). |
| `status_contains`  | substring                       | the status line contains `value`.                          |
| `no_layout_issues` | (ignored)                       | no normalization/layout issues were detected.              |

> The dashboard needs a reasonably tall terminal to render without overflow —
> use at least `100x35` for clean `no_layout_issues` assertions on the dashboard.
> Smaller sizes legitimately report `height_overflow` / `width_overflow`.

### Example scenarios

The [`scenarios/`](scenarios/) directory holds committed, verified examples:

| File                            | Demonstrates                                              |
| ------------------------------- | -------------------------------------------------------- |
| `open-today.json`               | Open the Today cockpit (`;w`) and assert its view.       |
| `command-palette-search.json`   | Open the command palette (`:`) and type a search query.  |
| `command-palette-all-blockers.json` | Execute the `all blockers` palette alias and assert the global blockers view. |
| `command-palette-task-actions-narrow-layout.json` | Open task action mode on the long-title narrow fixture at `72x20`. |
| `command-action-help.json`      | Invoke the `help` command by name (no palette) and assert its view. |
| `navigate-outliner.json`        | Open the global outliner (`;o`) and move the cursor.     |
| `resize-stress.json`            | Resize through extremes and confirm a clean settle.      |
| `focus-empty-minimum-layout.json` | Open the empty Backlog view at `72x20` and assert no layout issues. |
| `baseline-layout.json`          | Assert no layout issues across baseline dashboard sizes. |
| `dashboard-search-alerts-minimum-layout.json` | Activate Dashboard project search on the alert-heavy fixture at `60x20`. |
| `settings-minimum-layout.json`  | Open Settings at `72x20` and assert no layout issues.    |
| `pipeline-hub-minimum-layout.json` | Open Pipeline Hub at `72x20` and assert no layout issues. |
| `metrics-minimum-layout.json`   | Open Metrics at `72x20` and assert no layout issues.     |
| `notifications-minimum-layout.json` | Open the alert-heavy Notification Center at `72x20` and assert no layout issues. |
| `task-template-creator-minimum-layout.json` | Open Task Template Creator at `72x20` and assert no layout issues. |
| `project-creator-minimum-layout.json` | Open Project Creator at `72x20` and assert no layout issues. |
| `project-editor-minimum-layout.json` | Open Project Editor at `72x20` and assert no layout issues. |
| `workflow-rule-creator-minimum-layout.json` | Open Workflow Rule Creator at `72x20` and assert no layout issues. |
| `inbox-processor-minimum-layout.json` | Open Inbox Processor at `72x20` and assert no layout issues. |
| `global-inbox-processor-minimum-layout.json` | Open Global Inbox Processor at `72x20` and assert no layout issues. |
| `date-input-minimum-layout.json` | Open the Set Due Date input at `72x20` and assert no layout issues. |
| `snooze-input-minimum-layout.json` | Open the Snooze Task input at `72x20` and assert no layout issues. |

Run any of them:

```bash
tt tui-test run --scenario docs/tui-testing/scenarios/open-today.json --json
```

## Interactive session mode (NDJSON)

`tt tui-test session` keeps a single driver alive and speaks newline-delimited
JSON: one request object per line on stdin, one response object per line on
stdout. State persists across requests, so it is the right tool for exploratory,
multi-step review where each step depends on the last. Logs, prompts, and human
text go to **stderr only** — stdout stays pure NDJSON.

```bash
tt tui-test session --fixture rich --size 100x35
```

Request — either a driver `action` or a session `command`, never both:

```json
{"id":"1","action":{"type":"resize","width":100,"height":35}}
{"id":"2","command":"observe"}
{"id":"3","action":{"type":"key","key":":"}}
{"id":"4","command":"reset"}
{"id":"5","command":"quit"}
```

Session commands:

| `command` | Effect                                                                 |
| --------- | ---------------------------------------------------------------------- |
| `observe` | Return a fresh observation without changing state.                     |
| `reset`   | Rebuild a fresh driver/workspace from the same fixture or config.      |
| `quit`    | Return a final observation and end the session.                        |

The top-level session `"command"` (`observe`/`reset`/`quit`) controls the NDJSON
loop itself and is a distinct field from the driver action type `"command"`. To
invoke a TUI command in session mode, send it as an action, not as a session
command:

```json
{"id":"6","action":{"type":"command","command":"help"}}
```

Each request carries either a session `"command"` or an `"action"`, never both.

Response:

```json
{"id":"1","ok":true,"observation":{...}}
{"id":"3","ok":false,"observation":{...},"error":{"kind":"action_failed","reason":"..."}}
```

Error kinds are stable strings you can branch on: `bad_request`,
`unknown_command`, `reset_failed`, `action_failed`. On a failed action the
response still carries the failure-point `observation`.

## Observation fields

Both `run` and `session` return the same normalized observation shape. Fields
marked *omitempty* are absent when empty.

| Field                | Meaning                                                                              |
| -------------------- | ------------------------------------------------------------------------------------ |
| `current_view`       | Name of the active view (e.g. `Dashboard`, `Today`, `CommandPalette`).               |
| `view_stack`         | The navigation stack from root to the active view.                                   |
| `width` / `height`   | The simulated terminal size.                                                         |
| `plain_screen`       | The rendered screen with styling stripped and invalid UTF-8 sanitized.               |
| `ansi_screen`        | *(omitempty)* The raw styled screen; present only with `--ansi` / `IncludeANSI`.     |
| `status`             | *(omitempty)* The current status-line message.                                       |
| `status_is_info`     | *(omitempty)* True when the status is an informational (not error) message.          |
| `error`              | *(omitempty)* The current error message shown to the user, if any.                   |
| `layout_issues`      | *(omitempty)* Detected normalization/layout problems — each `{kind, message, line}`. |
| `pending_async`      | *(omitempty)* Count of commands still outstanding after the drain budget (drain with a `wait` action). |
| `snapshot`           | *(omitempty)* A compact semantic map of the active view's state (see below).         |
| `available_commands` | *(omitempty)* Commands the active view exposes, sorted by name.                       |

`layout_issue.kind` values include `invalid_utf8`, `width_overflow` (a line wider
than the terminal), and `height_overflow` (more rendered lines than the terminal
height). `line` is 1-based, or `0` for screen-wide issues.

### `snapshot`

A compact, stable per-view semantic map — high-signal fields only (counts,
cursor, selected-item title, active mode/tab/perspective), never whole task or
project structs. It is present only for views that implement it: **Dashboard,
Today, Outliner, Kanban, CommandPalette** (Cross-Project Outliner/Kanban reuse the
same models). Keys vary by view and are defined in
`internal/tui/uitest_snapshot.go` — consult that file for the authoritative set.
Examples:

- Dashboard: `{cursor, project_count, selected_project, show_archived}`
- CommandPalette: `{action_mode, cursor, query, result_count, selected_category, selected_item}`
- Cross-Project Outliner: `{cross_project, cursor, perspective, row_count}`

Because snapshot rows/columns are rebuilt lazily during a render, the driver
always renders the active view before reading the snapshot — live scenarios need
no special handling.

### `available_commands`

Each entry is `{name, shortcut, category, aliases}` for a command the active view
exposes, sorted by name for stable output. This lets a review host discover the
available actions without scraping help text. `shortcut` and `aliases` are
*omitempty*. The dashboard exposes ~63 commands; some views (e.g. the palette
itself) expose none, in which case the field is absent.

## Safety guarantees

Both modes are read-only with respect to your real data:

- **Fixture mode** builds a fresh, isolated temp workspace (temp data dir + config)
  and drives the app there. External tool integrations (fzf, glow) are disabled so
  the run stays fully headless. The workspace is removed when the run ends (pass
  `--keep-workspace` to keep it and print its path).
- **Copied-config mode** (`--config PATH`, or the inherited persistent `--config`)
  stages the source config bytes into a temp workspace, copies the data dir into
  temp storage, and rewrites `data_dir` + project paths onto the copy before
  launching. Projects whose data directory lives **outside** the data dir are
  copied too — each is staged into its own subdirectory under the temp workspace
  and repointed onto that copy — so no source project directory is driven in
  place. The **source config and task files are never mutated** — the copy
  absorbs every write (including first-load config migrations). This is verified
  end to end in the test suite. `--config` and `--fixture` are mutually exclusive.

Direct live writes to your real data directory are out of scope for v1.

## Known layout issues

These are real, reproducible layout defects the harness can surface but that the
committed scenarios do **not** yet guard. They are tracked as follow-ups, not
fixed.

- **Cross-project outliner multi-select overflows at 60×20.** Entering
  multi-select in the global outliner (`;o` → `v`) prefixes each row with a
  `[ ]` checkbox. That extra width pushes the trailing `#ID` past the edge of the
  ~33-column tree pane, so it wraps and the screen reports `height_overflow` at
  60×20. The shipped scenario
  `scenarios/outliner-multiselect-minimum-layout.json` only asserts
  `no_layout_issues` at **72×20**, where the rows still fit — so this narrower
  overflow is currently invisible to regression coverage. Repro: `;o` → `v` at
  60×20. Root cause: the checkbox-prefix width forces `#ID` to wrap in the narrow
  tree pane.
