package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

type RecentPaletteItem struct {
	Category string `json:"category"`
	ID       string `json:"id"`
	Count    int    `json:"count,omitempty"`
}

// OutlinerRouteState is the machine-local persisted view state for one
// Outliner scope. It is deliberately a presentation DTO rather than a
// navigation Route: typed NavigationState remains the sole route authority,
// while this value remembers only the mode and the three predicates needed to
// migrate retired modes without losing their meaning.
type OutlinerRouteState struct {
	Mode          string                  `json:"mode,omitempty"`
	FilterStates  models.WorkflowStateSet `json:"filter_states,omitzero"`
	FilterSnoozed bool                    `json:"filter_snoozed,omitempty"`
	FilterPinned  bool                    `json:"filter_pinned,omitempty"`
}

// UnmarshalJSON keeps legacy route persistence readable without making retired
// states valid in the target WorkflowStateSet API. Review filters normalize to
// Ready, matching task-state conversion; unknown values remain hard errors.
func (s *OutlinerRouteState) UnmarshalJSON(data []byte) error {
	var raw struct {
		Mode          string                 `json:"mode,omitempty"`
		FilterStates  []models.WorkflowState `json:"filter_states,omitempty"`
		FilterSnoozed bool                   `json:"filter_snoozed,omitempty"`
		FilterPinned  bool                   `json:"filter_pinned,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	states := make([]models.WorkflowState, 0, len(raw.FilterStates))
	for _, legacy := range raw.FilterStates {
		target, ok := models.TargetStateForLegacy(legacy)
		if !ok {
			return fmt.Errorf("invalid workflow state %q", legacy)
		}
		states = append(states, target)
	}
	filterStates, err := models.NewWorkflowStateSet(states...)
	if err != nil {
		return err
	}
	*s = OutlinerRouteState{
		Mode:          raw.Mode,
		FilterStates:  filterStates,
		FilterSnoozed: raw.FilterSnoozed,
		FilterPinned:  raw.FilterPinned,
	}
	return nil
}

const (
	OutlinerModeAll      = "all"
	OutlinerModeInbox    = "inbox"
	OutlinerModeNext     = "next"
	OutlinerModeBacklog  = "backlog"
	OutlinerModeDue      = "due"
	OutlinerModePriority = "priority"
	OutlinerModeBlocked  = "blocked"
)

// NormalizeOutlinerRouteState returns a defensive, canonical copy of state.
// Missing and unknown modes fall back to All. Retired mode names are translated
// to one of the ten modes plus the smallest canonical predicate that preserves
// their meaning.
func NormalizeOutlinerRouteState(state OutlinerRouteState) OutlinerRouteState {
	state.FilterStates = cloneWorkflowStateSet(state.FilterStates)
	mode := normalizeOutlinerModeToken(state.Mode)
	switch mode {
	case "active":
		state.Mode = OutlinerModeAll
		state.FilterStates = mustWorkflowStateSet(models.StateReady, models.StateInProgress)
	case "waiting":
		state.Mode = OutlinerModeAll
		state.FilterStates = mustWorkflowStateSet(models.StateWaiting)
	case "standby":
		state.Mode = OutlinerModeAll
		state.FilterStates = mustWorkflowStateSet(models.StatePaused)
	case "someday":
		state.Mode = OutlinerModeAll
		state.FilterStates = mustWorkflowStateSet(models.StateSomeday)
	case "review":
		state.Mode = OutlinerModeAll
		// Review tasks are converted to Ready plus the review tag by the task
		// compatibility reader. The retired route has no tag facet, so Ready is
		// the closest target-state predicate it can retain.
		state.FilterStates = mustWorkflowStateSet(models.StateReady)
	case "snoozed":
		state.Mode = OutlinerModeAll
		state.FilterSnoozed = true
	case "completed", "complete", "done":
		state.Mode = OutlinerModeAll
		state.FilterStates = mustWorkflowStateSet(models.StateDone, models.StateCanceled)
	case "due-soon", "due-date", "due-dates":
		state.Mode = OutlinerModeDue
	case "high-priority":
		state.Mode = OutlinerModePriority
	case "pinned":
		state.Mode = OutlinerModeAll
		state.FilterPinned = true
	case "recent", "recently-visited", "recent-tasks":
		state.Mode = OutlinerModeAll
	case "daily-review":
		state.Mode = OutlinerModeAll
	case "parked":
		state.Mode = OutlinerModeAll
	case OutlinerModeAll,
		OutlinerModeInbox,
		OutlinerModeNext,
		OutlinerModeBacklog,
		OutlinerModeDue,
		OutlinerModePriority,
		OutlinerModeBlocked:
		state.Mode = mode
	default:
		state.Mode = OutlinerModeAll
	}
	return state
}

func normalizeOutlinerModeToken(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	mode = strings.NewReplacer("_", "-", " ", "-").Replace(mode)
	for strings.Contains(mode, "--") {
		mode = strings.ReplaceAll(mode, "--", "-")
	}
	return mode
}

func mustWorkflowStateSet(states ...models.WorkflowState) models.WorkflowStateSet {
	set, err := models.NewWorkflowStateSet(states...)
	if err != nil {
		panic(err)
	}
	return set
}

func cloneWorkflowStateSet(states models.WorkflowStateSet) models.WorkflowStateSet {
	return mustWorkflowStateSet(states.Values()...)
}

const (
	// CurrentTUIStateVersion is the last v4 route/presentation version understood
	// by the legacy-domain migration converter.
	//
	// Version 4 introduces the ten-mode OutlinerRouteState and migrates retired
	// perspectives, smart focuses, lenses, startup destinations, project view
	// states, and cross-Outliner lifecycle filters into canonical route DTOs.
	CurrentTUIStateVersion = 4
)
