package tui

const (
	pinnedSectionName                  = "Pinned"
	pinnedSectionCollapseKey           = "_synthetic:pinned"
	pinnedProjectSectionCollapsePrefix = "_pinned_project:"
)

// pinnedSectionBuild carries the transient Pinned project groups and the
// qualified identity set used to suppress those same tasks from the active mode
// below.
type pinnedSectionBuild struct {
	groups    []projectRowGroup
	count     int
	projected map[TaskIdentity]bool
}

// buildPinnedSection realizes Pinned from the same scoped candidates, reference
// time, and session filters as the active mode. Only modes declaring
// ShowsPinned call it. Projected relatives form a
// section-local tree; roots whose parent is outside Pinned keep their ancestor
// breadcrumb. Project groups get a synthetic header in the unscoped Outliner.
// A root ranks by its own pin position rather than its subtree's earliest one,
// so J/K swapping two adjacent rows moves exactly those rows.
func (m *OutlinerModel) buildPinnedSection(input OutlinerModeInput) pinnedSectionBuild {
	if input.TaskFilter == nil {
		input.TaskFilter = m.workModeTaskFilterAt(input.Now)
	}
	projection := buildPinnedModeProjection(input)
	rows := make([]outlinerRow, 0, len(projection.rows))
	for _, modeRow := range projection.rows {
		row := m.outlinerRowFromModeRow(modeRow)
		row.hasVisibleChildren = false
		row.IsPinnedSection = true
		rows = append(rows, row)
	}
	return pinnedSectionBuild{
		groups:    m.groupSectionRowsByProject(rows, sectionForestOptions{}),
		count:     len(projection.rows),
		projected: projection.projected,
	}
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

	m.appendProjectGroupedForests(
		section.groups,
		outlinerRow{IsPinnedSection: true},
		pinnedProjectSectionCollapseKey,
	)
}
