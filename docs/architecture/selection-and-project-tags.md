[← Architecture index](../architecture.md)

### Selection and subordinate tools

Visual selection stores project-qualified task identities independently of
current visibility and classifies them as total, visible, and hidden. `v`
toggles the cursor task, so a selection can name rows that are not adjacent and
movement never reshapes it. `V` starts a fresh inclusive task-row range and
movement recalculates it while skipping headers; `V` or `Esc` freezes the
result, and `v` freezes it while dropping the cursor row. `Ctrl+A` replaces the
selection with every visible task row. Bulk lifecycle, planning, priority, due,
tags, defer, and pin requests validate the complete selection before one
transaction. The three clipboard projections are the read-only member of that
set: the application layer resolves the whole selection from one snapshot,
deduplicates the project-qualified identities, and assembles every block before
the single adapter-owned clipboard write, so an unresolvable target or a failed
write leaves the clipboard untouched rather than holding part of a selection.
`OutlinerModel.ClipboardSelectionOrder` supplies the order that text reads in —
displayed rows first, then the hidden members by project slug and project-tree
position — and it stays local to those operations, because copied text is the
one case where target order is the result; every other operation keeps the
resolver's identity ordering. Structural and relation operations, edits, and
inspection reject visual selection rather than falling back to the cursor. Only
a committed Delete clears selection.

The Outliner opens on Active Projects. Applying scope clears navigation and
selection but preserves the active mode, search, and filters. `Ctrl+P` opens
the Project Hub in pinned mode, whose base population is pinned active projects:
parked projects remain available and archived projects do not. `P` opens Select
Project on a live cursor in all mode, over active and archived projects, each
row carrying its task count and its tag chips. Administering a project is a
direct key rather than a menu row: `Enter` focuses, `a` creates, `e` edits,
`*` pins, `#` sets tags, and `Ctrl+N` opens project Notes, which leaves the
`Space` sheet holding the two lifecycle actions and the bulk tag set. The
catalog-owned Unfiled ID is the protection authority. `/` opens search; `Esc`
leaves the search input with the query and its matches intact, so a search result set is browsable with the
full browse grammar; `/` reopens search with the previous query still editable.
`ShouldCaptureEsc` names the unwind rungs in order — the action sheet, the
query, then the selection — and browse holding none of them is what lets `Esc`
close Select Project. The selection is a rung because retiring `A` left no other
way to drop every checkbox at once.

The typed `ManageParams.ProjectHubMode` is part of route identity and is passed
through every reconstruction into `ProjectHubModel`. Pinned mode filters the
base population before the query and `Ctrl+T` open-task axis, labels its one
category Pinned Projects, and uses pinned-specific title and empty/no-match
copy. A live mutation rebuilds the surface by stable identity; an unpinned or
archived row therefore leaves both cursor and selection immediately rather than
surviving as a stale action target.

`ProjectHubInputStrategy` is that grammar, and it is a peer of
`SimplifiedInputStrategy` rather than a copy: the registry projects one strategy
into dispatch, footer hints, `?` help, and `operation.ValidateOperationOnePath`, so a
surface earns its own vocabulary by declaring bindings, not by adding a key
router. The Hub's cursor keys resolve to the core navigation operations because
moving a list cursor is the same act everywhere; those descriptors answer only
to the Outliner, so the Hub moves its own cursor and the keymap beside the
strategy carries their footer hints.

Select Project is a `ListSelectionMulti` surface: `v` toggles the row under the
cursor, `Ctrl+A` selects every match, and `Space` over a non-empty selection
opens a bulk sheet closed over Add tag, Remove tag, Park, and Unpark.
The four descriptors resolve `OperationTargetSelectedEntities`, so the sheet
freezes the project identities into the command context rather than into a host
value, leaving the executor's central value-provider step free to open the tag
prompt. `ListSurfaceModel.ActionState()` intersects operation IDs across the
selection, so an archived row in it offers no bulk action at all.

`projectHubAliases` is the one alias vocabulary every Hub row is built from, so
the active, archived, and pinned populations answer a query identically: the
name, the slug, the lifecycle phrase (`active project` or `archived project`),
the whitespace-collapsed description when it has one, and every normalized
project tag twice — bare and `#`-prefixed, because the chip the row draws is
written `#parked` and a query that copies what it reads must match. A blank
description is skipped rather than joined in, which would leave a stray
separator in the haystack `listItemScore` scores against. Because
`listItemScore` already folds `Aliases` into that haystack beside `PrimaryText`
and `SecondaryText`, widening the vocabulary needed no ranking change:
`sortListMatches` still orders by score with `sourceIndex` as the stable
tie-break, so pinned-first listing order survives a search.

Nothing a task owns enters that vocabulary. Select Project chooses a project,
and a task title, description, tag, context, or comment matching a project row
would promise the row holds work it does not name; the Outliner's `/` is where
task content is searched, and
`TestProjectHubSearchNeverMatchesTaskContent` puts every task-owned channel to
both Hub modes to hold the boundary.

`Ctrl+T` is the surface's other axis: `projectHubTaskFilter` cycles the list
through every project, only the projects holding at least one open task, and only
the projects holding none open, in both Hub modes, so the key works while the
search input owns keys and while browsing matches. The two narrowings are one
cycling axis rather than two toggles because they are mutually exclusive readings
of the same count. The filter runs at item-build time, so
`ListSurfaceModel.Reload` prunes hidden rows from the selection and rebuilds
matches against the retained query. Its data is `ProjectHubModel.taskCounts`, a
per-project tally of the open tasks beside the total that covers every cataloged
project — available, parked, and archived — rather than only the base task
workspace, so `tasks not loaded` leaves the Hub in the ordinary case. Both
narrowings read that tally's open count, which is the number the row prints first
in `N open · N tasks`, so the filter and the row cannot disagree; a known-empty
project prints the bare `0 tasks` and holds no open work, so it lands in
`without open tasks` beside a project whose every task is finished. A project
whose count is unknown stays
visible in all three states: `projectHubFilterHides` returns early on an absent
count, so neither narrowing may claim a project it could not read.

The footer hint and the two empty-list lines all read through
`projectHubTaskFilter.next()`, naming the state the next press installs rather
than the one the axis holds: with three states a single press no longer returns
to the whole list, so copy describing the current narrowing would leave the way
back unsaid. The axis is session state on a Hub that
`ManageHubModel.prepareActiveView` rebuilds on every `P`, so it resets on every open.

The preview pane carries two blocks with different owners. The header is
`ListPreview`, the renderer-neutral text snapshot taken when the item is built:
`projectPreview` spends a title row on the name and one `Summary` row on
`<lifecycle> · <tally phrase> · slug <slug>`, appending ` · pinned` only when
there is a pin to report. It declares no `Fields`. Five labeled rows cost six
lines of a pane thirty tall to print four numbers and a boolean, and
`Pinned: false` was noise on every project that had no pin. `projectTaskPhrase`
is the one reading of the tally, so the meta line and the row's own counts
cannot disagree. `Body` stays the description and is capped at
`listSurfacePreviewBodyLines` wrapped lines, since free text of any length would
otherwise swallow the rows the rest of the pane exists to fill.

Everything below the rule is the Hub's, through
`ListSurfaceRenderConfig.PreviewBody`: the renderer resolves the pane width,
subtracts the header it has just drawn and the rule, and hands the remainder to
the callback. Task rows cannot travel as `ListPreview` text, because a row is
styled per segment, sized to its column budget, and thinned by
`indicatorDropLadder` at the width it is drawn at — none of which an item built
before layout knows. So the hook lets the pane paint during the render instead
of letting the item carry a string it could not have sized. What it paints is
`RenderOutlinerTaskRow` on an unmounted `OutlinerModel` over the previewed
project: the pane shows the product's own task row rather than a second row
grammar that would drift from it, and it has no cursor, so no row opens Peek.

`projectPreviewTaskRows` flattens the tree once per project and
`projectHubPreviewState` caches it beside the painter until
`rebuildProjectSurface` invalidates both. Roots there are the tasks no other
task claims as a child rather than the tasks reporting no parent, so a task
whose `ParentID` dangles is still listed exactly once and the pane's row count
keeps agreeing with the total the header printed. The tasks come from
`projectTaskTally.Tasks`, retained by the count pass that already reads every
cataloged project from disk, which is what fills the pane for a parked or
archived project — each reaches the Hub as a rehydrated catalog record holding
no tasks of its own. A project the pass could not read says so rather than
rendering as an empty one.

Paging is `Ctrl+F`/`Ctrl+B` because `Ctrl+D`/`Ctrl+U` already move the list
cursor, and the cursor is the surface's subject while the pane merely follows
it. `PageProjectPreview` applies the lower bound alone: the pane's row capacity
belongs to the render pass that computes the height, so `listsurface.ListPreviewScrollWindow`
clamps the tail there, converging on a slice that pays for the continuation rows
it turns out to need. A page is `height-2`, both continuations always
subtracted, so a page down and a page up land on the same offset whether or not
a continuation happened to be showing; the price is one row of deliberate
overlap. `previewTasksScrollable` gates the footer hint on the pane being on
screen — `ListSurfacePreviewGeometry` is the same width arithmetic the renderer
runs — and on the cursor project holding tasks, never on a page count, which
would be one frame stale in the very pass that settles it.

`<` and `>` move the split. `ListSurfaceRenderConfig.ListWidthPercent` is the
list's share, zero meaning the 55% default and clamped to 25–75%, and
`ListSurfacePreviewGeometry` still lets both panes' minimum widths win over it,
so no percent can break the compact fallback. The value is session-only:
`ManageHubModel` outlives each Hub it mounts and lends every new one the same
`ProjectHubPreviewSplit`, so a reopen keeps the divider and a restart forgets it.
`ShiftPreviewSplit` moves it 5% a press and is inert while `previewPaneVisible`
says no pane is drawn — the same gate, minus a query being typed, that shows the
`</> split` hint.

### Project tags

Three orthogonal axes describe where work sits, and none of them substitutes for
another:

| Axis | Belongs to | Meaning |
| --- | --- | --- |
| Workflow state | a task | One of the eight lifecycle states. |
| Tags | a project | Free-form labels, one of which decides base task workspace membership. |
| Archive lifecycle | a project | Whether the project is retired from administration. |

A cataloged project carries a set of tags. `models.NormalizeProjectTags`
validates each entry with the ordinary tag rules, then lowercases, dedupes, and
sorts the whole set, which keeps the persisted catalog bytes deterministic and
makes the namespace case-insensitive. Load validation applies the same
normalization, so a persisted tag set the write APIs could not have minted fails
closed instead of loading. The protected Unfiled project may never carry the
reserved `parked` tag.

`models.ParkedProjectTag` (`parked`) is the sole behavioral project tag: a
non-archived project carrying it is *parked*. It is reserved in the project-tag
namespace only, so an ordinary task may still carry a `parked` tag with no
behavioral meaning. Every other project tag is descriptive.

`models.BehavioralProjectTags` maps each behavioral tag to its name, its
one-sentence summary, and the semantic badge role it renders with. It is the
single source every explaining surface reads — the picker explainer, the badge,
and the project editor's Tags field — and it is metadata only: adding an entry
explains a tag, it does not make one decide anything.
Those explaining surfaces wrap complete summaries inline, and the picker's
legend is the first thing a short terminal takes back: when the height budget
cannot show the rows the list holds, the legend is dropped whole and its rows go
to the list, which only then scrolls. The section-jump hint stays, since it names
live keys rather than restating a vocabulary, and the overlay still reserves
exactly what it rendered.
Focusable rows use the registry's compact `RowSummary`; previews and the inline
explainer use the complete `Summary`.

`models.SemanticTaskTags` is the matching registry on the task side, and the
parallel structure is deliberate: both publish the name and one-sentence summary
every explaining surface reads, and neither carries an effect. The labels differ
on purpose. A *behavioral* project tag decides something — `parked` removes a
project from the base task workspace — while a *semantic* task tag only names a
kind of work, touching no filter, sort, or workflow rule and never coloring the
title. The shapes
differ for the same reason: `BehavioralProjectTags` is a map keyed by tag name
because its entries are independent, and `SemanticTaskTags` is an ordered slice
because the `bug` > `question` > `epic` > `story` > `feature` > `idea` precedence
is part of what it publishes. That precedence does two things. The `#` and `f#`
pickers read the slice order to build their `SEMANTIC` section, and those rows
and the explainer line beneath them resolve one color through
`theme.SemanticTaskTitleStyle` — a per-role accent the vocabulary reserves, whose
`...Title` palette keys are a historical name kept because the palette is
published. The picker row wears the color the vocabulary gives that name, which
is the strongest thing the row is about; it is not a preview of a title, because
the title it would have painted is neutral text. And `theme.GetTaskStateBadge`
resolves a task's several semantic tags down to one — the highest-precedence
match, case-insensitive, regardless of storage order — and tints the state
glyph's foreground with that same accent, keeping the workflow symbol and badge
background; the capture preview borrows that badge, so a capture previews the
glyph its row will wear. That glyph is the only single-winner surface: each tag
still reaches Peek as its own chip, and a task row draws no tags.
Their seeding is asymmetric on purpose: `#` offers all six whether or not any
task carries them, because offering an unused tag is how it gets discovered,
while `f#` stays usage-derived, because a filter row nothing carries can only
empty the projection. The retained `internal/tui/listsurface/tag_selector.go` path reached
from `internal/tui/handler_helpers.go` (`ShowTagSelectorMsg`) is the one tag surface that carries
no semantic tier, because it never reads `taskTagListSource`; the obsolete
bulk-selector message path has been removed.
Those explaining surfaces wrap complete summaries inline, and the picker's
legend is the first thing a short terminal takes back: when the height budget
cannot show the rows the list holds, the legend is dropped whole and its rows go
to the list, which only then scrolls. The section-jump hint stays, since it names
live keys rather than restating a vocabulary, and the overlay still reserves
exactly what it rendered.
Focusable rows use the registry's compact `RowSummary`; previews and the inline
explainer use the complete `Summary`.

`theme.TagChipStyle` (`internal/tui/theme/styles_helpers.go`) is the single tag
color authority, and it generalizes that "one lookup, so the surface previews the
effect" rule to every tag rather than the six semantic ones. It hashes
`models.NormalizeTag(tag)` onto a slot of `tagChipColors`, which
`rebuildTagChipColors` re-resolves from `config.DefaultTagChipRamp(variant)` on
every `ApplyTheme`. The slot index, not a resolved hex, is therefore the durable
unit of tag identity: `T` re-colors every chip and slot `i` stays the same color
family in both themes, so toggling never reshuffles which tag wears which hue.
The ramp is the explicit tier below the reserved accents — six slots at ≤65%
saturation, gated in `internal/config/theme_defaults_test.go` for perceptual
distance from each other, from the ten accent colors, and from both alert row
backgrounds — which keeps a chip quieter than the badges beside it by
construction rather than by convention.

The ramp is a mnemonic, not a legend. A slot promises "this tag is reliably that
color", never "this color means that kind of tag": six slots over an unbounded
vocabulary collide by design, and two unrelated tags sharing one hue is expected
rather than a defect. Nothing reads a chip color back, so no behavior can depend
on one. Do not engineer that ambiguity away by adding meaning to a slot.

Which surfaces resolve through it splits in two. A surface that renders a tag
*as itself* takes the chip: the Peek `Tags:` line and every
row of the retained tag selector, whose `TagInfo` payload answers
`ListSurfacePrimaryStyle` (`internal/tui/listsurface/list_surface_adapters.go`) so the
initial adaptation and each `ReplaceItems` land one color without either call
site knowing. A picker row is an offer rather than a label, so it previews the
color it is about to paint, which makes a colored chip and a colored row one
rule seen twice: a row wears the color of the strongest thing its tag does, and
its chip color only when it changes no other color. So a `SEMANTIC` row takes
the reserved accent `theme.SemanticTaskTitleStyle` and a
behavioral project row its badge role, while the descriptive rows of both `#`
editors and of `f#`, the tag rows the `#` picker's `CONTEXTS` section collects
by context usage — `f#` reads a census with no context counts in it, so that
section never appears there — the minted row an inline add mode produces, and
the facet rows
of `F#` fall back to `theme.TagChipStyle`. One tag can therefore wear two colors
across surfaces — `#bug` is the vocabulary's red under `SEMANTIC` and its own
slot color as a chip — so a document or test expecting a semantic picker row and
its chip to agree is naming a contract that does not exist. Two rows stay plain
deliberately: the `##name` context radio rows, which write a field rather than
name a tag and whose "No context" clear row would read as disabled in
`MutedStyle`, and `F#`'s `Untagged` sentinel row, which names the empty tag set
rather than a tag. The Project Hub is outside the split entirely:
`ProjectTagChips` joins names into `ListItem.SecondaryText`, which has no
per-tag style hook, so those tags render in the row's one secondary color.

`OrderedTaskTags` (`internal/tui/listsurface/shared_render_helpers.go`) is the matching
order authority: every non-semantic tag alphabetically, then every semantic tag,
also alphabetically. Storage keeps tags in insertion order, so this is what makes
the tag set rather than the typing history decide the reading sequence. Peek is
now the one surface rendering a task's chips — the row carries no tags — so the
order no longer decides which chips survive a width, and there is no overflow
count to keep honest; it stays a pure permutation so nothing can be dropped on
the way to a wrapped line. Semantic tags are deprioritized, never excluded: the
six names are a vocabulary every task shares, so the tags that cannot be guessed
lead, and `RunOptions.DisableColor` only reaches `tea.WithEnvironment` where no
renderer can detect it, so a task tagged only `bug` must still show `#bug`.

`#` is bound to two descriptors, `tui.task.set-tags` and
`tui.project.set-tags`, marked `SharedKey` on the input strategy. `*` pairs
`tui.task.toggle-pin` with `tui.scope.pin-project` the same way. A descriptor
carries exactly one `TargetSource` and one `OperationCategory`, so a single
polymorphic operation would have to branch on runtime context inside
`defaultOperationTargetSource`. Instead the trie node holds both bindings and
resolves the first *applicable* one in declaration order, which lets the cursor
decide the meaning while `?` help and contextual guidance keep two honest labels. A key
declared twice without `SharedKey` is still a hard registration error.

Both pairs are disjoint because `ProjectOperationApplicable` refuses a section
header outright. Without that gate the pair would not be disjoint at all: the
project arm falls back to `currentProjectID`, which degrades to the sole scoped
project, so in a single-project scope a lane header — which names no project —
would answer a project key with the scoped project it never named. The check
reads the bare `HasSectionHeaderCursor` flag rather than pairing it with an empty
`ProjectID`, because `OutlinerModel.operationCommandContext()` fills `ProjectID`
from `m.project` on its fallback arm while `AppModel`'s does not; a `ProjectID`
clause would make the footer advertise the key where dispatch refuses. Both
builders set the flag identically from `cursorNamesSectionHeader()`.

The task descriptor is `Set Tags & Context`, because the singleton task context
is edited in the same overlay. Its rows carry a `context:` ID prefix and a
`taskContextRow` payload, which keeps them apart from the `tag:` rows the
`CONTEXTS` section also collects by context usage; the clear row takes the
reserved `context:0` name, so no real context can mint a colliding ID. Context
is single-valued, so those rows answer `#` as a radio group and refuse `Space`
by name, while `a` mints a tag or a context depending on the row under the
cursor. The picker configures tri-state membership at every cardinality, so it
always emits `OperationTagsValue`; a chosen context that differs from the one
every target already agreed on rides that payload into
`TaskOperationRequest.ContextSet`, and the same `bulkAttribute` mutate closure
writes the tags and the context, so a simultaneous edit is one transaction and
one undo entry.

That membership is also the order the overlay opens in.
`FreezeAppliedValuesFirst` runs once, in the factory, immediately after
`ConfigureValueMembership`, because the snapshot it reads is what that call
establishes: every row the targets already carry — checked or `[-]` — is stamped
with a presentation `GroupID` of `TaskTagAppliedCategory` and moved to the head
of the item slice, sorted by the same `taskTagPickerLess` the sections below it
use, and the cursor is reseated on the first of them. `listsurface` buckets and
heads rows by `ListItem.GroupOrCategory()` while every other reader keeps asking
`CategoryID`, which is the whole separation: a promoted row is drawn under
`APPLIED` and is still a semantic, context, or descriptive value, so `s`, `c`,
and `t` reach it, a minted one enters its own section, and the chip colors are
unchanged. `jumpToTagCategory` prefers a row still drawn under its own section
and falls back to a promoted one, so a section the promotion emptied answers
instead of refusing.

Nothing recomputes that grouping afterwards, which is what freezes the order:
`ToggleCurrentMultiValue` and every other membership write goes through
`syncValueMembershipRows`, which rewrites `Indeterminate` and calls `SetItems`
with the rows in the order it found them, so a toggle changes a mark and moves
nothing. Reopening the overlay is what recomputes it. The clear-context row is
excluded by ID: every target without a context agrees on it, so it would
otherwise read as applied while naming no value. The promotion is gated on
`ValueProviderTags` alone — the filter vocabularies, the project-tag pickers, and
the dependency picker share this surface and keep their source's own order.

Setting the reserved tag is the one project tag edit that confirms first. The
prompt names the task count leaving the base workspace, the committed status
names both recovery routes, the population's `ReloadHeaderProjectID` lands the
cursor on a surviving project header instead of letting `ClampCursor` choose,
and the base scope anchor reads `Active Projects · N parked` for as long as
anything is parked. Descriptive tags apply with no prompt.

A bulk tag edit is resolved before anything commits: every named project is
looked up, its replacement tag set computed, and no-op projects dropped, so the
prompt can name the real impact and a request that would write nothing says so
instead. The resolved effect decides the prompt rather than the chosen action,
which makes typing `parked` into Add tag confirm exactly like Park does, and the
whole edit lands as one catalog transaction and therefore one undo entry. A bulk
park reuses the same cursor anchor as the single-project path, passing every
departing project so the anchor skips them all; `anchorOutlinerCursorAfterProjectPark`
self-guards on the active Outliner, which is what leaves Project Hub parking
alone. With no surviving header the anchor deliberately names a departing
project, which stops resolving after the reload and settles the cursor at the
top of the projection.

The `F#` picker doubles as the tag vocabulary editor: `v` toggles a facet and
`Space` on a tag row opens a sheet closed over Rename everywhere, Remove
everywhere, Park all members, and Unpark all members. The sheet freezes the
tag's member projects into the command context and emits an ordinary
`ProjectBulkTagsMsg`, so it inherits the park confirmation, the single catalog
transaction, and the undo entry. The edited tag lives on the AppModel rather
than in the sheet's frozen value because a nested value step overwrites that
value on its way back to the executor. Remove everywhere routes through a value
provider that returns an immediate empty value when no member would be left
without any tag, and opens a destination picker only when one would.

The two project populations stay separate by name.
`Runtime.TaskWorkspaceProjects()` returns non-archived projects that are not
parked and is the only population the base Outliner reads.
`Runtime.AvailableProjects()` returns every non-archived project, so a parked
project remains reachable for Notes, capture and inline-edit `%project`
destinations, Select Project, and one explicit project focus. Focusing a parked
project anchors the Outliner with the `parked` tag appended to the project name,
and leaving that focus returns to the base workspace with no parked rows behind.

Parking a project is a catalog-only change. Its Today and Next references stay
durable and are filtered at read time instead of being cleared, so unparking
restores the plan untouched. Archiving preserves a project's tags and restoring
returns them unchanged.

The workspace-wide Notes manager carries an explicit `NoteLocation` plus
filename identity with every row and opens in All Notes unless the Outliner
cursor names one active project header, which opens that project's scope. One
workspace snapshot loads Global plus every project returned by
`Runtime.AvailableProjects()`; archived projects are not owners, but a parked
project remains one. Successfully
loaded rows form one canonical collection, then the selected All, Global, or
project scope and persistent title/body search derive the visible list. Rows
sort by modified time unless a search is active, when relevance leads and
modified time breaks ties, and every row renders its owner label. Title and body
search follow the same smart-case rule as the Outliner, and a relaxed retry says
so in the count label.

`p` opens the searchable scope picker, `/` edits the accepted search, `a`
chooses a creation destination, `e` renames, and `%` moves one parsed note to
another Global or active-project owner. `Space` opens the shell's context action
sheet over the single row under the cursor, hosted as `ContextActionHostNotes`
and holding Delete alone; Delete is menu-only and delegates its confirmation
back to the manager that owns the row, which is why Notes gains a sheet without
gaining a selection. `NotesInputStrategy` is a peer strategy that declares those
bindings, and `ValidateNotesOperationOnePath` holds them to the same
direct-or-menu rule as the Outliner and the Project Hub.
Open/edit, create, rename, move, delete, undo, and redo
all refresh the affected canonical collections through the workspace ledger;
move commits its destination write and revision-checked source delete as one
artifact transaction. Malformed notes remain available for raw editing and
deletion but cannot be renamed or moved until repaired. Search and scope are
session-only. Note tags, task links, pins, bulk actions, and archived
owners remain outside the product. Editor resolution is
`TT_EDITOR`, then `VISUAL`, then `EDITOR`.

The body preview is one more consumer of the shared list-surface layout rather
than a second responsive grammar. `renderNotesContent` asks
`ListSurfacePreviewGeometry` for the split, so the pane appears at exactly the
72 columns every list surface widens at and the list renderer is handed the
resolved pane width instead of reading the terminal's. The header goes through
`RenderListSurfacePreview` — title, owner label, and modification time — and its
`PreviewBody` hook draws the note body; `JoinListSurfacePanes` joins the two.
An empty list, a workspace error, or an open location picker has no note to
preview and keeps the full-width list.

`notesPreviewState` caches the body rendered through `textfit.RenderMarkdown`
at the pane width, keyed on note identity, the Notes scope, the active query
with the case mode that produced the visible rows, the revision, and that width
together.
The revision is `RawRevision` when the note carries one and a digest of the body
otherwise, so an in-memory edit that never reached disk still moves the key. Any
one component moving re-renders and re-anchors, which is what makes cursor
moves, reloads, edits, query changes, and scope changes invalidate by
construction; while the whole key holds, the offset is the reader's and ordinary
re-renders leave it alone. `SetSize` drops the cache outright, since a resize can
change the pane width, the row budget, or whether the pane exists at all.

`notesPreviewAnchor` decides where a rebuilt cache opens. Search scores the raw
body, so a hit is located again against the rendered, wrapped, ANSI-stripped
lines: every term is tried for a literal occurrence first, and only if no term
is found that way does a per-line fuzzy pass run, so an incidental subsequence
near the top can never overrule a literal hit further down. The earliest line
found wins and the anchor spends one row above it on context. A title-only
match, an empty query, and a region the rendered text cannot express all yield
the top. `Ctrl+F`/`Ctrl+B` move the offset by the page size the render recorded
and apply only the lower bound; they are claimed ahead of the search text input
while a query is being edited, so refining a query and reading the note it lands
on are one interaction, and the keys neither accept the draft nor reach the
input. The render owns the
tail clamp, because only it knows how many rows the header left:
`ListPreviewScrollWindow` converges on a window that reserves a row for each
`RenderListContinuation` it needs, the same accounting the Project Hub task pane
does through the same helper. A page is the budget less both continuation rows,
so consecutive pages overlap by one line rather than drifting apart. A note
whose `ParseError` is set renders that reason as a warning and never as
Markdown, since only its filename is reliable. That warning is not a body, so
paging is gated on `notesPreviewPageable` rather than on `notesPreviewVisible`:
the pane can be on screen with nothing a page could move, and the narrower
predicate is what the keys, the `?` help row, and the footer hint all read, so
an unparsed note advertises no control and cannot have its offset written.

### Project lifecycle

`internal/engine/projects.go` is the one place a project is created, renamed,
described, retagged, parked, archived, restored, or deleted. Each function
prepares on a cloned catalog and planning state, commits through the
application service's checked transaction and undo ledger, and refreshes the
runtime state only after the commit. The desktop's project creator, project
editor, and project delete call it, and the mobile bridge's `command.project`
serves the same functions, so both frontends write the same catalog records,
`tasks.md` rewrites, and directory moves. `engine.MeasureProjectImpact` counts
the tasks, notes, and attachments a delete would take from the project's own
directory, which is what lets an archived project be measured as honestly as an
active one and what the bridge's guarded delete compares against.
