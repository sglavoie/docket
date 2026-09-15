[← Architecture index](../architecture.md)

#### Row indicators

A task row promotes three operational signals and no more: the deadline, an
owed review, and the outgoing-reference census. Everything a row used to
abbreviate beside them — the tag chips, the context badge, the Waiting and Doing
ages, the deferral, the checklist tally, the comment and attachment counts, the
recency chip — is now read in Peek, at full value rather than in a fixed-width
badge. The trade is deliberate and one-directional: a row is scanned to decide
what to do next, so it carries required interaction context and spends every
other column on the title, while Peek is the complete document one `b` away.
Nothing was deleted from the product; each signal has exactly one surface now
instead of a truncated copy and a whole one.

A deferral is therefore state-dependent prose on the Peek dates line rather than
a row segment. `inlinePeekDatesLine` names `defer <date>` while
`IsDeferredAt(now)` holds, then `woke <age>` for `DeferWakeWindow` (72h, defined
in `internal/tui/query/primary_view_shared.go`), then nothing.
`taskWokeRecently` is the one lapsed-deferral predicate and `FormatRecencyAge`
the one age token, so the two halves cannot be formatted two ways.
The workspace pulse deliberately carries no `woke` component: lapsed deferrals
name work that requires navigation before action. Its fourth component is instead
the current local day's Done count, a non-deficit progress signal, and the ladder
in `formatWorkspacePulseComponents` drops Done, Inbox, overdue, then Today.

`chrome.FormatDeferDate` renders the pending half of that ladder against `now`,
because
a deferral is an instant and Peek has to say when the task comes back. It
resolves the value into `now`'s zone first — instants read from disk keep the
fixed-offset zone `parseProjectDocumentInstant` preserves for byte-stable
revisions — and then reads one clock for all three decisions. A midnight-valued
deferral is a whole day and keeps the bare `Jan 2` it has always had; one
returning later today is `15:04`, since the day is already on screen everywhere
else; anything further out is `Jan 2 15:04`, with a `'06` suffix off-year. The
midnight test is `value.Equal(util.StartOfDay(value))` rather than a call into
`operationDateSecondaryText`, which shares only that predicate and disagrees on
every other token.

A deferral that lapses while the Outliner is open has to bring its row back
without a key press, and it does so on the existing 60-second `status.PeriodicSaveMsg`
tick rather than a second scheduler. Each row rebuild stores `NextDeferWake`,
the `earliestDeferWake` min-scan over the flat `project.Tasks` of the scoped
projects, which already contains every nested child, so no tree walk is needed.
`wakeDeferredRows` compares that clock against the tick and, once it is past,
clears it and calls `m.population.invalidate()` and nothing else, leaving the render pass
Bubble Tea runs next as the single rebuild authority and
`ComputeOutlinerViewport` as sole owner of `ViewScrollOffset`.
`safeToRebuildForWake` refuses while an overlay, picker, inline row editor,
visual selection, or the flat ranked list is live, because rebuilding underneath
one of those moves ground the user is standing on. A query that is merely
narrowing is not such a state: it is re-answered on every rebuild, so the woken
task is matched against it like any other row. A refusal leaves the clock
standing so the next tick retries; the wake is deferred, never dropped, at a
cost of at most one tick. A consumed wake also recomputes the workspace pulse at
the tick's own timestamp, because `ComputeWorkspacePulse` excluded that task
while its deferral was pending: the counts and the row it makes visible move in
the same update. A refused wake refreshes nothing within the day, so the clock
and the pulse stay in step. Across local midnight the cached completion activity
is stale and has to be requeried anyway, so `refreshWorkspacePulseAcrossLocalDay`
runs the same refresh and then restores Inbox, Today, and Overdue whenever
`deferWakeOwedAt` still reports a lapsed clock: the new day's `Done` advances
while those three counters wait for the rebuild that makes their rows visible.

Waiting and Doing ages read the state clock, never the touch time, and they are
one value on one surface: `inlinePeekStateValue` appends
`FormatRecencyAge(task.StateChangedAt, now)` to the state for those two states
alone, so Peek says `waiting 12d` or `doing now` and the row says neither. A
comment, a tag, or a reparent on a blocked task therefore cannot reset how long
it has been blocked, and the state badge on the row carries the fact while the
age carries the duration. A Waiting deadline remains the ordinary
`FormatDueDate` segment on the row; check-ins are a separate schedule and never
reinterpret or suppress it.

`formatReviewIndicator` spells `models.TaskNeedsAttention` and nothing else, so
the row and `chrome.PickHeroAttribute` read one predicate and a row can never
demote a badge for a review it does not name. It has three tiers: a bare `review`
for the stored flag alone, which has no day to be late against and stays until
`R` clears it, then `review today` and `review +<age>` once
`Task.CheckInIsDueAt` says the schedule has arrived and is eligible. Future
schedules stay in Peek, while deferral and terminal state suppress the row
signal. Waiting and Doing ages remain muted beside it. The periodic tick compares
the projection's frozen local day with the current one and safely invalidates
rows, `?review`, Review counts, and Peek; the refresh remains owed during an
overlay, picker, editor, visual selection, or ranked result, and the eventual
rebuild restores the cursor by qualified task identity.

Every due reading is a background badge rather than colored text, so a date
reads as one bounded object in a row whose title is neutral and whose other
signals are badges of their own. `ClassifyDueDate`'s three tiers and
`formatReviewIndicator`'s three share the `makeDueBadgeStyle` family, which holds the section header's
contrast contract — a tone-aware fill under a high-contrast foreground. A tier
passes that constructor one `dueBadgeTone`: the fill and the ink that reads on
it, once per theme, plus whether the tier is emphasized. The overdue and today
tiers are, and emphasis is a pair — bold, and ink pushed away from the fill
rather than toward it, which is why the light fills are pale teals under
near-black ink rather than the saturated mid-tones white text needed. Upcoming
declares neither, so a deadline that has arrived reads as weight while one the
user still has time for reads as tone. `TestDueBadgeInkContrastIsTieredInBothThemes`
holds the emphasized pair to the enhanced ratio and the quiet tier strictly
below both, and `TestEmphasizedDueBadgeInkFollowsThePaper` holds the direction so
a repaint cannot invert a tier. The fills are
curated dark/light pairs rather than palette derivations, so a palette override
cannot mint an unreadable badge. They live in the teal band because the twelve
`ViewMode` section fills already claim every other hue, and a chip that borrowed
one read as a section header instead of a date; `TestDueBadgeFillsAvoidSectionBadges`
holds the two populations apart, and urgency travels on saturation within the
one hue so lightness stays in the band section badges already occupy. The chip's
two `Padding(0, 1)` cells need
no width formula of their own: `outlinerNonTitleWidth` sums `lipgloss.Width`
over the already-styled segments, so the title budget shrinks by exactly two
like any other segment growth.

The attention ladder is `chrome.PickHeroAttribute`, and it is the row's one
authority on how loudly a task is asking: overdue, then review, then due-today,
then priority-1, with a terminal task answering `HeroNone` whatever it carries.
`HeroReview` reads `models.TaskNeedsAttention`, the same predicate
`formatReviewIndicator` spells, so the ladder and the row agree by construction.
A hero the priority badge did not win demotes that badge through
`chrome.DemoteBadge`, which is what leaves one saturated badge per row;
`chrome.PriorityIsHero` is the one projection of `HeroKind` the row renderer
needs. The state badge is outside the ladder in both directions: it is identity
rather than urgency, so it is never a hero and never demoted, which is what
leaves an ordinary Waiting row readable without making it look urgent. Waiting
earns a hero only on the terms any other task does — its own deadline, review, or
priority.

`theme.TaskTitleStyleFor` is the single title-style authority and answers
lifecycle alone: muted or disabled, canceled, cursor-selected, otherwise ordinary
text. No signal in the ladder reaches a title. `theme.SemanticTaskTitleStyle`
survives as the six-name vocabulary's reserved accent with the `SEMANTIC` picker
rows as its one consumer; its `...Title` palette keys are historical names kept
because the palette is a published surface.

Repeated deferral starts at `DeferCount >= 3` and is Peek's alone:
`inlinePeekDatesLine` appends `deferred ×N`, so it opens no section of its own in
the Peek body and costs the row nothing at any width. `taskHasRepeatedDeferrals`
stays the one threshold predicate.

A task holding soft references renders a trailing `~S/N` from
`renderDependencyIndicator`. It is deliberately ungated by a `ColumnConfig`
column: naming one would put a dependency back in the column vocabulary the
retired hard-dependency panels owned, and the count is not a fact a user hides.
`N` is over stored locators rather than resolvable ones, so a reference to
terminal, deleted, or unloaded work counts exactly like a reference to open
work, and the same number is what the edit grammar round-trips.

`S` and the color are the reading of the targets, and `DependencyCensusFor`
computes both in one pass so they can never disagree. It sorts every stored
locator into `settled`, `blocked`, and `unreadable` over `app.DependencyIndex`,
whose population is what `DependencyDetailProjects` returns — the same one Peek
resolves its detail lines against, so the row and Peek report the same
arithmetic. The census is strictly one hop: a target's own references are never
walked, because a transitive reading would assert reachability through projects
the view never loaded, would smear one unreadable locator over a whole path, and
would answer "everything reaches everything" inside a legal cycle.

`dependencyCensus.counts()` writes `S/N`, or the bare `N` when the two agree,
and `renderDependencyIndicator` appends `?` when `unreadable > 0`, which is what
carries under `--no-color` the one thing the muted tier used to say alone.
`dependencyCensus.outlook()` reduces the same numbers to a tier and
`dependencyOutlook.style()` maps it to a foreground:

| Tier | Reading | Style |
| --- | --- | --- |
| `dependencyOutlookBlocked` | at least one resolvable target is nonterminal | `DependencyIndicatorBlockedStyle` |
| `dependencyOutlookSettled` | every reference resolves to a terminal task, or to a task absent from a project this view did load | `DependencyIndicatorSettledStyle` |
| `DependencyOutlookUnreadable` | nothing readable is blocked, but a locator points into a project this view did not load | `MutedStyle` |

Precedence runs blocked over unreadable over settled: a row with something
left to wait on says so rather than hiding behind what it could not resolve. The
unreadable tier stays muted rather than green for the same reason
`DependencyDetailUnavailable` says `unavailable` and not `missing` — painting an
unloaded project's task as finished would assert a fact the view cannot read.
The two named styles exist so retuning a due-date or status color never silently
repaints a reference count, and both are rebuilt in `rebuildTaskStyles`. Neither
`blocked` nor `blocking` is ever rendered bare: the row shows numbers, and the
chrome that names the words spells them `?blocked` and `?blocking`, so a reader
cannot mistake either for the Waiting lifecycle state.

The index is built once per row-rebuild pass, not once per reference per row per
frame, which is strictly cheaper than the linear scan it replaced.
`OutlinerModel.dependencyIndex` builds it lazily over `DependencyDetailProjects`
and holds it until `invalidateDependencyIndex` drops it, which `setLoadedProjects`
and the `m.population.invalidate()` path both do. Invalidation is by replacement,
never by patching. The older note here argued a memo would risk a stale color when
a target in another project changed state without a row rebuild; it cannot,
because changing a target's state is a mutation and a mutation invalidates the
population.
`quickEditOptionalMetadata` builds its own unstyled `~N`: that is a grammar echo
of the edit buffer, not a row indicator, and it is deliberately left uncolored.

`indicatorDropLadder` is the row's drop order, and it stays a list rather than a
set even now that it holds three entries, because the order is the contract and
an append is not a position. The reference census goes first: Peek names every
target in full, and no deadline or owed review depends on it. The owed review
follows, because it says a task wants looking at while the deadline says when
something is actually due. The deadline goes last — it is the final commitment
width pressure may hide. A new indicator gets a position in that list, not an
append.

The ladder is short because the budget is: the seven entries it lost were not
demoted within it, they left the row for Peek. Adding one back means arguing that
it is required to decide what to do next from the list alone, not merely that it
is useful.

The search match cue sits above that ladder rather than in it. A projected row
whose hit came from a field the title cannot show carries one muted
`· description`, `· resume`, `· checklist`, `· comment`, `· tag`, `· context`,
`· attachment`, `· project`, or `· details` segment, mapped from
`outlinerRow.MatchSource` by `matchSourceLabels`; a title hit and every ordinary
row carry none. It is a `taskRowSegments` field, so `titleWidth` charges it once
like any other segment, and it is still the first thing a crowded row sheds,
ahead of every `indicatorDropLadder` entry, because an explanation is worth
nothing beside an unreadable title and because shedding it first leaves a
non-search row's degradation exactly as it was. It carries more now than it did:
`· tag` and `· context` used to explain a hit the row itself also showed, and
they are now the only thing on the row that says why a result is there.

The relation marker is not in that ladder and never drops.
`taskRelationMarker` writes both directions of the hidden-family relation into
one `taskRowSegments` field, so `titleWidth` subtracts its columns before the
title is cut instead of offering it whatever the title left over. Offering it the
remainder is what made the ancestor mark a 40-column path on a short row, a
`‹ Re…` stub on a medium one, and nothing at all on a long one — vanishing
exactly where a child row most needed distinguishing from a root row. Its
meaning is now the same at every width. One segment rather than two also keeps
the reserve near three columns on a row that is both a promoted child and a
detached parent.

Upward, a promoted section root carries `  ‹`: this row has a parent the outline
above it does not show. Which parent is Peek's line to name. Downward, a row
carries `  ›N` when `HasVisibleChildren` is false and the task has N open
children — the parent that entered Doing while its children stayed in Backlog,
whose row otherwise has no connector beneath it, no fold indicator, and reads
exactly like a leaf. It is counted because `›` alone leaves open the question it
raises, how much work hangs off the row; `‹` stays bare because a task has one
parent. Only `IncompleteChildren` count, since a parent whose children are all
terminal hides no open work.

The pin marker is the third reserved `taskRowSegments` field rather than a
ladder entry, for the same reason: a pin that disappears at 60 columns is a pin
the user cannot trust anywhere. `taskPinnedMarker` renders a bold `*` when
`task.Pinned && !row.IsPinnedSection`. The predicate is the row's lane, not the
mode's `ShowsPinned` flag, so the flat ranked list — which returns before any
Pinned section is built — and a filter-admitted terminal or deferred pinned row
inside All or Today all get the mark, while the lane's own rows stay bare
because the section header above them already states it.

A pinned *project* wears the same glyph, from the same `PinnedMarker()` helper,
on every surface that lists one: the Outliner project group header, the Select
Project rows, and the `%` Move to Project picker. One vocabulary, so `*` writes
what `*pin` names and what every list shows. It is a glyph rather than a color
because `SelectedLine` strips styling and re-renders, and `RenderListSurfaceRow`
skips `PrimaryStyle` on the cursor row outright — a style-only signal is
invisible on exactly the row the user is on, while a character survives both.
On the group header the marker leads the badge, so it lands in the column a
pinned task's `*` occupies one row below, and it goes on before
`appendProjectHeaderPressure` measures, so its two columns are inside that
budget and `today` then `overdue` shed first; the marker itself is never
dropped. Unlike the task row's it is not *reserved*: a header has no
`ColumnLayout`, so an unpinned header pays nothing for it. On the list surfaces
it rides `ListItem.PrimaryMarker`, pre-styled chrome drawn immediately before
the primary text and charged to `markerWidth` so `LayoutListText`'s budget stays
honest. Like `PrimaryStyle` it is presentation metadata the behavior model never
inspects — query and ranking read the plain `PrimaryText` — which is what keeps
a pinned row findable by its own name. An archived project keeps the marker,
because the pin is real state that survives archiving, but
`ProjectOperationRequirements` keeps `RequireActive` on the operation, so the
`* project pin` footer hint and the `?` help row correctly disappear there: the
marker shows the state, the key hides because the act is unavailable.
Select Project's preview pane keeps its `· pinned` line — prose for one project,
glyph for scanning a list.

The trigger is zero rendered children rather than a rendered-child count: when
some children are shown the connectors and fold indicator already say "parent",
and the arithmetic would have to account for
`excludeTaskTreePromotingChildren` seating a grandchild in a clone's `Children`.
For the same reason the count comes from `openChildCounts`, a per-rebuild census
of the authoritative flat `project.Tasks` assigned in `buildCanonicalModeRows`
above the flat ranked list's return, and never from `row.task.Children`: All,
Inbox, and Backlog rows carry filter-pipeline clones with rewritten children
while flat-mode rows carry authoritative pointers, so reading the row's own task
would give the marker two meanings depending on the mode.

#### Status flash

Changed status text flashes the footer divider: the bar takes a severity color
and runs a 500ms impact–sweep–settle animation over `footerFlashFrames` (20)
frames of `footerFlashFrameInterval` (25ms): 100ms of full-width `━`, an eased
heavy segment with a severity head and accent-fading trail, then a short dissolve
back to the thin `─` accent bar. The footer content rows simultaneously take a
14% severity-tinted background wash that eases back to the theme background over
the same 500ms; existing badge backgrounds remain intact. It is a flash, not a
pulse — `workspacePulse` is the title row's counts chip. The interactive TUI
ignores an inherited `NO_COLOR`, while the explicit `--no-color` flag still
disables color; the moving heavy glyph segment keeps the flash visible even then.
Non-interactive commands retain the inherited environment.

The trigger is status text only. The three `AppModel` setters advance
`feedback.seq`, the shell error beside it is compared by text because it has no
clock, and
`InteractiveViewState.setStatus` advances its own stamp; a breadcrumb, mode
badge, or which-key change moves on nearly every keystroke and must never arm it.
Cleared text arms nothing, because neither `clearStatus` nor the auto-clear
timers advance a sequence. Arming happens in exactly one place, the deferred
closure in `AppModel.Update`, which is what lets ~50 void setter call sites stay
untouched; it must remain that one defer, since a second would run first under
LIFO and see neither the recovery nor the assigned command. The footer divider
appearance feeds all three dividers `renderFooterChrome` can emit — its own
separator and both guidance calls — or the flash dies whenever a which-key row is
up. The status strip's divider is the fourth divider and deliberately not one of
them: the strip is view content, so it takes neither the moving glyph nor the
footer content wash. The breadcrumb keeps its plain foreground accent while its
background participates in the footer wash.

A view-owned status message is transient by default and expires like the
shell's own. `SetTransientStatus` marks it, `scheduleViewStatusExpiry` reads the
same before/after probe the flash arms on and batches a `ViewStatusClearMsg`
inside that one deferred closure, and the handler clears only when the message's
view and `StatusStamp` still match, so an older tick can never dismiss a newer
message. `setStatus` stays sticky for a live state descriptor such as the
multi-select summary, which would otherwise leave an active selection unlabeled.
Clearing arms no flash, since `clearStatus` advances no sequence. A mode
transition clears the view status outright rather than waiting for the tick,
because a message naming its mode misreports the next one.
