package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// OpenLoops surfaces unresolved-task counts (Zeigarnik Effect). These are the
// counters rendered into the title row across daily-driver views so the user
// always has a passive readout of "what's still open" without leaving any view.
type OpenLoops struct {
	Inbox      int
	TodayDone  int
	TodayTotal int
	Overdue    int
	HasTasks   bool // true when at least one task exists across all projects
}

// IsZero reports whether all counter components are zero, in which case the
// formatted pill should be empty and the title row should not allocate space.
func (o OpenLoops) IsZero() bool {
	return o.Inbox == 0 && o.TodayDone == 0 && o.TodayTotal == 0 && o.Overdue == 0
}

// RefreshOpenLoopsMsg requests a recompute + cache update of AppModel.openLoops.
// It is dispatched from mutation paths so the counter stays in sync after edits.
type RefreshOpenLoopsMsg struct{}

// openLoopsAware is implemented by views that render the title-row Zeigarnik
// counter. AppModel injects the latest counts before each render so the cached
// value stays consistent across daily-driver views.
type openLoopsAware interface {
	setOpenLoops(OpenLoops)
}

// ComputeOpenLoops counts the open loops across all loadable projects using the
// same cross-scope loader Focus Board uses. dailyPlanIDs supplies the Today
// scope (which tasks are planned for today) so the counter agrees with the
// Focus Board Today tab regardless of completion status.
func ComputeOpenLoops(cfg *config.Config, dailyPlanIDs map[string]bool) OpenLoops {
	if cfg == nil {
		return OpenLoops{}
	}
	var out OpenLoops
	svc := NewProjectService()
	_, projects, _ := LoadCrossScopeTasks(cfg, svc, CrossScopeOpts{})
	for _, project := range projects {
		for _, task := range project.Tasks {
			out.HasTasks = true
			// Today scope: include planned tasks regardless of completion (matches Focus Board).
			if dailyPlanIDs != nil && dailyPlanIDs[task.ID] {
				out.TodayTotal++
				if task.Status.IsComplete() {
					out.TodayDone++
				}
			}

			if task.Status.IsComplete() {
				continue
			}

			if task.Section == models.SectionInbox && task.IsRoot() {
				out.Inbox++
			}
			if task.IsOverdue() {
				out.Overdue++
			}
		}
	}
	return out
}

// FormatOpenLoops renders a compact "inbox 3 · today 2/7 · overdue 1" pill,
// suppressing zero components individually. Returns "" when all counts are zero
// so callers can decide whether to allocate any title-row space at all.
func FormatOpenLoops(o OpenLoops) string {
	var parts []string
	if o.Inbox > 0 {
		parts = append(parts, fmt.Sprintf("inbox %d", o.Inbox))
	}
	if o.TodayTotal > 0 {
		if o.TodayDone == o.TodayTotal {
			parts = append(parts, SuccessStyle.Render("today ✓"))
		} else {
			parts = append(parts, fmt.Sprintf("today %d/%d", o.TodayDone, o.TodayTotal))
		}
	}
	if o.Overdue > 0 {
		parts = append(parts, PreviewWarningStyle.Render(fmt.Sprintf("overdue %d", o.Overdue)))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " · ")
}
