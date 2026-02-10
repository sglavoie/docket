package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// TaskDetailModel is a full-screen read-only view of a single task.
type TaskDetailModel struct {
	task          *models.Task
	project       *models.Project
	config        *config.Config
	configManager *config.Manager

	scrollOffset int
	totalLines   int
	width        int
	height       int

	// Prefix key state for which-key style popups
	pendingPrefix string
}

// NewTaskDetailModel creates a new task detail view.
func NewTaskDetailModel(task *models.Task, project *models.Project, cfg *config.Config, mgr *config.Manager) *TaskDetailModel {
	return &TaskDetailModel{
		task:          task,
		project:       project,
		config:        cfg,
		configManager: mgr,
	}
}

func (m *TaskDetailModel) Init() tea.Cmd {
	return nil
}

func (m *TaskDetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case TaskUpdatedMsg:
		// Re-fetch task pointer in case project was reloaded
		if msg.Task != nil {
			if fresh := m.project.GetTaskByID(msg.Task.ID); fresh != nil {
				m.task = fresh
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *TaskDetailModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// ShouldCaptureEsc returns true so the detail view handles esc/q itself.
func (m *TaskDetailModel) ShouldCaptureEsc() bool {
	return m.pendingPrefix != ""
}

// HasTextInput returns false — no text inputs in this view.
func (m *TaskDetailModel) HasTextInput() bool {
	return false
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
		"!": PriorityPrefixHints,
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

		// Priority hints with active indicator
		priorityHints := make([]PrefixHint, len(PriorityPrefixHints))
		copy(priorityHints, PriorityPrefixHints)
		for i := range priorityHints {
			switch priorityHints[i].Key {
			case "1":
				priorityHints[i].Active = m.task.Priority == 1
			case "2":
				priorityHints[i].Active = m.task.Priority == 2
			case "3":
				priorityHints[i].Active = m.task.Priority == 3
			case "4":
				priorityHints[i].Active = m.task.Priority == 4
			case "5":
				priorityHints[i].Active = m.task.Priority == 5
			}
		}
		hints["!"] = priorityHints
	}

	return hints
}

// viewHeight returns the number of visible content lines (total minus title and footer).
func (m *TaskDetailModel) viewHeight() int {
	h := m.height - 2 // title bar + footer
	if h < 1 {
		h = 1
	}
	return h
}

// clampScroll ensures scrollOffset is within valid bounds.
func (m *TaskDetailModel) clampScroll() {
	maxScroll := m.totalLines - m.viewHeight()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}
