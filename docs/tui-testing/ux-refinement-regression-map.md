# UX refinement regression map

Every behavior the UX refinement work plans to change already has a place where
a regression would show. This file names that place for each planned change, so
a later session edits a recorded expectation on purpose instead of discovering
afterwards that nothing was watching.

The map is curated, not derived. A scenario that merely navigates past a row is
not a contract about that row, and only reading the scenarios tells the two
apart — see [Setup navigation versus tested
navigation](#setup-navigation-versus-tested-navigation).

`ux_refinement_regression_map_test.go` in the repository root holds this file to
its own claims: every artifact path named below must exist, and each planned
session must name at least one.

## Baseline

The `ux-acceptance` fixture is the shared "before". It is the one fixture where
the axes every other fixture isolates are simultaneously true: three projects, a
three-generation subtree, titles too long for any supported width, overdue,
due-today, waiting, deferred, review, and pinned attention, a resume note, a
description, tags, a context, a checklist, a comment, an attachment, a
cross-project reference, and a completed task. It is new rather than an
extension of `rich`, because changing `rich` would have rewritten committed
contracts that have nothing to do with this work.

Four committed scenarios record the current product against it:

| Scenario | Records |
| --- | --- |
| `docs/tui-testing/scenarios/ux-acceptance-row-baseline.json` | Lane grouping, the compact project label every cross-project task row carries, the full indicator ladder, and which titles today's row clips. |
| `docs/tui-testing/scenarios/ux-acceptance-peek-baseline.json` | The Peek document's body order, its bounded window and position row, its paging keys, and the named cross-project dependent. |
| `docs/tui-testing/scenarios/ux-acceptance-narrow-baseline.json` | 60×20, 80×24, 100×35, and 120×40 layout health, the footer rank drops, and the narrow Peek window. |
| `docs/tui-testing/scenarios/ux-acceptance-selection-baseline.json` | Peek closing when a selection begins, the multi-select summary, `Ctrl+A` over the visible projection, and `Esc` unwinding. |

Two facts these recorded, which shape the sessions that follow:

- The overdue title is clipped at **every** supported width, 120 columns
  included. Widening the terminal is not the fix; the indicators beside it are
  what cost the room.
- At 100×35 the launch view spent **12 of its 34 rows** on expanded project
  headers — six for one project alone, because each lane opened its own. Session
  8 spent that budget on the tasks instead: the same fixture now draws **22
  rows**, none of them a heading, and the four-task `check-ins` fixture draws
  **8 rows where it drew 12**.

## Clipping is visible now

`normalizeScreen` validates geometry, and a clipped line is inside the terminal
width by construction. So it raised no layout issue, and a `contains` assertion
on the full text failed exactly the way a missing row does. Sessions 2, 5, 8,
10, and 16 all turn on titles and required text surviving a width, which is the
distinction that has to be legible.

Two additions close it, both in the harness:

- Every observation carries `clipped_lines`: the rendered lines a fitter
  truncated, with line numbers.
- The `not_clipped` assertion returns the same verdict `contains` would, and
  differs only in what a failure says — it names the longest prefix of the value
  the screen shows before a `…`.

Clipping stays an observation and never a `UITestIssue`. It is legal today on
rows, chips, and guidance, so promoting it would change what `no_layout_issues`
means on all 266 committed scenarios. It is also the complement of the existing
render audit rather than a copy of it: the audit hears only from call sites that
declared their text required, while this reads the finished screen.

## Per-session regression homes

### Session 2 — Neutral titles and selective attention styling

- Go: `internal/tui/attention_styling_acceptance_test.go`,
  `internal/tui/primary_row_title_style_test.go`,
  `internal/tui/task_title_regression_test.go`,
  `internal/tui/outliner_cursor_highlight_test.go`,
  `internal/tui/theme/styles_helpers_test.go`,
  `internal/tui/theme/semantic_tag_title_style_test.go`,
  `internal/tui/chrome/hero_attribute_test.go`,
  `internal/tui/outliner/tag_chip_style_test.go`,
  `internal/tui/outliner/operational_title_test.go`
- Scenarios: `docs/tui-testing/scenarios/ux-acceptance-row-baseline.json`,
  `docs/tui-testing/scenarios/theme-toggle.json`,
  `docs/tui-testing/scenarios/task-context-row-wide-layout.json`,
  `docs/tui-testing/scenarios/semantic-task-tags-split.json`,
  `docs/tui-testing/scenarios/tags-picker-direct.json`
- Contracts: `CLAUDE.md` — a task title is neutral text and
  `theme.TaskTitleStyleFor` answers lifecycle alone,
  `chrome.PickHeroAttribute` as the overdue > review > due-today > priority-1
  ladder over badges, `theme.OverdueRowStyle` as the one row tint,
  `theme.TagChipStyle` as the single tag color authority, `OrderedTaskTags` as
  the single chip order, the `bug > question > epic > story > feature > idea`
  order as a listing order only, and "a slot is a mnemonic, never a legend".
  `docs/decisions/behavioral-signals-contracts.md` (Task-row indicators),
  `docs/architecture/row-indicators.md`,
  `docs/architecture/selection-and-project-tags.md`,
  `docs/guide/outliner-rows.md`, `docs/guide/outliner-chrome.md`.
- `internal/tui/attention_styling_acceptance_test.go` is where the ANSI evidence
  lives: the matrix renders each combination in dark and light, resting,
  focused, selected, and both, then projects the bytes through an ASCII color
  profile. Its companion holds one title fixed across every signal and requires
  the monochrome rows to stay pairwise distinct, which is what proves each
  signal survives without color rather than merely rendering beside one.

### Session 3 — Peek contains every detail removed from rows

- Go: `internal/tui/outliner/inline_preview_test.go`,
  `internal/tui/outliner/inline_preview_resume_test.go`,
  `internal/tui/outliner/inline_peek_project_document_test.go`,
  `internal/tui/outliner/inline_peek_paging_test.go`,
  `internal/tui/responsive_peek_test.go`,
  `internal/tui/outliner/outliner_render_rows_pressure_test.go`
- Scenarios: `docs/tui-testing/scenarios/ux-acceptance-peek-baseline.json`,
  `docs/tui-testing/scenarios/outliner-peek-scrolling.json`,
  `docs/tui-testing/scenarios/outliner-peek-paging.json`,
  `docs/tui-testing/scenarios/outliner-peek-whole-title.json`,
  `docs/tui-testing/scenarios/outliner-checklist-rows-fold.json`,
  `docs/tui-testing/scenarios/outliner-project-peek-group-header.json`,
  `docs/tui-testing/scenarios/outliner-project-peek-focused-header.json`
- Contracts: `CLAUDE.md` — the Row and Peek budget, in particular "Nothing sits
  outside that window and the document has no header", the stated body
  order (title, parent, status, dates, resume, description, checklist count,
  attachments, comment count, `Dependencies`/`Dependents`, context, tags),
  "place a new section deliberately", "Checklist and comments stay counts; their
  items are rows and their panel", and the unresolvable-reference
  fallback shared with `project export`. `docs/guide/outliner-rows.md`,
  `docs/architecture/search-filters-and-sorting.md`,
  `docs/architecture/row-indicators.md`.

### Session 4 — Dock Peek and unify viewport geometry

- Go: `internal/tui/outliner/peek_dock_test.go` — the dock's fixed height under
  every cursor at 60×20, 80×24, and 120×40, the one reservation opening it
  spends, the placeholder over a row owning no document, and the divider landing
  on the same screen row as the cursor crosses between a one-line task and a
  paged one — `internal/tui/responsive_peek_test.go`,
  `internal/tui/outliner/inline_peek_paging_test.go`,
  `internal/tui/outliner/inline_peek_empty_project_width_test.go`,
  `internal/tui/outliner/empty_project_peek_guidance_test.go`
- Scenarios: `docs/tui-testing/scenarios/ux-acceptance-narrow-baseline.json`,
  `docs/tui-testing/scenarios/ux-acceptance-selection-baseline.json`,
  `docs/tui-testing/scenarios/ux-acceptance-peek-baseline.json`,
  `docs/tui-testing/scenarios/outliner-peek-scrolling.json`,
  `docs/tui-testing/scenarios/outliner-peek-paging.json`,
  `docs/tui-testing/scenarios/outliner-peek-whole-title.json`,
  `docs/tui-testing/scenarios/task-context-reserved-segment-narrow.json`,
  `docs/tui-testing/scenarios/presentation-size-matrix.json`
- Contracts: `CLAUDE.md` — Peek as a dock below the list whose height
  `PeekDockRows` states and `ComputeOutlinerViewport` reserves without reading
  the cursor or the document, that same function as the single window authority
  and the only place a render moves `ViewScrollOffset`, "Never
  recompute the window in a key handler", `StatusStripHeight` reserving the strip,
  `peekOffsetForRow` pure with `scrollPeek` the only writer, the
  `Ctrl+D`/`Ctrl+U` versus `Ctrl+F`/`Ctrl+B` split, `PeekContext.Visible` as
  consumption against `CanPage*` as availability, and `PgDn`/`PgUp` staying the
  cursor's. `docs/guide/outliner-rows.md`,
  `docs/architecture/search-filters-and-sorting.md`,
  `docs/architecture/presentation-chrome.md`.

### Session 5 — Simplify task rows and protect inline editing

- Go: `internal/tui/outliner/outliner_render_rows_test.go`,
  `internal/tui/outliner/outliner_render_indicators_test.go`,
  `internal/tui/outliner/outliner_row_budget_acceptance_test.go`,
  `internal/tui/inline_row_editor_test.go`,
  `internal/tui/inline_row_editor_commit_test.go`,
  `internal/tui/check_in_visual_acceptance_test.go`,
  `internal/tui/outliner_cursor_highlight_test.go`,
  `internal/tui/responsive_peek_test.go`,
  `internal/tui/textfit/text_width_helpers_test.go`
- Scenarios: `docs/tui-testing/scenarios/ux-acceptance-row-baseline.json`,
  `docs/tui-testing/scenarios/ux-acceptance-narrow-baseline.json`,
  `docs/tui-testing/scenarios/check-in-flags-and-stale.json`,
  `docs/tui-testing/scenarios/task-context-reserved-segment-narrow.json`,
  `docs/tui-testing/scenarios/doing-age-narrow.json`
- Contracts: `CLAUDE.md` — `indicatorDropLadder` as a list where a new indicator
  gets a position, the trailing `~S/N` counting stored locators regardless of
  target state, `taskRowSegments` reserving the relation marker and the pin
  marker at every width while the context and tags are Peek's,
  `taskPinnedMarker` reading
  `task.Pinned && !row.IsPinnedSection`, `TruncateToVisualWidth` having no
  counterpart because its dropped tail is not recoverable, and the four-step
  pulse ladder. `docs/decisions/behavioral-signals-contracts.md`,
  `docs/architecture/row-indicators.md`, `docs/guide/outliner-rows.md`.

### Session 6 — Represent project groups independently of header rows

- Go: `internal/tui/outliner/outliner_row_group_outlinersplit_test.go`,
  `internal/tui/outliner/outliner_project_grouping_outlinersplit_test.go`,
  `internal/tui/outliner/outliner_projection_forest_test.go`,
  `internal/tui/outliner/outliner_primary_rows_test.go`,
  `internal/tui/outliner_project_grouping_test.go`,
  `internal/tui/outliner_project_group_toggle_test.go`
- Scenarios: `docs/tui-testing/scenarios/ux-acceptance-row-baseline.json`,
  `docs/tui-testing/scenarios/sort-project-group-order.json`,
  `docs/tui-testing/scenarios/outliner-project-header-fold.json`,
  `docs/tui-testing/scenarios/outliner-fold-all.json`
- Contracts: `CLAUDE.md` — `appendProjectGroupHeader` as the one kernel with no
  per-mode copy, each grouping surface owning a disjoint `CollapsedSections`
  namespace that is never cleared on a mode change, grouping never reordering a
  mode's primary sort, `finalizeModeRows` as the one seam every
  `buildCanonicalModeRows` exit takes, and `OutlinerRowGroup` as what a group is
  — two existing fold keys, no row index, stamped on every row of a block,
  `groupsRowsByProject` as the one predicate the emitter and the metadata share,
  no reconstruction by header scan or lane marker, and `modeCursorAnchor` holding
  no group because two modes may spell one project fold alike.
  `docs/architecture/tui-architecture.md`.

### Session 7 — Decouple folding and navigation from visible headings

- Go: `internal/tui/outliner/outliner_headerless_groups_outlinersplit_test.go`,
  `internal/tui/simplified_navigation_test.go`,
  `internal/tui/input_strategy_navigation_test.go`,
  `internal/tui/outliner_fold_all_test.go`,
  `internal/tui/outliner_section_toggle_test.go`,
  `internal/tui/outliner_project_group_toggle_test.go`,
  `internal/tui/view_navigation_controller_test.go`,
  `internal/tui/outliner/outliner_focused_project_header_test.go`
- Scenarios: `docs/tui-testing/scenarios/outliner-header-jump.json`,
  `docs/tui-testing/scenarios/outliner-fold-all.json`,
  `docs/tui-testing/scenarios/outliner-project-header-fold.json`,
  `docs/tui-testing/scenarios/outliner-project-peek-group-header.json`,
  `docs/tui-testing/scenarios/navigate-outliner.json`,
  `docs/tui-testing/scenarios/outliner-navigation-aliases.json`
- Contracts: `CLAUDE.md` — `s`, `p`, and `t` folding the section, project, and
  task axes, root `w`/`z`/`g` as chord prefixes with cursor-top at `gg` or
  `Home`, `IsCollapsibleHeader` taking no `h`/`l`, `p`, or `Enter`,
  `HasProjectHeaderCursor` still answering for every other project operation,
  and a group staying reachable whether or not its heading is drawn —
  `rowStartsGroup` as `n`/`N`'s stop set, `VisibleSections` and
  `VisibleProjectGroups` as what `s` and `p` fold, `h`'s last rung folding the
  cursor's own group, `l` on a summary landing on the task the fold covered, and
  `foldState.GroupFocus` living exactly as long as that fold.
  `docs/guide/outliner-keys.md`, `docs/architecture/operation-grammar.md`,
  `docs/architecture/tui-architecture.md`.
- Headerless coverage: the model tests reach the headerless presentation
  directly, since `appendProjectGroupHeader` now draws a heading for folded
  groups alone; Session 8 retired the `CompactProjectGroups` switch along with
  the expanded heading it made optional.

### Session 8 — Switch to compact project labels

- Go: `internal/tui/outliner/outliner_render_rows_test.go`,
  `internal/tui/outliner/outliner_render_rows_pressure_test.go`,
  `internal/tui/header_target_audit_test.go`,
  `internal/tui/project_operation_section_header_test.go`
- Scenarios: the full [header-asserting inventory](#scenarios-that-assert-an-expanded-project-header-row)
  below, plus `docs/tui-testing/scenarios/ux-acceptance-row-baseline.json`,
  whose `count_contains` of each project slug is the direct measurement of how
  many header rows a lane spends.
- Contracts: `CLAUDE.md` — `appendProjectGroupHeader` suppressing both
  single-project forms, `IsFocusedProjectHeader` over the owner
  `FocusedProjectHeaderOwner` resolves, `IsCollapsibleHeader` drawing no fold
  glyph and no counts, `HasContentRows` rather than the row count deciding an
  empty projection, `HasProjectHeaderCursor` as the gate every other project
  operation reads, and `outlinerProjectOperationTargetsHeader` narrowing Edit
  project, Add project comment, and Project comments.
  `docs/architecture/selection-and-project-tags.md`,
  `docs/guide/outliner-modes.md`.

### Session 9 — Make all full-form fields reachable by scrolling

- Go: `internal/tui/taskform/task_form_viewport_test.go` — the done-when itself:
  every field of every tab reached by `Tab` and by `Shift+Tab` at the form's own
  floor — 68×18 when session 9 wrote it, 60×18 since session 10 lowered the
  floor to the app's — the
  position row naming what is off screen, a fitting body costing no row, a
  refusal readable beside the field it seats the focus on, a resize in both
  directions, the return from a nested picker, and the pinning rule for a field
  taller than the window — `internal/tui/taskform/task_form_guidance_test.go`,
  `internal/tui/taskform/task_form_schedule_test.go`,
  `internal/tui/taskform/task_form_extras_test.go`,
  `internal/tui/chrome/fullscreen_helpers_test.go`,
  `internal/tui/form_editor_guidance_test.go`
- Scenarios: `docs/tui-testing/scenarios/form-viewport-minimum-height.json`,
  `docs/tui-testing/scenarios/form-extras-attachments-narrow.json`,
  `docs/tui-testing/scenarios/form-extras-comments-narrow.json`,
  `docs/tui-testing/scenarios/task-form-edit-standard-layout.json`
- Contracts: `CLAUDE.md` — a value provider being exactly one of `TextEntry()`,
  `TagEntry()`/`DateEntry()`, or a plain list, "a value the presets cannot
  express belongs in an inline mode on that picker, never a standalone form",
  and the form viewport: `formBodyBuilder` recording a field once where it
  starts, `formViewportOffset` as the pure window authority, `scrollFormBody`
  spending a row only on a body that overflows, `clampFormBody` reduced to the
  last defensive cut, and the width guard retained for session 10.
  `docs/architecture/operation-grammar.md`,
  `docs/architecture/capture-and-outline-editor.md`, `docs/guide/capture.md`.

### Session 10 — Make forms and nested pickers work at 60 columns

- Go: `internal/tui/taskform/task_form_basics_test.go`,
  `internal/tui/taskform/task_form_children_test.go`,
  `internal/tui/taskform/check_in_editor_test.go`,
  `internal/tui/chrome/fullscreen_title_fitting_test.go`,
  `internal/tui/narrow_layout_helpers_test.go`
- Scenarios:
  `docs/tui-testing/scenarios/form-extras-attachments-minimum-layout.json`,
  `docs/tui-testing/scenarios/form-minimum-width-tabs-and-pickers.json`,
  `docs/tui-testing/scenarios/date-picker-narrow-resize-key-confinement.json`,
  `docs/tui-testing/scenarios/defer-picker-minimum-layout.json`,
  `docs/tui-testing/scenarios/task-form-edit-minimum-layout.json`
- Contracts: `CLAUDE.md` — `ValueProviderCheckIn` opening one two-field draft
  where bulk use starts blank and labels the replacement, the `@` picker as the
  one date picker over Due, Defer, and Check-in, `Tab`/`Shift+Tab` landing on the
  adjacent section's first match including disabled rows, the preset key as one
  `chrome` table read by field and preset ID, and the overlay reserving the
  guidance it rendered rather than a fixed row count.

### Session 11 — Present capture as a contextual bottom modal

- Go: `internal/tui/chrome/hosted_panel_test.go`,
  `internal/tui/taskform/task_composer_test.go`,
  `internal/tui/taskform/task_composer_expand_test.go`,
  `internal/tui/taskform/task_composer_seed_test.go`,
  `internal/tui/app_view_test.go`,
  `internal/tui/presentation_chrome_policy_test.go`,
  `internal/tui/chrome/capture_footer_test.go`,
  `internal/tui/capture_footer_test.go`
- Scenarios: `docs/tui-testing/scenarios/capture-bottom-panel.json`,
  `docs/tui-testing/scenarios/capture-review.json`,
  `docs/tui-testing/scenarios/capture-dependency-cross-project.json`,
  `docs/tui-testing/scenarios/capture-grammar.json`,
  `docs/tui-testing/scenarios/capture-attachment.json`,
  `docs/tui-testing/scenarios/capture-comment.json`,
  `docs/tui-testing/scenarios/capture-project-header-destination.json`,
  `docs/tui-testing/scenarios/capture-due-time.json`
- Contracts: `CLAUDE.md` — the hosted panel: `route.HostedPanelView` and
  `chrome.HostedPanelLayout` as the one seam a view declares a bounded bottom
  presentation through, `HostRows` being a pure function of the frame, the panel
  never painting a title or a pulse because its host keeps painting both, the
  bounded preview taking the compact run rather than a clipped group list, and
  the expanded composer still owning the whole content area. Also the standing
  capture rules this session preserved: transient input overlays being exempt
  from the one-key-one-act rule, `&link` and `;comment;` riding the capturing
  task's own ChangeSet so one undo entry covers task, bytes, and comments,
  multiline capture validating every nonblank line before committing one
  transaction, and the composer preset carrying a typed seed rather than
  `InitialText`. `docs/guide/capture.md`,
  `docs/architecture/capture-and-outline-editor.md`.

### Session 12 — Make ordinary slash search ranked by default

- Go: `internal/tui/outliner/outliner_search_projection_outlinersplit_test.go` —
  the two presentations being declared at the open rather than by the query:
  slash opening the flat ranked list across the mode's lanes while
  `SetSearchQuery` keeps the narrowing, and one return frame surviving a repeated
  slash so a deliberate `or` holds and `Esc` still hands back the section zoom the
  ranked open suspended. Also `internal/tui/outliner/outliner_search_tiers_test.go`,
  `internal/tui/outliner/outliner_search_gate_outlinersplit_test.go`,
  `internal/tui/outliner/outliner_search_notes_tier_outlinersplit_test.go`, which
  reach the ranked list through `searchRankedList` and so pin the tiers, the gate,
  and the notes tier independently of which presentation the key chooses.
- Scenarios: `docs/tui-testing/scenarios/search-compose-filters.json` and
  `docs/tui-testing/scenarios/outliner-header-jump.json` carry the round trip —
  slash showing no lanes and no section header, `or` narrowing the same query back
  into the mode, and both readings naming the same results. The rest record the
  new default at the point they already searched:
  `docs/tui-testing/scenarios/outliner-search-sigils.json`,
  `docs/tui-testing/scenarios/outliner-search-notes-tier.json`,
  `docs/tui-testing/scenarios/outliner-search-notes-scoped.json`,
  `docs/tui-testing/scenarios/outliner-rich-search-autocomplete.json`,
  `docs/tui-testing/scenarios/outliner-search-focus-jump.json`,
  `docs/tui-testing/scenarios/outliner-focus-esc-priority.json`,
  `docs/tui-testing/scenarios/sort-fixed-order-modes.json`,
  `docs/tui-testing/scenarios/stale-lane-and-filter.json`,
  `docs/tui-testing/scenarios/project-selection-one-axis.json`,
  `docs/tui-testing/scenarios/search-context-repeats-union.json`,
  `docs/tui-testing/scenarios/outliner-set-parent-picker.json`,
  `docs/tui-testing/scenarios/outliner-set-parent-detach.json`,
  `docs/tui-testing/scenarios/outliner-set-parent-disabled-rows.json`,
  `docs/tui-testing/scenarios/outliner-indent-outdent-brackets.json`,
  `docs/tui-testing/scenarios/task-context-row-wide-layout.json`,
  `docs/tui-testing/scenarios/filter-tag-merge-undo.json`,
  `docs/tui-testing/scenarios/check-in-integrated-lifecycle.json`,
  `docs/tui-testing/scenarios/ux-acceptance-narrow-baseline.json`.
  `docs/tui-testing/scenarios/search-restoration.json` and
  `docs/tui-testing/scenarios/outliner-search-history-recall.json` are the two
  that had to keep passing untouched: the return frame and the recall ring belong
  to the open, not to the presentation it declares.
- Contracts: `CLAUDE.md` — the slash key opening the ranked list while
  `BeginSearchProjection` opens the narrowing, `SetSearchQuery` never gaining the
  ranked open, and the open declaring the presentation in the same act it captures
  the return frame, so a repeated slash restates neither. Also the standing search
  rules this session preserved: slash search as the fourth surface of the same
  registry rather than a parallel query language, sigils compiling to an AND gate
  applied before scoring with the case-relaxed retry re-running that same gate,
  the notes tier as the lowest and omitted whenever the query carries a sigil,
  and `fs` being unavailable in Recent. `docs/guide/search-and-lenses.md`,
  `docs/guide/outliner-keys.md`,
  `docs/architecture/search-filters-and-sorting.md`,
  `docs/architecture/canonical-query.md`.

### Session 13 — Suspend and restore lenses during temporary search

- Go: `internal/tui/outliner_lens_lifecycle_test.go`,
  `internal/tui/outliner_lens_scope_exit_test.go`,
  `internal/tui/app_launch_lens_test.go`,
  `internal/tui/outliner/outliner_presentation_checkpoint_outlinersplit_test.go`
- Scenarios: `docs/tui-testing/scenarios/named-lens-escape-restoration.json`,
  `docs/tui-testing/scenarios/named-lens-from-search-results.json`,
  `docs/tui-testing/scenarios/lens-rename-resume-undo-order.json`,
  `docs/tui-testing/scenarios/goto-relation-lens-round-trip.json`
- Contracts: `CLAUDE.md` — a lens owning project scope where an empty scope means
  the lens does not own that axis, an active-lens pointer that no longer resolves
  failing closed silently, suspension being `forgetActiveLensFrame` so navigating
  writes no lens definition and no machine-local pointer, and `Esc` spending the
  return frame after Peek and selection and before query, lens, focus, and scope.
  `docs/architecture/named-lenses.md`, `docs/guide/search-and-lenses.md`.

### Session 14 — Generalize mutation cursor recovery

- Go: `internal/tui/async_reload_navigation_test.go`,
  `internal/tui/outliner/task_locator_coordinator_test.go`,
  `internal/tui/outliner/outliner_relocation_recovery_outlinersplit_test.go`,
  `internal/tui/outliner/outliner_review_advance_test.go`
- Scenarios:
  `docs/tui-testing/scenarios/inbox-triage-advance-across-project-groups.json`,
  `docs/tui-testing/scenarios/inbox-triage-advance-promotes-children.json`,
  `docs/tui-testing/scenarios/inbox-triage-undo-does-not-advance.json`,
  `docs/tui-testing/scenarios/review-acknowledge-and-advance.json`,
  `docs/tui-testing/scenarios/recovery-origin-guidance.json`,
  `docs/tui-testing/scenarios/workflow-chord-destinations.json`,
  `docs/tui-testing/scenarios/combined-promote-deadline-step-move.json`
- Contracts: `CLAUDE.md` — retention being group-scoped rather than a bare
  visibility test, the recovery order of promoted child then following then
  preceding task, `R`'s successor sequence being captured before the commit and
  rechecked after it, expanding no fold and never wrapping, "a refusal advances
  nothing; undo takes the mutation back and leaves the cursor", the periodic
  tick's invalidation remaining owed under an overlay and restoring the cursor by
  qualified task identity, and `revealCommittedTask` opening the child's ancestor
  chain alone. `docs/guide/outliner-modes.md`.

### Session 15 — Make undo focus and feedback match committed work

- Go: `internal/tui/coordinator/undo_focus_controller_test.go`,
  `internal/tui/handler_task_undo_identity_test.go`,
  `internal/tui/outliner/outliner_jump_history_outlinersplit_test.go`,
  `internal/tui/undo_context_title_test.go`,
  `internal/tui/workspace_ledger_undo_test.go`
- Scenarios: `docs/tui-testing/scenarios/undo-focus-committed-target.json`,
  `docs/tui-testing/scenarios/undo-focus-restart-uncorrelated.json`,
  `docs/tui-testing/scenarios/cleanup-undo-redo.json`,
  `docs/tui-testing/scenarios/outliner-move-to-project-bulk-undo.json`,
  `docs/tui-testing/scenarios/outliner-project-delete-undo.json`,
  `docs/tui-testing/scenarios/outliner-task-reference-undo.json`
- Contracts: `CLAUDE.md` — undo/redo as one 50-entry workspace ledger where a
  successful forward mutation clears redo and a failure records nothing, a lens
  mutation recording no ledger entry so `u` never takes a lens back,
  `supersedeDueStepsForEffect` reading `Edits`/`Removed` and never `Steps`, a
  step's toast naming the outcome through `RecoveryContext`, and the new
  paragraph on `UndoFocusController`: bounded session-local pairs keyed by
  transaction ID whose stacks are a subsequence of the ledger's, the seams that
  may stamp a single target, `RefocusRestoredTask` seating the cursor only on a
  task already drawn and refusing under a visual selection, and the restore
  naming the location otherwise.
  `docs/architecture/persistence-and-transactions.md`,
  `docs/architecture/tui-architecture.md`,
  `docs/architecture/presentation-chrome.md`, `docs/guide/outliner-chrome.md`.

### Session 16 — Quiet shell chrome and make Escape guidance accurate

- Go: `internal/tui/footer_ranked_guidance_test.go`,
  `internal/tui/footer_flash_test.go`,
  `internal/tui/chrome/footer_helpers_test.go`,
  `internal/tui/narrow_layout_helpers_test.go`,
  `internal/tui/outliner_escape_guidance_test.go` (every profile's `Esc` entry
  read off the cascade, then pressed; and every profile's required entries held
  to `chrome.MinTerminalWidth`),
  `internal/tui/chrome/escape_vocabulary_test.go` (the shared draft vocabulary
  and the same width floor for the editor profiles)
- Scenarios: `docs/tui-testing/scenarios/ux-acceptance-narrow-baseline.json`,
  `docs/tui-testing/scenarios/presentation-size-matrix.json`,
  `docs/tui-testing/scenarios/in-app-help.json`,
  `docs/tui-testing/scenarios/goto-relation-esc-order.json`,
  `docs/tui-testing/scenarios/outliner-escape-guidance-ladder.json` (the whole
  ladder walked at 60×20, plus the draft/discard wording on the composer)
- Contracts: `CLAUDE.md` — "A footer has one source: the strategy's ranks, or the
  gated keymap where modal sub-states narrow a key", notice and status rendering
  only in the shell-pinned strip whose height `StatusStripHeight` reserves, every
  advisory routing through `AdvisoryLine`/`AdvisoryStatus` with "Never gate a row
  at its render site", only status text arming footer flash, the `Esc`
  unwind order, and the new sentence making `EscSteps`' own `Action` the single
  authority every profile, sentence, and footer entry reads.
  `docs/guide/outliner-chrome.md`,
  `docs/architecture/presentation-chrome.md`.

### Session 17 — Render menus over a dimmed host

- Go: `internal/tui/menu_bar_render_test.go` (the host compared column by column
  beside the open box, the frame after the close compared to the frame before the
  open, and `LastViewport` held across open, move, and close),
  `internal/tui/menu_bar_test.go`, `internal/tui/menu_bar_parity_test.go`,
  `internal/tui/chrome/overlay_popup_test.go` (`OverlayPopupDimmedAt`: the rows
  above the anchor kept but dimmed, the box at its column, and a popup too tall
  for what the anchor leaves fitted rather than clipped),
  `internal/tui/outliner/menu_bar_render_outlinersplit_test.go`
- Scenarios: `docs/tui-testing/scenarios/menu-over-dimmed-host.json` (the menu
  over an open Peek dock, over a two-row visual selection, and across a resize to
  60×20 and back, plus the bulk sheet dimming the same host),
  `docs/tui-testing/scenarios/outliner-menu-bar.json`,
  `docs/tui-testing/scenarios/outliner-menu-bar-persist.json`,
  `docs/tui-testing/scenarios/goto-relation-walk.json` (the relation picker
  keeping the title at both sizes),
  `docs/tui-testing/scenarios/outliner-space-action-catalog.json`,
  `docs/tui-testing/scenarios/action-sheet-picker-cancel.json`
- Contracts: `CLAUDE.md` — an entity operation being direct-only or menu-only and
  never both in that surface's sheet, the menu bar as a projection of the
  registry rather than a sheet, `M` costing one list row subtracted in
  `ContentHeight` alone, the new sentences making `composeOutlinerModal` the one
  seam both view-local modals take, `chrome.OverlayPopupDimmedAt` the compositing,
  the header rows the anchor reserves uncoverable, and a modal covering rows
  rather than displacing them, the row being exactly `m.width` and dropping whole
  labels, and `MenuBarVisible` being `MenuBarPinned() || MenuBar.Open`.
  `docs/architecture/operation-grammar.md`,
  `docs/architecture/presentation-chrome.md`, `docs/guide/outliner-chrome.md`.

### Session 18 — Complete integrated acceptance and final review

- Go: `internal/tui/uitest_example_scenarios_test.go` (the committed-inventory
  gate and the replay of every scenario), `internal/tui/uitest_clipping_test.go`
- Scenarios: all four `ux-acceptance-*` baselines,
  `docs/tui-testing/scenarios/presentation-size-matrix.json`,
  `docs/tui-testing/scenarios/daily-loop.json`
- Contracts: `CLAUDE.md` in full, plus `docs/baselines/baseline.md`, whose
  committed-scenario count is gated by `TestBaselineDocumentMatchesMeasuredGates`
  and must be updated in the same change that adds or removes a scenario.

## Setup navigation versus tested navigation

Session 8 removes expanded project-header rows, which changes how many `down`
presses reach a given task. Two populations are affected, and they need
different treatment.

### Scenarios that assert an expanded project header row

These name a project slug in a `contains`, `not_contains`, or `count_contains`
value, so the header is the contract and the assertion has to be rewritten
deliberately:

- `docs/tui-testing/scenarios/action-sheet-picker-cancel.json`
- `docs/tui-testing/scenarios/combined-promote-deadline-step-move.json`
- `docs/tui-testing/scenarios/form-basics-context-from-capture.json`
- `docs/tui-testing/scenarios/inbox-triage-advance-across-project-groups.json`
- `docs/tui-testing/scenarios/inbox-triage-advance-promotes-children.json`
- `docs/tui-testing/scenarios/inbox-triage-undo-does-not-advance.json`
- `docs/tui-testing/scenarios/move-project-search-navigation.json`
- `docs/tui-testing/scenarios/named-lens-scope-deleted-project.json`
- `docs/tui-testing/scenarios/outliner-focus-parent.json`
- `docs/tui-testing/scenarios/outliner-focused-project-header.json`
- `docs/tui-testing/scenarios/outliner-fold-all.json`
- `docs/tui-testing/scenarios/outliner-inline-edit-dependency-undo.json`
- `docs/tui-testing/scenarios/outliner-move-to-project-bulk-narrow-layout.json`
- `docs/tui-testing/scenarios/outliner-move-to-project-bulk-undo.json`
- `docs/tui-testing/scenarios/outliner-move-to-project-parked.json`
- `docs/tui-testing/scenarios/outliner-move-to-project-subtree.json`
- `docs/tui-testing/scenarios/outliner-move-to-project-undo.json`
- `docs/tui-testing/scenarios/outliner-project-delete-undo.json`
- `docs/tui-testing/scenarios/outliner-project-header-actions.json`
- `docs/tui-testing/scenarios/outliner-project-header-fold.json`
- `docs/tui-testing/scenarios/outliner-project-header-rejects-task-keys.json`
- `docs/tui-testing/scenarios/outliner-project-peek-focused-header.json`
- `docs/tui-testing/scenarios/outliner-project-peek-group-header.json`
- `docs/tui-testing/scenarios/outliner-project-pin.json`
- `docs/tui-testing/scenarios/project-comments-panel-integration.json`
- `docs/tui-testing/scenarios/project-header-pressure-narrow.json`
- `docs/tui-testing/scenarios/project-header-pressure.json`
- `docs/tui-testing/scenarios/project-park-confirm-and-recover.json`
- `docs/tui-testing/scenarios/project-tags-picker.json`
- `docs/tui-testing/scenarios/recent-focus-latest.json`
- `docs/tui-testing/scenarios/sort-project-group-order.json`
- `docs/tui-testing/scenarios/ux-acceptance-row-baseline.json`

Not all of these are about headers as such. `outliner-project-header-actions`,
`outliner-project-header-fold`, `outliner-project-header-rejects-task-keys`,
`outliner-project-peek-group-header`, `project-header-pressure`, and
`project-header-pressure-narrow` test the header itself, and their contracts move
to whatever replaces it. The rest name a slug only to prove which project a task
belongs to, and a compact label satisfies them once the expected text changes.

### Scenarios whose navigation is setup only

176 committed scenarios press `down` or `j` at least once, and 145 press it twice
or more:

```sh
grep -lc '"key": "\(down\|j\)"' docs/tui-testing/scenarios/*.json
```

Most of those presses are getting to a row, not testing that arriving works. When
a removed header changes a count, fix the count — do not convert the setup into a
harness command or a direct cursor assignment. Three reasons:

- No committed scenario sets the cursor today. The 17 distinct harness `command`
  actions in the suite are all workspace or overlay setup with no public
  shortcut; adding a cursor-setting one would make session 8 the change that
  quietly retired public-key navigation coverage.
- 73 Go test files in `internal/tui` and `internal/tui/outliner` already assign
  `.Cursor` or call `SetCursorToID` directly. A row-count change is invisible to
  every one of them, which is exactly why the scenarios have to keep pressing
  keys.
- The scenarios where arriving *is* the contract —
  `docs/tui-testing/scenarios/navigate-outliner.json`,
  `docs/tui-testing/scenarios/outliner-navigation-aliases.json`,
  `docs/tui-testing/scenarios/outliner-header-jump.json`, and
  `docs/tui-testing/scenarios/outliner-range-selection.json` — must keep public
  keys and gain the new presentation's expectations rather than be relaxed.

## Preserved through all of it

Two things the plan does not touch, recorded here because a refactor can erode
them without any test objecting:

- The seven-mode catalog stays as it is;
  `docs/tui-testing/scenarios/mode-catalog.json` is its home.
- The check-in behavior settled just before this work —
  `models.CheckInSchedule` as one value, `NextCheckInAfter` counting from the
  review, `SetCheckIn` leaving the review flag and clock alone, calendar-day dates
  surviving a timezone transition, and bulk replacement starting from an empty
  schedule — stays as it is. Its homes are
  `docs/tui-testing/scenarios/check-in-integrated-lifecycle.json`,
  `docs/tui-testing/scenarios/check-in-flags-and-stale.json`,
  `docs/tui-testing/scenarios/active-work-check-in-offers.json`, and
  `internal/tui/outliner/check_in_calendar_acceptance_test.go`.
