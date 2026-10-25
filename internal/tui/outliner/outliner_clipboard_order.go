package outliner

import (
	"sort"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// ClipboardSelectionOrder is the order a copied selection reads in: the members
// with a row in the current projection first, in displayed row order, then the
// members the projection hides, by project slug and then project-tree position.
// It deduplicates and drops nothing.
//
// It is deliberately local to the clipboard operations rather than folded into
// the shared target normalization. That normalization sorts by identity because
// the order a mutation applies in is not something anyone reads; copied text is
// the one case where the order IS the result, and only there is it worth paying
// a projection lookup to produce it.
//
// Explicit selection is the whole contract: a selected parent brings no
// descendant along, so a hidden child of a selected parent is copied only when
// it was selected in its own right.
func (m *OutlinerModel) ClipboardSelectionOrder(
	identities []taskref.TaskIdentity,
) []taskref.TaskIdentity {
	pending := make(map[taskref.TaskIdentity]bool, len(identities))
	for _, identity := range identities {
		if identity.Valid() {
			pending[identity] = true
		}
	}
	ordered := make([]taskref.TaskIdentity, 0, len(pending))
	if m != nil {
		for _, visible := range m.VisibleTaskIdentities() {
			if !pending[visible] {
				continue
			}
			delete(pending, visible)
			ordered = append(ordered, visible)
		}
	}
	return append(ordered, m.hiddenClipboardOrder(pending)...)
}

// hiddenClipboardOrder sorts the selected identities that no visible row
// represents. They have no displayed position to inherit, so they take the
// workspace's own: project slug, then the project's tree order, then the task
// ID as the final tiebreak, which is what keeps the result stable when a
// project no longer resolves and neither of the first two keys is available.
func (m *OutlinerModel) hiddenClipboardOrder(
	pending map[taskref.TaskIdentity]bool,
) []taskref.TaskIdentity {
	hidden := make([]taskref.TaskIdentity, 0, len(pending))
	for identity := range pending {
		hidden = append(hidden, identity)
	}
	slugs := make(map[string]string, len(hidden))
	ranks := make(map[string]map[string]int, len(hidden))
	for _, identity := range hidden {
		if _, resolved := slugs[identity.ProjectID]; resolved {
			continue
		}
		// The project ID stands in for a slug that cannot be read, so identities
		// from one project stay adjacent even when nothing resolves them.
		slugs[identity.ProjectID] = identity.ProjectID
		if m == nil {
			continue
		}
		_, project := m.TaskAndProjectForIdentity(identity)
		if project == nil {
			continue
		}
		if project.Slug != "" {
			slugs[identity.ProjectID] = project.Slug
		}
		ranks[identity.ProjectID] = projectTreeRanks(project)
	}
	rankOf := func(identity taskref.TaskIdentity) (int, bool) {
		rank, ok := ranks[identity.ProjectID][identity.TaskID]
		return rank, ok
	}
	sort.Slice(hidden, func(i, j int) bool {
		left, right := hidden[i], hidden[j]
		if slugs[left.ProjectID] != slugs[right.ProjectID] {
			return slugs[left.ProjectID] < slugs[right.ProjectID]
		}
		leftRank, leftRanked := rankOf(left)
		rightRank, rightRanked := rankOf(right)
		if leftRanked != rightRanked {
			return leftRanked
		}
		if leftRanked && leftRank != rightRank {
			return leftRank < rightRank
		}
		return left.TaskID < right.TaskID
	})
	return hidden
}

// projectTreeRanks numbers a project's tasks depth-first from its roots, which
// is the order the Outliner would draw them with every fold open. A task the
// walk cannot reach still gets a position, after everything reachable and in
// stored order, so a broken parent link degrades the order instead of dropping
// the task from the copied text.
func projectTreeRanks(project *models.Project) map[string]int {
	ranks := make(map[string]int, len(project.Tasks))
	var walk func(tasks []*models.Task)
	walk = func(tasks []*models.Task) {
		for _, task := range tasks {
			if task == nil {
				continue
			}
			if _, seen := ranks[task.ID]; seen {
				continue
			}
			ranks[task.ID] = len(ranks)
			walk(task.Children)
		}
	}
	walk(project.GetRootTasks())
	for _, task := range project.Tasks {
		if task == nil {
			continue
		}
		if _, seen := ranks[task.ID]; !seen {
			ranks[task.ID] = len(ranks)
		}
	}
	return ranks
}
