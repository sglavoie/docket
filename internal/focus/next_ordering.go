package focus

import "github.com/sglavoie/tt/internal/config"

// NextOrderRank maps each project-qualified task in the durable Next order to
// its 0-based position. The ordered collection itself defines membership.
// Duplicate qualified entries keep their first (earliest) position.
func NextOrderRank(order []config.ProjectTaskLocator) map[config.ProjectTaskLocator]int {
	rank := make(map[config.ProjectTaskLocator]int, len(order))
	for i, locator := range order {
		if _, seen := rank[locator]; !seen {
			rank[locator] = i
		}
	}
	return rank
}

// RankOrUnindexed returns the queue rank for locator, or the unindexed sentinel
// (len(rank)) when the task is not in the order index. Every indexed member has
// a rank in [0, len(rank)), so unindexed tasks sort after all indexed members.
func RankOrUnindexed(rank map[config.ProjectTaskLocator]int, locator config.ProjectTaskLocator) int {
	if r, ok := rank[locator]; ok {
		return r
	}
	return len(rank)
}
