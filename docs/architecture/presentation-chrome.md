[← Architecture index](../architecture.md)

### Presentation chrome

`WorkspacePulse` is app-owned workspace state with Inbox, Today, overdue, and
current-local-day Done counts. Full-screen presentations receive it through the
exhaustive `route.PresentationChromePolicy`; transient selectors and pickers remain
pulse-free and preserve the host's title row. There are no presentation
exemptions: every reachable presentation has shell, surface, or overlay guidance
ownership.

`RenderOperationalTitle` is the one-line title fitter. Identity on the left
contains surface, scope, and mode exactly once, and the workspace pulse is the
right region. Today is normally mandatory. When width is constrained, the fitter
removes Done, Inbox, overdue, and Today in that order.
`formatWorkspacePulseComponents` omits a component at zero and returns the empty
string when nothing is left, so the `workspace: ` prefix never stands alone; the
drop ladder's order and every counter are unchanged, since the ladder decides
what may be shown and the zero test decides what is worth showing.

Selection, lens, search, filters, sort, focus, and section zoom are no longer in
that line. `OutlinerModel.BuildViewContextEntries` describes them once as typed
`ViewContextEntry` values — label, value, a rendered `Row`, a `RowCompact`, and
whether anything peels them — and two consumers read that one description rather
than each other's rendered text: `pulse.RenderContextRow` draws the optional
second header row, and `queryReadoutDetailLines` lists the read-only half under
`Q`. The row tries every segment whole, then every segment compact, then drops
whole segments from the right, spending the reclaimed room on
`+N more · Q details`; only a single segment that still does not fit is
truncated, and that case still names the readout. Existing meanings survive
because the entries are built from the same predicates the title used:
`ActiveLensLabel` carries `(suspended)` and `(modified)`, the search entry
distinguishes ranked from narrowing, and `effectiveSort` returns no sort under a
fixed-order mode.

The selection entry is the one of those that is not merely a restatement. It is
the sole persistent summary of a live selection — the status channel writes
nothing about one, so whatever the entry sheds is shed everywhere — and that is
why the hidden count is not what the compact spelling gives up: a count silently
covering rows off screen is the one number a bulk gesture must not be surprised
by, so both spellings carry it whenever it is nonzero and the compact one drops
the word `selected` the number already implies. Its `Value` spells the whole
breakdown on the same terms and on those terms only: `Q` is the one place the
visible half is written down, and with nothing hidden that half is the total
restated, which is not worth the wrapped readout rows it costs at a narrow
width. Being derived from the live rows at render time is what makes
a stale count unreachable rather than merely refreshed: a fold, a filter, or a
mode change moves members across the visible/hidden line without touching the
selection, and no seam has to notice.

`contextRowRows()` is presence-only — width chooses between full and compact
segments, never whether the row exists — which is what lets `ContentHeight`,
which takes no width, and `outlinerHostHeaderRows`, which anchors modals and the
hosted capture panel, charge the same row without a second arithmetic.
`writeOutlinerHeader` is the one writer, taken by the populated path and
`renderOutlinerEmptyState` alike, so the two cannot disagree about the header's
height.

Today mode adds a reference-class pace comparison to the middle region:
`9 open · median 3 done/day (30d)`. `ActivityCoordinator` refreshes one cached
`app.CompletionActivity` snapshot alongside operational chrome updates; the
shell injects that cache immediately before the Outliner paints, while the open
count comes from the separately injected `WorkspacePulse`. No history means no
comparison. The full pace candidate outranks the passive pulse; after the pulse
ladder is exhausted, pace degrades through the no-window and no-label forms,
then disappears before required interaction context is truncated.

`GuidanceHint` and `GuidanceSpec` are the persistent control grammar. Providers
emit complete key/label entries with priority and required status. Browse states
have a one-row budget; edit, add, confirm, autocomplete, and expanded-detail
states may use two. A pending chord alone asks for `chrome.MaxGuidanceRows`,
three, which is the clamp `guidanceSegmentLines` applies to every spec: a
chord's entries are the whole answer to the prefix just pressed, and three rows
are what hold the `w` chord's ten at `chrome.MinTerminalWidth`. The footer
already grows upward and `ShellFrameMetrics` measures it, so no view reserves
anything for the third row. The renderer reserves required entries first,
preserves provider order, and drops supplemental state before controls.
Selection order is `Priority` and never display order, so a provider
reprioritises without moving an entry: capture draws `Ctrl+V paste` where it
always stood and ranks it behind every other optional hint through
`chrome.CapturePasteHint`, and the composer ranks `Ctrl+P project` ahead of the
`Ctrl+F` and `Shift+Enter` entries drawn before it.
Operation descriptors, gated keymaps, and `ListSurface` state remain the
authorities for keys and applicability.

`GuidanceHint.Group` is a heading a provider draws over the entry it names, and
it is a field on that entry rather than an entry of its own because the packer
selects optional hints by priority and stops at the first that does not fit. A
standalone heading would therefore be reservable while every member under it was
shed, leaving a division the row cannot show. Riding the first member makes
heading and member pack, fit, and drop as one thing:
`guidanceSegment.text()` prepends it, `fitToWidth` charges its width to the same
head it charges the key to and preserves it through the cut, and
`renderGuidanceSegment` paints it in `theme.HelpStyle` ahead of the key. Only the
first member of a run carries one — the shell compares each hint's group against
the previous one — so the row reads `Plan: n next · t today · Status: i inbox`
rather than repeating the word ten times.

`OperationKeyBinding.HintKeep` is the one exception to a chord selecting in
declaration order, and `wd` alone declares it. `pendingChordGuidanceSpec` gives
such an entry the priority of its run's first member: behind the entry carrying
the heading, so Done is never drawn under `Plan:`, and ahead of every other
member of that run, so a footer too narrow for all eight states sheds Done
last. Its slot in the row does not move.

#### One active instruction set

A composed frame stacks presentations, and only one of them answers the next key
press. `route.ActiveGuidanceOwner` names that layer, resolved in the order
`handleGlobalKeyPress` dispatches input: a shell-owned confirmation, then a
shell-mounted modal, then a pending chord, then the presentation itself.
`AppModel.activeGuidance` is the one resolution and `SuppressesShellGuidance` the
closed table it is read through, so an owner added to the vocabulary declares
whether it silences the footer instead of inheriting an answer.

The three shell confirmations are `projectops.Controller.ConfirmationArmed`,
`taskMutationController.confirmationArmed`, and `operations.Confirm`; each draws
its own answers inside its box. A confirmation the active view owns — the
composer's discard question — is deliberately not among them, because there the
surface is still the layer speaking and its `ContextualGuidance` has already
swapped the draft's controls for the prompt's exits. `shellModalMounted` covers
what `handleActiveKeyOverlay` routes to: the check-in setup pass, the
action-sheet flow through its own `OverlayMounted`, the lens picker, and the
conflict browser.

Whether the footer is a presentation's voice at all is
`PresentationChromePolicy.HostGuidanceStandsBeneath`, and the question it asks is
not whether this presentation drew its own acts but whether anyone else is still
under it. A transient presentation is composited over a host that is still drawn
and still filling that footer, so the ranks there describe a grammar no press
reaches. A fullscreen presentation replaced that host: even where it draws its
acts inside its content, as the Comments panel, the project editor, Notes, and
the Hub all do, the footer beneath it holds that same surface's breadcrumb and
its `Esc back · ? help` — the second half of one instruction set and the only
statement of either on the screen. That is why the predicate reads `kind` rather
than `guidance`, and why it is the exact complement of
`WorkspacePulseEligible`: one classification, asked twice.

`route.SelfGuidedPresentation` is the per-frame override for the case a
`PresentationID` cannot answer, where one view owns two presentations: the task
composer's compact panel publishes through the shell, and its expanded form
draws its own footer over the whole content area and answers `true`. An
unclassified presentation publishes, so a footer that may be the only
instructions on screen is never emptied on a guess.

Suppression is `suppressGuidanceRows`: the occupant is replaced by blank rows of
its own height, each filled to the footer's own width. The frame is laid out
against that height, so returning fewer rows would move everything above the
footer when an overlay opened and move it back when it closed. Those rows are
blank but present, which is the distinction the composition turns on: the empty
string is how this footer spells a block that is not there at all, so a one-row
occupant blanked to `""` would be joined and measured as nothing and the footer
would shrink under a message it is still carrying. An occupant that was already
absent stays absent, since there is no row to hold. It also takes the occupant
whole rather than keeping part — while a modal owns every key, the breadcrumb's
`← <prev>` preview names a destination that press would not reach. A view-local modal, such as the
Outliner's menu bar, relation picker, or query readout, needs none of this: it is
answered inside its surface's own `ContextualGuidance` ordering, and the shell
keeps publishing whatever that surface says.

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

`renderFooterChrome` is two blocks under one divider, not one line with two
possible tenants. `footerFeedbackLine` renders the success, info, or error the
last mutation left, `footerControlContent` renders the pending chord's rows or
the surface's ranks with `suppressGuidanceRows` already applied, and
`footerChromeContent` joins whichever of the two are non-empty in that order.
Feedback is therefore the row directly under the divider and the controls keep
the terminal's last row: a message arriving grows the footer upward into the
content area rather than moving the keys the user is reading. Both halves are
pure functions of the model, which is what lets them be read twice in one frame.

`chrome.ShellFrameMetrics` is that reading. `FooterRows` is the divider plus the
two blocks, floored at the `ShellFooterBaselineRows` every view has always been
sized against by `viewNavigationController.viewSize`, and `ExcessRows` is the
difference — what a view owes back when the footer says more than one thing.
`AppModel.shellFrameMetrics` builds it from the same two functions the render
calls, so a measurement that disagreed with the paint would have to be a
different function. Nothing in that path consumes anything: the focus notice is
the strip's one-shot and the strip is view content, deliberately outside this
measurement.

`route.ShellFrameAware` is the delivery, and it reaches exactly one view.
`syncShellFrameMetrics` runs once per `Update`, from the same deferred closure as
`syncHostedPanelHost` and for the same reason — it is the one place the footer
this update produced exists before the frame is drawn. It records the
`presentation.PresentationID` it told, skips an unchanged reading, and hands the
zero value back to the previous recipient when the active presentation changes.
That single-recipient rule is the whole double-reservation defense: with the
capture panel open over the outline, both views are drawn and both could
plausibly subtract the same rows, and the panel is the one that does.
`OutlinerModel.ShellFrame` feeds `outlinerListRows`,
`TaskComposerModel.shellFrame` feeds `composerPanelShareRows` and
`compactPanelRows`, and `OutlineErrorsViewModel.shellFrame` feeds its scroll
`capacity`; each charges `ExcessRows` once. The panel charges it to the share it
asks the host for and not to the rows it draws, which is the only place a
content-sized panel can put it: taking the row out of its own height would drop
the input, preview, or refusal row the content asked for, so the footer's extra
row comes out of the host's budget through `SetHostedBudget` — a loan the
outline's anchor keeps its window across — and the panel draws exactly what it
measured.
The refusal surface fills the rows it was given exactly, so it is the case a
footer carrying both a message and the blank rows it holds for the host would
otherwise cost its last row — the row naming `Esc close`.

`route.HostedBudgetView` is the second seam built on that same reason, and the
two are worth reading together: `ShellFrameAware` carries a budget beside the
size so a growing footer never re-anchors a list, and `HostedBudgetView` carries
the panel's own budget the same way — `SetHostedBudget` for every height a panel
leaves its host, `ReleaseHostedBudget` once at the close, plain `SetSize` for a
host implementing neither. Both exist because `SetSize` is the resize a terminal
causes and a list surface answers it by dropping its window, so anything
temporary that travels through it is spending a window it did not mean to.
`docs/architecture/capture-and-outline-editor.md` has the anchor the outline
keeps across that loan.

A full-height view skips `renderFooterChrome` and draws its own footer, so the
shell composes into that frame instead of replacing it.
`composeSurfaceFooterFeedback` finds the surface's own divider with
`surfaceFooterDividerIndex` — the last all-`─` row above the final line — and
spends the body rows immediately above it, which keeps the frame exactly as tall
as the terminal and leaves every published control row intact. A frame with no
divider, or one whose divider leaves no body row to spend, keeps its footer whole
and drops the feedback; Zen mode drops it by classification rather than by
geometry. The function it replaced, `replaceFooter`, did the opposite: it
overwrote the surface's own guidance with the message, which is why a Notes or
Help frame could be left with a success line and no statement of `Esc back`.

The outstanding-conflict indicator leads the strip because it is the one durable
line there: it stands until the last bundle is resolved, while the two
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
struct-literal test models indicator-free. Resolution is available through the conflict overlay and CLI.

Sync conflicts are the count's other half and are refreshed apart from the
bundles, because reading them is not a directory listing: the sync status port
derives the open conflicts from the whole history. `refreshOutstandingConflicts`
reads both halves, and runs at launch, when a reload adopts a change, when `gc`
opens or resolves, and after an exchange pass that could have moved the status:
one that imported or observed a change, published anything (a resolution among
what it may have published), was asked for by hand, or whose
`automaticSyncOutcome` differs from the pass before it. That outcome spells
everything the `sync:` line reads apart from its age, so a pass that starts or
stops failing, or changes what is waiting either way, is read once and its
repetitions are not. Every other pass only moves the cached status's
last-success time and redraws the line through `renderSyncAdvisory`. In a
session exchanging on its own timer the 60s tick takes
`recountOutstandingConflicts`, which recounts the bundles against the sync
conflicts last read and never calls the port; a session with no exchange timer
still reads both halves on that tick. What another process imported is noticed
through `workspace.Fingerprint`, which covers the sync tracking state: the
change owes a reload, and adopting it reads both halves.

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

`ConflictBrowserModel` is that count's browser and resolver, and it is an overlay
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
gone. The overlay registers no operations and has no `Space` sheet. It sends a
frozen `conflict.ResolveMsg` to the shell after confirmation. The shell resolves
through `engine.Runtime.ResolveSyncConflict` or `ResolveProjectConflict`, sharing
the runtime's transaction service, sync history, and undo ledger. Success reloads
the listing and workspace views; validation and stale-state errors retain the
editor draft. Diffs use stored text, including documents from older schemas.
The merge editor retains raw text without tab expansion or line limits.

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
a display budget: the first nine axes carry digits wherever the window sits, a
tenth is drawn without one and reached by scrolling, and
`queryReadoutPeelAllLimit` bounds the peel-all loop against a future axis that
fails to clear itself.

The readout is a bounded viewport because the context row now forwards to it.
`queryReadoutWindow` is its one measurement — the wrapped document, how many
lines the window draws, and whether anything is off it — and the render, every
scroll write, `queryReadoutPageRows`, and `queryReadoutOverflows` all read it, so
none of them can disagree about where the window ends. `queryReadoutBodyRows`
mirrors `outlinerListRows`' own arithmetic (`ExcessRows`, the
`peekDockFooterWrapRows` floor under an open dock, the dock, the strip's signal
rows, then `queryReadoutChromeRows`) rather than calling `StatusStripHeight`,
which would be asking for the height this function is still deciding;
`queryReadoutMinBodyRows` is the floor and the reserved outline row is inside
that chrome constant, so the readout can never take the list's last row.
`clampQueryReadoutOffset` is pure and applied at render as well as at every
write, which is how a peel that shortened the document or a resize that shortened
the window re-seats itself without either having to be noticed;
`PeelQueryReadoutRow` still calls `scrollQueryReadout(0)` so the stored offset the
next key adds to is already in range, through `reseatQueryReadoutAfterPeel` and
always after the status that peel publishes. That ordering is the whole of what
the helper is for: the readout's window is what the strip leaves, the strip is a
row taller while it holds a message, and a re-seat taken against a momentarily
empty channel would clamp the offset against a window larger than the one the
frame goes on to draw. Since the render-time clamp can only reduce an offset
further, a position lost that way is lost for good, and the reader's window jumps
to the top of a document that still overflows. That render-time clamp does not write back,
on purpose, so a resize that *grew* the window leaves the stored offset past the
new maximum while the screen already shows the clamped line;
`scrollQueryReadout` therefore clamps before it adds its delta rather than after,
which is what makes `j`, `k`, `PgUp`, and `PgDn` travel their full distance from
the line the reader can see. Adding to the stale number instead would spend the
first upward key normalizing it, leaving the readout where it was, and would
shorten a page turn by however far the offset overran.
`scrollQueryReadoutTo` is absolute and needs no such reading, since `Home` and
`End` name a bound rather than a distance. `scrollQueryReadout` and
`scrollQueryReadoutTo` write `QueryReadout.Offset` and nothing else, which is
what lets the six scrolling keys borrow `j`/`k`, `PgUp`/`PgDn`, and `Home`/`End`
while the cursor, the list's own offset, and the selection stay put. They are
consumed whether or not the document moves, since the readout owns input
outright, but the footer advertises them only over a document that can move.

Peek's dock is the frame's other reservation, and while its height now follows
the frame it is still fixed in the sense that matters: `peekDockGeometry`
measures neither the cursor nor the document, so `ComputeOutlinerViewport`
subtracts the same rows under a one-line task and under a forty-line one, and one
resolved geometry is what the render, the clamp, the two page distances, and the
hints all read. `peekDockPreferredRows` is a third of the Outliner's own height
clamped to seven and twelve rows, `peekDockAvailableRows` is what
`ContentHeight` leaves after the footer excess, the strip, and
`peekDockListFloorRows`, the smaller wins, and a total under `peekDockMinRows`
puts the dock down rather than clipping it. It subtracts a guard row as well, against a footer
wrapping at minimum width, and the measured frame budget below is what turned
that guard from a guess into a floor rather than removing it: the dock advertises
its own paging keys only over a document that can move, so the footer wraps under
one cursor and not under the next, and a reservation following the measurement
raw would hand the dock's position back to the cursor. `peekDockFooterWrapRows`
is that floor — one row, while the dock is open — and a footer taller than it is
still charged in full.
The pinned context row is the one thing inside that window that is not a row.
`indexSectionHeaderRows` records, from `finalizeModeRows`, where each lane's
header landed, keyed by the section half of `OutlinerRowGroup`; it runs there
rather than where a header is appended because the Pinned separator, the
checklist expansion, and the focused project header all move rows afterwards.
`sectionHeaderAbove` reads that index and the first window row's own group, so
"is this lane's header above the window" is answered without scanning for one,
and it distrusts an index the rows no longer bear out. `pinnedContextWindow`,
called by `ComputeOutlinerViewport` alone, then decides the window: it tries the
full height, and where that window opens inside a lane it windows one row fewer
through the same `PersistentIndexWindow` call and reports `pinned`. Both heights
are tried against the stored offset, never one against the other's result, so an
idle frame writes back the offset it found and `k` from the last row does not
scroll. `VisibleRows` is unchanged — the screen rows the list occupies — so
`outlinerListRows`, the dock, the strip, and the scrollbar measure what they
measured before, while `startIdx`/`endIdx` on `LastViewport` are the smaller
window; `VisibleRowWindow`, `CursorScrollOffset`, and the hosted-panel anchor
therefore place the cursor among the rows actually drawn, and `zt` seats it on
the first row under the pinned one. Two windows give the row back: one the
smaller height opens exactly on a header starts on that header at full height,
and at the end of the list, where that would leave the last screen row blank,
the full window stands unpinned. `renderPinnedContextRow` builds the line from
`renderSectionHeader` with the header row's name and count, the project slug
from the first window row's `Group.ProjectID`, a muted accent, and the trailing
rule that still sets it apart in a profile carrying no color; it is padded or
cut to exactly `rowWidth`. Either single-project form, the ranked list, and any
first row carrying no section pin nothing.

Opening Peek moves the list once and nothing the cursor does afterwards moves it
again. `writePeekDock` pads the list block out to `VisibleRows` before writing
the dock, so a projection shorter than its window leaves the dock at the screen
row the reservation already paid for rather than pulling it up the frame.
`outlinerListRows` is that reservation, and it is one definition because both
render paths measure against it: `ComputeOutlinerViewport` spends it on rows and
`renderOutlinerEmptyState` spends it on the focused project header and the
explanation under it, padding the remainder the same way and then drawing the
dock whole. The empty projection is no exception — a dock it shortened or pulled
up would be a different height and a different position per projection, which is
the moving target docking retired. It differs in one way that changes nothing
about the geometry: it reads that reservation without computing a window, because
it has no rows to window and no scroll offset to write, which is why
`outlinerListRows` is pure and charges the status strip and the suggestion pane
the populated path charges. Below the supported floor the clamp leaves it a
single row and the shell's own clipping decides what fits; at every supported
size the divider, the content rows the geometry gave it, and the position row are
all on screen with the header and the explanation above them.

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

The dropdown is composited over the view it was opened from, not written into the
content stream in place of it. `RenderOutlinerTreeContent` renders the host first
— bar row, operational title, suggestion pane, the window `ComputeOutlinerViewport`
returns, the Peek dock — and hands it to `composeOutlinerModal`, the one seam both
of the Outliner's view-local modals take. A resolved dropdown goes through
`chrome.OverlayPopupDimmedAt`, anchored at `outlinerHostHeaderRows()` and at the
open label's `StartCol`, so the bar row and the title carrying the workspace pulse
are never covered and the box lands under the label it belongs to; the relation
picker goes through the bottom-anchored `chrome.OverlayPopupDimmed`, fitted to the
same `outlinerModalBudget`, so it keeps that header too. That centered path
alone tidies its sides: a margin under eight columns is painted with
`theme.OverlayHaloStyle` on the popup's rows, since a sliver of a row cut
mid-word reads as debris, while a wider margin and every anchored popup keep the
host. Both dim the host through
one `DimContent` call over the composed frame, so the compositing is one pass and
a modal never renders another modal's frame as its base.

The host is drawn whole and then covered, which is what stops every row on screen
from stepping down and back up once per press: the box occupies rows rather than
displacing them, and `MenuBarVisible()` is already true while a menu is open, so
`ContentHeight()` is the same number the close redraws against. `ViewScrollOffset`
and `LastViewport` therefore hold the window the list frame just drew — the one
the close redraws and the one a key re-emitted through the non-modal arm pages
against — and an open Peek dock, a multi-select summary, and a visual selection
are all still underneath, in place, for the same reason. The compositing is keyed
on the drawn block rather than on the `MenuBar` gate, since a dropdown that
resolved to nothing must leave the host alone rather than dim it for no box.
`outlinerModalBudget` is what the anchor leaves — the frame height less
`StatusStripHeight`, less the reserved header rows — and it is the box's own
budget, so a menu too tall for it scrolls its list inside the box instead of
losing interior rows to `FitOverlayPopupHeight`. It reuses `RenderOverlayBox` and `RenderCompactListSurface`
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
reversed. Inverse video is an attribute rather than a color, which is the point:
`colorprofile.ASCII` drops every hue the row asked for and forwards SGR 7, so the
bar is the one part of the mark a terminal without color still draws. `gutterBg`
is what the columns after the bar carry: `rowBg` where the row has one, and
`ColorFlashBg` during a flash, which floods the gutter and reads as more of the
same mark rather than a different one. `gutterPiece`/`gutterSpace` merge that
background into each glyph's own style, because a background applied over
already-styled text ends at that text's first reset. `trail` is the third
question, answered in `outlinerCursorRow`.

`rowBg` is the opt-in, and `overTaskRow()` is the only place it is taken.
A task row is which work you are on; a section header, a project header and a
checklist row are places the cursor can stand, so they keep the bar and the dim
trailing fill and take no band. A flash swaps the resting `ColorCursorRowBg` for
`ColorFlashDimBg`, so the band brightens with the gutter instead of standing
still under a pulse.

The bar, the band and the trail are derived in `ApplyTheme` from the theme's
cursor color in three directions — the bar away from the terminal's own paper so
one column is unmissable, the trail toward and past it so the reach never
competes with the mark, and the band a short step from the paper — and which of
`lightenHex`/`darkenHex` is which flips with the theme. So does the size of each
step, because the directions are not symmetric. Darkening preserves chroma, so
the light theme's bar is a distinct dark block at a moderate factor; lightening
washes chroma out, so the dark theme's bar goes to `0.85` — most of the way to
white — or it lands on a mid gray that the row's own badges and chips already
out-shout. The band's own factors are the quietest of the three, because it is
the one that has to sit under text without changing what the text says.

`outlinerCursorRow` truncates the content to the columns the gutter leaves, lays
`rowBg` under the truncated result with `textfit.PaintBackground`, then pads the
remainder with `cursorPaint.trail()`, so a marked row is exactly `rowWidth`
columns at every width — the contract `AttachScrollbarAtWidth` depends on — and
degrades to a truncated gutter when `rowWidth` is no wider than the gutter
itself. The band is laid after the cut so the arming sequences it inserts are
never what the width measurement counts. `PaintBackground` arms the background
once and re-arms it after every reset the content carries, rewriting no byte of
the row: SGR is cumulative, so each segment's own foreground, bold, strikethrough
and badge background still paint on top, and the band fills only the cells no
segment claimed. A run rendered one rune at a time — a search match, a
struck-through canceled title — therefore stops being one contiguous string on
the line while every rune of it keeps its own style whole.

The band is also background precedence, stated in `applyTaskRowLineStyle`: the
marked branch returns before `taskRowTint` and `ZebraLine` are consulted, so the
overdue tint and the zebra stripe are neither applied over nor under it. Nothing
about the deadline is lost to that — the row keeps its bolded due badge, its `!`
hero emphasis and its overdue text color, all of which are foregrounds the band
sits beneath. `taskRowTint` still returns the overdue row background instead of
applying it, and overdue is the only tint it answers with: a repainted row is the
loudest mark the list has, so it belongs to the top of the attention ladder
alone. `SelectedLine` stays the whole-line authority on every other list surface,
which is what keeps a picker, a Notes row and a Hub row saying "selected" rather
than "current".

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
