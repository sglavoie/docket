package outliner

import (
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/theme"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// planningTodayEligible is kept local to the row pipeline so every visual
// caller uses the application layer's canonical planning invariant.
func planningTodayEligible(task *models.Task) bool {
	return task != nil && app.PlanningTodayEligible(task.State())
}

// buildPinnedModeProjection selects visible pinned tasks from the current
// scope, orders them newest-pin-first, and carries ancestor breadcrumbs for
// their eventual synthetic-section rendering. Session 22 owns attaching that
// section to each mode; this helper owns the visibility and ordering contract.
func buildPinnedModeProjection(input OutlinerModeInput) pinnedModeProjection {
	input.Now = input.now()
	projection := pinnedModeProjection{projected: make(map[taskref.TaskIdentity]bool)}
	var candidates []orderedModeTask
	for _, candidate := range orderedModeTasks(input) {
		task, project := candidate.row.Task, candidate.row.Project
		if task == nil || project == nil || !task.Pinned || !isComputedOpenWork(task, input) {
			continue
		}
		if input.TaskExclude != nil && input.TaskExclude(task, project) {
			continue
		}
		if input.TaskFilter != nil && !input.TaskFilter(task, project) {
			continue
		}
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if c := comparePinnedAt(left.row.Task, right.row.Task); c != 0 {
			return c < 0
		}
		return compareOrderedModeTasks(left, right) < 0
	})
	projection.rows = make([]OutlinerModeRow, 0, len(candidates))
	for _, candidate := range candidates {
		row := candidate.row
		row.Breadcrumb = taskAncestorBreadcrumb(row.Task)
		identity := taskref.For(row.Task, row.Project)
		projection.projected[identity] = true
		projection.rows = append(projection.rows, row)
	}
	return projection
}

// comparePinnedAt is pin order: newest pin first, a task without a pin instant
// last.
func comparePinnedAt(a, b *models.Task) int {
	switch {
	case a.PinnedAt != nil && b.PinnedAt == nil:
		return -1
	case a.PinnedAt == nil && b.PinnedAt != nil:
		return 1
	case a.PinnedAt == nil && b.PinnedAt == nil:
		return 0
	}
	return b.PinnedAt.Compare(*a.PinnedAt)
}

func (m *OutlinerModel) buildPrimaryModeRowsWithInput(
	projects []*models.Project,
	input OutlinerModeInput,
	pinned map[taskref.TaskIdentity]bool,
) {
	mode := m.PrimaryMode
	if !isPrimaryOutlinerMode(mode) {
		return
	}

	if mode == chrome.ModeAll || mode == chrome.ModeAgenda {
		m.buildAllPrimaryRows(projects, input, pinned)
		return
	}
	if mode == chrome.ModeToday || mode == chrome.ModeNext {
		m.appendPlanningModeRows(mode, input, pinned)
		return
	}

	switch mode {
	case chrome.ModeInbox, chrome.ModeBacklog:
	default:
		return
	}
	m.flattenPrimaryProjectTrees(m.buildPrimaryProjectTrees(projects, mode, input, pinned), mode)
}

func (m *OutlinerModel) buildAllPrimaryRows(
	projects []*models.Project,
	input OutlinerModeInput,
	pinned map[taskref.TaskIdentity]bool,
) {
	today := m.buildTodaySection(input, pinned)
	next := m.buildNextSection(input, pinned, today.suppressed)
	doing := projectedDoingTasks(input)
	review := m.buildReviewSection(input, pinned, today.suppressed, next.suppressed, doing)

	grouped := make(map[models.WorkflowStateGroup][]stateGroupProjectForest)
	for _, project := range projects {
		if project == nil {
			continue
		}
		roots := m.primaryModeRoots(project)
		roots = excludeTaskTreePromotingChildren(roots, m.primaryDefaultExclusion(input.Now))
		roots = filterTaskTreePromotingChildren(roots, func(task *models.Task) bool {
			return input.TaskFilter == nil || input.TaskFilter(task, project)
		})
		roots = removePlanningRowsFromTree(roots, project.ID, today.suppressed)
		roots = removePlanningRowsFromTree(roots, project.ID, next.suppressed)
		roots = removeProjectedRowsFromTree(roots, project.ID, pinned)
		roots = removeProjectedRowsFromTree(roots, project.ID, review.projected)
		roots = m.sortPrimaryTaskTree(roots)
		for group, forest := range buildStateGroupTaskForests(roots) {
			if forest == nil || forest.count == 0 {
				continue
			}
			grouped[group] = append(grouped[group], stateGroupProjectForest{
				project: project,
				forest:  forest,
			})
		}
	}

	if m.PrimaryMode == chrome.ModeAgenda {
		m.agendaLeftOut = agendaLeftOutLanes(grouped)
	}
	lifted := liftedLaneCounts(input, pinned, review.projected, today.suppressed, next.suppressed)
	for _, group := range m.allPrimaryGroupOrder() {
		switch group {
		case models.StateGroupToday:
			m.appendPlanningSection(today, lifted[group])
		case models.StateGroupNext:
			m.appendPlanningSection(next, lifted[group])
		default:
			m.appendStateGroupForests(group, grouped[group], lifted[group])
		}
		if group == models.StateGroupDoing {
			m.appendReviewSection(review)
		}
	}
}

// LiftedCounts is how many of a group's members another lane drew instead of
// that group: Pinned and Review, which promote a task above its lane, and Today
// and Next, which seat a planned task in its plan. The four are disjoint: Review
// never claims a pinned task, and a pinned or reviewed task counts once, under
// the lane that drew it.
type LiftedCounts struct {
	Pinned int
	Today  int
	Next   int
	Review int
}

// Any reports whether the group lost a member to another lane.
func (c LiftedCounts) Any() bool { return c.Pinned > 0 || c.Today > 0 || c.Next > 0 || c.Review > 0 }

// liftedLaneCounts counts, per All group, the members another lane drew instead
// of that group. The workspace pulse and the Now card count a state wherever its
// rows are drawn, so a lane header naming what was lifted out of it is what lets
// `Doing (1) +2 in Today` agree with `+2 more in Doing` and `Inbox (1)` with
// `inbox 2`. A pinned or reviewed planned task belongs to its plan's group, which
// is where All would otherwise have drawn it; a planned task neither pin nor
// review claimed is lifted out of its state's group into its plan.
func liftedLaneCounts(
	input OutlinerModeInput,
	pinned, review, today, next map[taskref.TaskIdentity]bool,
) map[models.WorkflowStateGroup]LiftedCounts {
	lifted := make(map[models.WorkflowStateGroup]LiftedCounts)
	for _, candidate := range orderedModeTasks(input) {
		task, project := candidate.row.Task, candidate.row.Project
		if task == nil || project == nil {
			continue
		}
		identity := taskref.For(task, project)
		group := theme.TaskStateGroup(task)
		if !pinned[identity] && !review[identity] {
			switch counts := lifted[group]; {
			case today[identity]:
				counts.Today++
				lifted[group] = counts
			case next[identity]:
				counts.Next++
				lifted[group] = counts
			}
			continue
		}
		switch {
		case today[identity]:
			group = models.StateGroupToday
		case next[identity]:
			group = models.StateGroupNext
		}
		counts := lifted[group]
		if pinned[identity] {
			counts.Pinned++
		} else {
			counts.Review++
		}
		lifted[group] = counts
	}
	return lifted
}

// projectedDoingTasks names the qualified rows claimed by the lifecycle Doing
// lane so derived sections can apply precedence without conflating state age
// with touch age. The section builders apply their own visibility guards.
func projectedDoingTasks(input OutlinerModeInput) map[taskref.TaskIdentity]bool {
	projected := make(map[taskref.TaskIdentity]bool)
	for _, candidate := range orderedModeTasks(input) {
		task, project := candidate.row.Task, candidate.row.Project
		if task == nil || project == nil || task.State() != models.StateDoing {
			continue
		}
		projected[taskref.For(task, project)] = true
	}
	return projected
}

func (m *OutlinerModel) appendPlanningModeRows(
	mode chrome.OutlinerModeID,
	input OutlinerModeInput,
	pinned map[taskref.TaskIdentity]bool,
) {
	// Today and Next render their planning body ungrouped, under no header of
	// their own, so no zoom key can ever name it. Today's Pinned section is the
	// one zoom reachable here and it must leave the body out entirely.
	if m.Focus.Zoom.Active() {
		return
	}
	var modeRows []OutlinerModeRow
	switch mode {
	case chrome.ModeToday:
		modeRows = BuildTodayModeRows(input)
	case chrome.ModeNext:
		modeRows = BuildNextModeRows(input)
	default:
		return
	}

	rows := make([]OutlinerRow, 0, len(modeRows))
	for _, modeRow := range modeRows {
		if modeRow.Task == nil || modeRow.Project == nil ||
			pinned[taskref.For(modeRow.Task, modeRow.Project)] {
			continue
		}
		row := m.outlinerRowFromModeRow(modeRow)
		row.IsTodaySection = mode == chrome.ModeToday
		row.IsNextSection = mode == chrome.ModeNext
		rows = append(rows, row)
	}
	group := models.StateGroupToday
	if mode == chrome.ModeNext {
		group = models.StateGroupNext
	}
	// The dedicated modes draw no section header, so their groups name the
	// planning project fold alone.
	m.appendPlanningRows(rows, group, "")
}

func (m *OutlinerModel) buildPrimaryProjectTrees(
	projects []*models.Project,
	mode chrome.OutlinerModeID,
	input OutlinerModeInput,
	pinned map[taskref.TaskIdentity]bool,
) []primaryProjectTree {
	var trees []primaryProjectTree
	for _, project := range projects {
		if project == nil {
			continue
		}
		roots := m.primaryModeRoots(project)
		hardExclude := m.primaryDefaultExclusion(input.Now)
		if mode == chrome.ModeBacklog {
			defaultExclude := hardExclude
			hardExclude = func(task *models.Task) bool {
				return defaultExclude(task) || input.TodayIDs[taskref.For(task, project)]
			}
		}
		roots = excludeTaskTreePromotingChildren(roots, hardExclude)

		membership := func(task *models.Task) bool {
			switch mode {
			case chrome.ModeInbox:
				return task.IsInState(models.StateInbox)
			case chrome.ModeBacklog:
				return task.IsInState(models.StateBacklog) &&
					!input.TodayIDs[taskref.For(task, project)]
			default:
				return false
			}
		}
		visible := func(task *models.Task) bool {
			return input.TaskFilter == nil || input.TaskFilter(task, project)
		}
		// Facets are task predicates, not ancestry requests. Apply them before
		// mode membership so every row that survives the later hierarchy
		// projection satisfies every active facet.
		roots = filterTaskTreePromotingChildren(roots, visible)
		// Inbox is a triage list, not a context-preserving search result. Once a
		// task leaves Inbox its row must disappear even when an Inbox descendant
		// survives. Promote that descendant to the nearest visible level while
		// retaining hierarchy between tasks that are themselves still in Inbox.
		if mode == chrome.ModeInbox {
			roots = excludeTaskTreePromotingChildren(roots, func(task *models.Task) bool {
				return !membership(task)
			})
		} else {
			// Backlog retains mode context for qualifying descendants, but the
			// strict facet projection above prevents nonmatching context rows.
			roots = filterTaskTreeKeepingAncestors(roots, membership)
		}
		roots = removeProjectedRowsFromTree(roots, project.ID, pinned)
		if len(roots) == 0 {
			continue
		}
		if mode == chrome.ModeInbox || mode == chrome.ModeBacklog {
			roots = m.sortPrimaryTaskTree(roots)
		}
		trees = append(trees, primaryProjectTree{project: project, roots: roots})
	}
	return trees
}

// removeProjectedRowsFromTree removes each project-qualified identity promoted
// into a synthetic section while preserving every non-promoted descendant in
// its original relative order. It is a pass over a tree the projection owns, so
// it shares what it leaves alone; see pruneOwnedTree.
func removeProjectedRowsFromTree(
	tasks []*models.Task,
	projectID string,
	projected map[taskref.TaskIdentity]bool,
) []*models.Task {
	if len(tasks) == 0 || len(projected) == 0 {
		return tasks
	}
	pruned, _ := pruneOwnedTree(tasks, func(task *models.Task) bool {
		return projected[taskref.TaskIdentity{ProjectID: projectID, TaskID: task.ID}]
	})
	return pruned
}

// Agenda is All's head, so the rest of All is work it chose not to draw rather
// than work that does not exist. The one line beneath its rows says how much
// sits in each lane it leaves out, so a growing Inbox or a Waiting task never
// leaves the screen without a trace, and names the key back to All.
//
// Like Today's canvas it is paint in the padding, never a row: it is not in
// m.Rows, takes no cursor, answers no click, and moves no window. It is drawn
// whole or not at all, from the rows the window left after the list.

const agendaLeftOutLead = "Not on the agenda: "

// agendaLeftOutLane is one lane Agenda leaves out and how many tasks it holds.
type agendaLeftOutLane struct {
	group models.WorkflowStateGroup
	count int
}

// agendaLeftOutLanes counts the forests All would have drawn in the lanes
// Agenda leaves out. They were built through the same filter, focus, and
// promotion passes as the drawn lanes, so a narrowed Agenda counts a narrowed
// remainder and a task lifted into Pinned or Review is never counted twice.
func agendaLeftOutLanes(grouped map[models.WorkflowStateGroup][]stateGroupProjectForest) []agendaLeftOutLane {
	var lanes []agendaLeftOutLane
	for _, group := range models.AgendaLeavesOutWorkflowStateGroups() {
		total := 0
		for _, item := range grouped[group] {
			if item.forest != nil {
				total += item.forest.count
			}
		}
		if total > 0 {
			lanes = append(lanes, agendaLeftOutLane{group: group, count: total})
		}
	}
	return lanes
}

// agendaLeftOutEligible reports whether the line may speak: an Agenda built as
// lanes, not a ranked list and not one lane zoomed, which leaves nothing beside
// it to compare against.
func (m *OutlinerModel) agendaLeftOutEligible() bool {
	return m.PrimaryMode == chrome.ModeAgenda && len(m.agendaLeftOut) > 0 &&
		!m.ShowsRankedList() && !m.Focus.Zoom.Active()
}

// agendaLeftOutLines is the line under a populated Agenda, after one blank
// row, when budget holds both. Lanes drop whole from the right to fit width;
// the line keeps its lead and the key back to All or is not drawn.
func (m *OutlinerModel) agendaLeftOutLines(width, budget int) []string {
	if budget < 2 || !m.agendaLeftOutEligible() {
		return nil
	}
	line := m.agendaLeftOutLine(width)
	if line == "" {
		return nil
	}
	return []string{"", line}
}

func (m *OutlinerModel) agendaLeftOutLine(width int) string {
	tail := ""
	if key := m.firstShortcut(operation.OperationCoreGotoAll); key != "" {
		tail = " · " + key + " All"
	}
	segments := make([]string, 0, len(m.agendaLeftOut))
	for _, lane := range m.agendaLeftOut {
		segments = append(segments, lane.group.Label()+" "+strconv.Itoa(lane.count))
	}
	for shown := len(segments); shown > 0; shown-- {
		text := agendaLeftOutLead + strings.Join(segments[:shown], " · ")
		if shown < len(segments) {
			text += " · …"
		}
		text += tail
		if lipgloss.Width(text) <= width {
			return theme.MutedStyle.Render(text)
		}
	}
	return ""
}
