package taskdetail

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// taskCommentOwner is the production CommentOwner for a task's comments. It
// keeps the existing task-comment vocabulary exactly: stored order, the
// application layer's three indexed mutations, and the editor buffers that name
// the task by ShortID and title.
type taskCommentOwner struct {
	task        *models.Task
	project     *models.Project
	coordinator coordinator.SubviewCoordinator
}

// NewTaskCommentOwner binds the shared Comments panel to one task.
func NewTaskCommentOwner(task *models.Task, project *models.Project, subview coordinator.SubviewCoordinator) CommentOwner {
	return &taskCommentOwner{task: task, project: project, coordinator: subview}
}

// Key spells the task identity rather than reusing it as a value, so the
// panel's asynchronous correlation stays a comparison between owner keys and
// never grows a second meaning for a task locator.
func (o *taskCommentOwner) Key() string {
	identity := o.identity()
	if !identity.Valid() {
		return ""
	}
	return fmt.Sprintf("task:%s/%s", identity.ProjectID, identity.TaskID)
}

func (o *taskCommentOwner) identity() taskref.TaskIdentity {
	return taskref.For(o.task, o.project)
}

func (o *taskCommentOwner) Title() CommentOwnerTitle {
	if o.task == nil {
		return CommentOwnerTitle{Prefix: "Comments"}
	}
	return CommentOwnerTitle{Prefix: fmt.Sprintf("Comments · #%d: ", o.task.ShortID), Task: o.task}
}

// Entries keeps the stored order a task's comments are written and read in.
func (o *taskCommentOwner) Entries() []CommentEntry {
	if o.task == nil {
		return nil
	}
	entries := make([]CommentEntry, 0, len(o.task.Comments))
	for _, comment := range o.task.Comments {
		entries = append(entries, CommentEntry{Text: comment.Text, CreatedAt: comment.CreatedAt})
	}
	return entries
}

func (o *taskCommentOwner) ExternalAddHeader() string {
	if o.task == nil {
		return "# New comment\n\n<!-- Enter the comment text below. -->\n\n"
	}
	return fmt.Sprintf("# New comment on #%d: %s\n\n<!-- Enter the comment text below. -->\n\n", o.task.ShortID, o.task.Title)
}

func (o *taskCommentOwner) ExternalEditHeader(index int) string {
	if o.task == nil {
		return "# Comment\n\n<!-- Edit the comment text below. The timestamp will be preserved. -->\n\n"
	}
	return fmt.Sprintf(
		"# Comment %d on #%d: %s\n\n<!-- Edit the comment text below. The timestamp will be preserved. -->\n\n",
		index, o.task.ShortID, o.task.Title)
}

func (o *taskCommentOwner) Add(text string) tea.Cmd {
	return o.coordinator.AddComment(o.identity(), text)
}

func (o *taskCommentOwner) Edit(index int, text string) tea.Cmd {
	return o.coordinator.EditComment(o.identity(), index, text)
}

func (o *taskCommentOwner) Remove(index int) tea.Cmd {
	return o.coordinator.RemoveComment(o.identity(), index)
}

// Reload re-resolves the task pointer inside its project, which a committed
// transaction replaces wholesale. The identity the mutations address is derived
// from the resolved task, so a stale pointer would otherwise keep addressing an
// object the workspace has already discarded.
func (o *taskCommentOwner) Reload() bool {
	if o.project == nil || o.task == nil {
		return false
	}
	task := o.project.GetTaskByID(o.task.ID)
	if task == nil {
		return false
	}
	o.task = task
	return true
}

func (o *taskCommentOwner) TaskRef() (*models.Task, *models.Project) {
	return o.task, o.project
}
