package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// LinkedTaskRelation represents a task and its relationship to the source task.
type LinkedTaskRelation struct {
	Task         *models.Task    // nil if cross-project task not found (broken link)
	Project      *models.Project // nil for same-project links
	RelationType string          // "blocks", "blocked_by", "related"
	Direction    string          // "→", "←", "↔"
	TaskRef      string          // Original reference string (for cross-project navigation)
	IsMissing    bool            // True if the linked task/project couldn't be found
}

// LinkedTaskSelectorModel is a picker for navigating to linked tasks.
type LinkedTaskSelectorModel struct {
	BaseSearchableView
	sourceTask   *models.Task
	project      *models.Project
	runtime      *config.RuntimeState
	coordinator  LinkCoordinator
	relations    []LinkedTaskRelation
	filtered     []LinkedTaskRelation // filtered relations based on search
	cursor       int
	cursorFlash  bool
	scrollOffset int
}

// NewLinkedTaskSelectorModelWithConfig creates a new linked task selector,
// resolving cross-project task refs against the domain-native catalog snapshot;
// unlink mutations commit through the link coordinator.
func NewLinkedTaskSelectorModelWithConfig(task *models.Task, project *models.Project, runtime *config.RuntimeState, coordinator LinkCoordinator) *LinkedTaskSelectorModel {
	m := &LinkedTaskSelectorModel{
		sourceTask:  task,
		project:     project,
		runtime:     runtime,
		coordinator: coordinator,
	}
	m.InitSearchInput("Search linked tasks...", 50, 30)
	m.loadRelations()
	m.filtered = m.relations
	return m
}

// HasLinkedTasks returns true if there are any linked tasks.
func (m *LinkedTaskSelectorModel) HasLinkedTasks() bool {
	return len(m.relations) > 0
}

// GetSingleLinkedTask returns the only linked task if there's exactly one and it's not missing.
func (m *LinkedTaskSelectorModel) GetSingleLinkedTask() *models.Task {
	if len(m.relations) == 1 && !m.relations[0].IsMissing {
		return m.relations[0].Task
	}
	return nil
}

// refreshAfterReload rebinds the selector to the task pointer produced by a
// project reload and rebuilds its relation cache. Keep the previous source
// pointer when the task is temporarily absent (for example after undoing a
// cross-project move) so a later redo can resolve the same stable task ID.
func (m *LinkedTaskSelectorModel) refreshAfterReload(project *models.Project) {
	if project != nil {
		m.project = project
	}
	if m.project == nil || m.sourceTask == nil {
		m.relations = nil
		m.filtered = nil
		m.cursor = 0
		return
	}
	fresh := m.project.GetTaskByID(m.sourceTask.ID)
	if fresh == nil {
		m.relations = nil
		m.filtered = nil
		m.cursor = 0
		return
	}
	m.sourceTask = fresh
	m.loadRelations()
	m.filterRelations()
	m.cursor = ClampCursor(m.cursor, len(m.filtered))
}

// SetSize sets the view dimensions.
func (m *LinkedTaskSelectorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *LinkedTaskSelectorModel) Init() tea.Cmd {
	return nil
}

// HasActiveSearch returns true if search mode is active or a filter is applied.
func (m *LinkedTaskSelectorModel) HasActiveSearch() bool {
	return m.IsSearchMode() || m.SearchQuery() != ""
}

// HasTextInput returns true when in search mode.
func (m *LinkedTaskSelectorModel) HasTextInput() bool {
	return m.IsSearchMode()
}

// ShouldCaptureEsc returns true when search is active so esc clears search first.
func (m *LinkedTaskSelectorModel) ShouldCaptureEsc() bool {
	return m.HasActiveSearch()
}

func (m *LinkedTaskSelectorModel) StatusBarLabel() string { return "Linked Tasks" }
