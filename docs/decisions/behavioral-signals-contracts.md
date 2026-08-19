# Behavioral signals contracts

These decisions lock the shared presentation and filtering contracts for the
behavioral signals work. Later changes may implement them, but must not choose
different keys, thresholds, precedence, wording, or drop positions.

## Task-row indicators

`indicatorDropLadder` is the narrow-row pressure order: position 1 disappears
first and position 10 disappears last. The final order bound to
`internal/tui/outliner_render_rows.go:496` is:

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

This preserves the existing rationale at
`internal/tui/outliner_render_rows.go:496-499`: counts leave early while the
most operational quiet-work signal leaves last. The draw order may group
related segments, but pressure must follow the table above.

The Doing-age text is `doing <age>`, reads `StateChangedAt`, and stays neutral:
it does not participate in `pickHeroAttribute` at
`internal/tui/hero_attribute.go:45`. Overdue, Waiting, due-today, and priority-1
remain the only hero candidates. Adding Doing as a fourth competing urgency
badge would contradict the existing one-hero hierarchy at
`internal/tui/hero_attribute.go:23-38`.

The repeated-deferral signal starts at the third deferral. For
`DeferCount >= 3`, its row segment is `deferred ×N`; smaller values render no
count. Peek adds the same `deferred ×N` token to its existing dates line
rather than consuming a new line in the five-line Peek ladder. This gives the
early-dropping row segment a durable detail surface without displacing another
Peek line.

## Stale work

### Facet key

The lowercase row-refinement chord is `fs` ("filter stale"). Inspection of
`internal/tui/filter_operations.go:12-60` and
`internal/tui/help_filter.go:62-110` confirms that filter descriptors and the
help view do not maintain a second key table: help fuzzy-filters the registry
projection. The current complete lowercase `f` keyspace is bound by
`internal/tui/input_strategy.go:319-329`:

| Chord | Meaning |
| --- | --- |
| `f!0`, `f!1`, `f!2`, `f!3` | priority threshold: clear, high, medium+, low+ |
| `f@` | due date |
| `fc` | completion age |
| `f$` | workflow states |
| `f#` | tags and task context |
| `fr` | Review tag |
| `fh` | hierarchy position |
| `fx` | clear all task filters |

`fs` is therefore an unused terminal below the existing `f` prefix. It must be
registered as a chord in `SimplifiedInputStrategy`, not as a `DirectKey`.
`OperationRegistry.Register` checks unconditional direct-key collisions at
`internal/tui/operation_registry.go:209-243`; like every existing filter
descriptor created at `internal/tui/filter_operations.go:30-46`, the stale
descriptor has an empty `DirectKey` and an applicability predicate, so it does
not claim a static direct key. Chord ownership is instead rejected on collision
by the input-strategy trie at `internal/tui/input_strategy.go:457-461`, where
`fs` has no existing owner. The active-context ownership pass at
`internal/tui/operation_registry.go:473-485` likewise sees no direct-key claim.

### Predicate, lane, and precedence

Stale means open work whose `UpdatedAt` touch time is at least 90 local calendar
days old. The threshold is hardcoded as `90`, and the boundary is inclusive:
age 89 is not stale; age 90 is stale. Compare calendar days, not elapsed
24-hour durations. This follows Recent's hardcoded local-day precedent at
`internal/tui/outliner_computed_rows.go:14-16` and
`internal/tui/outliner_computed_rows.go:169-202` while intentionally using
`UpdatedAt`, not `StateChangedAt`.

The base stale predicate excludes terminal and currently deferred tasks. It may
match a Doing task; lane precedence, rather than the age predicate, prevents
duplication. The `fs` facet applies this base predicate as a lowercase row
refinement in every compatible mode.

In All mode, Stale draws immediately after Review and before Inbox. Its claim
loses to every existing stronger claim: Pinned, Today, Next, Review, and Doing.
A task satisfying any of those remains in that section and never duplicates in
Stale. This extends the Review rule at
`internal/tui/outliner_review_section.go:14-30`: a deliberate planning claim or
active-work state outranks a derived age claim. Existing relative precedence
among those five sections is unchanged. The insertion point binds the All-mode
assembly at `internal/tui/outliner_primary_rows.go:116-163`.

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
`internal/tui/operational_title.go:16-30`.

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
`internal/tui/operational_title.go:27-67` that passive title state disappears
before required context.

## Workspace pulse

The full component order is fixed as:

```text
workspace: inbox 3 · today 2 · overdue 1 · done 4
```

`done` is the number completed on the current local day and renders even when
zero; it is a progress count, not another deficit count. Component order binds
`WorkspacePulse` and `formatWorkspacePulseComponents` at
`internal/tui/workspace_pulse.go:14-21` and
`internal/tui/workspace_pulse.go:100-115`.

The four-step pressure ladder, first removed to last removed, is:

1. `done`
2. `inbox`
3. `overdue`
4. `today`

Thus the new retrospective component never displaces live operational state,
and the three existing components retain their current relative drop order.
Today remains the final survivor. Replace the current boolean pair with this
explicit ladder and have `internal/tui/operational_title.go:131-149` iterate its
candidates.

## Vocabulary check

The user-facing terms fixed here are `doing`, `deferred`, `stale`, `today`,
`median`, `pace`, `workspace`, `inbox`, `overdue`, and `done`. None appears in
the retired product-concept map at
`retired_product_vocabulary_test.go:249-256`. Future documentation and scenario
names must keep these spellings or choose another term that also remains
outside that map.
