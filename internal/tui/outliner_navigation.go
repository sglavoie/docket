package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
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
		for _, project := range m.population.projects {
			if project != nil && project.GetTaskByID(task.ID) == task {
				projectID = project.ID
				break
			}
		}
	}
	for i, row := range m.rows {
		if row.task != nil && row.task.ID == task.ID && (m.project != nil || row.ProjectID == projectID) {
			m.Cursor = i
			return true
		}
	}
	if projectID == "" {
		return false
	}

	if bucket := theme.TaskStateGroupLabel(task); bucket != "" && m.project != nil {
		delete(m.collapsedSections, bucket)
	}
	m.clearSectionZoom()
	m.ensureExpandedNodes()
	for ancestor := task.Parent; ancestor != nil; ancestor = ancestor.Parent {
		m.expandedNodes[taskref.TaskIdentity{ProjectID: projectID, TaskID: ancestor.ID}] = true
	}
	m.rebuildRows()

	for i, row := range m.rows {
		if row.task != nil && row.task.ID == task.ID && (m.project != nil || row.ProjectID == projectID) {
			m.Cursor = i
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
	m.clearSearchProjectionWithoutRestore()
	// The row facets are replaced wholesale by what the target needs to be
	// visible: the reveal writes clauses like any other facet, so the carriers,
	// the chips, and the predicate follow it.
	clauses := make([]models.QueryClause, 0, 2)
	if task.IsTerminalState() {
		clauses = query.AppendFilterClause(clauses, models.QueryFieldState,
			models.QueryOperatorIs, task.State().String())
	}
	if task.IsDeferred() {
		clauses = query.AppendFilterClause(clauses, models.QueryFieldDeferred,
			models.QueryOperatorPresent, "")
	}
	m.replaceFilterClauses(query.FilterQueryTaskFields(), clauses)
	m.selectOutlinerMode(chrome.ModeAll)
	for i, row := range m.rows {
		if row.task != nil && row.task.ID == task.ID {
			m.Cursor = i
			return
		}
	}
	m.SetTransientStatus("Task is unavailable in this project")
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
		for _, p := range m.population.projects {
			if p != nil && p.GetTaskByID(task.ID) == task {
				projectID = p.ID
				break
			}
		}
	}
	if projectID == "" {
		m.SetTransientStatus("Task project not found")
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
		m.expandedNodes[taskref.TaskIdentity{ProjectID: projectID, TaskID: ancestor.ID}] = true
	}
	m.rebuildRows()

	if !m.moveCursorToProjectTask(task.ID, projectID) {
		m.SetTransientStatus("Task is hidden by current filters")
	}
}

func (m *OutlinerModel) moveCursorToProjectTask(taskID, projectID string) bool {
	for i, row := range m.rows {
		if row.task != nil && row.ProjectID == projectID && row.task.ID == taskID {
			m.Cursor = i
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
