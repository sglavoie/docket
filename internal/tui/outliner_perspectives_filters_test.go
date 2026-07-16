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
	wantKeys := []int{9, 1, 2, 3, 4, 5, 6, 7, 0, 8}
	for id, p := range presets {
		if p.ID != id {
			t.Errorf("preset %d: stable ID = %d", id, p.ID)
		}
		if p.Key != wantKeys[id] {
			t.Errorf("preset %d: key = %d, want %d", id, p.Key, wantKeys[id])
		}
		if p.Name == "" {
			t.Errorf("preset %d: empty name", id)
		}
	}
}

func TestPerspectiveByKey(t *testing.T) {
	want := []struct {
		name string
		id   int
	}{
		{"Completed", PerspectiveCompleted},
		{"All", PerspectiveAll},
		{"Active", PerspectiveActive},
		{"Next", PerspectiveNext},
		{"Inbox", PerspectiveInbox},
		{"Standby", PerspectiveStandby},
		{"Waiting", PerspectiveWaiting},
		{"Someday", PerspectiveSomeday},
		{"Review", PerspectiveReview},
		{"Snoozed", PerspectiveSnoozed},
	}
	for key, expected := range want {
		p, ok := PerspectiveByKey(key)
		if !ok {
			t.Fatalf("PerspectiveByKey(%d) returned false", key)
		}
		if p.Key != key {
			t.Errorf("PerspectiveByKey(%d).Key = %d", key, p.Key)
		}
		if p.ID != expected.id || p.Name != expected.name {
			t.Errorf("PerspectiveByKey(%d) = (%d, %q), want (%d, %q)", key, p.ID, p.Name, expected.id, expected.name)
		}
	}

	for _, bad := range []int{-1, 10, 100} {
		_, ok := PerspectiveByKey(bad)
		if ok {
			t.Errorf("PerspectiveByKey(%d) should return false", bad)
		}
	}
}

func TestPerspectiveByIDPreservesPersistedMeanings(t *testing.T) {
	want := map[int]string{
		PerspectiveSnoozed:   "Snoozed",
		PerspectiveAll:       "All",
		PerspectiveCompleted: "Completed",
		PerspectiveReview:    "Review",
	}
	for id, name := range want {
		p, ok := PerspectiveByID(id)
		if !ok || p.Name != name {
			t.Errorf("PerspectiveByID(%d) = (%q, %v), want %q", id, p.Name, ok, name)
		}
	}
	for _, bad := range []int{-1, 10, 100} {
		if _, ok := PerspectiveByID(bad); ok {
			t.Errorf("PerspectiveByID(%d) should return false", bad)
		}
	}
}

func TestCyclePerspectiveUsesCanonicalOrder(t *testing.T) {
	order := []int{
		PerspectiveAll,
		PerspectiveActive,
		PerspectiveNext,
		PerspectiveInbox,
		PerspectiveStandby,
		PerspectiveWaiting,
		PerspectiveSomeday,
		PerspectiveReview,
		PerspectiveSnoozed,
		PerspectiveCompleted,
	}
	for idx, id := range order {
		wantNext := order[(idx+1)%len(order)]
		if got := CyclePerspective(id, 1); got != wantNext {
			t.Errorf("forward from %d = %d, want %d", id, got, wantNext)
		}
		wantPrevious := order[(idx+len(order)-1)%len(order)]
		if got := CyclePerspective(id, -1); got != wantPrevious {
			t.Errorf("backward from %d = %d, want %d", id, got, wantPrevious)
		}
	}
}

func TestSnoozedPerspective(t *testing.T) {
	p, _ := PerspectiveByID(PerspectiveSnoozed)
	if p.Name != "Snoozed" {
		t.Fatalf("expected 'Snoozed', got %q", p.Name)
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	snoozed := &models.Task{WorkflowState: models.StateReady, SnoozedUntil: &tomorrow}
	if !p.Filter(snoozed) {
		t.Error("Snoozed should accept snoozed tasks")
	}

	normal := &models.Task{WorkflowState: models.StateReady}
	if p.Filter(normal) {
		t.Error("Snoozed should reject non-snoozed tasks")
	}
}

func TestAllPerspective(t *testing.T) {
	p, _ := PerspectiveByID(PerspectiveAll)
	if p.Name != "All" {
		t.Fatalf("expected 'All', got %q", p.Name)
	}
	if p.Filter == nil {
		t.Fatal("All perspective should have a filter that excludes completed tasks")
	}
	if !p.Filter(&models.Task{WorkflowState: models.StateReady}) {
		t.Error("All should accept todo tasks")
	}
	if !p.Filter(&models.Task{WorkflowState: models.StateInProgress}) {
		t.Error("All should accept in_progress tasks")
	}
	if p.Filter(&models.Task{WorkflowState: models.StateDone}) {
		t.Error("All should reject done tasks")
	}
	if p.Filter(&models.Task{WorkflowState: models.StateCanceled}) {
		t.Error("All should reject canceled tasks")
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	snoozed := &models.Task{WorkflowState: models.StateReady, SnoozedUntil: &tomorrow}
	if p.Filter(snoozed) {
		t.Error("All should reject snoozed tasks")
	}
}

func TestActivePerspective(t *testing.T) {
	p, _ := PerspectiveByID(PerspectiveActive)
	accept := []*models.Task{
		{WorkflowState: models.StateReady},
		{WorkflowState: models.StateInProgress},
	}
	reject := []*models.Task{
		{WorkflowState: models.StateInbox},
		{WorkflowState: models.StateNext},
		{WorkflowState: models.StateDone},
		{WorkflowState: models.StateWaiting},
		{WorkflowState: models.StateReview},
		{WorkflowState: models.StateCanceled},
	}

	for _, task := range accept {
		if !p.Filter(task) {
			t.Errorf("Active should accept %s tasks", task.State())
		}
	}
	for _, task := range reject {
		if p.Filter(task) {
			t.Errorf("Active should reject %s tasks", task.State())
		}
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	snoozedActive := &models.Task{WorkflowState: models.StateInProgress, SnoozedUntil: &tomorrow}
	if p.Filter(snoozedActive) {
		t.Error("Active should reject snoozed tasks")
	}
}

func TestNextPerspective(t *testing.T) {
	p, _ := PerspectiveByID(PerspectiveNext)

	for _, pri := range []int{1, 2, 3, 4, 5} {
		task := &models.Task{WorkflowState: models.StateNext, Priority: pri}
		if !p.Filter(task) {
			t.Errorf("Next should accept next-state priority %d", pri)
		}
	}

	for _, state := range []models.WorkflowState{models.StateInbox, models.StateReady, models.StateInProgress} {
		task := &models.Task{WorkflowState: state, Priority: 1}
		if p.Filter(task) {
			t.Errorf("Next should reject %s tasks regardless of priority", state)
		}
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	snoozed := &models.Task{WorkflowState: models.StateNext, Priority: 1, SnoozedUntil: &tomorrow}
	if p.Filter(snoozed) {
		t.Error("Next should reject snoozed tasks")
	}
}

func TestInboxPerspective(t *testing.T) {
	p, _ := PerspectiveByID(PerspectiveInbox)

	accept := &models.Task{WorkflowState: models.StateInbox, Tags: nil}
	if !p.Filter(accept) {
		t.Error("Inbox should accept inbox-state tasks with no tags")
	}

	acceptTagged := &models.Task{WorkflowState: models.StateInbox, Tags: []string{"work"}}
	if !p.Filter(acceptTagged) {
		t.Error("Inbox should accept inbox-state tasks with tags")
	}

	for _, state := range []models.WorkflowState{models.StateReady, models.StateNext, models.StateInProgress} {
		task := &models.Task{WorkflowState: state, Tags: nil}
		if p.Filter(task) {
			t.Errorf("Inbox should reject tagless %s tasks", state)
		}
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	snoozed := &models.Task{WorkflowState: models.StateInbox, Tags: nil, SnoozedUntil: &tomorrow}
	if p.Filter(snoozed) {
		t.Error("Inbox should reject snoozed tasks")
	}
}

func TestStandbyPerspective(t *testing.T) {
	p, _ := PerspectiveByID(PerspectiveStandby)
	if p.Name != "Standby" {
		t.Fatalf("expected 'Standby', got %q", p.Name)
	}

	accept := &models.Task{WorkflowState: models.StatePaused}
	if !p.Filter(accept) {
		t.Error("Standby should accept standby tasks")
	}

	for _, state := range []models.WorkflowState{
		models.StateReady,
		models.StateInProgress,
		models.StateDone,
		models.StateWaiting,
		models.StateReview,
		models.StateCanceled,
	} {
		task := &models.Task{WorkflowState: state}
		if p.Filter(task) {
			t.Errorf("Standby should reject %s tasks", state)
		}
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	snoozed := &models.Task{WorkflowState: models.StatePaused, SnoozedUntil: &tomorrow}
	if p.Filter(snoozed) {
		t.Error("Standby should reject snoozed tasks")
	}
}

func TestWaitingPerspective(t *testing.T) {
	p, _ := PerspectiveByID(PerspectiveWaiting)

	accept := &models.Task{WorkflowState: models.StateWaiting}
	if !p.Filter(accept) {
		t.Error("Waiting should accept blocked tasks")
	}

	for _, state := range []models.WorkflowState{
		models.StateReady,
		models.StateInProgress,
		models.StateDone,
		models.StateReview,
	} {
		task := &models.Task{WorkflowState: state}
		if p.Filter(task) {
			t.Errorf("Waiting should reject %s tasks", state)
		}
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	snoozed := &models.Task{WorkflowState: models.StateWaiting, SnoozedUntil: &tomorrow}
	if p.Filter(snoozed) {
		t.Error("Waiting should reject snoozed tasks")
	}
}

func TestSomedayPerspective(t *testing.T) {
	p, _ := PerspectiveByID(PerspectiveSomeday)

	for _, pri := range []int{1, 2, 3, 4, 5} {
		task := &models.Task{WorkflowState: models.StateSomeday, Priority: pri}
		if !p.Filter(task) {
			t.Errorf("Someday should accept someday-state priority %d", pri)
		}
	}

	for _, pri := range []int{1, 4, 5} {
		task := &models.Task{WorkflowState: models.StateReady, Priority: pri}
		if p.Filter(task) {
			t.Errorf("Someday should reject ready-state priority %d", pri)
		}
	}

	task := &models.Task{WorkflowState: models.StateInProgress, Priority: 5}
	if p.Filter(task) {
		t.Error("Someday should reject non-todo tasks")
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	snoozed := &models.Task{WorkflowState: models.StateSomeday, Priority: 5, SnoozedUntil: &tomorrow}
	if p.Filter(snoozed) {
		t.Error("Someday should reject snoozed tasks")
	}
}

func TestCompletedPerspective(t *testing.T) {
	p, _ := PerspectiveByID(PerspectiveCompleted)

	accept := &models.Task{WorkflowState: models.StateDone}
	if !p.Filter(accept) {
		t.Error("Completed should accept done tasks")
	}

	canceled := &models.Task{WorkflowState: models.StateCanceled}
	if !p.Filter(canceled) {
		t.Error("Completed should accept canceled tasks")
	}

	for _, state := range []models.WorkflowState{
		models.StateReady,
		models.StateInProgress,
		models.StateWaiting,
	} {
		task := &models.Task{WorkflowState: state}
		if p.Filter(task) {
			t.Errorf("Completed should reject %s tasks", state)
		}
	}
}

func TestReviewPerspective(t *testing.T) {
	p, _ := PerspectiveByID(PerspectiveReview)

	accept := &models.Task{WorkflowState: models.StateReview}
	if !p.Filter(accept) {
		t.Error("Review should accept in_review tasks")
	}

	for _, state := range []models.WorkflowState{
		models.StateReady,
		models.StateInProgress,
		models.StateDone,
		models.StateWaiting,
	} {
		task := &models.Task{WorkflowState: state}
		if p.Filter(task) {
			t.Errorf("Review should reject %s tasks", state)
		}
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	snoozed := &models.Task{WorkflowState: models.StateReview, SnoozedUntil: &tomorrow}
	if p.Filter(snoozed) {
		t.Error("Review should reject snoozed tasks")
	}
}

func TestBuildSectionOrderWithGTDSectionsOnly(t *testing.T) {
	tasks := []*models.Task{
		{WorkflowState: models.StateReady},
		{WorkflowState: models.StateNext},
	}
	order := BuildSectionOrder(tasks)
	if !slices.Equal(order, SectionOrder) {
		t.Errorf("expected canonical SectionOrder, got %v", order)
	}
}

func TestBuildSectionOrderKeepsGTDOnlyWithNonGTDSections(t *testing.T) {
	tasks := []*models.Task{
		{WorkflowState: models.StateReady},
		{WorkflowState: models.StateReady},
		{WorkflowState: models.StateNext},
	}
	order := BuildSectionOrder(tasks)
	if !slices.Equal(order, SectionOrder) {
		t.Errorf("non-GTD sections should not become section headers, got %v", order)
	}
}

func TestBuildSectionOrderReturnsIndependentCopy(t *testing.T) {
	tasks := []*models.Task{{WorkflowState: models.StateReady}}
	order := BuildSectionOrder(tasks)
	order[0] = "changed"
	if SectionOrder[0] != models.SectionInbox {
		t.Errorf("mutating returned order should not mutate SectionOrder, got %q", SectionOrder[0])
	}
}

func TestBuildCrossProjectSectionOrderKeepsGTDOnly(t *testing.T) {
	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{{WorkflowState: models.StateReady}}},
		{projectID: "p2", tasks: []*models.Task{{WorkflowState: models.StateReady}}},
	}
	order := BuildCrossProjectSectionOrder(groups)
	if !slices.Equal(order, SectionOrder) {
		t.Errorf("non-GTD sections should not become cross-project section headers, got %v", order)
	}
}

func TestHeaderSectionForTaskIgnoresNonGTDSection(t *testing.T) {
	if got := headerSectionForTask(&models.Task{WorkflowState: models.StateNext}); got != models.SectionNext {
		t.Errorf("GTD task header section = %q, want %q", got, models.SectionNext)
	}
	if got := headerSectionForTask(&models.Task{WorkflowState: models.StateReady}); got != "" {
		t.Errorf("non-GTD task header section = %q, want Active empty section", got)
	}
	if got := headerSectionForTask(nil); got != "" {
		t.Errorf("nil task header section = %q, want Active empty section", got)
	}
}
