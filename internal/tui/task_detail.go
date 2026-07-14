package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TaskDetailModel is a full-screen read-only view of a single task.
type TaskDetailModel struct {
	BaseView
	InteractiveViewState
	task    *models.Task
	project *models.Project
	// runtime backs the domain-native scoring read (project scoring profile).
	runtime *config.RuntimeState

	// taskItem routes the detail view's own status/priority mutations through the
	// application-layer single-task coordinator. The shell wires it via
	// SetTaskItemCoordinator (taskItemWireable) from showView; a zero value is
	// only reachable in isolated tests that never dispatch a mutation.
	taskItem TaskItemCoordinator

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

// SetTaskItemCoordinator injects the application-layer single-task coordinator
// so the detail view's status/priority mutations commit through the ledger
// (taskItemWireable). Wired by the shell from showView/replaceView.
func (m *TaskDetailModel) SetTaskItemCoordinator(c TaskItemCoordinator) { m.taskItem = c }

// refreshAfterReload reloads the detail view's project from disk and re-resolves
// the shown task after a coordinator commit. The single-task coordinator commits
// to disk and reloads the shell's shared project pointer, but the detail view may
// hold a distinct (e.g. cross-project) pointer, so it reloads its own copy here.
// Called by the shell's refreshTaskViewsAfterSave after every committed mutation.
func (m *TaskDetailModel) refreshAfterReload() {
	if m.project == nil || m.task == nil {
		return
	}
	if err := storage.LoadProject(m.project); err != nil {
		logging.Warn("failed to reload project %s in task detail: %v", m.project.Name, err)
		return
	}
	if fresh := m.project.GetTaskByID(m.task.ID); fresh != nil {
		m.task = fresh
		m.renderedDesc = ""
	}
}

// NewTaskDetailModel creates a new task detail view.
func NewTaskDetailModel(task *models.Task, project *models.Project, runtime *config.RuntimeState) *TaskDetailModel {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(0),
	)
	if err != nil {
		logging.Warn("glamour renderer init failed: %v", err)
	}
	return &TaskDetailModel{
		task:       task,
		project:    project,
		runtime:    runtime,
		mdRenderer: renderer,
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
		// Re-fetch task pointer in case project was reloaded.
		// Only react to messages targeting the task currently shown — otherwise
		// a broadcast for an unrelated task would swap m.task out from under us.
		if msg.Task != nil && m.task != nil && msg.Task.ID == m.task.ID {
			if fresh := m.project.GetTaskByID(msg.Task.ID); fresh != nil {
				m.task = fresh
				m.renderedDesc = ""
			} else {
				return m, func() tea.Msg { return BackMsg{} }
			}
		}
		return m, nil
	case TaskPropertyChangedMsg:
		if msg.Task != nil && m.task != nil && msg.Task.ID == m.task.ID {
			if fresh := m.project.GetTaskByID(msg.Task.ID); fresh != nil {
				m.task = fresh
				m.renderedDesc = ""
			} else {
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

// IsFullHeight returns true because task detail renders its own footer.
func (m *TaskDetailModel) IsFullHeight() bool { return true }

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
		state := m.task.State()
		for i := range statusHints {
			// Each "," prefix key maps directly to the workflow state it applies;
			// the hint is active when the task is already in that state.
			if target, ok := StateForPrefixKey(statusHints[i].Key); ok {
				statusHints[i].Active = state == target
			}
		}
		hints[","] = statusHints
		hints["!"] = BuildPriorityPrefixHints(m.task.Priority)
	}

	return hints
}

// viewHeight returns the number of visible content lines after reserving the
// title and the task detail footer's actual wrapped height.
func (m *TaskDetailModel) viewHeight() int {
	return max(m.height-1-m.footerHeight(), 1)
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
