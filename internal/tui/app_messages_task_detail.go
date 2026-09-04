package tui

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/route"
)

// =============================================================================
// Task Comments Messages (Show/Close Pattern)
// =============================================================================

// ShowTaskCommentsMsg opens the comments view for a task.
type ShowTaskCommentsMsg struct {
	Task    *models.Task
	Project *models.Project
}

// CloseTaskCommentsMsg closes the comments view.
type CloseTaskCommentsMsg struct{ route.PopViewMsg }

// CommentAddedMsg signals a comment was added.
type CommentAddedMsg struct{ TransactionID string }

// CommentDeletedMsg signals a comment was deleted.
type CommentDeletedMsg struct{ TransactionID string }

// CommentEditedMsg signals a comment was edited.
type CommentEditedMsg struct{ TransactionID string }

// =============================================================================
// Task Attachments Messages (Show/Close Pattern)
// =============================================================================

// ShowTaskAttachmentsMsg opens the attachments view for a task.
type ShowTaskAttachmentsMsg struct {
	Task    *models.Task
	Project *models.Project
}

// CloseTaskAttachmentsMsg closes the attachments view.
type CloseTaskAttachmentsMsg struct{ route.PopViewMsg }
