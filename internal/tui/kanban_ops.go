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

// ─── prefixDispatchView interface ─────────────────────────────────────────────

// selectMsg returns a message to navigate to the task in its project.
func (m *KanbanModel) selectMsg(task *models.Task, project *models.Project) tea.Msg {
	return KanbanSelectMsg{Task: task, Project: project}
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
	tasks := make([]*models.Task, 0, len(m.allCards))
	for _, card := range m.allCards {
		tasks = append(tasks, card.Task)
	}
	source := ViewKanban
	if m.crossProject {
		source = ViewCrossProjectKanban
	}
	return buildTagCmdFromTasks(tasks, m.filter.Tags, source, "Filter by Tags")
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
		case KanbanModeCustomSections:
			section := targetCol.ID
			if section == "unset" {
				section = ""
			}
			result = pf(ref.project, ref.task.ID, func(t *models.Task) { t.CustomSection = section })
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
	}

	n := len(selected)
	m.FinishBulk(&m.statusMsg, fmt.Sprintf("Moved %d %s → %s", n, pluralize(n, "task", "tasks"), targetCol.Label))
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

	case KanbanModeCustomSections:
		section := targetCol.ID
		if section == "unset" {
			section = ""
		}
		model, cmd = m.applyKanbanCustomSectionChange(project, task, section)

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

// applyKanbanCustomSectionChange sets the CustomSection field on a task.
func (m *KanbanModel) applyKanbanCustomSectionChange(project *models.Project, task *models.Task, section string) (tea.Model, tea.Cmd) {
	pf := workflowPersist(func() WorkflowSaveOptions {
		return BuildWorkflowOpts(m.getPersister(), project, m.config, m.configManager, nil, "", "", nil)
	})
	result := pf(project, task.ID, func(t *models.Task) {
		t.CustomSection = section
	})
	if result.Err != nil {
		m.setErr(result.Err)
		return m, nil
	}
	label := section
	if label == "" {
		label = "none"
	}
	m.setStatus(fmt.Sprintf("Custom section → %s", label))
	m.reloadData()
	m.clampCursor()
	return m, nil
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
