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
// presentations. Inbox, Today, and Overdue count actionable work and exclude
// terminal and deferred tasks; Done is the current local day's progress count.
type WorkspacePulse struct {
	Inbox   int
	Today   int
	Overdue int
	Done    int
}

type workspacePulseLevel uint8

const (
	workspacePulseFull workspacePulseLevel = iota
	workspacePulseWithoutDone
	workspacePulseWithoutInbox
	workspacePulseTodayOnly
	workspacePulseHidden
)

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

// formatWorkspacePulseComponents is the title-fitting seam for the fixed
// pressure ladder: Done, Inbox, Overdue, then Today. The full form keeps the
// semantic component order stable, and Today and Done render even at zero.
func formatWorkspacePulseComponents(pulse WorkspacePulse, level workspacePulseLevel) string {
	if level >= workspacePulseHidden {
		return ""
	}

	parts := make([]string, 0, 4)
	if level < workspacePulseWithoutInbox && pulse.Inbox > 0 {
		parts = append(parts, MutedStyle.Render(fmt.Sprintf("inbox %d", pulse.Inbox)))
	}
	parts = append(parts, MutedStyle.Render(fmt.Sprintf("today %d", pulse.Today)))
	if level < workspacePulseTodayOnly && pulse.Overdue > 0 {
		parts = append(parts, PreviewWarningStyle.Render(fmt.Sprintf("overdue %d", pulse.Overdue)))
	}
	if level < workspacePulseWithoutDone {
		parts = append(parts, MutedStyle.Render(fmt.Sprintf("done %d", pulse.Done)))
	}
	return MutedStyle.Render("workspace: ") + strings.Join(parts, MutedStyle.Render(" · "))
}
