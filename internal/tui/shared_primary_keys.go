package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// primaryProjectView is the interface that primary views (ProjectTree, Kanban,
// Outliner) must implement to use the shared key dispatcher.
type primaryProjectView interface {
	getProject() *models.Project
	getCursorTask() *models.Task
	getConfig() *config.Config
	reloadFromDisk() tea.Cmd
	performUndo() tea.Cmd
}

// handleCommonPrimaryViewKey handles keys shared across the three primary
// project views (ProjectTree, Kanban, Outliner). It returns (cmd, true) when
// the key was consumed, or (nil, false) when the caller should continue with
// view-specific handling.
func handleCommonPrimaryViewKey(key string, v primaryProjectView) (tea.Cmd, bool) {
	project := v.getProject()

	switch key {
	// View switching
	case "ctrl+p":
		return func() tea.Msg { return ShowProjectTreeMsg{Project: project} }, true
	case "ctrl+k":
		return func() tea.Msg { return ShowKanbanMsg{Project: project} }, true
	case "ctrl+o":
		return func() tea.Msg { return ShowOutlinerMsg{Project: project} }, true

	// Project cycling
	case "`":
		return func() tea.Msg { return CycleProjectMsg{Forward: true} }, true
	case "~":
		return func() tea.Msg { return CycleProjectMsg{Forward: false} }, true
	case "@":
		return func() tea.Msg { return ShowProjectCycleManagerMsg{} }, true

	// Secondary views
	case "B":
		return func() tea.Msg { return ShowBlockersMsg{Project: project} }, true
	case "p":
		return func() tea.Msg { return ShowPriorityViewMsg{Project: project} }, true
	case "w":
		return func() tea.Msg { return ShowInboxProcessorMsg{Project: project} }, true

	// Capture
	case "i":
		cfg := v.getConfig()
		return func() tea.Msg {
			return ShowQuickCaptureMsg{Project: project, AllProjects: cfg.Projects}
		}, true
	case "I":
		return func() tea.Msg { return ShowBulkCaptureMsg{Project: project} }, true

	// Navigation / tools
	case "\\":
		return func() tea.Msg { return ShowTaskJumperMsg{Project: project} }, true
	case "{":
		return func() tea.Msg { return ShowNotesMsg{Project: project} }, true
	case "}":
		return func() tea.Msg { return ShowNoteCreatorMsg{Project: project} }, true

	// Reload from disk
	case "ctrl+r":
		return v.reloadFromDisk(), true

	// Edit last modified task
	case "ctrl+l":
		recentTasks := storage.GetRecentTasks(project.Tasks, 1)
		if len(recentTasks) > 0 {
			return func() tea.Msg { return EditTaskMsg{Task: recentTasks[0]} }, true
		}
		return nil, true

	// Undo
	case "u":
		return v.performUndo(), true
	case "ctrl+z":
		return func() tea.Msg { return ShowUndoHistoryMsg{Project: project} }, true

	// Score editor
	case "S":
		task := v.getCursorTask()
		if task != nil {
			return func() tea.Msg {
				return ShowScoreEditorMsg{Task: task, Project: project}
			}, true
		}
		return nil, true
	}

	return nil, false
}

// activatePrefixKey checks whether the given key activates a prefix key
// sequence shared across primary views. Returns the prefix string and true
// if the key is a recognized prefix activator, or ("", false) otherwise.
// cursorOnTask should be false when the cursor is on a non-task element
// (e.g. section header) and task-specific prefixes should be skipped.
func activatePrefixKey(key string, cursorOnTask bool) (string, bool) {
	switch key {
	case ",", "z", "g", "m", "!", "d", "s", "t", "b", "F":
		// Task-specific prefixes require a cursor task
		if !cursorOnTask {
			switch key {
			case ",", "!", "d", "s", "t", "b", "m":
				return "", false
			}
		}
		return key, true
	case "c":
		if cursorOnTask {
			return key, true
		}
		return "", false
	}
	return "", false
}

// handleSortKey handles shared sort key presses (1-5, 0). It calls
// SortFieldForKey and updates the provided sort state. Returns true if the
// key was consumed. The caller should call its own rebuild/refresh method
// after this returns true, using the onReset callback for sort-reset (key 0)
// and onToggle for field changes.
func handleSortKey(key string, sortField *SortField, ascending *bool, onReset func(), onToggle func(SortField)) bool {
	field, isReset, ok := SortFieldForKey(key)
	if !ok {
		return false
	}
	if isReset {
		*sortField = SortByDefault
		*ascending = true
		if onReset != nil {
			onReset()
		}
	} else {
		if *sortField == field {
			*ascending = !*ascending
		} else {
			*sortField = field
			*ascending = true
		}
		if onToggle != nil {
			onToggle(field)
		}
	}
	return true
}

// cycleCustomSectionTab handles [ and ] keys for cycling through custom
// section tabs with wraparound. Returns true if the key was handled.
// The afterCycle callback is called after the tab index changes so the
// caller can refresh its data.
func cycleCustomSectionTab(key string, tab *int, tabCount int, afterCycle func()) bool {
	if tabCount == 0 {
		return false
	}
	switch key {
	case "[":
		if *tab > 0 {
			*tab--
		} else {
			*tab = tabCount - 1
		}
		if afterCycle != nil {
			afterCycle()
		}
		return true
	case "]":
		if *tab < tabCount-1 {
			*tab++
		} else {
			*tab = 0
		}
		if afterCycle != nil {
			afterCycle()
		}
		return true
	}
	return false
}
