# UX daily-use refinement — regression map

This is the third regression map over the same product, and it is a third file
on purpose. `docs/tui-testing/ux-refinement-regression-map.md` numbers eighteen
sessions and `docs/tui-testing/ux-clarity-regression-map.md` numbers eight; this
plan numbers thirteen, so all three numberings overlap and none of them can be
merged into another without re-addressing contracts that are already settled.
Each map keeps its own enforced gate. The gate for this one is
`ux_daily_use_regression_map_test.go`, which holds it to four claims: every
artifact it names exists, every planned session has at least one regression
home, both earlier maps survive with their last session numbers intact, and
every recorded expectation in
`internal/tui/ux_daily_use_baseline_test.go` is named here by the session that
owns replacing it.

Session 1 owns no later regression home. It establishes the baseline, the review
frames, and this map; sessions 2–13 each own a behavior change and the
expectations that change rewrites.

## Baseline and review evidence

| Artifact | What it is |
| --- | --- |
| `internal/tui/ux_daily_use_baseline_test.go` | The eleven recorded expectations. Each one describes current behavior a named later session replaces. |
| `internal/tui/ux_daily_use_review_matrix_test.go` | The eight review frames, exported at four sizes in dark, light, and monochrome. |
| `ux_daily_use_regression_map_test.go` | The gate over this document. |
| `internal/tui/uitest_fixtures_registry_test.go` | The `ux-acceptance`, `narrow`, `multi-project`, and `release` fixtures every frame and every recorded expectation reuses. |

The aggregate race baseline was completed for this plan with the CI timeout
convention — `go test -race -count=1 -timeout=0 ./...` — and every package
passed. The per-package deadline has to be disabled because `internal/tui` alone
runs 874s under `-race`, with `internal/config` at 333s and `internal/storage` at
98s; the planning baseline's failure was that ten-minute deadline and not a
defect. `scripts/ci.sh` lines 22–38 are where that convention already lives, and
each session contract keeps the 3600-second outer bound.

### Exporting the before-frames

`TestUXDailyUseReviewMatrix` asserts its invariants with or without an export
directory. Set `TT_UX_DAILY_USE_REVIEW_DIR` to write the frames:

```sh
TT_UX_DAILY_USE_REVIEW_DIR=/tmp/ux-daily-use-before \
  go test ./internal/tui/ -run TestUXDailyUseReviewMatrix -count=1
```

Each frame writes three files per variant and size — `<name>.ansi` with styling
intact, `<name>.txt` plain, and `<name>.mono.txt` with the styled frame projected
through the ASCII color profile — plus one `frames.tsv` manifest naming the
fixture, the variant, the size, the sessions that replace the frame, and the
measured header, content, Peek-dock, footer, and clipped-line counts. The export
is reproducible: one isolated fixture driver per frame, no shared workspace, and
no dependence on the order the frames are staged in.

### The eight frames

| Frame | Fixture | Records | Replaced by |
| --- | --- | --- | --- |
| `startup-focus` | multi-project | The launch view, before any key press. | 11, 12 |
| `project-label-widths` | multi-project | Rows whose project names are of unequal length, with the cursor on a task. | 2, 3 |
| `dense-scrolled` | release | A long list scrolled to its end, where row geometry is under the most pressure. | 2, 3, 6 |
| `hidden-selection` | ux-acceptance | A visual selection whose members are partly folded away. | 4 |
| `workflow-chord` | ux-acceptance | The `w` chord's ten destinations as one flat run. | 5 |
| `capture-panel` | narrow | The compact capture panel over a live host, holding a three-line draft. | 6, 7 |
| `bulk-tag-picker` | ux-acceptance | The task-tag picker over a multi-task selection, with mixed values. | 8 |
| `check-in-offer` | ux-acceptance | The check-in editor raised automatically by a state change. | 9, 10 |

## Layout measurements before the plan

Rows measured from the exported frames, as `header + content + footer`. Light
matches dark exactly in every count, so one column per size serves both; the
Peek dock is closed in all thirty-two frames, so its reservation is zero
throughout.

| Frame | 60×20 | 80×24 | 100×35 | 120×40 |
| --- | --- | --- | --- | --- |
| `startup-focus` | 1 + 17 + 2 | 1 + 21 + 2 | 1 + 32 + 2 | 1 + 37 + 2 |
| `project-label-widths` | 1 + 17 + 2 | 1 + 21 + 2 | 1 + 32 + 2 | 1 + 37 + 2 |
| `dense-scrolled` | 1 + 17 + 2 | 1 + 21 + 2 | 1 + 32 + 2 | 1 + 37 + 2 |
| `hidden-selection` | 2 + 16 + 5 | 2 + 20 + 5 | 2 + 31 + 4 | 2 + 36 + 4 |
| `workflow-chord` | 1 + 17 + 3 | 1 + 21 + 3 | 1 + 32 + 3 | 1 + 37 + 3 |
| `capture-panel` | 1 + 9 + 3 | 1 + 13 + 3 | 1 + 19 + 3 | 1 + 23 + 3 |
| `bulk-tag-picker` | 2 + 16 + 5 | 2 + 20 + 5 | 2 + 31 + 4 | 2 + 36 + 4 |
| `check-in-offer` | 1 + 17 + 3 | 1 + 21 + 3 | 1 + 32 + 3 | 1 + 37 + 3 |

Clipped lines, which is where the plan's row-budget work will be visible:

| Frame | 60×20 | 80×24 | 100×35 | 120×40 |
| --- | --- | --- | --- | --- |
| `startup-focus` | 0 | 0 | 0 | 0 |
| `project-label-widths` | 0 | 0 | 0 | 0 |
| `dense-scrolled` | 1 | 2 | 3 | 3 |
| `hidden-selection` | 0 | 0 | 0 | 0 |
| `workflow-chord` | 1 | 1 | 1 | 1 |
| `capture-panel` | 2 | 2 | 2 | 2 |
| `bulk-tag-picker` | 4 | 2 | 2 | 6 |
| `check-in-offer` | 3 | 6 | 1 | 1 |

No frame reports a layout issue at any of the four sizes, in any variant, and no
monochrome frame loses content its styled counterpart carried. Those two are
asserted rather than tabulated, in `TestUXDailyUseReviewMatrix`.

## Layout measurements after the plan

Session 13 fills this section from the same export, run against the finished
product, so the two tables above are the only before-evidence it needs.

## Expectations later sessions deliberately replace

Each row is a test that asserts what the product does today because the plan
changes it. The owning session rewrites the assertion; anyone else who hits one
has found a regression.

| Recorded expectation | Test | Owner |
| --- | --- | --- |
| ~~A task row names its project in a `[label]` **ahead** of the title, and title start columns therefore vary with project-name length.~~ Rewritten by session 2: the label follows the title in one measured column, so every title and every label starts in one column. | `TestUXDailyUseProjectLabelFollowsTheTitleInOneColumn` | 2 |
| ~~The cursor row is marked in its first column and trailing fill alone: the bytes from the gutter through the title are identical focused and unfocused.~~ Rewritten by session 3: a task row also wears one band under everything it drew, and the assertion is now that every foreground the unfocused row carried still reads on the focused one. | `TestUXDailyUseCursorRowWearsOneBandUnderItsContent` | 3 |
| A visual selection with hidden members is counted **twice** — once in the status strip and again in the footer. | `TestUXDailyUseSelectionCountsAreStatedTwice` | 4 |
| The `w` chord lists all ten destinations as one flat run, with no Plan/Status division. | `TestUXDailyUseWorkflowChordListsOneFlatRun` | 5 |
| Opening and closing the capture panel moves `ViewScrollOffset` even though the cursor never moved, so the round trip loses the scroll window. | `TestUXDailyUseHostedPanelResizeLosesTheScrollWindow` | 6 |
| The capture panel's divider sits at a fixed row regardless of how many lines the draft holds. | `TestUXDailyUseCapturePanelHeightIgnoresItsDraft` | 7 |
| The single-task tag picker lists already-applied values **last**. | `TestUXDailyUseTagPickerListsAppliedValuesLast` | 8 |
| The bulk tag picker lists partially-applied (`[-]`) values **last**. | `TestUXDailyUseBulkTagPickerListsMixedValuesLast` | 8 |
| Entering Doing raises the full check-in editor automatically, interrupting the state change. | `TestUXDailyUseStateChangeRaisesTheCheckInEditor` | 9, 10 |
| A fresh launch seats the cursor on row zero, which is a section header rather than a task. | `TestUXDailyUseLaunchSeatsTheCursorOnASectionHeader` | 11 |
| Launch guidance names navigation only — no `w workflow`, `Space actions`, or `a capture`. | `TestUXDailyUseLaunchGuidanceNamesNavigationOnly` | 12 |

### Documentation assertions

Prose contracts are replaced by the same rule: the session that changes the
behavior rewrites the sentence and the gate that requires it, in the same diff.

| Assertion | Where | Owner |
| --- | --- | --- |
| ~~"each of its task rows carries a compact `[label]` ahead of the title"~~ Rewritten by session 2 to "in a column after the title". | `docs/architecture/tui-architecture.md` line 197 | 2 |
| ~~The required string "one compact `[label]` segment per task row, from `taskProjectLabel`…"~~ Rewritten by session 2 to the column form, with `ComputeOutlinerRowLayout` and the clamp's direction added beside it. | `architecture_docs_test.go` lines 97–105 | 2 |
| ~~"`taskProjectLabel` is reserved on those same terms and joins no ladder… the labels while it is expanded"~~ Rewritten by session 2, which also added the `ComputeOutlinerRowLayout` geometry rule and raised the budget for it. | `CLAUDE.md` lines 627–649 | 2 |
| ~~"`outlinerCursorRow` marks the cursor row in its first column and trailing fill alone, not its content"~~ Rewritten by session 3 to the band and its background precedence, and the budget raised for it. | `CLAUDE.md` line 532 | 3 |
| ~~The cursor-row paint description — `cursorPaint`, the gutter truncation, the `trail()` pad~~ Rewritten by session 3, which added `rowBg`/`overTaskRow`, the band's derivation, `textfit.PaintBackground`, and why the bar is an attribute. | `docs/architecture/presentation-chrome.md` lines 479–512 | 3 |
| "A view status is transient except the multi-select summary." | `CLAUDE.md` line 410 | 4 |
| The multi-select summary as a second statement of an active selection | `docs/architecture/row-indicators.md` line 381; `docs/architecture/presentation-chrome.md` line 462 | 4 |
| The workflow chord row listing ten destinations flat, and "entering Doing or Waiting without a schedule offers an optional check-in" | `docs/guide/outliner-keys.md` line 12 | 5, 10 |
| "`HostRows` is a pure function of the frame — never of the cursor, the draft, or the document" | `CLAUDE.md` line 363 | 6, 7 |
| The `SEMANTIC` picker row precedence and `OrderedTaskTags` as the single chip order | `CLAUDE.md` lines 800, 816 | 8 |
| "`ValueProviderCheckIn` opens one two-field draft…" and the `taskform.CheckInEditorModel` value step | `CLAUDE.md` lines 106–111; `docs/architecture/operation-grammar.md` lines 35–45, 484 | 9, 10 |
| "Every launch starts Active Projects, All, with no restored navigation or selection" | `CLAUDE.md` lines 127–131 | 11 |
| "the class `?` help and the footer ranks belong to" | `CLAUDE.md` line 193 | 12 |

`contributor_guidance_docs_test.go` holds `CLAUDE.md` to a 68500-byte rules-only
budget. A session that spends more of that budget than it reclaims raises the
constant in its own diff and says why in the comment ledger above it, the way
every previous raise did; nobody raises it on another session's behalf.

## Per-session regression homes

### Session 2 — Trailing project column and task-row geometry

- Go: `internal/tui/ux_daily_use_baseline_test.go`, `internal/tui/outliner/outliner_project_label_outlinersplit_test.go`, `internal/tui/outliner/outliner_row_budget_acceptance_test.go`, `internal/tui/inline_row_editor_test.go`
- Frames: `project-label-widths`, `dense-scrolled`
- Contracts: `architecture_docs_test.go` lines 97–105; `docs/architecture/tui-architecture.md` line 197; `CLAUDE.md` lines 627–649
- Landed: `chrome.ColumnLayout.ProjectColumn` and `OutlinerModel.ComputeOutlinerRowLayout` — one geometry measured over the whole projection; `taskRowSegments` split into `preTitleWidth`/`postTitleWidth` with the project field between the cues and the indicators; `projectLabelBudget` holding the label at `projectLabelWidth` up to `projectLabelGrowthFloorWidth` and growing it toward `projectLabelCeilingWidth` past that; `RenderOutlinerQuickEditRow` and the Hub preview painter taking that same layout.

### Session 3 — Current-row tint and semantic colors

- Go: `internal/tui/outliner_cursor_highlight_test.go`, `internal/tui/attention_styling_acceptance_test.go`, `internal/tui/primary_row_title_style_test.go`, `internal/tui/chrome/cursor_test.go`, `internal/tui/theme/styles_palette_test.go`, `internal/tui/textfit/background_test.go`, `internal/tui/outliner/outliner_render_rows_test.go`, `internal/tui/outliner/outliner_search_cue_outlinersplit_test.go`, `internal/tui/ux_daily_use_baseline_test.go`
- Frames: `project-label-widths`, `dense-scrolled`
- Contracts: `CLAUDE.md` line 532; `docs/architecture/presentation-chrome.md` lines 479–512
- Landed: `theme.ColorCursorRowBg` derived in `ApplyTheme` a short step from the terminal's own paper, `theme.CursorBarStyle` switched to inverse video so the mark survives a profile carrying no color, `cursorPaint.rowBg` with `overTaskRow()` as the one opt-in and `gutterBg()`/`trail()` reading it, `textfit.PaintBackground` arming a background that re-arms after every reset so no foreground is rewritten, and `applyTaskRowLineStyle` returning ahead of `taskRowTint` and `ZebraLine`, which is the whole of background precedence.

### Session 4 — One selection summary and accurate frame budgets

- Go: `internal/tui/outliner_multiselect_layout_test.go`, `internal/tui/chrome/frame_budget_test.go`, `internal/tui/outliner_range_selection_test.go`, `internal/tui/ux_daily_use_baseline_test.go`
- Frames: `hidden-selection`
- Contracts: `CLAUDE.md` line 410; `docs/architecture/row-indicators.md` line 381; `docs/architecture/presentation-chrome.md` line 462
- Landed: —

### Session 5 — Plan and Status presentation

- Go: `internal/tui/footer_ranked_guidance_test.go`, `internal/tui/outliner/workflow_state_group_presentation_test.go`, `internal/tui/ux_daily_use_baseline_test.go`
- Frames: `workflow-chord`
- Contracts: `docs/guide/outliner-keys.md` line 12
- Landed: —

### Session 6 — Viewport-preserving hosted-panel resizing

- Go: `internal/tui/chrome/hosted_panel_test.go`, `internal/tui/app_handlers_view_capture_test.go`, `internal/tui/ux_daily_use_baseline_test.go`
- Frames: `dense-scrolled`, `capture-panel`
- Contracts: `CLAUDE.md` line 363
- Landed: —

### Session 7 — Content-sized compact capture

- Go: `internal/tui/capture_footer_test.go`, `internal/tui/chrome/capture_footer_test.go`, `internal/tui/quick_capture_interrupt_test.go`, `internal/tui/ux_daily_use_baseline_test.go`
- Frames: `capture-panel`
- Contracts: `CLAUDE.md` line 363
- Landed: —

### Session 8 — Applied-first task tags and context

- Go: `internal/tui/task_tags_picker_semantic_test.go`, `internal/tui/bulk_picker_test_helpers_test.go`, `internal/tui/project_tags_picker_test.go`, `internal/tui/ux_daily_use_baseline_test.go`
- Frames: `bulk-tag-picker`
- Contracts: `CLAUDE.md` lines 800, 816
- Landed: —

### Session 9 — Independent check-in editor ownership safeguards

- Go: `internal/tui/task_check_in_editor_test.go`, `internal/tui/check_in_setup_test.go`, `internal/tui/ux_daily_use_baseline_test.go`
- Frames: `check-in-offer`
- Contracts: `CLAUDE.md` lines 106–111; `docs/architecture/operation-grammar.md` lines 35–45
- Landed: —

### Session 10 — Non-interrupting check-in feedback across commit paths

- Go: `internal/tui/task_check_in_offer_test.go`, `internal/tui/check_in_visual_acceptance_test.go`, `internal/tui/footer_flash_test.go`, `internal/tui/ux_daily_use_baseline_test.go`
- Frames: `check-in-offer`
- Contracts: `docs/guide/outliner-keys.md` line 12; `docs/architecture/operation-grammar.md` line 484
- Landed: —

### Session 11 — First-task launch selection

- Go: `internal/tui/app_launch_lens_test.go`, `internal/tui/outliner/outliner_primary_rows_test.go`, `internal/tui/ux_daily_use_baseline_test.go`
- Frames: `startup-focus`
- Contracts: `CLAUDE.md` lines 127–131
- Landed: —

### Session 12 — Contextual guidance priorities

- Go: `internal/tui/outliner_guidance_test.go`, `internal/tui/footer_ranked_guidance_test.go`, `internal/tui/ux_daily_use_baseline_test.go`
- Frames: `startup-focus`
- Contracts: `CLAUDE.md` line 193
- Landed: —

### Session 13 — Integrated acceptance and final review

- Go: `internal/tui/ux_daily_use_review_matrix_test.go`, `internal/tui/ux_presentation_matrix_acceptance_test.go`, `ux_daily_use_regression_map_test.go`
- Frames: all eight, re-exported into "Layout measurements after the plan"
- Contracts: `just ci`, which is this plan's only session-level gate wider than the aggregate race command
- Landed: —
