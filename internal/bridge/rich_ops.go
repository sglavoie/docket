package bridge

import (
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// The rich-content ops: task and project comments, project and global notes,
// and task attachments. Each one calls the application use case the desktop's
// panels call, so a comment, a note, or an attachment the phone writes is the
// same durable record and commits through the same transaction engine.

// commentView is one comment as a client reads it. A task comment is
// addressed by its index and the text read there; a project comment by its
// stored id and that text.
type commentView struct {
	Index     int       `json:"index"`
	ID        string    `json:"id,omitempty"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

func viewTaskComments(task *models.Task) []commentView {
	views := make([]commentView, 0, len(task.Comments))
	for index, comment := range task.Comments {
		if comment == nil {
			continue
		}
		views = append(views, commentView{Index: index, Text: comment.Text, CreatedAt: comment.CreatedAt})
	}
	return views
}

func viewProjectComments(project *models.Project) []commentView {
	views := make([]commentView, 0, len(project.Comments))
	for index, comment := range project.Comments {
		if comment == nil {
			continue
		}
		views = append(views, commentView{Index: index, ID: comment.ID, Text: comment.Text, CreatedAt: comment.CreatedAt})
	}
	return views
}

// changedSinceRead is the refusal for an index-addressed record that no
// longer holds what the client read there.
func changedSinceRead(field string, index int) *Error {
	return newError(CodeValidation, "%s %d changed since it was read; reload and try again", field, index).
		with("kind", "invalid_input").with("field", field).with("reason", "changed since it was read")
}

// ---- comments ----

type commentPayload struct {
	Task      *locator `json:"task,omitempty"`
	ProjectID string   `json:"project_id,omitempty"`
	Action    string   `json:"action"`
	Text      string   `json:"text,omitempty"`
	Index     *int     `json:"index,omitempty"`
	CommentID string   `json:"comment_id,omitempty"`
	Original  *string  `json:"original,omitempty"`
}

type commentResult struct {
	Task      *taskView     `json:"task,omitempty"`
	ProjectID string        `json:"project_id,omitempty"`
	CommentID string        `json:"comment_id,omitempty"`
	Comments  []commentView `json:"comments"`
	Committed committed     `json:"committed"`
}

// commandComment adds, edits, or deletes one task or project comment. An
// edit or a delete names the comment and carries the text the client read
// there, and is refused when the stored comment no longer holds it, so an
// import or another edit in the meantime never redirects it.
func (b *Bridge) commandComment(request Request) (any, error) {
	var payload commentPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	switch {
	case (payload.Task == nil) == (payload.ProjectID == ""):
		return nil, newError(CodeInvalidRequest, "a comment names exactly one owner: task or project_id")
	case payload.Action != "add" && payload.Action != "edit" && payload.Action != "delete":
		return nil, newError(CodeInvalidRequest, "comment action %q is not add, edit, or delete", payload.Action)
	case payload.Action != "add" && payload.Original == nil:
		return nil, newError(CodeInvalidRequest, "%s needs original, the text read", payload.Action)
	}
	if payload.Task != nil {
		return b.taskComment(payload)
	}
	return b.projectComment(payload)
}

func (b *Bridge) taskComment(payload commentPayload) (any, error) {
	if err := payload.Task.validate("task"); err != nil {
		return nil, err
	}
	target := payload.Task.app()
	if payload.Action != "add" {
		if payload.Index == nil {
			return nil, newError(CodeInvalidRequest, "%s needs index", payload.Action)
		}
		read, err := b.readTask(target)
		if err != nil {
			return nil, err
		}
		index := *payload.Index
		comments := read.Comments
		if index < 0 || index >= len(comments) || comments[index].Text != *payload.Original {
			return nil, changedSinceRead("comment", index)
		}
	}
	mutation, err := b.commitTask(func(service *app.Service) (app.TaskResult, error) {
		var result app.CommentResult
		var err error
		switch payload.Action {
		case "add":
			result, err = service.AddComment(target, payload.Text)
		case "edit":
			result, err = service.EditComment(target, *payload.Index, payload.Text)
		default:
			result, err = service.RemoveComment(target, *payload.Index)
		}
		return result.Task, err
	})
	if err != nil {
		return nil, err
	}
	read, err := b.readTask(target)
	if err != nil {
		return nil, err
	}
	return commentResult{Task: &mutation.Task, Comments: read.Comments, Committed: mutation.Committed}, nil
}

func (b *Bridge) projectComment(payload commentPayload) (any, error) {
	service := b.workspace.runtime.Service()
	var result app.ProjectCommentResult
	var err error
	switch payload.Action {
	case "add":
		result, err = service.AddProjectComment(payload.ProjectID, payload.Text)
	default:
		if payload.CommentID == "" {
			return nil, newError(CodeInvalidRequest, "%s needs comment_id", payload.Action)
		}
		target := app.ProjectCommentTarget{ProjectID: payload.ProjectID, CommentID: payload.CommentID, Original: *payload.Original}
		if payload.Action == "edit" {
			result, err = service.EditProjectComment(target, payload.Text)
		} else {
			result, err = service.RemoveProjectComment(target)
		}
	}
	if err != nil {
		return nil, err
	}
	comments, err := b.projectComments(payload.ProjectID)
	if err != nil {
		return nil, err
	}
	return commentResult{
		ProjectID: result.ProjectID,
		CommentID: result.CommentID,
		Comments:  comments,
		Committed: committed{
			TransactionID: result.TransactionID,
			Projects:      map[string]string{result.ProjectID: result.Revision},
			Domains:       map[string]string{},
		},
	}, nil
}

// projectComments reads one available project's comments in stored order:
// creation time, then identity.
func (b *Bridge) projectComments(projectID string) ([]commentView, error) {
	snapshot, err := b.workspace.runtime.Snapshot()
	if err != nil {
		return nil, err
	}
	project := snapshot.ProjectByID(projectID)
	if project == nil {
		return []commentView{}, nil
	}
	return viewProjectComments(project), nil
}
