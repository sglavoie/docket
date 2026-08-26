# Lean Search and Notes Restoration

## Context

Restore the valuable parts of retired discovery features without adding views, routes, persisted fields, or a universal Finder:

- Enrich the existing Outliner task search and add grammar-aware autocomplete.
- Expand Select Project search across project metadata only.
- Restore a responsive Notes body preview.
- Preserve current navigation, scope, and `Esc` semantics.

The current code already has the important foundations this work must extend rather than replace:

- `internal/tui/outliner_search_projection.go` owns scope-wide task search, exact qualified references, smart-case retry, title-first ranking, and restoration of the ordinary Outliner projection.
- `internal/tui/outliner_search_predicates.go` compiles sigil predicates through `textgrammar.ParseSearch`.
- `internal/textgrammar/grammar.go` and `internal/textgrammar/search.go` are the canonical `SurfaceSearch` vocabulary and search-only preset/example sources.
- `internal/tui/autocomplete.go` already provides longest-trigger detection, cursor-aware insertion, bounded dropdown rendering, and overflow rows.
- `internal/tui/list_surface_view.go` owns the 72-column wide-layout threshold, shared 55/45 geometry, preview header/body hook, pane joiner, and continuation-row styling.
- `internal/tui/project_hub.go`, `internal/tui/notes_view.go`, `internal/tui/notes_view_render.go`, and `internal/tui/notes_view_update.go` retain separate Project Hub and Notes search surfaces.

## Scope and constraints

- No storage, task, project, note, machine-state, CLI, route, or migration changes.
- `/`, `P`, and Notes remain separate discovery surfaces; no universal Finder or workspace-wide search overlay returns.
- Existing `Ctrl+O`/`Ctrl+I`, scope stacks, search landing, and `Esc` restoration behavior remain unchanged.
- Do not restore dot-repeat, review workflows, dashboards, alerts, Kanban, saved filters, recurrence, dependencies, decisions, templates, scoring, automation, AI, or retired Notes tags/links/pins/multi-selection.
- Search autocomplete is Outliner-only; Project Hub and Notes remain ordinary fuzzy text searches.
- Reuse `textgrammar.SurfaceSearch` and shared autocomplete/list-preview primitives. Do not introduce parallel trigger, preset, geometry, or paging grammars.
- Preserve current source-order tie breaking, lifecycle visibility, focus boundaries, mutation refresh behavior, and responsive minimum-height guarantees.

## Sequencing

The sessions are serial and intentionally converge in six steps:

1. establish authoritative match tiers and source attribution;
2. render that attribution without destabilizing task rows;
3. add autocomplete on top of the settled search model;
4. restore the responsive Notes pane and paging foundation;
5. make the Notes pane search-aware and harden invalidation/help behavior;
6. expand Project Hub metadata aliases and close the work through committed scenarios and the full CI gate.

Sessions 1-3 share the Outliner search model. Sessions 4-5 share Notes preview state. Session 6 is the cross-surface acceptance gate and may repair integration defects discovered by the scenarios or CI, but it must not broaden the feature scope.

## Risks

- Autocomplete could appear correct while owning a parallel vocabulary that drifts when `SurfaceSearch` changes. Keep triggers, cardinality, values, clears, presets, and examples projected from `internal/textgrammar`, and extend the consumer-contract tests so a second registry fails loudly.
- Suggestion or preview rows could render outside the measured height budget, clipping the selected row or footer at width 72 or at minimum heights. Boundary unit tests and TUI `no_layout_issues` assertions must cover the actual wide/narrow transition and constrained heights.
- Cached autocomplete candidates or Notes anchors could remain plausible but stale after mutations, query edits, scope changes, note revisions, cursor moves, or resizes. Tests must exercise every invalidation axis and prove manual Notes paging remains stable only while the full anchor key is unchanged.

## Session 1: Outliner multi-field matching and source attribution

### Goal

Extend plain-text `/` matching across all approved task and project fields with deterministic ranking tiers while preserving every existing search invariant. Carry the winning non-title source into each projected task row for Session 2 to render.

### Files and current seams

- `internal/tui/outliner_search_projection.go`: `outlinerSearchMatch`, `searchProjectionMatchesWithin`, `scoreSearchProjectionTask`, exact qualified-ID lookup, stable sort, and projection row construction.
- `internal/tui/outliner_view_rows.go`: `outlinerRow`, the transient presentation record that may carry match attribution without changing persisted models.
- `internal/models/task.go` and `internal/models/comment.go`: authoritative checklist, comment, attachment, tag, context, and description fields; read-only model references, not storage changes.
- `internal/tui/outliner_search_projection_test.go` and `internal/tui/outliner_search_predicates_test.go`: existing search invariants and focused regression coverage.

### Implementation

- Add a small transient match-source type for title, description, checklist, comment, tag, context, attachment, project, and combined details. Do not add a persisted task field.
- Score free text in these deterministic tiers:
  1. title;
  2. description;
  3. checklist text;
  4. comments;
  5. tags and context;
  6. attachment filenames/URLs and original paths;
  7. project name, slug, description, and qualified task reference;
  8. combined details when query terms span fields.
- Within the tags/context tier, retain which axis produced the best match so Session 2 can distinguish `tag` from `context`. Resolve equal scores deterministically by the declared field order.
- Evaluate combined details only after every individual tier fails; it exists for multi-term queries whose terms are satisfied across fields and must never outrank a single-field match.
- Keep title-first field rank, descending fuzzy score within a tier, and catalog/task source order as the final stable tie breaker.
- Preserve exact `project#ID` lookup as an exact path rather than allowing generic fuzzy matching to weaken it.
- Preserve `textgrammar.ParseSearch` sigil predicates, sigil-only searches, smart-case retry, terminal-task admission, Focus candidate boundaries, loaded project scope, and project-prefix behavior.
- Carry the match source from `outlinerSearchMatch` into the `outlinerRow` produced by `buildSearchProjectionRows`. Ordinary rows must retain the zero/no-cue source.
- Ensure every rebuild after task/project mutations recomputes both match membership and the source attribution.

### Tests

- Cover each tier independently, including checklist completion state not affecting text matching, comment text, attachment filename/URL/original path, project metadata, and qualified reference text.
- Cover tag versus context attribution inside their shared rank.
- Cover multi-term queries spanning two or more fields and prove a single-field match wins over combined details.
- Preserve smart-case narrowing and case-insensitive fallback, dense fuzzy fallback, sigil-only searches, exact qualified IDs, terminal visibility, Focus/scope boundaries, stable ties, and mutation-driven refresh.
- Assert ordinary non-search rows carry no source attribution.

### Verification

Run `go test -count=1 ./internal/tui`.

### Done when

- Every approved field is searchable at its specified rank.
- Cross-field queries work only through the final combined-details tier.
- Existing search projection, landing, restoration, predicate, and lifecycle tests remain green.
- Each projected result carries the correct transient source for Session 2.

## Session 2: Outliner non-title source cues and width pressure

### Goal

Render one muted explanation for non-title matches without changing title emphasis, cursor semantics, row width, or the established task-indicator degradation order.

### Files and current seams

- `internal/tui/outliner_render_rows.go`: `taskRowSegments`, `buildTaskRowSegments`, `titleWidth`, `content`, `indicatorDropLadder`, and `renderOutlinerTaskRow`.
- `internal/tui/outliner_view_rows.go`: Session 1 match-source value on each row.
- `internal/tui/outliner_render_rows_test.go`, `internal/tui/outliner_cursor_highlight_test.go`, and `internal/tui/responsive_peek_test.go`: row budgeting, cursor paint, and responsive degradation coverage.

### Implementation

- Map the Session 1 source to exactly one cue: `· description`, `· checklist`, `· comment`, `· tag`, `· context`, `· attachment`, `· project`, or `· details`.
- Render no cue for a title match, a zero source, or an ordinary non-search row.
- Add the cue as a distinct task-row segment so ANSI-aware width measurement charges it exactly once.
- Render it with the existing muted style and preserve the Outliner's split cursor paint: the cue remains content, while the gutter and trailing fill continue to mark the cursor.
- Make the cue the first search-only segment removed when the row cannot retain a readable title. It must disappear before any member of the existing `indicatorDropLadder`; do not reorder existing indicators.
- Preserve current context/tag segments, relationship markers, Peek title overflow, column alignment, zebra striping, and visual-width truncation for wide runes.

### Tests

- Render every cue label and assert title matches render none.
- Cover selected and unselected rows in both themes without losing the cursor bar/trailing fill or semantic title color.
- Verify visual width for narrow terminals and Unicode titles.
- Construct pressure cases proving the cue drops before checklist, comment, attachment, due, defer, waiting, doing, and check-back indicators.
- Prove Peek and project preview rows without search attribution remain unchanged.

### Verification

Run `go test -count=1 ./internal/tui`.

### Done when

- Every non-title result explains its source at comfortable widths.
- The cue is the first expendable search segment under width pressure.
- Cursor styling, title visibility, existing indicators, and preview rendering remain regression-free.

## Session 3: Grammar-derived Outliner search autocomplete

### Goal

Add Outliner-only autocomplete whose vocabulary and cardinality are projections of `textgrammar.SurfaceSearch`, with scoped dynamic values, cursor-aware acceptance, result navigation, bounded rendering, and the required Esc cascade.

### Files and current seams

- `internal/textgrammar/grammar.go` and `internal/textgrammar/search.go`: `Definitions(SurfaceSearch)`, cardinality, clears, `SearchPresetValues`, and `SearchExampleValues`.
- `internal/tui/autocomplete.go`: `AutocompleteState`, `DetectTrigger`, `BuildAcceptedInputAt`, and `RenderDropdownWithLimit`.
- `internal/tui/outliner_search_autocomplete.go` and `internal/tui/outliner_search_autocomplete_test.go`: new focused implementation and regression-test files; keep autocomplete-specific state and helpers out of the projection scorer and generic autocomplete primitives.
- `internal/tui/outliner.go`: Outliner session-only state and size/reset lifecycle.
- `internal/tui/outliner_keys.go`: active-search input handling.
- `internal/tui/outliner_render_content.go`: Outliner header, extra-row accounting, and viewport calculation.
- `internal/tui/outliner_keys_hints.go`: search interaction guidance.
- `internal/tui/app_simplified_input.go`: inline-control gate that must allow existing Enter result landing while suggestions are open.
- `internal/tui/textgrammar_consumer_contract_test.go`: AST/consumer guards against parallel grammar lists.

### Implementation

- Add a focused Outliner search-autocomplete module and state; do not add autocomplete to Project Hub or Notes.
- Build triggers from `textgrammar.Definitions(textgrammar.SurfaceSearch)` and use the longest matching trigger at the text-input cursor.
- Generate suggestions as follows:
  - `!`: priorities 1-3 from the search definition and shared priority parsing/names;
  - `#` and `-#`: normalized tags present on candidate tasks;
  - `##`: normalized contexts present on candidate tasks;
  - `@`: search due presets plus the rolling-hour search example;
  - `@@`: search deferral presets plus supported definition date/hour values and examples;
  - `$`: shared workflow destinations plus Today and Next;
  - `*`: the search surface's pin and unpin values/clears;
  - `%`: projects in the currently loaded search scope.
- Build dynamic tag/context values from the same Focus-bounded candidate tasks search can inspect. Build project values from the loaded scope search can inspect; never load a wider catalog just for completion.
- Rank dynamic values with existing fuzzy helpers and preserve stable source order for ties.
- Parse the settled query prefix with `SurfaceSearch`: exclude already-used repeatable tag/remove-tag values and suppress a singleton field once accepted. Treat context, priority, due, defer, destination, pin, and project as singletons according to the grammar definition rather than a parallel field list.
- Preserve text after the cursor when Tab accepts a suggestion and place the cursor after the inserted token and trailing space.
- While suggestions are open:
  - Up/Down and Ctrl+N/P move the suggestion cursor;
  - Tab accepts the suggestion;
  - j/k navigate search results through the existing list-navigation behavior;
  - Enter bypasses autocomplete and reaches the existing Outliner result-landing/focus operation;
  - Esc closes suggestions, while the next Esc clears/restores search.
- Render at most five total autocomplete rows, including overflow counts, using `RenderDropdownWithLimit`.
- Measure the rendered dropdown's actual height and subtract it from the Outliner row viewport so suggestions never push the cursor row or footer off screen.
- Clear/recompute autocomplete when search closes, scope/Focus/candidates change, a singleton is accepted, or the active token no longer has suggestions.
- Update guidance to describe suggestion controls only while the dropdown is active and retain ordinary search/result guidance otherwise.

### Tests

- Cover all triggers, especially longest matches for `#`/`##`, `@`/`@@`, and `-#`.
- Prove every static value comes through the search grammar projection and add a consumer contract that rejects a second trigger/value registry.
- Cover Focus/scoped candidate values, mutation refresh, normalized tags/contexts, project scope, stable ranking, used repeatables, accepted singletons, and empty candidate sets.
- Cover mid-query insertion with Unicode before/after the cursor and preservation of trailing tokens.
- Cover Up/Down, Ctrl+N/P, Tab, j/k, Enter landing, suggestion-first Esc, search-second Esc, and behavior when suggestions close because input changes.
- Cover five-row top/middle/bottom overflow rendering, actual-height viewport subtraction, width truncation, and minimum-height layouts.

### Verification

Run `go test -count=1 ./internal/tui`.

### Done when

- Autocomplete exposes exactly the supported `SurfaceSearch` grammar and scoped values.
- Key ownership matches the approved interaction model without changing ordinary search landing or restoration.
- The dropdown remains bounded and the Outliner cursor/footer remain visible at every tested size.

## Session 4: Responsive Notes preview and paging foundation

### Goal

Restore a responsive, pageable Notes body preview using the existing list-preview geometry and rendering helpers, while keeping narrow terminals list-only.

### Files and current seams

- `internal/tui/list_surface_view.go`: `listSurfacePreviewGeometry`, `renderListSurfacePreview`, `joinListSurfacePanes`, and `renderListContinuation`.
- `internal/tui/notes_view.go`: Notes state, selection, reload, scope, and resize lifecycle.
- `internal/tui/notes_view_render.go`: current full-width list rendering and header/footer height accounting.
- `internal/tui/notes_view_update.go`: browse/search key ownership and cursor changes.
- `internal/tui/notes_keymap.go`: Notes browse/help key declarations.
- `internal/tui/markdown.go`: shared Markdown rendering and ANSI-aware width bounding.
- `internal/models/note.go`: title, body, modification time, parse error, filename, and raw revision.
- `internal/tui/notes_preview.go` and `internal/tui/notes_preview_test.go`: new focused preview state/rendering module and its regression suite.
- `internal/tui/notes_hardening_test.go` and a new focused preview test file: minimum-layout and preview behavior.

### Implementation

- Add a Notes preview state/module that owns the selected note identity, rendered-line cache, offset, and render-derived page size. Keep it session-only.
- Resolve wide/narrow behavior through `listSurfacePreviewGeometry` with the existing defaults: width 72 and above may show the 55/45 list-preview split; narrower widths render the current list only.
- Refactor Notes list rendering to accept the resolved list-pane width rather than reading the full view width internally.
- Join the panes with the shared pane joiner and use the shared preview renderer for the header/body separator.
- Preview the selected note's title, owner, and formatted modification time above the Markdown body.
- Render the body through `renderMarkdown` at the actual preview width and split the styled result into bounded display lines.
- Show an explicit muted empty-body state for a successfully parsed note with no body.
- Show an explicit warning state for `Note.ParseError`; do not attempt to render unreliable note metadata/body as valid Markdown.
- Page with Ctrl+F/Ctrl+B using a stable render-derived page size and one row of overlap/context where appropriate.
- Show exact upper/lower continuation counts using the shared continuation style.
- Reset preview offset when the cursor selects another note, the note disappears/reloads, or the preview geometry changes.
- Preserve current list scroll behavior, collection-error header, responsive header/footer measurement, narrow layout, and minimum-height cursor visibility.

### Tests

- Test the exact 71/72 boundary, wider sizes, and repeated wide-narrow-wide resizing.
- Verify the 55/45 geometry and that the list uses its pane width for owner/title/body-match indicators.
- Cover Markdown headings, lists, emphasis, wrapping, tables/long tokens, empty bodies, parse errors, owner labels, and modification time.
- Cover Ctrl+F/Ctrl+B paging, upper/lower counts, tail clamping, cursor-change reset, resize reset, and short bodies.
- Exercise heights where header, search line, collection errors, footer, and status leave only one or two content rows; the selected list row and footer must remain visible.

### Verification

Run `go test -count=1 ./internal/tui`.

### Done when

- Notes automatically uses the shared wide layout at 72 columns and remains list-only below it.
- The selected note has a correct, bounded Markdown preview with explicit empty/error states.
- Paging and continuation counts work without clipping the list or footer.

## Session 5: Notes body-match anchoring and interaction hardening

### Goal

Make the Session 4 preview search-aware, recompute automatic anchors only when their inputs change, retain manual paging while stable, and expose paging controls only when the pane exists.

### Files and current seams

- `internal/tui/notes_search.go`: `noteSearchMatch`, title/body scoring, live draft, accepted query, cancel, and clear transitions.
- `internal/tui/notes_view.go`: search-match map, rederive/selection restoration, scope and refresh lifecycle.
- Session 4's Notes preview module and `internal/tui/notes_view_render.go`: rendered lines, cache key, automatic anchor, and page window.
- `internal/tui/notes_view_update.go`: search-edit key precedence and browse paging.
- `internal/tui/notes_guidance.go` and `internal/tui/notes_keymap.go`: footer/help gating.
- `internal/tui/notes_search_test.go`, `internal/tui/projects_notes_guidance_test.go`, and the focused preview tests.

### Implementation

- For a body match, find the earliest matching region in the Markdown output rendered at the current preview width, using the same effective smart-case mode as the visible result set. Start one rendered line above it when possible.
- Extend transient match/preview data only as needed to locate that region; do not persist search positions or note metadata.
- Define the automatic-anchor/cache key from:
  - selected `NoteRowIdentity`;
  - active draft or accepted query;
  - note `RawRevision`, with body content as the in-memory fallback when a revision is absent;
  - Notes scope;
  - resolved preview width.
- Reset/re-render/re-anchor whenever any key component changes, including reloads and body mutations that retain the same filename.
- Once the automatic anchor is established, manual Ctrl+F/Ctrl+B paging must remain stable across ordinary re-renders while the key is unchanged.
- A title-only match starts at the top; an empty query starts at the top; an unlocatable fuzzy body region falls back deterministically to the top rather than retaining a stale offset.
- Handle Ctrl+F/Ctrl+B before the search text input while Notes search is being edited, without accepting/canceling the query or inserting control characters.
- Advertise Ctrl+F/Ctrl+B in contextual guidance and `?` help only when the shared geometry says a preview pane exists and a current note can be previewed. Keep existing search apply/cancel and Esc guidance required.
- Preserve search draft versus accepted-query semantics, smart-case fallback label, scope changes, refresh selection, and the existing Esc cascade.

### Tests

- Cover body matches at the top, middle, and bottom of rendered Markdown with one preceding context line.
- Cover multi-term and fuzzy body matches, smart-case narrowing, relaxed fallback, title-only matches, and no query.
- Cover every cache-key axis: note identity, draft/accepted query, raw/body revision, scope, and preview width.
- Prove manual paging remains stable for an unchanged key and resets/re-anchors for a changed key.
- Cover Ctrl+F/Ctrl+B during search editing, accepted search, ordinary browse, narrow layout, no current row, parse-error row, and minimum height.
- Verify footer/help entries appear only when the responsive pane exists and do not displace required search/Esc controls.

### Verification

Run `go test -count=1 ./internal/tui`.

### Done when

- A body-search result opens around its first rendered match with context.
- Automatic anchoring never becomes stale and manual paging never resets spuriously.
- Paging works during search editing and is advertised only when actionable.

## Session 6: Project metadata search and end-to-end assurance

### Goal

Expand Select Project across approved project metadata only, add committed cross-surface scenarios, and close the work through the repository's full CI gate.

### Files and current seams

- `internal/tui/project_hub.go`: `projectHubItems` constructs active, archived, and pinned `ListItem` aliases.
- `internal/tui/project_hub_test.go`: lifecycle populations, pinned mode, fuzzy search, task filtering, selection, and current Enter/Esc behavior.
- `internal/tui/uitest_fixtures_helpers_test.go`: rich, project-mix, and Notes scenario data.
- `internal/tui/uitest_example_scenarios_test.go`: required final-product scenario inventory.
- `docs/tui-testing/scenarios/`: committed scenario definitions.
- `docs/tui-testing/scenarios/outliner-rich-search-autocomplete.json`, `docs/tui-testing/scenarios/project-hub-metadata-search.json`, and `docs/tui-testing/scenarios/notes-responsive-preview.json`: new committed acceptance scenarios.
- `scripts/run_tui_scenarios.sh`: standalone committed-scenario gate.
- `scripts/ci.sh`: authoritative repository CI gate, including the committed TUI scenarios.

### Implementation

- Add project descriptions and project tags to each Project Hub item's fuzzy aliases.
- Add both plain and `#`-prefixed aliases for every normalized project tag.
- Preserve project name, slug, lifecycle aliases (`active project`/`archived project`), source population, fuzzy ranking, search navigation, Enter selection, task-count filters, and Esc behavior.
- Apply the same metadata rules in active, archived, and pinned modes.
- Never add task titles, task descriptions, task tags, task comments, or task content to Project Hub aliases.
- Add focused tests that search by description, plain tag, and prefixed tag in every applicable lifecycle/mode, plus negative tests proving task-only content never matches.
- Enrich scenario fixtures only with transient test data needed to demonstrate the approved behavior; do not change production persistence schemas.
- Add and register committed scenarios for:
  - rich Outliner multi-field search, cue rendering, grammar autocomplete, Tab acceptance, result navigation, and two-step Esc;
  - Project Hub description/tag search across relevant populations while task-only text remains absent;
  - Notes preview at wide and narrow widths, Markdown/body-match anchoring, paging/continuations, and responsive help/layout behavior.
- Include `no_layout_issues` assertions at wide, boundary, narrow, and minimum-height sizes.
- Run focused scenarios while iterating, then use the full CI script as the final authoritative gate. Repair only defects within this plan's scope.

### Tests

- Verify Project Hub metadata search independently of the scenarios.
- Ensure scenario fixtures remain deterministic and the committed inventory names all new files.
- Replay all committed scenarios, not only the three new ones.
- Run race-enabled Go tests, coverage, formatting checks, and `golangci-lint` through the repository CI script.

### Verification

Run `./scripts/ci.sh`.

### Done when

- Select Project matches project descriptions and plain/prefixed project tags without matching task content.
- The three new scenarios pass and the existing scenario inventory remains green.
- The complete repository CI gate passes with no new view, route, persisted field, CLI, or migration behavior.

## Cross-session acceptance

- Verify every Outliner field tier, cross-field queries, smart case, fuzzy fallback, sigil-only searches, exact qualified IDs, terminal visibility, stable tie-breaking, and mutation-driven result refresh.
- Verify source cues, cursor styling, visual-width truncation, and that the cue drops before existing task indicators.
- Test autocomplete triggers, scoped dynamic values, singleton/repeatable behavior, cursor-aware insertion, bounded rendering, Tab acceptance, result navigation, and the two-step Esc cascade.
- Verify Project Hub searches descriptions and project tags in active, archived, and pinned modes while task content remains unmatched.
- Verify Notes wide/narrow boundaries, Markdown and error rendering, body-match anchoring, paging, resize/query/cursor resets, search-edit interaction, footer/help gating, and minimum-height behavior.
- Add TUI scenarios for rich task search/autocomplete, project metadata search, and wide/narrow Notes preview; then run the full CI and scenario suites.

Rollback is code-only: each session changes transient search/presentation behavior and tests, with no migrations or persisted-state changes. If a session cannot satisfy its focused tests without broadening scope, revert that session's code changes while retaining its tests and findings for replanning.
