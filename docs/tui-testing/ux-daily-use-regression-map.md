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
| ~~A visual selection with hidden members is counted **twice** — once in the status strip and again in the footer.~~ Rewritten by session 4: the context row is the sole persistent summary, the status channel says nothing about a selection, the hidden count survives into the compact spelling, and `Q` keeps all three numbers. | `TestUXDailyUseSelectionCountsAreStatedOnce` | 4 |
| ~~The `w` chord lists all ten destinations as one flat run, with no Plan/Status division.~~ Rewritten by session 5: the row heads `n next` and `t today` with `Plan:` and the eight lifecycle states with `Status:`, Plan leading, and the assertion is now where that division falls rather than that it is absent. | `TestUXDailyUseWorkflowChordDividesPlanFromStatus` | 5 |
| ~~Opening and closing the capture panel moves `ViewScrollOffset` even though the cursor never moved, so the round trip loses the scroll window.~~ Rewritten by session 6: the rows a panel takes are a loan rather than a resize, so the close hands back the window the open borrowed from, and the assertion is now that the offset is unchanged. | `TestUXDailyUseHostedPanelResizeKeepsTheScrollWindow` | 6 |
| ~~The capture panel's divider sits at a fixed row regardless of how many lines the draft holds.~~ Rewritten by session 7: the panel is sized by the content it holds, so the divider rises a row per draft line until the 14-row ceiling stops it, and the assertion is now that difference, the host's four-row floor, and that growth kept the cursor's line on screen. | `TestUXDailyUseCapturePanelIsSizedByItsContent` | 7 |
| ~~The single-task tag picker lists already-applied values **last**.~~ Rewritten by session 8: the values the task carries are promoted to an `APPLIED` section that leads the list, and the assertion is now that they lead, that a value checked while the picker is open does not join them, and that a query rearranges neither. | `TestUXDailyUseTagPickerLeadsWithAppliedValues` | 8 |
| ~~The bulk tag picker lists partially-applied (`[-]`) values **last**.~~ Rewritten by session 8: partially applied values lead beside universally applied ones while keeping their own mark, and the clear-context row every untagged target agrees on stays in `CONTEXTS` rather than being promoted. | `TestUXDailyUseBulkTagPickerLeadsWithMixedValues` | 8 |
| ~~Entering Doing raises the full check-in editor automatically, interrupting the state change.~~ Rewritten by session 10: the commit says what it left unscheduled and opens nothing, so the assertion is now that the footer names the task and the keyless `Set up check-ins` entry, that no editor and no `Esc skips` notice appear, and that driving that entry is still what raises the draft. | `TestUXDailyUseStateChangeNotesTheMissingCheckIn` | 9, 10 |
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
| ~~"A view status is transient except the multi-select summary."~~ Rewritten by session 4: every view status is transient, the Outliner writes none about a selection, and the context row's `selection` entry is the one persistent summary. | `CLAUDE.md` line 409 | 4 |
| ~~The multi-select summary as a second statement of an active selection~~ Rewritten by session 4: the sticky-`SetStatus` exception is gone, and the entry's own contract — hidden count in both spellings, whole breakdown in `Q` — is written down beside the other context entries instead. | `docs/architecture/row-indicators.md` line 422; `docs/architecture/presentation-chrome.md` line 36 | 4 |
| The workflow chord row listing ten destinations flat, and "entering Doing or Waiting without a schedule offers an optional check-in" | `docs/guide/outliner-keys.md` line 12 | 5, 10 |
| ~~"`HostRows` is a pure function of the frame — never of the cursor, the draft, or the document"~~ Rewritten by session 7: `HostRows` is the frame less what the panel's content asks for, bounded by the panel's ceiling and the host's floor, and the settled-state rule that keeps the measurement from becoming a feedback loop is written down beside it, with the budget raised for both. The same session retired the divider's destination half. | `CLAUDE.md` lines 372–381; `internal/tui/chrome/hosted_panel.go` line 12 | 6, 7 |
| ~~"because `OutlinerModel.SetSize` drops `ViewScrollOffset`"~~ — the rationale that followed the row above, retired by session 6: the rows a panel takes are a loan rather than a resize, `route.HostedBudgetView` is the seam, and `SetSize` dropping the window is now what ends a loan rather than what justifies losing one. The purity claim itself survives untouched and stays session 7's. | `CLAUDE.md` lines 373–383 | 6 |
| The `SEMANTIC` picker row precedence and `OrderedTaskTags` as the single chip order | `CLAUDE.md` lines 800, 816 | 8 |
| "`ValueProviderCheckIn` opens one two-field draft…" and the `taskform.CheckInEditorModel` value step | `CLAUDE.md` lines 106–111; `docs/architecture/operation-grammar.md` lines 35–45, 484 | 9, 10 |
| "Every launch starts Active Projects, All, with no restored navigation or selection" | `CLAUDE.md` lines 127–131 | 11 |
| "the class `?` help and the footer ranks belong to" | `CLAUDE.md` line 193 | 12 |

`contributor_guidance_docs_test.go` holds `CLAUDE.md` to a 70500-byte rules-only
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

- Go: `internal/tui/outliner_multiselect_layout_test.go`, `internal/tui/chrome/frame_budget_test.go`, `internal/tui/outliner_range_selection_test.go`, `internal/tui/query_readout_test.go`, `internal/tui/ux_daily_use_baseline_test.go`
- Frames: `hidden-selection`
- Contracts: `CLAUDE.md` line 409; `docs/architecture/row-indicators.md` line 422; `docs/architecture/presentation-chrome.md` line 36; `docs/guide/outliner-chrome.md` line 175; `docs/guide/outliner-rows.md` line 340
- Landed: `OutlinerModel.UpdateVisualSelectionStatus` and its five call sites removed, along with the `setStatus` wrapper and the `ClearMultiSelect` alias, so no Outliner path writes a sticky status any more; `selectionContextEntry` rewritten to state `N selected` with `(M hidden)` retained in `RowCompact` whenever the hidden count is nonzero, and its `Value` made the unconditional total/visible/hidden breakdown `Q` reads; the exit paths no longer call `ClearStatus`, so leaving a selection leaves unrelated operation feedback standing. The frame budget needed no edit: `contextRowRows()` already charged the context row under `MultiSelectMode` and `StatusStripHeight` derives from `StatusStripBody`, so the row the strip stopped spending flowed to `ComputeOutlinerViewport` by construction — which `TestUXClarityIntegratedFrameSpendsEveryRowOnce` proves over the `combined-context` frame. Removing the sticky write did expose one measurement that had been reading the right window by accident: `PeelQueryReadoutRow` re-seated its offset between the axis write and the status it publishes, and only the selection summary sitting in the channel had been keeping that strip the right height. `reseatQueryReadoutAfterPeel` now runs after the status, since the render's clamp can only reduce an offset further and a position lost to a too-large window is lost for good; `TestQueryReadoutPeelReseatsAgainstTheStripItWillDraw` holds the ordering. `Q`'s selection value stayed conditional on a nonzero hidden count for the same budget reason: with nothing hidden the visible half is the total restated, and spelling it cost the readout two wrapped rows at 60 columns.

### Session 5 — Plan and Status presentation

- Go: `internal/tui/footer_ranked_guidance_test.go`, `internal/tui/outliner/workflow_state_group_presentation_test.go`, `internal/tui/ux_daily_use_baseline_test.go`, `internal/tui/taskform/task_composer_test.go`, `internal/tui/outliner/inline_preview_test.go`
- Frames: `workflow-chord`
- Contracts: `docs/guide/outliner-keys.md` line 12; `docs/guide/outliner-rows.md` line 267; `docs/guide/capture.md` line 144; `docs/architecture/presentation-chrome.md` line 74; `docs/architecture/operation-grammar.md` line 785; `docs/architecture/search-filters-and-sorting.md` line 394; `docs/architecture/capture-and-outline-editor.md` line 97
- Landed: `internal/tui/chrome/plan_status_vocabulary.go` as the one home of the two words — `PlanFieldLabel`/`StatusFieldLabel` for a labelled field, `PlanFieldKey`/`StatusFieldKey` for Peek's lowercase `label value` idiom, `PlanHintGroup`/`StatusHintGroup` for a which-key heading, and `PlanStatusValue` as the single place a canonical lowercase token is capitalized — so four surfaces cannot drift into four synonyms. `OperationKeyBinding.HintGroup` carries the division on the one declaration that already carries the sequence, which is why no dispatch path, operation identity, or key changed: the ten `w` terminals were only reordered so Plan's two lead. `ChordHint.Group` is stamped on every member by `DecoratedChordHintsForPrefix`, and `pendingChordGuidanceSpec` turns a run into a heading by emitting one only where the group changes. `chrome.GuidanceHint.Group` is a field on the entry rather than an entry of its own, because the packer stops at the first optional hint that will not fit and a standalone heading could therefore outlive every member under it; riding the first member makes the pair pack, fit, and drop as one thing, which `TestPendingChordGuidanceNeverStrandsADivisionHeading` sweeps every width from 60 to 160 to prove. The division costs one entry at the 60-column floor — `d done` and `c canceled` are shed where only `c canceled` was before — and nothing at the two baseline sizes, where all ten still fit. On the capture side `renderComposerDestination` is gone and `effectiveRouteHeader` returns three independently aggregated axes, so the route row reads `Project: X · Plan: Today · Status: Backlog` with the plan half written only under `Destination.IsPlanning()` and `Mixed` confined to the one axis a multiline draft disagrees about; the row dropped its icon and `$` token because no sigil can spell `Mixed`, while `quick_capture_preview.go` keeps that echo and appends the same two labelled chunks under the same gate, which is what left `capture-defer-sigil` and `capture-review` passing untouched. `inlinePeekStatusLine` now leads with `plan` and follows with `status` (`inlinePeekStateValue` renamed to `inlinePeekStatusValue`), since `today` standing alone in the old trailing position read as a tenth state.

### Session 6 — Viewport-preserving hosted-panel resizing

- Go: `internal/tui/outliner/outliner_host_budget_test.go`, `internal/tui/app_handlers_view_capture_test.go`, `internal/tui/ux_daily_use_baseline_test.go`
- Frames: `dense-scrolled`, `capture-panel`
- Contracts: `CLAUDE.md` lines 373–383; `docs/architecture/capture-and-outline-editor.md`; `docs/architecture/presentation-chrome.md`
- Landed: `route.HostedBudgetView` — `SetHostedBudget` and `ReleaseHostedBudget` — as the seam the shell hands a bounded panel's leftover rows through, beside `ShellFrameAware` and for the same reason it exists: a list surface drops its scroll window every time it is resized, so a budget that travels through `SetSize` is indistinguishable from a terminal that changed size. `applyHostedBudget`/`releaseHostedBudget` in `app_view.go` are the one dispatch, so `syncHostedPanelHost` and `releaseHostedPanelHost` each kept their single call site and every host implementing neither is resized exactly as before. The outline answers both through `hostBudgetAnchor`, captured once at the first budget and re-seated on every later one, which is what makes a panel growing with its draft, a syntax guide opening, and an expanded form collapsing back one loan with one repayment instead of a drift whichever height came last decided. The anchor is a position rather than a number — the top task row's qualified identity, the cursor's behind it, the held offset behind that — because capture commits work into the projection underneath it, and a window restored by row number would slide the document under a user who had just captured above their position. `seatWindowAtAnchor` writes `ViewScrollOffset` and nothing else: keeping the cursor visible and cutting the window to the document stay `ComputeOutlinerViewport`'s, which runs on the very frame the seat is for, so the shortened window still clamps no further than the cursor requires and nothing here is a state rollback — folds, scope, query, selection, and the rows themselves are never read or written. `SetSize` drops the anchor with the window, so a terminal resized while a panel is open ends the loan rather than resurrecting one it already dropped. The cluster lives in `outliner.go` beside `SetSize` rather than in a file of its own, because the package was at its forty-file ceiling and the two acts are the same concern read twice.

### Session 7 — Content-sized compact capture

- Go: `internal/tui/taskform/task_composer_test.go`, `internal/tui/app_view_test.go`, `internal/tui/capture_footer_test.go`, `internal/tui/chrome/capture_footer_test.go`, `internal/tui/quick_capture_interrupt_test.go`, `internal/tui/ux_daily_use_baseline_test.go`, `internal/tui/ux_clarity_baseline_test.go`
- Frames: `capture-panel`
- Contracts: `CLAUDE.md` lines 367–392; `internal/tui/chrome/hosted_panel.go`; `docs/architecture/capture-and-outline-editor.md`; `docs/architecture/presentation-chrome.md`; `docs/guide/capture.md`
- Landed: `composerPanelContentRows` is the panel's demand — `composerPanelFixedRows` for the divider and the route row, the input's own wrapped line count, the error block, and the pane underneath (the suggestion list's items, one row for a multiline draft's summary, otherwise the grouped preview through the new `grammar.QuickCaptureInput.PreviewBlockRows`) — and `composerPanelShareRows` is what the host is told, bounded by `composerPanelMinRows` below, `composerPanelMaxRows` above, and `composerHostMinRows` kept for the host. The old frame-only `composerPanelRows` and its five chrome rows are gone, so a blank capture is three rows instead of eight and the outline keeps five it used to lend. Every demand term reads settled state — `Input.Value()` wrapped at `Input.Width()`, the recorded `Errors`, `len(AC.Items)`, the stored parse — so nothing re-parses during measurement or render and no term reads the panel's own height, which is the loop a content-sized panel could otherwise close. The shell's footer excess rides the share rather than the drawn rows, because taking it out of the panel's own height would drop the row the content asked for; `compactPanelRows` subtracts it again for the render, so the panel draws exactly what it measured and the host is charged for the footer above it through the loan `hostBudgetAnchor` keeps a window across. `HelpExpanded` deliberately moves no term, since measuring the open syntax guide as nothing shrank the share and cost the host the window session 6 had just bought. `composerInputRows` is the in-panel priority: the input keeps its first row unconditionally and yields the rest to refusals, so a message naming what went wrong never scrolls off a full panel, and the bounded preview and autocomplete paths are unchanged behind the same fourteen-row ceiling they were written for. The divider dropped its destination half — `HeaderLabel` is now `Capture`, the footer's `Supplemental` fallback is gone — because the route row states the project and `Ctrl+P` rewrites it, so the old label was both a repetition and the copy that went stale; a child capture keeps `Capture → child of …`, the one value no axis of that row carries.

### Session 8 — Applied-first task tags and context

- Go: `internal/tui/task_tags_picker_semantic_test.go`, `internal/tui/bulk_picker_test_helpers_test.go`, `internal/tui/project_tags_picker_test.go`, `internal/tui/ux_daily_use_baseline_test.go`
- Frames: `bulk-tag-picker`
- Contracts: `CLAUDE.md` lines 800, 816
- Landed: —

### Session 9 — Independent check-in editor ownership safeguards

- Go: `internal/tui/task_check_in_editor_test.go`, `internal/tui/check_in_setup_test.go`, `internal/tui/ux_daily_use_baseline_test.go`
- Frames: `check-in-offer`
- Contracts: `docs/architecture/operation-grammar.md`, the interaction-ownership paragraph under optional check-in setup
- Landed: `interactionOwnership` in `internal/tui/app_update.go` is the asynchronous safeguard as its own state: one count of which user interaction owns the work in flight, holding no queue, no draft, and no pending offer, where `checkInOfferController.epoch` previously was both that count and the offer's cancellation flag. The envelopes moved with it and lost the offer's name — `checkInOfferCommandMsg`/`checkInOfferOriginMsg` are `originTaggedCommandMsg`/`commandOriginMsg`, `withCheckInOfferOrigin`/`runCheckInOfferCommand` are `withCommandOrigin`/`runOriginTaggedCommand`, `beginCheckInOfferUpdate` is `beginCommandOriginUpdate`, and `checkInOfferInput` is `isInteractionInput` — so the deterministic runner in `internal/tui/uitest_runner_test.go` unwraps an envelope named for what it carries rather than for one consumer of it. `originTaggedMessage` is the tagged set and names `operationhost.CheckInRejectedMsg` on its own terms, delegating the committed evidence to `checkInOfferObservedMessage`, which is the offer's own list and the one line Session 10 takes out. `retireInteraction` is the one act every former `cancelPendingCheckInOffer` site now spells: it advances the count and then calls `forgetInteractionScopedWork`, the whole of what retiring an interaction costs the queue, so the stale-refusal protection reads ownership alone and no longer any offer state. Nothing about delivery moved: the same durable messages reach the same handlers, the duplicate Enter/Ctrl+S submission guard still retires nothing, and `internal/tui/task_check_in_editor_test.go` gains `TestExplicitCheckInRejectionReopensOnlyItsOwnInteraction`, which drives a refused explicit save through the shell with the offer queue empty throughout and proves both halves: the owning interaction gets the exact draft, the error, and the frozen request back, while an interaction retired by one intervening key press gets nothing and the editor stays closed.

### Session 10 — Non-interrupting check-in feedback across commit paths

- Go: `internal/tui/task_check_in_feedback_test.go`, `internal/tui/check_in_visual_acceptance_test.go`, `internal/tui/task_check_in_editor_test.go`, `internal/tui/ux_daily_use_baseline_test.go`, `internal/app/task_state_effect_test.go`
- Scenarios: `docs/tui-testing/scenarios/active-work-check-in-feedback.json`
- Frames: `check-in-offer`
- Contracts: `docs/guide/outliner-keys.md` line 12; `docs/guide/capture.md` line 12; `docs/architecture/operation-grammar.md` line 479
- Landed: Entering Doing or Waiting without a schedule now says so and opens nothing. `internal/tui/task_check_in_feedback.go` replaces `task_check_in_offer.go` one file for one, keeping the root package at its ceiling: `observeCheckInFeedbackMessage` reads committed `TaskStateEffect` evidence off the five result envelopes, `noteCheckInFeedback` is also called directly by the two paths that publish their own toast before returning — `commitTaskContentEdit` and the outline buffer — and `publishCheckInFeedback` runs from the single deferred closure in `AppModel.Update`, so a path that speaks for itself is never displaced. The note is `No check-in: <title> · menu: Set up check-ins`, or `No check-in: N tasks` for a committed subset, and it names the keyless target-free workspace entry rather than `@` or `Space`, since those read the cursor and the committed task may be a lane away. `committedCheckInTitles` reloads each committed project and re-reads each stored task, so a target scheduled or moved out of active work between commit and render drops out silently and an unreadable project contributes nothing; transaction IDs mark every committed transaction seen whether or not it counted anything, so duplicate deliveries, reloads, and redraws stay quiet. `feedbackController.appendNote` splices the note ahead of the `· u undo` and `· Ctrl+O origin` suffixes, advancing no `seq`, so the handler's own severity and flash decision are untouched and a standalone note is quiet `setInfoStatus`. Because the note takes no keyboard it asks nothing of interaction ownership: the deferred closure observes every commit, owned or not, while `originTaggedMessage` narrows to the closed set that still needs ownership — `CheckInRejectedMsg` and the three operation envelopes — so Session 9's stale-refusal protection survives intact. The whole `CheckInOffer` chain is gone: `operation.CommandContext.CheckInOffer`, the request field, the eligibility skip-guard in `task_operation_execute.go`, the editor's `Optional check-in for N task(s) · Esc skips setup` notice, `app.TaskOperationRequest.CheckInOffer`, the unexported `bulkSetCheckIn` and `bulkAttributeOverSnapshotGuarded`. `internal/app/task_state_effect_test.go` replaces the stale-eligibility contract with `TestExplicitCheckInSaveTakesNoEligibilityGuard`, which pins that an explicit save judges only locator resolution and non-terminal state and attaches no strict precondition, while a concurrent write is still refused by the project's own `ExpectedRevision` contract.

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
