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
| `fresh` | A genuine blank install that exercises protected-Unfiled bootstrap. |
| `rich` | Hierarchy, workflow states, metadata, and relationships. |
| `busy` | Overdue, due-soon, waiting, and five Inbox tasks. |
| `workflow` | A compact lifecycle spread for state and planning operations. |
| `narrow` | Long titles and tags for minimum-width rendering. |
| `multi-project` | Several distinctly named projects, each holding one backlog task. |
| `release` | A large Unicode catalog for viewport and resize stress. |
| `parked` | One project in the base task workspace and one tagged `parked`. |
| `project-mix` | An active project with a task, an active project with none, a parked project with a task, and an archived project with none. |
| `signals` | A lapsed deferral beside a whole-day and an hour-valued pending one, concentrated overdue work, and a durable Today membership. |
| `checklist` | The only fixture whose tasks carry checklist items, in the three shapes the fold ladder distinguishes. |

Only `parked` and `project-mix` split the base task workspace from the catalog.
Every other fixture leaves all of its projects untagged, so parked behavior is
indistinguishable from an empty catalog there. `project-mix` is also the only
fixture carrying an archived project, so the archived Select Project section is
observable nowhere else.

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
ctrl+e ctrl+r ctrl+u ctrl+d ctrl+o ctrl+i ctrl+l ctrl+n ctrl+y ctrl+c
```

The final public grammar is:

```text
Tab/Shift+Tab                    All, Today, Inbox, Next, Backlog, Due
j/k h/l S g/G Ctrl+U/Ctrl+D      navigation
H/M/L                            cursor to top/middle/bottom of screen
zt zz zb                         cursor row to top/middle/bottom of screen
n/N                              next/previous grouping header, wrapping
Ctrl+O/Ctrl+I                     older/newer relocated-task positions
Ctrl+L                           Recent, focused on the newest activity
a/A e/E Ctrl+E O                 capture and edit
wi wb wn wt wg wr ww wp wm wd wc    workflow chord
!0-3 @ # d P                    metadata
< > J K                          structure
y Y Ctrl+Y                      clipboard
v V Ctrl+A Space                selection and actions
Enter Shift+Enter               Focus and Peek
x                               tick the checklist item under the cursor
p Ctrl+N T / ? u Ctrl+R Esc Ctrl+C   global
fp0-3 fs fc ft fr f@ fd fx      filters
```

The old standalone `b`, `d`, and `t` actions and comma state prefix are
intentionally unbound, as are `c`, `m`, `:`, `R`, and `Ctrl+Esc`.

The presentation grammar is also part of the scenario contract. Full-screen
titles stay on one row and carry the workspace pulse; browse guidance uses one
row while active interactions may use two; transient overlays preserve the host
title; and every important state must report no structured layout-audit issue.
The committed size matrix is exactly 60×20, 72×20, 100×35, and 160×55.

| Terminal size | Presentations and states |
| --- | --- |
| 60×20 | Outliner, Help, Global Notes, Project Hub, and task comments |
| 72×20 | Outliner pending-filter chord |
| 100×35 | Outliner Peek and visual selection |
| 160×55 | Help |

### Assertions

| Assertion | Value | Meaning |
| --- | --- | --- |
| `contains` | substring | The plain screen contains the value. |
| `not_contains` | substring | The plain screen does not contain the value. |
| `count_contains` | substring plus `count` | The plain screen contains the value exactly `count` times. |
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
| Fresh install and protected-Unfiled routing | `fresh-install-default.json`, `fresh-install-capture.json`, `capture-grammar.json` |
| Inbox triage through the unified `s` state/planning chord | `daily-loop.json`, `state-chord.json` |
| Six modes and navigation aliases | `mode-catalog.json`, `navigate-outliner.json`, `outliner-navigation-aliases.json` |
| Header-to-header jumps, wrapping, and the headerless refusal | `outliner-header-jump.json` |
| Priority, workflow, deferred, and clear filters | `filter-chord-priority.json`, `filter-chord-states.json`, `filter-chord-clear.json` |
| Hour-granular deferral presets, `+Nh` typed entry, and the defer-only refusal | `defer-hour-presets.json`, `defer-picker-direct.json`, `defer-custom-entry-bulk.json`, `date-custom-entry.json`, `defer-wake-marker.json` |
| Scope and project administration | `project-hub-*.json`, `project-*.json` |
| Workspace-wide Notes, scope, search, and location pickers | `notes-tool.json`, `notes-scope-picker.json`, `notes-search.json`, `notes-location-pickers.json`, `global-notes-resize-minimum-layout.json` |
| Comments and attachments | `rich-tools.json` |
| Checklist row grammar and its central task-key refusal | `outliner-checklist-row-actions.json`, `outliner-checklist-row-rejects-task-keys.json` |
| Exact three-item `Space` menu | `outliner-space-action-catalog.json` |
| Inline edit validation, autocomplete, atomic cross-project relocation, resize, undo, and redo | `outliner-inline-edit-transaction.json`, `recovery-origin-guidance.json` |
| Capture/edit/outline grammar parity and registry-owned static forms | `capture-grammar.json`, `textgrammar_consumer_contract_test.go`, `quick_edit_test.go`, `outline_editor_test.go` |
| Repeatable `<child` adoption, alias dedupe, additive children, and undo/redo on both surfaces | `capture-child-adoption.json`, `outliner-inline-edit-child-adoption.json` |
| `*pin` at capture and the edit-only `*unpin` clear | `capture-pin.json` |
| Outline editing refusing both structural sigils as competing parentage channels | `outline-editor-guard.json` |
| Full-width cursor-following Peek | `outliner-peek-wide-layout.json`, `outliner-peek-narrow-layout.json` |
| Search projection and exact restoration | `visible-search-minimum-layout.json`, `search-restoration.json` |
| Toggle and range selection with bulk intersection | `outliner-toggle-selection.json`, `outliner-range-selection.json`, `outliner-multiselect-*.json`, `outliner-bulk-action-intersection.json` |
| Project header cursor, folding, and task-key refusal | `outliner-project-header-fold.json`, `outliner-project-header-rejects-task-keys.json` |
| Project tags on the shared `#` key, with parked confirm and recovery | `project-tags-picker.json`, `project-park-confirm-and-recover.json` |
| Project picker multi-select and bulk tag actions | `project-hub-bulk-tag-selection.json` |
| Cleanup plus workspace undo/redo | `cleanup-undo-redo.json` |
| Semantic task tag split, seeded in `#` and usage-derived in `ft` | `semantic-task-tags-split.json`, `tags-picker-direct.json` |
| Semantic colors in both themes | `theme-toggle.json` plus `semantic_tag_title_style_test.go` |
| Deterministic launch and duplicate-submit hardening | `app_deterministic_startup_test.go`, `task_composer_test.go`, `task_form_save_create_test.go`, `task_form_save_edit_test.go` |
| Generated operation help and retired-key absence | `in-app-help.json`, `mode-catalog.json` |
| Responsive and large-catalog rendering | `baseline-layout.json`, `resize-stress.json` |
| Integrated title, guidance, and terminal-size matrix | `presentation-size-matrix.json` |
| Workspace-pulse composition and transient-overlay ownership | `workspace-pulse-presentation-grammar.json` |
| Projects, Notes, forms, task tools, Help, and transient overlays | `presentation-size-matrix.json`, `presentation-surfaces.json`, `rich-tools.json` |
| Correlated undo and relocation-origin feedback | `recovery-origin-guidance.json`, `project-park-confirm-and-recover.json` |
| Nested Focus, bounded search, Esc priority, modes, filters, reload, and external-change recovery | `outliner-focus-esc-priority.json`, `outliner_focus_test.go`, `filter_facets_projection_test.go` |

The full shell gate also runs snapshot, failure-injection, collision,
idempotence, prepared-activation recovery, restore-and-reactivate, and
v4→v5→v6 config tests plus CLI startup recovery. Those tests are the migrated
install scenarios: JSON fixtures begin after workspace construction, so schema
migration and startup recovery stay at the real storage/startup boundary rather
than being simulated inside the TUI driver.

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
