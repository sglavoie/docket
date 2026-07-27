package tui

import "github.com/sglavoie/tt/internal/models"

const (
	todaySectionName        = "Today"
	todaySectionCollapseKey = "_planning:today"
)

type todaySectionBuild struct {
	rows             []outlinerRow
	suppressed       map[TaskIdentity]bool
	visiblePlanCount int
}

// buildTodaySection realizes the Daily Plan as a synthetic section inside the
// All mode. Membership is constrained by the canonical application planning
// invariant; snoozed entries stay hidden by default but an explicit snoozed
// filter may reveal them without mutating the durable plan.
func (m *OutlinerModel) buildTodaySection(
	projects []*models.Project,
	candidates map[string][]*models.Task,
) todaySectionBuild {
	result := todaySectionBuild{suppressed: make(map[TaskIdentity]bool)}
	if len(projects) == 0 {
		return result
	}

	input := m.buildOutlinerModeInput(projects)
	input.CandidateTasks = candidates
	input.TaskExclude = func(task *models.Task, _ *models.Project) bool {
		return !planningTodayEligible(task) ||
			(task.IsSnoozedAt(input.Now) && !m.hasExplicitSnoozedFilter())
	}
	input.TaskFilter = m.workModeTaskFilter()

	// Suppress every explicit plan member from the normal workflow forest even
	// when it is snoozed (and therefore intentionally absent from Today). This
	// prevents a context-preserving tree filter from reintroducing a snoozed
	// planned parent above an otherwise visible child.
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range candidates[project.ID] {
			if task == nil {
				continue
			}
			identity := taskIdentityFor(task, project)
			if input.DailyPlanIDs[identity] && planningTodayEligible(task) {
				result.suppressed[identity] = true
			}
		}
	}

	modeRows := BuildTodayModeRows(input)
	if len(modeRows) == 0 {
		return result
	}

	rootRowIndexes := make([]int, 0, len(modeRows))
	for _, modeRow := range modeRows {
		if modeRow.Task == nil || modeRow.Project == nil {
			continue
		}
		identity := taskIdentityFor(modeRow.Task, modeRow.Project)
		row := m.outlinerRowFromModeRow(modeRow)
		row.IsTodaySection = true
		row.IsDailyPlanMember = input.DailyPlanIDs[identity]
		if row.IsDailyPlanMember {
			result.visiblePlanCount++
		}
		if modeRow.Depth == 0 {
			rootRowIndexes = append(rootRowIndexes, len(result.rows))
		}
		result.suppressed[identity] = true
		result.rows = append(result.rows, row)
	}

	// The shared Today row builder emits roots without a section-level tree
	// connector. Inside All, give those roots the same connector language
	// as every other built-in section while retaining the existing descendant
	// prefixes and open-before-terminal ordering.
	for i, rowIndex := range rootRowIndexes {
		result.rows[rowIndex].prefix = buildTreePrefix(nil, i == len(rootRowIndexes)-1)
		nextIndex := rowIndex + 1
		result.rows[rowIndex].hasVisibleChildren =
			nextIndex < len(result.rows) && !result.rows[nextIndex].IsDailyPlanMember
	}

	return result
}

func (m *OutlinerModel) appendTodaySection(section todaySectionBuild) {
	if section.visiblePlanCount == 0 || len(section.rows) == 0 {
		return
	}
	m.rows = append(m.rows, outlinerRow{
		IsSectionHeader: true,
		IsTodaySection:  true,
		SectionName:     todaySectionName,
		SectionCount:    section.visiblePlanCount,
		StateGroup:      models.StateGroupToday,
		CollapseKey:     todaySectionCollapseKey,
	})
	if !m.collapsedSections[todaySectionCollapseKey] {
		m.rows = append(m.rows, section.rows...)
	}
}

// removeTodayRowsFromTree removes every identity represented by Today or
// explicitly suppressed from the normal workflow forest (such as a snoozed
// plan member). Children not represented by Today are promoted so filtering
// never strands a visible task behind a removed parent.
func removeTodayRowsFromTree(
	tasks []*models.Task,
	projectID string,
	suppressed map[TaskIdentity]bool,
) []*models.Task {
	if len(tasks) == 0 || len(suppressed) == 0 {
		return tasks
	}
	result := make([]*models.Task, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		children := removeTodayRowsFromTree(task.Children, projectID, suppressed)
		if suppressed[TaskIdentity{ProjectID: projectID, TaskID: task.ID}] {
			result = append(result, children...)
			continue
		}
		clone := *task
		clone.Children = children
		result = append(result, &clone)
	}
	return result
}
