[← Architecture index](../architecture.md)

### Presentation chrome

`WorkspacePulse` is app-owned workspace state with Inbox, Today, overdue, and
current-local-day Done counts. Full-screen presentations receive it through the
exhaustive `route.PresentationChromePolicy`; transient selectors and pickers remain
pulse-free and preserve the host's title row. There are no presentation
exemptions: every reachable presentation has shell, surface, or overlay guidance
ownership.

`RenderOperationalTitle` is the one-line title fitter. Identity on the left
contains surface, scope, and mode exactly once; selection, search, filters,
focus, or editor state forms the interaction context; and the workspace pulse is
the right region. Today is normally mandatory. When width is constrained, the
fitter removes Done, Inbox, overdue, and Today in that order before truncating
required interaction context.

Today mode adds a reference-class pace comparison to the middle region:
`today 9 · median 3/day (30d)`. `ActivityCoordinator` refreshes one cached
`app.CompletionActivity` snapshot alongside operational chrome updates; the
shell injects that cache immediately before the Outliner paints, while the open
count comes from the separately injected `WorkspacePulse`. No history means no
comparison. The full pace candidate outranks the passive pulse; after the pulse
ladder is exhausted, pace degrades through the no-window and no-label forms,
then disappears before required interaction context is truncated.

`GuidanceHint` and `GuidanceSpec` are the persistent control grammar. Providers
emit complete key/label entries with priority and required status. Browse states
have a one-row budget; edit, add, confirm, autocomplete, expanded-detail, and
pending-chord states may use two. The renderer reserves required entries first,
preserves provider order, and drops supplemental state before controls.
Operation descriptors, gated keymaps, and `ListSurface` state remain the
authorities for keys and applicability.

Escape names an act rather than a state, and no profile spells that act itself.
On the Outliner the cascade is the authority: each `listsurface.EscStep` carries
its own `Action`, `listsurface.NextEscAction` walks that list the way
`RunEscCascade` walks it, and `OutlinerModel.EscAction`/`EscHint` are what every
profile, empty-state sentence, and footer entry reads. A step whose act depends
on what it stands over answers for itself — a query typed under a lens exits
through `SearchExitAction`, which says `restore lens` rather than `clear
search` — so one key is never advertised two ways.

On a surface holding an editable draft the same question is answered by
`chrome.DraftEscapeHint`: unsaved text raises the discard question and reads
`Esc discard`, an untouched draft leaves outright and reads `Esc cancel`. The
question itself is `chrome.DiscardGuidanceSpec` and not the destructive
confirmation chrome, because declining it keeps the draft and stays in the
editor. `chrome.RequiredExitGuidanceSpec` is the two-row editor contract whose
exit entry is the caller's own word; `RequiredActionGuidanceSpec` is that same
contract at its `Esc cancel` default. A panel whose modal sub-state narrows one
key spells both halves as two gated `keymap` rows rather than branching a label,
which is what keeps the footer's one source intact. Required entries are
reserved before any optional one and are held to `chrome.MinTerminalWidth`, so
the way out of a state survives the narrowest supported terminal whole.

`ViewStatusStripProvider` is the bottom status strip. `renderStatusBar` composes
the active view's strip on top of `renderFooterChrome` under a static dim `─`
divider, so the strip is the top of the status bar rather than the end of the
view. Growing the status bar is what pins it: `AppModel.View` sizes and pads
content against the bar's measured height, so the strip sits directly above the
footer with no blank padding, at any guidance height, and is never the line
`clipContentHeight` eats. The Outliner is the one provider. The
outstanding-conflict indicator, its focus notice, and its own status message are
the strip's three lines, `StatusStripHeight` reserves the divider plus their
wrapped height from `ComputeOutlinerViewport`'s row budget, and
`ViewStatusStrip` consumes the one-shot focus notice — safe because
`AppModel.View` renders the view before the status bar, so the reservation and
the strip see one frame's inputs.

The outstanding-conflict indicator leads the strip because it is the one durable
line there: it stands until the CLI resolves the last bundle, while the two
below it come and go, so a transient line arriving never moves it to another
row. It is not part of `WorkspacePulse` — `workspacePulseLevel` is a contiguous
ordinal ladder and `ComputeWorkspacePulse` takes no filesystem-derived input, so
the ladder has no rung for it. The shell owns the count:
`AppModel.refreshOutstandingConflicts` calls `ConflictStore.List` at launch and
on the existing 60s `status.PeriodicSaveMsg` tick, caches the length on the
feedback cluster's `outstandingConflicts`, and injects that cluster's
`outstandingConflictCount` accessor into every
Outliner through `wireOutlinerOperationExecutor`, so no render reads the
directory and no new timer is armed. The store is read directly rather than
through an `internal/app` port, following the TUI's existing direct
`storage.LoadProject` reads: counting bundles validates, mutates, and commits
nothing, so there is no application-layer policy for a port to hold. A nil
accessor is inert, which is what keeps the Hub's preview painter and
struct-literal test models indicator-free. Resolution stays in the CLI.

The pulse the title row does carry has one home, and that is a correction rather
than a restatement. `OutlinerModel` embeds `listsurface.BaseSearchableView`,
which embeds `pulse.WorkspacePulseState`, so the promoted `SetWorkspacePulse`
and `CurrentWorkspacePulse` already were the pair every other full-screen
presentation uses. The Outliner nevertheless declared its own `workspacePulse`
field and its own `SetWorkspacePulse` beside them, shadowing both: the shell's
injection wrote the shadow, `BuildOutlinerTitleLine` read the shadow, and the
promoted `CurrentWorkspacePulse()` read an embedded copy nothing ever wrote. It
was invisible only because no Outliner code path took the promoted route — the
shadow was one accidental caller away from reading a zero pulse, and the two
spellings could not be told apart at a call site. Removing the field and the
method makes the injected value and the read value the same storage by
construction, which is why the title still renders identically. The one
mechanical consequence is that `OperationalTitleSpec.Pulse` is a pointer and an
accessor has no address, so `BuildOutlinerTitleLine` names the value in a local
first; that local is the whole cost of not being able to shadow it again.

`internal/tui/outliner/advisories.go` is the opt-out that count answers to, and it is
deliberately a category rather than a switch on one line. An advisory is ambient
workspace state — on screen because of what the workspace holds, not because of a
key just pressed — and today the category has exactly two members: the conflict
indicator and the `N projects failed to load` notice both loads emit. Each routes
through `AdvisoryLine` (a strip row) or `AdvisoryStatus` (a status message), and
that is the whole rule: a later ambient row inherits `--no-advisories` and the
View menu's toggle by using those helpers, with nothing to edit in the gate
itself. `AdvisoryLine` returning the empty string is what makes the row's height
reservation follow for free, since `StatusStripHeight` measures the same body
`StatusStripBody` paints. Suppression is session state on the shell
(`AppModel.feedback.hideAdvisories`, injected as `advisoriesHiddenAccessor` by
`wireOutlinerOperationExecutor`) rather than a per-render read of the machine
file, because two inputs decide it: `MachineState.AdvisoriesHidden`, read once in
`NewAppModel`, and `RunOptions.HideAdvisories`, which `RunApp` turns into
`WithHiddenAdvisories()` — a flag forcing hidden for one run and writing nothing.
`operation.OperationCoreToggleAdvisories` writes both, mirroring `coreToggleMenuBar`, so
toggling back after a flagged launch persists "shown": the state a user can see
is the state that was saved. It is registered with `TargetCardinalityNone` and
bound to no key at all, which is what lets it be menu-only without becoming an
entity operation the one-path rule would refuse. Feedback from the user's own
operations is not an advisory, nor is the query readout, nor the refusal that
explains a population where every project failed to load — a silent empty screen
is worse than a line a flag cannot reach.

`ConflictBrowserModel` is that count's read-only companion, and it is an overlay
on the lens picker's template rather than a surface: a `ListSurfaceModel` inside
`RenderOverlayBox`, one `AppModel` field, one arm in `handleActiveKeyOverlay`,
one in `dimmedOverlayRenderers`, and one `ConflictBrowserClosedMsg` case in
`handleOperationOverlayMessage`. It declares no `PresentationID`, no
`InputStrategy`, and no manifest entry, so `surfaceOperationGates()` still lists
five strategies and `operation.ValidateOperationOnePath` — which iterates manifests — has
nothing new to check. `gc` opens it through the existing go-to chord on
`SimplifiedInputStrategy`, which is why root `c` stays retired. The host port is
`AppModel.openConflictBrowser`: it reads `ConflictStore.List` through the same
direct-store access the indicator uses, settles `feedback.outstandingConflicts` from that
read, and refuses with a status rather than mounting an empty box. Inspecting a
row calls `ConflictStore.Load` instead of reusing the listed record, so a bundle
the CLI resolved in another terminal refuses instead of painting a record that is
gone. The overlay registers no operations, has no `Space` sheet, and calls no
mutating store method: resolution stays in `tt conflict resolve`.

The query readout is the strip's fourth line and the expansion of the
operational title rather than a rival to it. Collapsed it renders nothing, since
the title already carries the lens, the scope, the query, and the chips;
`Q` sets the `QueryReadout` sub-state and `QueryReadoutBody` then lists one row per
independently peelable narrowing, rebuilt from live state every frame so nothing
is remembered or inferred. It is pure, which is what lets `StatusStripHeight`
measure exactly the string `StatusStripBody` paints, and it sits under the
conflict indicator, which keeps the lead. `QueryReadoutRows` reads the axes in
the order the title reads them — lens, scope, mode, query, then
`commonFilterReadoutRows` and `projectFilterReadoutRows` — and a facet row's
value is `query.FilterClauseParts` output, so a row, a chip, and a typed query spell
one narrowing with one set of characters. A mode declares no clauses, so its row
names the mode instead of spelling a gate the catalog never held.

`CommonFilterFacetCount`'s split is promoted from a count into the peel unit, so
the readout cannot invent a facet boundary the facet writes do not have.
`peelQueryReadoutAxis` is the single write and routes each axis through the call
its own key uses — `RestoreLensBaseline`, `ClearProjectScope`,
`selectOutlinerMode(ModeAll)`, `ClearSearch`, `setLensQuery("")`, or
`replaceFilterClauses(row.fields, nil)` — so a peel cannot reach a state the
grammar cannot. It reports whether the peel owes a population reload, which this
model cannot perform: `queryReadoutRowRequiresReload` asks
`models.QueryFieldRequiresReload` over the row's own fields, the peel refuses
under `ShowsRankedList()` in the same words the `F` chord refuses in, and
otherwise raises `QueryReadoutReloadRequestMsg` for the shell's
`projectFilterReloadCmd`. `SaveViewAsLensRequestMsg` is the same delegation for
`s`, because the name prompt and the lens transaction both live above the
Outliner. While the readout is open it is a modal sub-state with its own gated
keymap: `queryReadoutGuidance` supplies the footer, so the readout takes no
footer rank of its own, and an unlisted key closes it rather than acting behind a
footer that is not on screen. `queryReadoutRowLimit` is the digit set rather than
a display budget, so a deeper stack names its remainder instead of dropping it,
and `queryReadoutPeelAllLimit` bounds the peel-all loop against a future axis
that fails to clear itself.

Peek's dock is the frame's other fixed reservation, and it is fixed in the
stronger sense: `peekDockHeight` returns `PeekDockRows` or zero and measures
neither the cursor nor the document, so `ComputeOutlinerViewport` subtracts the
same rows — that constant plus one guard row against a footer wrapping to three
lines at minimum width — under a one-line task and under a forty-line one.
Opening Peek moves the list once and nothing the cursor does afterwards moves it
again. `writePeekDock` pads the list block out to `VisibleRows` before writing
the dock, so a projection shorter than its window leaves the dock at the screen
row the reservation already paid for rather than pulling it up the frame. The
empty projection is the one path that may draw the dock short, because it
computes no viewport and has no list below a shortened dock to shift: it clamps
the block to the rows `ContentHeight` still has after the header and the
explanation, which always survive.

The menu bar and its open menu are the same kind of thing one level apart. The
bar is machine-local chrome: `M` writes `MenuBarVisible`, which `MenuBarPinned`
reads back, `menuBarRows` charges
the list exactly one row, and `ContentHeight` is the only subtraction, so a bar
that resolved to no menus at all still owes its row rather than letting the
reservation and the paint disagree. `resolvedMenuBar` projects
`OutlinerMenuSpecs` against the live catalog every frame and caches nothing,
which is what lets a menu whose every entry fell away leave the bar entirely,
and `menuBarLayout` reports where each label landed rather than only what the
row reads, because a dropdown is padded to a column and `RenderTabBar` returns
a string alone.

The open menu is the `Q` readout's template applied a second time: the gate and
two indices named together as the model's `MenuBar` field, whose own
`menuBarState` owns every write to them, `closeMenuBar` in every projection-replacing
reset, a first branch in `HandleKeyMsg` delegating to `handleMenuBarKey`, a
claim in `SimplifiedInlineControlOwnsInput`, and `menuBarGuidance` as the first
case of `ContextualGuidance` — which reads its own close key back through
`InputShortcutForOperation` instead of spelling it, so a rebinding moves the
footer hint and the close arm together. It mints no `PresentationID`, opens no
overlay, claims no `InputStrategy`, and takes no manifest entry, so
`surfaceOperationGates()` still lists five strategies; its keymap is gated, so
it takes no footer rank. `tui.core.open-menu-bar` is a second operation beside
the toggle rather than a mode of it, because opening has to reveal a hidden bar
in the same press, which is why one key never means "show" once and "open" the
next time. The reveal is the sub-state's own: `MenuBarVisible` is
`MenuBarPinned() || MenuBar.Open`, so the handler calls `OpenMenuBar` and
commits nothing, and `closeMenuBar` returns the row without a second write. The
View menu's own checkbox reads `MenuBarPinned` rather than `MenuBarVisible`, so
a tick describes the flag `M` writes rather than the band the open menu is
hanging from.

It diverges from the readout in exactly one place, deliberately. The readout
closes on any unlisted key and swallows it; the menu closes and re-emits the key
as a message, so the root grammar runs it one update later. The bar is a map of
the grammar and every row spells its own accelerator, so a key read off a row
has to work when it is pressed — the menu is non-modal, and nothing below
`handleMenuBarKey` is unreachable while a menu is up. `Enter` is the one entry
that dispatches, and it does so by raising `MenuBarActivateMsg` for
`handleOperationOverlayMessage`, the same delegation `s` makes with
`SaveViewAsLensRequestMsg`: this model holds no executor, and the menu is
already closed by the time the operation resolves its target.

The bar mints one category the grammar did not have: the menu-only operation. A
descriptor with no binding in any `InputStrategy` was always legal, since nothing
forces a key, but until the bar existed nothing could reach one.
`internal/tui/menu_bar_test.go` bounds the category with its fourth parity
rule — a menu-only operation's
`TargetSource` must be `OperationTargetGlobal` or `OperationTargetScopedProjects`,
so it is target-free or view-scoped and never a task or a project. That rule is
independent of `operation.ValidateOperationOnePath`, which sees no menu spec and keeps
governing the entity action sheets alone; the bar is a projection of the registry
in the same class as `HelpItemsForInputStrategy` and `GuidanceHintsForInputStrategy`,
so it names directly-bound operations freely and adds a route rather than a second
path to arbitrate. `tui.core.clear-folds` inhabits the category: it clears
`ExpandedNodes` and the whole `CollapsedSections` map in one act, which is what
the three reversing toggles `s`, `p`, and `t` cannot spell between them, and it is
the one core operation that reports success as well as refusal — a menu row closes
its dropdown over the rows it changed, so silence would read as a swallowed press
rather than as a change already on screen.

The dropdown is drawn inline rather than composited. It is written into the
content stream immediately under the bar row, left-padded to the open label's
`StartCol` and clamped by `overlayPopup`'s own arithmetic so the right edge
stays on screen — and it is the entire frame. `RenderOutlinerTreeContent` branches
to `renderOutlinerMenuFrame` before it consults the row count, so an open menu
draws the bar row and the block alone: no operational title, no suggestion pane,
no rows, and no `ComputeOutlinerViewport` call at all. The list is not shrunk to
make room for a menu, which is what stops every row on screen from stepping down
and back up once per press; skipping the window authority is what leaves
`ViewScrollOffset` and `LastViewport` holding the window the last list frame drew,
which is both the window the close redraws and the one a key re-emitted through
the non-modal arm pages against. The branch is keyed on the drawn block rather
than on the `MenuBar` gate, since a dropdown that resolved to nothing must fall back
to an ordinary frame rather than blank the view. `menuBarDropdownRowBudget` then
spends what the hidden list freed: `ContentHeight` plus the undrawn title row,
less `StatusStripHeight`, because the shell pins that strip and a box claiming
its rows would be clipped rather than scrolled. It reuses `RenderOverlayBox` and `RenderCompactListSurface`
over a surface rebuilt each frame, sized to its longest entry rather than to a
fraction of the screen, since a box anchored under one label that claimed three
fifths of the terminal would read as a modal. The two stored indices are the
only state that can go stale, because everything else is re-resolved every
frame, so `resolveMenuBarItem` runs during the render as well as in the keymap
and writes its answer back — a highlight left on a separator, or past the end of
a menu that lost rows to the cursor moving, lands on the nearest runnable entry
before either the paint or `Enter` reads it.

The Outliner cursor row is drawn by `outlinerCursorRow`, not by `SelectedLine`.
Every Outliner row renderer splits itself at one seam: a leading structural
gutter and the content that follows it. For a task row the gutter is
`taskRowSegments.gutter()` — cursor prefix, multi-select box, tree connectors,
fold arrow — and the content is `taskRowSegments.content()`, the state badge
onward; a checklist row's gutter stops before its `[ ]` box, and a section or
project header's gutter is its fold indicator and the spaces around it.

`cursorPaint` decides once per row how it wears the cursor, and every renderer
asks it three questions. `bar` renders the row's first column, and on the cursor
row it discards that column's own style for `CursorBarStyle` — inverse video, the
loudest treatment one column can carry alone. Every column it can cover is
structural: a blank margin, or a section header's fold arrow, which stays legible
reversed. `gutterBg` is what the columns after the bar carry, and it is nil at
rest — the resting mark is one column, so the connectors and the box beside it
stay ordinary. A flash spends exactly that width: `gutterBg` becomes
`ColorFlashBg` and floods the gutter, which reads as more of the same mark rather
than a different one. `gutterPiece`/`gutterSpace` merge that background into each
glyph's own style, because a background applied over already-styled text ends at
that text's first reset. `trail` is the third question, answered in
`outlinerCursorRow`.

The bar and the trail are derived in `ApplyTheme` from the theme's cursor color
in opposite directions — the bar away from the terminal's own paper so one column
is unmissable, the trail toward and past it so the reach never competes with the
mark — and which of `lightenHex`/`darkenHex` is which flips with the theme. So
does the size of each step, because the two directions are not symmetric.
Darkening preserves chroma, so the light theme's bar is a distinct dark block at
a moderate factor; lightening washes chroma out, so the dark theme's bar goes to
`0.85` — most of the way to white — or it lands on a mid gray that the row's own
badges and chips already out-shout.

`outlinerCursorRow` truncates the content to the columns the gutter leaves, then
pads the remainder with `cursorPaint.trail()`, so a marked row is exactly
`rowWidth` columns at every width — the contract `AttachScrollbarAtWidth` depends
on — and degrades to a truncated gutter when `rowWidth` is no wider than the
gutter itself. The seam is a concatenation only: it joins the same pieces at the
same widths the row already had, so `titleWidth` remains the single title-budget
authority. `taskRowTint` returns the overdue row background instead of
applying it, which is what lets the cursor row wear that tint on its content
region while the bar and trailing fill say where the cursor is. Overdue is the
only tint it answers with: a repainted row is the loudest mark the list has, so
it belongs to the top of the attention ladder alone. `SelectedLine`
stays the whole-line authority on every other list surface.

Recovery feedback is transaction-correlated. `storage.CommitResult.TransactionID`
flows through application results and TUI messages, and `u undo` appears only
when it equals the newest global ledger entry. Relocations add `Ctrl+O origin`
only after jump history records an origin; `Ctrl+I` returns to the relocated
task.

A ledger restore decorates its own message on the same terms. When the session's
undo-focus memory correlates the restored transaction with one committed task,
the message gains `Ctrl+O origin` if the cursor actually moved to it, and
`now in <Project> (<Lane>)` when the current projection does not draw it. An
uncorrelated restore — a bulk gesture, a project commit, a transaction from a
previous session — names neither and moves nothing. See
[`tui-architecture.md`](tui-architecture.md).

All mode projects open work as Pinned, Today, Doing, Review,
Inbox, Next, Paused, Backlog, Waiting, and Someday. Pinned is a transient projection
the mode catalog admits through `ShowsPinned`, set on All and Today
only; Inbox, Next, Backlog, Due, and Recent own one narrow ordered answer that a
promoted group would split. It is
grouped by project: groups sort by their newest pin, tasks inside a group start
newest-first, and `J`/`K` reorder them manually within their group. A pinned
subtree ranks by its own root's pin, so a swap moves exactly the rows it
displaces. A pinned reorder swaps both tasks' `PinnedAt` values in one
transaction spanning at most two projects. `Ctrl+J`/`Ctrl+K` extend that to the
whole run between the cursor and its group's edge, redistributing only the
timestamps that run already carries so pins outside it keep their instants.
It retains the ancestor marker and an
identity suppression set, so promoted tasks appear only once.
Review similarly promotes open tasks carrying the stored `Review` flag or an
eligible check-in whose day has arrived, after Doing. Claim order and draw order
are different things: Pinned, Today, and Next take every candidate first;
scheduled-only attention also yields to Doing, while a manual flag retains the
existing Review claim over Doing. Old work is not promoted into a Stale lane;
the Stale filter and lens remain explicit cleanup tools.
The projection does not change lifecycle or planning. The flag is a boolean on
the task beside `Pinned`, written by the `=` sigil on all four grammar surfaces,
toggled on a row by that same character as a root key, and narrowed by the `f=`
facet. What it still has no key for is being entered as a destination: `$review`
is rejected — a destination is a singleton and a flagged task keeps its lifecycle
state — and a `wr` terminal would contradict the parser. `ApplyReview` stores no instant of its own, unlike `ApplyPinned`, whose
`PinnedAt` orders the pinned section; nothing reads when a task was flagged, so
a timestamp would be a field with no reader.

Stale is an explicit cleanup filter and lens, with no automatic lane. It
matches open, currently undeferred work whose `UpdatedAt` touch time is at least
90 local-calendar days old, measured inclusively by local calendar day. The lowercase `fs`
facet applies that same predicate as a row refinement except in Recent, where it
is unavailable. Entering Recent with it active clears only Stale and reports
`Stale filter cleared: Recent only includes the last 30 days`. The `fs` facet
never reloads the project population.
