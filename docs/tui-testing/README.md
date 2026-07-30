# Deterministic TUI scenarios

The repository includes a test-only driver for the real Bubble Tea
`AppModel`. It uses the production operation registry, input strategy,
navigation, persistence, and renderer while replacing terminal I/O with
deterministic actions and normalized observations. It is not a public `tt`
command.

Fixtures live in isolated temporary workspaces. They never open or mutate the
user's selected workspace, and external programs such as editors and fuzzy
finders are disabled unless a test explicitly supplies a controlled
replacement.

## Run the acceptance scenarios

Replay every committed JSON scenario plus the focused snapshot-first migration
and interrupted-activation recovery tests:

```bash
./scripts/run_tui_scenarios.sh
```

During authoring, replay one JSON file:

```bash
./scripts/run_tui_scenarios.sh \
  docs/tui-testing/scenarios/daily-loop.json
```

Every `*.json` file in `docs/tui-testing/scenarios/` is discovered by
`TestExampleScenariosPass`; there is no separate scenario manifest to update.
The full script is the acceptance gate.

## Fixtures

| Fixture | Purpose |
| --- | --- |
| `empty` | A workspace with no user projects. |
| `fresh` | A genuine blank install that exercises protected-Inbox bootstrap. |
| `rich` | Hierarchy, workflow states, metadata, and relationships. |
| `busy` | Overdue, due-soon, blocked, waiting, and five Inbox tasks. |
| `workflow` | A compact lifecycle spread for state and planning operations. |
| `narrow` | Long titles and tags for minimum-width rendering. |
| `contexts` | Several projects across regular and custom scope labels. |
| `release` | A large Unicode catalog for viewport and resize stress. |

## Scenario format

```json
{
  "name": "visible-search",
  "fixture": "rich",
  "size": "100x35",
  "actions": [
    { "type": "key", "label": "open search", "key": "/" },
    { "type": "text", "label": "find login work", "text": "login" },
    { "type": "assert", "assert": "view", "value": "CrossProjectOutliner" },
    { "type": "assert", "assert": "no_layout_issues" }
  ]
}
```

Unknown fields, malformed sizes or keys, empty text actions, unsupported
assertions, and trailing JSON are rejected before execution.

### Actions

| Type | Fields | Behavior |
| --- | --- | --- |
| `key` | `key` | Send one parsed key press. |
| `text` | `text` | Type literal runes into the active input. |
| `resize` | `width`, `height` | Resize the simulated terminal. |
| `wait` | none | Drain pending asynchronous work within the test budget. |
| `command` | `command` | Invoke a stable harness or production operation ID. |
| `assert` | `assert`, optional `value` | Check a fresh normalized observation. |

An optional `label` describes user intent and is repeated in failure output.
Prefer public keys for acceptance behavior. A `command` action is appropriate
only for deterministic setup or for a deep link deliberately absent from the
public grammar.

Accepted key names include printable public keys plus:

```text
tab shift+tab enter space esc
up down left right home end pgup pgdown
ctrl+r ctrl+u ctrl+d ctrl+y ctrl+c
```

The final public grammar is:

```text
Tab/Shift+Tab                    All, Inbox, Next, Backlog, Due
j/k h/l g/G Ctrl+U/Ctrl+D        navigation
a/A e/E                          capture and edit
s d b t n                        daily state and planning
,i ,w ,p ,s ,c                  other workflow states
!0-5 @ # z P                    metadata
< > J K                          structure
y Y Ctrl+Y                      clipboard
v V Space Enter                 selection, actions, Peek
c N T / ? u Ctrl+R Esc Ctrl+C   global
fp0-5 fs ft fd fb fz fx         filters
```

`m`, `:`, `R`, `p`, and `Ctrl+Esc` are intentionally unbound.

### Assertions

| Assertion | Value | Meaning |
| --- | --- | --- |
| `contains` | substring | The plain screen contains the value. |
| `not_contains` | substring | The plain screen does not contain the value. |
| `view` | view name | `current_view` equals the value. |
| `status_contains` | substring | The informational/status line contains the value. |
| `no_layout_issues` | none | Rendering has no overflow, UTF-8, or height audit issue. |

Use semantic labels and view names instead of exact full-screen snapshots or
color escape sequences. Add `no_layout_issues` at every important narrow or
resized state.

## Final-product coverage

The committed suite exercises the product contracts directly:

| Contract | Representative scenarios |
| --- | --- |
| Capture grammar and protected-Inbox routing | `capture-grammar.json`, `fresh-install-capture.json` |
| Inbox triage and daily `n`/`b`/`t`/`s`/`d` loop | `daily-loop.json` |
| Five modes and navigation aliases | `mode-catalog.json`, `navigate-outliner.json`, `outliner-navigation-aliases.json` |
| Priority, workflow, blocked, snoozed, and clear filters | `filter-chord-priority.json`, `filter-chord-states.json`, `filter-chord-clear.json` |
| Scope and project administration | `context-hub-open.json`, `context-hub-*.json`, `project-*.json` |
| Combined Global/project Notes | `notes-tool.json`, `global-notes-resize-minimum-layout.json` |
| Checklist, comments, decisions, and attachments | `rich-tools.json` |
| Exact nine-item `Space` menu | `outliner-space-action-catalog.json` |
| Full-width cursor-following Peek | `outliner-peek-wide-layout.json`, `outliner-peek-narrow-layout.json` |
| Visual selection and bulk intersection | `outliner-multiselect-*.json`, `outliner-bulk-action-intersection.json` |
| Dark/light machine-local theme | `theme-toggle.json` |
| Generated operation help and retired-key absence | `in-app-help.json`, `mode-catalog.json` |
| Responsive and large-catalog rendering | `baseline-layout.json`, `resize-stress.json` |

The full shell gate also runs the config-package migration tests for valid v4
activation, corrupt/future input refusal, idempotence, and prepared activation
recovery. JSON fixtures begin after workspace construction, so migration and
startup recovery belong at that lower boundary rather than being simulated
inside the TUI driver.

## Observation contract

Each action observes:

| Field | Meaning |
| --- | --- |
| `current_view` | Active typed presentation, such as `CrossProjectOutliner`, `Notes`, or `Help`. |
| `view_stack` | Current typed presentation path. |
| `width`, `height` | Simulated terminal dimensions. |
| `plain_screen` | ANSI-free rendering. |
| `status`, `status_is_info`, `error` | Shell feedback state. |
| `layout_issues` | Structured render-audit failures. |
| `pending_async` | Commands still pending after bounded draining. |
| `snapshot` | Small view-specific semantic state. |
| `available_commands` | Applicable production operations from the active input strategy. |

Snapshots are intentionally bounded. The Outliner snapshot includes mode,
cursor, row count, scope/search state, and selected identity rather than entire
task or project objects. Harness-only setup commands never appear in
`available_commands`.

## Authoring checklist

1. Start from the smallest fixture that demonstrates the behavior.
2. Drive public behavior with public keys.
3. Use harness commands only for setup with no public shortcut.
4. Assert stable semantics, including the final view and layout health.
5. Replay the individual file, then run the complete acceptance script.
