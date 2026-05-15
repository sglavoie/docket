package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
)

// TaskDetailModel is a full-screen read-only view of a single task.
type TaskDetailModel struct {
	BaseView
	InteractiveViewState
	task          *models.Task
	project       *models.Project
	config        *config.Config
	configManager *config.Manager

	scrollOffset      int
	totalLines        int
	mdRenderer        *glamour.TermRenderer
	renderedDesc      string
	renderedDescWidth int
	renderedDescTheme string

	// historyExpanded controls progressive disclosure of the History cluster
	// (Comments, Decisions, Attachments). When false, only a one-line summary
	// header is rendered. Toggled by `H`. Intentionally not persisted.
	historyExpanded bool

	// openLoops mirrors AppModel.openLoops for the title-row Zeigarnik counter.
	openLoops OpenLoops
}

// setOpenLoops injects the latest cross-project open-loop counts into the view.
func (m *TaskDetailModel) setOpenLoops(o OpenLoops) { m.openLoops = o }

// NewTaskDetailModel creates a new task detail view.
func NewTaskDetailModel(task *models.Task, project *models.Project, cfg *config.Config, mgr *config.Manager) *TaskDetailModel {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(0),
	)
	if err != nil {
		logging.Warn("glamour renderer init failed: %v", err)
	}
	return &TaskDetailModel{
		task:          task,
		project:       project,
		config:        cfg,
		configManager: mgr,
		mdRenderer:    renderer,
	}
}

func (m *TaskDetailModel) Init() tea.Cmd {
	return nil
}

func (m *TaskDetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)
	case TaskUpdatedMsg:
		// Re-fetch task pointer in case project was reloaded
		if msg.Task != nil {
			if fresh := m.project.GetTaskByID(msg.Task.ID); fresh != nil {
				m.task = fresh
				m.renderedDesc = ""
			} else if msg.Task.ID == m.task.ID {
				return m, func() tea.Msg { return BackMsg{} }
			}
		}
		return m, nil
	case TaskPropertyChangedMsg:
		if msg.Task != nil {
			if fresh := m.project.GetTaskByID(msg.Task.ID); fresh != nil {
				m.task = fresh
				m.renderedDesc = ""
			} else if msg.Task.ID == m.task.ID {
				return m, func() tea.Msg { return BackMsg{} }
			}
		}
		return m, nil
	}
	return m, nil
}

// ShouldCaptureEsc returns true so the detail view handles esc/q itself.
func (m *TaskDetailModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc()
}

// GetPrefixPopupInfo returns popup info when a prefix key is pending.
func (m *TaskDetailModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  m.prefixHints(),
	}
}

// prefixHints returns the prefix hint map for this view.
func (m *TaskDetailModel) prefixHints() PrefixHints {
	hints := PrefixHints{
		",": StatusPrefixHints,
		"!": BuildPriorityPrefixHints(0),
	}

	// Add status-aware active hints
	if m.task != nil {
		statusHints := make([]PrefixHint, len(StatusPrefixHints))
		copy(statusHints, StatusPrefixHints)
		for i := range statusHints {
			switch statusHints[i].Key {
			case "s":
				statusHints[i].Active = m.task.Status == models.StatusInProgress
			case "d":
				statusHints[i].Active = m.task.Status == models.StatusDone
			case "b":
				statusHints[i].Active = m.task.Status == models.StatusBlocked
			case "x":
				statusHints[i].Active = m.task.Status == models.StatusCanceled
			case "o":
				statusHints[i].Active = m.task.Status == models.StatusTodo
			case "r":
				statusHints[i].Active = m.task.Status == models.StatusReview
			}
		}
		hints[","] = statusHints
		hints["!"] = BuildPriorityPrefixHints(m.task.Priority)
	}

	return hints
}

// viewHeight returns the number of visible content lines (total minus title and footer).
func (m *TaskDetailModel) viewHeight() int {
	return max(m.height-TitleFooterOverhead, 1)
}

// clampScroll ensures scrollOffset is within valid bounds.
func (m *TaskDetailModel) clampScroll() {
	maxScroll := max(m.totalLines-m.viewHeight(), 0)
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m *TaskDetailModel) StatusBarLabel() string {
	if m.project != nil {
		return fmt.Sprintf("Task Detail: %s", m.project.Name)
	}
	return "Task Detail"
}
