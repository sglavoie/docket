package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// resolveProjectForRow returns the project that owns the given row.
// In single-project mode it returns m.project; in cross-project mode it
// resolves via crossResolveProject using the row's ProjectID.
func (m *OutlinerModel) resolveProjectForRow(row outlinerRow) *models.Project {
	if m.project != nil {
		return m.project
	}
	return m.crossResolveProject(row.ProjectID)
}

// NavigateToTask navigates to a task in the current view. If the task is
// inside a collapsed parent, ancestors are expanded first.
func (m *OutlinerModel) NavigateToTask(task *models.Task) {
	m.navigateToTask(task)
}

func (m *OutlinerModel) navigateToTask(task *models.Task) bool {
	if task == nil {
		return false
	}
	projectID := ""
	if m.project != nil {
		projectID = m.project.ID
	} else {
		for _, project := range m.loadedProjects {
			if project != nil && project.GetTaskByID(task.ID) == task {
				projectID = project.ID
				break
			}
		}
	}
	for i, row := range m.rows {
		if row.task != nil && row.task.ID == task.ID && (m.project != nil || row.ProjectID == projectID) {
			m.cursor = i
			return true
		}
	}
	if projectID == "" {
		return false
	}

	if bucket := taskStateGroupLabel(task); bucket != "" && m.project != nil {
		delete(m.collapsedSections, bucket)
	}
	m.clearSectionZoom()
	m.ensureExpandedNodes()
	for ancestor := task.Parent; ancestor != nil; ancestor = ancestor.Parent {
		m.expandedNodes[TaskIdentity{ProjectID: projectID, TaskID: ancestor.ID}] = true
	}
	m.rebuildRows()

	for i, row := range m.rows {
		if row.task != nil && row.task.ID == task.ID && (m.project != nil || row.ProjectID == projectID) {
			m.cursor = i
			return true
		}
	}
	return false
}

// revealTask honors explicit task navigation even when persisted view
// state would hide the target. The initial attempt preserves the current view;
// only a genuinely hidden target resets visibility constraints.
func (m *OutlinerModel) revealTask(task *models.Task) {
	if m.navigateToTask(task) {
		return
	}

	m.focusStack = nil
	m.clearSectionZoom()
	m.filter = NewFilterState()
	m.clearSearchProjectionWithoutRestore()
	if task.IsTerminalState() {
		states, _ := models.NewWorkflowStateSet(task.State())
		m.filter.setStateFilter(states, true)
	}
	if task.IsDeferred() {
		m.filter.Deferred = true
	}
	m.selectOutlinerMode(ModeAll)
	for i, row := range m.rows {
		if row.task != nil && row.task.ID == task.ID {
			m.cursor = i
			return
		}
	}
	m.setTransientStatus("Task is unavailable in this project")
}

// NavigateToTaskInProject navigates to a task in the cross-project outliner.
// Project identity is part of the match so duplicate task IDs in different
// projects cannot move the cursor to the wrong row.
func (m *OutlinerModel) NavigateToTaskInProject(task *models.Task, project *models.Project) {
	if task == nil {
		return
	}
	if m.project != nil {
		m.NavigateToTask(task)
		return
	}

	projectID := ""
	if project != nil {
		projectID = project.ID
	}
	if projectID == "" {
		for _, p := range m.loadedProjects {
			if p != nil && p.GetTaskByID(task.ID) == task {
				projectID = p.ID
				break
			}
		}
	}
	if projectID == "" {
		m.setTransientStatus("Task project not found")
		return
	}

	if m.moveCursorToProjectTask(task.ID, projectID) {
		return
	}

	section := effectiveDisplaySection(task)
	delete(m.collapsedSections, "_xp:"+section)
	m.crossFocusStack = nil
	m.clearSectionZoom()
	m.ensureExpandedNodes()
	for ancestor := task.Parent; ancestor != nil; ancestor = ancestor.Parent {
		m.expandedNodes[TaskIdentity{ProjectID: projectID, TaskID: ancestor.ID}] = true
	}
	m.rebuildRows()

	if !m.moveCursorToProjectTask(task.ID, projectID) {
		m.setTransientStatus("Task is hidden by current filters")
	}
}

func (m *OutlinerModel) moveCursorToProjectTask(taskID, projectID string) bool {
	for i, row := range m.rows {
		if row.task != nil && row.ProjectID == projectID && row.task.ID == taskID {
			m.cursor = i
			return true
		}
	}
	return false
}

func (m *OutlinerModel) StatusBarLabel() string {
	if m.project != nil {
		return fmt.Sprintf("Outliner: %s", m.project.Name)
	}
	return m.crossStatusBarLabel()
}
