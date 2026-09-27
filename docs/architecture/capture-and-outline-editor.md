[← Architecture index](../architecture.md)

### Capture and inline-edit grammar

One parser in `internal/textgrammar` serves capture, inline edit, the external
editor's title line, and the outline buffer; the surface decides which fields it
admits. Capture resolves the target
project first, then the project-local parent, then the project-local children.
Omitted destinations default roots to Inbox and children to Backlog, and adopting
children changes neither default, because `normalizeCaptureDestination` reads
only `ParentID`.

`coreCaptureMessage` resolves the destination from the task Focus root, then the
scoped project, then `cursorProjectHeaderProject()`, then Unfiled. A Focus root
that `taskAcceptsDeeperChild` admits opens the child preset built by
`focusChildComposerPreset`; at the nesting cap only its project survives the
step. Focus is not the cursor — it is the explicit projection anchor `Enter`
pushed — so it precedes the header row. The header step is the only place
capture reads the cursor, and it is safe because the function returns nil on
every task row: capture still never borrows a task cursor's project. The
composer names whichever destination it resolved through
`TaskComposerPreset.HeaderLabel`, set in `captureComposerPreset` and
`focusChildComposerPreset`, so the three routes are never told apart only by
where the task landed.
`A` on a project header row keeps rejecting, since a project has no task to
parent under.

`D` reaches the same composer over a draft instead of a blank line.
`Service.PrepareTaskRepeatDraft` reads one task through the ordinary snapshot
and returns a `TaskRepeatDraft`: a value carrying the retained fields and the
Backlog destination, every slice copied, nothing written. `repeatComposerPreset`
wraps it as `TaskComposerPreset.Seed`, and `NewTaskComposerModel` hands a
non-nil seed to `seedExpandedForm`, which builds the expanded `TaskFormModel`
straight away and leaves `Mode` on `ComposerExpanded`. `View`, `Update`, `Init`,
`SetSize`, and `SetTaskFormCoordinator` all delegate to `Expanded` when it is
set, so a composer constructed in expanded mode needs no other wiring than the
one `openTaskComposer` already does.

`seedExpandedForm` is `ExpandRequest`'s sibling and deliberately not a call into
it. An expansion starts from a parsed capture line; a seed starts from stored
field values that must never meet the parser, which is what keeps a title of
`Fix bug #123` a title. That is also why the seed is a typed struct beside
`InitialText` rather than a spelling of it: the compact surface parses that
field, and a repetition routed through it would mint tags out of prose. What the
seed omits is as load-bearing as what it carries — `NewTaskFormModelCreate`'s
defaults supply no parent, due date, deferral, pin, review flag, subtask,
attachment, comment, or reference, so the cleared half of the repeat contract is
the form's resting state rather than a list of assignments a later edit could
forget. `TaskComposerSeed.ChecklistItems` is `[]string` for the same reason: a
struct that cannot spell a tick cannot carry one across.

The seed leaves `presetDestination` on Backlog, which `IsPlanning` rejects, so
`saveCreate` reads the destination off `draft.state` and a repetition never
inherits the source's Today or Next membership. Creation itself is unchanged:
the draft rides the same `AddTaskRequest` every expanded save builds, so
identity, `ShortID`, timestamps, checklist persistence, and the single undo
entry all come from the one creation transaction. Opening a draft and
abandoning it writes nothing at all.

#### The capture panel and its host

Compact capture is a bounded panel pinned to the bottom of the content area,
drawn over the presentation it was raised from. `route.HostedPanelView` is how a
view declares that, and `chrome.HostedPanelLayout` is the value it returns: one
`HostRows` budget and one `HostDrawn` flag, with the zero value meaning the view
owns the whole area. That is what lets the composer declare both of its
presentations without a second type — `ComposerExpanded` returns the zero layout
and the full form takes the screen.

The shell does not draw the panel on top of its host; it hands the host the rows
the panel left and re-renders it into them, then stacks the two with
`chrome.ComposeHostedPanel`. Overlaying would hide the host's bottom rows, and
the row a list surface most needs to keep is the one its cursor is on.

Handing those rows over is not a resize, and `route.HostedBudgetView` is where
that distinction lives. It sits beside `route.ShellFrameAware` for the same
reason that one exists: a list surface drops its scroll window every time it is
resized, so a temporary budget travelling through `SetSize` is indistinguishable
from a terminal that changed size, and the window the user was reading is gone
either way. The shell calls `SetHostedBudget` for every budget a panel leaves a
host — the first and every later one, since a panel that changes height changes
the loan rather than taking a second — and `ReleaseHostedBudget` exactly once,
with the whole content area, when the panel closes. `applyHostedBudget` and
`releaseHostedBudget` in `app_view.go` are the one dispatch: a host implementing
neither is resized for both, which is what it always was.

`OutlinerModel` implements it. `hostBudgetAnchor` is captured at the first budget
and re-seated on every later one, so a panel growing with its draft, a syntax
guide opening, and an expanded form collapsing back are one loan with one
repayment instead of a drift whichever height came last decided. What it holds is
a position rather than a number — the top task row's qualified identity, the
cursor's behind it, and the held offset behind that — because capture commits
work into the projection underneath it, and a window put back by row number would
slide the document under a user who had just captured above their position.
`seatWindowAtAnchor` writes `ViewScrollOffset` and nothing else: keeping the
cursor visible and cutting the window to the document belong to
`ComputeOutlinerViewport`, which runs on the frame the seat is for, so a
shortened window still clamps no further than the cursor requires. `SetSize`
drops the anchor along with the window, so a terminal resized while a panel is
open ends the loan rather than resurrecting one it already dropped.

The panel's height is what its content asks for. `composerPanelContentRows` is
that demand: `composerPanelFixedRows` for the divider and the route row, the
input's own wrapped line count, the error block when there is one, and the pane
under it — the suggestion list's items while autocomplete is open, one row for a
multiline draft's summary, and otherwise the grouped preview's height through
`grammar.QuickCaptureInput.PreviewBlockRows`. `composerPanelShareRows` turns that
into the share the host is told about, between `composerPanelMinRows` — the two
fixed rows and one input line, which a blank capture is — and
`composerPanelMaxRows`, the fourteen-row ceiling the bounded paths were written
for. `composerHostMinRows` is the other end of it: four rows the host keeps, and
a frame too short to seat both spends what it has on those first.

Two properties are what make that measurement safe to run on every frame. Every
term reads settled state — the value the input already holds, wrapped at the
width it already has, the errors the last submission recorded, `len(m.AC.Items)`,
and the parse the last keystroke stored — so nothing is re-parsed to learn a
height and no term reads the panel's own height, which is what a loop between
the two would need. And the rows the shell's footer costs beyond its two-row
baseline ride the *share* rather than the drawn panel: `ExcessRows` is added to
the demand and to the floor, and `compactPanelRows` subtracts it again for the
render, so the panel draws exactly its content while the host is charged for the
footer that content stands above.

Inside the ceiling the bounded paths are unchanged, and `composerInputRows` is
where the priority falls: the body is the share less the two fixed rows, the
input keeps its first row unconditionally, and every row beyond it yields to the
error block, so a refusal is never what scrolls off a panel that has run out of
room. The pane below it is windowed by the preview and autocomplete paths that
already existed, which is why the ceiling is the number they were sized against.

The syntax guide is the one presentation that measures as the pane it replaced.
Expanding it sets `HostDrawn` false — the panel takes the frame while the guide
is up — but it moves no term of the demand in either direction, because a share
that shrank while the guide was open would hand the host a smaller budget and
cost it the window `hostBudgetAnchor` exists to keep. For the same reason
`syncHostedPanelHost` runs once per `Update`, in the shell's one deferred
closure, rather than once per frame.
`hostedPanelHostRecord` names the one view owed its budget back, since a sweep
over the registry would reset every other surface's window to say so;
`syncHostedPanelHost` still reads the host's own `Width()`/`Height()` rather than
trusting that record, so a re-registration that reset the host to the full budget
is corrected on the next update instead of silently clipping its cursor row away.

The panel has no title row, so it also has no workspace pulse: the host under it
is still painting its own, and `TestPresentationChromeHostedPanelLeavesThePulseToItsHost`
holds the composition to exactly one. `chrome.RenderLabeledDivider` is the
heading form it uses instead — `── Capture ─────`, one row, the plain rule when
there is no room for a label. The label names the act and stops there: the
destination is the route row's, which `Ctrl+P` rewrites, so a second copy on the
divider would be both a repetition and the half that goes stale. A child capture
is the exception, `── Capture → child of Launch platform ──`, because no axis of
that row carries a parent. Its styles are built locally rather
than taken from `theme.SubtitleStyle`, which carries a bottom margin that on a
single row is a newline through the middle of the divider.

`renderComposerRouteRow` is the row under that divider, and
`effectiveRouteHeader` is what it reads: three independent axes — project, plan,
status — each aggregated across the draft's lines on its own, so a multiline
capture that agrees about the project and disagrees about the destination says
`Mixed` in one field rather than in all three. The plan half is written only when
`Destination.IsPlanning()`, because the eight lifecycle destinations already
spell their own status and a `Plan:` label over nothing would claim a membership
the capture does not create. The row names words alone, no icon and no `$` token,
since an axis that can read `Mixed` is one no sigil can spell; the preview keeps
the sigil echo alone, since it is only ever drawn beneath this row and a second
copy of the pair would say the same two words twice. The spellings come from
`chrome.PlanFieldLabel`/`StatusFieldLabel` through `chrome.PlanStatusValue`,
which is the one place a canonical lowercase token is capitalized.

`ComposeBoundedPreviewBlock` is the preview's form of the same bound. Where
`ComposePreviewBlock` protects a lower pane, the panel has none, so the choice is
between the grouped preview while it fits whole and the compact wrapped run that
names every recognized attribute. Keeping the first few groups and clipping the
rest would read as a complete preview, which is the one outcome a capture surface
may not produce. The discard question is left to the shell's own confirm dialog
for the same accounting: a second copy inside the panel would spend a preview row
repeating what the overlay already asks.

`##context` is the singleton task-context sigil. It shares tag normalization,
but projects one optional field rather than a repeatable tag set. Capture and
outline input reject `##0`; inline edit consumes it as the explicit clear while
the outline buffer clears context by omitting the token. That permanently
reserves `"0"` as a context name even though `!0`, `@0`, and `>0` retain their
ordinary meanings.

The expanded form carries the context too. `AddTaskRequest.Context` is the
optional field `normalizeAddTaskRequest` runs through
`models.NormalizeTaskContext` only when it is non-empty, mirroring the branch
`prepareCapturedTask` uses, so an empty value is the unset field rather than a
refusal and `buildNormalizedAddTask` writes it beside the priority. On the form
side it is the eighth Basics row, an inline `textinput` like Schedule's Due
rather than a picker, because a context is a free value with the tag spelling
contract and not a closed set. `commitAllTextInputs` flushes it into
`taskFormDraft.context`, `normalizeDraftContext` canonicalizes it before the
request is built — an empty field clearing it, an unspellable one landing the
error back on that row with the draft still open — and `saveCreate` passes the
result on the same `AddTaskRequest` the checklist rides, so a context and its
checklist commit in one `ChangeSet` under one undo entry. The field is bounded
at `models.MaxTagLength` so it cannot truncate a value the canonical writer
would have accepted.

`@@defer` is the deferral sigil, and it lives one character away from `@date`.
Go's RE2 engine has no lookahead, so the separation is structural instead: the
due pattern's last alternative is `[^\s@]\S*`, which refuses a leading `@` and
makes `@@` unreachable by the due pattern whatever order the registry is walked
in. No legal due value starts with `@`, so the tightening costs nothing, and
`TestDueSigilCannotSwallowTheDeferSigil` and
`TestDueFormsSurviveTheDeferTightening` pin both halves of that claim.

A deferral is an instant rather than a calendar day, which is why the sigil
accepts the `+Nh` offsets `util.ParseCalendarDate` refuses by name. It also
carries a lifecycle effect the other metadata sigils do not: `deferralRefusesState`
is the terminal half of `deferTask`'s policy, shared by capture and inline edit so
a refusal is worded the same wherever it is expressed, and a deferral that is
honored claims Backlog. `grammar.ComposerDeferredDestination` applies that route to both
the composer's header and its preview, so neither promises a landing place the
submitted request will not honor. Entering a state clears `DeferredUntil`, so both
surfaces write the instant *after* the transition — `buildPreparedCapturedTask`
after `initializeCreatedTask`, and `applyPreparedTaskLineDeferral` after
`applyTaskLineDestination`. Written alongside the other scalars it would be
wiped by the Backlog move the deferral itself asked for.

On the search surface `@@any` and `@@none` split the population the deferred
filter facet splits, reading `IsDeferredAt` rather than the field's presence, so
a lapsed deferral counts as awake. A value that names an instant is literal
instead and compares calendar days, which is what makes `@@+2h` and `@@today`
name the same set. The outline surface declares no defer form at all, and
`parseOutlineBodies` rejects the token by name for the same reason it rejects
`%project`, `>parent`, and `<child`: `applyOutlinePlan` has no field to put it in,
so silently folding it into a title would be the only alternative.

`*pin`'s value set is closed to `pin` and `unpin`, and an unrecognized value is
not consumed: `Refactor *args handling` keeps `*args` in its title. That is the
tag rejection behavior rather than priority's, and the asymmetry is deliberate.
Priority captures `!9` in order to diagnose a typo, because `!` followed by a
digit is not something a title says by accident; `*` occurs in ordinary prose and
in code identifiers, so pin would diagnose far more real titles than typos.
Re-pinning an already-pinned task preserves `PinnedAt`, since that field breaks
the pinned section's ties and every outline apply would otherwise reshuffle
them.

`&link` is the attachment sigil, and it reuses `*pin`'s rejection behavior for
the same reason: `&` is ordinary prose. `validAttachmentTokenValue` is a purely
lexical test — an `http://` or `https://` prefix, a `/` anywhere, or a
`name:digits` shape whose name half contains a dot — so `R&D`, `Q&A`,
`&10:30 standup`, and `&nbsp;` are never consumed, while `&main.go:42` is. The
test is lexical because `internal/textgrammar` must not import
`internal/storage`; whether a path resolves is `app.ResolveAttachmentInput`'s
question, asked once so every surface classifies a paste identically.

How far one `&` token reaches is a second question, separate from what it means.
A path a file manager copied holds spaces — `~/Library/Application Support/…` is
where a screenshot tool writes — and a bare `\S+` run stopped at the first one,
attaching half the path and leaving the rest in the title. The pattern therefore
tries a single-quoted span, then a double-quoted one, then a run whose spaces are
backslash-escaped, before that bare run. Widening the span is all those
alternatives do: `normalize` on the registry entry runs
`util.NormalizePastedPathInput` — the same normalizer the Attachments panel runs
on a paste — over whichever alternative matched, and `tokenValue` is the one
place it runs, so the value a token carries and the value `placementDiagnostics`
re-validates cannot disagree. The lexical test then reads the plain path, which
is why quoting never exempts a value from it: `Ask &"why" before the demo` keeps
its quoted phrase in the title, and an unterminated quote falls through to the
bare run and stays the prefix it already was.

That sigil is also why the registry now carries two orders. `registry` is parse
order and `guideOrder` is teach order, projected by `Definitions` and
`GuideDefinitions` respectively. The attachment entry has to sit ahead of
`FieldTag`, because RE2 has no lookaround: `#([\w][\w.:-]*)` stops at the `&`, and
`extractTokens` replaces the matched span with a single space, so peeling
`#work` out of `Task #work&http://x/y` would leave a whitespace-preceded `&` and
mint an attachment the user never wrote. Peeling `&` first leaves it glued to
the tag, where it stays literal title text. Parse order is not a curriculum,
though, so `guideOrder` teaches attachments last, beside their own group, and
`SyntaxGuideGroups` is the one consumer reading that projection.
`validateGuideOrder` runs at `init()` and asserts the two lists hold identical
sets, which is what makes the split safe: a field added to one and forgotten in
the other would otherwise vanish from the guide or gain an unreachable entry.

An `&link` value reaches the application layer as a raw string on
`CaptureRequest.Attachments` or `AddTaskRequest.Attachments`.
`resolveCapturedAttachments` classifies the whole line before anything mutates,
adding the two guards a single paste does not need — `storage.SafeChildName` on
the destination filename, and a per-line dedupe so two files resolving to one
name refuse by name instead of overwriting each other. `applyCapturedAttachments`
then records the metadata and returns the `storage.AttachmentCopy` ops, which
ride the capturing task's own `ChangeSet` through
`commitProjectWithSideEffects`. Metadata precedes staging exactly as in
`AttachFile`, so a rejected attachment aborts the capture without writing a byte,
and one undo entry takes back the task and its attachments together. In
`CaptureMany` the ops are grouped per project onto each `projectCommit`, and they
are built after the hierarchy guards run so a rejected batch has staged nothing.
The sigil is capture-only: `parseOutlineBodies` rejects it beside `%project`,
`>parent`, `<child`, and `@@defer`, and search names it in `refusedSearchFields`,
which re-validates the captured value on `SurfaceCapture` first so only a
genuinely attachable value blocks a query. `ExpandSingle` no longer refuses it:
the expanded form's Extras tab is the attachment channel it was missing, so
`ExpandRequest` seeds `taskFormDraft.attachments` from `CaptureRequest.Attachments`
and the two halves of that rule — a refusal only while the value has nowhere to
land — moved together.

The Extras tab is the fourth `formTabNames` entry and the second dynamic one,
counting its focusable rows from its two collapsible sections exactly as
Children does. `a` toggles the Attachments section and `c` the Comments one,
`Ctrl+L` appends an attachment draft and opens `formOverlayAttachmentEdit` on
it, `Ctrl+Y` appends a comment draft and opens `formOverlayCommentEdit`, `Enter`
edits the focused draft or toggles the section its header names, and `Ctrl+D`
deletes the focused draft. `Ctrl+J`/`Ctrl+K` reorder attachments only: a comment
carries its own creation timestamp, so its position is chronology rather than an
ordering the author chooses. Confirming an item overlay with a blank value drops
the draft, which is the rule `TaskFormChecklistItemEditedMsg` and
`TaskFormSubtaskEditedMsg` already followed; `discardCanceledNewItem` is the one
place all four undo a canceled add. `saveCreate` skips blank drafts and passes
the raw values through as `AddTaskRequest.Attachments` and
`AddTaskRequest.Comments`, so the form classifies and normalizes nothing and the
task, its bytes, and its comments commit in one `ChangeSet` under one undo
entry. Extras is also why `buildKeyMap` became tab-aware: its `Ctrl+D` and
`Ctrl+J`/`Ctrl+K` mean something different from Schedule's due-date picker and
Children's reorder rows, and its `c` means something different from Children's
checklist toggle, so the Schedule, Children, and Extras blocks are each scoped
to `m.activeTab` wholesale rather than row by row, following the precedent
`guidanceSpec` set.

Two collapsible sections of drafts are also how the form outgrew its height. The
body used to be cut to the rows that were left and the cut row replaced with
`↓ more (resize terminal to view all)`, which was accurate and useless: the
fields below the fold were unreachable at any focus, so a reader holding a
twelve-item checklist at the minimum supported height had no remedy but a bigger
terminal. The four tab renderers now write into a `formBodyBuilder`, which is a
`strings.Builder` that also records the body line each focusable field starts on.
A field is declared once, where it starts, and `formFieldSpan` reads its extent
off the next declaration, so a renderer that already emits its rows in focus
order gains bounds without bracketing anything, and a value row, a wrapped
textarea, and a section's empty state are all measured the same way.

`formViewportOffset` turns that into the first visible line, purely: it takes the
body's height, the marks, the focus, and the row budget, and no render writes
scroll state, so two renders of one form cannot disagree about where the window
sits. A focused field is scrolled into view whole when it fits and pinned to its
own first line when it is taller than the window, since a control whose start is
off screen cannot be read at all while its tail is one `Tab` away from being the
next field's problem. `scrollFormBody` spends the window's last row on `N above ·
N below`, in the words Peek's position row already uses, and a body that fits is
returned untouched — the common case loses no row to a scroll it never performs.
The error banner stays chrome rather than body for the same reason the focus
drives the window: `handleSave` seats a refusal on the offending field, so the
banner and the field it names are read together or not at all. What still reaches
`clampFormBody` is only what no focus indexes — an inline overlay, and the
expanded form's own body as the last defensive cut — and the help column is cut to the body's own
height by `clampFormHelpPane` before the two are joined, so reference text beside
the field can never spend the position row that says where the reader is.

Width was the other half of the same defect, and it failed harder: the form
refused to draw at all below 68 columns, showing the same "terminal too small"
notice the app shows below 60, because every tab renderer had been written
against `chrome.DefaultFormFieldFactory().TitleWidth`. That 68 was never a
requirement — it was one field factory's preference, read by four files as a
floor. `formMinWidth` and `formMinHeight` are now `chrome.MinTerminalWidth` and
`chrome.MinTerminalHeight-2`, the app's own floor and the rows the shell allots
the form, so the form needs no more room than the surfaces it is opened from,
and `formBodyPreferredWidth` keeps the 68 as a ceiling: `View` computes one
`bodyWidth` of `min(formBodyPreferredWidth, m.width)` and hands it to whichever
tab renderer runs. At 68 columns and above the body is laid out exactly as it
always was, which is why this change moved no committed wide-terminal contract;
below it, every control shortens instead of spilling past the frame.

`task_form_widths.go` is the one place a preferred width meets the columns
chrome actually spends, and its constants are measured rather than guessed: a
bubbles `textinput` renders `SetWidth(n)+3` columns, `chrome.RenderOverlayBox`
spends six columns and four rows outside its inner width, a
`RenderFormLabelWithWidth` marker takes two, and
`theme.SelectorOptionSelectedStyle` adds two more to any value it renders.
`formValueWidth` subtracts the label column, the marker, and that selection pad
from the body; `formInputWidth` fits a preferred input width into what is left;
`formOverlayInnerWidth` is what every nested picker and inline editor calls
instead of restating the frame arithmetic, with `formOverlayTitleMinInner`
keeping a title from being the thing that overflows. The selection pad is
charged whether or not the row is focused, deliberately: paying it only when
focused would make a row change length as the focus lands on it.

`renderFormSectionHeader` and `renderFormItemRow` are the four collapsible
sections' one shape, so a fifth inherits the fitting rather than restating it.
Where a row cannot have everything, the value wins: `formFitHint` drops whole
`" · "` entries from the right and returns nothing when nothing fits, and a
section header gives up its trailing rule down to `formSectionTrailRuleMin`
before a single key is shed. That trade is safe because the keys a hint names
are also in the tab's guidance footer, while the value is the row's whole point.
An item value longer than the body is truncated on the row and recovered with
`Enter`, which opens it in its own editor at full length — the counterpart a
truncation owes.

Nested overlays took the last of it. Five pickers and four inline editors open
inside the form, and at the floor their frames were taller than the rows left
between the tab bar and the footer, so the priority picker lost its bottom
border at 60 and the state, due, and check-in overlays had been losing theirs at
68 without a test watching. `renderOverlayView` gives an open overlay the form's
whole allotted area instead: hidden, not shrunk, the same choice the Outliner
menu bar makes. It costs nothing, because an overlay owns every key while it is
open, so the tab bar and the footer underneath it were naming routes that were
not live.

`;comment;` sits at parse-order position 0, ahead of the attachment entry, and it
is there for a stronger version of the same reason. A comment is free prose that
may legitimately contain any other sigil, and RE2 cannot express "not inside a
`;…;` span", so the only way `Fix login ;check the #auth flow;` can file one
comment mentioning `#auth` rather than a comment plus a tag is for the comment
span to be peeled before every other pattern runs. The pattern
`(?:^|\s);(\S[^;]*);` carries the rest of the rule: the leading boundary keeps
`Fix a; then b` literal, the required closing delimiter keeps an unterminated
`Ship ;note` literal, and `\S[^;]*` makes an empty or blank span unmatchable, so
`validateAny` never sees a comment the application layer would have to reject.
Teach order is unaffected — `guideOrder` places `FieldComment` last, after
`FieldAttachment`, each in its own group.

Comments reach the application layer as raw strings on `CaptureRequest.Comments`
and `AddTaskRequest.Comments`. `resolveCapturedComments` runs every one through
the same `normalizeCommentText` an interactively added comment goes through,
before the snapshot in `buildAddTask` and inside `prepareCapturedTask` on the
capture path, so a rejected comment aborts with nothing mutated.
`applyCapturedComments` then calls `Task.AddCommentAt` with the service clock's
instant. Unlike an attachment it stages no side effect at all: a comment is task
document content, so it commits with the task write itself and needs no
`AttachmentOp`, and the single ChangeSet that creates the task already covers it.
Its refusals mirror the attachment's exactly — `parseOutlineBodies` and
`refusedSearchFields` — with wording that names the Comments panel, and so does
the one refusal that lifted: `ExpandSingle` carries a comment span into the
expanded form now that `ExpandRequest` seeds `taskFormDraft.comments` from
`CaptureRequest.Comments`. `Context` then followed the same pair rule once the
Basics tab grew its row: `ExpandRequest` seeds both `taskFormDraft.context` and
`contextInput` — the input as well as the draft, since `commitAllTextInputs`
reads the live input and an unseeded one would erase the value at save time —
and the refusal went with it. `ExpandSingle` is left with exactly two, `ChildIDs`
and `Pinned`, each still a value the form has no channel for.

`<child` is the one repeatable structural sigil. `>parent` stays
`CardinalitySingleton` because a task has one parent, while `FieldChild` is
`CardinalityRepeatable`; that declaration is the entire mechanism, since
`duplicateSingletonDiagnostics` already skips a non-singleton field. Both sigils
name a task by exact short ID or unique case-insensitive title prefix through the
one `ResolveStructuralReference` matcher, which differs between them only in the
relationship word its ambiguity message carries. A task named as both `>parent`
and `<child` on one line is refused, and across a multiline capture two lines
claiming the same child are refused before any mutation, with the message naming
both the failing line and the first claiming one. Aliases resolving to the same
stable task ID collapse to the first occurrence, in the parser and again in the
application layer, and both must stay first-occurrence or the preview and the
commit will disagree about order.

Adoption keeps the flat `p.Tasks` order of every pre-existing task:
`ApplyParentAssignments` never touches the slice, and
`ReorderRootTasksByCompletion` is a stable partition that moves a task only when
its root crosses the completed/uncompleted boundary. That hook deliberately does
not gather each root's descendants into one contiguous run, because gathering
resequences tasks the mutation never named — adopting a task that sits before an
unrelated root would otherwise push it past that root. `FieldChild` declares no
`Clears`, which is what makes `<0` an ordinary unresolvable reference rather than
a detach: a task leaves its parent through `>parent` or `>0` and nowhere else.
`resolveChild` reserves that token before any lookup runs, so a title beginning
with a zero can never make `<0` succeed on either surface.

The terminal-adoption refusal is one policy stated the same way on both surfaces.
`captureDestinationChildrenGuard` and `taskLineDestinationChildrenGuard` each run
after the hierarchy patch lands, so `$done` and `$canceled` are judged against the
children the mutation produces, and both raise the same `ChildrenBlockedError`
the ordinary lifecycle guard raises. Reconciliation still owns the one case no
destination names: an already-terminal task that adopts an open child is reopened
to Doing. Inline edit refuses a child locator outside the destination project
with `invalid child: child must belong to the effective project`.

`~task` is the soft-reference sigil, and it is the one repeatable reference that
crosses a project boundary: `grammar.ResolveDependencyReference` accepts a bare alias,
resolved in the effective project like `>parent` and `<child`, or a
`project-slug:alias` pair resolved against the loaded population. It is peeled
before `FieldTag` for the attachment entry's reason — `#([\w][\w.:-]*)` stops at
the tilde in `Task #work~foo`, and peeling the tag first would leave a
whitespace-preceded `~` that minted a reference out of prose — and
`FieldRemoveDependency` (`-~`) is peeled before it so the two-rune trigger comes
off whole rather than depending on the leading-boundary rule alone.

The three forms divide by surface because they divide by what a surface holds.
Capture takes `~task` only: a task being created has no stored set, so `-~task`
and `~0` are refused there by name. Inline edit and the outline buffer take all
three. Search takes `~task` and `-~task` and compiles them into
`admitsDependencies`, an AND gate over the stored list and an AND of nots beside
it: the negation resolves its locator through the same
`grammar.ResolveDependencyReference` its positive twin uses, so an ambiguous or unknown
alias is reported identically, and the resolved target then subtracts the tasks
the positive form would have selected. `~0` is still named in
`placementDiagnostics` and `refusedSearchFields` rather than compiled into a
locator that could never match, because a query holds no stored set to clear. The
`~` row picker is the fourth surface and it holds a stored set, so it speaks all
three forms without spelling any of them: checking an unstored row is `~task`,
unchecking a stored one is `-~task`, and unchecking every one of them is what
`~0` says.

`DependencyPatch` is what carries the intent into `Service.EditTaskLine`, and
its zero value means *no intent* rather than an empty set: `prepared.dependenciesSet`
gates the write, so a save that never mentioned a reference preserves the stored
list byte for byte. That rule is what makes every other save path safe — the
external editor returns a title and a body and nothing else, and the expanded
task form is create-only and gains no reference control — so neither can erase a
set it never displayed. The outline buffer is the exception that proves the rule:
it dumps every stored reference, so its absence *is* an intent there, and
`TestOutlineDependencyRoundTripSurvivesUnchanged` pins the round trip.

`outlineDependencyAlias` picks the shortest form that still addresses the
target — bare short ID inside the dumped project, slug-qualified in another
loaded one, durable `project-id:task-id` otherwise — and the parser accepts the
locator form back unchanged, which is how a reference to archived or deleted
work survives a round trip the workspace cannot resolve.

`Ctrl+E` is the second surface that speaks the edit grammar. Its buffer's `# `
heading is a title line, so `handleTaskContentEdited` routes it through the same
`grammar.ParseQuickEdit` and `outliner.TaskLineEditRequest` the inline editor uses, then rides the
edited body on that request's `Description` field. One `Service.EditTaskLine`
call therefore commits title, sigils, structure, and body as one transaction and
one undo entry; a second call would record two. The title is compared against the
one written into the buffer and parsed only when it differs, so a literal sigil
character the user never touched cannot be consumed. Because the composer batches
its create-and-open beside a reload, the handler re-resolves the project and task
by ID before parsing: `grammar.ParseQuickEdit` compares task pointers by identity, and a
stale pointer would make the edited task look like a stranger.

A recognized value the grammar cannot honor refuses the whole save. Nothing is
written, and the handler returns an `EditTaskContentMsg` naming the same buffer
path, the rejected title, the refusal, and the description the user typed but has
not committed, which reopens the editor on the file still on disk. The refusal
is rendered into the buffer's single instruction comment; that comment stays one
physical line and any `-->` inside a quoted user value is neutralized, since
`parseTaskContent` consumes only the first comment line and would keep a second
one as description text. Saving the reopened buffer with the title unchanged is
the loop guard: it abandons the edit, reports the original refusal, and removes
the buffer.

### Outline editor

`O` is the one operation that leaves the TUI to collect its input. It dumps the
cursor's project as an indented buffer, hands it to `$EDITOR` through
`external.ResolveEditor`/`external.EditorCommand`/`external.CreateTempFileWithContent`,
and submits the parsed result to the single `ApplyOutlinePlan` use case, which
resolves one snapshot and commits one checked transaction like every other
mutation.

The dump is a complete model subtree of one project, not the rendered
projection: folds and filters are ignored, because indentation in the buffer is
parentage and a partial dump would reparent whatever it omitted. The same
reasoning refuses the operation in every mode that is not `OrderPolicyTree` —
Today, Next, Due, and Recent promote rows across section boundaries — and under
a visual selection, which names rows rather than a project.

Each dumped task carries a trailing `[[tt:<first 8 hex of task.ID>]]` marker,
asserted unique at dump time so a collision aborts before the editor opens. The
outline layer strips that marker before the remainder of the line reaches
`grammar.ParseQuickCapture` with `skipProject=true`, which is what keeps one capture
parser true for the whole product and makes a `>parent` or `<child` sigil an
error rather than a competing parentage channel. `FieldChild` is absent from
`SurfaceOutline`, so the rejection is what stops the sigil from surviving inside
an outline title. The buffer is declarative: the dump emits
current values, so an omitted sigil clears that value. A pinned task's line
therefore carries `*pin` and deleting that token is what unpins it; `*unpin`
does not exist here, because omission already says it. `=review` is the same
shape for the same reason. A line without a marker
creates a task, and a dumped task whose line disappeared is deleted.

Deletion is the one thing the buffer cannot decide on its own, because a file
authored minutes ago cannot present the counts a destructive action has to show.
The decision therefore moves to the return leg: the adapter reads the removals
off the project the buffer was dumped from, arms one `ConfirmDialog` naming the
tasks and the attachments they destroy, and only a confirmation reaches
`ApplyOutlinePlan`, as `DeleteTaskIDs` plus `ConfirmDeletions`. The use case
refuses any list that disagrees with the buffer's absences in either direction,
so what was shown and what is removed are provably the same set. Removal,
attachment deletion, planning-ref cleanup, and the edits beside them commit as
one transaction under the `Apply Outline` undo entry; a survivor whose parent
line vanished is re-parented by its own indentation rather than cascaded away.
Declining cancels the whole apply and keeps the temp file. Scope is the list of
IDs the dump walk actually printed, not `project.Tasks`, because a task with a
dangling `ParentID` is unreachable from any root and so never appears in the
buffer that would have to speak for it.

Emptiness follows from the same rule rather than being a syntax error: a buffer
with every line removed deletes every task in the scope, and it is refused only
when it also removes nothing, which is the truncated-file case. That ruling
belongs to `handleOutlineEdited` and to `ApplyOutlinePlan`, never to
`grammar.ParseOutlineBuffer`, because only the two of them know the scope the emptiness
is being read against.

Validation runs in two phases, both before any mutation. The outline phase
checks marker syntax, unknown and duplicate markers, indent multiples and jumps,
`models.MaxNestingDepth`, and children orphaned by a rejected parent; the line
phase validates each stripped remainder. Every problem is collected rather than
reported one at a time, and a rejection renders the line-numbered `grammar.LineError`
list the multiline composer already uses, on its own scrollable
`OutlineErrorsViewModel` surface because a status bar is one truncated line. The
temp file is kept on every refusal so the reported line numbers still resolve.

The project revision captured at dump time is re-checked on return. A revision
that moved aborts without committing and keeps the buffer, because stale user
intent is not storage-level divergence and the conflict-bundle machinery would
be answering a different question. That check is also what makes the deletion
counts safe to present: it is enforced inside `ApplyOutlinePlan` before a single
title is overwritten, so nothing can change between the prompt the user answered
and the write it authorized.
