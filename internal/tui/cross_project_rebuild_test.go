package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCrossProjectRebuildRowsProjectIDInvariant(t *testing.T) {
	projAlpha := &models.Project{
		ID:   "proj-alpha",
		Name: "Alpha",
		Tasks: []*models.Task{
			makeTask("ta1", "Alpha task 1", ""),
			makeTask("ta2", "Alpha task 2", models.SectionNext),
		},
	}
	projAlpha.BuildTaskTree()

	projBeta := &models.Project{
		ID:   "proj-beta",
		Name: "Beta",
		Tasks: []*models.Task{
			makeTask("tb1", "Beta task 1", models.SectionInbox),
			makeTask("tb2", "Beta task 2", models.SectionNext),
		},
	}
	projBeta.BuildTaskTree()

	taskOwner := make(map[string]string)
	for _, proj := range []*models.Project{projAlpha, projBeta} {
		for _, task := range proj.Tasks {
			taskOwner[task.ID] = proj.ID
		}
	}

	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: 1,
		loadedProjects:    []*models.Project{projAlpha, projBeta},
	}

	m.crossProjectRebuildRows()

	if len(m.rows) == 0 {
		t.Fatal("crossProjectRebuildRows produced no rows")
	}

	for i, row := range m.rows {
		switch {
		case row.IsSectionHeader:
			if row.ProjectID != "" {
				t.Errorf("row[%d] IsSectionHeader but has non-empty ProjectID %q", i, row.ProjectID)
			}
		case row.IsSeparator:
			if row.ProjectID != "" {
				t.Errorf("row[%d] IsSeparator but has non-empty ProjectID %q", i, row.ProjectID)
			}
		default:
			if row.ProjectID == "" {
				title := ""
				if row.task != nil {
					title = row.task.Title
				}
				t.Errorf("row[%d] task row %q has empty ProjectID", i, title)
			}
			if row.task != nil {
				wantProjectID := taskOwner[row.task.ID]
				if wantProjectID != "" && row.ProjectID != wantProjectID {
					t.Errorf("row[%d] task %q: ProjectID want %q, got %q", i, row.task.Title, wantProjectID, row.ProjectID)
				}
			}
		}
	}
}

func TestCrossProjectRebuildRowsPerspectiveRouting(t *testing.T) {
	projAlpha := &models.Project{
		ID:   "route-alpha",
		Name: "Alpha",
		Tasks: []*models.Task{
			makeTask("ra1", "Alpha task", ""),
		},
	}
	projAlpha.BuildTaskTree()

	projBeta := &models.Project{
		ID:   "route-beta",
		Name: "Beta",
		Tasks: []*models.Task{
			makeTask("rb1", "Beta task", ""),
		},
	}
	projBeta.BuildTaskTree()

	t.Run("All perspective routes to buildCrossProjectRowsWithSectionHeaders", func(t *testing.T) {
		m := &OutlinerModel{
			collapsedSections: make(map[string]bool),
			expandedNodes:     make(map[string]bool),
			MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
			activePerspective: PerspectiveAll,
			loadedProjects:    []*models.Project{projAlpha, projBeta},
		}

		m.crossProjectRebuildRows()

		if len(m.rows) == 0 {
			t.Fatal("expected non-empty rows for All perspective")
		}

		sectionHeaderCount := 0
		separatorCount := 0
		for _, row := range m.rows {
			if row.IsSectionHeader {
				sectionHeaderCount++
			}
			if row.IsSeparator {
				separatorCount++
			}
		}

		if sectionHeaderCount == 0 {
			t.Error("All perspective: expected at least one IsSectionHeader row")
		}
		if separatorCount != 0 {
			t.Errorf("All perspective: expected zero IsSeparator rows, got %d", separatorCount)
		}
	})

	t.Run("non-All perspective routes to buildProjectGroupedRows", func(t *testing.T) {
		p1task := makeTask("np1", "P1 active task", "")
		p1task.Status = models.StatusInProgress
		p1 := &models.Project{
			ID:    "nonall-p1",
			Name:  "P1",
			Tasks: []*models.Task{p1task},
		}
		p1.BuildTaskTree()

		p2task := makeTask("np2", "P2 active task", "")
		p2task.Status = models.StatusInProgress
		p2 := &models.Project{
			ID:    "nonall-p2",
			Name:  "P2",
			Tasks: []*models.Task{p2task},
		}
		p2.BuildTaskTree()

		m := &OutlinerModel{
			collapsedSections: make(map[string]bool),
			expandedNodes:     make(map[string]bool),
			MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
			activePerspective: PerspectiveAll + 1,
			loadedProjects:    []*models.Project{p1, p2},
		}

		m.crossProjectRebuildRows()

		if len(m.rows) == 0 {
			t.Fatal("expected non-empty rows for non-All perspective with two projects")
		}

		sectionHeaderCount := 0
		separatorCount := 0
		for _, row := range m.rows {
			if row.IsSectionHeader {
				sectionHeaderCount++
			}
			if row.IsSeparator {
				separatorCount++
			}
		}

		if separatorCount == 0 {
			t.Error("non-All perspective with two projects: expected at least one IsSeparator row")
		}
		if sectionHeaderCount != 0 {
			t.Errorf("non-All perspective: expected zero IsSectionHeader rows, got %d", sectionHeaderCount)
		}
	})
}

func TestCrossProjectGroupingByPerspective(t *testing.T) {
	task1 := makeTask("g1", "Task 1", "")
	task1.Status = models.StatusInProgress
	proj1 := &models.Project{ID: "grp-p1", Name: "P1", Tasks: []*models.Task{task1}}
	proj1.BuildTaskTree()

	task2 := makeTask("g2", "Task 2", "")
	task2.Status = models.StatusInProgress
	proj2 := &models.Project{ID: "grp-p2", Name: "P2", Tasks: []*models.Task{task2}}
	proj2.BuildTaskTree()

	newModel := func(perspective int) *OutlinerModel {
		return &OutlinerModel{
			collapsedSections: make(map[string]bool),
			expandedNodes:     make(map[string]bool),
			MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
			activePerspective: perspective,
			loadedProjects:    []*models.Project{proj1, proj2},
		}
	}

	t.Run("All perspective produces section-grouped rows", func(t *testing.T) {
		m := newModel(PerspectiveAll)
		m.crossProjectRebuildRows()

		var sectionHeaders, separators int
		for _, row := range m.rows {
			if row.IsSectionHeader {
				sectionHeaders++
			}
			if row.IsSeparator {
				separators++
			}
		}
		if sectionHeaders == 0 {
			t.Error("expected IsSectionHeader rows for All perspective, got none")
		}
		if separators != 0 {
			t.Errorf("expected no IsSeparator rows for All perspective, got %d", separators)
		}
	})

	t.Run("non-All perspective produces project-grouped rows with IsSeparator", func(t *testing.T) {
		m := newModel(PerspectiveAll + 1)
		m.crossProjectRebuildRows()

		var sectionHeaders, separators int
		for _, row := range m.rows {
			if row.IsSectionHeader {
				sectionHeaders++
			}
			if row.IsSeparator {
				separators++
			}
		}
		if separators == 0 {
			t.Error("expected IsSeparator rows between project groups for non-All perspective, got none")
		}
		if sectionHeaders != 0 {
			t.Errorf("expected no IsSectionHeader rows for non-All perspective, got %d", sectionHeaders)
		}
	})
}

func TestCrossProjectRebuildRowsActionableFilter(t *testing.T) {
	// Active task in empty section (actionable)
	actionableTask := makeTask("act-1", "Actionable task", "")
	actionableTask.Status = models.StatusTodo

	// Task in Inbox section (not actionable)
	inboxTask := makeTask("inbox-1", "Inbox task", models.SectionInbox)
	inboxTask.Status = models.StatusTodo

	// Blocked task (not actionable)
	blockedTask := makeTask("blk-1", "Blocked task", "")
	blockedTask.Status = models.StatusBlocked

	// Task with incomplete dependency (not actionable)
	blocker := makeTask("blocker-1", "Blocker", "")
	blocker.Status = models.StatusTodo
	depTask := makeTask("dep-1", "Depends on blocker", "")
	depTask.Status = models.StatusTodo
	depTask.DependsOn = []string{"blocker-1"}

	proj := &models.Project{
		ID:   "proj-actionable",
		Name: "ActionableTest",
		Tasks: []*models.Task{
			actionableTask, inboxTask, blockedTask, blocker, depTask,
		},
	}
	proj.BuildTaskTree()

	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: PerspectiveAll,
		loadedProjects:    []*models.Project{proj},
		filter:            NewFilterState(),
	}

	// Without actionable filter, all tasks should appear
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	allTaskTitles := collectCrossProjectTaskTitles(m)
	if !allTaskTitles["Actionable task"] {
		t.Error("expected Actionable task without filter")
	}
	if !allTaskTitles["Inbox task"] {
		t.Error("expected Inbox task without filter")
	}
	if !allTaskTitles["Blocked task"] {
		t.Error("expected Blocked task without filter")
	}

	// Enable actionable filter
	m.filter.Actionable = true
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	filteredTitles := collectCrossProjectTaskTitles(m)

	if !filteredTitles["Actionable task"] {
		t.Error("actionable filter should keep actionable task")
	}
	if !filteredTitles["Blocker"] {
		t.Error("actionable filter should keep blocker (it has no blockers itself)")
	}
	if filteredTitles["Inbox task"] {
		t.Error("actionable filter should hide Inbox task")
	}
	if filteredTitles["Blocked task"] {
		t.Error("actionable filter should hide blocked task")
	}
	if filteredTitles["Depends on blocker"] {
		t.Error("actionable filter should hide task with incomplete dependencies")
	}
}

func TestCrossProjectRebuildRowsSectionFilter(t *testing.T) {
	nextTask := makeTask("next-1", "Next task", models.SectionNext)
	inboxTask := makeTask("inbox-1", "Inbox task", models.SectionInbox)
	activeTask := makeTask("active-1", "Active task", "")

	proj := &models.Project{
		ID:    "proj-section-filter",
		Name:  "SectionFilterTest",
		Tasks: []*models.Task{nextTask, inboxTask, activeTask},
	}
	proj.BuildTaskTree()

	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: PerspectiveAll,
		loadedProjects:    []*models.Project{proj},
		filter:            NewFilterState(),
	}

	// Enable section filter for Next only
	m.filter.Sections[models.SectionNext] = true
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	titles := collectCrossProjectTaskTitles(m)

	if !titles["Next task"] {
		t.Error("section filter should keep Next task")
	}
	if titles["Inbox task"] {
		t.Error("section filter should hide Inbox task")
	}
	if titles["Active task"] {
		t.Error("section filter should hide Active task (empty section)")
	}
}

func collectCrossProjectTaskTitles(m *OutlinerModel) map[string]bool {
	titles := make(map[string]bool)
	for _, row := range m.rows {
		if !row.IsSectionHeader && !row.IsSeparator && row.task != nil {
			titles[row.task.Title] = true
		}
	}
	return titles
}

func TestSearchTagOnlyMatchConsistency(t *testing.T) {
	taggedTask := makeTask("tag-task-1", "Unrelated title", "")
	taggedTask.Tags = []string{"backend", "api"}

	untaggedTask := makeTask("tag-task-2", "Another unrelated title", "")

	proj := &models.Project{
		ID:    "proj-tag-test",
		Name:  "TagTest",
		Tasks: []*models.Task{taggedTask, untaggedTask},
	}
	proj.BuildTaskTree()

	const query = "backend"

	singleM := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: PerspectiveAll,
		project:           proj,
	}
	singleM.SetSearchQuery(query)
	singleM.rebuildRows()

	singleTaskTitles := make(map[string]bool)
	for _, row := range singleM.rows {
		if !row.IsSectionHeader && !row.IsSeparator && row.task != nil {
			singleTaskTitles[row.task.Title] = true
		}
	}

	if !singleTaskTitles[taggedTask.Title] {
		t.Error("single-project outliner: tag-matched task not found in rows (tag-only search returned no match)")
	}
	if singleTaskTitles[untaggedTask.Title] {
		t.Error("single-project outliner: untagged task should not appear when query matches neither its title nor tags")
	}

	crossM := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: PerspectiveAll,
		loadedProjects:    []*models.Project{proj},
	}
	crossM.SetSearchQuery(query)
	crossM.crossProjectRebuildRows()

	crossTaskTitles := make(map[string]bool)
	for _, row := range crossM.rows {
		if !row.IsSectionHeader && !row.IsSeparator && row.task != nil {
			crossTaskTitles[row.task.Title] = true
		}
	}

	if !crossTaskTitles[taggedTask.Title] {
		t.Error("cross-project outliner: tag-matched task not found in rows (tag-only search returned no match)")
	}
	if crossTaskTitles[untaggedTask.Title] {
		t.Error("cross-project outliner: untagged task should not appear when query matches neither its title nor tags")
	}
}

func TestCrossHandleEnterKeyCollapseTogglesRows(t *testing.T) {
	proj := &models.Project{
		ID:   "proj-toggle",
		Name: "Toggle",
		Tasks: []*models.Task{
			makeTask("t1", "Task one", models.SectionInbox),
			makeTask("t2", "Task two", models.SectionInbox),
		},
	}
	proj.BuildTaskTree()

	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: PerspectiveAll,
		loadedProjects:    []*models.Project{proj},
	}
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()

	countTaskRows := func() int {
		n := 0
		for _, r := range m.rows {
			if !r.IsSectionHeader && !r.IsSeparator && r.task != nil {
				n++
			}
		}
		return n
	}

	initialTaskRows := countTaskRows()
	if initialTaskRows == 0 {
		t.Fatal("expected at least one task row after initial build")
	}

	headerIdx := -1
	for i, r := range m.rows {
		if r.IsSectionHeader {
			headerIdx = i
			break
		}
	}
	if headerIdx == -1 {
		t.Fatal("no section header row found; cannot test collapse toggle")
	}
	m.cursor = headerIdx

	_, _, handled := m.crossHandleEnterKey()
	if !handled {
		t.Fatal("crossHandleEnterKey returned handled=false on section header")
	}
	afterCollapse := countTaskRows()
	if afterCollapse >= initialTaskRows {
		t.Errorf("after collapse: want fewer than %d task rows, got %d", initialTaskRows, afterCollapse)
	}

	for i, r := range m.rows {
		if r.IsSectionHeader {
			m.cursor = i
			break
		}
	}

	_, _, handled = m.crossHandleEnterKey()
	if !handled {
		t.Fatal("crossHandleEnterKey returned handled=false on second toggle")
	}
	afterExpand := countTaskRows()
	if afterExpand != initialTaskRows {
		t.Errorf("after expand: want %d task rows, got %d", initialTaskRows, afterExpand)
	}
}
