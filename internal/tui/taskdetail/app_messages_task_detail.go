package taskdetail

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

// ShowProjectCommentsMsg opens the same comments view over a project. It is a
// second message rather than a nullable field on the task one because the two
// carry different owners: the shell builds a different adapter for each, and a
// message that could name both would let a handler mount a panel on neither.
type ShowProjectCommentsMsg struct {
	Project *models.Project
}

// CloseTaskCommentsMsg closes the comments view.
type CloseTaskCommentsMsg struct{ route.PopViewMsg }

// Owner and Token on the four messages below identify which submission an
// outcome answers. The shell routes them by presentation, which says where they
// land but not what they are for, so the panel needs the owner key it submitted
// against and the process-unique token it submitted under to tell its own
// confirmed transaction from an abandoned or another owner's one.
//
// ProjectID on the three success messages names the project the transaction
// committed against. The panel projects its owner's stored comments rather than
// editing them, so the shell has to reload that project into its in-memory copy
// before the panel re-reads it — the same reload handleTransactionResult does
// for every other committed mutation, which these messages replace.

// CommentAddedMsg signals a comment was added.
type CommentAddedMsg struct {
	Owner         string
	Token         uint64
	ProjectID     string
	TransactionID string
}

// CommentDeletedMsg signals a comment was deleted.
type CommentDeletedMsg struct {
	Owner         string
	Token         uint64
	ProjectID     string
	TransactionID string
}

// CommentEditedMsg signals a comment was edited.
type CommentEditedMsg struct {
	Owner         string
	Token         uint64
	ProjectID     string
	TransactionID string
}

// CommentMutationFailedMsg reports a refused comment commit. The shared
// ErrorMsg carries no correlation, so a failure needs its own message for the
// panel to know which draft to restore — and for a panel that has moved on to
// know the refusal is not its own.
type CommentMutationFailedMsg struct {
	Owner string
	Token uint64
	Err   error
}

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
