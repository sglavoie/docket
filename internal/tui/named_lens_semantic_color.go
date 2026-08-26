package tui

import (
	"slices"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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
// Unknown tokens are not errors here. A lens carrying a state, mode, due bucket,
// or population spelling this build does not know simply fails every rung that
// would have read it, and a definition that reaches the bottom of the ladder is
// neutral: uncolored ordinary text, never an invented color.

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

// String names the kind for test failures and debugging only. Nothing reads a
// role back out of its spelling.
func (k namedLensRoleKind) String() string {
	switch k {
	case namedLensRoleNone:
		return "none"
	case namedLensRoleOverdue:
		return "overdue"
	case namedLensRoleWaiting:
		return "waiting"
	case namedLensRoleDueToday:
		return "due-today"
	case namedLensRolePriority:
		return "priority"
	case namedLensRoleDue:
		return "due"
	case namedLensRoleDeferred:
		return "deferred"
	case namedLensRoleReview:
		return "review"
	case namedLensRoleCompleted:
		return "completed"
	case namedLensRoleWorkflowGroup:
		return "workflow-group"
	case namedLensRoleWarning:
		return "warning"
	case namedLensRoleMode:
		return "mode"
	default:
		return "unknown"
	}
}

// namedLensRole is one resolved semantic identity. The three payload fields are
// meaningful only for the kind that names them; every other kind leaves them
// zero, which keeps the whole value comparable and directly table-testable.
type namedLensRole struct {
	Kind     namedLensRoleKind
	Priority int
	Group    models.WorkflowStateGroup
	Mode     OutlinerModeID
}

// namedLensSemanticRole classifies one lens definition. It is pure: no theme, no
// runtime, no identity, and no allocation the caller can observe.
func namedLensSemanticRole(lens config.NamedLens) namedLensRole {
	tasks := lens.Tasks
	states, statesKnown := namedLensStates(tasks.States)
	grouped := statesKnown && len(states) > 0

	switch {
	case tasks.DueDate == config.LensDueOverdue:
		return namedLensRole{Kind: namedLensRoleOverdue}
	case grouped && slices.Contains(states, models.StateWaiting):
		return namedLensRole{Kind: namedLensRoleWaiting}
	case tasks.DueDate == config.LensDueToday:
		return namedLensRole{Kind: namedLensRoleDueToday}
	case models.ValidatePriority(tasks.Priority, false) == nil:
		return namedLensRole{Kind: namedLensRolePriority, Priority: tasks.Priority}
	case namedLensDueBucketAccented(tasks.DueDate):
		return namedLensRole{Kind: namedLensRoleDue}
	case tasks.Deferred:
		return namedLensRole{Kind: namedLensRoleDeferred}
	case namedLensSelectsReviewTag(tasks.Tags):
		return namedLensRole{Kind: namedLensRoleReview}
	case tasks.CompletionAgeSet || (grouped && namedLensStatesAllTerminal(states)):
		return namedLensRole{Kind: namedLensRoleCompleted}
	}

	if grouped {
		if group, ok := namedLensSingleStateGroup(states); ok {
			return namedLensRole{Kind: namedLensRoleWorkflowGroup, Group: group}
		}
	}

	if tasks.Pinned ||
		namedLensPopulationAccented(lens.Projects.ParkedMode) ||
		namedLensPopulationAccented(lens.Projects.ArchivedMode) ||
		tasks.Stale {
		return namedLensRole{Kind: namedLensRoleWarning}
	}

	if mode, err := lensModeAxis.value(lens.Mode); err == nil && mode != ModeAll {
		return namedLensRole{Kind: namedLensRoleMode, Mode: mode}
	}

	return namedLensRole{Kind: namedLensRoleNone}
}

// namedLensPrimaryStyle is the one entry point a picker calls. It returns a
// foreground-only style, or false for a definition with no recognized role, so
// a neutral row keeps the list's ordinary text exactly as it is.
func namedLensPrimaryStyle(lens config.NamedLens) (lipgloss.Style, bool) {
	return namedLensRoleStyle(namedLensSemanticRole(lens))
}

// namedLensRoleStyle resolves a role through the shared theme accents. Waiting
// and Completed borrow their group's accent rather than declaring a second one:
// they are the waiting and done groups seen through a filter.
func namedLensRoleStyle(role namedLensRole) (lipgloss.Style, bool) {
	switch role.Kind {
	case namedLensRoleOverdue:
		return lensAccentOverdueStyle, true
	case namedLensRoleWaiting:
		return workflowStateGroupAccentStyle(models.StateGroupWaiting)
	case namedLensRoleDueToday:
		return lensAccentDueTodayStyle, true
	case namedLensRolePriority:
		return namedLensPriorityAccentStyle(role.Priority)
	case namedLensRoleDue:
		return lensAccentDueStyle, true
	case namedLensRoleDeferred:
		return lensAccentDeferredStyle, true
	case namedLensRoleReview:
		return lensAccentReviewStyle, true
	case namedLensRoleCompleted:
		return workflowStateGroupAccentStyle(models.StateGroupDone)
	case namedLensRoleWorkflowGroup:
		return workflowStateGroupAccentStyle(role.Group)
	case namedLensRoleWarning:
		return lipgloss.NewStyle().Foreground(ColorWarning), true
	case namedLensRoleMode:
		if group, ok := outlinerModeStateGroup(role.Mode); ok {
			return workflowStateGroupAccentStyle(group)
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
	foreground := GetTitlePriorityStyle(priority).GetForeground()
	if _, unset := foreground.(lipgloss.NoColor); unset || foreground == nil {
		return lipgloss.Style{}, false
	}
	return lipgloss.NewStyle().Foreground(foreground), true
}

// namedLensStates parses the persisted state tokens. A single unrecognized
// spelling disqualifies the whole set: a lens selecting states this build cannot
// read is not a lens whose states we may color by.
func namedLensStates(tokens []string) ([]models.WorkflowState, bool) {
	states := make([]models.WorkflowState, 0, len(tokens))
	for _, token := range tokens {
		state, ok := models.ParseWorkflowState(token)
		if !ok {
			return nil, false
		}
		states = append(states, state)
	}
	return states, true
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
// `all` and `none` select by the presence of a date rather than by its urgency,
// and an absent bucket is not a filter at all.
func namedLensDueBucketAccented(bucket string) bool {
	return bucket == config.LensDueWeek
}

// namedLensPopulationAccented reports whether a population axis has been moved
// off its default. Excluding parked or archived projects is what the Outliner
// already does, so only admitting them is a signal.
func namedLensPopulationAccented(mode string) bool {
	return mode == config.LensPopulationInclude || mode == config.LensPopulationOnly
}

// namedLensSelectsReviewTag reports whether the lens selects the promoted Review
// tag, matched on the normalized spelling the tag itself is stored under.
func namedLensSelectsReviewTag(tags []string) bool {
	return slices.ContainsFunc(tags, func(tag string) bool {
		return models.NormalizeTag(tag) == reviewTaskTag
	})
}
