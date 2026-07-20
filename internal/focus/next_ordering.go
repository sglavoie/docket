package focus

import "github.com/sglavoie/tt/internal/config"

// FocusQueueRank maps each task ID present in the Focus Queue order index to its
// 0-based position. Membership in Next is the StateNext workflow state, not
// presence in this list — the list only records the user's preferred ordering
// among Next tasks. Callers filter to StateNext first, then use this rank to
// place indexed members ahead of unindexed StateNext tasks. Duplicate IDs keep
// their first (earliest) position.
func FocusQueueRank(queue []config.FocusQueueItem) map[string]int {
	rank := make(map[string]int, len(queue))
	for i, item := range queue {
		if _, seen := rank[item.TaskID]; !seen {
			rank[item.TaskID] = i
		}
	}
	return rank
}

// RankOrUnindexed returns the queue rank for taskID, or the unindexed sentinel
// (len(rank)) when the task is not in the order index. Every indexed member has
// a rank in [0, len(rank)), so unindexed StateNext tasks sort after all indexed
// members.
func RankOrUnindexed(rank map[string]int, taskID string) int {
	if r, ok := rank[taskID]; ok {
		return r
	}
	return len(rank)
}
