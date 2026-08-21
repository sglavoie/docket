package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

const (
	staleTaskThresholdDays            = 90
	staleSectionCollapseKey           = "_promoted:stale"
	staleProjectSectionCollapsePrefix = "_promoted_project:stale:"
)

type staleSectionBuild struct {
	groups    []projectRowGroup
	count     int
	projected map[TaskIdentity]bool
}

// taskIsStale reports whether ordinary open work has gone untouched for at
// least staleTaskThresholdDays local calendar days. UpdatedAt is the touch
// clock; StateChangedAt intentionally does not participate.
func taskIsStale(task *models.Task, now time.Time) bool {
	if now.IsZero() {
		now = time.Now()
	}
	if task == nil || task.IsTerminalState() || task.IsDeferredAt(now) {
		return false
	}
	today := util.StartOfDay(now)
	updated := util.StartOfDay(task.UpdatedAt.In(now.Location()))
	return util.DaysBetween(updated, today) >= staleTaskThresholdDays
}

// buildStaleSection promotes old untouched work into the sparse All-mode Stale
// lane. Every deliberate planning, review, or active-work claim is stronger
// than derived age, so those rows move rather than duplicate.
func (m *OutlinerModel) buildStaleSection(
	input OutlinerModeInput,
	pinned, today, next, review, doing map[TaskIdentity]bool,
) staleSectionBuild {
	input.Now = input.now()
	result := staleSectionBuild{projected: make(map[TaskIdentity]bool)}
	var rows []outlinerRow
	for _, candidate := range orderedModeTasks(input) {
		task, project := candidate.row.Task, candidate.row.Project
		if task == nil || project == nil {
			continue
		}
		identity := taskIdentityFor(task, project)
		if !taskIsStale(task, input.Now) || pinned[identity] || today[identity] ||
			next[identity] || review[identity] || doing[identity] {
			continue
		}
		if input.TaskExclude != nil && input.TaskExclude(task, project) {
			continue
		}
		if input.TaskFilter != nil && !input.TaskFilter(task, project) {
			continue
		}
		row := m.outlinerRowFromModeRow(candidate.row)
		row.Breadcrumb = taskAncestorBreadcrumb(task)
		row.hasVisibleChildren = false
		result.projected[identity] = true
		result.count++
		rows = append(rows, row)
	}
	result.groups = m.groupSectionRowsByProject(rows, sectionForestOptions{rankRootsBySubtree: true})
	return result
}

func (m *OutlinerModel) appendStaleSection(section staleSectionBuild) {
	if section.count == 0 || !m.sectionZoomAdmits(staleSectionCollapseKey) {
		return
	}
	m.rows = append(m.rows, outlinerRow{
		IsSectionHeader: true,
		SectionName:     "Stale",
		SectionCount:    section.count,
		CollapseKey:     staleSectionCollapseKey,
	})
	if m.sectionCollapsed(staleSectionCollapseKey) {
		return
	}
	m.appendProjectGroupedForests(
		section.groups,
		outlinerRow{},
		func(projectID string) string {
			return staleProjectSectionCollapsePrefix + projectID
		},
	)
}
