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
// section; tasks from all projects in that section appear underneath it.
// Empty sections are skipped. Task rows carry the ProjectID of their owning
// project (for color badge rendering); header rows have no ProjectID so that
// no per-project label is rendered on the shared header.
func (m *OutlinerModel) buildCrossProjectRowsWithSectionHeaders(groups []projectTasksGroup) {
	// Bucket tasks by section within each project, preserving project identity.
	type sectionGroup struct {
		projectID string
		tasks     []*models.Task
	}
	sectionsPerProject := make(map[string][]sectionGroup)
	for _, g := range groups {
		bySection := make(map[string][]*models.Task)
		for _, t := range g.tasks {
			bySection[t.Section] = append(bySection[t.Section], t)
		}
		for section, tasks := range bySection {
			sectionsPerProject[section] = append(sectionsPerProject[section], sectionGroup{
				projectID: g.projectID,
				tasks:     tasks,
			})
		}
	}

	for _, sectionName := range SectionOrder {
		projGroups := sectionsPerProject[sectionName]
		if len(projGroups) == 0 {
			continue
		}

		// Emit ONE shared section header for all projects in this section.
		// Per requirements, tasks from multiple projects appear under the same
		// shared section header; per-row project identity uses color badges only.
		totalCount := 0
		for _, pg := range projGroups {
			totalCount += m.countTasksRecursive(pg.tasks)
		}
		collapseKey := sectionName
		m.rows = append(m.rows, outlinerRow{
			IsSectionHeader: true,
			SectionName:     sectionName,
			SectionCount:    totalCount,
			CollapseKey:     collapseKey,
		})

		if !m.collapsedSections[collapseKey] {
			for _, pg := range projGroups {
				m.flattenTaskTreeWithFoldsAndProjectID(pg.tasks, nil, pg.projectID, &m.rows)
			}
		}
	}
}

// buildProjectGroupedRows builds m.rows for non-All perspectives: each
// project's tasks are flattened independently with project identity preserved,
// and a single IsSeparator row is inserted between consecutive project groups.
// No section headers or horizontal rules are used.
func (m *OutlinerModel) buildProjectGroupedRows(groups []projectTasksGroup) {
	for i, g := range groups {
		if i > 0 {
			m.rows = append(m.rows, outlinerRow{IsSeparator: true})
		}
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
			task:      task,
			prefix:    prefix,
			ProjectID: projectID,
		})

		if len(task.Children) > 0 && m.isExpanded(task.ID) {
			child := make([]bool, len(ancestorIsLast)+1)
			copy(child, ancestorIsLast)
			child[len(ancestorIsLast)] = isLast
			m.flattenTaskTreeWithFoldsAndProjectID(task.Children, child, projectID, rows)
		}
	}
}
