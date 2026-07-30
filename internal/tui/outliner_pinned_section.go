package tui

const (
	pinnedSectionName        = "Pinned"
	pinnedSectionCollapseKey = "_synthetic:pinned"
)

// pinnedSectionBuild carries the transient Pinned rows and the qualified
// identity set used to suppress those same tasks from the active mode below.
type pinnedSectionBuild struct {
	rows      []outlinerRow
	projected map[TaskIdentity]bool
}

// buildPinnedSection realizes Pinned from the same scoped candidates, reference
// time, and session filters as the active mode. It is deliberately flat:
// promoted tasks carry their project identity and ancestor breadcrumb rather
// than duplicating their durable hierarchy inside the synthetic section.
func (m *OutlinerModel) buildPinnedSection(input OutlinerModeInput) pinnedSectionBuild {
	if input.TaskFilter == nil {
		input.TaskFilter = m.workModeTaskFilterAt(input.Now)
	}
	projection := buildPinnedModeProjection(input)
	result := pinnedSectionBuild{
		rows:      make([]outlinerRow, 0, len(projection.rows)),
		projected: projection.projected,
	}
	for index, modeRow := range projection.rows {
		row := m.outlinerRowFromModeRow(modeRow)
		row.prefix = buildTreePrefix(nil, index == len(projection.rows)-1)
		row.hasVisibleChildren = false
		row.IsPinnedSection = true
		result.rows = append(result.rows, row)
	}
	return result
}

func (m *OutlinerModel) appendPinnedSection(section pinnedSectionBuild) {
	if len(section.rows) == 0 {
		return
	}
	m.rows = append(m.rows, outlinerRow{
		IsSectionHeader: true,
		IsPinnedSection: true,
		SectionName:     pinnedSectionName,
		SectionCount:    len(section.rows),
		CollapseKey:     pinnedSectionCollapseKey,
	})
	if !m.collapsedSections[pinnedSectionCollapseKey] {
		m.rows = append(m.rows, section.rows...)
	}
}
