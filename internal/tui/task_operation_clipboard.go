package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// operationProjectsClipboard names the three read-only projections. They branch
// off the mutation path before it because two things are true of them and of
// nothing else: the order of their targets is the order the copied text reads
// in, and they commit nothing, so there is no transaction for a relocation
// capture or a check-in offer to follow.
func operationProjectsClipboard(id operation.OperationID) bool {
	switch id {
	case operation.OperationTaskCopyTitle,
		operation.OperationTaskCopyDetails,
		operation.OperationTaskCopyTitleDescription:
		return true
	default:
		return false
	}
}

// executeTaskCopyOperation copies the whole resolved selection, hidden members
// included, as one clipboard write. An empty visual selection never reaches it:
// the descriptors are one-or-more over the selected tasks, so the resolver
// refuses instead of falling back to the cursor.
func (m *AppModel) executeTaskCopyOperation(
	invocation operation.OperationInvocation,
) operation.OperationHandlerResult {
	invocation.Targets = m.clipboardTargetOrder(invocation.Targets)
	request, action, summary, handled, err := applicationTaskOperationRequest(invocation)
	if err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	if !handled {
		return operation.OperationHandlerResult{Err: fmt.Errorf(
			"operation %s is not a clipboard projection",
			invocation.Descriptor.ID,
		)}
	}
	counts := operationSelectionCounts(invocation.Context, len(request.Targets))
	return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{
		Cmd: withClipboardHiddenCount(
			m.secondaryFeatures.TaskItem.ExecuteOperation(request, action, summary),
			counts.Hidden,
		),
	}}
}

// clipboardTargetOrder replaces the resolver's identity ordering with the order
// the copied text reads in. The resolver stays sorted by identity for every
// other operation, because only copied text makes its target order visible.
func (m *AppModel) clipboardTargetOrder(
	targets operation.ResolvedOperationTargets,
) operation.ResolvedOperationTargets {
	if len(targets.Targets) < 2 {
		return targets
	}
	byIdentity := make(map[taskref.TaskIdentity]operation.OperationTarget, len(targets.Targets))
	identities := make([]taskref.TaskIdentity, 0, len(targets.Targets))
	for _, target := range targets.Targets {
		if !target.Task.Valid() {
			// A malformed set is the request builder's error to report, not this
			// reordering's to hide.
			return targets
		}
		identities = append(identities, target.Task)
		byIdentity[target.Task] = target
	}
	ordered := m.Outliner().ClipboardSelectionOrder(identities)
	reordered := make([]operation.OperationTarget, 0, len(ordered))
	for _, identity := range ordered {
		reordered = append(reordered, byIdentity[identity])
	}
	targets.Targets = reordered
	return targets
}

// withClipboardHiddenCount stamps the projection with the part of the selection
// the view was not showing. The application layer cannot supply it: a hidden
// member is a fact about the projection the selection was frozen against, not
// about the workspace the text was read from.
func withClipboardHiddenCount(cmd tea.Cmd, hidden int) tea.Cmd {
	if cmd == nil || hidden <= 0 {
		return cmd
	}
	return func() tea.Msg {
		msg := cmd()
		projected, ok := msg.(coordinator.TaskClipboardProjectedMsg)
		if !ok {
			return msg
		}
		projected.Hidden = hidden
		return projected
	}
}
