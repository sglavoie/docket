package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestPerspectivePresetCount(t *testing.T) {
	presets := PerspectivePresets()
	if len(presets) != 10 {
		t.Fatalf("expected 10 presets, got %d", len(presets))
	}
	for i, p := range presets {
		if p.Key != i {
			t.Errorf("preset %d: expected key %d, got %d", i, i, p.Key)
		}
		if p.Name == "" {
			t.Errorf("preset %d: empty name", i)
		}
	}
}

func TestPerspectiveByKey(t *testing.T) {
	for key := 0; key <= 9; key++ {
		p, ok := PerspectiveByKey(key)
		if !ok {
			t.Errorf("PerspectiveByKey(%d) returned false", key)
		}
		if p.Key != key {
			t.Errorf("PerspectiveByKey(%d).Key = %d", key, p.Key)
		}
	}

	for _, bad := range []int{-1, 10, 100} {
		_, ok := PerspectiveByKey(bad)
		if ok {
			t.Errorf("PerspectiveByKey(%d) should return false", bad)
		}
	}
}

func TestSnoozedPerspective(t *testing.T) {
	p, _ := PerspectiveByKey(0)
	if p.Name != "Snoozed" {
		t.Fatalf("expected 'Snoozed', got %q", p.Name)
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	snoozed := &models.Task{Status: models.StatusTodo, SnoozedUntil: &tomorrow}
	if !p.Filter(snoozed) {
		t.Error("Snoozed should accept snoozed tasks")
	}

	normal := &models.Task{Status: models.StatusTodo}
	if p.Filter(normal) {
		t.Error("Snoozed should reject non-snoozed tasks")
	}
}

func TestAllPerspective(t *testing.T) {
	p, _ := PerspectiveByKey(1)
	if p.Name != "All" {
		t.Fatalf("expected 'All', got %q", p.Name)
	}
	if p.Filter == nil {
		t.Fatal("All perspective should have a filter that excludes completed tasks")
	}
	if !p.Filter(&models.Task{Status: models.StatusTodo}) {
		t.Error("All should accept todo tasks")
	}
	if !p.Filter(&models.Task{Status: models.StatusInProgress}) {
		t.Error("All should accept in_progress tasks")
	}
	if p.Filter(&models.Task{Status: models.StatusDone}) {
		t.Error("All should reject done tasks")
	}
	if p.Filter(&models.Task{Status: models.StatusCanceled}) {
		t.Error("All should reject canceled tasks")
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	snoozed := &models.Task{Status: models.StatusTodo, SnoozedUntil: &tomorrow}
	if p.Filter(snoozed) {
		t.Error("All should reject snoozed tasks")
	}
}

func TestActivePerspective(t *testing.T) {
	p, _ := PerspectiveByKey(2)
	accept := &models.Task{Status: models.StatusInProgress}
	reject := []*models.Task{
		{Status: models.StatusTodo},
		{Status: models.StatusDone},
		{Status: models.StatusBlocked},
		{Status: models.StatusReview},
		{Status: models.StatusCanceled},
	}

	if !p.Filter(accept) {
		t.Error("Active should accept in_progress tasks")
	}
	for _, task := range reject {
		if p.Filter(task) {
			t.Errorf("Active should reject %s tasks", task.Status)
		}
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	snoozedActive := &models.Task{Status: models.StatusInProgress, SnoozedUntil: &tomorrow}
	if p.Filter(snoozedActive) {
		t.Error("Active should reject snoozed tasks")
	}
}

func TestNextPerspective(t *testing.T) {
	p, _ := PerspectiveByKey(3)

	for _, pri := range []int{1, 2} {
		task := &models.Task{Status: models.StatusTodo, Priority: pri}
		if !p.Filter(task) {
			t.Errorf("Next should accept todo priority %d", pri)
		}
	}

	for _, pri := range []int{3, 4, 5} {
		task := &models.Task{Status: models.StatusTodo, Priority: pri}
		if p.Filter(task) {
			t.Errorf("Next should reject todo priority %d", pri)
		}
	}

	task := &models.Task{Status: models.StatusInProgress, Priority: 1}
	if p.Filter(task) {
		t.Error("Next should reject non-todo tasks")
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	snoozed := &models.Task{Status: models.StatusTodo, Priority: 1, SnoozedUntil: &tomorrow}
	if p.Filter(snoozed) {
		t.Error("Next should reject snoozed tasks")
	}
}

func TestInboxPerspective(t *testing.T) {
	p, _ := PerspectiveByKey(4)

	accept := &models.Task{Status: models.StatusTodo, Tags: nil}
	if !p.Filter(accept) {
		t.Error("Inbox should accept todo tasks with no tags")
	}

	acceptEmpty := &models.Task{Status: models.StatusTodo, Tags: []string{}}
	if !p.Filter(acceptEmpty) {
		t.Error("Inbox should accept todo tasks with empty tags slice")
	}

	rejectTags := &models.Task{Status: models.StatusTodo, Tags: []string{"work"}}
	if p.Filter(rejectTags) {
		t.Error("Inbox should reject todo tasks with tags")
	}

	rejectStatus := &models.Task{Status: models.StatusInProgress, Tags: nil}
	if p.Filter(rejectStatus) {
		t.Error("Inbox should reject non-todo tasks")
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	snoozed := &models.Task{Status: models.StatusTodo, Tags: nil, SnoozedUntil: &tomorrow}
	if p.Filter(snoozed) {
		t.Error("Inbox should reject snoozed tasks")
	}
}

func TestStandbyPerspective(t *testing.T) {
	p, _ := PerspectiveByKey(5)
	if p.Name != "Standby" {
		t.Fatalf("expected 'Standby', got %q", p.Name)
	}

	accept := &models.Task{Status: models.StatusStandby}
	if !p.Filter(accept) {
		t.Error("Standby should accept standby tasks")
	}

	for _, status := range []models.TaskStatus{
		models.StatusTodo,
		models.StatusInProgress,
		models.StatusDone,
		models.StatusBlocked,
		models.StatusReview,
		models.StatusCanceled,
	} {
		task := &models.Task{Status: status}
		if p.Filter(task) {
			t.Errorf("Standby should reject %s tasks", status)
		}
	}
}

func TestWaitingPerspective(t *testing.T) {
	p, _ := PerspectiveByKey(6)

	accept := &models.Task{Status: models.StatusBlocked}
	if !p.Filter(accept) {
		t.Error("Waiting should accept blocked tasks")
	}

	for _, status := range []models.TaskStatus{
		models.StatusTodo,
		models.StatusInProgress,
		models.StatusDone,
		models.StatusReview,
	} {
		task := &models.Task{Status: status}
		if p.Filter(task) {
			t.Errorf("Waiting should reject %s tasks", status)
		}
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	snoozed := &models.Task{Status: models.StatusBlocked, SnoozedUntil: &tomorrow}
	if p.Filter(snoozed) {
		t.Error("Waiting should reject snoozed tasks")
	}
}

func TestSomedayPerspective(t *testing.T) {
	p, _ := PerspectiveByKey(7)

	for _, pri := range []int{4, 5} {
		task := &models.Task{Status: models.StatusTodo, Priority: pri}
		if !p.Filter(task) {
			t.Errorf("Someday should accept todo priority %d", pri)
		}
	}

	for _, pri := range []int{1, 2, 3} {
		task := &models.Task{Status: models.StatusTodo, Priority: pri}
		if p.Filter(task) {
			t.Errorf("Someday should reject todo priority %d", pri)
		}
	}

	task := &models.Task{Status: models.StatusInProgress, Priority: 5}
	if p.Filter(task) {
		t.Error("Someday should reject non-todo tasks")
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	snoozed := &models.Task{Status: models.StatusTodo, Priority: 5, SnoozedUntil: &tomorrow}
	if p.Filter(snoozed) {
		t.Error("Someday should reject snoozed tasks")
	}
}

func TestCompletedPerspective(t *testing.T) {
	p, _ := PerspectiveByKey(8)

	accept := &models.Task{Status: models.StatusDone}
	if !p.Filter(accept) {
		t.Error("Completed should accept done tasks")
	}

	canceled := &models.Task{Status: models.StatusCanceled}
	if p.Filter(canceled) {
		t.Error("Completed should reject canceled tasks")
	}

	for _, status := range []models.TaskStatus{
		models.StatusTodo,
		models.StatusInProgress,
		models.StatusBlocked,
	} {
		task := &models.Task{Status: status}
		if p.Filter(task) {
			t.Errorf("Completed should reject %s tasks", status)
		}
	}
}

func TestReviewPerspective(t *testing.T) {
	p, _ := PerspectiveByKey(9)

	accept := &models.Task{Status: models.StatusReview}
	if !p.Filter(accept) {
		t.Error("Review should accept in_review tasks")
	}

	for _, status := range []models.TaskStatus{
		models.StatusTodo,
		models.StatusInProgress,
		models.StatusDone,
		models.StatusBlocked,
	} {
		task := &models.Task{Status: status}
		if p.Filter(task) {
			t.Errorf("Review should reject %s tasks", status)
		}
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	snoozed := &models.Task{Status: models.StatusReview, SnoozedUntil: &tomorrow}
	if p.Filter(snoozed) {
		t.Error("Review should reject snoozed tasks")
	}
}

func TestBuildSectionOrderNoCustomSections(t *testing.T) {
	tasks := []*models.Task{
		{Section: ""},
		{Section: models.SectionNext},
	}
	order := BuildSectionOrder(tasks)
	if !slices.Equal(order, SectionOrder) {
		t.Errorf("expected canonical SectionOrder, got %v", order)
	}
}

func TestBuildSectionOrderKeepsGTDOnlyWithCustomSections(t *testing.T) {
	tasks := []*models.Task{
		{Section: "", CustomSection: "Zulu"},
		{Section: "", CustomSection: "Alpha"},
		{Section: models.SectionNext},
	}
	order := BuildSectionOrder(tasks)
	if !slices.Equal(order, SectionOrder) {
		t.Errorf("custom sections should not become section headers, got %v", order)
	}
}

func TestBuildSectionOrderReturnsIndependentCopy(t *testing.T) {
	tasks := []*models.Task{
		{CustomSection: "Sprint"},
		{CustomSection: "Sprint"},
	}
	order := BuildSectionOrder(tasks)
	order[0] = "changed"
	if SectionOrder[0] != models.SectionInbox {
		t.Errorf("mutating returned order should not mutate SectionOrder, got %q", SectionOrder[0])
	}
}

func TestBuildCrossProjectSectionOrderKeepsGTDOnly(t *testing.T) {
	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{{CustomSection: "Beta"}}},
		{projectID: "p2", tasks: []*models.Task{{CustomSection: "Alpha"}}},
	}
	order := BuildCrossProjectSectionOrder(groups)
	if !slices.Equal(order, SectionOrder) {
		t.Errorf("custom sections should not become cross-project section headers, got %v", order)
	}
}

func TestHeaderSectionForTaskMapsCustomSectionToActive(t *testing.T) {
	if got := headerSectionForTask(&models.Task{Section: models.SectionNext}); got != models.SectionNext {
		t.Errorf("GTD task header section = %q, want %q", got, models.SectionNext)
	}
	if got := headerSectionForTask(&models.Task{CustomSection: "Sprint-1"}); got != "" {
		t.Errorf("custom-section task header section = %q, want Active empty section", got)
	}
	if got := headerSectionForTask(nil); got != "" {
		t.Errorf("nil task header section = %q, want Active empty section", got)
	}
}

func TestCustomSectionTabsForProjectKeepsCustomSections(t *testing.T) {
	project := &models.Project{
		Sections: []string{models.SectionNext, "Zulu", models.SectionInbox, "Alpha"},
	}
	want := []string{"All", "Alpha", "Zulu"}
	if got := CustomSectionTabs(project); !slices.Equal(got, want) {
		t.Errorf("CustomSectionTabs() = %v, want %v", got, want)
	}
}

func TestCustomSectionTabsForProjectsAggregatesSortedUniqueSections(t *testing.T) {
	projects := []*models.Project{
		nil,
		{Sections: []string{models.SectionNext, "Beta", "Alpha"}},
		{Sections: []string{models.SectionInbox, "Alpha", "Gamma"}},
	}
	want := []string{"All", "Alpha", "Beta", "Gamma"}
	if got := CustomSectionTabsForProjects(projects); !slices.Equal(got, want) {
		t.Errorf("CustomSectionTabsForProjects() = %v, want %v", got, want)
	}
}

func TestCustomSectionTabsForProjectsWithoutCustomSections(t *testing.T) {
	projects := []*models.Project{
		{Sections: []string{models.SectionNext, models.SectionInbox}},
		nil,
	}
	if got := CustomSectionTabsForProjects(projects); got != nil {
		t.Errorf("CustomSectionTabsForProjects() = %v, want nil", got)
	}
}
