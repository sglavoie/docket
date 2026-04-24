package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
)

var focusTabNames = []string{"Queue", "Suggested", "By Project", "By Due Date"}

const (
	focusTabQueue     = 0
	focusTabSuggested = 1
	focusTabByProject = 2
	focusTabByDueDate = 3
)

// FocusTask is an alias for focus.FocusTask.
type FocusTask = focus.FocusTask

// ScoredFocusTask is an alias for focus.ScoredFocusTask.
type ScoredFocusTask = focus.ScoredFocusTask

// FocusViewModel displays the "Backlog" engine for cross-project task suggestions.
type FocusViewModel struct {
	BaseView
	InteractiveViewState
	ViewBundle

	projectService *ProjectService

	// Tasks — all actionable tasks from all active projects
	tasks    []FocusTask
	filtered []FocusTask

	// Work queue (session-persistent, pointers to AppModel-held state)
	queue        *[]FocusTask
	queueIDs     map[string]bool
	dailyPlanIDs map[string]bool

	// Ranked suggestions
	ranked       []ScoredFocusTask
	dismissedIDs map[string]bool

	// Tabs
	tabs TabState

	// Sub-tabs
	subTab           int
	subTabRowFocused bool
	subTabNames      []string

	// Filter
	FilterMixin

	// Inline preview
	previewMode bool

	// Show dismissed suggestions (Suggested tab)
	showDismissed bool
}

// NewFocusViewModel creates a new focus view model.
func NewFocusViewModel(cfg *config.Config, mgr *config.Manager, queue *[]FocusTask, queueIDs map[string]bool, dailyPlanIDs map[string]bool, dismissedIDs map[string]bool) *FocusViewModel {
	m := &FocusViewModel{
		ViewBundle:     ViewBundle{config: cfg, configManager: mgr},
		projectService: NewProjectService(),
		queue:          queue,
		queueIDs:       queueIDs,
		dailyPlanIDs:   dailyPlanIDs,
		dismissedIDs:   dismissedIDs,
		FilterMixin:    FilterMixin{filter: newFilterState()},
		tabs:           TabState{Names: focusTabNames},
	}
	m.initFilterMixin(&m.pendingPrefix, cfg)
	m.loadTasks()
	return m
}

// getTaskAtCursor returns the task at the current cursor position.
func (m *FocusViewModel) getTaskAtCursor() *FocusTask {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	return &m.filtered[m.cursor]
}

func (m *FocusViewModel) Init() tea.Cmd {
	return nil
}
func (m *FocusViewModel) getTaskRef() (*models.Task, *models.Project) {
	ft := m.getTaskAtCursor()
	if ft == nil {
		return nil, nil
	}
	return ft.Task, ft.Project
}

func (m *FocusViewModel) getPersister() Persister           { return m.projectService }
func (m *FocusViewModel) getConfigManager() *config.Manager { return m.configManager }
func (m *FocusViewModel) reloadData()                       { m.loadTasks() }
func (m *FocusViewModel) setErr(err error)                  { m.err = err }
func (m *FocusViewModel) setStatus(msg string)              { m.statusMessage = msg }

// prefixDispatchView implementation.

func (m *FocusViewModel) selectMsg(task *models.Task, project *models.Project) tea.Msg {
	return FocusViewSelectMsg{Task: task, Project: project}
}

func (m *FocusViewModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	return m.handleFocusFilterPrefix(key)
}

func (m *FocusViewModel) handleFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	return m.handleFocusFilterSubPrefix(prefix, key)
}

// clampCursor ensures cursor is within valid range.
func (m *FocusViewModel) clampCursor() {
	m.ClampCursorTo(len(m.filtered))
}

// escSteps defines the Esc dismissal cascade: filters.
func (m *FocusViewModel) escSteps() []EscStep {
	return []EscStep{
		{Active: func() bool { return m.showDismissed }, Clear: func() {
			m.showDismissed = false
			m.recomputeSuggested()
			m.clampCursor()
		}},
		FilterClearEscStep(m.HasActiveFilters, func() {
			m.ClearAllFilters()
			m.applyFilter()
			m.clampCursor()
			m.setStatus("Filters cleared")
		}),
	}
}

// ShouldCaptureEsc returns true when prefix mode, filters, or confirm dialog are active.
func (m *FocusViewModel) ShouldCaptureEsc() bool {
	return SecondaryEscCapture(&m.InteractiveViewState, m.escSteps())
}

func (m *FocusViewModel) viewID() ViewType { return ViewFocusView }
func (m *FocusViewModel) viewName() string { return "Backlog" }

// GetHelpItems returns help items for this view.
func (m *FocusViewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewFocusView)
}

func (m *FocusViewModel) HandledKeys() []string {
	return concatKeys(
		listNavigationKeys(),
		statusPrefixKeys(),
		dueDatePrefixKeys(),
		linkedTaskPrefixKeys(),
		commonActionKeys(),
		[]string{
			"g", "gp",
			"enter", "J", "K", "F", "m", "t",
			"1", "2", "3", "4", "5",
			"w", "u", "esc",
		},
	)
}

func (m *FocusViewModel) StatusBarLabel() string { return "Backlog" }

// IsFullHeight returns true because the backlog view renders its own footer.
func (m *FocusViewModel) IsFullHeight() bool { return true }
