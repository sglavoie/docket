package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestRebuildRows_AllPerspective(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 1, collapsedSections: make(map[string]bool)}
	m.rebuildRows()
	assertRowTitles(t, m.rows, []string{
		"[Active]", "Epic", "Story A", "Task A1", "Story B", "Task B1", "Bug Root",
	})
}

func TestRebuildRows_AllPerspectiveGroupsCustomSectionsUnderActive(t *testing.T) {
	customTask := &models.Task{
		ID:            "custom",
		Title:         "Sprint task",
		Status:        models.StatusTodo,
		CustomSection: "Sprint",
	}
	nextTask := &models.Task{
		ID:      "next",
		Title:   "Next task",
		Status:  models.StatusTodo,
		Section: models.SectionNext,
	}
	proj := &models.Project{
		ID:       "project",
		Name:     "Test",
		Sections: []string{models.SectionNext, "Sprint"},
		Tasks:    []*models.Task{customTask, nextTask},
	}

	m := &OutlinerModel{project: proj, activePerspective: 1, collapsedSections: make(map[string]bool)}
	m.rebuildRows()

	assertRowTitles(t, m.rows, []string{"[Active]", "Sprint task", "[Next]", "Next task"})
}

func TestRebuildRows_CustomSectionTabMatchesLegacySectionValue(t *testing.T) {
	legacyTask := &models.Task{
		ID:      "legacy",
		Title:   "Legacy task",
		Status:  models.StatusTodo,
		Section: "Legacy",
	}
	otherTask := &models.Task{
		ID:            "other",
		Title:         "Other task",
		Status:        models.StatusTodo,
		CustomSection: "Sprint",
	}
	proj := &models.Project{
		ID:       "project",
		Name:     "Test",
		Sections: []string{"Legacy", "Sprint"},
		Tasks:    []*models.Task{legacyTask, otherTask},
	}

	m := &OutlinerModel{
		project:           proj,
		activePerspective: 1,
		collapsedSections: make(map[string]bool),
		customSectionTab:  map[string]int{proj.ID: 1},
	}
	m.rebuildRows()

	assertRowTitles(t, m.rows, []string{"[Active]", "Legacy task"})
}

func TestRebuildRows_ActivePerspective(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 2}
	m.rebuildRows()
	assertRowTitles(t, m.rows, []string{"Epic", "Story A"})
}

func TestRebuildRows_NextPerspective(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 3}
	m.rebuildRows()
	assertRowTitles(t, m.rows, []string{"Epic", "Story A", "Task A1", "Bug Root"})
}

func TestRebuildRows_InboxPerspective(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 4}
	m.rebuildRows()
	assertRowTitles(t, m.rows, []string{"Epic", "Story A", "Task A1", "Bug Root"})
}

func TestRebuildRows_StandbyPerspective(t *testing.T) {
	proj := buildTestProject()
	standbyTask := &models.Task{
		ID:       "standby1",
		ShortID:  8,
		Type:     models.TaskTypeTask,
		Title:    "Standby Task",
		Status:   models.StatusStandby,
		Children: []*models.Task{},
	}
	proj.Tasks = append(proj.Tasks, standbyTask)

	m := &OutlinerModel{project: proj, activePerspective: 5}
	m.rebuildRows()
	assertRowTitles(t, m.rows, []string{"Standby Task"})
}

func TestRebuildRows_WaitingPerspective(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 6}
	m.rebuildRows()
	assertRowTitles(t, m.rows, []string{"Epic", "Story B"})
}

func TestRebuildRows_SomedayPerspective(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 7}
	m.rebuildRows()
	assertRowTitles(t, m.rows, []string{})
}

func TestRebuildRows_CompletedPerspective(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 8}
	m.rebuildRows()
	assertRowTitles(t, m.rows, []string{"Epic", "Story A", "Task A2"})
}

func TestRebuildRows_ReviewPerspective(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 9}
	m.rebuildRows()
	assertRowTitles(t, m.rows, []string{"Epic", "Story B", "Task B1"})
}

func TestRebuildRows_SnoozedPerspective(t *testing.T) {
	proj := buildTestProject()
	tomorrow := time.Now().Add(24 * time.Hour)
	for _, task := range proj.Tasks {
		if task.ID == "bugRoot" {
			task.SnoozedUntil = &tomorrow
		}
	}

	m := &OutlinerModel{project: proj, activePerspective: 0}
	m.rebuildRows()
	assertRowTitles(t, m.rows, []string{"Bug Root"})
}

func TestRebuildRows_AllPerspectiveExcludesSnoozed(t *testing.T) {
	proj := buildTestProject()
	tomorrow := time.Now().Add(24 * time.Hour)
	for _, task := range proj.Tasks {
		if task.ID == "bugRoot" {
			task.SnoozedUntil = &tomorrow
		}
	}

	m := &OutlinerModel{project: proj, activePerspective: 1, collapsedSections: make(map[string]bool)}
	m.rebuildRows()
	assertRowTitles(t, m.rows, []string{
		"[Active]", "Epic", "Story A", "Task A1", "Story B", "Task B1",
	})
}

func TestRebuildRows_ParentExcludedWhenNoDescendantsMatch(t *testing.T) {
	parent := &models.Task{
		ID:       "parent",
		ShortID:  1,
		Type:     models.TaskTypeEpic,
		Title:    "Parent",
		Status:   models.StatusTodo,
		Priority: 3,
		Children: []*models.Task{},
	}
	child := &models.Task{
		ID:       "child",
		ShortID:  2,
		Type:     models.TaskTypeTask,
		Title:    "Child",
		Status:   models.StatusTodo,
		Priority: 3,
		Children: []*models.Task{},
		ParentID: &parent.ID,
	}
	parent.Children = []*models.Task{child}

	proj := &models.Project{
		Name:  "Test",
		Tasks: []*models.Task{parent, child},
	}

	m := &OutlinerModel{project: proj, activePerspective: 2}
	m.rebuildRows()
	if len(m.rows) != 0 {
		t.Fatalf("expected 0 rows when no tasks match, got %d: %v", len(m.rows), rowTitles(m.rows))
	}
}

func TestRebuildRows_ParentPreservedForMatchingDescendant(t *testing.T) {
	grandparent := &models.Task{
		ID:       "gp",
		ShortID:  1,
		Type:     models.TaskTypeEpic,
		Title:    "Grandparent",
		Status:   models.StatusTodo,
		Priority: 3,
		Children: []*models.Task{},
	}
	mid := &models.Task{
		ID:       "mid",
		ShortID:  2,
		Type:     models.TaskTypeStory,
		Title:    "Middle",
		Status:   models.StatusTodo,
		Priority: 3,
		Children: []*models.Task{},
		ParentID: &grandparent.ID,
	}
	leaf := &models.Task{
		ID:       "leaf",
		ShortID:  3,
		Type:     models.TaskTypeTask,
		Title:    "Leaf",
		Status:   models.StatusDone,
		Priority: 3,
		Children: []*models.Task{},
		ParentID: &mid.ID,
	}
	mid.Children = []*models.Task{leaf}
	grandparent.Children = []*models.Task{mid}

	proj := &models.Project{
		Name:  "Test",
		Tasks: []*models.Task{grandparent, mid, leaf},
	}

	m := &OutlinerModel{project: proj, activePerspective: 8}
	m.rebuildRows()
	assertRowTitles(t, m.rows, []string{"Grandparent", "Middle", "Leaf"})
}

func TestRebuildRows_DefaultPerspectiveIsAll(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: -1, collapsedSections: make(map[string]bool)}
	m.rebuildRows()
	if len(m.rows) != 7 {
		t.Fatalf("default perspective should show 7 rows (1 header + 6 tasks), got %d", len(m.rows))
	}
	if !m.rows[0].IsSectionHeader {
		t.Errorf("first row should be section header")
	}
}

func TestRebuildRows_CursorClampedAfterFilter(t *testing.T) {
	m := &OutlinerModel{project: buildTestProject(), activePerspective: 1, collapsedSections: make(map[string]bool)}
	m.rebuildRows()
	m.cursor = 6

	m.activePerspective = 7
	m.rebuildRows()
	if m.cursor != 0 {
		t.Errorf("cursor should clamp to 0 when rows empty, got %d", m.cursor)
	}
}
