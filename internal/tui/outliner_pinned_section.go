package tui

const (
	pinnedSectionName                  = "Pinned"
	pinnedSectionCollapseKey           = "_synthetic:pinned"
	pinnedProjectSectionCollapsePrefix = "_pinned_project:"
)

// pinnedProjectGroup holds one project's section-local portion of the transient
// Pinned projection. The forest retains hierarchy among projected relatives;
// the group header supplies project context in the cross-project Outliner.
type pinnedProjectGroup struct {
	projectID string
	rows      []outlinerRow
	forest    sectionRowForest
}

// pinnedSectionBuild carries the transient Pinned project groups and the
// qualified identity set used to suppress those same tasks from the active mode
// below.
type pinnedSectionBuild struct {
	groups    []pinnedProjectGroup
	count     int
	projected map[TaskIdentity]bool
}

// buildPinnedSection realizes Pinned from the same scoped candidates, reference
// time, and session filters as the active mode. Projected relatives form a
// section-local tree; roots whose parent is outside Pinned keep their ancestor
// breadcrumb. Project groups get a synthetic header in the unscoped Outliner.
func (m *OutlinerModel) buildPinnedSection(input OutlinerModeInput) pinnedSectionBuild {
	if input.TaskFilter == nil {
		input.TaskFilter = m.workModeTaskFilterAt(input.Now)
	}
	projection := buildPinnedModeProjection(input)
	result := pinnedSectionBuild{
		groups:    make([]pinnedProjectGroup, 0, len(projection.rows)),
		count:     len(projection.rows),
		projected: projection.projected,
	}
	groupIndexes := make(map[string]int, len(projection.rows))
	for _, modeRow := range projection.rows {
		row := m.outlinerRowFromModeRow(modeRow)
		row.hasVisibleChildren = false
		row.IsPinnedSection = true

		projectID := ""
		if modeRow.Project != nil {
			projectID = modeRow.Project.ID
		}
		groupIndex, ok := groupIndexes[projectID]
		if !ok {
			groupIndex = len(result.groups)
			groupIndexes[projectID] = groupIndex
			result.groups = append(result.groups, pinnedProjectGroup{projectID: projectID})
		}
		result.groups[groupIndex].rows = append(result.groups[groupIndex].rows, row)
	}
	for groupIndex := range result.groups {
		group := &result.groups[groupIndex]
		group.forest = m.buildSectionRowForest(group.rows)
		group.rows = nil
	}
	return result
}

func pinnedProjectSectionCollapseKey(projectID string) string {
	return pinnedProjectSectionCollapsePrefix + projectID
}

func (m *OutlinerModel) appendPinnedSection(section pinnedSectionBuild) {
	if section.count == 0 {
		return
	}
	m.rows = append(m.rows, outlinerRow{
		IsSectionHeader: true,
		IsPinnedSection: true,
		SectionName:     pinnedSectionName,
		SectionCount:    section.count,
		CollapseKey:     pinnedSectionCollapseKey,
	})
	if m.collapsedSections[pinnedSectionCollapseKey] {
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
			IsPinnedSection: true,
			ProjectID:       group.projectID,
			SectionCount:    group.forest.count,
			CollapseKey:     pinnedProjectSectionCollapseKey(group.projectID),
		}
		m.rows = append(m.rows, header)
		if !m.isSectionRowCollapsed(header) {
			m.flattenSectionRowForest(group.forest, &m.rows)
		}
	}
}
