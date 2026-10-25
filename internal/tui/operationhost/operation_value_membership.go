package operationhost

import (
	"fmt"

	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

func (c *Controller) TaskRefs(shell Shell, context operation.CommandContext) ([]taskref.TaskProjectRef, error) {
	identities := operation.NormalizedTaskIdentities(context.Selection.Tasks)
	if len(identities) == 0 {
		if context.Selection.Visual {
			return nil, fmt.Errorf("select at least one task")
		}
		if current := operation.CurrentTaskIdentity(context); current.Valid() {
			identities = []taskref.TaskIdentity{current}
		}
	}
	refs := make([]taskref.TaskProjectRef, 0, len(identities))
	for _, identity := range identities {
		task, project := taskAndProjectForIdentity(shell, identity)
		if task == nil || project == nil {
			return nil, fmt.Errorf("selected task is no longer available")
		}
		refs = append(refs, taskref.TaskProjectRef{Task: task, Project: project})
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("select a task")
	}
	return refs, nil
}
