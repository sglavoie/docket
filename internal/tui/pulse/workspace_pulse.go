package pulse

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
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

// WorkspacePulseState is embedded by full-screen presentations that own a
// workspace-pulse title. The app shell refreshes it immediately before paint.
// Keeping this state separate from view-specific scope makes task- and
// project-scoped screens label the readout as workspace-wide.
type WorkspacePulseState struct {
	workspacePulse WorkspacePulse
}

func (s *WorkspacePulseState) SetWorkspacePulse(pulse WorkspacePulse) {
	s.workspacePulse = pulse
}

func (s *WorkspacePulseState) CurrentWorkspacePulse() WorkspacePulse {
	return s.workspacePulse
}

// WorkspacePulseAware is implemented by views that render the workspace pulse.
// AppModel injects the latest cached value before each render.
type WorkspacePulseAware interface {
	SetWorkspacePulse(WorkspacePulse)
}

// ComputeWorkspacePulse counts actionable work across an already-loaded base
// task workspace at one injected reference time. Selecting and loading that
// population is the caller's, because the snapshot loader sits well above this
// package; passing the partial result through preserves the cross-scope
// loader's partial failure behavior, since a project that failed to load is
// simply absent from projects and contributes nothing.
//
// Today membership is project-qualified and counted at most once per planning
// identity. Stale references, ineligible tasks, and references to excluded
// projects do not contribute because only loaded eligible tasks are considered.
func ComputeWorkspacePulse(projects []*models.Project, todayIDs map[taskref.TaskIdentity]bool, now time.Time) WorkspacePulse {
	var pulse WorkspacePulse
	countedToday := make(map[taskref.TaskIdentity]bool, len(todayIDs))
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || task.State().IsTerminal() || task.IsDeferredAt(now) {
				continue
			}

			identity := taskref.For(task, project)
			if identity.Valid() && todayIDs[identity] && !countedToday[identity] && app.PlanningTodayEligible(task.State()) {
				pulse.Today++
				countedToday[identity] = true
			}
			if task.IsInState(models.StateInbox) {
				pulse.Inbox++
			}
			if TaskIsOverdueAt(task, now) {
				pulse.Overdue++
			}
		}
	}
	return pulse
}

func TaskIsOverdueAt(task *models.Task, now time.Time) bool {
	if task == nil || task.DueDate == nil || task.State().IsTerminal() {
		return false
	}
	if task.DueHasTime() {
		return task.DueDate.Before(now)
	}
	nowYear, nowMonth, nowDay := now.Date()
	dueYear, dueMonth, dueDay := task.DueDate.Date()
	today := time.Date(nowYear, nowMonth, nowDay, 0, 0, 0, 0, time.UTC)
	due := time.Date(dueYear, dueMonth, dueDay, 0, 0, 0, 0, time.UTC)
	return due.Before(today)
}

// formatWorkspacePulseComponents is the title-fitting seam for the fixed
// pressure ladder: Done, Inbox, Overdue, then Today. The full form keeps the
// semantic component order stable.
//
// A zero component is omitted rather than rendered, because "today 0 · done 0"
// spends the header's scarcest columns saying that nothing is there. The ladder
// is unchanged by that: it still drops Done, then Inbox, then Overdue, then
// Today, and it decides which of the nonzero components survive a narrow row.
// When none of them is nonzero there is no readout at all, so the "workspace: "
// prefix goes with them rather than standing over an empty list.
func formatWorkspacePulseComponents(pulse WorkspacePulse, level workspacePulseLevel) string {
	if level >= workspacePulseHidden {
		return ""
	}

	parts := make([]string, 0, 4)
	if level < workspacePulseWithoutInbox && pulse.Inbox > 0 {
		parts = append(parts, theme.MutedStyle.Render(fmt.Sprintf("inbox %d", pulse.Inbox)))
	}
	if pulse.Today > 0 {
		parts = append(parts, theme.MutedStyle.Render(fmt.Sprintf("today %d", pulse.Today)))
	}
	if level < workspacePulseTodayOnly && pulse.Overdue > 0 {
		parts = append(parts, theme.PreviewWarningStyle.Render(fmt.Sprintf("overdue %d", pulse.Overdue)))
	}
	if level < workspacePulseWithoutDone && pulse.Done > 0 {
		parts = append(parts, theme.MutedStyle.Render(fmt.Sprintf("done %d", pulse.Done)))
	}
	if len(parts) == 0 {
		return ""
	}
	return theme.MutedStyle.Render("workspace: ") + strings.Join(parts, theme.MutedStyle.Render(" · "))
}

// WindowTitle is the pulse as the terminal's window title: `tt`, then each
// nonzero component in the readout's order and spelling, without Done, which
// is progress rather than something waiting. It is plain text and never
// shortened, since a title bar elides on its own. tt.app reads the overdue
// component back for its Dock badge (macos/TTApp/Shell/TTWindowTitle.swift),
// so the spelling `overdue N` is a contract with that parser.
func WindowTitle(pulse WorkspacePulse) string {
	parts := []string{"tt"}
	if pulse.Inbox > 0 {
		parts = append(parts, fmt.Sprintf("inbox %d", pulse.Inbox))
	}
	if pulse.Today > 0 {
		parts = append(parts, fmt.Sprintf("today %d", pulse.Today))
	}
	if pulse.Overdue > 0 {
		parts = append(parts, fmt.Sprintf("overdue %d", pulse.Overdue))
	}
	return strings.Join(parts, " · ")
}
