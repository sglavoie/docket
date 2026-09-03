package tui

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// handleEditTaskContent launches external editor for task title and description.
// A reopen carries the refused title, the refusal, and the description the user
// already typed, so nothing the buffer held is lost to a rejected sigil.
func (m *AppModel) handleEditTaskContent(msg EditTaskContentMsg) (bool, tea.Cmd) {
	if msg.Task == nil {
		return true, nil
	}

	title, description := msg.Task.Title, msg.Task.Description
	if msg.RejectedTitle != "" {
		title, description = msg.RejectedTitle, msg.Description
	}
	content := fmt.Sprintf("# %s\n\n%s\n\n%s", title, taskContentInstruction(msg.Notice), description)

	tmpPath, err := writeTaskContentBuffer(msg.Path, content)
	if err != nil {
		return true, status.ErrorCmd("%s", err)
	}

	editor, editorErr := external.ResolveEditor("")
	if editorErr != nil {
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
		return true, status.ErrorCmd("editor failed: %w", msg.Err)
	}
	if msg.Task == nil || msg.Project == nil {
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
		removeTaskContentBuffer(msg.Path)
		return true, status.ErrorCmd("task edit context is unavailable")
	}

	baseline := msg.RejectedTitle
	if baseline == "" {
		baseline = task.Title
	}

	if msg.NewTitle == baseline {
		if msg.RejectedTitle != "" {
			removeTaskContentBuffer(msg.Path)
			return true, status.ErrorCmd("%s", msg.Notice)
		}
		// An untouched title is never parsed, so a literal `#123` in it cannot be
		// silently eaten by the tag sigil.
		return true, m.commitTaskContentEdit(task, project, &quickEditResult{
			title:      task.Title,
			parentMode: quickEditParentPreserve,
		}, msg)
	}

	result, err := parseQuickEdit(msg.NewTitle, task, project, m.availableProjects())
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

func (m *AppModel) commitTaskContentEdit(task *models.Task, project *models.Project, result *quickEditResult, msg TaskContentEditedMsg) tea.Cmd {
	source := taskref.For(task, project)
	req := taskLineEditRequest(source, result)
	req.Description = &msg.NewDescription
	req.PlanningArtifacts = shellPlanningReconcileArtifactBuilder(m.runtime, m.configManager)
	if req.DestinationProjectID != source.ProjectID {
		req.PlanningArtifacts = shellStructuralPlanningArtifactBuilder(m.runtime, m.configManager)
	}
	var relocation *outlinerRelocationCapture
	if out, ok := m.outlinerPrimary(); ok {
		relocation = out.captureOutlinerRelocation([]taskref.TaskIdentity{source})
	}

	edited, err := m.appService.EditTaskLine(req)
	removeTaskContentBuffer(msg.Path)
	if err != nil {
		return m.handleTaskLineEditFailed(TaskLineEditFailedMsg{Source: source, Err: err})
	}
	return m.handleTaskLineEdited(TaskLineEditedMsg{Result: edited, Source: source, Relocation: relocation})
}
