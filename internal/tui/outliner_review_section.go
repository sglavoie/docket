package tui

const (
	reviewSectionCollapseKey           = "_promoted:review"
	reviewProjectSectionCollapsePrefix = "_promoted_project:review:"
)

type reviewSectionBuild struct {
	groups    []projectRowGroup
	count     int
	projected map[TaskIdentity]bool
}

// buildReviewSection promotes ordinary open tasks carrying #review into the
// sparse All-mode Review lane. Pinning and explicit Today/Next membership are
// stronger claims: a deliberate planning commitment outranks a descriptive tag,
// so planning a review-tagged task moves its row instead of duplicating it.
func (m *OutlinerModel) buildReviewSection(
	input OutlinerModeInput,
	pinned, today, next map[TaskIdentity]bool,
) reviewSectionBuild {
	result := reviewSectionBuild{projected: make(map[TaskIdentity]bool)}
	var rows []outlinerRow
	for _, candidate := range orderedModeTasks(input) {
		task, project := candidate.row.Task, candidate.row.Project
		identity := taskIdentityFor(task, project)
		if task == nil || project == nil || task.IsTerminalState() || task.IsDeferredAt(input.now()) ||
			!task.HasTag(reviewTaskTag) ||
			pinned[identity] || today[identity] || next[identity] {
			continue
		}
		if input.TaskExclude != nil && input.TaskExclude(task, project) {
			continue
		}
		if input.TaskFilter != nil && !input.TaskFilter(task, project) {
			continue
		}
		row := m.outlinerRowFromModeRow(candidate.row)
		row.PromotedTag = reviewTaskTag
		row.Breadcrumb = taskAncestorBreadcrumb(task)
		row.hasVisibleChildren = false
		result.projected[identity] = true
		result.count++
		rows = append(rows, row)
	}
	result.groups = m.groupSectionRowsByProject(rows, sectionForestOptions{rankRootsBySubtree: true})
	return result
}

func (m *OutlinerModel) appendReviewSection(section reviewSectionBuild) {
	if section.count == 0 || !m.sectionZoomAdmits(reviewSectionCollapseKey) {
		return
	}
	m.rows = append(m.rows, outlinerRow{
		IsSectionHeader: true,
		SectionName:     "Review",
		SectionCount:    section.count,
		CollapseKey:     reviewSectionCollapseKey,
		PromotedTag:     reviewTaskTag,
	})
	if m.sectionCollapsed(reviewSectionCollapseKey) {
		return
	}
	m.appendProjectGroupedForests(
		section.groups,
		outlinerRow{PromotedTag: reviewTaskTag},
		func(projectID string) string {
			return reviewProjectSectionCollapsePrefix + projectID
		},
	)
}
