package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
)

// WorkspacePulse is the compact operational readout shared by task-workspace
// presentations. Every component counts actionable work: terminal and deferred
// tasks are excluded from Inbox, Today, and Overdue.
type WorkspacePulse struct {
	Inbox   int
	Today   int
	Overdue int
}

// workspacePulseState is embedded by full-screen presentations that own a
// workspace-pulse title. The app shell refreshes it immediately before paint.
// Keeping this state separate from view-specific scope makes task- and
// project-scoped screens label the readout as workspace-wide.
type workspacePulseState struct {
	workspacePulse WorkspacePulse
}

func (s *workspacePulseState) setWorkspacePulse(pulse WorkspacePulse) {
	s.workspacePulse = pulse
}

func (s *workspacePulseState) currentWorkspacePulse() WorkspacePulse {
	return s.workspacePulse
}

// workspacePulseAware is implemented by views that render the workspace pulse.
// AppModel injects the latest cached value before each render.
type workspacePulseAware interface {
	setWorkspacePulse(WorkspacePulse)
}

// ComputeWorkspacePulse counts actionable work across the base task workspace
// at one injected reference time. It preserves the cross-scope loader's partial
// failure behavior: successfully loaded projects contribute while failures are
// warned and skipped.
//
// Today membership is project-qualified and counted at most once per planning
// identity. Stale references, ineligible tasks, and references to excluded
// projects do not contribute because only loaded eligible tasks are considered.
func ComputeWorkspacePulse(rs *config.RuntimeState, todayIDs map[TaskIdentity]bool, now time.Time) WorkspacePulse {
	if rs == nil {
		return WorkspacePulse{}
	}

	var pulse WorkspacePulse
	countedToday := make(map[TaskIdentity]bool, len(todayIDs))
	_, projects, failed := loadCrossScopeSelection(rs.TaskWorkspaceProjects(), nil)
	for _, failure := range failed {
		logging.Warn("workspace pulse: %s", failure.Detail)
	}
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || task.State().IsTerminal() || task.IsDeferredAt(now) {
				continue
			}

			identity := taskIdentityFor(task, project)
			if identity.Valid() && todayIDs[identity] && !countedToday[identity] && app.PlanningTodayEligible(task.State()) {
				pulse.Today++
				countedToday[identity] = true
			}
			if task.IsInState(models.StateInbox) {
				pulse.Inbox++
			}
			if taskIsOverdueAt(task, now) {
				pulse.Overdue++
			}
		}
	}
	return pulse
}

func taskIsOverdueAt(task *models.Task, now time.Time) bool {
	if task == nil || task.DueDate == nil || task.State().IsTerminal() {
		return false
	}
	nowYear, nowMonth, nowDay := now.Date()
	dueYear, dueMonth, dueDay := task.DueDate.Date()
	today := time.Date(nowYear, nowMonth, nowDay, 0, 0, 0, 0, time.UTC)
	due := time.Date(dueYear, dueMonth, dueDay, 0, 0, 0, 0, time.UTC)
	return due.Before(today)
}

// formatWorkspacePulseComponents is the title-fitting seam for progressively
// removing passive pulse components. Callers keep the semantic order stable;
// Today remains present even when its count is zero.
func formatWorkspacePulseComponents(pulse WorkspacePulse, includeInbox, includeOverdue bool) string {
	parts := make([]string, 0, 3)
	if includeInbox && pulse.Inbox > 0 {
		parts = append(parts, fmt.Sprintf("inbox %d", pulse.Inbox))
	}
	parts = append(parts, fmt.Sprintf("today %d", pulse.Today))
	if includeOverdue && pulse.Overdue > 0 {
		parts = append(parts, PreviewWarningStyle.Render(fmt.Sprintf("overdue %d", pulse.Overdue)))
	}
	if len(parts) == 0 {
		return ""
	}
	return "workspace: " + strings.Join(parts, " · ")
}
