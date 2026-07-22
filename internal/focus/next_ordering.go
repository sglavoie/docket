package focus

import "github.com/sglavoie/tt/internal/config"

// FocusQueueRank maps each project-qualified task present in the Focus Queue
// order index to its 0-based position. Membership in Next is the StateNext
// workflow state, not presence in this list — the list only records the user's
// preferred ordering among Next tasks. Duplicate qualified entries keep their
// first (earliest) position.
func FocusQueueRank(queue []config.FocusQueueItem) map[config.ProjectTaskLocator]int {
	rank := make(map[config.ProjectTaskLocator]int, len(queue))
	for i, item := range queue {
		locator := config.ProjectTaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID}
		if _, seen := rank[locator]; !seen {
			rank[locator] = i
		}
	}
	return rank
}

// RankOrUnindexed returns the queue rank for locator, or the unindexed sentinel
// (len(rank)) when the task is not in the order index. Every indexed member has
// a rank in [0, len(rank)), so unindexed StateNext tasks sort after all indexed
// members.
func RankOrUnindexed(rank map[config.ProjectTaskLocator]int, locator config.ProjectTaskLocator) int {
	if r, ok := rank[locator]; ok {
		return r
	}
	return len(rank)
}
