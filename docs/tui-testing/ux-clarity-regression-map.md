# UX clarity regression map

The UX clarity refinement is eight serial sessions over feedback, guidance,
context, and Peek. Seven of them change something the product already does, and
this file names, for each one, where a change would show and which recorded
expectation the session is expected to rewrite on purpose.

This map is separate from
[`ux-refinement-regression-map.md`](ux-refinement-regression-map.md), which
covers the earlier eighteen-session plan. That map and its enforced numbering
stay exactly as they are: the two plans number their sessions independently, so
merging them would make every "session N" in either document ambiguous.
`ux_clarity_regression_map_test.go` in the repository root holds this file to its
own claims, and the older gate keeps holding the older file to its.

The map is curated. A test that merely passes through one of these states is not
a contract about it; only reading the test tells the two apart.

## Baseline and review evidence

Both passes read the `ux-acceptance` fixture, which is the one fixture where the
axes every other fixture isolates are simultaneously true. A second fixture would
have made the before and after frames incomparable.

| Artifact | Records |
| --- | --- |
| `internal/tui/ux_clarity_review_matrix_test.go` | The review matrix: six staged frames at four sizes in dark, light, and monochrome, exported as ANSI and plain text with a measurement manifest. |
| `internal/tui/ux_clarity_baseline_test.go` | The combined states this plan changes, asserted semantically and geometrically, each naming the session that owns rewriting it. |
| `internal/tui/ux_presentation_matrix_acceptance_test.go` | The earlier plan's presentation matrix, which keeps answering for row, hierarchy, and selection presentation while this plan runs. |
| `docs/tui-testing/scenarios/ux-acceptance-daily-loop.json` | The committed daily loop the eighth session replays. |
| `docs/tui-testing/scenarios/ux-acceptance-narrow-baseline.json` | Layout health and footer rank drops at all four reviewed sizes. |

### Exporting the before-frames

The matrix writes its frames to a caller-selected directory:

```bash
TT_UX_CLARITY_REVIEW_DIR=/tmp/ux-clarity-before \
  go test ./internal/tui/ -run TestUXClarityReviewMatrix -count=1
```

With the variable unset the export still runs, into the test's own temporary
directory, so the writer is exercised on every ordinary run and naming a
directory only relocates the output. Each frame produces three files —
`<variant>-<W>x<H>-<frame>.ansi`, `-mono.ansi`, and `.txt` — beside one
`frames.tsv` index. The monochrome capture is the styled frame projected through
the ASCII color profile rather than a third palette, which is what proves dark
and light carry the same content.

Run the same command after a session with a different directory and diff the two
`frames.tsv` files: a row that moved is a measurement that moved.

### The six frames

| Frame | Records | Replaced by |
| --- | --- | --- |
| `launch` | The resting Outliner: one header row, no feedback, the surface's own ranks. | Session 5 |
| `feedback-over-guidance` | A routine success standing where the capture controls were. | Session 4 |
| `picker-over-outline` | A value picker's instructions and the host's ranks on screen together. | Session 3 |
| `nested-discard-prompt` | A discard confirmation over a capture draft over the outline. | Session 3 |
| `combined-context` | Selection, search, filters, and sort competing for the one header row. | Session 5 |
| `capture-over-peek` | Capture raised over an open Peek dock of fixed height. | Session 7 |

## Current layout measurements

Taken from `frames.tsv` at `dc06394e`. Dark and light measure identically at
every size, so only one column is given. `content` is the Outliner's list window,
`dock` the Peek dock including its divider and position line, and `footer` the
shell-composed bottom bar.

| Frame | 60×20 | 80×24 | 100×35 | 120×40 |
| --- | --- | --- | --- | --- |
| `launch` | header 1 · content 17 · footer 2 | header 1 · content 21 · footer 2 | header 1 · content 32 · footer 2 | header 1 · content 37 · footer 2 |
| `feedback-over-guidance` | content 9 · footer 2 | content 13 · footer 2 | content 19 · footer 2 | content 23 · footer 2 |
| `picker-over-outline` | content 17 · footer 2 | content 21 · footer 2 | content 32 · footer 2 | content 37 · footer 2 |
| `nested-discard-prompt` | content 9 · footer 2 | content 13 · footer 2 | content 19 · footer 2 | content 23 · footer 2 |
| `combined-context` | content 17 · footer 5 | content 21 · footer 5 | content 32 · footer 4 | content 37 · footer 4 |
| `capture-over-peek` | content 9 · dock 7 · footer 3 | content 13 · dock 7 · footer 3 | content 19 · dock 7 · footer 3 | content 23 · dock 7 · footer 3 |

Three facts these record, which shape the sessions that follow:

- The header is **one row at every size**, and the context axes past its fit are
  on screen nowhere. At 120×40 all three axes and the workspace pulse fit; at
  60×20 the line truncates mid-value. Session 5 owns the second row and the `Q`
  details that recover the remainder.
- The Peek dock is **seven rows at every height**, 60×20 and 120×40 alike, so
  every row a short terminal lacks comes out of the list instead. At 60×20 with
  capture open the list is left with 9 rows of window. Session 7 replaces this
  with `clamp(floor(H/3), 7, 12)` — eleven rows at 100×35 — and a three-row
  displayed minimum under pressure.
- With capture open the footer is **2 rows when a status displaced the controls
  and 3 when it did not**, because status and guidance are one occupant today.
  Session 4 budgets them separately.

## Expectations later sessions deliberately replace

Each test below asserts today's behavior and says so in its own doc comment. A
failure after the owning session lands is that session's signal, not a
regression; the session that reaches the new behavior owns rewriting the
contract in the same change.

| Recorded expectation | Test | Owner |
| --- | --- | --- |
| ~~A routine copy success arms the animated footer divider.~~ Replaced by session 2: a routine success reports and animates nothing, a warning still does, and the quiet line retires the warning's animation. | `TestUXClarityRoutineSuccessIsQuiet`, formerly `TestUXClarityRoutineSuccessArmsTheFooterFlash` | Session 2 — done |
| A picker owns every key while the host below still publishes its ranks. | `TestUXClarityPickerLeavesHostGuidanceOnScreen` | Session 3 |
| A discard prompt stands on an open composer over the outline, three deep. | `TestUXClarityDiscardPromptStandsOnTheOpenComposer` | Session 3 |
| Status displaces the capture controls rather than standing beside them. | `TestUXClarityFeedbackTakesTheCaptureControlsRow` | Session 4 |
| Transient messages replace capture controls in the footer. | `TestCaptureFooterYieldsToTemporaryStatusThenReturns` | Session 4 |
| Every context axis competes for one header row and the overflow is unreadable. | `TestUXClarityCombinedContextExceedsTheOneRowHeader` | Session 5 |
| The Outliner's content height counts exactly one header row. | `OutlinerModel.ContentHeight` in `internal/tui/outliner/outliner_rows.go` | Session 5 |
| Peek's dock costs the same rows at every terminal height. | `TestUXClarityCaptureOverPeekKeepsAFixedDock` | Session 7 |
| Peek's body, window, and page sizes are constants. | `inlinePeekBodyRows` in `internal/tui/outliner/inline_peek_document.go` | Session 7 |

## Per-session regression homes

### Session 2 — Make routine feedback quiet

- Go: `internal/tui/footer_flash_test.go`,
  `internal/tui/app_view_status_test.go`,
  `internal/tui/operation_executor_status_test.go`,
  `internal/tui/advisories_test.go`,
  `internal/tui/advisories_shell_test.go`,
  `internal/tui/ux_clarity_baseline_test.go`
- Scenarios: `docs/tui-testing/scenarios/outliner-status-strip.json`,
  `docs/tui-testing/scenarios/ux-acceptance-daily-loop.json`
- Contracts: `CLAUDE.md` — only warning and error status text arms footer flash,
  and cleared text and other chrome arm nothing; quiet feedback supersedes
  through `retireFlash`; arming stays in the one deferred closure in
  `AppModel.Update`; divider appearance is fed to all three `renderFooterChrome`
  dividers and the strip divider stays outside; a view status is transient except
  the multi-select summary; advisories route through `AdvisoryLine` or
  `AdvisoryStatus` and are never gated at a render site.
- Landed: `footerFlashSeverity.animates()` in
  `internal/tui/app_view_status.go` is the closed severity table, and
  `feedbackController.retireFlash` in `internal/tui/feedback_controller.go` is
  the supersession. `internal/tui/footer_flash_test.go` holds the two halves
  apart — `TestFooterFlashStaysQuietOnRoutineShellStatus` and
  `TestFooterFlashStaysQuietOnViewStatusLine` against
  `TestFooterFlashArmsOnWarningStatus` and `TestFooterFlashArmsOnError` — with
  `TestQuietFeedbackRetiresAnAnimatedWarning` on the stale ticks and
  `TestViewStatusExpiryStaysIndependentOfTheFlash` on the clock that must not
  follow the animation.
- The regression to watch for is not a loud success but a quiet warning: a
  future severity added next to the wrong branch inherits silence, which is why
  `TestFooterFlashSeverityAnimatesOnlyWarningAndError` states the table in full
  rather than asserting only the cases the product reaches today.

### Session 3 — Give the active interaction sole ownership of guidance

- Go: `internal/tui/overlay_guidance_test.go`,
  `internal/tui/outliner_guidance_test.go`,
  `internal/tui/outliner_escape_guidance_test.go`,
  `internal/tui/footer_ranked_guidance_test.go`,
  `internal/tui/form_editor_guidance_test.go`,
  `internal/tui/projects_notes_guidance_test.go`,
  `internal/tui/contextual_action_sheet_test.go`,
  `internal/tui/quick_capture_interrupt_test.go`,
  `internal/tui/ux_clarity_baseline_test.go`
- Scenarios: `docs/tui-testing/scenarios/outliner-escape-guidance-ladder.json`,
  `docs/tui-testing/scenarios/outliner-search-sigil-guidance.json`,
  `docs/tui-testing/scenarios/tags-picker-context-radio.json`,
  `docs/tui-testing/scenarios/date-picker-narrow-resize-key-confinement.json`
- Contracts: `CLAUDE.md` — a footer has one source, the strategy's ranks or the
  gated keymap where modal sub-states narrow a key; Esc is an act with one source
  and `chrome.DraftEscapeHint` is what a draft-bearing surface reads; a
  draft-discard question is not `ConfirmationGuidanceSpec`; a modal covers rows
  rather than displacing them; required entries survive whole at
  `chrome.MinTerminalWidth`.
- `TestReplaceFooterRemovesEveryStructuredGuidanceRow` in
  `internal/tui/overlay_guidance_test.go` is the existing statement about what a
  composed footer may carry, and
  `TestUXClarityPickerLeavesHostGuidanceOnScreen` is the one this session
  inverts: it requires the host's ranks to be present today so that requiring
  their absence is a visible edit rather than a silently relaxed assertion.

### Session 4 — Separate feedback from controls and measure the complete frame

- Go: `internal/tui/capture_footer_test.go`,
  `internal/tui/chrome/capture_footer_test.go`,
  `internal/tui/chrome/footer_helpers_test.go`,
  `internal/tui/chrome/hosted_panel_test.go`,
  `internal/tui/app_view_status_test.go`,
  `internal/tui/app_handlers_view_capture_test.go`,
  `internal/tui/ux_clarity_baseline_test.go`
- Scenarios: `docs/tui-testing/scenarios/capture-bottom-panel.json`,
  `docs/tui-testing/scenarios/form-extras-comments-narrow.json`,
  `docs/tui-testing/scenarios/global-notes-resize-minimum-layout.json`,
  `docs/tui-testing/scenarios/project-inbox-mode-resize-minimum-layout.json`
- Contracts: `CLAUDE.md` — notice and status render only in the shell-pinned
  strip whose height `StatusStripHeight` reserves from
  `ComputeOutlinerViewport`; `route.HostedPanelView` is the one seam and
  `chrome.HostedPanelLayout` the whole of what it says; `HostRows` is a pure
  function of the frame; `syncHostedPanelHost` resizes once per `Update` and
  never per frame; `ComposeHostedPanel` pins the panel to the bottom and spends
  declined rows on blank lines.
- `TestCaptureFooterYieldsToTemporaryStatusThenReturns` is the test this session
  replaces by name. The measurement it is replaced with has to stay pure:
  `HostRows` may not read the cursor, the draft, or the document, so a height
  derived from an unconsumed notice is the failure mode to watch for.

### Session 5 — Introduce the context row and complete `Q` details

- Go: `internal/tui/query_readout_test.go`,
  `internal/tui/filter_context_chord_test.go`,
  `internal/tui/outliner/operational_title_test.go`,
  `internal/tui/outliner/outliner_render_rows_test.go`,
  `internal/tui/outliner/outliner_render_rows_pressure_test.go`,
  `internal/tui/outliner/menu_bar_render_outlinersplit_test.go`,
  `internal/tui/ux_clarity_baseline_test.go`
- Scenarios: `docs/tui-testing/scenarios/query-readout-stacked-axes.json`,
  `docs/tui-testing/scenarios/query-readout-narrow-layout.json`,
  `docs/tui-testing/scenarios/query-readout-save-as-lens.json`,
  `docs/tui-testing/scenarios/today-pace-readout.json`,
  `docs/tui-testing/scenarios/filter-context-facet.json`,
  `docs/tui-testing/scenarios/outliner-multiselect-narrow-layout.json`
- Contracts: `CLAUDE.md` — `Q` names one row per peelable axis and nothing it
  cannot prove, facet rows spell `FilterClauseParts`, `peelQueryReadoutAxis` is
  the one write and routes each axis through the call its own key uses, peel is
  idempotent at zero, `queryReadoutRowLimit` is the digit set and names its
  overflow, and the open readout is a gated keymap taking no footer rank; the
  menu bar costs one list row subtracted in `ContentHeight` alone and is never
  measured again; the four-step pulse ladder drops done, inbox, overdue, then
  today.
- The optional second row is the one addition that changes an arithmetic three
  other things read: `ContentHeight`, the hosted capture host, and the modal
  anchor `chrome.OverlayPopupDimmedAt` composites below. It must be counted
  exactly once, the same way the menu bar row already is.

### Session 6 — Reorder Peek around current work

- Go: `internal/tui/outliner/inline_preview_test.go`,
  `internal/tui/outliner/inline_preview_resume_test.go`,
  `internal/tui/outliner/inline_peek_project_document_test.go`,
  `internal/tui/outliner/empty_project_peek_guidance_test.go`
- Scenarios: `docs/tui-testing/scenarios/outliner-peek-wide-layout.json`,
  `docs/tui-testing/scenarios/outliner-peek-whole-title.json`,
  `docs/tui-testing/scenarios/outliner-peek-scrolling.json`,
  `docs/tui-testing/scenarios/ux-acceptance-peek-baseline.json`,
  `docs/tui-testing/scenarios/outliner-project-peek-group-header.json`
- Contracts: `CLAUDE.md` — the Peek body order is enumerated and a new section is
  placed deliberately; the body is complete and wrapped and never elided;
  `inlinePeekDocument` separates the whole detail from the window of it and
  `Lines` is the only way to either; status keys on stored values so a tick never
  resets the window; the window is keyed by typed Peek owner, document
  fingerprint, and terminal size, with schedule date, cadence, and last-review
  instant part of that fingerprint; project Peek keeps its own order through
  `inlinePeekProjectDocumentFor`.
- Reordering moves the fingerprint's inputs around, not its contents, so the
  paging test to watch is the one that proves a 60-second tick does not reset the
  offset. Cover reachability rather than position counts: a section that moved
  and a section that vanished look identical to an index assertion.

### Session 7 — Make Peek responsive and integrate constrained capture

- Go: `internal/tui/outliner/inline_peek_paging_test.go`,
  `internal/tui/outliner/peek_dock_test.go`,
  `internal/tui/outliner/inline_peek_empty_project_width_test.go`,
  `internal/tui/responsive_peek_test.go`,
  `internal/tui/ux_clarity_baseline_test.go`
- Scenarios: `docs/tui-testing/scenarios/outliner-peek-paging.json`,
  `docs/tui-testing/scenarios/outliner-peek-narrow-layout.json`,
  `docs/tui-testing/scenarios/capture-bottom-panel.json`,
  `docs/tui-testing/scenarios/outliner-peek-scrolling.json`,
  `docs/tui-testing/scenarios/resize-stress.json`
- Contracts: `CLAUDE.md` — `PeekDockRows` is the divider plus
  `inlinePeekWindowRows` and `outlinerListRows` the one reservation that
  subtracts it; the reservation reads no cursor and no document; both render
  paths measure against one window and pad the remainder, and the empty
  projection is no exception; a row owning no document draws
  `peekDockPlaceholderDocument` at that same height; `peekOffsetForRow` is pure
  and `scrollPeek` the only writer; `Ctrl+D`/`Ctrl+U` travel two content rows and
  `Ctrl+F`/`Ctrl+B` four; `ComputeOutlinerViewport` is the single window
  authority.
- The 60×20 combination of context, Peek, capture, and required guidance is the
  case with the least room, and `capture-over-peek` in the review matrix is the
  frame of it. `TestUXClarityCaptureOverPeekKeepsAFixedDock` fails deliberately
  when the dock becomes height-dependent; replacing it with the clamp is this
  session's work, and the empty-projection path has to produce the same dock as
  the populated one.

### Session 8 — Integrated acceptance and final review

- Go: `internal/tui/ux_clarity_review_matrix_test.go`,
  `internal/tui/ux_clarity_baseline_test.go`,
  `internal/tui/ux_presentation_matrix_acceptance_test.go`,
  `ux_clarity_regression_map_test.go`,
  `ux_refinement_regression_map_test.go`
- Scenarios: `docs/tui-testing/scenarios/ux-acceptance-daily-loop.json`,
  `docs/tui-testing/scenarios/ux-acceptance-narrow-baseline.json`,
  `docs/tui-testing/scenarios/ux-acceptance-row-baseline.json`,
  `docs/tui-testing/scenarios/ux-acceptance-selection-baseline.json`,
  `docs/tui-testing/scenarios/resize-stress.json`
- Contracts: `CLAUDE.md` in whole, since the acceptance pass is where the
  invariants this plan touched are reconciled with what the code now does;
  `docs/guide/outliner-chrome.md`, `docs/guide/outliner-rows.md`, and
  `docs/architecture/presentation-chrome.md` are the user-facing and mechanism
  halves of the same reconciliation.
- Re-export the review frames into a second directory and diff `frames.tsv`
  against the baseline run recorded above. Every moved measurement should belong
  to a session that claimed it; one that does not is a finding.
