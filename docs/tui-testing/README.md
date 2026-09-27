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
The script only re-selects tests the ordinary suite already runs, so `just ci`
covers the same ground without invoking it.

A scenario never sleeps out one of the app's timers. Each driver owns a virtual
clock, a timer scheduled through `status.TickCmd` reports itself instead of firing, and
a `wait` action advances that clock. Schedule a new timer through `tea.Tick`
directly and a scenario pays its real duration, which is both slow and a race
against the drain budget.

The 60s periodic tick is armed on that virtual clock at launch, so a scenario
reaches it by naming a span (`{ "type": "wait", "duration": "61s" }`) rather than
by waiting. That is the only way to observe something only the tick does — the
outstanding-conflict count, which no local mutation can announce because another
machine's sync wrote the bundle.

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
| `inbox-triage` | The only fixture whose Inbox spans two project groups, the second holding a nested Inbox parent whose child is promoted when the parent is triaged away. |
| `release` | A large Unicode catalog for viewport and resize stress. |
| `parked` | One project in the base task workspace and one tagged `parked`. |
| `project-mix` | An active project with a task, an active project with none, an active project whose two tasks are both finished, a parked project with a task, and an archived project with none. |
| `signals` | A lapsed deferral beside a whole-day and an hour-valued pending one, concentrated overdue work, and a durable Today membership. |
| `stale` | Old and fresh open work for lifecycle-lane placement, the Stale facet, and narrow-width contracts. |
| `check-ins` | A schema-19 workspace activated through the real migration path, with a timed Waiting deadline, unscheduled Doing/Waiting work, and dormant cleanup. |
| `checklist` | The only fixture whose tasks carry checklist items, in the three shapes the fold ladder distinguishes. |
| `ux-acceptance` | Three projects whose axes are simultaneously true — a three-generation subtree, long titles, overdue/today/waiting/deferred/review attention, resume note, description, tags, context, checklist, comment, attachment, a cross-project reference, a pin, and a done task — so one workspace answers a row, a Peek, a selection, and a narrow terminal at once. |

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
| `wait` | optional `duration` | Let time pass: drain pending asynchronous work and advance the virtual clock so due timers fire. `duration` is Go duration syntax (`"90s"`, `"2m"`) and moves the clock only, since the drain budget bounds real work; omit it for the short default. |
| `command` | `command` | Invoke a stable harness or production operation ID. |
| `restart` | none | Rebuild the app from durable state alone, the way relaunching `tt` does. |
| `switch_machine` | optional `machine` | Move machine-local state to another machine's home, then restart, the way opening the same synced workspace on a second machine does. Without `machine` the home is always one no machine has used. |
| `assert` | `assert`, optional `value` | Check a fresh normalized observation. |

An optional `label` describes user intent and is repeated in failure output.
Prefer public keys for acceptance behavior. A `command` action is appropriate
only for deterministic setup or for a deep link deliberately absent from the
public grammar.

A `restart` action re-reads the workspace from disk and constructs a new app over
it, keeping only the terminal size and the virtual clock. Nothing a session held
in memory survives, so anything asserted after a restart is an assertion about
what was actually persisted. Use it for the launch contracts: what a relaunch
reopens with, and what it deliberately does not.

A `switch_machine` action is a restart onto a second machine. Machine identity is
the home directory, so the action moves `$HOME` and relaunches against the same
workspace directory. Omit `machine` and the home is an empty one: the new app
finds no theme, no active-lens pointer, and no machine-local lens copy, and
whatever a scenario still sees is what the portable workspace carried. Use that
for the multi-machine contracts, where the question is which half of the state
travelled.

Give `machine` a name and the action addresses a durable machine instead. The
first switch to a name mints that machine's home; every later switch to the same
name returns to it, with whatever that machine committed still in place. This is
the only way to write "machine A, then machine B, then back to A" — an unnamed
switch always lands somewhere new, so a round trip would look like a third
machine that has never seen the workspace. A name is scoped to the scenario, is
free-form, and is only ever compared for equality.

Accepted key names include printable public keys plus:

```text
tab shift+tab enter space esc
up down left right home end pgup pgdown
ctrl+e ctrl+r ctrl+u ctrl+d ctrl+o ctrl+i ctrl+l ctrl+n ctrl+p ctrl+y ctrl+c
ctrl+f ctrl+b
leftshift rightshift leftctrl rightctrl leftalt rightalt
leftsuper rightsuper leftmeta rightmeta lefthyper righthyper
capslock numlock scrolllock
```

The modifier and lock names on the last three lines are a key pressed by itself,
which a terminal reporting every key as an escape code sends between a chord
prefix and the shifted second key that needs it. Send one to replay what a real
keyboard produces for `f!` or `F#`; the shell drops it before any strategy sees
it, so the chord survives.

The final public grammar is:

```text
Tab/Shift+Tab                    All, Today, Inbox, Next, Backlog, Due, Recent
ga gi gt gn gb g@ gr             go-to chord, one key per mode
1 2 3 4 5 6 7                    the same seven modes, in Tab-cycle order
0                                reset the view to the launch view
j/k h/l gg/G Ctrl+U/Ctrl+D       navigation
zt zz zb                         cursor row to top/middle/bottom of screen
s p t                            fold toggles: sections, project groups, subtrees
n/N                              next/previous grouping header, wrapping
Ctrl+O/Ctrl+I                    older/newer relocated-task positions
Ctrl+L                           Recent, focused on the newest activity
Ctrl+P                           Select Pinned Project
gp                               the cursor task's project, in All
g~ gc                            a related task, and the conflict browser
a/A e/E Ctrl+E O D               capture, edit, and repeat
wi wb wn wt wg ww wp wm wd wc    workflow chord
!0-3 @ # * = r R                 metadata, review, resume note, acknowledge
[ ] > < % J K Ctrl+J Ctrl+K      structure
& ; C ~                          attachments, comments, add comment, references
y Y Ctrl+Y                       clipboard
v V Ctrl+A Space                 selection and actions
Enter Shift+Enter b              Focus, widen focus, and Peek
Ctrl+F/Ctrl+B                    page the open Peek
c x                              add and tick checklist items
M Ctrl+M                         pin the menu bar, open it for one act
P Q \ Ctrl+N T / ? u Ctrl+R Esc q Ctrl+C  global
f!0-3 f@ fa fw f# fc f% f= fh f~ fs f* fx  filters
F# Fp FP Fa FA Fx                project population filters
o@ o! oc ou ot ox or             sort axis
```

The metadata keys are the capture sigils themselves: `@` opens the one picker
holding both the due date and the deferral, and `#` opens the one picker holding
both tags and the task context, so neither `@@` nor `##` is a key sequence. `&`
and `;` are the last two sigils to become keys, and the only pair that opens a
panel rather than a picker: `&link` and `;comment;` write at capture time, and
on a row the same characters open the Attachments and Comments panels that own
add and delete, with reorder and unlink in Attachments alone. `C` and `;` are the same pair once more on a
project header, where both answer for the project rather than for a task. The
old standalone `d` action and comma state prefix are intentionally unbound, as
are `m`, `$`, `:`, `H`, `L`, and `Ctrl+Esc`. `b`, `c`, `t`, `P`, `R`, and `w` are
live again with different acts: Peek, Add Checklist Item, the subtree fold,
Select Project, Mark Reviewed, and the workflow chord. The workflow chord is the
one key that does not spell its sigil: `$destination` is typed, `w` is pressed.

The presentation grammar is also part of the scenario contract. Full-screen
titles stay on one row and carry the workspace pulse; browse guidance uses one
row while active interactions may use two; transient overlays preserve the host
title; and every important state must report no structured layout-audit issue.
The committed size matrix is exactly 60×20, 72×20, 100×35, and 160×55.

| Terminal size | Presentations and states |
| --- | --- |
| 60×20 | Outliner, Help, Global Notes, Project Hub, task comments, and the expanded task form below its width floor |
| 72×20 | Outliner pending-filter chord, and the expanded task form's Extras tab |
| 100×35 | Outliner Peek and visual selection |
| 160×55 | Help |

### Assertions

| Assertion | Value | Meaning |
| --- | --- | --- |
| `contains` | substring | The plain screen contains the value. |
| `not_contains` | substring | The plain screen does not contain the value. |
| `not_clipped` | substring | The plain screen contains the value, and a failure says whether it was absent or truncated. |
| `count_contains` | substring plus `count` | The plain screen contains the value exactly `count` times. |
| `view` | view name | `current_view` equals the value. |
| `status_contains` | substring | The informational/status line contains the value. |
| `no_layout_issues` | none | Rendering has no overflow, UTF-8, or height audit issue. |
| `clipboard_contains` | substring | The text the last copy wrote contains the value. |

Use semantic labels and view names instead of exact full-screen snapshots or
color escape sequences. Add `no_layout_issues` at every important narrow or
resized state.

`not_clipped` reaches the one defect normalization hides. A line a width fitter
truncated still fits the terminal, so it raises no layout issue, and a `contains`
assertion on the full text then fails exactly the way a missing row does.
`not_clipped` returns the same verdict `contains` would and differs only in what
it says when it fails: it names the longest prefix of the value the screen shows
before a `…`, so `screen clips "Restore the overdue billing reconciliation
export" to "Restore the overdue billing reco…"` is distinguishable from `screen
does not contain … , clipped or whole`. Reach for it wherever a title, a chip, or
a guidance line is the thing under test rather than the backdrop. Every
observation also carries `clipped_lines` for the same reason; clipping is legal
today, so it is never a layout issue and `no_layout_issues` keeps its meaning.

A scenario run replaces the process-wide clipboard write for its duration, so
`y`, `Y`, and `Ctrl+Y` land in a buffer `clipboard_contains` reads instead of the
system clipboard. The buffer holds the most recent copy only, and it is empty
until one lands, so an assertion placed before the copy fails rather than
passing vacuously. It is the only way to check *which* tasks a selection copied
and in what order: the status line names the count and the hidden tally, never
the text.

## Final-product coverage

The committed suite exercises the product contracts directly:

| Contract | Representative scenarios |
| --- | --- |
| Fresh install and protected-Unfiled routing | `fresh-install-default.json`, `fresh-install-capture.json`, `capture-grammar.json` |
| Inbox triage through the unified `w` state/planning chord | `daily-loop.json`, `state-chord.json`, `workflow-chord-destinations.json` |
| Cursor recovery after a departure, in every mode: crossing a project group header, landing on a promoted child, walking past the descendants a moved parent carried away to the sibling that stayed, and holding still under undo | `inbox-triage-advance-across-project-groups.json`, `inbox-triage-advance-promotes-children.json`, `relocation-recovery-subtree-move.json`, `inbox-triage-undo-does-not-advance.json` |
| Launch folding Backlog and Someday alone: each header still counting its lane, the cursor on the first task drawn, `l` opening it, and neither `Esc` nor `0` nor anything but a relaunch seeding it again | `outliner-launch-backlog-folded.json` |
| Searching for unprioritized work with `!0`, and its negation keeping only prioritized work | `search-priority-unset.json` |
| The flat ranked list naming each row's project, in its heading color and padded so titles share one column, at the minimum width too, and a single-project scope carrying no label | `outliner-search-ranked-project-label.json` |
| Six modes and navigation aliases | `mode-catalog.json`, `navigate-outliner.json`, `outliner-navigation-aliases.json` |
| Header-to-header jumps, wrapping, and the headerless refusal | `outliner-header-jump.json` |
| Priority, workflow, deferred, and clear filters | `filter-chord-priority.json`, `filter-chord-states.json`, `filter-chord-clear.json` |
| One `@` picker over due and defer, hour-granular deferral presets, `+Nh` typed entry, and the terminal-task defer refusal | `defer-hour-presets.json`, `defer-picker-direct.json`, `defer-custom-entry-bulk.json`, `date-custom-entry.json`, `defer-wake-marker.json`, `date-picker-terminal-task.json`, `due-picker-direct.json` |
| The preset keyboard each field owns, the key every row prints, and `Tab` crossing to the other field's first listed row | `date-picker-preset-keys.json`, `date-picker-section-switch.json` plus `internal/tui/valuepicker/operation_date_preset_key_dates_test.go` for the instant each key commits across both daylight-saving transitions |
| What a preset key leaves behind rather than what it says: due-today planning, a deferral taking that membership with it, both halves surviving a relaunch, and one entry taking either back | `date-picker-planning-persist.json` plus `internal/tui/date_picker_durable_outcomes_test.go` for the stored task, the planning artifact, and the clearing that leaves a membership standing |
| A frozen target set: a cross-project selection committed as one transaction, members hidden by a fold committed with it, and a selection one terminal member disables for the defer field alone | `date-picker-frozen-cross-project-selection.json`, `date-picker-hidden-selected-members.json` plus `internal/tui/date_picker_durable_outcomes_test.go` for a target removed under the picker, refused by name instead of falling back to the cursor, and one completed under it, refused whole against a fresh snapshot |
| Waiting and its check-back date as two transactions, taken back and redone one at a time, and the same picker answering a preset key, a typed date, or `Esc` | `waiting-check-back-transaction-boundary.json` |
| The picker at the minimum width and across a resize: the guidance tail it drops, the keys that still commit unhinted, and the mode digits and fold keys it never hands to the list behind it | `date-picker-narrow-resize-key-confinement.json`, `defer-picker-minimum-layout.json`, `due-picker-minimum-layout.json` |
| Scope and project administration | `project-hub-*.json`, `project-*.json` |
| Workspace-wide Notes, scope, search, and location pickers | `notes-tool.json`, `notes-scope-picker.json`, `notes-search.json`, `notes-location-pickers.json`, `global-notes-resize-minimum-layout.json` |
| Comments and attachments | `rich-tools.json` |
| Attachments direct grammar, its five-row `Space` sheet, and the retired keys | `rich-tools.json`, `action-sheet-picker-cancel.json` |
| `gg`/`Home` and `G`/`End` on Notes, Comments, and Attachments, and the silent abandon of a lone `g` | `notes-ends-navigation.json`, `task-comments-ends-navigation.json`, `task-attachments-ends-navigation.json` |
| Both end spellings named once in each entity surface's own `?` help | `project-hub-help.json`, `notes-help.json`, `task-comments-help.json`, `task-attachments-undo.json` |
| `u` and `Ctrl+R` reaching the one workspace ledger from inside a panel, refreshing its list and naming the transaction | `task-attachments-undo.json`, `task-comments-undo.json` |
| A restore correlated to the one task its transaction was aimed at: the cursor follows a restored task the view draws and offers `Ctrl+O origin`, a restore into an undrawn lane names `now in <Project> (<Lane>)` instead of rewriting the projection, a bulk entry's undo offers neither and keeps the selection, and a relaunch drops the correlation while the ledger still restores | `undo-focus-committed-target.json`, `undo-focus-restart-uncorrelated.json`, `cleanup-undo-redo.json` plus `internal/tui/coordinator/undo_focus_controller_test.go` for the subsequence, restart, and redo-branch cases and `internal/tui/handler_task_undo_identity_test.go` for the shell's two answers |
| The canonical confirmed deletion carrying that same correlation: deleting the one row the cursor named puts the undo back on it, while a two-row selection deleted together stays uncorrelated and leaves the cursor where the user had it | `undo-focus-single-delete.json` plus `internal/tui/task_delete_focus_target_test.go` for the gesture cardinality and the refused-transaction pass-through |
| Checklist row grammar and its central task-key refusal | `outliner-checklist-row-actions.json`, `outliner-checklist-row-rejects-task-keys.json` |
| Single-row `Space` menu, and the `&`/`;` keys that emptied it | `outliner-space-action-catalog.json` |
| The same `Space` resolving to the project sheet on a header, and the enumerated project delete under one undo | `outliner-project-header-actions.json`, `outliner-project-delete-undo.json` |
| The one non-collapsible header a focused project leads with: no count, no fold key, the project acts reachable from it, an empty project keeping it above the explanation, and the cross-project grouping restored by the scope pop | `outliner-focused-project-header.json` |
| Every entity surface's footer and `Space` sheet at the 60x20 minimum | `outliner-multiselect-minimum-layout.json`, `project-hub-minimum-layout.json`, `global-notes-resize-minimum-layout.json`, `task-attachments-minimum-layout.json`, `task-comments-minimum-layout.json` |
| Inline edit validation, autocomplete, atomic cross-project relocation, resize, undo, and redo | `outliner-inline-edit-transaction.json`, `recovery-origin-guidance.json` |
| Capture/edit/outline grammar parity and registry-owned static forms | `capture-grammar.json`, `internal/tui/textgrammar_consumer_contract_test.go`, `internal/tui/quick_edit_test.go`, `internal/tui/outline_editor_test.go` |
| Repeatable `<child` adoption, alias dedupe, additive children, and undo/redo on both surfaces | `capture-child-adoption.json`, `outliner-inline-edit-child-adoption.json` |
| Cross-project `~task` capture and its completion, the `~N` count and both Peek reference directions, the direct dependency search with its `-~task`, `~0`, and unresolvable refusals, inline `-~task` and `~0` under undo and redo, and the locator an unavailable target falls back to | `capture-dependency-cross-project.json`, `search-dependency-direct.json`, `outliner-inline-edit-dependency-undo.json`, `outliner-dependency-unavailable-fallback.json` |
| The row-level `~` picker: the stored set preselected, per-project categories qualifying a row without repeating the name, the source task owning no row, and one commit carrying an addition and a removal together | `outliner-task-reference-picker.json` |
| A stored reference the view cannot read staying an enabled checked row that says only `unavailable`, droppable from the picker, and `~` refusing a visual selection rather than falling back to the cursor | `outliner-task-reference-unavailable-row.json` |
| One `u` taking back both halves of a reference edit, proving the picker commits one ledger entry | `outliner-task-reference-undo.json` |
| The `g~` walk: both halves listed and named, the computed half omitted when nobody points back, cancelling moving nothing, the hop landing in the target's own project, and one Esc handing the departed view back | `goto-relation-walk.json` |
| A visit that changes the workspace: the destination completed and undone while standing on it, and the return resolving the origin's row against the data as it now stands rather than repainting the frame | `goto-relation-return-after-mutation.json` |
| The Esc order around a relationship trail: an open menu and Peek claimed ahead of it, a range costing its own press before the frozen selection it leaves, and the trail spent before the search and the launch view below it | `goto-relation-esc-order.json` |
| Navigation leaving lens state alone: a lens suspended by the hop, handed back whole by the return, dismissed only by its own press, and its definition unchanged by browsing, following, returning, and a relaunch | `goto-relation-lens-round-trip.json` |
| A locator the picker cannot read staying a listed row that says only `unavailable`, the footer naming why it cannot be taken, `Enter` refusing it in place, and the overlay holding at both the widest and the minimum size in either theme | `goto-relation-disabled-row-layout.json` |
| The `=` key writing the same flag as the `=review` sigil, the lane it promotes, and one `u` taking it back, at the minimum and widest sizes in either theme | `review-key-layout-matrix.json`, `review-lane-planning-precedence.json` |
| Relation marker in both directions, reserved across a resize | `outliner-parent-marker.json`, `capture-child-adoption.json` |
| `*pin` at capture and the edit-only `*unpin` clear | `capture-pin.json` |
| `&link` and `;comment;` committing with the task they create, the prose both sigils leave alone, and a quoted `&link` path whose spaces survive capture | `capture-attachment.json`, `capture-comment.json` |
| The expanded form's Extras tab attaching a draft that commits with the task, the whole nested attachment flow completing at the app's own 60-column floor, and the draft surviving repeated resizing | `form-extras-attachments-narrow.json`, `form-extras-attachments-minimum-layout.json` |
| Every form tab and its nested pickers at 60×20: priority, state, flags, due date, and the shared check-in editor each keeping a whole frame, the last Basics field reached by the window, a checklist item added, and a dirty cancel answered | `form-minimum-width-tabs-and-pickers.json` |
| A compact `;comment;` expanding into a seeded Extras tab, a second comment added there, and task, attachment, and comments undone as one entry | `form-extras-comments-narrow.json` |
| Outline editing refusing both structural sigils as competing parentage channels | `outline-editor-guard.json` |
| Full-width cursor-following Peek | `outliner-peek-wide-layout.json`, `outliner-peek-narrow-layout.json` |
| The same `b` over a project: both header forms, the empty project's `No comments yet. Press C to add one.`, a comment refreshing the open pane, undo taking it back, the window paged with `Ctrl+F`/`Ctrl+B` and `Ctrl+D`/`Ctrl+U`, and the pane handed between a project and a task owner | `outliner-project-peek-group-header.json`, `outliner-project-peek-focused-header.json` |
| One run over a project's whole comment life: `C` and `;` answering for an empty project's focused header, the panel editing and deleting through the identity the row carries, `u`/`Ctrl+R` reaching the ledger from inside it, the Peek window paged by both pairs, the same `;` handing a task its own comments, and a relaunch reading the collection back | `project-comments-panel-integration.json` |
| Search projection and exact restoration | `visible-search-minimum-layout.json`, `search-restoration.json` |
| The session-only query ring recalled with `↑`/`↓` inside the `/` input, its draft restored on the way back, and nothing surviving a relaunch | `outliner-search-history-recall.json` |
| Named lenses: the `\` picker, its two action rows at the narrowest width, the three starters, the `1`-`9` quick-apply ordinals and their precedence under a typed digit, save/rename/replace/delete, temporary-search suspension and its restore, exact Escape restoration, retained scope, and the literal backslash inside text input | `named-lens-picker-open.json`, `named-lens-apply-starters.json`, `named-lens-ordinal-apply.json`, `named-lens-save-and-manage.json`, `named-lens-search-suspension.json`, `named-lens-escape-restoration.json`, `named-lens-scope-retention.json`, `named-lens-from-search-results.json`, `named-lens-empty-state.json`, `named-lens-literal-backslash.json` |
| The active lens surviving a relaunch: its saved axes reapplied over the clean launch view, a scoped lens reopening in its project, the first Esc landing where a lens-free launch would have, and a pointer that no longer resolves failing closed in silence | `named-lens-escape-restoration.json`, `named-lens-scoped-restart.json`, `named-lens-restart-dangling.json` |
| A lens definition surviving a machine switch: the workspace carries it to a machine that has never seen it, that machine applies its whole projection, the active-lens pointer stays behind, and a third machine reseeds no starter over the workspace set | `named-lens-machine-switch.json` |
| The built-in view gallery end to end: opened from `Add built-in view`, the seeded starters disabled, search, add-and-apply closing both overlays, and the added template disabled by stable ID on reopening | `named-lens-built-in-gallery.json` |
| Toggle and range selection with bulk intersection | `outliner-toggle-selection.json`, `outliner-range-selection.json`, `outliner-multiselect-*.json`, `outliner-bulk-action-intersection.json` |
| One `%` carrying a whole selection into one destination, under one undo entry, at the default and minimum widths | `outliner-move-to-project-bulk-undo.json`, `outliner-move-to-project-bulk-narrow-layout.json` |
| Project header cursor, folding, and task-key refusal | `outliner-project-header-fold.json`, `outliner-project-header-rejects-task-keys.json` |
| Project tags on the shared `#` key, with parked confirm and recovery | `project-tags-picker.json`, `project-park-confirm-and-recover.json` |
| Project picker multi-select and bulk tag actions | `project-hub-bulk-tag-selection.json` |
| Select Project's compact preview header and the task pane it fills, paged with `Ctrl+F`/`Ctrl+B` and dropped by the compact fallback | `project-hub-task-preview.json` plus `internal/tui/project_hub_preview_tasks_test.go` for the rows the pane lists and `internal/tui/projecthub/project_hub_preview_tasks_test.go` for the paging — the basename exists in both packages |
| The read-only conflict browser on `gc`: the refusal with nothing recorded, the strip's count, the list, the inspected record, `Esc` unwinding one level at a time, and the detail pane scrolling to its closing command at the narrowest width | `conflict-browser-open.json`, `conflict-browser-narrow-layout.json` |
| Cleanup plus workspace undo/redo | `cleanup-undo-redo.json` |
| Semantic task tag split, seeded in `#` and usage-derived in `f#`, which narrows tags alone | `semantic-task-tags-split.json`, `tags-picker-direct.json` |
| Context radio inside `#`, its inline mint, and one undo over a simultaneous tag and context change | `tags-picker-context-radio.json`, `tags-picker-context-mint.json` |
| Task contexts as their own `fc` facet: OR across names, the `No context` row naming the gap, and `f#` staying tag-only beside it | `filter-context-facet.json`, `task-context-tag-facet.json` |
| Projects as their own `f%` row facet: OR across exact slugs, the `%slug` chip and `Q` axis, the seeded reopen, the minimum width, and `fx` | `filter-projects-facet.json` |
| Planning membership outranking the Review lane, so `wn` moves one row | `review-lane-planning-precedence.json` |
| Migration → setup → due attention → early review → recurrence → completion/reopen, with restart-backed persistence | `check-in-integrated-lifecycle.json` |
| Stored flags versus scheduled attention, preserved `=review` and `?stale`, and `fs` cleanup | `check-in-flags-and-stale.json` |
| Calendar midnight, deferred wake, Doing/Today precedence and project Today counts from reloaded task documents | `internal/tui/outliner/check_in_calendar_acceptance_test.go` plus the safe-refresh tests in `internal/tui/defer_wake_refresh_test.go` |
| Dark/light, ordinary/selected rows, monochrome output, editor and setup at 110×35, 62×24 and 60×20 | `internal/tui/check_in_visual_acceptance_test.go` |
| Stale work staying in its lifecycle lane, `fs` outside All, and narrow layout | `stale-lane-and-filter.json` |
| Semantic colors in both themes | `theme-toggle.json` plus `internal/tui/theme/semantic_tag_title_style_test.go` |
| Tag chip order, its `+N` remainder, and the row as a prefix of Peek | `task-tag-chip-order.json` plus `internal/tui/outliner/tag_chip_style_test.go` |
| Deterministic launch and duplicate-submit hardening | `internal/tui/app_deterministic_startup_test.go`, `internal/tui/taskform/task_composer_test.go`, `internal/tui/taskform/task_form_save_create_test.go` |
| Generated operation help and retired-key absence | `in-app-help.json`, `mode-catalog.json` |
| Each surface's `?` naming its own grammar rather than the Outliner's | `project-hub-help.json`, `task-comments-help.json` |
| Responsive and large-catalog rendering | `baseline-layout.json`, `resize-stress.json` |
| Integrated title, guidance, and terminal-size matrix | `presentation-size-matrix.json` |
| Workspace-pulse composition and transient-overlay ownership | `workspace-pulse-presentation-grammar.json` |
| Shell-pinned status strip, its wrap at narrow width, and its clear on a mode change | `outliner-status-strip.json` |
| The ambient-advisory opt-out: the keyless `View → Advisories` row and its checkbox, the conflict count going absent rather than dim, and the choice surviving a relaunch | `advisories-menu-toggle.json` |
| Projects, Notes, forms, task tools, Help, and transient overlays | `presentation-size-matrix.json`, `presentation-surfaces.json`, `rich-tools.json` |
| Correlated undo and relocation-origin feedback | `recovery-origin-guidance.json`, `project-park-confirm-and-recover.json` |
| Nested Focus, bounded search, Esc priority, modes, filters, reload, and external-change recovery | `outliner-focus-esc-priority.json`, `internal/tui/outliner_focus_test.go`, `internal/tui/filter_facets_projection_test.go` |
| One day's whole loop over one workspace: scan, fold, navigate, Peek, capture plain and with metadata, a refused capture, a declined and then accepted discard, every expanded-form tab, inline edit, a lens saved from results and suspended by a search inside it, triage, undo, an abandoned chord, both menus, an empty result, an arrived minute, the `Q` readout opened over a stacked facet and peeled by digit, a start that offers a check-in and the `Esc` that skips it, a completion, and the undo whose `Ctrl+O origin` one key spends | `ux-acceptance-daily-loop.json` plus `internal/tui/ux_presentation_matrix_acceptance_test.go` for the same fixture read at four sizes in dark, light, and monochrome, and `internal/tui/ux_clarity_integrated_acceptance_test.go` for the frame arithmetic under every combination of Peek, the menu bar, and the readout |

The full shell gate also runs snapshot, failure-injection, collision,
idempotence, prepared-activation recovery, restore-and-reactivate, and
v4→v5→v6 config tests plus CLI startup recovery. Those tests are the migrated
install scenarios. The `check-ins` fixture activates historical documents through
the manager before launching its JSON scenarios; recovery fault injection stays
at the real storage/startup boundary.

For visual review, set `TT_CHECK_IN_REVIEW_DIR` to an artifact directory while
running `TestCheckInVisualAcceptance`. It writes the rendered ANSI screens and
the same screens passed through the terminal monochrome profile. Scenario
`wait` advances timer delivery only; exact calendar-boundary assertions use an
explicit `OutlinerModeInput.Now` after reloading stored task documents.

Release compatibility requires actual old-binary execution:
`env TT_OLD_BINARY_REF=08c0cb08fec10154e6d2c9455e32d4c4ffa5f72d just cross-machine`.
That schema-19 binary validates the legacy Waiting seed before the new binary
activates it. The gate then checks one-off review, setup eligibility, early
recurrence, completion/reopen and the preserved timed deadline. The check-in
races run in separate processes in both commit orders: review/reschedule and
terminal/reminder conflicts, independent content/schedule merges, and strict
deadline-step refusal versus mergeable scheduling.

[`ux-refinement-regression-map.md`](ux-refinement-regression-map.md) maps the
planned UX refinement work onto those scenarios and their Go tests: which file
records each behavior today, which scenarios assert an expanded project-header
row rather than merely navigating past one, and why that navigation must stay on
public keys. The four `ux-acceptance-*-baseline.json` scenarios are its recorded
"before"; `ux-acceptance-daily-loop.json` is the integrated "after" that replays
a whole day over the same fixture.

[`ux-clarity-regression-map.md`](ux-clarity-regression-map.md) is the separate
map for the later, independently numbered UX clarity plan, which reads the same
fixture. It names the regression home for each of that plan's sessions, records
the layout measurements and the combined states the plan replaces, and documents
`TT_UX_CLARITY_REVIEW_DIR`, the directory
`internal/tui/ux_clarity_review_matrix_test.go` exports its reviewable ANSI,
plain-text, and monochrome before-frames to. It also records the after-frames,
re-exported once the plan landed, with every moved measurement attributed to the
session that claimed it. The two maps stay separate files because both number
their sessions from one.

[`ux-daily-use-regression-map.md`](ux-daily-use-regression-map.md) is the third
such map, for the thirteen-session daily-use refinement, and it is a third file
for the same reason: eighteen, eight, and thirteen sessions overlap completely,
so no two of those numberings can share a document. It names the regression home
for each of its sessions, records the before-frame layout and clipped-line
measurements, assigns every recorded expectation and every prose contract the
plan replaces to its owning session, and documents
`TT_UX_DAILY_USE_REVIEW_DIR`, the directory
`internal/tui/ux_daily_use_review_matrix_test.go` exports its eight ANSI,
plain-text, and monochrome before-frames to at 60×20, 80×24, 100×35, and
120×40. `internal/tui/ux_daily_use_baseline_test.go` is where the expectations
it replaces are recorded, and `ux_daily_use_regression_map_test.go` is the gate
that holds the map to all of it — including that both earlier maps still carry
their own last session number.

`internal/tui/ux_clarity_integrated_acceptance_test.go` is that plan's
integrated pass, and it asserts only what spans its sessions: that the header,
the menu bar row, the list window, the Peek dock, the status strip, and the
footer add up to exactly the terminal under every combination of those
occupants, that a resize through the floor and through the widest reviewed size
returns the frame byte for byte, and that a title of double-width and combining
characters fits every reviewed terminal while staying wholly readable in Peek.

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
| `clipped_lines` | Every rendered line a width fitter truncated, with its 1-based line number. Not a layout issue: clipping fits the terminal by construction. |
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
