package tui

import "github.com/sglavoie/tt/internal/models"

// projectTasksGroup pairs a project ID with the filtered task roots for that
// project, preserving project identity through section aggregation so that
// each flattened task row can carry its owning project's ID.
type projectTasksGroup struct {
	projectID string
	tasks     []*models.Task
}

// buildCrossProjectRowsWithSectionHeaders groups tasks from multiple projects
// under canonical GTD section headers. One shared header is emitted per
// section; within each non-collapsed section, a synthetic project sub-header
// is emitted before that project's task rows so that the All perspective
// visually clusters tasks first by section and then by project.
// Empty sections are skipped. Task rows carry the ProjectID of their owning
// project (for color badge rendering); section header rows have no ProjectID
// so that no per-project label is rendered on the shared header, while
// project sub-header rows carry the owning project's ID for later renderer
// resolution (display name/slug) and navigation skipping.
func (m *OutlinerModel) buildCrossProjectRowsWithSectionHeaders(groups []projectTasksGroup) {
	// Bucket tasks by section within each project, preserving project identity.
	type sectionGroup struct {
		projectID string
		forest    *sectionTaskForest
	}
	sectionsPerProject := make(map[string][]sectionGroup)
	for _, g := range groups {
		for section, forest := range buildSectionTaskForests(g.tasks) {
			sectionsPerProject[section] = append(sectionsPerProject[section], sectionGroup{
				projectID: g.projectID,
				forest:    forest,
			})
		}
	}

	for _, sectionName := range BuildCrossProjectSectionOrder(groups) {
		projGroups := sectionsPerProject[sectionName]
		if len(projGroups) == 0 {
			continue
		}

		// Emit ONE shared section header for all projects in this section.
		// Per requirements, tasks from multiple projects appear under the same
		// shared section header; per-row project identity uses colored slug badges.
		totalCount := 0
		for _, pg := range projGroups {
			totalCount += pg.forest.count
		}
		// "_xp:" is the authoritative sentinel prefix for All-perspective
		// cross-project section headers. Because each section header is shared
		// across all projects (one header per canonical section, no per-project
		// sub-header), a fixed namespace is sufficient and avoids collision with
		// single-project keys (which fall back to bare sectionName). This diverges
		// from the "projectID:sectionName" composite described in design decision
		// 019cfd3c014f7003, which applies to non-All perspectives where per-project
		// grouping would require distinct keys — not to the shared All-perspective
		// headers here.
		collapseKey := "_xp:" + sectionName
		m.rows = append(m.rows, outlinerRow{
			IsSectionHeader: true,
			SectionName:     sectionName,
			SectionCount:    totalCount,
			CollapseKey:     collapseKey,
		})

		if !m.collapsedSections[collapseKey] {
			for _, pg := range projGroups {
				// Emit a synthetic, render-only project sub-header before each
				// project's task rows. This row carries the project's ID so the
				// renderer can resolve the display name/slug, but it owns no
				// task and is skipped by keyboard navigation and bulk
				// operations (see outlinerRow.isTaskRow). Task hierarchy and
				// project data are not mutated — sub-headers exist purely for
				// visual grouping.
				m.rows = append(m.rows, outlinerRow{
					IsProjectHeader: true,
					ProjectID:       pg.projectID,
				})
				m.flattenSectionForest(pg.forest, pg.projectID, &m.rows)
			}
		}
	}
}

// buildProjectGroupedRows builds m.rows for non-All perspectives: each visible
// project emits one synthetic IsProjectHeader row carrying the project's ID,
// followed by the project's task rows flattened with project identity
// preserved. No section headers, horizontal rules, or inter-project separators
// are used; project headers themselves provide the visual grouping.
func (m *OutlinerModel) buildProjectGroupedRows(groups []projectTasksGroup) {
	for _, g := range groups {
		m.rows = append(m.rows, outlinerRow{
			IsProjectHeader: true,
			ProjectID:       g.projectID,
		})
		m.flattenTaskTreeWithFoldsAndProjectID(g.tasks, nil, g.projectID, &m.rows)
	}
}

// flattenTaskTreeWithFoldsAndProjectID is like flattenTaskTreeWithFolds but
// sets ProjectID on every emitted task row. This preserves project identity
// through tree recursion so color badges and navigation can resolve the owning
// project without a secondary lookup.
func (m *OutlinerModel) flattenTaskTreeWithFoldsAndProjectID(
	tasks []*models.Task,
	ancestorIsLast []bool,
	projectID string,
	rows *[]outlinerRow,
) {
	for i, task := range tasks {
		isLast := i == len(tasks)-1
		prefix := ""
		if len(ancestorIsLast) > 0 || len(tasks) > 0 {
			prefix = buildTreePrefix(ancestorIsLast, isLast)
		}
		*rows = append(*rows, outlinerRow{
			task:               task,
			prefix:             prefix,
			hasVisibleChildren: len(task.Children) > 0,
			ProjectID:          projectID,
		})

		if len(task.Children) > 0 && m.isExpanded(task.ID) {
			child := make([]bool, len(ancestorIsLast)+1)
			copy(child, ancestorIsLast)
			child[len(ancestorIsLast)] = isLast
			m.flattenTaskTreeWithFoldsAndProjectID(task.Children, child, projectID, rows)
		}
	}
}
