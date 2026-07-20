package focus

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// LegacyNextQueueMemberIDs returns the task IDs of Focus Queue members that
// predate the state-driven Next invariant and must be backfilled to StateNext.
// Next membership is now the StateNext workflow state, so a queued task that
// exists and is non-terminal but is not yet StateNext is a legacy member whose
// only evidence lives in the ordering-only index; it must be migrated so
// first-use pruning cannot silently interpret it as stale and drop it. Missing
// and terminal members are left alone — those are genuinely stale and handled by
// the prune path.
//
// The returned IDs preserve queue order and are de-duplicated. This function is
// deliberately side-effect free: it names what to migrate but never mutates task
// state or the order index. The caller commits the StateNext backfill through the
// workspace transaction boundary (BulkTransition) so the migration is journaled,
// atomic, and undoable — never a raw per-project save. A queue with no legacy
// members yields an empty slice (nothing to migrate).
func LegacyNextQueueMemberIDs(queue []config.FocusQueueItem, projects []*models.Project) []string {
	seen := make(map[string]bool)
	var ids []string
	for _, item := range queue {
		for _, project := range projects {
			if project == nil {
				continue
			}
			task := project.GetTaskByID(item.TaskID)
			if task == nil {
				continue
			}
			if !task.IsTerminalState() && task.State() != models.StateNext && !seen[task.ID] {
				seen[task.ID] = true
				ids = append(ids, task.ID)
			}
			break
		}
	}
	return ids
}
