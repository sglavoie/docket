package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
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

func (m *KanbanModel) getRuntime() *config.RuntimeState  { return m.runtime }
func (m *KanbanModel) getConfigManager() *config.Manager { return m.configManager }

// SetBulkCoordinator injects the application-layer bulk coordinator. The shell
// wires it from the single showView/replaceView site via the bulkWireable
// interface, so no Kanban construction path can forget to inject it.
func (m *KanbanModel) SetBulkCoordinator(c BulkCoordinator) { m.bulk = c }

// SetTaskItemCoordinator injects the application-layer single-task coordinator so
// the single-card priority move commits through the application layer. The shell
// wires it from the same showView/replaceView site via the taskItemWireable
// interface.
func (m *KanbanModel) SetTaskItemCoordinator(c TaskItemCoordinator) { m.taskItem = c }

// enabledWorkflowRules returns the workspace's enabled workflow rules, or nil
// when no config is bound (isolated render tests).
func (m *KanbanModel) enabledWorkflowRules() []workflow.WorkflowRule {
	if m.runtime == nil {
		return nil
	}
	return m.runtime.Automation().GetEnabledWorkflowRules()
}

// workflowOpsView accessors: Kanban does not embed ViewBundle, so it exposes its
// own single-task coordinator and enabled rules for the shared wf* helpers that
// back its single-card (non-selection) actions.
func (m *KanbanModel) taskItemCoordinator() TaskItemCoordinator { return m.taskItem }
func (m *KanbanModel) workflowRules() []workflow.WorkflowRule   { return m.enabledWorkflowRules() }

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
			return m, wfClearDueDate(m)
		}
		return m, wfSetQuickDueDate(m, days)
	}

	if isClear {
		return m, bulkListClearDueDateCmd(m.bulk, refs)
	}
	return m, bulkListDueDateCmd(m.bulk, refs, days)
}

func (m *KanbanModel) applySelectedBulkSnooze(key string) (tea.Model, tea.Cmd) {
	snoozeDate, _, ok := resolveQuickSnoozeDate(key)
	if !ok {
		return m, nil
	}

	refs := m.selectedTaskProjectRefs()
	if len(refs) == 0 {
		return m, wfQuickSnooze(m, snoozeDate)
	}

	return m, bulkListSnoozeCmd(m.bulk, refs, snoozeDate)
}

// runSelectedBulkAction applies an arbitrary in-memory task mutation (toggle
// active/pin/dim) to every selected task, grouped by owning project, then commits
// each touched project through the workflow-aware coordinator. The view sets its
// own success status synchronously; the shell owns reload via the async batch
// result (empty summary so it does not blank the status the view just set).
func (m *KanbanModel) runSelectedBulkAction(
	refs []TaskProjectRef,
	undoAction string,
	successMsg func(int) string,
	mutate func(project *models.Project, taskID string) bool,
) (tea.Model, tea.Cmd) {
	count := 0
	var changedProjects []*models.Project
	for _, group := range groupTaskProjectRefsByProject(refs) {
		project := group.Project
		changed := false
		for _, item := range group.Items {
			if mutate(project, item.Task.ID) {
				count++
				changed = true
			}
		}
		if changed {
			changedProjects = append(changedProjects, project)
		}
	}

	m.FinishBulk(&m.statusMsg, successMsg(count))
	return m, m.commitChangedProjects(changedProjects, undoAction, count)
}

// commitChangedProjects dispatches one workflow-aware commit per touched project,
// batching the resulting async commands. It returns nil when nothing changed so an
// unwired coordinator (isolated render tests) is never invoked.
func (m *KanbanModel) commitChangedProjects(projects []*models.Project, undoAction string, count int) tea.Cmd {
	if len(projects) == 0 {
		return nil
	}
	opts := app.ProjectWorkflowOptions{
		Rules:       m.enabledWorkflowRules(),
		UndoAction:  undoAction,
		UndoContext: fmt.Sprintf("%d tasks", count),
	}
	cmds := make([]tea.Cmd, 0, len(projects))
	for _, project := range projects {
		cmds = append(cmds, m.taskItem.CommitProjectWorkflow(project, opts, ""))
	}
	// A single touched project (the common non-cross-project board) returns its
	// command directly so the shell receives one BatchTransactionResultMsg; only a
	// genuine cross-project selection needs the tea.Batch fan-out.
	if len(cmds) == 1 {
		return cmds[0]
	}
	return tea.Batch(cmds...)
}

func (m *KanbanModel) applySelectedBulkState(state models.WorkflowState) (tea.Model, tea.Cmd) {
	refs := m.selectedTaskProjectRefs()
	if len(refs) == 0 {
		return m, wfChangeState(m, state)
	}

	return m, bulkListStateCmd(m.bulk, m.enabledWorkflowRules(), refs, state)
}

func (m *KanbanModel) applySelectedBulkPriority(key string) (tea.Model, tea.Cmd) {
	priority, ok := PriorityForPrefixKey(key)
	if !ok {
		return m, nil
	}

	refs := m.selectedTaskProjectRefs()
	if len(refs) == 0 {
		return m, wfChangePriority(m, key)
	}

	return m, bulkListPriorityCmd(m.bulk, refs, priority)
}

func (m *KanbanModel) applySelectedBulkSectionMove(section string) (tea.Model, tea.Cmd) {
	refs := m.selectedTaskProjectRefs()
	if len(refs) == 0 {
		return m, wfSectionMove(m, section)
	}

	// Sections are retired: a "send to section X" bulk action is a workflow-state
	// transition committed through the coordinator, which handles cross-project
	// selections natively (one atomic transaction), so the former same-project
	// restriction no longer applies.
	cmd, ok := bulkListSectionMoveCmd(m.bulk, m.enabledWorkflowRules(), refs, section)
	if !ok {
		m.setStatus(fmt.Sprintf("Unknown section %q", section))
	}
	return m, cmd
}

func (m *KanbanModel) applySelectedBulkTogglePin() (tea.Model, tea.Cmd) {
	refs := m.selectedTaskProjectRefs()
	if len(refs) == 0 {
		return m, wfTogglePin(m)
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
		return m, wfToggleDim(m)
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
		// Single-card delete commits the subtree through the coordinator-backed
		// workflow path (folding durable planning-ref cleanup into the same
		// transaction); the shell reloads the board from the async
		// BatchTransactionResultMsg.
		return m, wfDeleteTask(m, m.getRuntime(), m.getConfigManager())
	}
	m.setStatus("bulk delete not supported in Kanban")
	return m, nil
}

// ─── commonActionView interface ───────────────────────────────────────────────

func (m *KanbanModel) viewID() PresentationID {
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

	// Apply the column move to every selected task in memory, grouped by owning
	// project, then commit each touched project through the workflow-aware
	// coordinator. The shell reloads the board from the committed transaction; a
	// task that cannot transition (status mode) is skipped and not counted.
	moved := 0
	var changedProjects []*models.Project
	seen := make(map[string]bool)
	for _, ref := range selected {
		if !m.applyBulkColumnMove(ref.project, ref.task, targetCol) {
			continue
		}
		moved++
		if !seen[ref.project.ID] {
			seen[ref.project.ID] = true
			changedProjects = append(changedProjects, ref.project)
		}
	}

	m.FinishBulk(&m.statusMsg, fmt.Sprintf("Moved %d %s → %s", moved, pluralize(moved, "task", "tasks"), targetCol.Label))
	return m, m.commitChangedProjects(changedProjects, "", moved)
}

// applyBulkColumnMove applies the mode-specific mutation that moving a card to
// targetCol represents, in memory. It returns whether the task was mutated so the
// caller can count moves and collect touched projects; a status transition the
// task cannot make (or an unknown mode) returns false.
func (m *KanbanModel) applyBulkColumnMove(project *models.Project, task *models.Task, targetCol *KanbanColumn) bool {
	switch m.mode {
	case KanbanModeSection:
		section := kanbanColIDToSection(targetCol.ID)
		return project.MutateTask(task.ID, func(t *models.Task) {
			assignSection(t, section)
			if section != "" {
				project.EnsureSection(section)
			}
		})
	case KanbanModeStatus:
		newState := models.TargetStateForStatus(models.TaskStatus(targetCol.ID))
		if reason := models.CanTransitionState(task.State(), newState); reason != "" {
			return false // skip tasks that can't transition
		}
		return project.MutateTask(task.ID, func(t *models.Task) {
			t.ApplyStateTransition(newState)
		})
	case KanbanModePriority:
		priority := kanbanColIDToPriority(targetCol.ID)
		return project.MutateTask(task.ID, func(t *models.Task) {
			t.Priority = priority
		})
	default:
		return false
	}
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

	// The single-card column move mutates the cursor task (getTaskRef), so the
	// coordinator-backed wf* helpers target exactly the task being moved. The
	// shell reloads the board from the async BatchTransactionResultMsg (as the
	// migrated priority path already does), so no inline reload is needed here.
	switch m.mode {
	case KanbanModeSection:
		section := kanbanColIDToSection(targetCol.ID)
		model, cmd = m, wfSectionMove(m, section)

	case KanbanModeStatus:
		newState := models.TargetStateForStatus(models.TaskStatus(targetCol.ID))
		if reason := models.CanTransitionState(task.State(), newState); reason != "" {
			m.statusMsg = reason
			return m, nil
		}
		model, cmd = m, wfChangeState(m, newState)

	case KanbanModePriority:
		newPriority := kanbanColIDToPriority(targetCol.ID)
		model, cmd = m.applyKanbanPriorityChange(project, task, newPriority)

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

// applyKanbanPriorityChange applies a single-card priority change through the
// application-layer single-task coordinator. The shell's transaction-result
// handler reloads the committed project and refreshes the board (reloadData +
// clampCursor), so no inline reload is needed here.
func (m *KanbanModel) applyKanbanPriorityChange(_ *models.Project, task *models.Task, priority int) (tea.Model, tea.Cmd) {
	if task.Priority == priority {
		return m, nil
	}
	cmd := m.taskItem.SetPriority(task.ID, priority)
	msg := cmd()
	if em, ok := msg.(TransactionErrorMsg); ok {
		m.setErr(em.Err)
		return m, nil
	}
	return m, func() tea.Msg { return msg }
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
