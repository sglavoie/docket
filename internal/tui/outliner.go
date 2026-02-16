package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// ZoomEntry records the state at a single zoom level so cursor position can
// be restored when zooming back out.
type ZoomEntry struct {
	TaskID      string // UUID of the task that was zoomed into
	CursorIndex int    // Cursor position at the moment of zoom-in
}

// OutlinerModel is the outliner view for navigating tasks in a hierarchical,
// zoomable tree. It implements the View interface as a standalone struct
// (not extending ProjectTreeModel) to allow independent evolution of
// navigation semantics, sort modes, and zoom-stack behavior.
type OutlinerModel struct {
	BaseView

	project       *models.Project
	config        *config.Config
	configManager *config.Manager

	zoomStack   []ZoomEntry
	multiSelect bool
	zoomNotice  string // Transient message shown when zoom target is removed

	cursor            int           // Index into rows (flat visible node list)
	rows              []outlinerRow // Flattened rows, rebuilt on each render
	activePerspective int           // 1-9 index into PerspectivePresets; 0 means "All" (key 1)
}

// NewOutlinerModel creates a new OutlinerModel for the given project.
func NewOutlinerModel(project *models.Project, cfg *config.Config, cfgManager *config.Manager) *OutlinerModel {
	return &OutlinerModel{
		project:       project,
		config:        cfg,
		configManager: cfgManager,
	}
}

// validateZoomStack walks the zoom stack from top to bottom, popping entries
// whose task no longer exists in the project. If any entries were removed, it
// sets a transient notice so the user knows their zoom target was deleted.
func (m *OutlinerModel) validateZoomStack() {
	if m.project == nil || len(m.zoomStack) == 0 {
		return
	}

	// Find the deepest valid entry (walking from the top).
	validDepth := -1
	for i := len(m.zoomStack) - 1; i >= 0; i-- {
		if m.project.GetTaskByID(m.zoomStack[i].TaskID) != nil {
			validDepth = i
			break
		}
	}

	if validDepth == len(m.zoomStack)-1 {
		return // Stack is fully valid, nothing to do.
	}

	if validDepth < 0 {
		// No valid ancestor — reset to root.
		m.zoomStack = m.zoomStack[:0]
		m.cursor = 0
		m.zoomNotice = "Zoom target was removed — returned to root"
	} else {
		// Pop to nearest valid ancestor, restore its cursor.
		m.cursor = m.zoomStack[validDepth].CursorIndex
		m.zoomStack = m.zoomStack[:validDepth+1]
		m.zoomNotice = "Zoom target was removed — returned to parent"
	}
}

// rebuildRows flattens the current task tree into the rows slice, applying the
// active perspective filter, and clamps the cursor to stay within bounds.
func (m *OutlinerModel) rebuildRows() {
	m.rows = m.rows[:0]
	if m.project == nil {
		m.clampCursor()
		return
	}

	// Ensure zoom stack entries still reference existing tasks.
	m.validateZoomStack()

	roots := m.project.GetRootTasks()

	// Scope to zoomed task's children when zoom stack is non-empty.
	if len(m.zoomStack) > 0 {
		topID := m.zoomStack[len(m.zoomStack)-1].TaskID
		if t := m.project.GetTaskByID(topID); t != nil {
			roots = t.Children
		}
	}

	p := m.currentPerspective()
	if p.Filter == nil {
		flattenTaskTree(roots, nil, &m.rows)
	} else {
		filtered := filterTaskTree(roots, p.Filter)
		flattenTaskTree(filtered, nil, &m.rows)
	}
	m.clampCursor()
}

// currentPerspective returns the active perspective preset.
// Defaults to the "All" perspective (index 0) when activePerspective is unset.
func (m *OutlinerModel) currentPerspective() Perspective {
	idx := m.activePerspective
	if idx < 1 || idx > 9 {
		idx = 1
	}
	p, _ := PerspectiveByKey(idx)
	return p
}

// filterTaskTree returns a copy of the task tree containing only tasks that
// match the predicate or have descendants that match. Parent tasks are kept
// visible when any child matches, preserving hierarchy context.
func filterTaskTree(tasks []*models.Task, predicate func(*models.Task) bool) []*models.Task {
	var result []*models.Task
	for _, task := range tasks {
		filteredChildren := filterTaskTree(task.Children, predicate)
		if predicate(task) || len(filteredChildren) > 0 {
			clone := *task
			clone.Children = filteredChildren
			result = append(result, &clone)
		}
	}
	return result
}

// clampCursor ensures the cursor stays within [0, len(rows)-1].
func (m *OutlinerModel) clampCursor() {
	if len(m.rows) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
}

// contentHeight returns the number of task rows visible on screen,
// accounting for the title header and optional breadcrumb line.
func (m *OutlinerModel) contentHeight() int {
	headerLines := 2 // title + blank line
	if len(m.zoomStack) > 0 {
		headerLines = 3 // title + breadcrumb + blank line
	}
	h := m.height - headerLines
	if h < 1 {
		return 1
	}
	return h
}

// Init implements tea.Model.
func (m *OutlinerModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *OutlinerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}
	return m, nil
}

// SetSize implements the View interface.
func (m *OutlinerModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
}
