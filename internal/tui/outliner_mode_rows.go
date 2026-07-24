package tui

import (
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
)

// OutlinerModeRow is the representation-neutral output of a purpose-built row
// strategy (Today, Next, Backlog). It carries only the data a renderer needs so
// the mode engine has a single row shape shared with the (still-operational)
// Planner rather than a second implementation. Priority/Source are derived by
// the caller when adapting to a concrete display row.
type OutlinerModeRow struct {
	Task       *models.Task
	Project    *models.Project
	TreePrefix string // non-empty for auto-expanded Today descendants
	ParentID   string // immediate parent task ID for Today descendants; empty for roots
	Depth      int    // 0 for roots; n for n-th level Today descendant
}

// OutlinerModeInput bundles the cross-project data a row strategy needs. It is
// the single seam through which Today, Next, and Backlog receive their inputs so
// the strategies stay pure and testable.
type OutlinerModeInput struct {
	Projects     []*models.Project
	DailyPlanIDs map[TaskIdentity]bool
	// CandidateTasks optionally narrows the tasks eligible to become Today
	// roots per project. A missing project entry falls back to Project.Tasks.
	CandidateTasks map[string][]*models.Task
	// DailyPlanOrder is the durable Today ordering (daily-plan positions).
	DailyPlanOrder []FocusTask
	// FocusQueue is the durable Next order index (StateNext ordering only).
	FocusQueue []config.FocusQueueItem
	// Scorer feeds the cross-project backlog ranker (project scoring profiles).
	Scorer *config.RuntimeState
	// ChildDepthLimit / FoldOverrides mirror the Planner's Today expansion knobs.
	// -1 means unlimited depth.
	ChildDepthLimit int
	FoldOverrides   map[TaskIdentity]bool
	// TaskFilter, when non-nil, narrows the Today section: a planned root or an
	// expanded descendant is kept only if it matches OR one of its rendered
	// descendants matches. Non-matching subtrees are pruned; the ancestor chain
	// of a matching descendant is retained as context so the tree stays
	// coherent, and tree connectors are recomputed from the surviving siblings.
	TaskFilter func(task *models.Task, project *models.Project) bool
	// TaskExclude is a hard subtree exclusion applied before TaskFilter's
	// ancestor-retention behavior. Today uses it to hide snoozed plan roots and
	// everything inherited solely through those roots.
	TaskExclude func(task *models.Task, project *models.Project) bool
	// Now is the reference time for the snooze check; the zero value uses
	// time.Now().
	Now time.Time
}

func (in OutlinerModeInput) now() time.Time {
	if in.Now.IsZero() {
		return time.Now()
	}
	return in.Now
}

// BuildTodayModeRows realizes the Today section: every daily-plan task (including
// completed tasks) as a root, ordered by state activeness then daily-plan
// position, with descendants expanded open-before-completed. It ports the
// Planner's loadItems/expandTodayChildren semantics verbatim.
func BuildTodayModeRows(in OutlinerModeInput) []OutlinerModeRow {
	var roots []OutlinerModeRow
	for _, project := range in.Projects {
		if project == nil {
			continue
		}
		tasks := project.Tasks
		if candidates, ok := in.CandidateTasks[project.ID]; ok {
			tasks = candidates
		}
		for _, task := range tasks {
			if task == nil {
				continue
			}
			if in.TaskExclude != nil && in.TaskExclude(task, project) {
				continue
			}
			if in.DailyPlanIDs[taskIdentityFor(task, project)] {
				roots = append(roots, OutlinerModeRow{Task: task, Project: project})
			}
		}
	}

	posMap := make(map[TaskIdentity]int, len(in.DailyPlanOrder))
	for i, ft := range in.DailyPlanOrder {
		posMap[taskIdentityFor(ft.Task, ft.Project)] = i
	}
	sort.SliceStable(roots, func(i, j int) bool {
		iOrd := TodayStateOrder(roots[i].Task.State())
		jOrd := TodayStateOrder(roots[j].Task.State())
		if iOrd != jOrd {
			return iOrd < jOrd
		}
		return posMap[taskIdentityFor(roots[i].Task, roots[i].Project)] <
			posMap[taskIdentityFor(roots[j].Task, roots[j].Project)]
	})

	isExpanded := todayModeExpanded(in.ChildDepthLimit, in.FoldOverrides)
	if in.TaskFilter == nil {
		if in.TaskExclude != nil {
			return expandFilteredTodayModeDescendants(
				roots,
				in.DailyPlanIDs,
				isExpanded,
				func(*models.Task, *models.Project) bool { return true },
				in.TaskExclude,
			)
		}
		return expandTodayModeDescendants(roots, in.DailyPlanIDs, isExpanded)
	}
	return expandFilteredTodayModeDescendants(roots, in.DailyPlanIDs, isExpanded, in.TaskFilter, in.TaskExclude)
}

// BuildNextModeRows realizes the Next horizon: every StateNext task (excluding
// terminal/snoozed, as the Planner does), ordered by the focus-queue index with
// unindexed members appended in stable input order.
func BuildNextModeRows(in OutlinerModeInput) []OutlinerModeRow {
	now := in.now()
	var rows []OutlinerModeRow
	for _, project := range in.Projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			if task.IsTerminalState() || task.IsSnoozedAt(now) {
				continue
			}
			if task.IsInState(models.StateNext) {
				rows = append(rows, OutlinerModeRow{Task: task, Project: project})
			}
		}
	}

	rank := focus.FocusQueueRank(in.FocusQueue)
	sort.SliceStable(rows, func(i, j int) bool {
		li := config.ProjectTaskLocator{ProjectID: rows[i].Project.ID, TaskID: rows[i].Task.ID}
		lj := config.ProjectTaskLocator{ProjectID: rows[j].Project.ID, TaskID: rows[j].Task.ID}
		return focus.RankOrUnindexed(rank, li) < focus.RankOrUnindexed(rank, lj)
	})
	return rows
}

// BuildBacklogModeRows realizes the Backlog horizon: actionable work minus Today
// (daily plan) and Next (StateNext), ranked by the shared cross-project scorer.
// The Today/Next exclusion happens before ranking so the visible order is
// invariant to how many same-project tasks sit in Today or Next.
func BuildBacklogModeRows(in OutlinerModeInput) []OutlinerModeRow {
	actionable := focus.CollectActionableTasksAt(in.Projects, in.now())
	candidates := make([]focus.FocusTask, 0, len(actionable))
	for _, ft := range actionable {
		if ft.Task == nil || ft.Project == nil {
			continue
		}
		if in.DailyPlanIDs[taskIdentityFor(ft.Task, ft.Project)] || ft.Task.IsInState(models.StateNext) {
			continue
		}
		candidates = append(candidates, ft)
	}

	ranked := focus.RankCrossProjectTasks(candidates, nil, in.Scorer)
	rows := make([]OutlinerModeRow, 0, len(ranked))
	for _, sft := range ranked {
		if sft.Task == nil || sft.Project == nil {
			continue
		}
		rows = append(rows, OutlinerModeRow{Task: sft.Task, Project: sft.Project})
	}
	return rows
}

// todayModeExpanded builds the per-node expansion predicate for the Today
// horizon, honoring explicit fold overrides before the depth limit. It is the
// single implementation shared with the Planner's isParentExpanded.
func todayModeExpanded(childDepthLimit int, foldOverrides map[TaskIdentity]bool) func(TaskIdentity, int) bool {
	return func(identity TaskIdentity, depth int) bool {
		if v, ok := foldOverrides[identity]; ok {
			return !v
		}
		if childDepthLimit < 0 {
			return true
		}
		return depth < childDepthLimit
	}
}

// expandTodayModeDescendants inserts descendants of each planned root into the
// row list depth-first, mirroring the Planner's expandTodayChildren.
func expandTodayModeDescendants(roots []OutlinerModeRow, dailyPlanIDs map[TaskIdentity]bool, isExpanded func(TaskIdentity, int) bool) []OutlinerModeRow {
	seen := make(map[TaskIdentity]bool, len(roots))
	for _, r := range roots {
		seen[taskIdentityFor(r.Task, r.Project)] = true
	}

	var expanded []OutlinerModeRow
	for _, r := range roots {
		expanded = append(expanded, r)
		if dailyPlanIDs[taskIdentityFor(r.Task, r.Project)] {
			expandPlannedTodayDescendants(r.Task, r.Project, nil, seen, isExpanded,
				func(child *models.Task, project *models.Project, treePrefix, parentID string, depth int) {
					expanded = append(expanded, OutlinerModeRow{
						Task:       child,
						Project:    project,
						TreePrefix: treePrefix,
						ParentID:   parentID,
						Depth:      depth,
					})
				})
		}
	}
	return expanded
}

// expandFilteredTodayModeDescendants is the filter-aware sibling of
// expandTodayModeDescendants. A planned root and each of its expanded
// descendants survive only when the task itself matches or one of its rendered
// descendants matches (ancestor retention). Pruning happens before tree
// connectors are assigned, so the surviving siblings drive the ├───/└───
// prefixes and the tree stays coherent.
func expandFilteredTodayModeDescendants(
	roots []OutlinerModeRow,
	dailyPlanIDs map[TaskIdentity]bool,
	isExpanded func(TaskIdentity, int) bool,
	filter func(*models.Task, *models.Project) bool,
	exclude func(*models.Task, *models.Project) bool,
) []OutlinerModeRow {
	seen := make(map[TaskIdentity]bool, len(roots))
	for _, r := range roots {
		seen[taskIdentityFor(r.Task, r.Project)] = true
	}

	// keep reports whether task's rendered subtree (task included) contains a
	// match. It respects the same seen set and expansion predicate the emission
	// uses, so a planned descendant surfaced as its own root does not keep its
	// ancestor alive.
	var keep func(task *models.Task, project *models.Project, depth int) bool
	keep = func(task *models.Task, project *models.Project, depth int) bool {
		if exclude != nil && exclude(task, project) {
			return false
		}
		if filter(task, project) {
			return true
		}
		if !isExpanded(taskIdentityFor(task, project), depth) {
			return false
		}
		for _, child := range task.Children {
			if seen[taskIdentityFor(child, project)] {
				continue
			}
			if keep(child, project, depth+1) {
				return true
			}
		}
		return false
	}

	var expanded []OutlinerModeRow
	for _, r := range roots {
		if !keep(r.Task, r.Project, 0) {
			continue
		}
		expanded = append(expanded, r)
		if dailyPlanIDs[taskIdentityFor(r.Task, r.Project)] {
			emitFilteredTodayDescendants(r.Task, r.Project, nil, seen, isExpanded, keep,
				func(child *models.Task, project *models.Project, treePrefix, parentID string, depth int) {
					expanded = append(expanded, OutlinerModeRow{
						Task:       child,
						Project:    project,
						TreePrefix: treePrefix,
						ParentID:   parentID,
						Depth:      depth,
					})
				})
		}
	}
	return expanded
}

// emitFilteredTodayDescendants mirrors expandPlannedTodayDescendants but drops
// children whose subtree contains no match before the open-before-completed
// partition and connector assignment, so the retained rows render as a coherent
// pruned tree.
func emitFilteredTodayDescendants(
	parent *models.Task,
	project *models.Project,
	ancestorIsLast []bool,
	seen map[TaskIdentity]bool,
	isExpanded func(identity TaskIdentity, depth int) bool,
	keep func(task *models.Task, project *models.Project, depth int) bool,
	emit func(child *models.Task, project *models.Project, treePrefix, parentID string, depth int),
) {
	if !isExpanded(taskIdentityFor(parent, project), len(ancestorIsLast)) {
		return
	}
	visible := make([]*models.Task, 0, len(parent.Children))
	for _, child := range parent.Children {
		if seen[taskIdentityFor(child, project)] {
			continue
		}
		seen[taskIdentityFor(child, project)] = true
		if keep(child, project, len(ancestorIsLast)+1) {
			visible = append(visible, child)
		}
	}
	sort.SliceStable(visible, func(i, j int) bool {
		return !visible[i].IsTerminalState() && visible[j].IsTerminalState()
	})
	for i, child := range visible {
		isLast := i == len(visible)-1
		emit(child, project, buildBoardTreePrefix(ancestorIsLast, isLast), parent.ID, len(ancestorIsLast)+1)
		next := make([]bool, len(ancestorIsLast)+1)
		copy(next, ancestorIsLast)
		next[len(ancestorIsLast)] = isLast
		emitFilteredTodayDescendants(child, project, next, seen, isExpanded, keep, emit)
	}
}

// expandPlannedTodayDescendants recursively emits descendants of a planned Today
// root, partitioning open children before completed ones at every depth and
// assigning tree connectors from the post-partition position. It is the single
// implementation of the Today child-expansion algorithm, shared by the Planner's
// collectChildren and the Outliner mode engine.
func expandPlannedTodayDescendants(
	parent *models.Task,
	project *models.Project,
	ancestorIsLast []bool,
	seen map[TaskIdentity]bool,
	isExpanded func(identity TaskIdentity, depth int) bool,
	emit func(child *models.Task, project *models.Project, treePrefix, parentID string, depth int),
) {
	if !isExpanded(taskIdentityFor(parent, project), len(ancestorIsLast)) {
		return
	}
	visible := make([]*models.Task, 0, len(parent.Children))
	for _, child := range parent.Children {
		if !seen[taskIdentityFor(child, project)] {
			visible = append(visible, child)
		}
	}
	sort.SliceStable(visible, func(i, j int) bool {
		return !visible[i].IsTerminalState() && visible[j].IsTerminalState()
	})
	for i, child := range visible {
		seen[taskIdentityFor(child, project)] = true
		isLast := i == len(visible)-1
		emit(child, project, buildBoardTreePrefix(ancestorIsLast, isLast), parent.ID, len(ancestorIsLast)+1)
		next := make([]bool, len(ancestorIsLast)+1)
		copy(next, ancestorIsLast)
		next[len(ancestorIsLast)] = isLast
		expandPlannedTodayDescendants(child, project, next, seen, isExpanded, emit)
	}
}
