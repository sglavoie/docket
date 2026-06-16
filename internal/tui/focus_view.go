package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
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

	// Multi-select
	MultiSelectState

	// Inline preview
	previewMode bool

	// Show dismissed suggestions (Suggested tab)
	showDismissed bool
}

// NewFocusViewModel creates a new focus view model.
func NewFocusViewModel(cfg *config.Config, mgr *config.Manager, queue *[]FocusTask, queueIDs map[string]bool, dailyPlanIDs map[string]bool, dismissedIDs map[string]bool) *FocusViewModel {
	m := &FocusViewModel{
		ViewBundle:       ViewBundle{config: cfg, configManager: mgr},
		projectService:   NewProjectService(),
		queue:            queue,
		queueIDs:         queueIDs,
		dailyPlanIDs:     dailyPlanIDs,
		dismissedIDs:     dismissedIDs,
		FilterMixin:      FilterMixin{filter: newFilterState()},
		MultiSelectState: MultiSelectState{selectedTasks: make(map[string]bool)},
		tabs:             TabState{Names: focusTabNames},
	}
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	m.Dispatcher.ModalGuards = []keymap.ModalGuard{
		{
			Active: func() bool { return m.confirm.IsActive() },
			Handle: func(key string) (tea.Model, tea.Cmd, bool) {
				if confirmed, handled := m.HandleConfirmKey(key); handled {
					if confirmed {
						model, cmd := taskOpsDeleteTask(m)
						return model, cmd, true
					}
					return m, nil, true
				}
				return nil, nil, false
			},
		},
	}
	m.initFilterMixinWithDispatcher(m.Dispatcher, cfg)
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

func (m *FocusViewModel) resetViewScrollOffset() {
	m.viewScrollOffset = 0
}

// SetSize updates the view dimensions and resets the persisted scroll window
// because the visible row budget may have changed.
func (m *FocusViewModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	m.resetViewScrollOffset()
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

// clampCursor ensures cursor is within valid range.
func (m *FocusViewModel) clampCursor() {
	m.ClampCursorTo(len(m.filtered))
}

// escSteps defines the Esc dismissal cascade: multi-select → filters.
func (m *FocusViewModel) escSteps() []EscStep {
	return []EscStep{
		MultiSelectEscStep(func() bool { return m.multiSelectMode }, m.toggleMultiSelectMode),
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
	return mergeHelpItems(
		GetKeybindingRegistry().GetAppLevelForView(m.viewID()),
		helpItemsFromKeyMap(m.buildKeyMap()),
	)
}

func (m *FocusViewModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *FocusViewModel) StatusBarLabel() string { return "Backlog" }

// IsFullHeight returns true because the backlog view renders its own footer.
func (m *FocusViewModel) IsFullHeight() bool { return true }

// Multi-select helpers.

func (m *FocusViewModel) toggleMultiSelectMode() {
	m.ToggleMode(&m.statusMessage, "Multi-select: space=toggle, a=all, A=deselect")
}

func (m *FocusViewModel) toggleTaskSelection() {
	ft := m.getTaskAtCursor()
	if ft == nil {
		return
	}
	m.ToggleAndUpdateStatus(ft.Task.ID, &m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

func (m *FocusViewModel) selectAllVisible() {
	for _, ft := range m.filtered {
		m.selectedTasks[ft.Task.ID] = true
	}
	m.UpdateStatus(&m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

func (m *FocusViewModel) deselectAll() {
	m.ClearSelection()
	m.UpdateStatus(&m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

// getSelectedBulkItems returns selected tasks as bulkTaskItem slice.
func (m *FocusViewModel) getSelectedBulkItems() []bulkTaskItem {
	var items []bulkTaskItem
	for _, ft := range m.filtered {
		if m.selectedTasks[ft.Task.ID] {
			items = append(items, bulkTaskItem{Task: ft.Task, Project: ft.Project})
		}
	}
	return items
}
