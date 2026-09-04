package lens

import (
	"slices"
	"strconv"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// A lens name is colored by what the lens selects and by nothing else. Neither
// its name, its UUID, nor whether it came from the built-in catalog reaches this
// file: two lenses with the same definition wear the same accent, and one lens
// edited under its own ID changes accent the moment its definition does. That is
// the whole contract, and it is what lets an added template and a hand-built
// twin of it look alike without anybody maintaining a list of which is which.
//
// A composite definition selects on several axes at once, so the ladder below is
// first-match rather than a sum: it names the one signal a reader would call the
// point of the lens. Waiting outranks priority for the same reason the task row
// treats waiting as a hero attribute — a blocked item is blocked whatever its
// number. A state set spanning two groups picks no group at all rather than an
// arbitrary member's color, and falls through to whatever the next rung sees.
//
// Unknown tokens are not errors here. A lens carrying a mode spelling this build
// does not know simply fails the rung that would have read it, and a narrowing
// token the canonical vocabulary refuses never becomes a clause at all: it stays
// in the query's verbatim remainder, which the ladder does not read. A definition
// that reaches the bottom is neutral: uncolored ordinary text, never an invented
// color.
//
// The ladder reads the canonical half of the saved query and only its positive
// clauses. A negated clause selects the complement of what it names, so coloring
// by it would give "not overdue" the overdue accent; a lens whose narrowing is
// entirely negative reaches the bottom of the ladder instead.

// namedLensRoleKind is the semantic identity of a lens definition, in the
// precedence order the ladder resolves.
type namedLensRoleKind int

const (
	namedLensRoleNone namedLensRoleKind = iota
	namedLensRoleOverdue
	namedLensRoleWaiting
	namedLensRoleDueToday
	namedLensRolePriority
	namedLensRoleDue
	namedLensRoleDeferred
	namedLensRoleReview
	namedLensRoleCompleted
	namedLensRoleWorkflowGroup
	namedLensRoleWarning
	namedLensRoleMode
)

// A kind is never spelled in production: no build path reads a role back out of
// its name, so the naming used by failure messages lives with the tests that
// print it (`roleText`) rather than here.

// namedLensRole is one resolved semantic identity. The three payload fields are
// meaningful only for the kind that names them; every other kind leaves them
// zero, which keeps the whole value comparable and directly table-testable.
type namedLensRole struct {
	Kind     namedLensRoleKind
	Priority int
	Group    models.WorkflowStateGroup
	Mode     chrome.OutlinerModeID
}

// namedLensSemanticRole classifies one lens definition. It is pure: no theme, no
// runtime, no identity, and no allocation the caller can observe.
func namedLensSemanticRole(lens config.NamedLens) namedLensRole {
	facets, _ := models.PartitionQueryString(lens.Query)
	states := namedLensStates(namedLensClauseValues(facets, models.QueryFieldState))
	grouped := len(states) > 0
	due, _ := namedLensClauseValue(facets, models.QueryFieldDue)
	priority := namedLensPriority(facets)

	switch {
	case due == models.QueryDueOverdue:
		return namedLensRole{Kind: namedLensRoleOverdue}
	case grouped && slices.Contains(states, models.StateWaiting):
		return namedLensRole{Kind: namedLensRoleWaiting}
	case due == models.QueryDueToday:
		return namedLensRole{Kind: namedLensRoleDueToday}
	case models.ValidatePriority(priority, false) == nil:
		return namedLensRole{Kind: namedLensRolePriority, Priority: priority}
	case namedLensDueBucketAccented(due):
		return namedLensRole{Kind: namedLensRoleDue}
	case namedLensHasClause(facets, models.QueryFieldDeferred):
		return namedLensRole{Kind: namedLensRoleDeferred}
	case namedLensHasClause(facets, models.QueryFieldReview):
		return namedLensRole{Kind: namedLensRoleReview}
	case namedLensHasClause(facets, models.QueryFieldCompleted) ||
		(grouped && namedLensStatesAllTerminal(states)):
		return namedLensRole{Kind: namedLensRoleCompleted}
	}

	if grouped {
		if group, ok := namedLensSingleStateGroup(states); ok {
			return namedLensRole{Kind: namedLensRoleWorkflowGroup, Group: group}
		}
	}

	if namedLensHasClause(facets, models.QueryFieldPinned) ||
		namedLensHasClause(facets, models.QueryFieldParked) ||
		namedLensHasClause(facets, models.QueryFieldArchived) ||
		namedLensHasClause(facets, models.QueryFieldStale) {
		return namedLensRole{Kind: namedLensRoleWarning}
	}

	if mode, err := ModeAxis.Value(lens.Mode); err == nil && mode != chrome.ModeAll {
		return namedLensRole{Kind: namedLensRoleMode, Mode: mode}
	}

	return namedLensRole{Kind: namedLensRoleNone}
}

// namedLensHasClause reports whether the query names one field positively.
func namedLensHasClause(query models.Query, field models.QueryField) bool {
	for _, clause := range query.Clauses {
		if clause.Field == field && clause.Polarity == models.QueryPolarityPositive {
			return true
		}
	}
	return false
}

// namedLensClauseValue reads the first positive value of one field. The ladder's
// value-carrying rungs are single-valued axes, so a field named twice is read by
// its canonically first clause rather than by an arbitrary one.
func namedLensClauseValue(query models.Query, field models.QueryField) (string, bool) {
	for _, clause := range query.Canonical().Clauses {
		if clause.Field == field && clause.Polarity == models.QueryPolarityPositive {
			return clause.Value, true
		}
	}
	return "", false
}

// namedLensClauseValues reads every positive value of one field, in canonical
// order.
func namedLensClauseValues(query models.Query, field models.QueryField) []string {
	values := make([]string, 0, 4)
	for _, clause := range query.Canonical().Clauses {
		if clause.Field == field && clause.Polarity == models.QueryPolarityPositive {
			values = append(values, clause.Value)
		}
	}
	return values
}

// namedLensPriority reads the priority threshold, or zero when the lens names
// none. A value the canonical vocabulary carried but this build cannot read as a
// number is zero too, which fails the priority rung the same way an absent one
// does.
func namedLensPriority(query models.Query) int {
	value, ok := namedLensClauseValue(query, models.QueryFieldPriority)
	if !ok {
		return 0
	}
	priority, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return priority
}

// NamedLensPrimaryStyle is the one entry point a picker calls. It returns a
// foreground-only style, or false for a definition with no recognized role, so
// a neutral row keeps the list's ordinary text exactly as it is.
func NamedLensPrimaryStyle(lens config.NamedLens) (lipgloss.Style, bool) {
	return namedLensRoleStyle(namedLensSemanticRole(lens))
}

// namedLensRoleStyle resolves a role through the shared theme accents. Waiting
// and Completed borrow their group's accent rather than declaring a second one:
// they are the waiting and done groups seen through a filter.
func namedLensRoleStyle(role namedLensRole) (lipgloss.Style, bool) {
	switch role.Kind {
	case namedLensRoleOverdue:
		return theme.LensAccentOverdueStyle, true
	case namedLensRoleWaiting:
		return theme.WorkflowStateGroupAccentStyle(models.StateGroupWaiting)
	case namedLensRoleDueToday:
		return theme.LensAccentDueTodayStyle, true
	case namedLensRolePriority:
		return namedLensPriorityAccentStyle(role.Priority)
	case namedLensRoleDue:
		return theme.LensAccentDueStyle, true
	case namedLensRoleDeferred:
		return theme.LensAccentDeferredStyle, true
	case namedLensRoleReview:
		return theme.LensAccentReviewStyle, true
	case namedLensRoleCompleted:
		return theme.WorkflowStateGroupAccentStyle(models.StateGroupDone)
	case namedLensRoleWorkflowGroup:
		return theme.WorkflowStateGroupAccentStyle(role.Group)
	case namedLensRoleWarning:
		return lipgloss.NewStyle().Foreground(theme.ColorWarning), true
	case namedLensRoleMode:
		if group, ok := chrome.OutlinerModeStateGroup(role.Mode); ok {
			return theme.WorkflowStateGroupAccentStyle(group)
		}
	case namedLensRoleNone:
		// A definition with no recognized role keeps the list's ordinary text,
		// which is what the shared no-style return below already says.
	}
	return lipgloss.Style{}, false
}

// namedLensPriorityAccentStyle carries the configured priority title color and
// nothing else, so a palette override reaches a lens name while the title
// style's bold and italic — which belong to a task row — stay behind.
func namedLensPriorityAccentStyle(priority int) (lipgloss.Style, bool) {
	foreground := theme.GetTitlePriorityStyle(priority).GetForeground()
	if _, unset := foreground.(lipgloss.NoColor); unset {
		return lipgloss.Style{}, false
	}
	return lipgloss.NewStyle().Foreground(foreground), true
}

// namedLensStates reads the persisted state tokens. A spelling this build cannot
// recognize never reaches here: models.NewQueryClause refuses it, so the token
// stays in the query's unparseable remainder and the lens simply names no state
// rather than naming one we would have to decline to color by.
func namedLensStates(tokens []string) []models.WorkflowState {
	states := make([]models.WorkflowState, 0, len(tokens))
	for _, token := range tokens {
		states = append(states, models.WorkflowState(token))
	}
	return states
}

func namedLensStatesAllTerminal(states []models.WorkflowState) bool {
	for _, state := range states {
		if !state.IsTerminal() {
			return false
		}
	}
	return true
}

// namedLensSingleStateGroup reports the one group every selected state belongs
// to. A set spanning two groups reports none.
func namedLensSingleStateGroup(states []models.WorkflowState) (models.WorkflowStateGroup, bool) {
	var group models.WorkflowStateGroup
	for i, state := range states {
		candidate := models.WorkflowStateGroupForState(state)
		if !candidate.IsValid() {
			return "", false
		}
		if i == 0 {
			group = candidate
			continue
		}
		if candidate != group {
			return "", false
		}
	}
	return group, group.IsValid()
}

// namedLensDueBucketAccented reports whether a due bucket names an active date
// window worth an accent. Overdue and today have already been answered above;
// a due clause selecting by the presence or absence of a date rather than by its
// urgency is not urgency, and an absent bucket is not a filter at all.
func namedLensDueBucketAccented(bucket string) bool {
	return bucket == models.QueryDueWeek
}
