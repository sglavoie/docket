package bridge

import (
	"github.com/sglavoie/docket/internal/app"
)

type deletePayload struct {
	Task locator `json:"task"`
}

type deleteResult struct {
	// Removed names the task and every descendant the subtree removal took,
	// so a client drops exactly these addresses.
	Removed   []locator `json:"removed"`
	Committed committed `json:"committed"`
}

// commandDelete permanently removes one task and its whole subtree through
// the desktop's delete use case, as one transaction its receipt can undo. A
// delete is guarded: the request's expect must name the task's project at the
// revision the confirmation was shown against, so a delete confirmed before
// more work arrived (a new subtask, an import) answers stale_revision and
// removes nothing.
func (b *Bridge) commandDelete(request Request) (any, error) {
	var payload deletePayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if err := payload.Task.validate("task"); err != nil {
		return nil, err
	}
	if request.Expect == nil || request.Expect.Projects[payload.Task.ProjectID] == "" {
		return nil, newError(CodeInvalidRequest, "a delete needs expect.projects to name project %s at the revision it read", payload.Task.ProjectID)
	}
	runtime := b.workspace.runtime
	before, err := currentRevisions(runtime)
	if err != nil {
		return nil, err
	}
	result, err := runtime.Service().Delete(app.DeleteRequest{Locator: payload.Task.app(), Confirm: true})
	if err != nil {
		return nil, err
	}
	if err := runtime.Refresh(); err != nil {
		return nil, err
	}
	after, err := currentRevisions(runtime)
	if err != nil {
		return nil, err
	}
	out := deleteResult{
		Removed: make([]locator, 0, len(result.RemovedTaskLocators)),
		Committed: committed{
			TransactionID: result.TransactionID,
			Projects:      changedRevisions(before.Projects, after.Projects),
			Domains:       changedRevisions(before.Domains, after.Domains),
		},
	}
	for _, removed := range result.RemovedTaskLocators {
		out.Removed = append(out.Removed, locator{ProjectID: removed.ProjectID, TaskID: removed.TaskID})
	}
	return out, nil
}
