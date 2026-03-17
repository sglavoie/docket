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
// under shared canonical GTD section headers. Each project's tasks are filtered
// independently before being merged into section buckets. Empty sections are
// skipped. Task rows carry the ProjectID of their owning project; header rows
// leave ProjectID empty.
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

	// Collect tasks per section across all projects, preserving project identity.
	type taggedTask struct {
		task      *models.Task
		projectID string
	}
	tasksBySection := make(map[string][]taggedTask)
	for _, g := range groups {
		for _, t := range g.tasks {
			tasksBySection[t.Section] = append(tasksBySection[t.Section], taggedTask{task: t, projectID: g.projectID})
		}
	}

	for _, sectionName := range sectionOrder {
		tagged := tasksBySection[sectionName]
		if len(tagged) == 0 {
			continue
		}

		// Count total tasks in this section (including children).
		count := 0
		for _, tt := range tagged {
			count += m.countTasksRecursive([]*models.Task{tt.task})
		}

		m.rows = append(m.rows, outlinerRow{
			IsSectionHeader: true,
			SectionName:     sectionName,
			SectionCount:    count,
		})

		if !m.collapsedSections[sectionName] {
			for _, tt := range tagged {
				m.flattenTaskTreeWithFoldsAndProjectID([]*models.Task{tt.task}, nil, tt.projectID, &m.rows)
			}
		}
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
