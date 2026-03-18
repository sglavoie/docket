package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// crossResolveProject returns the loaded project matching the given ID by
// scanning m.loadedProjects. Returns nil when not found.
func (m *OutlinerModel) crossResolveProject(projectID string) *models.Project {
	for _, p := range m.loadedProjects {
		if p.ID == projectID {
			return p
		}
	}
	return nil
}

// crossGetMutationTarget returns the cursor row, its resolved owning project,
// and ok=true when a valid single-task mutation target exists:
//   - cross-project mode (m.project == nil)
//   - cursor within bounds
//   - row is not a separator or section header
//   - row has a non-nil task
//   - ProjectID resolves to a loaded project
func (m *OutlinerModel) crossGetMutationTarget() (row outlinerRow, proj *models.Project, ok bool) {
	if m.project != nil || len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return outlinerRow{}, nil, false
	}
	row = m.rows[m.cursor]
	if row.IsSeparator || row.IsSectionHeader || row.task == nil {
		return outlinerRow{}, nil, false
	}
	proj = m.crossResolveProject(row.ProjectID)
	if proj == nil {
		return outlinerRow{}, nil, false
	}
	return row, proj, true
}

// crossPersistFn returns a persistFn scoped to the given project. It calls
// Backup() on the project before the mutation and Save() after, ensuring only
// the owning project is written to disk.
func (m *OutlinerModel) crossPersistFn(_ *models.Project) persistFn {
	svc := m.projectService
	return func(project *models.Project, taskID string, mutate func(*models.Task)) TaskOpResult {
		if project.GetTaskByID(taskID) == nil {
			return TaskOpResult{StatusMessage: "Task no longer exists in project"}
		}
		if err := svc.Backup(project); err != nil {
			return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
		}
		project.MutateTask(taskID, mutate)
		if err := svc.Save(project); err != nil {
			return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
		}
		_ = svc.Load(project)
		return TaskOpResult{}
	}
}

// crossHandleStatusMutation resolves the owning project from the cursor row,
// applies a status transition, and forces a row rebuild via rebuildRows.
// It is called from handlePrefixKey when prefix="," and m.project==nil.
//
// In multi-select mode with selections active, routes to crossApplyBulkStatusChange.
//
// Returns (model, cmd, true) when the key maps to a status and was consumed.
// Returns (nil, nil, false) when not in cross-project mode or no status mapping.
func (m *OutlinerModel) crossHandleStatusMutation(key string) (tea.Model, tea.Cmd, bool) {
	if m.project != nil {
		return nil, nil, false
	}
	status, ok := StatusForPrefixKey(key)
	if !ok {
		return nil, nil, false
	}
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		model, cmd := m.crossApplyBulkStatusChange(status)
		return model, cmd, true
	}
	row, proj, valid := m.crossGetMutationTarget()
	if !valid {
		return m, nil, true
	}
	pf := m.crossPersistFn(proj)
	result := applyStatusChange(pf, proj, row.task.ID, status)
	if result.Err != nil {
		return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.crossUndoCtx.Set(StatusVerb(status)+" Task", row.task.Title)
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}
	m.rebuildRows()
	return m, result.Cmd, true
}

// crossApplyBulkStatusChange collects selected tasks from current rows, shows a
// confirmation dialog with the aggregate count, and stores the pending operation
// in m.bulkOp for execution after the user confirms.
func (m *OutlinerModel) crossApplyBulkStatusChange(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	var tasks []*models.Task
	for _, row := range m.rows {
		if row.IsSeparator || row.IsSectionHeader || row.task == nil {
			continue
		}
		if m.selectedTasks[row.task.ID] {
			tasks = append(tasks, row.task)
		}
	}
	if len(tasks) == 0 {
		return m, nil
	}
	m.bulkOp.SetOperation(newStatus, tasks, 0, nil)
	m.confirm.Show("bulk-status", fmt.Sprintf("Change %d tasks to %s?", len(tasks), string(newStatus)))
	m.clearMultiSelect()
	return m, nil
}

// crossExecutePendingBulkStatus applies the pending cross-project bulk status
// change after the user confirms the dialog. It groups the pending tasks by
// project, calls Backup() then Save() per project, and halts on first failure
// without processing remaining projects. All affected project IDs and backup
// paths are stored in m.crossUndoCtx for undo.
func (m *OutlinerModel) crossExecutePendingBulkStatus() (tea.Model, tea.Cmd) {
	if len(m.bulkOp.GetTasks()) == 0 {
		m.bulkOp.Clear()
		return m, nil
	}

	status := m.bulkOp.GetStatus()
	tasks := m.bulkOp.GetTasks()

	// Group tasks by project.
	type projectGroup struct {
		proj  *models.Project
		tasks []*models.Task
	}
	grouped := make(map[string]*projectGroup)
	var projectOrder []string
	for _, task := range tasks {
		proj := m.crossFindProjectForTask(task.ID)
		if proj == nil {
			continue
		}
		if _, exists := grouped[proj.ID]; !exists {
			grouped[proj.ID] = &projectGroup{proj: proj}
			projectOrder = append(projectOrder, proj.ID)
		}
		grouped[proj.ID].tasks = append(grouped[proj.ID].tasks, task)
	}

	svc := m.projectService
	var projectIDs []string

	for _, projID := range projectOrder {
		group := grouped[projID]
		if err := svc.Backup(group.proj); err != nil {
			m.bulkOp.Clear()
			return m, func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("backup failed: %w", err)} }
		}
		projectIDs = append(projectIDs, projID)
		for _, task := range group.tasks {
			group.proj.MutateTask(task.ID, func(t *models.Task) {
				t.ApplyTransition(status)
			})
		}
		if err := svc.Save(group.proj); err != nil {
			m.bulkOp.Clear()
			return m, func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("save failed: %w", err)} }
		}
		_ = svc.Load(group.proj)
	}

	m.crossUndoCtx.Set("Bulk Status Change", fmt.Sprintf("%d tasks -> %s", len(tasks), status))
	m.crossUndoCtx.ProjectIDs = projectIDs

	m.statusMessage = fmt.Sprintf("Changed %d tasks to %s", len(tasks), status)
	m.bulkOp.Clear()
	m.rebuildRows()
	return m, nil
}

// crossHandlePriorityMutation resolves the owning project from the cursor row,
// applies a priority change, and forces a row rebuild via rebuildRows.
// It is called from handlePrefixKey when prefix="!" and m.project==nil.
//
// Returns (cmd, true) when key maps to a priority level and was consumed.
// Returns (nil, false) when no priority mapping exists for the key.
func (m *OutlinerModel) crossHandlePriorityMutation(key string) (tea.Cmd, bool) {
	priority, ok := PriorityForPrefixKey(key)
	if !ok {
		return nil, false
	}
	row, proj, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, true
	}
	pf := m.crossPersistFn(proj)
	result := applyPriorityChange(pf, proj, row.task.ID, priority)
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.crossUndoCtx.Set("Set Priority", row.task.Title)
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}
	m.rebuildRows()
	return result.Cmd, true
}

// crossHandleSnoozeMutation resolves the owning project from the cursor row,
// applies a snooze date mutation, and forces a row rebuild via rebuildRows.
// It is called from handlePrefixKey when prefix="s" and m.project==nil.
//
// "sd" emits SetSnoozeDateMsg (date picker). All other recognized snooze keys
// mutate in-place, call Backup() then Save(), and rebuild rows synchronously.
//
// Returns (cmd, true) when the key was consumed.
// Returns (nil, false) when the key is not a recognized snooze sub-key.
func (m *OutlinerModel) crossHandleSnoozeMutation(key string) (tea.Cmd, bool) {
	if key == "d" {
		row, proj, valid := m.crossGetMutationTarget()
		if !valid {
			return nil, true
		}
		task := row.task
		return func() tea.Msg {
			return SetSnoozeDateMsg{Task: task, Project: proj}
		}, true
	}

	snoozeDate, desc, ok := resolveQuickSnoozeDate(key)
	if !ok {
		return nil, false
	}
	row, proj, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, true
	}

	if err := m.projectService.Backup(proj); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("backup failed: %w", err)} }, true
	}
	proj.MutateTask(row.task.ID, func(t *models.Task) {
		t.SnoozedUntil = snoozeDate
	})
	if err := m.projectService.Save(proj); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("save failed: %w", err)} }, true
	}
	_ = m.projectService.Load(proj)

	m.crossUndoCtx.Set("Snooze", row.task.Title)
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}

	if snoozeDate == nil {
		m.statusMessage = "Snooze cleared"
	} else {
		m.statusMessage = fmt.Sprintf("Snoozed until %s (%s)", snoozeDate.Format("Mon Jan 2"), desc)
	}
	m.rebuildRows()
	return nil, true
}

// crossHandleTagsMutation emits ShowTagSelectorMsg with the resolved owning
// project so the tag selector can save changes to the correct project file.
// It is called from handlePrefixKey when prefix="t", key="t" and m.project==nil.
//
// Returns (cmd, true) when a valid target exists, (nil, false) otherwise.
func (m *OutlinerModel) crossHandleTagsMutation() (tea.Cmd, bool) {
	row, proj, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, false
	}
	task := row.task
	tags := append([]string{}, task.Tags...)
	return func() tea.Msg {
		return ShowTagSelectorMsg{Task: task, Project: proj, CurrentTags: tags}
	}, true
}

// crossFindProjectForTask scans m.loadedProjects to find the project that
// contains the task with the given ID. Returns nil when not found.
func (m *OutlinerModel) crossFindProjectForTask(taskID string) *models.Project {
	for _, p := range m.loadedProjects {
		if p.GetTaskByID(taskID) != nil {
			return p
		}
	}
	return nil
}

// crossExecuteDelete performs a delete of m.deleteTarget in cross-project mode.
// It resolves the owning project via crossFindProjectForTask, calls Backup()
// and Save() on that project only, and forces a row rebuild via rebuildRows.
//
// Called from handleConfirmDialog when m.project==nil and the confirmed action
// is "delete" or "delete-recurring".
func (m *OutlinerModel) crossExecuteDelete() (tea.Model, tea.Cmd) {
	if m.deleteTarget == nil {
		return m, nil
	}
	task := m.deleteTarget
	m.deleteTarget = nil
	m.statusMessage = ""

	proj := m.crossFindProjectForTask(task.ID)
	if proj == nil {
		m.statusMessage = "Project for task not found"
		return m, nil
	}

	pf := m.crossPersistFn(proj)
	result := applyDeleteTask(pf, proj, task.ID, m.config.DataDir)
	if result.Err != nil {
		m.err = result.Err
		return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
	}
	m.crossUndoCtx.Set("Delete Task", task.Title)
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}
	m.rebuildRows()
	return m, result.Cmd
}
