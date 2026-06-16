package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// flashMoveCmd sets flashCardID on the model for the given task and returns a
// tea.Cmd that clears it after a short delay.
func (m *KanbanModel) flashMoveCmd(taskID string) tea.Cmd {
	m.flashCardID = taskID
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg {
		return kanbanFlashClearMsg{}
	})
}

// ─── taskOpsView interface ────────────────────────────────────────────────────

func (m *KanbanModel) getTaskRef() (*models.Task, *models.Project) {
	card := m.getCursorCard()
	if card == nil {
		return nil, nil
	}
	return card.Task, card.Project
}

func (m *KanbanModel) getPersister() Persister           { return m.svc }
func (m *KanbanModel) getConfig() *config.Config         { return m.config }
func (m *KanbanModel) getConfigManager() *config.Manager { return m.configManager }

// reloadData reloads all loaded projects and rebuilds columns, preserving the
// active column by ID so it remains focused after data changes.
func (m *KanbanModel) reloadData() {
	var prevColID string
	if m.activeCol >= 0 && m.activeCol < len(m.columns) {
		prevColID = m.columns[m.activeCol].ID
	}
	m.allCards = loadKanbanItems(m.loadedProjects, m.svc)
	m.rebuildCols()
	if prevColID != "" {
		for i, col := range m.columns {
			if col.ID == prevColID {
				m.activeCol = i
				break
			}
		}
	}
	m.adjustHorizontalScroll()
}

// clampCursorAllCols clamps cursors and scroll offsets for every column.
// Called after a terminal resize to ensure nothing is out of bounds.
func (m *KanbanModel) clampCursorAllCols() {
	saved := m.activeCol
	for i := range m.columns {
		m.activeCol = i
		m.clampCursor()
	}
	m.activeCol = saved
}

// clampCursor ensures the cursor in the active column is within valid bounds.
func (m *KanbanModel) clampCursor() {
	if m.activeCol < 0 || m.activeCol >= len(m.columns) {
		return
	}
	col := &m.columns[m.activeCol]
	if len(col.Tasks) == 0 {
		col.Cursor = 0
		col.ScrollOffset = 0
		return
	}
	if col.Cursor < 0 {
		col.Cursor = 0
	}
	if col.Cursor >= len(col.Tasks) {
		col.Cursor = len(col.Tasks) - 1
	}
	m.clampScrollToVisible(m.activeCol)
}

func (m *KanbanModel) setErr(err error)     { m.statusMsg = err.Error() }
func (m *KanbanModel) setStatus(msg string) { m.statusMsg = msg }

func (m *KanbanModel) selectedTaskProjectRefs() []TaskProjectRef {
	if !m.multiSelectMode || m.SelectedCount() == 0 {
		return nil
	}

	refs := make([]TaskProjectRef, 0, m.SelectedCount())
	for _, col := range m.columns {
		for i := range col.Tasks {
			card := &col.Tasks[i]
			if card.Task == nil || card.Project == nil || !m.IsSelected(card.Task.ID) {
				continue
			}
			refs = append(refs, TaskProjectRef{Task: card.Task, Project: card.Project})
		}
	}
	return refs
}

func (m *KanbanModel) applySelectedBulkDueDate(days int, isClear bool) (tea.Model, tea.Cmd) {
	refs := m.selectedTaskProjectRefs()
	if len(refs) == 0 {
		if isClear {
			return taskOpsClearDueDate(m)
		}
		return taskOpsSetQuickDueDate(m, days)
	}

	dueDate, desc := dueDateFromDays(days, isClear)
	count, err := taskOpsApplyBulk(m, refs, "Set Due Date", func(project *models.Project, taskID string) bool {
		return project.MutateTask(taskID, func(t *models.Task) {
			if dueDate != nil {
				d := *dueDate
				t.DueDate = &d
			} else {
				t.DueDate = nil
			}
			t.HasTime = false
		})
	})
	if err != nil {
		m.setErr(err)
		return m, nil
	}

	msg := fmt.Sprintf("Due date set to %s on %d tasks", desc, count)
	if isClear {
		msg = fmt.Sprintf("Cleared due date on %d tasks", count)
	}
	m.FinishBulk(&m.statusMsg, msg)
	return m, nil
}

func (m *KanbanModel) applySelectedBulkSnooze(key string) (tea.Model, tea.Cmd) {
	snoozeDate, desc, ok := resolveQuickSnoozeDate(key)
	if !ok {
		return m, nil
	}

	refs := m.selectedTaskProjectRefs()
	if len(refs) == 0 {
		return taskOpsQuickSnooze(m, snoozeDate)
	}

	count, err := taskOpsApplyBulk(m, refs, "Snooze", func(project *models.Project, taskID string) bool {
		return project.MutateTask(taskID, func(t *models.Task) {
			if snoozeDate != nil {
				d := *snoozeDate
				t.SnoozedUntil = &d
			} else {
				t.SnoozedUntil = nil
			}
		})
	})
	if err != nil {
		m.setErr(err)
		return m, nil
	}

	msg := fmt.Sprintf("Snoozed %d tasks %s", count, desc)
	if snoozeDate == nil {
		msg = fmt.Sprintf("Cleared snooze on %d tasks", count)
	}
	m.FinishBulk(&m.statusMsg, msg)
	return m, nil
}

func (m *KanbanModel) runSelectedBulkAction(
	refs []TaskProjectRef,
	undoAction string,
	successMsg func(int) string,
	mutate func(project *models.Project, taskID string) bool,
) (tea.Model, tea.Cmd) {
	count, err := taskOpsApplyBulk(m, refs, undoAction, mutate)
	if err != nil {
		m.setErr(err)
		return m, nil
	}
	m.FinishBulk(&m.statusMsg, successMsg(count))
	return m, nil
}

func (m *KanbanModel) applySelectedBulkStatus(status models.TaskStatus) (tea.Model, tea.Cmd) {
	refs := m.selectedTaskProjectRefs()
	if len(refs) == 0 {
		return taskOpsChangeStatus(m, status)
	}

	return m.runSelectedBulkAction(refs, StatusVerb(status)+" Task", func(count int) string {
		return fmt.Sprintf("Changed %d tasks to %s", count, status)
	}, func(project *models.Project, taskID string) bool {
		task := project.GetTaskByID(taskID)
		if task == nil || task.CanTransitionTo(status) != "" {
			return false
		}
		if status == models.StatusDone && project.HasIncompleteDependencies(task) {
			return false
		}
		if status == models.StatusDone {
			for _, child := range task.Children {
				if child.Status != models.StatusDone && child.Status != models.StatusCanceled {
					return false
				}
			}
		}
		project.MutateTask(taskID, func(t *models.Task) {
			t.ApplyTransition(status)
		})
		return true
	})
}

func (m *KanbanModel) applySelectedBulkPriority(key string) (tea.Model, tea.Cmd) {
	priority, ok := PriorityForPrefixKey(key)
	if !ok {
		return m, nil
	}

	refs := m.selectedTaskProjectRefs()
	if len(refs) == 0 {
		return taskOpsChangePriority(m, key)
	}

	return m.runSelectedBulkAction(refs, "Set Priority", func(count int) string {
		return fmt.Sprintf("Set priority %d on %d tasks", priority, count)
	}, func(project *models.Project, taskID string) bool {
		task := project.GetTaskByID(taskID)
		if task == nil || task.Priority == priority {
			return false
		}
		project.MutateTask(taskID, func(t *models.Task) {
			t.Priority = priority
		})
		return true
	})
}

func (m *KanbanModel) applySelectedBulkStage(stage string) (tea.Model, tea.Cmd) {
	refs := m.selectedTaskProjectRefs()
	if len(refs) == 0 {
		return taskOpsChangeStage(m, stage)
	}

	return m.runSelectedBulkAction(refs, "Set Stage", func(count int) string {
		if stage == "" {
			return fmt.Sprintf("Cleared stage on %d tasks", count)
		}
		return fmt.Sprintf("Set stage on %d tasks", count)
	}, func(project *models.Project, taskID string) bool {
		task := project.GetTaskByID(taskID)
		if task == nil || task.Stage == stage {
			return false
		}
		project.MutateTask(taskID, func(t *models.Task) {
			t.Stage = stage
		})
		return true
	})
}

func (m *KanbanModel) applySelectedBulkSectionMove(section string) (tea.Model, tea.Cmd) {
	refs := m.selectedTaskProjectRefs()
	if len(refs) == 0 {
		return taskOpsSectionMove(m, section)
	}
	if _, _, ok := sameProjectTaskRefs(refs); !ok {
		m.setStatus("bulk section move not supported across projects")
		return m, nil
	}

	return m.runSelectedBulkAction(refs, "Move to Section", func(count int) string {
		return fmt.Sprintf("Moved %d tasks to %s", count, kanbanSectionStatusLabel(section))
	}, func(project *models.Project, taskID string) bool {
		task := project.GetTaskByID(taskID)
		if task == nil || task.InSection(section) {
			return false
		}
		project.MutateTask(taskID, func(t *models.Task) {
			assignSection(t, section)
		})
		if section != "" {
			project.EnsureSection(section)
		}
		return true
	})
}

func (m *KanbanModel) applySelectedBulkToggleActive() (tea.Model, tea.Cmd) {
	refs := m.selectedTaskProjectRefs()
	if len(refs) == 0 {
		return taskOpsToggleActive(m)
	}

	return m.runSelectedBulkAction(refs, "Toggle Active", func(count int) string {
		return fmt.Sprintf("Toggled active on %d tasks", count)
	}, func(project *models.Project, taskID string) bool {
		return project.MutateTask(taskID, func(t *models.Task) {
			t.ToggleActive()
		})
	})
}

func (m *KanbanModel) applySelectedBulkTogglePin() (tea.Model, tea.Cmd) {
	refs := m.selectedTaskProjectRefs()
	if len(refs) == 0 {
		return taskOpsTogglePinned(m)
	}

	return m.runSelectedBulkAction(refs, "Toggle Pin", func(count int) string {
		return fmt.Sprintf("Toggled pin on %d tasks", count)
	}, func(project *models.Project, taskID string) bool {
		return project.MutateTask(taskID, func(t *models.Task) {
			t.TogglePinned()
		})
	})
}

func (m *KanbanModel) applySelectedBulkToggleDim() (tea.Model, tea.Cmd) {
	refs := m.selectedTaskProjectRefs()
	if len(refs) == 0 {
		return taskOpsToggleDimmed(m)
	}

	return m.runSelectedBulkAction(refs, "Toggle Dim", func(count int) string {
		return fmt.Sprintf("Toggled dim on %d tasks", count)
	}, func(project *models.Project, taskID string) bool {
		return project.MutateTask(taskID, func(t *models.Task) {
			t.ToggleDimmed()
		})
	})
}

func (m *KanbanModel) applySelectedBulkDelete() (tea.Model, tea.Cmd) {
	if len(m.selectedTaskProjectRefs()) == 0 {
		return taskOpsDeleteTask(m)
	}
	m.setStatus("bulk delete not supported in Kanban")
	return m, nil
}

func kanbanSectionStatusLabel(section string) string {
	if section == "" {
		return "Active"
	}
	return section
}

// ─── commonActionView interface ───────────────────────────────────────────────

func (m *KanbanModel) viewID() ViewType {
	if m.crossProject {
		return ViewCrossProjectKanban
	}
	return ViewKanban
}

func (m *KanbanModel) viewName() string {
	if m.crossProject {
		return "Cross-Project Kanban"
	}
	return "Kanban"
}

func (m *KanbanModel) GetHelpItems() []HelpItem {
	return mergeHelpItems(
		GetKeybindingRegistry().GetAppLevelForView(m.viewID()),
		helpItemsFromKeyMap(m.buildKeyMap()),
	)
}

// JumpToTop moves the cursor to the top of the active column (gg).
func (m *KanbanModel) JumpToTop() tea.Cmd {
	if m.activeCol < 0 || m.activeCol >= len(m.columns) {
		return nil
	}
	col := &m.columns[m.activeCol]
	col.Cursor = 0
	col.ScrollOffset = 0
	return nil
}

// refreshAndClamp re-filters allCards and rebuilds columns without reloading
// from disk. Used after filter state changes.
func (m *KanbanModel) refreshAndClamp() {
	m.rebuildCols()
	m.clampCursor()
}

// buildTagCmd opens the fzf tag picker for the visible cards' tag set.
func (m *KanbanModel) buildTagCmd() tea.Cmd {
	source := ViewKanban
	if m.crossProject {
		source = ViewCrossProjectKanban
	}
	return buildTagCmdFromProjectTasks(m.loadedProjects, tasksFromCrossProjectCards(m.allCards), m.filter.Tags, source, "Filter by Tags", false)
}

// handleFilterPrefix handles the second key after pressing F (filter prefix).
func (m *KanbanModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	if cmd, handled := handleFilterPrefixForView(m, key); handled {
		return m, cmd
	}
	return m, nil
}

// handleFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fd, etc.).
func (m *KanbanModel) handleFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	handleFilterSubPrefixForView(m, prefix, key)
	return m, nil
}

// ─── H/L column move ──────────────────────────────────────────────────────────

// moveTaskLeft moves the focused task (or all selected tasks) one column left (H).
func (m *KanbanModel) moveTaskLeft() (tea.Model, tea.Cmd) {
	if m.multiSelectMode && m.SelectedCount() > 0 {
		return m.bulkMoveSelectedTasks(-1)
	}
	return m.moveTaskToAdjacentCol(-1)
}

// moveTaskRight moves the focused task (or all selected tasks) one column right (L).
func (m *KanbanModel) moveTaskRight() (tea.Model, tea.Cmd) {
	if m.multiSelectMode && m.SelectedCount() > 0 {
		return m.bulkMoveSelectedTasks(+1)
	}
	return m.moveTaskToAdjacentCol(+1)
}

// bulkMoveSelectedTasks moves all selected tasks to the adjacent column (delta = ±1).
func (m *KanbanModel) bulkMoveSelectedTasks(delta int) (tea.Model, tea.Cmd) {
	targetIdx := m.activeCol + delta
	if targetIdx < 0 || targetIdx >= len(m.columns) {
		return m, nil
	}
	targetCol := &m.columns[targetIdx]

	// Collect selected cards by iterating the visible column tasks so order is stable.
	type cardRef struct {
		task    *models.Task
		project *models.Project
	}
	var selected []cardRef
	for _, col := range m.columns {
		for i := range col.Tasks {
			card := &col.Tasks[i]
			if m.IsSelected(card.Task.ID) {
				selected = append(selected, cardRef{task: card.Task, project: card.Project})
			}
		}
	}
	if len(selected) == 0 {
		return m, nil
	}

	var lastCmd tea.Cmd
	moved := 0
	for _, ref := range selected {
		pf := workflowPersist(func() WorkflowSaveOptions {
			return BuildWorkflowOpts(m.getPersister(), ref.project, m.config, m.configManager, nil, "", "", nil)
		})
		var result TaskOpResult
		switch m.mode {
		case KanbanModeSection:
			result = applySectionChange(pf, ref.project, ref.task.ID, kanbanColIDToSection(targetCol.ID))
		case KanbanModeStatus:
			newStatus := models.TaskStatus(targetCol.ID)
			if reason := ref.task.CanTransitionTo(newStatus); reason != "" {
				continue // skip tasks that can't transition
			}
			result = applyStatusChange(pf, ref.project, ref.task.ID, newStatus)
		case KanbanModePriority:
			result = applyPriorityChange(pf, ref.project, ref.task.ID, kanbanColIDToPriority(targetCol.ID))
		case KanbanModeStages:
			stage := targetCol.ID
			if stage == "unset" {
				stage = ""
			}
			result = applyStageChange(pf, ref.project, ref.task.ID, stage)
		default:
			continue
		}
		if result.Err != nil {
			m.setErr(result.Err)
			continue
		}
		if result.Cmd != nil {
			lastCmd = result.Cmd
		}
		moved++
	}

	m.FinishBulk(&m.statusMsg, fmt.Sprintf("Moved %d %s → %s", moved, pluralize(moved, "task", "tasks"), targetCol.Label))
	m.reloadData()
	m.clampCursor()
	return m, lastCmd
}

// moveTaskToAdjacentCol moves the focused task to the column at activeCol+delta.
// The mode determines what task attribute is changed.
func (m *KanbanModel) moveTaskToAdjacentCol(delta int) (tea.Model, tea.Cmd) {
	card := m.getCursorCard()
	if card == nil {
		return m, nil
	}
	targetIdx := m.activeCol + delta
	if targetIdx < 0 || targetIdx >= len(m.columns) {
		return m, nil
	}
	targetCol := &m.columns[targetIdx]
	task := card.Task
	project := card.Project
	taskID := task.ID

	var model tea.Model
	var cmd tea.Cmd

	switch m.mode {
	case KanbanModeSection:
		section := kanbanColIDToSection(targetCol.ID)
		model, cmd = taskOpsSectionMove(m, section)

	case KanbanModeStatus:
		newStatus := models.TaskStatus(targetCol.ID)
		if reason := task.CanTransitionTo(newStatus); reason != "" {
			m.statusMsg = reason
			return m, nil
		}
		model, cmd = taskOpsChangeStatus(m, newStatus)

	case KanbanModePriority:
		newPriority := kanbanColIDToPriority(targetCol.ID)
		model, cmd = m.applyKanbanPriorityChange(project, task, newPriority)

	case KanbanModeStages:
		stage := targetCol.ID
		if stage == "unset" {
			stage = ""
		}
		model, cmd = taskOpsChangeStage(m, stage)

	case KanbanModeScoreBuckets:
		// Score is derived, not directly settable; open the score editor instead.
		return m, func() tea.Msg {
			return ShowScoreEditorMsg{Task: task, Project: project}
		}

	default:
		return m, nil
	}

	// Trigger flash highlight on the moved card in the target column.
	if km, ok := model.(*KanbanModel); ok {
		flashCmd := km.flashMoveCmd(taskID)
		return km, tea.Batch(cmd, flashCmd)
	}
	return model, cmd
}

// kanbanColIDToSection maps a section column ID back to a GTD section name.
// The empty string represents the Active (no-section) state.
func kanbanColIDToSection(id string) string {
	switch id {
	case "inbox":
		return models.SectionInbox
	case "next":
		return models.SectionNext
	case "waiting":
		return models.SectionWaiting
	case "someday":
		return models.SectionSomeday
	case "review":
		return models.SectionReview
	case "standby":
		return models.SectionStandby
	case "completed":
		return models.SectionCompleted
	default:
		return "" // Active = empty section
	}
}

// kanbanColIDToPriority maps a priority column ID to its numeric value.
// Returns 0 for "unset" (no priority).
func kanbanColIDToPriority(id string) int {
	switch id {
	case "p1":
		return 1
	case "p2":
		return 2
	case "p3":
		return 3
	case "p4":
		return 4
	case "p5":
		return 5
	}
	return 0
}

// applyKanbanPriorityChange applies a priority change via the workflow pipeline.
func (m *KanbanModel) applyKanbanPriorityChange(project *models.Project, task *models.Task, priority int) (tea.Model, tea.Cmd) {
	pf := workflowPersist(func() WorkflowSaveOptions {
		return BuildWorkflowOpts(m.getPersister(), project, m.config, m.configManager, nil, "", "", nil)
	})
	result := applyPriorityChange(pf, project, task.ID, priority)
	if result.Err != nil {
		m.setErr(result.Err)
	} else if result.StatusMessage != "" {
		m.setStatus(result.StatusMessage)
	}
	m.reloadData()
	m.clampCursor()
	return m, result.Cmd
}

// ─── J/K in-column reorder ────────────────────────────────────────────────────

// reorderCardUp swaps the cursor card with the card above it (K key).
// This is an in-memory reorder only and does not persist.
func (m *KanbanModel) reorderCardUp() {
	if m.activeCol < 0 || m.activeCol >= len(m.columns) {
		return
	}
	col := &m.columns[m.activeCol]
	if col.Cursor <= 0 || len(col.Tasks) < 2 {
		return
	}
	col.Tasks[col.Cursor], col.Tasks[col.Cursor-1] = col.Tasks[col.Cursor-1], col.Tasks[col.Cursor]
	col.Cursor--
	m.clampScrollToVisible(m.activeCol)
}

// reorderCardDown swaps the cursor card with the card below it (J key).
// This is an in-memory reorder only and does not persist.
func (m *KanbanModel) reorderCardDown() {
	if m.activeCol < 0 || m.activeCol >= len(m.columns) {
		return
	}
	col := &m.columns[m.activeCol]
	if col.Cursor >= len(col.Tasks)-1 || len(col.Tasks) < 2 {
		return
	}
	col.Tasks[col.Cursor], col.Tasks[col.Cursor+1] = col.Tasks[col.Cursor+1], col.Tasks[col.Cursor]
	col.Cursor++
	m.clampScrollToVisible(m.activeCol)
}
