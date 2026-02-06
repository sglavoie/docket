package tui

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleFilterBarKeys handles F - filter prefix popup activation.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleFilterBarKeys(key string) (bool, tea.Cmd) {
	// F is now handled as a prefix key in handleKeyMsg
	return false, nil
}

// handleFilterPrefix handles the second key after pressing F (filter prefix).
func (m *ProjectTreeModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	// Clear active saved view for filter changes (but not for Fv which opens saved views)
	if key != "v" {
		m.clearActiveSavedView()
	}
	switch key {
	case "c":
		// Toggle completed view
		if m.viewMode == ViewModeCompleted {
			m.viewMode = ViewModeActive
		} else {
			m.viewMode = ViewModeCompleted
		}
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return m, nil
	case "p":
		// Toggle high priority filter
		if m.filter.Priority == 0 {
			m.filter.Priority = 1
		} else {
			m.filter.Priority = 0
		}
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return m, nil
	case "t":
		// Open type sub-popup
		m.pendingPrefix = "Ft"
		return m, nil
	case "s":
		// Open status sub-popup
		m.pendingPrefix = "Fs"
		return m, nil
	case "#":
		// Tag filter selector
		return m, func() tea.Msg {
			return ShowTagFilterSelectorMsg{ProjectTree: m}
		}
	case "@":
		// Section filter selector
		return m, func() tea.Msg {
			return ShowSectionFilterSelectorMsg{ProjectTree: m}
		}
	case "d":
		// Open due-date sub-popup
		m.pendingPrefix = "Fd"
		return m, nil
	case "a":
		// Open attribute sub-popup
		m.pendingPrefix = "Fa"
		return m, nil
	case "r":
		// Toggle actionable/ready filter
		m.filter.Actionable = !m.filter.Actionable
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return m, nil
	case "S":
		// Open score threshold sub-popup
		m.pendingPrefix = "FS"
		return m, nil
	case "v":
		// Open saved views sub-prefix popup (s=save, l=load, d=delete)
		m.pendingPrefix = "Fv"
		return m, nil
	case "x":
		// Clear all filters
		m.filter.Type = ""
		m.filter.Status = ""
		m.filter.Priority = 0
		m.filter.Tags = make(map[string]bool)
		m.filter.Sections = make(map[string]bool)
		m.filter.DueDate = DueDateAll
		m.filter.Actionable = false
		m.filter.Attrs = make(map[string]string)
		m.filter.ScoreMin = 0
		if m.viewMode == ViewModeCompleted {
			m.viewMode = ViewModeActive
		}
		m.ClearSearch()
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return m, nil
	}
	return m, nil
}

// handleFilterTypePrefix handles the third key after pressing Ft (filter type sub-prefix).
func (m *ProjectTreeModel) handleFilterTypePrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "e":
		m.filter.Type = models.TaskTypeEpic
	case "s":
		m.filter.Type = models.TaskTypeStory
	case "t":
		m.filter.Type = models.TaskTypeTask
	case "b":
		m.filter.Type = models.TaskTypeBug
	case "i":
		m.filter.Type = models.TaskTypeIdea
	case "0":
		m.filter.Type = ""
	default:
		return m, nil
	}
	m.updateVisibility()
	m.pruneInvisibleSelections()
	m.clampCursor()
	return m, nil
}

// handleFilterStatusPrefix handles the third key after pressing Fs (filter status sub-prefix).
func (m *ProjectTreeModel) handleFilterStatusPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "t":
		m.filter.Status = models.StatusTodo
	case "s":
		m.filter.Status = models.StatusInProgress
	case "b":
		m.filter.Status = models.StatusBlocked
	case "d":
		m.filter.Status = models.StatusDone
	case "x":
		m.filter.Status = models.StatusCanceled
	case "0":
		m.filter.Status = ""
	default:
		return m, nil
	}
	m.updateVisibility()
	m.pruneInvisibleSelections()
	m.clampCursor()
	return m, nil
}

// handleFilterDuePrefix handles the third key after pressing Fd (filter due-date sub-prefix).
func (m *ProjectTreeModel) handleFilterDuePrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "o":
		m.filter.DueDate = DueDateOverdue
	case "t":
		m.filter.DueDate = DueDateToday
	case "w":
		m.filter.DueDate = DueDateThisWeek
	case "n":
		m.filter.DueDate = DueDateNone
	case "0":
		m.filter.DueDate = DueDateAll
	default:
		return m, nil
	}
	m.updateVisibility()
	m.pruneInvisibleSelections()
	m.clampCursor()
	return m, nil
}

// handleFilterAttrPrefix handles the second key after pressing Fa (filter attribute).
// Shows attribute categories: s=severity, r=reproducible, e=effort, c=category, v=value, w=worktype, o=scope, p=points, V=validated
func (m *ProjectTreeModel) handleFilterAttrPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "s":
		// Bug severity - open sub-popup
		m.pendingPrefix = "Fas"
		return m, nil
	case "r":
		// Bug reproducible - open sub-popup
		m.pendingPrefix = "Far"
		return m, nil
	case "e":
		// Effort (idea, task) - open sub-popup
		m.pendingPrefix = "Fae"
		return m, nil
	case "c":
		// Idea category - open sub-popup
		m.pendingPrefix = "Fac"
		return m, nil
	case "v":
		// Idea value - open sub-popup
		m.pendingPrefix = "Fav"
		return m, nil
	case "w":
		// Task worktype - open sub-popup
		m.pendingPrefix = "Faw"
		return m, nil
	case "o":
		// Epic scope - open sub-popup
		m.pendingPrefix = "Fao"
		return m, nil
	case "p":
		// Story points - open sub-popup
		m.pendingPrefix = "Fap"
		return m, nil
	case "V":
		// Idea validated - toggle
		if m.filter.Attrs["idea.validated"] == "true" {
			delete(m.filter.Attrs, "idea.validated")
		} else {
			m.filter.Attrs["idea.validated"] = "true"
			m.filter.Type = models.TaskTypeIdea // auto-set type filter
		}
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return m, nil
	case "0":
		// Clear attribute filters
		m.filter.Attrs = make(map[string]string)
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return m, nil
	}
	return m, nil
}

// handleFilterAttrSeverityPrefix handles bug severity value selection (Fas prefix).
func (m *ProjectTreeModel) handleFilterAttrSeverityPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "c":
		m.filter.Attrs["bug.severity"] = string(models.BugSeverityCritical)
		m.filter.Type = models.TaskTypeBug
	case "j":
		m.filter.Attrs["bug.severity"] = string(models.BugSeverityMajor)
		m.filter.Type = models.TaskTypeBug
	case "m":
		m.filter.Attrs["bug.severity"] = string(models.BugSeverityMinor)
		m.filter.Type = models.TaskTypeBug
	case "o":
		m.filter.Attrs["bug.severity"] = string(models.BugSeverityCosmetic)
		m.filter.Type = models.TaskTypeBug
	case "0":
		delete(m.filter.Attrs, "bug.severity")
	default:
		return m, nil
	}
	m.updateVisibility()
	m.pruneInvisibleSelections()
	m.clampCursor()
	return m, nil
}

// handleFilterAttrReproduciblePrefix handles bug reproducible value selection (Far prefix).
func (m *ProjectTreeModel) handleFilterAttrReproduciblePrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y":
		m.filter.Attrs["bug.reproducible"] = string(models.BugReproducibleYes)
		m.filter.Type = models.TaskTypeBug
	case "n":
		m.filter.Attrs["bug.reproducible"] = string(models.BugReproducibleNo)
		m.filter.Type = models.TaskTypeBug
	case "s":
		m.filter.Attrs["bug.reproducible"] = string(models.BugReproducibleSometimes)
		m.filter.Type = models.TaskTypeBug
	case "0":
		delete(m.filter.Attrs, "bug.reproducible")
	default:
		return m, nil
	}
	m.updateVisibility()
	m.pruneInvisibleSelections()
	m.clampCursor()
	return m, nil
}

// handleFilterAttrEffortPrefix handles effort value selection (Fae prefix).
func (m *ProjectTreeModel) handleFilterAttrEffortPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "s":
		m.filter.Attrs["effort"] = string(models.EffortSmall)
	case "m":
		m.filter.Attrs["effort"] = string(models.EffortMedium)
	case "l":
		m.filter.Attrs["effort"] = string(models.EffortLarge)
	case "u":
		m.filter.Attrs["effort"] = string(models.EffortUnknown)
	case "0":
		delete(m.filter.Attrs, "effort")
	default:
		return m, nil
	}
	m.updateVisibility()
	m.pruneInvisibleSelections()
	m.clampCursor()
	return m, nil
}

// handleFilterAttrCategoryPrefix handles idea category value selection (Fac prefix).
func (m *ProjectTreeModel) handleFilterAttrCategoryPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "f":
		m.filter.Attrs["idea.category"] = string(models.IdeaCategoryFeature)
		m.filter.Type = models.TaskTypeIdea
	case "i":
		m.filter.Attrs["idea.category"] = string(models.IdeaCategoryImprovement)
		m.filter.Type = models.TaskTypeIdea
	case "r":
		m.filter.Attrs["idea.category"] = string(models.IdeaCategoryRefactor)
		m.filter.Type = models.TaskTypeIdea
	case "s":
		m.filter.Attrs["idea.category"] = string(models.IdeaCategoryResearch)
		m.filter.Type = models.TaskTypeIdea
	case "0":
		delete(m.filter.Attrs, "idea.category")
	default:
		return m, nil
	}
	m.updateVisibility()
	m.pruneInvisibleSelections()
	m.clampCursor()
	return m, nil
}

// handleFilterAttrValuePrefix handles idea value selection (Fav prefix).
func (m *ProjectTreeModel) handleFilterAttrValuePrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "l":
		m.filter.Attrs["idea.value"] = string(models.IdeaValueLow)
		m.filter.Type = models.TaskTypeIdea
	case "m":
		m.filter.Attrs["idea.value"] = string(models.IdeaValueMedium)
		m.filter.Type = models.TaskTypeIdea
	case "h":
		m.filter.Attrs["idea.value"] = string(models.IdeaValueHigh)
		m.filter.Type = models.TaskTypeIdea
	case "0":
		delete(m.filter.Attrs, "idea.value")
	default:
		return m, nil
	}
	m.updateVisibility()
	m.pruneInvisibleSelections()
	m.clampCursor()
	return m, nil
}

// handleFilterAttrWorktypePrefix handles task worktype value selection (Faw prefix).
func (m *ProjectTreeModel) handleFilterAttrWorktypePrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "d":
		m.filter.Attrs["task.worktype"] = string(models.WorkTypeDevelopment)
		m.filter.Type = models.TaskTypeTask
	case "o":
		m.filter.Attrs["task.worktype"] = string(models.WorkTypeDocumentation)
		m.filter.Type = models.TaskTypeTask
	case "t":
		m.filter.Attrs["task.worktype"] = string(models.WorkTypeTesting)
		m.filter.Type = models.TaskTypeTask
	case "v":
		m.filter.Attrs["task.worktype"] = string(models.WorkTypeDevOps)
		m.filter.Type = models.TaskTypeTask
	case "e":
		m.filter.Attrs["task.worktype"] = string(models.WorkTypeDesign)
		m.filter.Type = models.TaskTypeTask
	case "0":
		delete(m.filter.Attrs, "task.worktype")
	default:
		return m, nil
	}
	m.updateVisibility()
	m.pruneInvisibleSelections()
	m.clampCursor()
	return m, nil
}

// handleFilterAttrScopePrefix handles epic scope value selection (Fao prefix).
func (m *ProjectTreeModel) handleFilterAttrScopePrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "d":
		m.filter.Attrs["epic.scope"] = string(models.EpicScopeDefined)
		m.filter.Type = models.TaskTypeEpic
	case "e":
		m.filter.Attrs["epic.scope"] = string(models.EpicScopeEvolving)
		m.filter.Type = models.TaskTypeEpic
	case "u":
		m.filter.Attrs["epic.scope"] = string(models.EpicScopeUndefined)
		m.filter.Type = models.TaskTypeEpic
	case "0":
		delete(m.filter.Attrs, "epic.scope")
	default:
		return m, nil
	}
	m.updateVisibility()
	m.pruneInvisibleSelections()
	m.clampCursor()
	return m, nil
}

// handleFilterAttrPointsPrefix handles story points value selection (Fap prefix).
func (m *ProjectTreeModel) handleFilterAttrPointsPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "1":
		m.filter.Attrs["story.points"] = "1"
		m.filter.Type = models.TaskTypeStory
	case "2":
		m.filter.Attrs["story.points"] = "2"
		m.filter.Type = models.TaskTypeStory
	case "3":
		m.filter.Attrs["story.points"] = "3"
		m.filter.Type = models.TaskTypeStory
	case "5":
		m.filter.Attrs["story.points"] = "5"
		m.filter.Type = models.TaskTypeStory
	case "8":
		m.filter.Attrs["story.points"] = "8"
		m.filter.Type = models.TaskTypeStory
	case "0":
		delete(m.filter.Attrs, "story.points")
	default:
		return m, nil
	}
	m.updateVisibility()
	m.pruneInvisibleSelections()
	m.clampCursor()
	return m, nil
}

// handleFilterScorePrefix handles the third key after pressing FS (filter score sub-prefix).
func (m *ProjectTreeModel) handleFilterScorePrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "1":
		m.filter.ScoreMin = 10
	case "2":
		m.filter.ScoreMin = 25
	case "3":
		m.filter.ScoreMin = 50
	case "4":
		m.filter.ScoreMin = 75
	case "5":
		m.filter.ScoreMin = 100
	case "6":
		m.filter.ScoreMin = 150
	case "7":
		m.filter.ScoreMin = 200
	case "8":
		m.filter.ScoreMin = 250
	case "9":
		m.filter.ScoreMin = 300
	case "c":
		// Apply persisted custom value
		if m.config != nil && m.config.ScoreFilterCustom != nil {
			m.filter.ScoreMin = *m.config.ScoreFilterCustom
		}
		// No-op if not set
	case "C":
		// Enter inline text input to set custom value
		m.enterScoreFilterInputMode()
		return m, nil
	case "0":
		m.filter.ScoreMin = 0
	default:
		return m, nil
	}
	m.updateVisibility()
	m.pruneInvisibleSelections()
	m.clampCursor()
	return m, nil
}

// enterScoreFilterInputMode initializes the score filter text input.
func (m *ProjectTreeModel) enterScoreFilterInputMode() {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "enter value"
	ti.CharLimit = 10
	ti.Width = 12
	if m.config != nil && m.config.ScoreFilterCustom != nil {
		ti.SetValue(fmt.Sprintf("%.0f", *m.config.ScoreFilterCustom))
	}
	ti.Focus()
	m.scoreFilterInput = ti
	m.scoreFilterInputMode = true
}

// handleScoreFilterInput handles key events while the score filter text input is active.
func (m *ProjectTreeModel) handleScoreFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		val, err := strconv.ParseFloat(m.scoreFilterInput.Value(), 64)
		if err == nil && val > 0 {
			// Persist to config
			m.config.ScoreFilterCustom = &val
			if m.configManager != nil {
				_ = m.configManager.Save(m.config)
			}
			// Apply as active filter
			m.filter.ScoreMin = val
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
		}
		m.scoreFilterInputMode = false
		return m, nil
	case "esc":
		m.scoreFilterInputMode = false
		return m, nil
	}
	var cmd tea.Cmd
	m.scoreFilterInput, cmd = m.scoreFilterInput.Update(msg)
	return m, cmd
}

// handleSavedViewPrefix handles the third key after pressing Fv (saved views sub-prefix).
func (m *ProjectTreeModel) handleSavedViewPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "s":
		// Save current view
		return m, func() tea.Msg {
			return ShowSaveViewDialogMsg{}
		}
	case "l":
		// Load/browse saved views
		return m, func() tea.Msg {
			return ShowCommandPaletteMsg{PreFilter: ":sv "}
		}
	case "d":
		// Delete a saved view (browse, then Tab → Delete)
		return m, func() tea.Msg {
			return ShowCommandPaletteMsg{PreFilter: ":sv "}
		}
	}
	return m, nil
}

// handleFilterKeys handles P, S, T, N - filter operations.
// Note: 't' for type filter cycling removed - use Ft instead (t is now task prefix).
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleFilterKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "P":
		m.clearActiveSavedView()
		if m.filter.Priority == 0 {
			m.filter.Priority = 1
		} else {
			m.filter.Priority = 0
		}
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil

	case "T":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			tasks := m.getSelectedTasks()
			return true, func() tea.Msg {
				return ShowBulkTagSelectorMsg{Tasks: tasks, Project: m.project}
			}
		}
		return true, func() tea.Msg {
			return ShowTagFilterSelectorMsg{ProjectTree: m}
		}

	case "N":
		return true, func() tea.Msg {
			return ShowSectionFilterSelectorMsg{ProjectTree: m}
		}

		// Note: Due date filter cycling removed from 'u' - use Fd instead.
		// 'u' is now Vim-style undo (handled in handleMiscActionKeys).
	}
	return false, nil
}

// toggleSort sets the sort field, toggling ascending/descending if already set.
func (m *ProjectTreeModel) toggleSort(field SortField) {
	m.clearActiveSavedView()
	if m.sortField == field {
		m.sortAscending = !m.sortAscending
	} else {
		m.sortField = field
		m.sortAscending = true
	}
	m.sortNodes()
	m.updateVisibility()
}

// sendTasksToActive moves selected tasks or cursor task to the Active view (clears section).
// Captures task states before changes to enable event-based workflow triggers.
func (m *ProjectTreeModel) sendTasksToActive(visible []*TreeNode) (bool, tea.Cmd) {
	// Capture task states before changes for event-based workflow triggers
	prevStates := captureTaskStates(m.project.Tasks)

	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		count := 0
		for id := range m.selectedTasks {
			if m.project.MutateTask(id, func(t *models.Task) {
				t.Section = ""
			}) {
				count++
			}
		}
		m.clearSelection()
		m.pendingUndoAction = "Move to Active"
		m.pendingUndoContext = fmt.Sprintf("%d tasks", count)
		return true, m.saveAndRefreshWithPrevStates(prevStates)
	}

	// Single task mode
	if !CursorInBounds(m.cursor, len(visible)) {
		return false, nil
	}
	taskID := visible[m.cursor].Task.ID
	task := m.project.GetTaskByID(taskID)
	if task == nil {
		return false, nil
	}
	m.project.MutateTask(taskID, func(t *models.Task) {
		t.Section = ""
	})
	m.pendingUndoAction = "Move to Active"
	m.pendingUndoContext = task.Title
	return true, m.saveAndRefreshWithPrevStates(prevStates)
}

// sendTasksToSection moves selected tasks or cursor task to the specified section.
// Captures task states before changes to enable event-based workflow triggers.
func (m *ProjectTreeModel) sendTasksToSection(section string, visible []*TreeNode) (bool, tea.Cmd) {
	// Capture task states before changes for event-based workflow triggers
	prevStates := captureTaskStates(m.project.Tasks)

	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		count := 0
		for id := range m.selectedTasks {
			if m.project.MutateTask(id, func(t *models.Task) {
				t.Section = section
			}) {
				count++
			}
		}
		m.clearSelection()
		m.pendingUndoAction = "Move to " + section
		m.pendingUndoContext = fmt.Sprintf("%d tasks", count)
		return true, m.saveAndRefreshWithPrevStates(prevStates)
	}

	// Single task mode
	if !CursorInBounds(m.cursor, len(visible)) {
		return false, nil
	}
	taskID := visible[m.cursor].Task.ID
	task := m.project.GetTaskByID(taskID)
	if task == nil {
		return false, nil
	}
	m.project.MutateTask(taskID, func(t *models.Task) {
		t.Section = section
	})
	m.pendingUndoAction = "Move to " + section
	m.pendingUndoContext = task.Title
	return true, m.saveAndRefreshWithPrevStates(prevStates)
}

// handleSortKeys handles 1, 2, 3, 4, 0 - sort operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleSortKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "1":
		m.toggleSort(SortByPriority)
		return true, nil
	case "2":
		m.toggleSort(SortByDueDate)
		return true, nil
	case "3":
		m.toggleSort(SortByCreatedAt)
		return true, nil
	case "4":
		m.toggleSort(SortByStatus)
		return true, nil
	case "5":
		m.toggleSort(SortByScore)
		return true, nil
	case "0":
		m.clearActiveSavedView()
		m.sortField = SortByDefault
		m.sortAscending = true
		m.buildTree()
		return true, nil
	}
	return false, nil
}

// handleTaskCRUDKeys handles enter, n, e, D - task create/read/update/delete operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleTaskCRUDKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "enter":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			m.clearSelection()
			return true, func() tea.Msg {
				return GlowViewMsg{Task: task}
			}
		}
		return true, nil

	case "i":
		return true, func() tea.Msg {
			return ShowQuickCaptureMsg{Project: m.project, AllProjects: m.config.Projects}
		}

	case "I":
		return true, func() tea.Msg {
			return ShowBulkCaptureMsg{Project: m.project}
		}

	case "e":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			return true, func() tea.Msg {
				return ShowBulkEditMsg{
					Tasks:   m.getSelectedTasks(),
					Project: m.project,
				}
			}
		}
		if CursorInBounds(m.cursor, len(visible)) {
			return true, func() tea.Msg {
				return EditTaskMsg{Task: visible[m.cursor].Task}
			}
		}
		return true, nil

	case "E":
		// Quick edit mode: inline editing with modifier syntax
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			m.enterQuickEditMode(task, false)
		}
		return true, nil

	}
	return false, nil
}

// handleClipboardKeys handles y, Y - clipboard operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleClipboardKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "y":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			tasks := m.getSelectedTasks()
			m.clearSelection()
			return true, func() tea.Msg {
				return CopyTasksTitleMsg{Tasks: tasks}
			}
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return CopyTaskTitleMsg{Task: task}
			}
		}
		return true, nil

	case "Y":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			tasks := m.getSelectedTasks()
			m.clearSelection()
			return true, func() tea.Msg {
				return CopyTasksDetailsMsg{Tasks: tasks}
			}
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return CopyTaskDetailsMsg{Task: task}
			}
		}
		return true, nil

	case "ctrl+y":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			tasks := m.getSelectedTasks()
			m.clearSelection()
			return true, func() tea.Msg {
				return CopyTasksTitleAndDescMsg{Tasks: tasks}
			}
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return CopyTaskTitleAndDescMsg{Task: task}
			}
		}
		return true, nil
	}
	return false, nil
}

// handleFilterOverlayKeys handles ctrl+f, ctrl+t, ctrl+s - filter/manager overlay operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleFilterOverlayKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "ctrl+f":
		return true, func() tea.Msg {
			return ShowFilterMsg{ProjectTree: m}
		}

	case "ctrl+t":
		return true, func() tea.Msg {
			return ShowTagManagerMsg{ProjectTree: m}
		}

	case "ctrl+s":
		return true, func() tea.Msg {
			return ShowSectionManagerMsg{ProjectTree: m}
		}
	}
	return false, nil
}

// handleTaskAttributeKeys handles *, ctrl+l - task attribute modification.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleTaskAttributeKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "*":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			count := len(m.selectedTasks)
			for _, task := range m.getSelectedTasks() {
				task.ToggleActive()
			}
			m.clearSelection()
			m.setUndoContext("Toggle Active", fmt.Sprintf("%d tasks", count))
			return true, m.saveAndRefresh()
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			task.ToggleActive()
			m.setUndoContext("Toggle Active", task.Title)
			return true, m.saveAndRefresh()
		}
		return true, nil

	case "ctrl+l":
		// Edit last modified task
		recentTasks := storage.GetRecentTasks(m.project.Tasks, 1)
		if len(recentTasks) > 0 {
			return true, func() tea.Msg {
				return EditTaskMsg{Task: recentTasks[0]}
			}
		}
		return true, nil
	}
	return false, nil
}

// handleMiscActionKeys handles z, R, ?, \, r, p - miscellaneous action operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleMiscActionKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "u":
		// Undo last change (Vim-style)
		return true, m.performUndo()

	case "ctrl+z":
		// Open undo history view
		return true, func() tea.Msg {
			return ShowUndoHistoryMsg{Project: m.project}
		}

	case "?":
		return true, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewProjectTree,
				ViewName:   "Project Tree",
				Items:      m.GetHelpItems(),
			}
		}

	case "\\":
		return true, func() tea.Msg {
			return ShowTaskJumperMsg{Project: m.project}
		}

	case "[":
		tabs := m.customSectionTabs()
		if tabs != nil {
			if m.customSectionTab > 0 {
				m.customSectionTab--
			} else {
				m.customSectionTab = len(tabs) - 1
			}
			m.updateVisibility()
			m.clampCursor()
		}
		return true, nil

	case "]":
		tabs := m.customSectionTabs()
		if tabs != nil {
			if m.customSectionTab < len(tabs)-1 {
				m.customSectionTab++
			} else {
				m.customSectionTab = 0
			}
			m.updateVisibility()
			m.clampCursor()
		}
		return true, nil

	case "{":
		return true, func() tea.Msg {
			return ShowNotesMsg{Project: m.project}
		}

	case "}":
		return true, func() tea.Msg {
			return ShowNoteCreatorMsg{Project: m.project}
		}

	case "ctrl+r":
		m.clearSelection()
		_ = storage.LoadProject(m.project)
		m.buildTree()
		return true, nil

	case "p":
		return true, func() tea.Msg {
			return ShowPriorityViewMsg{Project: m.project}
		}

	case "S":
		// Open score editor for selected task
		if len(visible) > 0 && m.cursor < len(visible) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return ShowScoreEditorMsg{Task: task, Project: m.project}
			}
		}
		return true, nil

	case "f":
		return true, func() tea.Msg {
			return ShowProjectFocusViewMsg{Project: m.project}
		}

	case "B":
		// Project blockers (capital B since lowercase b is now prefix)
		return true, func() tea.Msg {
			return ShowBlockersMsg{Project: m.project}
		}

	case "w":
		// Open inbox processor workflow
		return true, func() tea.Msg {
			return ShowInboxProcessorMsg{Project: m.project}
		}

	case "ctrl+k":
		// Open kanban board view
		return true, func() tea.Msg {
			return ShowKanbanMsg{Project: m.project}
		}

	case "A":
		// Archive completed tasks (only in Completed view)
		if m.viewMode == ViewModeCompleted {
			completedCount := m.project.CountCompletedTasks()
			if completedCount > 0 {
				m.confirmAction = ConfirmArchiveTasks
			}
		}
		return true, nil
	}
	return false, nil
}

// handleActionKeys routes action keys to specialized sub-handlers.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleActionKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	if handled, cmd := m.handleTaskCRUDKeys(key, visible); handled {
		return handled, cmd
	}
	if handled, cmd := m.handleClipboardKeys(key, visible); handled {
		return handled, cmd
	}
	if handled, cmd := m.handleFilterOverlayKeys(key); handled {
		return handled, cmd
	}
	if handled, cmd := m.handleTaskAttributeKeys(key, visible); handled {
		return handled, cmd
	}
	if handled, cmd := m.handleMiscActionKeys(key, visible); handled {
		return handled, cmd
	}
	return false, nil
}
