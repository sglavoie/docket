[← Architecture index](../architecture.md)

### Named lenses

A named lens is one durable Outliner projection on exactly six axes — identity,
name, mode, project scope, one narrowing query, and the sort axis — persisted as
`config.NamedLens` in the portable `lenses.json` domain, while the theme, the
menu bar's visibility, and the active-lens pointer stay machine-local. The
menu-bar flag carries `omitempty` and `NewMachineState` never seeds it, so the
omitted key is the default and a machine that has never shown the bar writes
nothing about it. The query is the whole narrowing:
`models.JoinQueryString` writes the canonical clause string the facets render to
and the richer spelling the user-facing grammar owns as one value, and
`models.PartitionQueryString` splits it back, so a facet chosen from a picker and
the same narrowing typed into slash search save as the same bytes. Schema 16
spread that predicate across a free-text query and two filter carriers, one
struct field per facet, which made the same narrowing expressible two ways;
schema 17 collapsed them, `MaxNamedLensQueryRunes` bounds the one remaining
string on load, and `internal/config/v16_named_lens.go` is the only code that
still knows the carriers. Scope is the one axis
a lens may decline to own: an empty `NamedLens.Scope` means the definition leaves
the view's scope exactly as it found it, which is deliberately not a way to force
the global population. `capturedLensScope` decides what a save records — never
the single-project Outliner, which is a scope no lens installs, and never a
parked focus, which is an exception to the base workspace rather than a scope the
base Outliner can be asked for. `captureLensState` reads the query axis through
`capturedLensQuery`, which is `m.lensQuery` under a lens and the accepted
`SearchQuery()` when an ordinary search query is narrowing the visible rows and
its input is closed. Only that axis crosses over: the search carrier itself
cannot survive underneath a lens, so `publishLens` calls
`clearSearchProjectionWithoutRestore` and the baseline hands the search back on
the way out. The definitions travel with the workspace but no lens
is a workspace entity, which decides the rest of the design: a lens mutation
goes through `CommitLensDefinitions` and so
opens no workspace transaction, its binding carries `DelegateTransaction`, and
because no transaction is recorded `OperationHistory` drops the entry, so `u`
never takes a lens back. `commitNamedLensMutation` clones the machine snapshot,
mutates the clone, validates it, and only then commits, so an invalid list is
never the one in hand.

Ordering is the other half of the contract: a TUI effect is published only after
the commit returns nil. The active-lens pointer is its one documented exception,
below. `ApplyNamedLens` decides every refusal — a malformed
definition, an unknown mode, a query the grammar recognizes and cannot honor, a
scope the base workspace does not admit — before it writes a single axis, so a
refusal is indistinguishable from never having asked. `lensScopeRefusal` tests
membership in `TaskWorkspaceProjects()` alone, which is already non-archived and
unparked, so one test answers all three cases; `GetProject` is read only to word
the message as archived, parked, or no longer existing, never to widen
admission. Parked therefore fails closed, with `parkedFocusProjectID` and
`settleParkedFocusException` the named seam a later revision would reopen it
through. When the lens changes which projects load, `beginLensProjectLoad`
carries the candidate filter with the load instead of writing it to the model and
bumps a generation counter; `handleLensProjectsLoaded` publishes only when its
own generation comes back, so a superseded load installs nothing. That start is
also where the restoration point is frozen: the pending application carries the
baseline captured before the read, so navigating, searching, or filtering while
the projects load cannot rewrite the view one `Esc` later returns to. That
baseline is taken in `ApplyNamedLens` before the scope push, not after it, so
one `Esc` hands back the scope the user was in rather than the one the lens
entered. Anything
that supersedes the application settles it rather than leaving it armed — a
synchronous lens, an ordinary reload, a cancelled refresh, and a restoration all
clear `pendingLens`, the loading flag, and the spinner, so a late result finds
nothing to publish into and leaves no half-armed chrome behind. Under a
single-project scope no load is required at all: the scope already fixed the
population, and the lens's project facets stay recorded but inert until it is
left. A lens that owns a scope therefore installs it *before* the load decision,
through `pushProjectScope` rather than by assigning the field, which is what
keeps the whole inert-facet mechanism untouched: `lensProjectLoadRequired` is
false under a scope, so a scoped lens reads no population at all and the other
order would start a load the scope immediately invalidates. Project facets are
population
selectors rather than row predicates, so `activeLensFrame` remembers that a
publication left them owed a load, and the explicit clear-scope route pays it:
`beginLensScopeExit` arms an application that carries no definition, and
`finishLensScopeExit` installs the population and drops the scope together. The
scope stands while that read runs, so replacing one project scope with another
keeps the facets inert and a wholly failed read leaves scope, population, and
lens exactly as they were. The frame itself is never rewritten by that exit: the
identity, the saved fingerprint, and the pre-lens restoration point all belong to
the application that installed the lens.

`activeLensFrame` is the one live lens; there is never a stack. It holds the
installed identity, the fingerprint the modified marker compares the live view
against, and the single restoration point `Esc` returns to. The fingerprint is
the whole of what a comparison needs, so the frame keeps no second copy of the
record: the definition itself lives in the lens domain. That fingerprint is taken
from the definition with its scope axis replaced by the scope the view is
actually in, because an unowned axis cannot be diverged from — which is also
what makes a later scope push or pop a real divergence. `lensModified` carves
scope out of "navigating" for exactly that reason: moving the cursor, folding,
selecting, and opening Peek all leave an unmodified lens unmodified, while
entering or leaving a project scope marks it, and `Replace from current view` is
what adopts the scope the user moved to. `lensRestorationPoint`
reuses the original capture whenever a lens is already live, so switching lenses
replaces the definition while keeping the baseline: one `Esc` lands on the view
the user actually left rather than on an intermediate lens. The Esc cascade
unwinds the lens after search and before the focus stack, and
`classifyEmptyCause` orders the empty state the same way — a lens owns the mode,
the whole narrowing, the sort, and the scope at once, so naming any
one of them would send the user to a key that cannot widen the list.

Under an active lens `/` calls `beginLensQueryEdit` and the shared search input
holds the lens's narrowing query instead of a scope-wide search: `Enter` commits
it, `Esc` restores `lensQueryEditOrigin`, and `buildOutlinerSearchIndicator`
reads the lens compilation rather than the search one for its badges, because
those are the rows on screen. `renderTaskTitle` reads the same compilation for
the same reason: `titleHighlightQuery` returns the search remainder and its case
mode when a search owns the projection and the lens remainder and
`lensCaseSensitive()` when a lens does, so a marked title always names the query
the visible rows were actually narrowed by.

`\` reaches the picker through the ordinary core operation
`tui.core.named-lenses`, bound directly in `SimplifiedInputStrategy` with no
`FooterRank`: `?` teaches it and the compact footer stays as quiet as it is
today. It is admitted from browse and from accepted ordinary-search results, and
it never fires while a text control owns input — `handleGlobalKeyPress` routes
the text prompt, value picker, action sheet, and lens picker first, and
`handleSimplifiedInputKey` declines under `simplifiedInlineControlOwnsInput`, so
`\` stays a literal character everywhere text is being typed. Help's own `\` is
surface-local and unaffected. `resetToGlobalAll` calls
`clearActiveLensWithoutRestore`, so no lens frame survives the launch reset and
none survives an `Esc` to the root either.

A launch nonetheless reopens the lens the pointer named, and the order is what
makes that safe. `NewAppModel` reads `Machine().ActiveNamedLens()` into
`pendingLaunchLens` before any message is dispatched, precisely because the
launch reset it is about to run clears that pointer; holding the value is what
lets the reset stay ignorant of a case it should not know. The lens is drained in
the `crossProjectsLoadedMsg` arm of `handleGlobalLifecycleMessage`, after the
population has been adopted, because a lens names a project scope and project
facets that cannot be validated against a population that has not landed yet; a
superseded generation adopts nothing and so drains nothing. `restorePendingLaunchLens`
takes the value exactly once and forgets it either way, then applies it through
the ordinary `ApplyNamedLens`, so every refusal already documented above decides
a launch too. A refusal is silent: the pointer is a hint the user last expressed
on this machine, not a record they are owed an error about. Because the reset
completed first, `lensRestorationPoint` captures the canonical Active Projects /
All view, and the first `Esc` lands exactly where a lens-free launch would have
started — with no new baseline machinery, since `outlinerViewSnapshot` already
carries `scopedProjectID` and `scopeStack`.

`MachineState.ActiveLensID` is the machine-local pointer at whichever lens is
live. `publishLens` writes it and `restoreLensBaseline` and
`clearActiveLensWithoutRestore` clear it, all three through the nil-safe
`persistActiveLens` seam the shell injects in `wireOutlinerOperationExecutor`;
`commitActiveLensPointer` is the `AppModel` side, built on
`commitNamedLensMutation` so it inherits the same clone→validate→commit path and
the `DataDir() == ""` in-memory fallback. Writing it from `publishLens` rather
than from `ApplyNamedLens` is what keeps it honest: an asynchronous application
superseded by a later generation publishes nothing, and persisting at the ask
would have named a lens the user never saw. It is also the one lens write not
ordered ahead of its TUI effect, because it records a resume hint rather than an
act — the projection is already installed, so a failed commit degrades to a
status warning instead of aborting an application that succeeded. An unchanged
pointer is skipped, so an ordinary `Esc` or global reset rewrites no bytes.
`ActiveNamedLens()` validates the record it finds for shape alone and never
referentially, so a pointer naming a lens another machine deleted loads without
error and is refused later, at the one place every other refusal is decided.

#### The built-in view catalog

`config.BuiltinNamedLensTemplates()` is the frozen nine-template catalog and
`config.StarterNamedLenses()` its first three; both are built from
`builtinNamedLensDefinitions()` on every call, so detachment is by construction
rather than by a `Clone` pass and an append can never reach a template the caller
was not handed. Only the starters are seeded: `NewMachineState()` calls
`StarterNamedLenses()`, and widening the catalog therefore widens what can be
added rather than what a fresh machine already holds. A template's identity is
its frozen UUID under the `6f1a0c4e-…` prefix, never its display order or its
mutable name, and the catalog is returned in declaration order — the gallery, not
`config`, sorts.

Adding one is the ordinary operation `tui.lens.add-template`: Filter category,
`OperationTargetGlobal` at `TargetCardinalityNone`, `CapabilityLensPicker`, and
`ValueProviderLensTemplate` with no `Applicability`. It resolves no target, so it
binds through `zeroTargetLensHandler` rather than through the single-target
branch, and an unmapped zero-target descriptor fails registration loudly instead
of silently asking for a cursor task. `resolveBuiltinLensTemplate` is the one
value reader: it requires a `string`, matches it against the catalog by exact ID,
and rejects a `config.NamedLens` payload as malformed, so the definition that is
committed is always the catalog's own rather than whatever a stale row carried.

`ValueProviderLensTemplate` is deliberately absent from `textEntry()`, which is
what routes it to a picker; its case in `newOperationValuePicker` returns before
`operationTaskRefs`, so the gallery opens with no entity under the cursor.
`lensTemplateListSource` is the whole of it: alphabetical rows whose `Aliases`
carry both the name and the `namedLensSummary`, so search finds a template by the
facets it sets, and whose payload is the stable ID alone.
`lensTemplateDisabledReason` asks the stable ID first and the case-insensitive
name second — reusing `namedLensNameTaken` rather than minting a second
uniqueness rule — so a renamed or replaced template stays unavailable while a
same-name stranger under a foreign ID reports the weaker collision. A disabled
row refuses through the shared seam: `selectedValue()` returns
`ErrListItemDisabled` wrapping the reason and `handleKey` turns it into a
`StatusMsg` without dismissing the overlay.

Availability is decided twice on purpose: once against the snapshot the gallery
renders and again inside `executeLensAddTemplateOperation` against the snapshot
the commit sees, because the machine can move between the two. The handler keeps
the section's commit-before-publish ordering — it requires the Outliner before it
writes, commits the record, and only then calls `ApplyNamedLens` — so a refusal
writes nothing and a failed commit applies nothing. Its refusals lead with the
template name because the status strip is where they land alone; the gallery's
row reasons sit under the name they describe and are worded from the template's
side instead. Nothing here is a workspace entity, so the add records no undo
entry either.

The nesting needs no routing of its own. `lensPickerActionRows` builds both
query-exempt rows and `lensPickerActionOperation` dispatches them, so a saved-lens
row falls through to `operationLensApply` exactly as before; row identity travels
as the stable `lens-picker:add-template` rather than as `Data`, which keeps
`CurrentLens` the single "is this a saved lens" test. Overlay key precedence is
`operationTextPrompt` → `operationValuePicker` → `contextActionSheet` →
`lensPicker`, so the gallery owns every key while it is open, and
`resumeContextActionRequest` already hands a resumed no-sheet request back to the
lens picker: a completed add therefore closes both overlays, a refusal is retained
on the main picker, and `OperationValueCanceledMsg` clears only the value step,
which is what returns `Esc` to the picker beneath.
