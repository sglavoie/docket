package tui

const (
	reviewSectionCollapseKey           = "_promoted:review"
	reviewProjectSectionCollapsePrefix = "_promoted_project:review:"
)

type reviewSectionBuild struct {
	groups    []pinnedProjectGroup
	count     int
	projected map[TaskIdentity]bool
}

// buildReviewSection promotes ordinary open tasks carrying #review into the
// sparse All-mode Review lane. Pinned remains the stronger projection.
func (m *OutlinerModel) buildReviewSection(input OutlinerModeInput, pinned map[TaskIdentity]bool) reviewSectionBuild {
	result := reviewSectionBuild{projected: make(map[TaskIdentity]bool)}
	groupIndexes := make(map[string]int)
	for _, candidate := range orderedModeTasks(input) {
		task, project := candidate.row.Task, candidate.row.Project
		if task == nil || project == nil || task.IsTerminalState() || task.IsSnoozedAt(input.now()) ||
			!task.HasTag(reviewTaskTag) || pinned[taskIdentityFor(task, project)] {
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
		identity := taskIdentityFor(task, project)
		result.projected[identity] = true
		result.count++
		index, ok := groupIndexes[project.ID]
		if !ok {
			index = len(result.groups)
			groupIndexes[project.ID] = index
			result.groups = append(result.groups, pinnedProjectGroup{projectID: project.ID})
		}
		result.groups[index].rows = append(result.groups[index].rows, row)
	}
	for index := range result.groups {
		group := &result.groups[index]
		group.forest = m.buildSectionRowForest(group.rows)
		group.rows = nil
	}
	return result
}

func (m *OutlinerModel) appendReviewSection(section reviewSectionBuild) {
	if section.count == 0 {
		return
	}
	m.rows = append(m.rows, outlinerRow{
		IsSectionHeader: true,
		SectionName:     "Review",
		SectionCount:    section.count,
		CollapseKey:     reviewSectionCollapseKey,
		PromotedTag:     reviewTaskTag,
	})
	if m.collapsedSections[reviewSectionCollapseKey] {
		return
	}
	showProjectHeaders := m.project == nil && !m.isProjectScoped()
	for _, group := range section.groups {
		if !showProjectHeaders {
			m.flattenSectionRowForest(group.forest, &m.rows)
			continue
		}
		header := outlinerRow{
			IsProjectHeader: true,
			ProjectID:       group.projectID,
			SectionCount:    group.forest.count,
			CollapseKey:     reviewProjectSectionCollapsePrefix + group.projectID,
			PromotedTag:     reviewTaskTag,
		}
		m.rows = append(m.rows, header)
		if !m.isSectionRowCollapsed(header) {
			m.flattenSectionRowForest(group.forest, &m.rows)
		}
	}
}
