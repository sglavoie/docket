# Behavior guide

The full behavior reference for `tt`. Use it to look up an action or rule — [`../../README.md`](../../README.md) is the front
door. Mechanism lives in [`../architecture.md`](../architecture.md).

| Guide | Covers |
| --- | --- |
| [`outliner.md`](outliner.md) | The index to the four Outliner files below |
| [`outliner-modes.md`](outliner-modes.md) | The seven modes, their order, project grouping, Recent, and the jumps between modes |
| [`outliner-chrome.md`](outliner-chrome.md) | The title row, menu bar, guidance, status strip and advisories, conflict browser, cursor mark, and tag colors |
| [`outliner-keys.md`](outliner-keys.md) | The key table, the pickers those keys open, the `Space` action sheets, and the edit surfaces |
| [`outliner-rows.md`](outliner-rows.md) | Row indicators and their drop order, checklists, Peek, and multi-select |
| [`capture.md`](capture.md) | Everyday capture, editing, routing, and recovery |
| [`capture-reference.md`](capture-reference.md) | Complete sigils, date rules, relationship exceptions, and outline editing |
| [`search-and-lenses.md`](search-and-lenses.md) | Finding work, explaining restrictions, saving views, and sorting |
| [`search-reference.md`](search-reference.md) | Exact matching rules, all filters, lens management, and ordering details |
| [`projects-and-notes.md`](projects-and-notes.md) | Project scope, Select Project, project tags including `parked`, and All Notes |
| [`storage.md`](storage.md) | The durable layout, the undo ledger, conflict recovery, and the migration chain up to the schema-23 catalog and its schema-20 task files |
| [`retired-keys.md`](retired-keys.md) | Every retired key, per surface, and what answers it now |

## Find an answer

Start with the common-action tables in each guide. They give the operation,
keys, and an example before the detailed rules. For everyday work, use the
recipes below; for a symbol on screen, use [Rows and Peek](outliner-rows.md);
for a missing task, use [search and restrictions](search-and-lenses.md).
Rendering budgets and implementation rationale belong in
[the architecture reference](../architecture/interaction-layout-details.md).

## Everyday recipes

**Waiting for someone.** Select the task and press `ww` to mark it Waiting.
Press `@`, then `Tab` twice to reach Check-in, and `Enter` to open its editor.
Choose a next check-in date, optionally a repeat interval, and save with
`Ctrl+S`. When the task needs attention, `R` acknowledges it and advances to
the next candidate. Waiting stays Waiting until you change its state.

**Pause work with a useful restart point.** On the task, press `r`, write a
short note such as “Next: reproduce the failure with an expired token,” and
press `Enter`. Then press `wp` to pause it. Later, `b` shows the note in Peek;
`wg` resumes Doing.

**Keep a review view ready.** Press `/`, type `?review`, and press `Enter`.
Open Named Lenses with `\`, choose `Save Current View`, and name it
“Weekly review.” Reopen that lens whenever you review your work; it shows
flagged tasks and tasks whose check-in is due. Use `R` to acknowledge each
candidate. The name is a reminder of your routine, not an automatic schedule.

**Find completed work.** Press `/` and search `$done` followed by words from
the task, for example `$done login redirect`. Adding `$done` admits completed
tasks to search. Press `b` on a result to inspect its details, and `Esc` to
unwind the view.

**Triage several tasks together.** Press `gi` for Inbox. Move to each task and
press `v` to include it, or use `Ctrl+A` for every visible task. Press `wn` to
plan the selection for Next, or `%` to move it to a project. Read the selection
count before acting: selected tasks hidden by a later filter still belong to
the batch. Press `u` to undo the whole change. See
[selection and bulk actions](outliner-rows.md#peek-and-multi-select).

**Find work that seems to have disappeared.** Open `Q` to read the view's active
restrictions. Press the digit beside one restriction to remove it, or `x` to
peel them all. Search with `?completed` to include finished work or `?deferred`
to include deferred tasks; read any `Also matches` hint for the population still
hidden. See [filters and search](search-and-lenses.md#filters-and-search).

**Recover a mistaken change.** Press `u` to undo or `Ctrl+R` to redo. If the
restored task is visible, the cursor follows it; otherwise the feedback names
where it went. For a visible single-task restoration, `Ctrl+O` returns to your
previous position. Open `Help → Recent Messages` to reread feedback that has
expired. See [the Outliner chrome](outliner-chrome.md).

**Update a saved view.** A `*` after a lens name means its settings have changed.
Open `Q` to see which ones. Then open Named Lenses with `\`, select the lens,
press `Space`, and choose `Replace from Current View`. Check the change summary
before confirming. This replaces the saved settings and cannot be undone with
`u`. See [named lenses](search-and-lenses.md#named-lenses).
