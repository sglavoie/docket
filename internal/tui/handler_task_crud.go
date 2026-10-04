package tui

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskform"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// handleTaskCRUDMessages handles task CRUD messages (create, edit, delete).
// Returns true if the message was handled.
func (m *AppModel) handleTaskCRUDMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case taskform.CreateTaskMsg:
		return m.handleCreateTask(msg)

	case coordinator.TaskCreatedMsg:
		return m.handleTaskCreated(msg)

	case TaskDeleteRequestedMsg:
		m.handleTaskDeleteRequested(msg)
		return true, nil

	}
	return false, nil
}

// handleCreateTask handles the CreateTaskMsg.
func (m *AppModel) handleCreateTask(msg taskform.CreateTaskMsg) (bool, tea.Cmd) {
	project := msg.Project
	if project == nil {
		project = m.currentProject
	}
	if project == nil {
		return true, nil
	}

	return true, m.openTaskComposer(m.createComposerPreset(project, msg.ParentTask))
}

// handleTaskCreated handles the coordinator.TaskCreatedMsg.
func (m *AppModel) handleTaskCreated(msg coordinator.TaskCreatedMsg) (bool, tea.Cmd) {
	// Capture whether this creation originated from the Quick Capture view and the
	// pre-capture task count before handleWithRefresh pops the view / reloads.
	// Coordinator path: the capture committed one atomic ChangeSet through the
	// application layer, so reload the committed project from disk and re-resolve
	// the task by ID. The shared navigation/status/open-after logic below then runs
	// off the reloaded pointers exactly as it did for the legacy in-place save.
	planningRefreshed := false
	if msg.PlanningChanged {
		planningRefreshed = m.refreshRuntime()
	}
	// A multi-line capture can span projects, and the planning rebuild below
	// resolves membership against the loaded tasks, so every project the commit
	// touched is reloaded before it, not just the first result's.
	reloaded := make(map[string]bool, len(msg.Results)+1)
	reload := func(projectID string) {
		if projectID != "" && !reloaded[projectID] {
			reloaded[projectID] = true
			m.reloadProjectByID(projectID)
		}
	}
	for _, result := range msg.Results {
		reload(result.ProjectID)
	}
	if msg.Result != nil {
		reload(msg.Result.ProjectID)
		if project := m.projectByID(msg.Result.ProjectID); project != nil {
			msg.Project = project
			// OpenTaskID overrides the navigation/open target (task-form
			// open-after-subtask targets a created child, not the parent).
			targetID := msg.Result.TaskID
			if msg.OpenTaskID != "" {
				targetID = msg.OpenTaskID
			}
			msg.Task = project.GetTaskByID(targetID)
		}
	}
	if planningRefreshed {
		m.rebuildPlanningFromRuntime()
	}

	refreshCmd := m.handleWithRefresh(!msg.KeepComposerOpen, true)

	if msg.Task != nil && !msg.KeepComposerOpen {
		project := msg.Project
		if project == nil && msg.Result != nil {
			project = m.projectByID(msg.Result.ProjectID)
		}
		if m.crossOutliner != nil && project != nil {
			m.crossOutliner.NavigateToTaskInProject(msg.Task, project)
		}
	}

	// Set status message showing section
	if msg.Task != nil {
		if bucket := m.createdTaskSectionLabel(msg.Task, msg.Project); bucket != "" {
			if parked := m.parkedOwnerLabel(msg.Project); parked != "" {
				m.setSuccessStatus(fmt.Sprintf("Created in %s — %s", bucket, parked))
			} else {
				m.setSuccessStatus(fmt.Sprintf("Created in %s", bucket))
			}
		}
	}
	if msg.CreatedCount > 1 {
		m.setSuccessStatus(fmt.Sprintf("Created %d tasks", msg.CreatedCount))
	}
	successMessage := m.feedback.message
	if successMessage == "" {
		successMessage = "Created task"
	}
	transactionID := ""
	if msg.Result != nil {
		transactionID = msg.Result.TransactionID
	}
	// A creation has no before address, so undoing it correlates to nothing and
	// leaves the view alone; redoing it puts the cursor back on the task it mints
	// again. A multiline capture created several tasks and names none of them.
	created := coordinator.CommittedFocusTarget{}
	if msg.Result != nil && msg.CreatedCount <= 1 {
		created.After = app.TaskLocator{ProjectID: msg.Result.ProjectID, TaskID: msg.Result.TaskID}
	}
	m.undoFocus.ObserveCommit(transactionID, created)
	feedbackCmd := m.setRecoveryFeedback(successMessage, transactionID, false)

	// Delegate to the current view so it can reload its state
	cmd := m.viewNav.views.Update(m.currentView(), msg)

	// If open-after requested, re-lookup the task (stale after reload) and open
	if msg.Task != nil && msg.OpenContent {
		project := msg.Project
		if project == nil {
			project = m.currentProject
		}
		if project != nil {
			if freshTask := project.GetTaskByID(msg.Task.ID); freshTask != nil {
				openMsg := EditTaskContentMsg{Task: freshTask, Project: project}
				return true, tea.Batch(refreshCmd, cmd, feedbackCmd, func() tea.Msg {
					return openMsg
				})
			}
		}
	}

	return true, tea.Batch(refreshCmd, cmd, feedbackCmd)
}

func (m *AppModel) createdTaskSectionLabel(task *models.Task, project *models.Project) string {
	if task == nil {
		return ""
	}
	if project != nil {
		identity := taskref.For(task, project)
		if m.todayIDs[identity] {
			return models.StateGroupToday.Label()
		}
		if m.nextOrderIDs[identity] {
			return models.StateGroupNext.Label()
		}
	}
	return theme.TaskStateGroupLabel(task)
}

// handleEditTaskContent launches external editor for task title and description.
// A reopen carries the refused title, the refusal, and the description the user
// already typed, so nothing the buffer held is lost to a rejected sigil.
func (m *AppModel) handleEditTaskContent(msg EditTaskContentMsg) (bool, tea.Cmd) {
	if msg.Task == nil {
		m.retireInteraction()
		return true, nil
	}

	title, description := msg.Task.Title, msg.Task.Description
	if msg.RejectedTitle != "" {
		title, description = msg.RejectedTitle, msg.Description
	}
	content := fmt.Sprintf("# %s\n\n%s\n\n%s", title, taskContentInstruction(msg.Notice), description)

	tmpPath, err := writeTaskContentBuffer(msg.Path, content)
	if err != nil {
		m.retireInteraction()
		return true, status.ErrorCmd("%s", err)
	}

	editor, editorErr := external.ResolveEditor("")
	if editorErr != nil {
		m.retireInteraction()
		removeTaskContentBuffer(tmpPath)
		return true, status.ErrorCmd("%s", editorErr)
	}
	c := external.EditorCommand(editor, tmpPath)

	task := msg.Task
	project := msg.Project
	rejectedTitle := msg.RejectedTitle
	notice := msg.Notice
	return true, tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			removeTaskContentBuffer(tmpPath)
			return TaskContentEditedMsg{Task: task, Project: project, Err: err}
		}
		content, readErr := os.ReadFile(tmpPath)
		if readErr != nil {
			removeTaskContentBuffer(tmpPath)
			return TaskContentEditedMsg{Task: task, Project: project, Err: readErr}
		}
		newTitle, newDesc := parseTaskContent(string(content), task)
		return TaskContentEditedMsg{
			Task:           task,
			Project:        project,
			Path:           tmpPath,
			RejectedTitle:  rejectedTitle,
			Notice:         notice,
			NewTitle:       newTitle,
			NewDescription: newDesc,
		}
	})
}

// taskContentInstruction renders the buffer's single instruction line. It stays
// one physical line because parseTaskContent consumes only the first comment
// line and would keep a second one as description text.
func taskContentInstruction(notice string) string {
	if notice == "" {
		return "<!-- Edit the title above and description below. -->"
	}
	collapsed := strings.Join(strings.Fields(notice), " ")
	// A refusal quotes the user's own value, which may itself close the comment.
	collapsed = strings.ReplaceAll(collapsed, "-->", "->")
	return fmt.Sprintf("<!-- %s. Fix the title above; description below. -->", collapsed)
}

func writeTaskContentBuffer(path, content string) (string, error) {
	if path == "" {
		return external.CreateTempFileWithContent(content, "tt-edit-", ".md")
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("failed to write to temp file: %w", err)
	}
	return path, nil
}

func removeTaskContentBuffer(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}

// handleTaskContentEdited routes the edited title through the same capture/edit
// grammar the inline row editor speaks, and commits it with the description as
// one checked transaction. A title the grammar refuses commits nothing and
// reopens the buffer instead.
func (m *AppModel) handleTaskContentEdited(msg TaskContentEditedMsg) (bool, tea.Cmd) {
	if msg.Err != nil {
		m.retireInteraction()
		return true, status.ErrorCmd("editor failed: %w", msg.Err)
	}
	if msg.Task == nil || msg.Project == nil {
		m.retireInteraction()
		return true, nil
	}

	// The composer's create-and-open path batches this open beside a reload, so
	// the captured pointers can be stale by the time the editor returns and the
	// parser compares task pointers by identity.
	project := m.projectByID(msg.Project.ID)
	var task *models.Task
	if project != nil {
		task = project.GetTaskByID(msg.Task.ID)
	}
	if task == nil {
		m.retireInteraction()
		removeTaskContentBuffer(msg.Path)
		return true, status.ErrorCmd("Task was deleted or moved while the editor was open · edit discarded")
	}

	baseline := msg.RejectedTitle
	if baseline == "" {
		baseline = task.Title
	}

	if msg.NewTitle == baseline {
		if msg.RejectedTitle != "" {
			m.retireInteraction()
			removeTaskContentBuffer(msg.Path)
			return true, status.ErrorCmd("%s", msg.Notice)
		}
		// An untouched title is never parsed, so a literal `#123` in it cannot be
		// silently eaten by the tag sigil.
		return true, m.commitTaskContentEdit(task, project, &grammar.QuickEditResult{
			Title:      task.Title,
			ParentMode: grammar.QuickEditParentPreserve,
		}, msg)
	}

	result, err := grammar.ParseQuickEdit(msg.NewTitle, task, project, m.availableProjects())
	if err != nil {
		reopen := EditTaskContentMsg{
			Task:          task,
			Project:       project,
			Path:          msg.Path,
			RejectedTitle: msg.NewTitle,
			Notice:        err.Error(),
			Description:   msg.NewDescription,
		}
		return true, func() tea.Msg { return reopen }
	}
	return true, m.commitTaskContentEdit(task, project, result, msg)
}

func (m *AppModel) commitTaskContentEdit(task *models.Task, project *models.Project, result *grammar.QuickEditResult, msg TaskContentEditedMsg) tea.Cmd {
	source := taskref.For(task, project)
	req := outliner.TaskLineEditRequest(source, result)
	req.Description = &msg.NewDescription
	req.PlanningArtifacts = coordinator.PlanningArtifacts(m.domain.runtime, m.domain.manager)
	if req.DestinationProjectID != source.ProjectID {
		req.PlanningArtifacts = coordinator.StructuralPlanningArtifacts(m.domain.runtime, m.domain.manager)
	}
	var relocation *outliner.OutlinerRelocationCapture
	if out, ok := m.outlinerPrimary(); ok {
		relocation = out.CaptureOutlinerRelocation([]taskref.TaskIdentity{source})
	}

	edited, err := m.domain.service.EditTaskLine(req)
	removeTaskContentBuffer(msg.Path)
	if err != nil {
		m.retireInteraction()
		return m.handleTaskLineEditFailed(coordinator.TaskLineEditFailedMsg{Source: source, Err: err})
	}
	// Recorded before the toast is published and spoken after it, which is the
	// whole of what the external editor owes: its own error path returns above.
	m.noteCheckInFeedback([]app.TaskResult{{ProjectID: edited.Task.ProjectID, TaskID: edited.Task.TaskID, TransactionID: edited.TransactionID, StateEffect: edited.StateEffect}})
	return m.handleTaskLineEdited(coordinator.TaskLineEditedMsg{Result: edited, Source: source, Relocation: relocation})
}

// workspaceLedgerRestoreIsNewest reports whether the durable workspace ledger
// holds a reversible transaction for the applicable action.
func (m *AppModel) workspaceLedgerRestoreIsNewest(redo bool) bool {
	ledger := workspaceLedgerFromConfig(m.domain.runtime)
	_, ok, err := ledgerNewestEntry(ledger, storage.GlobalScope(), redo)
	return err == nil && ok
}

// ledgerNewestEntry previews the entry a subsequent undo (redo=false) or redo
// (redo=true) would restore for the scope, without applying it.
func ledgerNewestEntry(ledger *storage.WorkspaceLedger, scope storage.LedgerScope, redo bool) (storage.LedgerEntry, bool, error) {
	if redo {
		return ledger.SelectNewestRedo(scope)
	}
	return ledger.SelectNewestUndo(scope)
}

// workspaceHistoryPreview reads the same global entries the undo keys restore.
// Missing or unreadable entries offer no menu action; the direct keys still
// report the ledger's own refusal when invoked.
func (m *AppModel) workspaceHistoryPreview() (undo, redo string) {
	ledger := workspaceLedgerFromConfig(m.domain.runtime)
	if entry, ok, err := ledgerNewestEntry(ledger, storage.GlobalScope(), false); err == nil && ok {
		undo = entry.Description()
	}
	if entry, ok, err := ledgerNewestEntry(ledger, storage.GlobalScope(), true); err == nil && ok {
		redo = entry.Description()
	}
	return undo, redo
}

// handleLedgerTaskUndoRedo restores the newest application-service workspace-ledger
// transaction (undo when redo=false, redo when true). It reverses (or replays) the
// entry, reloads every project the transaction touched, refreshes the runtime
// snapshot so restored planning and other config artifacts are observed, repairs
// task→project references, rebuilds the in-memory planning collections, and fans
// the change out to the open task views — the same refresh the legacy restore
// completion performs. Because it reads the durable ledger rather than in-memory
// state, the restore survives a restart.
//
// The ledger is one workspace history, so an entry can carry catalog effects a
// task view never shows: a tag assignment, an archive, a rename. The
// catalog-reading surfaces are therefore rebuilt from the restored snapshot on
// the same terms every forward catalog mutation rebuilds them.
func (m *AppModel) handleLedgerTaskUndoRedo(redo bool) tea.Cmd {
	m.retireInteraction()
	ledger := workspaceLedgerFromConfig(m.domain.runtime)
	restore := ledger.UndoWithEntry
	verb, label := "Undid", "undo"
	if redo {
		restore = ledger.RedoWithEntry
		verb, label = "Redo", "redo"
	}
	entry, desc, err := restore(storage.GlobalScope())
	if err != nil {
		// An exhausted ledger is not an error: say so quietly rather than toast
		// a spurious failure.
		if status.IsExhaustedHistory(err) {
			return status.NothingToRestoreCmd(redo)
		}
		return status.ErrorCmd("%s", status.FormatUndoRedoRestoreError(label, err))
	}
	restored := make([]*models.Project, 0, len(entry.ProjectIDs))
	for _, id := range entry.ProjectIDs {
		m.reloadProjectByID(id)
		if p := m.projectByID(id); p != nil {
			restored = append(restored, p)
		}
	}
	m.refreshRuntime()
	if err := m.repairTaskProjectRefsAfterRestore(restored); err != nil {
		return status.ErrorCmd("%s", err)
	}
	m.rebuildPlanningFromRuntime()
	m.refreshLoadedTaskWorkspace()
	// The entry's transaction ID is read only here, after the restore actually
	// succeeded: a refused or failed restore changed no durable state and must
	// leave the remembered clocks — and the correlated focus target below —
	// exactly as it found them.
	m.observeRestoredDueClocks(entry.TransactionID, redo)
	// The cursor moves before the views fan out, because the disk-backed reload
	// refreshTaskViewsAfterSave starts captures the cursor's own task: seating it
	// first is what makes the reload land back on the restored row by identity
	// rather than drag the view to wherever it started.
	focusCmd, detail := m.refocusRestoredTask(entry.TransactionID, redo)
	m.clearToggleHistory()
	m.setSuccessStatus(appendRestoreDetail(fmt.Sprintf("%s: %s", verb, desc), detail))
	m.refreshProjectHubIfActive()
	cmds := m.refreshTaskViewsAfterSave()
	cmds = append(cmds, focusCmd, m.refreshActiveNotesManager(), m.statusReadingTimeout())
	return tea.Batch(cmds...)
}

// refocusRestoredTask spends this restore's correlation, if there is one, and
// reports the command the cursor move produced together with the detail the
// status line owes.
//
// The three outcomes are distinct and each says something different. An
// uncorrelated entry — a bulk change, a project mutation, a transaction from
// before this session started — moves nothing and says nothing extra, which is
// the ordinary ledger undo unchanged. A correlated task the projection already
// draws is seated, and only a seating that actually vacated a row advertises the
// return. A correlated task the view does not admit leaves the view exactly as
// it was and names where the restored work now lives, so the answer is on screen
// instead of the user having to go looking for it.
func (m *AppModel) refocusRestoredTask(transactionID string, redo bool) (tea.Cmd, string) {
	address, ok := m.undoFocus.ObserveRestore(transactionID, redo)
	if !ok {
		return nil, ""
	}
	identity := taskref.FromLocator(address)
	task, project := m.taskAndProjectForIdentity(identity)
	if task == nil || project == nil {
		// The restore removed the task at that address — undoing a creation, for
		// instance — so there is nothing to point at and nothing to name.
		return nil, ""
	}
	out := m.crossOutliner
	if out == nil {
		return nil, m.restoredTaskLocation(task, project)
	}
	cmd, seated, originRecorded := out.RefocusRestoredTask(identity)
	if !seated {
		return nil, m.restoredTaskLocation(task, project)
	}
	if !originRecorded {
		return cmd, ""
	}
	return cmd, "Ctrl+O origin"
}

// restoredTaskLocation names where a restored task now lives, in the same terms
// a creation reports its own destination: the owning project and the lane the
// task landed in, planning membership included.
func (m *AppModel) restoredTaskLocation(task *models.Task, project *models.Project) string {
	label := m.createdTaskSectionLabel(task, project)
	if label == "" {
		return fmt.Sprintf("now in %s", project.Name)
	}
	return fmt.Sprintf("now in %s (%s)", project.Name, label)
}

// appendRestoreDetail folds the restore's own detail onto the status line in the
// canonical trailing position. An empty detail leaves the line exactly as the
// ledger worded it.
func appendRestoreDetail(message, detail string) string {
	if detail == "" {
		return message
	}
	return message + " · " + detail
}

func (m *AppModel) repairTaskProjectRefsAfterRestore(projects []*models.Project) error {
	runtimeErr := repairRuntimeTaskRefsAfterRestore(m.domain.runtime)(projects)
	m.rewireTaskProjectRefsAfterRestore(projects)
	return runtimeErr
}

func (m *AppModel) rewireTaskProjectRefsAfterRestore(projects []*models.Project) {
	refs := make(map[taskref.TaskIdentity]TaskProjectRef)
	refsByTaskID := make(map[string][]TaskProjectRef)
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task != nil && task.ID != "" {
				ref := TaskProjectRef{Task: task, Project: project}
				refs[taskref.For(task, project)] = ref
				refsByTaskID[task.ID] = append(refsByTaskID[task.ID], ref)
			}
		}
	}
	resolve := func(task *models.Task, project *models.Project) (TaskProjectRef, bool) {
		if task == nil {
			return TaskProjectRef{}, false
		}
		if project != nil {
			ref, ok := refs[taskref.For(task, project)]
			return ref, ok
		}
		candidates := refsByTaskID[task.ID]
		if len(candidates) == 1 {
			return candidates[0], true
		}
		return TaskProjectRef{}, false
	}

	for i := range m.nextOrder {
		if ref, ok := resolve(m.nextOrder[i].Task, m.nextOrder[i].Project); ok {
			m.nextOrder[i].Task = ref.Task
			m.nextOrder[i].Project = ref.Project
		}
	}
	for i := range m.today {
		if ref, ok := resolve(m.today[i].Task, m.today[i].Project); ok {
			m.today[i].Task = ref.Task
			m.today[i].Project = ref.Project
		}
	}
}
