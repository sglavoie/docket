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
| `launch` | The resting Outliner: one header row, no feedback, the surface's own ranks. | Session 5 — done (kept: a resting view narrows nothing, so it still spends one row) |
| `feedback-over-guidance` | A routine success standing where the capture controls were. | Session 4 — done |
| `picker-over-outline` | A value picker owning the one instruction set over a quiet host outline. | Session 3 — done |
| `nested-discard-prompt` | A discard confirmation over a capture draft over the outline. | Session 3 — done |
| `combined-context` | Selection, search, filters, and sort competing for the one header row. | Session 5 — done (the axes take a second row and `Q` holds what it drops) |
| `capture-over-peek` | Capture raised over an open Peek dock of fixed height. | Session 7 — done (the dock is a share of the frame, and capture shrinks it to what the panel leaves) |

## Layout measurements before the plan

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

- The header was **one row at every size**, and the context axes past its fit
  were on screen nowhere. At 120×40 all three axes and the workspace pulse fit;
  at 60×20 the line truncated mid-value. Session 5 replaced this: identity keeps
  the first row, the live axes take an optional second one charged once against
  the list, and what that row cannot draw it drops whole and forwards to `Q`,
  whose bounded scrollable document is where the full value is read. A resting
  view narrows nothing, so `launch` still measures one header row.
- The Peek dock was **seven rows at every height**, 60×20 and 120×40 alike, so
  every row a short terminal lacked came out of the list instead. At 60×20 with
  capture open the list was left with 9 rows of window. Session 7 replaced this
  with `clamp(floor(H/3), 7, 12)` — eleven rows at 100×35 — capped by what the
  frame has left after the headers, the strip, the footer, and one task row, and
  floored at the three rows a displayed dock owes.
- With capture open the footer was **2 rows when a status displaced the controls
  and 3 when it did not**, because status and guidance were one occupant. Session
  4 budgeted them separately: it is now 3 rows resting and 4 with a status
  standing, and `chrome.ShellFrameMetrics` is what the surface underneath
  reserves the difference from.

## Layout measurements after the plan

Re-exported by session 8 from the same six frames, the same four sizes, and the
same two palettes. Dark and light still measure identically, so one column still
carries both.

| Frame | 60×20 | 80×24 | 100×35 | 120×40 |
| --- | --- | --- | --- | --- |
| `launch` | header 1 · content 17 · footer 2 | header 1 · content 21 · footer 2 | header 1 · content 32 · footer 2 | header 1 · content 37 · footer 2 |
| `feedback-over-guidance` | content 9 · footer 4 | content 13 · footer 4 | content 19 · footer 4 | content 23 · footer 4 |
| `picker-over-outline` | content 17 · footer 2 | content 21 · footer 2 | content 32 · footer 2 | content 37 · footer 2 |
| `nested-discard-prompt` | content 9 · footer 2 | content 13 · footer 2 | content 19 · footer 2 | content 23 · footer 2 |
| `combined-context` | header 2 · content 16 · footer 6 | header 2 · content 20 · footer 6 | header 2 · content 31 · footer 5 | header 2 · content 36 · footer 5 |
| `capture-over-peek` | content 9 · dock 7 · footer 3 | content 13 · dock 7 · footer 3 | content 19 · dock 7 · footer 3 | content 23 · dock 8 · footer 3 |

Every cell that moved belongs to a session that claimed it, which is the whole
point of the re-export:

- `feedback-over-guidance` gained **two footer rows at every size**, 2 → 4. That
  is session 4: the status no longer stands in the controls' row, so the footer
  carries a feedback block and a controls block over one divider. The content
  rows did not move, because the panel above it is what shrank.
- `combined-context` gained **one header row and one footer row and gave up one
  content row**, at every size. The header row is session 5's context row, and it
  is charged once — the content row it cost is that charge. The footer row is
  session 4's again, under the same selection summary.
- `capture-over-peek` moved its dock **only at 120×40**, 7 → 8. That is session
  7: the preferred height is `clamp(floor(H/3), 7, 12)`, so 40 rows prefer 12,
  and what the frame has left under an open capture panel is 8. At the three
  shorter sizes the available half is still the binding one and the dock measures
  exactly what it did before, which is why the clamp is visible in one cell
  rather than four.
- `launch`, `picker-over-outline`, and `nested-discard-prompt` did not move at
  all. That is the claim sessions 3 and 5 both made and is worth reading as a
  result rather than as an absence: a resting view narrows nothing, so it still
  spends one header row, and a picker that took sole ownership of guidance took
  no rows with it.

No other cell moved, and no cell moved that no session named.

## Expectations later sessions deliberately replace

Each test below asserts today's behavior and says so in its own doc comment. A
failure after the owning session lands is that session's signal, not a
regression; the session that reaches the new behavior owns rewriting the
contract in the same change.

| Recorded expectation | Test | Owner |
| --- | --- | --- |
| ~~A routine copy success arms the animated footer divider.~~ Replaced by session 2: a routine success reports and animates nothing, a warning still does, and the quiet line retires the warning's animation. | `TestUXClarityRoutineSuccessIsQuiet`, formerly `TestUXClarityRoutineSuccessArmsTheFooterFlash` | Session 2 — done |
| ~~A picker owns every key while the host below still publishes its ranks.~~ Replaced by session 3: the picker is the only layer publishing, the rows its host's ranks occupied stay reserved, and closing it restores the footer byte for byte. | `TestUXClarityPickerOwnsGuidanceAndKeepsHostRows`, formerly `TestUXClarityPickerLeavesHostGuidanceOnScreen` | Session 3 — done |
| A discard prompt stands on an open composer over the outline, three deep. Kept by session 3 and extended: the footer answers for the prompt, and neither presentation underneath it speaks. | `TestUXClarityDiscardPromptStandsOnTheOpenComposer` | Session 3 — done |
| ~~Status displaces the capture controls rather than standing beside them.~~ Replaced by session 4: the footer says both, feedback on its own row above the controls, at every size in the matrix. | `TestUXClarityFeedbackStandsBesideTheCaptureControls`, formerly `TestUXClarityFeedbackTakesTheCaptureControlsRow` | Session 4 — done |
| ~~Transient messages replace capture controls in the footer.~~ Replaced by session 4: the message takes a row beside them and clearing it restores the footer byte for byte. | `TestCaptureFooterKeepsControlsBesideATemporaryStatus`, formerly `TestCaptureFooterYieldsToTemporaryStatusThenReturns` | Session 4 — done |
| ~~Every context axis competes for one header row and the overflow is unreadable.~~ Replaced by session 5: the axes take an optional second row, it drops whole segments and names what it dropped, and every dropped value is readable in the bounded `Q` document. | `TestUXClarityCombinedContextIsReadableAtEverySize`, formerly `TestUXClarityCombinedContextExceedsTheOneRowHeader` | Session 5 — done |
| ~~The Outliner's content height counts exactly one header row.~~ Replaced by session 5: it counts the identity row plus `contextRowRows()`, which is presence-only and charged exactly once. | `OutlinerModel.ContentHeight` in `internal/tui/outliner/outliner_rows.go`, `TestOutlinerContextRowIsAbsentFromADefaultView` | Session 5 — done |
| ~~Peek's dock costs the same rows at every terminal height.~~ Replaced by session 7: the dock is `clamp(floor(H/3), 7, 12)` capped by what the frame leaves, it grows with height, capture shrinks it and closing capture gives it back, and typing under capture moves it not at all. | `TestUXClarityPeekDockIsAShareOfTheFrame`, formerly `TestUXClarityCaptureOverPeekKeepsAFixedDock` | Session 7 — done |
| ~~Peek's body, window, and page sizes are constants.~~ Replaced by session 7: `peekDockGeometry` is the one measurement, and the body, the window, the total, and both page distances are derived from its `bodyRows`. | `peekDockGeometry` in `internal/tui/outliner/inline_peek_paging.go` | Session 7 — done |

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

- Go: `internal/tui/guidance_ownership_test.go`,
  `internal/tui/app_view_status_test.go`,
  `internal/tui/route/presentation_chrome_policy_test.go`,
  `internal/tui/overlay_guidance_test.go`,
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
  `internal/tui/overlay_guidance_test.go` was the existing statement about what a
  composed footer may carry — session 4 replaced both it and the behavior it
  described with `TestSurfaceFooterFeedbackStandsAboveEveryStructuredGuidanceRow`
  — and
  `TestUXClarityPickerLeavesHostGuidanceOnScreen` was the one this session
  inverted: it required the host's ranks to be present so that requiring their
  absence would be a visible edit rather than a silently relaxed assertion.
- Landed: `route.ActiveGuidanceOwner` in
  `internal/tui/route/presentation_chrome_policy.go` is the ownership vocabulary
  and `AppModel.activeGuidance` in `internal/tui/app_view_status.go` the one
  resolution, with `suppressGuidanceRows` blanking the occupant at its own
  height and `surfaceFooterContent` holding the branches it takes whole.
  `route.SelfGuidedPresentation` in `internal/tui/route/view.go` is the
  per-frame override the task composer answers, since one `PresentationID` names
  its compact panel and its expanded form alike.
  `internal/tui/guidance_ownership_test.go` enumerates the covered surfaces —
  every arm of `shellModalMounted`, the three shell confirmations, the chord's
  place in the order, the expanded composer, and the Outliner's view-local menu —
  and each case asserts the same three properties, because they fail
  differently: the text is gone, the height is unchanged, and putting the
  interaction down restores the frame byte for byte.
- The boundary the last rung stands on is `HostGuidanceStandsBeneath`, which asks
  whether a host is still drawn under this presentation rather than whether this
  presentation drew its own acts. The two read alike and only one is right: the
  Comments panel, the project editor, Notes, and the Hub all print their controls
  inside their content, but each replaced the host outright, so the footer
  beneath holds that same surface's breadcrumb and its `Esc back · ? help` — the
  one statement of either, not a duplicate. `TestExampleScenariosPass` states
  both halves already: `task-comments-help.json` requires `? help` under a panel
  that draws its own row, and `project-editor-minimum-layout.json` requires the
  breadcrumb, while `tags-picker-direct.json`, `project-tags-picker.json`, and
  `semantic-task-tags-split.json` now require the host's ranks to be absent under
  a picker and back after its close.
- The regression to watch for is not a footer that speaks under an overlay but
  one that goes quiet under nothing: a new presentation left unclassified, or an
  owner added beside the wrong branch of `SuppressesShellGuidance`, would blank
  the only instructions a surface has.
  `TestActiveGuidanceOwnerSuppressionIsAClosedTable` and
  `TestHostGuidanceStandsBeneathNamesExactlyTheTransientPresentations` in
  `internal/tui/route/presentation_chrome_policy_test.go` state both defaults in
  full rather than asserting only the cases the product reaches today, and the
  second holds the predicate to being the exact complement of
  `WorkspacePulseEligible`, so the two questions cannot drift into two
  classifications.

### Session 4 — Separate feedback from controls and measure the complete frame

- Go: `internal/tui/capture_footer_test.go`,
  `internal/tui/shell_frame_budget_test.go`,
  `internal/tui/overlay_guidance_test.go`,
  `internal/tui/chrome/frame_budget_test.go`,
  `internal/tui/outliner/peek_dock_test.go`,
  `internal/tui/app_view_status_test.go`,
  `internal/tui/app_handlers_view_capture_test.go`,
  `internal/tui/ux_clarity_baseline_test.go`
- Scenarios: `docs/tui-testing/scenarios/capture-bottom-panel.json`,
  `docs/tui-testing/scenarios/form-extras-comments-narrow.json`,
  `docs/tui-testing/scenarios/global-notes-resize-minimum-layout.json`,
  `docs/tui-testing/scenarios/project-inbox-mode-resize-minimum-layout.json`
- Contracts: `CLAUDE.md` — the footer is feedback then controls and feedback
  never takes the controls' place; `chrome.ShellFrameMetrics` is the one
  measurement of that footer and `route.ShellFrameAware` the one delivery, once
  per `Update`, to the active view alone; a view drawing its own footer keeps it
  and pays for the feedback row out of its body; notice and status still render
  only in the shell-pinned strip whose height `StatusStripHeight` reserves from
  `ComputeOutlinerViewport`; `route.HostedPanelView` is the one seam and
  `chrome.HostedPanelLayout` the whole of what it says; `HostRows` is a pure
  function of the frame; `syncHostedPanelHost` resizes once per `Update` and
  never per frame; `ComposeHostedPanel` pins the panel to the bottom and spends
  declined rows on blank lines.
- `TestCaptureFooterYieldsToTemporaryStatusThenReturns` was the test this session
  replaced by name. The measurement it is replaced with has to stay pure:
  `AppModel.shellFrameMetrics` renders the footer's two blocks into strings and
  measures those, and nothing under it consumes a one-shot notice —
  `ViewStatusStrip` is the one status surface that does consume, and it is
  deliberately not measured there, because the strip is view content the view
  already reserves.
  `TestShellFrameMetricsAreAPureMeasurement` holds that by taking the
  measurement twice around a live focus notice and requiring the notice to
  survive both.
- The removals to watch are the compensating reservations, not the footer: each
  was a local guess at how far the shell footer might overflow, and each is now
  the one delivered `ExcessRows`. A second one added back beside it is a double
  reservation: `TestPeekDockReservationRidesTheMeasuredFooter` and
  `TestShellFrameBudgetIsChargedExactlyOnce` are the two that would catch it,
  from the list's end and from the composed frame's.
- The Peek dock's guard row is the one that stayed, and it stayed as a floor
  rather than as a second charge. The measurement it would have been replaced by
  is one this view's own cursor moves — the dock advertises `Ctrl+D more` over a
  document that can page and withholds it over one that cannot, so at minimum
  width the footer wraps for one row and not for the next — and a dock following
  that difference is the moving target `dc06394e` locked shut.
  `peekDockFooterWrapRows` is the floor, `max`'d under `ExcessRows` and never
  added to it, so a genuinely taller footer is still charged whole.
  `TestPeekDockCostsTheListOneFixedReservation` and the presentation matrix's
  60x20 and 80x24 rows are what hold it.

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

Landed as: `BuildViewContextEntries` is the one structured description, and the
compact row and `Q` both read it rather than parsing each other's rendered text.
`contextRowRows()` is presence-only, which is what lets `ContentHeight` (which
takes no width) and `outlinerHostHeaderRows` charge the same row without a second
arithmetic; `renderOutlinerEmptyState` reaches it through the shared
`writeOutlinerHeader`, so the populated and empty paths cannot disagree.
`pulse.RenderContextRow` drops whole segments from the right and spends the
reclaimed room on `+N more · Q details`, so a shortened value always names where
the whole of it is. `queryReadoutWindow` is the readout's one measurement — the
document, the window, and whether anything is off it — read by the render, every
scroll write, and the footer alike, and `queryReadoutBodyRows` mirrors
`outlinerListRows`' own arithmetic rather than asking `StatusStripHeight` for a
height it is still deciding. The digits stay `QueryReadoutRows()`' first nine
wherever the window sits; the read-only axes are an `Also in view` block with no
digits. `formatWorkspacePulseComponents` now omits a zero component and returns
the empty string rather than a bare `workspace:` prefix, which changes neither
the drop ladder's order nor any counter.

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

Landed as: the one mixed dates line is two sections. `inlinePeekScheduleLine` is
what the task still owes — deadline, a deferral still pending, the next check-in
and its repeat — and sits directly under the status line, where the status
summary already answers "what is this right now". `inlinePeekHistoryLine` is
what it has already spent — created, updated, last review, a lapsed deferral's
wake, the avoidance count — and is the last section in the document, below the
tags that used to close it, because nothing in it is owed. Both are built on the
shared `inlinePeekFields`, so an empty value is still no field and neither line
gained a way to render a blank one; the old line's Doing/Waiting ages stayed on
the status line, untouched. Splitting one line into two grows every document
carrying both halves by a row, which is why the acceptance scenarios' positions
moved by one rather than staying put — the count is evidence the section arrived,
not the assertion that it did, so each one is paired with a `contains` naming the
content of the page it lands on. The deferral instant is keyed into both
`inlinePeekScheduleKey` and `inlinePeekHistoryKey`: exactly one of the two
sections is drawn from it, so keying it once would drop it from the fingerprint
in the other case. Both keys still read stored instants formatted
`time.RFC3339Nano` and never a rendered duration, which is what the drift test
proves directly now — it compares two renders an hour and a half apart and
asserts the lines moved while the keys did not.

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

Landed as: `peekDockGeometry` is the one measurement, and it carries `bodyRows`
alone — `WindowRows`, `TotalRows`, `HalfPageRows`, and `FullPageRows` are all
derived from it, so the render, the offset clamp, the paging distances, and the
availability hints cannot disagree about a height. `PeekDockRows` became a method
for that reason: the constant it replaced was the thing four callers each read
independently. The height is `min(peekDockPreferredRows, peekDockAvailableRows)`
— the preferred half is `clamp(floor(H/3), 7, 12)` over the Outliner's own
allotted height, and the available half is what `ContentHeight` has left after the
measured footer excess, the status strip, and one task row — and a total under
`peekDockMinRows` (one content line, the divider, the position line) draws no
dock at all rather than a clipped one, which is the existing unavailable-host
path. Both halves read the frame and neither reads the cursor, the document, or
the draft, so a short document, a project header, a placeholder row, and the
empty projection all get identical geometry, and typing into capture moves
nothing: `syncHostedPanelHost` already resized the host once per `Update`, so
capture shrinking Peek and closing capture restoring it fall out of the frame
arithmetic rather than out of a second rule. `HalfPageRows` is
`max(bodyRows/2, 1)` and `FullPageRows` is `max(bodyRows - 1, 1)`, which
reproduce the retired constants 2 and 4 exactly at the old five-line body and
keep a one-row overlap at every other height; `Ctrl+D`/`Ctrl+U` and
`Ctrl+F`/`Ctrl+B` still belong to the dock and `PgDn`/`PgUp` still to the list.
The offset key deliberately did not gain the dock height — it stays owner,
fingerprint, width, and terminal height — so a status message that shortens the
dock clamps the window against the new geometry instead of re-anchoring it.
`queryReadoutBodyRows` reads `peekDockPreferredRows` rather than the settled
geometry, because the settled one would ask the strip for a height the strip is
still deciding; preferred is never smaller than settled, so the charge is
conservative in the one safe direction.

The Contracts bullet above names `inlinePeekWindowRows` because that is what the
contract said when the session opened. It and `inlinePeekBodyRows` are retired:
both were constants, and the whole point of the change is that a height is no
longer a constant. Anything that read either one reads `peekDockGeometry` now —
`BodyRows` where the old `inlinePeekBodyRows` was meant and `WindowRows` where
`inlinePeekWindowRows` was — and the paging distances that were spelled `2` and
`4` are `HalfPageRows` and `FullPageRows`.

### Session 8 — Integrated acceptance and final review

- Go: `internal/tui/ux_clarity_integrated_acceptance_test.go`,
  `internal/tui/ux_clarity_review_matrix_test.go`,
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

Landed as: `internal/tui/ux_clarity_integrated_acceptance_test.go`, which
deliberately re-asserts none of the seven sessions' own outcomes —
`internal/tui/ux_clarity_baseline_test.go` owns those and names the session
behind each. What
it holds is the arithmetic that spans them, in three tests.

`TestUXClarityIntegratedFrameSpendsEveryRowOnce` is the one that could only be
written here. Sessions 4, 5, and 7 each added a reservation against the same
terminal, and the failure they can produce together is one of them reserving rows
another already did. The test sums what the separate authorities each say they
drew — the Outliner's header rows, the list window `ComputeOutlinerViewport`
handed the render, the dock `peekDockGeometry` sized, the strip
`StatusStripHeight` reserved, and the footer `chrome.ShellFrameMetrics` measured
— and requires the total to be the terminal, with the resting frame, each of the
three occupants alone, and all three at once, at all four sizes. A row charged
twice makes the total short and a row charged to nobody makes it long, and
neither is visible to a test that reads one occupant. The single permitted
shortfall is `peekDockFooterWrapRows`, and only while the dock is open.

Two things that pass had to be got right for the sum to mean anything.
`renderStatusBar` composes the strip divider, the strip, *and* the footer, so
reading the footer off its height double-counts the strip; the budget reads
`shellFrameMetrics().FooterRows()` and then checks it against what that bar
painted minus the strip, so a metric that stopped matching its own paint is a
failure rather than a sum that agrees with itself and with nothing on screen. And
an open readout can legitimately leave `dock 0` at the shorter sizes: the strip
grew, the dock's available half fell under `peekDockMinRows`, and session 7's
rule is that such a dock is not drawn at all rather than drawn clipped.

`TestUXClarityIntegratedResizeReturnsTheFrameItLeft` takes a frame holding Peek,
a scrolled Peek window, the menu bar, and the expanded readout down to the floor
and back, and again out to the widest reviewed size and back, and requires the
screen byte for byte and the same dock height. A resize is the one act that moves
all four of this plan's frame-dependent behaviors at once, and one that is not a
pure function of the frame — that remembers a height it saw on the way down, or
seats a window it cannot re-derive — shows up here and nowhere else.

`TestUXClarityIntegratedWideTextFitsEveryReviewedTerminal` drives a title of
double-width CJK, a combining mark, and an emoji through capture, the committed
row, and Peek, at four sizes in both palettes. It is typed rather than added to
the `ux-acceptance` fixture, because a new fixture row would rewrite every
committed scenario that reads that population. Its assertion runs both ways: the
frame never exceeds its terminal in cells, and the whole title is recoverable
from the dock's wrapped body, which is what proves a fitter measured cells rather
than runes and that Peek elided nothing.

`docs/tui-testing/scenarios/ux-acceptance-daily-loop.json` gained the beats the
session names and the scenario lacked: `Q` opened over a stacked facet, one axis
peeled by its own digit, the start that offers an optional check-in and the `Esc`
that skips it without losing the start, the completion, the undo that names
`Ctrl+O origin`, and the `Ctrl+O` that spends it.
