package tui

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// analyzeBulkStateTransition separates tasks that can transition to newState
// from those blocked by transition rules, incomplete dependencies (completion
// only), or incomplete children (any terminal state).
func analyzeBulkStateTransition(project *models.Project, tasks []*models.Task, newState models.WorkflowState) (valid []*models.Task, blockedCount int) {
	for _, task := range tasks {
		if models.CanTransitionState(task.State(), newState) != "" {
			continue
		}
		if newState == models.StateDone && project.HasIncompleteDependencies(task) {
			blockedCount++
			continue
		}
		if newState.IsTerminal() && task.HasIncompleteChildren() {
			blockedCount++
			continue
		}
		valid = append(valid, task)
	}
	return
}

// resolveQuickSnoozeDate maps a snooze sub-key to a snooze date and description.
// Returns (nil, "", false) for unrecognized keys. The "d" (date picker) case
// must be handled by the caller.
func resolveQuickSnoozeDate(key string) (snoozeDate *time.Time, desc string, ok bool) {
	today := util.StartOfDay(time.Now())

	switch key {
	case "t":
		d := today.AddDate(0, 0, 1)
		return &d, "tomorrow", true
	case "w":
		d := today.AddDate(0, 0, 7)
		return &d, "+1 week", true
	case "m":
		d := today.AddDate(0, 1, 0)
		return &d, "+1 month", true
	case "n":
		daysUntilMon := (8 - int(today.Weekday())) % 7
		if daysUntilMon == 0 {
			daysUntilMon = 7
		}
		d := today.AddDate(0, 0, daysUntilMon)
		return &d, "next Monday", true
	case "0":
		return nil, "cleared", true
	default:
		return nil, "", false
	}
}

// toggleFlagKind identifies which boolean flag a toggleSpec operates on, so the
// coordinator-backed single-task path can dispatch to the matching application
// mutation (SetPinned / ToggleDimmed).
type toggleFlagKind int

const (
	toggleFlagPinned toggleFlagKind = iota
	toggleFlagDimmed
)

// toggleSpec describes a boolean toggle operation on a task.
type toggleSpec struct {
	kind     toggleFlagKind            // which flag this spec toggles
	mutateFn func(t *models.Task)      // e.g. (*Task).TogglePinned
	readFlag func(t *models.Task) bool // post-state getter, e.g. func(t) { return t.Pinned }
	onFmt    string                    // format string when flag is true,  e.g. "Pinned task #%d"
	offFmt   string                    // format string when flag is false, e.g. "Unpinned task #%d"
}

var (
	togglePinned = toggleSpec{
		kind:     toggleFlagPinned,
		mutateFn: (*models.Task).TogglePinned,
		readFlag: func(t *models.Task) bool { return t.Pinned },
		onFmt:    "Pinned task #%d",
		offFmt:   "Unpinned task #%d",
	}
	toggleDimmed = toggleSpec{
		kind:     toggleFlagDimmed,
		mutateFn: (*models.Task).ToggleDimmed,
		readFlag: func(t *models.Task) bool { return t.Dimmed },
		onFmt:    "Dimmed task #%d",
		offFmt:   "Undimmed task #%d",
	}
)

// dueDateLabel returns a human-friendly label for a relative due date.
func dueDateLabel(daysFromNow int) string {
	switch daysFromNow {
	case 0:
		return "today"
	case 1:
		return "tomorrow"
	case 7:
		return "next week"
	case 30:
		return "next month"
	default:
		return fmt.Sprintf("+%d days", daysFromNow)
	}
}

// dueDateFromDays returns a *time.Time pointer and description for a relative
// day offset. When isClear is true, it returns (nil, "cleared").
// The returned time is normalized to start-of-day (midnight) for consistency
// with all other due-date setters and the day-level storage format.
func dueDateFromDays(days int, isClear bool) (*time.Time, string) {
	if isClear {
		return nil, "cleared"
	}
	d := util.StartOfDay(time.Now().AddDate(0, 0, days))
	return &d, dueDateLabel(days)
}

// cleanupConfigTaskRefs removes deleted task IDs from FocusQueue, DailyPlan,
// and RecentTaskVisits in the config. This mirrors the cleanup done in
// cmd/task_delete.go and prevents stale references from accumulating.
func cleanupConfigTaskRefs(cfg *config.Config, mgr *config.Manager, idsToDelete map[string]bool) {
	if cfg == nil || mgr == nil || len(idsToDelete) == 0 {
		return
	}
	if cfg.RemoveTaskRefs(idsToDelete) > 0 {
		if err := mgr.Save(cfg); err != nil {
			logging.Warn("failed to save config after cleaning task refs: %v", err)
		}
	}
}

// collectDescendantIDs collects all IDs of a task and its descendants recursively.
func collectDescendantIDs(tasks []*models.Task, rootIDs map[string]bool) map[string]bool {
	idsToDelete := make(map[string]bool)
	var collectIDs func(t *models.Task)
	collectIDs = func(t *models.Task) {
		idsToDelete[t.ID] = true
		for _, child := range t.Children {
			collectIDs(child)
		}
	}
	for _, t := range tasks {
		if rootIDs[t.ID] {
			collectIDs(t)
		}
	}
	return idsToDelete
}
