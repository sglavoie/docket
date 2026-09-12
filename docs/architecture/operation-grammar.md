[← Architecture index](../architecture.md)

### One operation grammar

`OperationRegistry` stores stable operation descriptors: label, category,
applicability, target cardinality, risk, value provider, and capabilities.
`OperationExecutor` freezes and resolves typed targets, requests values, gates
confirmation, and invokes the bound handler.

Five surfaces hold entities, so five `InputStrategy` values exist, and
`surfaceOperationGates()` is the whole inventory: there is no sixth grammar and
no unlisted one. Each pairs a strategy with an ordered manifest of that
surface's production operations, and the two are the only inputs any projection
reads.

| Strategy | Manifest | Menu keeps |
| --- | --- | --- |
| `SimplifiedInputStrategy` | `taskOperationManifest` + `taskDirectOperationManifest` + `outlinerProjectOperationManifest` | Delete alone on a task row; Edit, Archive, Delete on a project header |
| `ProjectHubInputStrategy` | `ProjectOperationManifest` | Archive or Unarchive, bulk tags |
| `NotesInputStrategy` | `NotesOperationManifest` | Delete |
| `AttachmentsInputStrategy` | `attachmentOperationManifest` | three origin acts, Unlink, Delete |
| `CommentsInputStrategy` | `commentOperationManifest` | Delete |

A manifest entry is a single record — ID, label, aliases, value provider,
category, risk, bulk eligibility — so descriptor metadata, action-sheet order,
and grouping all read one place and no surface can describe an operation two
ways. `DirectOnly` is the field that splits it: a `DirectOnly` entry keeps its
descriptor, provider, and bulk coordinator but leaves the menu, because the
grammar already reaches it on press one, and every remaining entry is a menu row
with no direct key. `Core` marks the entries whose descriptor
`registerCoreInputOperations` owns, so the manifest ranks and groups them
without restating a label or a risk the core descriptor already carries.

The split lands the same way everywhere. What the surface is for stays direct;
what is destructive or rare goes behind `Space`, and a destructive row confirms
before any transaction begins. That is why the entity menus are so short: they
are the residue of the rule, not a curated list.

The Outliner is the one surface whose strategy reads more than one manifest,
because its rows name more than one kind of entity. `Space` resolves against the
row under the cursor — a checklist item, a project group header, or a task — and
`outlinerProjectOperationManifest` is the third answer. One-path is per-surface,
so an operation may legitimately be a direct key on one surface and a menu row
on another. Edit project used to be that example — `e` on the Project Hub, a menu
row here — and stopped being one when the Outliner's `e` learned to read the row
under the cursor: `e` edits the task, the checklist item, or the project the
cursor names, exactly one of the three descriptors applying at any cursor, so the
manifest entry became `DirectOnly` and left the sheet, which keeps Archive and
Delete. The narrowing that makes it safe is
`outlinerProjectOperationTargetsHeader`: `e` is a root key in the Outliner, so
the cursor is the only thing that says which project was meant, and
`ProjectOperationApplicable`'s sole-scope fallback — right for a Hub row that
already named its project — would otherwise edit the one scoped project from a
section header or an empty projection. Add project comment is the second
operation behind that same narrowing, for the same reason and on the same terms:
`C` is a root key too, and a quick comment that borrowed the sole scoped project
from a section header would write to a project the user never pointed at. Project comments is the third,
and the reason the set is a set rather than a special case: `;` is a root key on
the same terms, so it declares itself in that map instead of restating the
guard. `validateOutlinerOperationOnePath`
therefore reads the manifests concatenated, and each sheet projects only its own,
which is what keeps a project row out of the task menu and the reverse. Unarchive
project is absent from the Outliner manifest for a different reason than Open
project: an Outliner project header is drawn from `TaskWorkspaceProjects()`,
which is non-archived by definition, so applicability could never admit the row.

What `;` opens on a project header is the Comments panel itself, bound to a
second `CommentOwner` rather than to a second panel. The adapter is the whole
seam: the presentation, the grammar, the confirmations, and the three commits
are the task panel's, and everything a project comment does differently lives in
`projectCommentOwner`. Two things do. The entries are projected newest first,
which is the reverse of the canonical storage order, so the row a user opens the
panel for is the top one; and because presentation no longer agrees with
position, every mutation crosses the seam as a `CommentRef` carrying the stored
comment id and the text the panel read there. `internal/app` re-resolves both
against a fresh snapshot before it writes, so a comment that moved, changed, or
disappeared since the editor opened is refused by name instead of rewritten by
index, and an edit that is admitted rewrites `Text` alone — the UUID and
`CreatedAt` are what the merge reconciles by, so neither may move. The task
adapter keeps its positional contract unchanged, because `models.Comment` has no
identity to address.

The adapter holds a project id and a resolver rather than a project pointer. A
committed transaction replaces the shell's in-memory project objects, so an
owner that kept the pointer it was mounted with would draw a discarded copy and
comment on it; `Reload` re-resolves by identity and reports presence alone, which
is what lets an archived or parked owner stay readable while `internal/app`
refuses the write with a reason.

The project path does not route through `runBatch`, which is the wrapper the
other batch operations use and whose tuple has no place for a comment identity.
`ProjectCommentCoordinator.commit` constructs `BatchTransactionResultMsg`
directly and preserves the committed `CommentID`. That is what
`commentCommitCorrelation` reads: it takes the transaction and project ids from
either committed shape — `TransactionResultMsg` for a task comment, the batch
result for a project one — and the comment identity from the batch shape alone,
because it is the only one whose owner mints one. `adaptCommentCommit` then
stamps that identity onto `CommentAddedMsg`, and `selectAddedComment` seats the
cursor by it. Both halves are load-bearing: without the project id the shell's
post-commit reload would name no project and re-read the state the commit had
just replaced, and without the identity the panel would have to seat the cursor
by position or by novelty on a list it re-projects after the commit — a list
that holds every other writer's additions too, so a successful local add would
aim the next read, copy, edit, and delete at somebody else's comment.

Footers do not follow the strategy everywhere, and the difference is
deliberate. The Outliner and the Project Hub rank their bindings and project the
footer through `GuidanceHintsForInputStrategy`, because a key there means one
thing whenever the list has a cursor. Notes, Attachments, and Comments cannot:
their footers have to narrow with a rename, an add, an edit, an expanded
comment, a confirmation, and — on Attachments — whether the selected row is a
URL or a code reference. Only the gated keymap beside those strategies can say
that, so it is their single footer source and their bindings carry no ranks or
labels at all. A footer has one source per surface, never two ranked
descriptions that can drift apart, and
`TestPanelFootersOnlyAdvertiseKeysTheStrategyBinds` holds the three panels to
it: every key their footer advertises is one their strategy binds, except the
three modal keys a comment composer owns while its textarea has focus.

`?` help meets the same limit from the other side. Core descriptors carry
`operation.CoreOutlinerApplicability`, so the registry half of a surface's help drops the
cursor keys, both ends of the list, `u`, `Ctrl+R`, and `?` itself, and only the
surface's own keymap can name them. That keymap binds one physical key per row,
which is why the composed spellings its strategy declares needed a projection of
their own: `keymap.WithHelpKey` renames the help row the way `WithFooterKey`
renames the compact hint, leaving `Binding.Key` authoritative for dispatch and
`HandledKeys`. `gg/Home` and `G/End` are therefore one row apiece on the Project
Hub, Notes, Comments, and Attachments — the chord, its alternate key, and the
hidden `end` alias documented once rather than four times or not at all — and
`TestEntitySurfaceHelpDocumentsBothEndSpellings` holds all four surfaces to it.

A `?` section is the category its own projection assigned, not a second
classification laid over it. `helpSectionsForCategories`
(`internal/tui/help/help_render.go`) is the one ordering rule the body and the tab bar
both read: the canonical `helpSectionOrder` first, then any category that list
does not name in first-seen order, then `Other`. A projection that mints a new
category therefore gets a section for free and can never be silently swallowed.

Two things reach `?` that the registry does not own. `HelpItem.Aliases` carries
`OperationDescriptor.Aliases` through, so `/` filtering matches vocabulary a
label hides — `defer` reaches **Set Due or Defer**, `hidden` reaches **Filter by
Due or Deferred** — and the row appends the aliases that earned it as a muted
suffix, dropped whole rather than clipped when the description column has no
room. Aliases feed matching only: their positions index into the alias, not into
the rendered columns. `GrammarHelpItems()` (`internal/tui/help/help_grammar.go`) then
adds the **Capture & Edit Grammar** section from
`textgrammar.GuideDefinitions` over the union of the capture, edit, and search
surfaces, making the sigil language discoverable outside the composer's own `?`.
It is the second `GuideDefinitions` consumer and teaches in the same order as the
first. `AppModel.coreInputHelpItems` appends it, because `?` is a core input
operation and that function is the Outliner's one help projection.

A value provider is exactly one of `operation.TextEntry()` (a separate overlay),
`operation.TagEntry()` (inline add mode), `operation.DateEntry()` (inline add mode), or a plain list
such as `ValueProviderParent`.
Inline modes bind `a` inside `OperationValuePickerModel` and must emit the same
value type the provider's preset rows carry, so the executor and the single and
bulk handlers stay untouched. A value the presets cannot express belongs in an
inline mode on the one picker, never in a standalone form.

`OperationTextPromptModel` is that overlay, and `OperationTextPromptOptions` is
where the providers that share it differ: the character bound, whether an empty
buffer is an answer or a refusal, whether the buffer is multiline, and what an
`Esc` on a non-empty buffer asks before discarding it. Multiline is the one
option that moves a key. A comment's stored text holds newlines, so `Enter` has
to write one and the apply moves to `Ctrl+S`; the prompt reads its own submit key
in exactly two places, the key switch and the guidance hints, so the footer
cannot advertise a key the handler does not take. The discard question is drawn
inside the prompt's own box rather than as a second floating overlay, because
`AppModel` paints the confirmation the controller owns and a box over this one
would hide the text the question is about. Two properties are the prompt's own
rather than the shell's: it emits one selection per open and answers every later
submit key with nothing, and `Restore` is what reopens it — on the text already
typed, with the reason the commit was refused — which is also what clears that
guard for the retry.

A terminal bracketed paste reaches that buffer through its own arm.
`tea.PasteMsg` is one message rather than a run of key presses, so
`handleActiveKeyOverlay` never sees it; `AppModel.handleActivePasteOverlay`
is the paste-side twin, and the text prompt is the only one of the five overlays
it names, because it is the only one that collects typed text. Everything the
prompt's key path enforces the paste path enforces too: the widget's own
`CharLimit` cuts an oversized paste to the same bound a typed one stops at, an
armed discard swallows the paste rather than rewriting the text it is asking
about, and a paste never submits — only the submit key does.

The vocabulary rename `e` opens on `ValueProviderFilterTags` and
`ValueProviderFilterContexts` is a third nested grammar on that same model
(`internal/tui/valuepicker/vocabulary_rename.go`), and `vocabularyRenameProvider`
is the whole set: those two facets are the only lists whose rows are stored task
names. Its three stages paint as the picker's body rather than as overlays above
it, which is why the browse state underneath is preserved by construction — the
list is neither drawn nor rebuilt while a stage is open, so the query, cursor,
and pending multi-select are simply never touched. The nested grammar runs after
the inline add modes and before the browse switch, so an open name step takes
every printable key as literal input exactly as a search does.

The picker owns the steps and none of the effects: it holds no service, no
workspace, and no frozen preview. Every sentence it shows is computed by
`internal/tui/vocabulary_rename_host.go` and handed back through
`ShowVocabularyRenamePlan`, `RefuseVocabularyRename`, and
`AdoptVocabularyRenameOutcome`, over the three messages the picker emits.
`vocabularyRenameAxis` is the one place a provider is bound to the application
namespace it renames in, the `models.QueryField` a saved lens spells it with, and
the row-ID prefix, sigil, and noun its wording is written in. The shell holds one
`vocabularyRenameSession` — a frozen `app.RenameTaskNamePreview` plus the lens
impacts computed from the same census — and releases it on every exit from the
confirmation, accepted or not, because a preview pins a whole workspace census.
An expired preview surfaces as `storage.ErrStaleRevision`/`ErrStaleArtifact` and
is named as recoverable; every other refusal is unwrapped to
`app.InvalidInputError.Reason`, since the picker is the field the refusal names.
`vocabularyRenameHeading` repeats the four titles the undo ledger writes rather
than importing them, because the ledger's are unexported and a drift would be
visible in the status line the commit hands back through `setRecoveryFeedback`.

Nothing rewrites a lens. `vocabularyRenameAffectedClauses` partitions each saved
query through the read path a lens already has and reports a clause when it would
admit exactly one of the two names, which is what makes a full name and a prefix
equally answerable; polarity never enters that test, because a negation owns no
vocabulary and `QueryClause.String` already renders the leading `-`. After a
commit the shell rebuilds the picker from the renamed workspace and carries the
browse state across, remapping the pending source row onto the destination and
deduplicating what a merge collapsed; only the picker-local search is given up,
and only when it would hide the row the rename produced.

The lens report is one row per lens and therefore unbounded, so both the
confirmation and the outcome window it. `VocabularyRenamePlan` carries that cut:
`Summary` is the bounded prose the confirmation is answerable for — destination,
counts, scope, the merge sentence, and the lens count — and is pinned, while
`Lines` scrolls under it. The confirmation's window is height-aware, budgeted
from the height `RenderWithin` is given less `vocabularyRenameChromeRows`, which
reserves the guidance's declared two-row budget rather than its packed height so
the body is measurable before the scroll hint is asked for. The post-commit
report keeps its fixed `vocabularyRenameReportLines` window: nothing is being
decided from it. `renderVocabularyRenameWindow` records the line count and the
window height because the render is the only place either is known, which is
what lets a scroll key clamp and a resize reseat an offset the new window cannot
hold. The scroll keys are the whole of what they do — the two answers stay
required in the footer, and no step commits, cancels, or edits from one.

`ValueProviderDate` is the one date picker a task operation reaches. It sections
due presets above defer presets so a single key answers both fields, and the
committed row carries `OperationDateValue.Deferred`, which is what tells the
single and bulk handlers which field to write. Because the two sections share
preset names, `dateListItemID` namespaces every row by its field.
`ValueProviderDueDate` is the due-only projection of the same presets, built
from the same namespace so one resolver answers either list.

The cursor's row is the active field, and it is the only thing that is. Reading
which field a row writes and deciding whether that row may be committed are two
separate questions, answered by two functions in
`operation_date_picker_sections.go`: `dateRowField` unwraps the row's own
`OperationDateValue` and reports `Deferred` for any row that carries one, while
`dateSectionUnderCursor` adds the activation gate and returns
`ErrListItemDisabled` with the row's stated reason. Navigation and guidance ask
the first, `a` and Enter the second, which is what lets a terminal task's
disabled defer rows stay reachable and self-explaining while still refusing to
be written. No second section index is kept: there is nothing for the cursor to
drift out of step with.

Narrowing the inline mode is therefore a cursor question rather than a provider
question, which is why `instantEntry()` cannot be a `ValueProviderID` method:
only a defer row builds the adder with `NewInstantDateAdder`, whose `allowsTime`
is what makes `+Nh` parse there and be refused by name where a calendar day is
stored. The same flag
passes `deferred` to `operationDatePresets`, which is what puts the three hour
rows and the morning row on the defer section alone. A terminal task carries
`deferRefusalReason` on its defer rows only, so the existing
`ErrListItemDisabled` path refuses the field on the row it belongs to instead of
hiding it, and the due rows beside it stay reachable. A preset is reported as the current
value only when the stored instant is reproducible from the clock, which is why
the hour rows never highlight: they inherit `time.Now()`'s monotonic reading,
while the day and morning rows are built with `time.Date` and compare cleanly
against a value parsed from disk. The morning row names 09:00 in the target
calendar day's location rather than nine hours after that day's midnight, so it
stays 09:00 across both daylight-saving transitions instead of drifting to 10:00
or 08:00.

Which row the combined picker opens on is the due field's business alone.
`operation_value_picker_factory.go` reads `CommonTaskDate` for due only, and
`OperationDateSelectedID` maps that to a due preset or, failing one, to
`DateListItemID(false, "clear")`. The cursor can sit in one section, that
section is always Due, so deferral agreement can neither move the highlight nor
withhold it: a selection sharing a deadline but not a hide-until still opens on
the deadline it shares, and an absent, mixed, or non-preset due date opens on
Due's Clear row, which is where the list would open anyway. The highlight is a
starting point, not a selection — opening writes nothing and leaves
`SelectedIDs` empty.

`Tab` and `Shift+Tab` cross between the sections. `switchDateSection` walks
`Matches()` for the first row whose `dateRowField` disagrees with the cursor's,
so the switch lands on a row the current search left on screen, disabled rows
included. Nothing to land on leaves cursor and query untouched and returns a
status naming the section through `DateSectionLabel`, the one authority for
both field names, which the section headers and the `a` hint also spell. The
browse `switch key` block consumes both keys on every picker, so a picker with
one section is inert rather than leaking `Tab` down to the Outliner's mode key;
search editing and the typed-date input claim their keys first, so neither
switches.

The preset shortcuts are one table in `chrome/date_preset_keys.go`, keyed by
field and stable preset ID, and every reading of a key goes through it:
`DatePresetKey` prints the hint, `DatePresetIDForKey` dispatches it, and
`DatePresetShortcutKey` reports that a key belongs to either field. `chrome` is
where it lives because the value picker and the creation form's own due picker
both sit above that layer and must not each own a keyboard. The form keeps its
order, its labels, and its day arithmetic and borrows the keys alone, so the two
surfaces cannot drift apart on what `m` means without a red test.

Rows carry their key in `ListItem.PrimaryMarker`, which the surface draws before
the primary text and charges width for separately, and which query and ranking
are already defined to skip — a hint is chrome, and typing `m` into the search
box must not match every row bound to `m`. An unkeyed row pads the same width so
the labels stay in one column.

`handleDatePresetShortcut` is the dispatch, gated on `provider.DateEntry()` and
placed after the search and typed-date handling and before the browse `switch
key` block rather than inside its default arm. The position is load-bearing:
`case "w", "s", "t", "c"` above that default already claims two of the due keys
for semantic-tag category jumps, so a default-arm shortcut would never see `t`
or `w`. The handler claims any key bound in *either* field, so a key the active
field leaves unbound is structurally silent instead of falling through to a
binding that means something else. It resolves the cursor's field with
`dateFieldUnderCursor`, maps key to preset ID, moves with `SetCursorToID` — which
searches the current matches alone, so a filtered-out preset is a miss and gets a
status naming it — and then commits through `SelectedValue` and
`OperationValueSelectedMsg` with the original request, which is the same path
Enter takes. A disabled row therefore refuses with `SelectedValue`'s own
`ErrListItemDisabled` reason and the picker stays open. Because the move lands on
the row and the row's `OperationDateValue` was built when the list was, a
shortcut commits the payload the row already held rather than recomputing a date
at keypress time.

The switch and the active field are ordinary gated `keymap` entries, not a
second chrome path: `switch field` is an optional Navigation hint at rank 3, and
`dateEntryNoun` writes `set due date` or `set defer date` into the existing `a`
hint. `preset key pick` joins them at rank 6, saying only that the keys are live,
because which key is which is printed on the rows. Because that can grow the footer, `RenderWithin` renders the guidance
before budgeting and reserves `8 + lipgloss.Height(guidanceText)` rows rather
than a fixed nine, so a wrapped footer costs a list row instead of a terminal
row, and the string it measured is the one it passes to `RenderOverlayBox`.
`fitRequiredGuidanceSegments` still protects Enter and Esc, so the optional hint
can only ever be the one dropped.

`-` and `+` reach the same due field without opening anything. They are two
registered operations, `tui.task.step-due-earlier` and
`tui.task.step-due-later`, because one descriptor carrying a direction would have
to branch its own label, and the footer and `?` help read that label as it
stands. Neither declares a `Provider` — a step has no value to ask for — and
neither takes a `FooterRank`: `@` already spends rank 3 on the date axis, and the
menu bar and the generated help project from the registry regardless of rank.
The keys are spelled as the characters a terminal produces. A keyboard without
the enhanced protocols reports `+` as `+`; one with them reports the unshifted
code with the shifted text beside it, and `Key.String()` prefers that text, so
one binding answers both. A second `shift+=` spelling would be dead vocabulary
that also shadows `=`, which is Toggle Review.

`DueStepQueue` in `internal/tui/coordinator` is the serialization those keys
need. The shell freezes each keypress into a `DueStepGesture` — direction, the
project-qualified locators the cursor or the visual selection named, and the
hidden-member count only the surface that froze the selection can know — and
enqueues it. Exactly one gesture is in flight; the rest wait in arrival order.
What the gesture deliberately does not carry is the deadline or the remembered
clocks: `dispatchDueStepGesture` reads `RememberedClocks` when it dispatches, so
the second press of a burst computes from the state the first press committed
rather than from the row that was on screen when it was typed. Each gesture is
its own `app.TaskOperationStepDueDate` transaction and its own undo entry; a
burst is never coalesced, so four presses are four `u` presses back.

A result returns inside `DueStepResultMsg`, stamped with the sequence the queue
armed. The stamp is what distinguishes a late delivery from the current one. On
a commit the successor is taken out of the queue first, the ordinary
`handleBatchTransactionResult` reload and relocation run, and only then is the
successor armed — otherwise that reload's own supersede check would cancel the
chain the queue is draining. On a refusal the whole pending sequence is
abandoned and counted, because the tail was frozen against a premise the
application layer has just rejected; `DueStepRefusal` wraps the reason rather
than restating it. Navigation retargets nothing, and a result is delivered
wherever the user has gone, help overlay included.

`supersedeDueStepsForEffect` is the one detach. It reads
`CommittedDueEffect.Edits` and `Removed` and never `Steps`, so an explicit due
edit, an outline apply, or a removal drops the queue while the queue's own
commits do not; `observeRestoredDueClocks` detaches on a ledger undo or redo for
the same reason. A detached gesture still commits and still reloads — the
transaction happened — but what it may still say about clock memory is narrowed
on the way through.

Those are two invalidations, not one. Pending work goes whole whatever caused the
detach, because a queued step is computed from a committed state that has just
moved. Committed clock evidence is narrowed per target, by the
`DueStepSupersession` the queue recorded against that gesture's sequence:
`SupersedeTargets` carries the tasks an edit or a removal actually named, and
`Superseded(sequence).Retain` drops those tasks' outcomes out of
`DueEffect.Steps` and keeps the rest. A deadline typed on one task is no
statement about the wall clock another task's step just cleared, so that task
still hands its clock back when the user steps out of unset. `Supersede` is the
form for a mover that names nothing — a ledger restore puts back whatever the
restored transaction wrote — and it blanks the effect whole, as does a delivery
the queue has no record of. Only `Steps` is narrowed: `Edits`, `Removed`, and
`Moves` forget or re-key a clock and can never resurrect one. A gesture detached
more than once before its result arrives accumulates every supersession it
outlived.

Supersession is about what a transaction may say. Reconciliation is the other
direction: what the workspace says back. `reconcileRememberedDueClocks` walks the
tasks memory still speaks about, not the rows on screen, and drops the clock of
every one the shell can currently read that now carries a deadline — whoever gave
it one. Three callers reach it, and they are the three moments the shell's
picture of the workspace changes. `observeCommittedDueEffect` runs it after
folding a commit in, `observeRestoredDueClocks` before applying a ledger delta,
and `adoptCrossProjectSnapshot` after rewiring an accepted population into the
project cache. Only the third can see a deadline no local effect ever named, so
it is the one that answers for another machine's write; without it the deadline
renders and the clock it replaced survives underneath, ready to be spent by the
next `+`.

That third call sits on the adoption rather than on either message that reaches
it. `CrossProjectsLoadedMsg` and a published `LensProjectsLoadedMsg` both adopt
through the same function, so one call covers both and a future population path
inherits it. Hanging it off the adoption is also what keeps acceptance and
observation the same question: every caller is already gated on the snapshot
being current, so a superseded load — which installs nothing — retires nothing
either, and the population still on its way remains the one that speaks for the
workspace. A target the shell cannot resolve, parked or archived or simply not
loaded, says nothing about its own deadline and keeps what was remembered for it.

What the strip says is the outcome, not the gesture. The ledger entry's context
is `+1 day`, which is what the undo browser should show and no answer at all to
the press, so `BatchTransactionResultMsg.RecoveryContext` carries
`DueStepSummary` — one task's landed deadline, or the changed and cleared counts
with the hidden members named — and `setRecoveryFeedbackWithContext` prints it in
the ledger context's place. The feedback controller records it beside the
fallback text, so the re-render delayed origin feedback performs keeps saying the
outcome instead of putting the gesture back. Every other caller passes no
override and reads exactly as before.

One operation may chain into a second one by following its committed command with
an `OperationValueRequestMsg`, which is how `ww` asks for the check-back date it
has just made relevant. `withWaitingCheckBackStep` runs the set-state command,
and only a committed result — the `transactionCommitted` gate — is followed
through `tea.Sequence` by the value request for
`operation.OperationTaskSetDueDate`. That
request names `ValueProviderDueDate` rather than the combined list `@` opens,
because the step is asking for a deadline and a defer section would answer a
different question. A
rejected move returns its `TransactionErrorMsg` untouched, so a task that never
reached Waiting can never be given a check-back date. The chain keys off
`invocation.Value` rather than runtime context, and it sits on the one shared
`applicationTaskOperationRequest` path, so single and bulk moves chain
identically and the second step targets the same tasks even though the transition
moved the cursor. Because the first transaction has already committed,
cancelling the second step cancels the date alone and leaves the Waiting move
standing.

`g` is a chord prefix, not a direct key: cursor-top is `gg` or `Home`. Its second
keys jump straight to one mode, so a seventh mode never costs six `Tab` presses.
The digits `1`–`7` are those same seven jumps at their `Tab`-cycle positions, and
they are a second `Keys` spelling on the go-to bindings rather than seven new
descriptors: one operation, one handler, and one composed `DisplayKey` such as
`gb/5`, which `?` help and the footer read as it stands instead of composing a
second one. `gg`/`Home` and the seven mode jumps are therefore where a single
binding declares both `Keys` and a `Sequence`; prefix-freeness is still fully
enforced by the chord trie, which is also what keeps a bare digit clear of the
`!` and `f!` chords that end in one.

`0` leads that digit row and is a separate operation,
`tui.core.reset-to-launch-view`, because it names no mode and so cannot be a
spelling on one of the seven. Its handler composes the axis calls the individual
keys already use — the lens frame, `ClearSearch`, the focus pops,
`ExitSectionZoom`, `ClearProjectScope`, the two filter families, the two sort
axes, and finally `selectOutlinerMode(ModeAll)`, which owns the single rebuild —
the way `peelQueryReadoutAxis` does, so the reset can only reach a view the
ordinary grammar can. It does not reuse `ResetToGlobalAll`, for two reasons.
That reset clears the machine-local active-lens pointer, and `0` must not: the
lens is put down for this session and opens again at the next launch, which is
why the pointer write is split out and `forgetActiveLensFrame` drops the frame
alone. And that reset never touches the `F` family, whose facets decide which
projects load at all; `0` clears them, so it takes the population reload through
the same command `Fx` raises. Every axis is read before any of them is written,
because the unwinding calls restore captured frames that reinstate one another's
state, and that read is what lets the strip name the axes it cleared or answer
`Already at the launch view`.

`w`, `z`, `f`, `F`, and `o` are the other root prefixes. `w` is the workflow
chord, so no lifecycle destination carries a bare root key and `d` is unbound
rather than aliasing `wd`; `z` is the scroll chord and never means defer. Every
other retired root key — `m`, `$`, `:`, the comma state
prefix, and `Ctrl+Esc` — is simply absent from the strategy, because a
compatibility shim would keep a second vocabulary alive next to the sigils.
`t`, `P`, `c`, `C`, `b`, and `R` left that list by an explicit decision rather
than by drift: `t` took the task-subtree fold, `P` took Select Project, `c` took
Add Checklist Item, `C` took quick Add Comment, `b` took Peek, and `R` took Mark
Reviewed, so all six are live keys with new acts rather than aliases for the ones
they used to name.

Set Parent's key is its sigil too: `>` opens `ValueProviderParent`, a plain list
built by `operationParentListSource` from the cursor task's own
`StructuralCandidates`, above one `No parent (project root)` row carrying the nil
parent `>0` names. Each candidate row carries `InvalidQuickEditParent` as its
`DisabledReason`, so a self-reference, a task inside the edited subtree, and a
parent that would breach `models.MaxNestingDepth` are visible refusals in the
list rather than errors behind `Enter` — the same three sentences the `>parent`
sigil produces, read from the same function. The chosen value reaches
`TaskItemCoordinator.Reparent`, which is the existing `Service.Reparent`
path, so `>` adds a key and a picker but no second reparent route. Indent and
outdent moved to `]` and `[` to free the sigil; they remain one-step tree moves,
not a parent choice.

`<` is the mirror: it opens `ValueProviderChild`, a multi-select plain list built
by `operationChildListSource`, and adopts the checked tasks. It is a separate
operation from Set Parent because `<child` is repeatable and additive, which a
single-valued replace cannot express. Two things make the additive contract
structural rather than a convention. Nothing is preselected, so an unchecked row
is the resting state and never reads as a removal; and the tasks the cursor task
already parents are filtered out of the source entirely, so no selection state in
the picker can ask for a detach — `FieldChild` declares no `Clears`, and `<0` is
an unresolvable reference, not the inverse of `>0`. Pressing `Enter` with nothing
checked therefore sends no children, which `Service.EditTaskLine` reports as an
unchanged no-op.

The checked IDs become `TaskLineEditRequest.Children` with the title carried
verbatim and the parent patch left on `TaskLineParentPreserve`, so the request
travels the same `applyTaskLineEditEdges` path the `<child` sigil does and lands
as one `ApplyParentAssignments` batch: cycles and depth are measured on the graph
the adoption produces, and the inherited `taskLineDestinationChildrenGuard` still
refuses a terminal destination holding an open adopted child. `TaskLineEditRequest`
carries an optional `Action` for the undo ledger, defaulted to `Edit Task Line`,
which the picker sets to `Adopt Children` so the entry names what the user did
rather than the request type that carried it.

`%` completes the set: it opens `ValueProviderProject` and moves the cursor
task's subtree to the chosen project. The list is built by
`operationProjectListSource` from `Runtime.AvailableProjects()` minus the task's
own project, so a parked destination stays reachable exactly as it is for Notes
and the `%project` sigil, while a move to the current owner — which is not a
move — has no row to press. The adapted rows carry the project ID as their
payload, which is what `TaskLineEditRequest.DestinationProjectID` names, and
their secondary text is the project slug rather than a task count: these are
catalog records whose task trees are not loaded in the picker, so a count would
report every destination as empty. `EditTaskLine` sees a destination that
differs from the source and takes `editTaskLineAcrossProjects`, so the subtree
lands through `moveTaskSubtreeBetweenProjects` with destination-minted ShortIDs
and both projects commit in one checked transaction. The command therefore
builds `coordinator.StructuralPlanningArtifacts` rather than the reconcile
builder Adopt Children uses, because planning references have to be requalified
to the new owner rather than merely reconciled, and it sets `Action` to `Move to
Project` for the ledger. `P` keeps Select Project: that operation changes the
projection, not a task's owner, so the two carry separate keys, labels, and
footer entries.

`~` is the fourth of the set and the only one that preselects. It opens
`ValueProviderTaskReference`, a multi-select plain list built by
`operationTaskReferenceListSource`, and `taskReferenceMembership` arrives as the
preselection: `operationValueAll` for a reference every target stores,
`operationValueSome` for one only part of them stores, and `operationValueNone`
for the rest. That
asymmetry with Adopt Children is a grammar fact rather than a taste. `FieldChild`
declares no `Clears`, so an unchecked child row could not be read as a detach and
the `<` picker is built so that no selection state can ask for one; `~` declares
both `~0` and `-~task`, so unchecking a preselected row already has a spelling in
the text grammar and means removal. The same fact is what makes the row identity
`taskReferenceRowID` — the durable `project-id:task-id` locator behind a
`reference:` prefix — load-bearing: a stored reference and the candidate row
offering that same target collapse to one row, which is what lets the checked set
be a diff instead of two lists.

The population is two halves and the stored half is never filtered. Every stored
reference gets an enabled row, terminal and unreadable targets included, because
a set editable only by hand is how a reference gets silently stranded; a target
the picker cannot read keeps its locator as its `PrimaryText` and carries
`DependencyDetailUnavailable`, the same word Peek uses. The second half is
`grammar.DependencyAdditionCandidates` — every open task in every loaded non-archived
project, minus the source and minus the stored half — which is exactly the
population an addition may name, so the picker needs no inline add-by-locator
mode. Rows group into one category per project in `DependencyDetailProjects()`
order, because a reference crosses project boundaries and a filtered list has to
stay qualified.

Over a selection both halves are the union across it: the stored half lists every
reference anybody carries, and the addition half withholds every selected task,
because naming one would be a relationship inside the selection whose direction
the gesture cannot state. That is the operation's one refusal — `bulkPatchDependencies`
names the offending task (`#7 Ship the API` is in the selection) and commits
nothing.

The commit is a difference rather than a set, and that is the one thing about
this picker that is not obvious. The overlay is not modal against the workspace:
an accepted cross-project load republishes task trees through
`adoptCrossProjectSnapshot` while `~` is still open, so the stored set the picker
preselected can be stale by the time Enter arrives. `selectedValue` therefore
emits `OperationTaskReferenceValue` — the rows checked that
`storedTaskReferenceBaseline` had not preselected as `Add`, the preselected rows
unchecked as `Remove` — and says nothing at all about a reference the user never
touched. `taskReferencePatch` then merges that intent into what the task stores
at execute time, read fresh from the workspace: an addition already present is
dropped and a removal with no target is dropped, so each half is idempotent
against the fresh state. A reference that appeared under the open picker is
therefore not spelled as a removal, and one that vanished under it is not spelled
as an addition — emitting the whole checked set would have done both.

`Clear` stays false even when nothing is checked: the application layer refuses
`Clear` beside `Add` or `Remove`, and removing every stored reference says the
same thing. Add and Remove cannot overlap by construction, so that refusal is
unreachable from this surface.

The tri-state membership is what lets one picker be `Bulk`. The standing
objection — N tasks carry N different stored sets, so a checked list would assert
one of them over all of them — is an objection to a *replacement* picker, and
`TagPatch`/`operationValueMembership` already answered it for tags. A row nobody
moved keeps its `Some` reading and is emitted by neither half, so the tasks that
disagree keep disagreeing; only a row driven all the way to `All` or `None`
becomes an addition or a removal. Cardinality then chooses the transaction and
not the meaning, exactly as `%` does: `applicationTaskOperationRequest` returns
unhandled below two targets so a selection-free `~` stays byte-for-byte the
task-line edit it always was, and two or more take
`app.TaskOperationSetDependencies` with `taskReferenceBulkPatch` as its
`DependencyPatch`. `bulkPatchDependencies` runs on `bulkAttributeOverSnapshot`,
which resolves the targets, plans once against the very snapshot the commit will
write, and only then mutates: each target gets its own `referenceResolver`, so a
self-reference and an unresolvable locator are judged per task while every one of
them reads one revision-consistent view. A target the patch leaves unchanged
commits without moving its `UpdatedAt`. The
patch becomes `TaskLineEditRequest.Dependencies` on one request with the title
carried verbatim and the parent left on `TaskLineParentPreserve`, so both halves
of the diff resolve against one snapshot and land in one transaction with one
undo entry, with `Action` set to `Set Dependencies` for the ledger; the paired
add and remove services would be two transactions of each, neither seeing the
other's half. A patch empty on both sides issues no command at all and reports
`Dependencies unchanged`, because an empty transaction would still write a ledger
entry `u` could take back — which now also covers a picker whose every edit the
fresh state already reflects. Dependents are computed from what other tasks store,
so they are never listed and never editable — `~` is outgoing only.

Pin's key is its sigil: `*` on an existing row does what `*pin` does in the
capture, inline-edit, and outline grammars. That is not a direct-only/menu-only
violation, because that rule governs which task operations reach the `Space`
menu, not whether an attribute is also expressible in the text grammar: `!`, `@`,
`#`, and the workflow chord are the sigils that say the same things.

Every keyed capture sigil but one binds the character it is written with: `!` as
a chord prefix, `#`, `@`, `*`, `=`, `%`, `>`, `<`, `~`, `&`, and `;` as root keys.
`$destination` is the exception, and the only one: its chord is `w`, because ten
lifecycle and planning destinations are the most-pressed chord of the daily loop
and an unshifted letter is worth more there than the correspondence is.
`grammarSigilKeyboardSpelling` names that one rename as data, and the rename
costs `$` its root key and `f$` its facet — the states facet follows the chord to
`fw` so one axis keeps one keyboard spelling.
`&` and `;` are the only pair that opens a panel rather than a picker, because the
panel already owns add, reorder, unlink, and delete.
`TestSigilKeysMatchTextGrammarSigils` walks the `textgrammar` triggers against
the strategy's own bindings, so renaming a sigil breaks the keymap loudly rather
than silently splitting the two vocabularies. Four triggers deliberately have no
key of their own. `=` is not one of them: review is a stored flag beside the pin,
so the character that writes it in a capture line is the key that toggles it on a
row, and the `f=` chord narrows by the same flag without competing for the root
key. `$review` stays a refused destination all the same, because a destination is
a singleton and review coexists with a lifecycle state.
`##` and `@@` cannot be chords under `#` and `@`, because the
chord trie is prefix-free and both first characters already resolve; each is
instead a mode inside the picker its first character opens — `##` the context
radio group inside Set Tags & Context, `@@` the defer section of the one date
picker. `-#` is a search-only exclusion form, and two runes are not one key
event, so the `#` picker expresses it by unchecking a tag. `-~` is that same
case one sigil over: it is an edit-surface removal form, and unchecking a
preselected row in the `~` picker is how one key event says it. Indent and outdent are
the reverse case: `]` and `[` are keys that answer no sigil, because a relative
one-step move has no capture form to name.

`SimplifiedInputStrategy` is the only Outliner key grammar. Direct dispatch,
handled-key metadata, chord hints, guidance hints, `?` help, and harness discovery
all project from that strategy. The task operation manifest separately holds the
one `Space` action:

1. Delete

`taskDirectOperationManifest` holds Comments and Attachments as `DirectOnly`
entries instead, bound to `;` and `&`. The one-path rule is what forces the
choice: a panel a key opens cannot also be a menu row, so the single-task sheet
became identical to the bulk sheet and stopped rendering a Content group.

Project moves and reparenting are expressed in the inline `e` editor with
`%project`, `>parent`/`>0`, and the repeatable `<child`, so structural edits
share the same atomic task-line transaction as title and metadata changes. The
edited task's own parent edge and every adopted child edge are submitted as one
`models.ApplyParentAssignments` batch, which is what lets a hierarchy inversion
commit: cycles and depth are measured on the graph the edit produces rather than
on the one it replaced. Adoption is additive and reaches only the effective
project, so unmentioned children keep their parent, an adopted task keeps its
subtree, and no reference can move a subtree across a project boundary. Capture
and inline edit share one terminal-adoption policy: each judges a `$done` or
`$canceled` destination after its hierarchy patch lands and refuses it while an
adopted child is still open, so neither surface reaches the completed-above-open
shape the post-mutation reconciliation would have to undo.

The checklist is absent because its rows own the whole mutation set; a menu
entry would be a second route to it.

`AttachmentsInputStrategy` is the third peer grammar and splits the same way.
`Enter`, `a`, `e`, `y`, `Y`, and `J`/`K` stay direct because they are what the
panel is for; `attachmentActionSheetOperations` orders the five the panel keeps
behind `Space`, hosted as `ContextActionHostAttachments`:

1. Reveal original
2. Open original
3. Open attachments folder
4. Unlink
5. Delete

The three origin acts are rare enough that a direct key spent them for nothing,
and Unlink and Delete are why the sheet exists: an attachment now leaves the list
only through a deliberate menu row and the panel's own confirmation, which the
binding delegates back to the panel that owns the cursor. The retirement that
matters most is `u`, which meant unlink here while meaning undo everywhere else;
`d` and `x` go the same way, `o` folds into `Enter`, and `O`, `Ctrl+E`, and
`Ctrl+O` become sheet rows, so the panel no longer answers any of the seven.
`validateAttachmentOperationOnePath` holds them to the same direct-or-menu rule.

`CommentsInputStrategy` is the fourth peer grammar and splits the same way.
`Enter`, `a`, `e`, `E`, `y`, and `Y` stay direct because they are what the panel
is for; `commentActionSheetOperations` holds Delete alone behind `Space`, hosted
as `ContextActionHostComments`. The panel has no multi-select, so that sheet is
one comment's action list rather than a bulk menu, and its binding delegates the
confirmation back to the panel that owns the cursor. The retirements are the
hidden `n` add alias and the `d`/`x` delete pair, which were the same house-style
aliases the Outliner and Notes already gave up; nothing named them, so the panel
simply stops answering them. `u` and `Ctrl+R` join the Attachments panel in
reaching the one workspace ledger through `WorkspaceUndoMsg`: every comment
mutation commits with an undo action and installs a `u undo` toast, so the
surface that advertises the key has to answer it.
`validateCommentOperationOnePath` holds them to the same direct-or-menu rule.

`registerSurfaceInputStrategies` walks `surfaceOperationGates()` and runs each
surface's check before registering its strategy, so a surface that drifted fails
at model construction rather than at the key press that would have found it. The
five named wrappers each delegate to one `operation.ValidateOperationOnePath`, which reads
the strategy's own bindings and rejects an entry that is both bound and listed,
or marked `DirectOnly` with no binding left to justify it. Keeping the wrappers
named per surface is what puts the surface's ID in the error text; the rule
itself exists once. The checklist row's own menu is outside every manifest and
deliberately lists Edit Item, which also answers `e`, because the menu is the
only route to Delete Item.

A chord prefix that its second key abandons is explained rather than swallowed.
`ChordGroupLabelForPrefix` names the longest declared group the path passed
through, and the miss sets a warning status such as `wq is not a workflow
action`. An unbound root key stays silent because it never showed a
which-key row, and a key the cursor's entity turned off keeps its own target
rejection sentence instead. A chord prefix whose whole subtree that entity turned
off refuses at the prefix, so a project header row never opens a which-key row
that could not complete.

A modifier or lock key pressed by itself is not a keystroke and never reaches the
grammar. `isModifierOnlyKeyPress` drops it in the one switch where key events
enter the app, beside the key-release swallow, so the Shift a chord's own second
key requires — `f#`, `f!`, `f~`, `F#`, `o@`, `g@` — cannot resolve as that second
key, and no view has to know the terminal reports it. The app requests no
keyboard enhancement beyond the disambiguation Bubble Tea already negotiates:
a richer Kitty flag set is what made a terminal report those presses at all, and
nothing may request one behind the renderer's back.
