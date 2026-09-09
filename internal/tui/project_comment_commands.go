package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/operationhost"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/taskdetail"
)

// addProjectCommentCmd appends one comment through the ProjectCommentCoordinator
// and rewrites only its rejection, exactly as addCommentCmd does for a task: a
// successful commit reaches the shell as the ordinary batch result and gets the
// ordinary reload, cursor settle, and recovery feedback, which is also what
// leaves Peek's open/closed state alone.
func (m *AppModel) addProjectCommentCmd(msg projecthub.AddProjectCommentMsg) tea.Cmd {
	if msg.Project == nil {
		return nil
	}
	cmd := m.secondaryFeatures.ProjectComment.Add(msg.Project.ID, msg.Project.Name, msg.Text)
	request, text := msg.Request, msg.Text
	return func() tea.Msg {
		out := cmd()
		if failure, ok := out.(coordinator.TransactionErrorMsg); ok {
			return projecthub.ProjectCommentRejectedMsg{Request: request, Text: text, Err: failure.Err}
		}
		return out
	}
}

// reopenProjectCommentPrompt puts the refused comment back in the buffer it was
// typed in, on the owner the prompt was frozen onto.
//
// It does not degrade to a toast the way the task comment and resume note do,
// and the difference is deliberate: those two rebuild from a target that has to
// resolve, so an unresolvable one leaves nothing to retry into. This prompt
// names its owner from the request itself, so there is always somewhere to put
// the text back — and text is the thing the refusal must not cost, whether the
// write failed, the document moved underneath it, or the project is gone.
func (m *AppModel) reopenProjectCommentPrompt(msg projecthub.ProjectCommentRejectedMsg) tea.Cmd {
	prompt := m.projectCommentPrompt(msg.Request)
	prompt.Restore(msg.Text, operationRefusalReason(msg.Err))
	m.operations.TextPrompt = prompt
	return prompt.InitCmd()
}

// projectCommentPrompt builds the quick-comment prompt on the project the
// request froze. The buffer opens empty because appending never edits an
// existing comment, and a project that no longer resolves is named by its stored
// identity rather than by whatever the cursor now holds: the frozen owner is the
// whole point, so degrading to a locator is right where borrowing another
// project would be wrong.
func (m *AppModel) projectCommentPrompt(
	request operation.OperationExecutionRequest,
) *operationhost.OperationTextPromptModel {
	identity := operation.CurrentEntityIdentity(request.Context)
	subtitle := identity.ProjectID
	if project := m.projectByID(identity.ProjectID); project != nil {
		subtitle = project.Name
	}
	label := string(operation.ValueProviderProjectComment)
	if descriptor, ok := m.GetOperationRegistry().Descriptor(request.ID); ok {
		label = descriptor.Label
	}
	return operationhost.NewOperationTextPromptModel(
		request, label, subtitle, "", "comment text",
		operationhost.ProjectCommentTextPromptOptions(taskdetail.CommentCharLimit),
	)
}
