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
// under canonical GTD section headers. Each project gets its own section header
// per section so that collapse state is tracked independently per project using
// composite "projectID:sectionName" keys. Empty sections are skipped. Task rows
// carry the ProjectID of their owning project; header rows carry the ProjectID
// of the owning project for context rendering.
func (m *OutlinerModel) buildCrossProjectRowsWithSectionHeaders(groups []projectTasksGroup) {
	// sectionOrder defines the canonical display order for the All perspective.
	sectionOrder := []string{
		"",
		models.SectionInbox,
		models.SectionNext,
		models.SectionWaiting,
		models.SectionReview,
		models.SectionStandby,
		models.SectionSomeday,
	}

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

	for _, sectionName := range sectionOrder {
		projGroups := sectionsPerProject[sectionName]
		if len(projGroups) == 0 {
			continue
		}

		for _, pg := range projGroups {
			collapseKey := pg.projectID + ":" + sectionName

			count := m.countTasksRecursive(pg.tasks)
			m.rows = append(m.rows, outlinerRow{
				IsSectionHeader: true,
				SectionName:     sectionName,
				SectionCount:    count,
				CollapseKey:     collapseKey,
				ProjectID:       pg.projectID,
			})

			if !m.collapsedSections[collapseKey] {
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
