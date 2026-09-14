# Behavioral signals contracts

These decisions lock the shared presentation and filtering contracts for the
behavioral signals work. Later changes may implement them, but must not choose
different keys, thresholds, precedence, wording, or drop positions.

## Task-row indicators

`indicatorDropLadder` is the narrow-row pressure order: position 1 disappears
first and position 10 disappears last. The final order declared as
`indicatorDropLadder` in `internal/tui/outliner/outliner_render_task_row.go` is:

| Position | Segment | Reason |
| ---: | --- | --- |
| 1 | `deferCount` | It is a supporting count, and Peek repeats it on the existing dates line. |
| 2 | `checklist` | The row badge and Peek still carry the count. |
| 3 | `comments` | The row badge and Peek still carry the count. |
| 4 | `attachments` | The row badge and Peek still carry the count. |
| 5 | `recency` | Recent mode and ordering already convey the same activity signal. |
| 6 | `deferred` | The task remains absent from ordinary work projections while its deferral is pending. |
| 7 | `waiting` | Lifecycle state still names Waiting after its age disappears. |
| 8 | `due` | A date is more operationally specific than the preceding supporting signals. |
| 9 | `doing` | It names work that has remained in Doing, in the same quiet-work class as check-back. |
| 10 | `check` | It names work already quiet on someone else's desk and remains the last survivor. |

This preserves the existing rationale documented with `indicatorDropLadder` in
`internal/tui/outliner/outliner_render_task_row.go`: counts leave early while the most
operational quiet-work signal leaves last. The draw order may group related
segments, but pressure must follow the table above.

The Doing-age text is `doing <age>`, reads `StateChangedAt`, and stays neutral:
it does not participate in `chrome.PickHeroAttribute` at
`internal/tui/chrome/hero_attribute.go`. Adding Doing as a competing urgency
badge would contradict the existing `chrome.HeroKind` and
`chrome.PickHeroAttribute` hierarchy in `internal/tui/chrome/hero_attribute.go`,
and that lock is unchanged.

The candidate list itself has since been consolidated. It is overdue, review,
due-today, and priority-1, in that precedence, and Waiting is no longer among
them: a lifecycle state says what a row is rather than how loudly it is asking,
so an ordinary Waiting row is quiet and a Waiting task earns a hero only on the
same terms as any other task. `review` is the one addition, and it is what
`models.TaskNeedsAttention` answers — the stored review flag or a check-in whose
day has arrived. The state badge is consequently never demoted: it is identity,
so it keeps its own colors on every row and is neither hero nor hero-suppressed.

The row tint follows the same rule. `theme.OverdueRowStyle` is the only one:
repainting a whole row is the loudest mark the list has, so it is spent on the
top of the ladder alone. Waiting held a second tint until this consolidation,
which made the quietest work in the workspace look like the most urgent.

The repeated-deferral signal starts at the third deferral. For
`DeferCount >= 3`, its row segment is `deferred ×N`; smaller values render no
count. Peek adds the same `deferred ×N` token to its existing dates line
rather than consuming a new line in the five-line Peek ladder. This gives the
early-dropping row segment a durable detail surface without displacing another
Peek line. Repeated deferral is an independent behavioral signal: hiding the
`defer` column hides only the pending defer date or recent wake marker, while
the row and Peek still show counts at or above the shared threshold.

## Stale work

### Facet key

The lowercase row-refinement chord is `fs` ("filter stale"). Inspection of
`filterOperationDescriptors` in `internal/tui/filterops/filter_operations.go` and
`(*HelpViewModel).filterItems` in `internal/tui/help/help_filter.go` confirms that
filter descriptors and the help view do not maintain a second key table: help
fuzzy-filters the registry projection. The lowercase `f` keyspace this decision
was taken against is bound by `SimplifiedInputStrategy` in
`internal/tui/operation/input_strategy_simplified.go`. The rows below carry
their present keys: the
locked decision here is that Stale's key is `fs`, and this table is only the
evidence that `fs` was free, so a later renumber inside it leaves the decision
untouched. Task contexts subsequently took `fc` as an axis of their own, moving
completion age to `fa` and leaving `f#` to tags alone.

| Chord | Meaning |
| --- | --- |
| `f!0`, `f!1`, `f!2`, `f!3` | priority threshold: clear, high, medium+, low+ |
| `f@` | due date |
| `fa` | completion age |
| `fw` | workflow states (this row read `f$` when the decision was taken) |
| `f#` | tags |
| `fc` | task contexts |
| `f=` | Review flag (this row read `fr` when the decision was taken) |
| `fh` | hierarchy position |
| `fx` | clear all task filters |

`fs` is therefore an unused terminal below the existing `f` prefix. It must be
registered as a chord in `SimplifiedInputStrategy`, not as a `DirectKey`.
`(*OperationRegistry).Register` in `internal/tui/operation/operation_registry.go` checks
unconditional direct-key collisions; like every existing filter descriptor
created by `filterOperationDescriptor` in `internal/tui/filterops/filter_operations.go`,
the stale descriptor has an empty `DirectKey` and an applicability predicate,
so it does not claim a static direct key. Chord ownership is instead rejected by
`(*inputChordNode).insert` in `internal/tui/operation/input_strategy_trie.go`,
where `fs` has no existing owner. The `validateActiveOperationShortcuts` pass in
`internal/tui/operation/operation_registry_validation.go` likewise sees no
direct-key claim.

### Predicate, lane, and precedence

Stale means open work whose `UpdatedAt` touch time is at least 90 local calendar
days old. The threshold is hardcoded as `90`, and the boundary is inclusive:
age 89 is not stale; age 90 is stale. Compare calendar days, not elapsed
24-hour durations. This follows Recent's hardcoded local-day precedent at
`recentModeCutoffDays` and `BuildRecentModeSections` in
`internal/tui/outliner/outliner_rows.go` while intentionally using `UpdatedAt`,
not `StateChangedAt`.

The base stale predicate excludes terminal and currently deferred tasks. It may
match a Doing task; lane precedence, rather than the age predicate, prevents
duplication. The `fs` facet applies this base predicate as a lowercase row
refinement in every compatible mode.

In All mode, Stale draws immediately after Review and before Inbox. Its claim
loses to every existing stronger claim: Pinned, Today, Next, Review, and Doing.
A task satisfying any of those remains in that section and never duplicates in
Stale. This extends the Review rule at
`(*OutlinerModel).buildReviewSection` in
`internal/tui/outliner/outliner_sections.go`: a deliberate planning claim or
active-work state outranks a derived age claim. Existing relative precedence
among those five sections is unchanged. The insertion point binds
`(*OutlinerModel).buildAllPrimaryRows` in
`internal/tui/outliner/outliner_projection.go`.

## Today pace readout

The Today-only middle-region wording is:

```text
today <open-count> · median <completions-per-day>/day (30d)
```

For example: `today 9 · median 3/day (30d)`. `today` names the current Today
load; `median` explicitly marks the outside-view statistic; and `(30d)` names
the observation window. When there is no completion history, render no pace
readout rather than a zero median. The readout belongs to the interaction/middle
region, never inside `WorkspacePulse`; this binds the region model at
`OperationalTitleSpec` in `internal/tui/pulse/operational_title.go`.

Under width pressure, preserve identity and required interaction context, then
use these pace candidates in order:

1. `today 9 · median 3/day (30d)`
2. `today 9 · median 3/day`
3. `today 9 · 3/day`
4. omit the whole pace readout

Never render `today 9` alone: without the reference-class rate it is no longer
the contracted comparison and duplicates pulse information. On Today, the pace
candidate outranks the passive workspace pulse; exhaust the pulse ladder before
shortening the pace candidate, then omit pace before truncating required
interaction context. This extends, rather than reverses, the existing rule at
`RenderOperationalTitle` in `internal/tui/pulse/operational_title.go` that passive
title state disappears before required context.

## Workspace pulse

The full component order is fixed as:

```text
workspace: inbox 3 · today 2 · overdue 1 · done 4
```

`done` is the number completed on the current local day and renders even when
zero; it is a progress count, not another deficit count. Component order binds
`WorkspacePulse` and `formatWorkspacePulseComponents` in
`internal/tui/pulse/workspace_pulse.go`.

The four-step pressure ladder, first removed to last removed, is:

1. `done`
2. `inbox`
3. `overdue`
4. `today`

Thus the new retrospective component never displaces live operational state,
and the three existing components retain their current relative drop order.
Today remains the final survivor. Replace the current boolean pair with this
explicit ladder and have `operationalPulseVariants` in
`internal/tui/pulse/operational_title.go` iterate its candidates.

## Vocabulary check

The user-facing terms fixed here are `doing`, `deferred`, `stale`, `today`,
`median`, `pace`, `workspace`, `inbox`, `overdue`, and `done`. None appears in
the retired product-concept map in `TestCurrentProductDocsExcludeRetiredConcepts`
at the repository-root `retired_product_vocabulary_test.go`, which is in
`package main` rather than under `internal/tui`. Future documentation and
scenario names
must keep these spellings or choose another term that also remains outside that
map.
