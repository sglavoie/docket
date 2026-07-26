package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// buildCrossPriorityRows aggregates tasks from all per-project groups into a
// flat list, classifies each into a PriorityGroup (Overdue, Today, This Week,
// Later, Backlog, Blocked/Waiting), sorts each group by urgency, and emits
// one shared section header per non-empty group followed by its task rows.
// Each task row carries its owning project's ID so badges resolve correctly,
// plus its PriorityTask wrapper so cached metadata such as ParentTitle remains
// attached after the grouped priority tasks are sorted into flat rows.
//
// Per-group collapse state is read from m.crossPriorityCollapsedGroups (session
// only). Blocked/Waiting is seeded collapsed on mode entry by
// enterCrossPriorityMode and rendered with a collapse-aware header so the
// count is always visible even when its tasks are hidden.
func (m *OutlinerModel) buildCrossPriorityRows(groups []projectTasksGroup) {
	type ownedPriorityTask struct {
		projectID string
		entry     *PriorityTask
	}

	buckets := make(map[PriorityGroup][]ownedPriorityTask, len(CrossPriorityGroupOrder()))

	for _, g := range groups {
		proj := m.projectByID(g.projectID)
		if proj == nil {
			continue
		}
		predicate := m.crossPriorityRowPredicate(proj)
		var visit func(tasks []*models.Task)
		visit = func(tasks []*models.Task) {
			for _, task := range tasks {
				if predicate(task) {
					pt := &PriorityTask{
						Task:         task,
						UrgencyScore: ComputeUrgencyScore(task, taskIsNext(task)),
						RelativeDue:  FormatRelativeDue(task.DueDate, false),
					}
					if task.ParentID != nil {
						if parent := proj.GetTaskByID(*task.ParentID); parent != nil {
							pt.ParentTitle = parent.Title
						}
					}
					group := CategorizeByDueDate(task)
					if IsPriorityBlocked(proj, task) {
						group = PriorityGroupWaitingOn
					}
					buckets[group] = append(buckets[group], ownedPriorityTask{
						projectID: g.projectID,
						entry:     pt,
					})
				}
				if len(task.Children) > 0 {
					visit(task.Children)
				}
			}
		}
		visit(g.tasks)
	}

	for _, group := range CrossPriorityGroupOrder() {
		entries := buckets[group]
		if len(entries) == 0 {
			continue
		}

		// Stable urgency sort matches single-project priority semantics so
		// ties (common with the coarse integer score) keep insertion order.
		flat := make([]*PriorityTask, len(entries))
		for i, e := range entries {
			flat[i] = e.entry
		}
		SortPriorityTasksByUrgency(flat)

		// Rebuild ownedPriorityTask order so projectID stays aligned with the
		// sorted entries.
		owners := make(map[*PriorityTask]string, len(entries))
		for _, e := range entries {
			owners[e.entry] = e.projectID
		}

		collapseKey := crossPriorityCollapseKey(group)
		m.rows = append(m.rows, outlinerRow{
			IsSectionHeader: true,
			SectionName:     priorityGroupHeaderLabel(group),
			SectionCount:    len(flat),
			CollapseKey:     collapseKey,
		})

		if m.crossPriorityCollapsedGroups[group] {
			continue
		}

		for _, pt := range flat {
			m.rows = append(m.rows, outlinerRow{
				task:         pt.Task,
				priorityTask: pt,
				ProjectID:    owners[pt],
			})
		}
	}
}

// projectByID returns the loaded project with the given ID, or nil if not
// loaded. Used by the priority row builder so dependency-aware blocking checks
// run against the owning project.
func (m *OutlinerModel) projectByID(id string) *models.Project {
	for _, p := range m.loadedProjects {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// crossPriorityRowPredicate returns the per-task predicate that decides
// whether a task is emitted as a flat row in cross-project Priority mode. It
// combines the same filters the normal cross-project path applies (pinned
// tag, search, common filters, saved criteria, actionable) but
// judges each task individually instead of relying on filterTaskTree's
// context-preserving parent rule. The owning project is required for
// criteria, dependency, and project-scoped common-filter checks.
func (m *OutlinerModel) crossPriorityRowPredicate(proj *models.Project) func(*models.Task) bool {
	pinnedTag := m.activePinnedTagFilter()
	query := m.SearchQuery()
	hasCommon := hasActiveCommonFilters(&m.filter.CommonFilterState)
	criteria := m.activeCriteriaFilter
	actionable := m.filter.Actionable

	return func(t *models.Task) bool {
		if !IsEligibleForPriority(t) {
			return false
		}
		if pinnedTag != "" && !t.HasTag(pinnedTag) {
			return false
		}
		if query != "" && !matchesSearch(t, query, m.runtime.Planning()) {
			return false
		}
		if hasCommon && !passesCommonFilters(t, &m.filter.CommonFilterState, proj, m.runtime.Automation()) {
			return false
		}
		if criteria != nil && !passesSavedFilterCriteria(t, proj, criteria, m.runtime.Automation()) {
			return false
		}
		if actionable && !taskIsActionableState(t, proj) {
			return false
		}
		return true
	}
}
