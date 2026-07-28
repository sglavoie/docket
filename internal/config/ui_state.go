package config

import (
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

const (
	OutlinerModeAll         = "all"
	OutlinerModeInbox       = "inbox"
	OutlinerModeNext        = "next"
	OutlinerModeBacklog     = "backlog"
	OutlinerModeDue         = "due"
	OutlinerModePriority    = "priority"
	OutlinerModeBlocked     = "blocked"
	OutlinerModeParked      = "parked"
	OutlinerModeDailyReview = "daily-review"
	OutlinerModeActivity    = "activity"
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
		state.FilterStates = mustWorkflowStateSet(models.StateReview)
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
		state.Mode = OutlinerModeActivity
	case OutlinerModeAll,
		OutlinerModeInbox,
		OutlinerModeNext,
		OutlinerModeBacklog,
		OutlinerModeDue,
		OutlinerModePriority,
		OutlinerModeBlocked,
		OutlinerModeParked,
		OutlinerModeDailyReview,
		OutlinerModeActivity:
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

// RecentDestinationKind identifies a finder destination without importing TUI
// route or presentation types into the machine-local config domain.
type RecentDestinationKind string

const (
	RecentDestinationTask    RecentDestinationKind = "task"
	RecentDestinationProject RecentDestinationKind = "project"
	RecentDestinationNote    RecentDestinationKind = "note"
	RecentDestinationTool    RecentDestinationKind = "tool"
	RecentDestinationMode    RecentDestinationKind = "mode"
	RecentDestinationScope   RecentDestinationKind = "scope"
	RecentDestinationTag     RecentDestinationKind = "tag"
)

// RecentDestination is the durable identity of a finder destination.
// ProjectID qualifies task and project-note IDs so equal entity IDs in
// different projects never share recency. Count is retained for future ranking
// while slice order remains the authoritative most-recent-first order.
type RecentDestination struct {
	Kind      RecentDestinationKind `json:"kind"`
	ID        string                `json:"id"`
	ProjectID string                `json:"project_id,omitempty"`
	Count     int                   `json:"count,omitempty"`
}

// Valid reports whether a destination has enough identity to be persisted.
// Retired kinds fall through to false, so a machine-local history written by an
// older build loses only the entries whose destination no longer exists.
func (destination RecentDestination) Valid() bool {
	if strings.TrimSpace(destination.ID) == "" {
		return false
	}
	switch destination.Kind {
	case RecentDestinationTask:
		return strings.TrimSpace(destination.ProjectID) != ""
	case RecentDestinationNote,
		RecentDestinationProject,
		RecentDestinationTool,
		RecentDestinationMode,
		RecentDestinationScope,
		RecentDestinationTag:
		return true
	default:
		return false
	}
}

// QualifiedID returns the stable finder identity represented by destination.
func (destination RecentDestination) QualifiedID() string {
	if !destination.Valid() {
		return ""
	}
	kind := string(destination.Kind)
	switch destination.Kind {
	case RecentDestinationTask:
		return kind + ":" + destination.ProjectID + ":" + destination.ID
	case RecentDestinationNote:
		projectID := destination.ProjectID
		if projectID == "" {
			projectID = "global"
		}
		return kind + ":" + projectID + ":" + destination.ID
	default:
		return kind + ":" + destination.ID
	}
}

const (
	// CurrentTUIStateVersion versions route and presentation state inside the
	// machine, planning, and settings domains independently from their envelope
	// schema. Each owning domain can therefore migrate and commit atomically.
	//
	// Version 4 introduces the ten-mode OutlinerRouteState and migrates retired
	// perspectives, smart focuses, lenses, startup destinations, project view
	// states, and cross-Outliner lifecycle filters into canonical route DTOs.
	CurrentTUIStateVersion = 4

	// MaxRecentPaletteItems is the durable cap for the one palette history that
	// survives the TUI-state migration.
	MaxRecentPaletteItems = 10

	// MaxRecentDestinations bounds the machine-local finder destination
	// history. Finder empty-query output uses only the three newest live
	// destinations, but retaining a slightly deeper history allows stale
	// references to disappear without emptying the shortlist.
	MaxRecentDestinations = 10
)
