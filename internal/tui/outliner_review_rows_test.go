package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// reviewTestEnv wires a cross-project OutlinerModel (m.project == nil) with a
// default runtime and the given projects loaded so the review row builders can
// resolve settings, filters, and project badges.
func reviewTestEnv(projects ...*models.Project) *OutlinerModel {
	m := minimalOutliner()
	m.runtime = config.EmptyRuntimeState("")
	m.loadedProjects = projects
	m.MultiSelectState = MultiSelectState{selectedTaskIdentities: make(map[TaskIdentity]bool)}
	return m
}

// dueTask builds a task in the given state with an optional due-date offset.
func dueTask(id, title string, state models.WorkflowState, daysOffset *int) *models.Task {
	task := &models.Task{ID: id, Title: title, WorkflowState: state}
	if daysOffset != nil {
		due := time.Now().AddDate(0, 0, *daysOffset)
		task.DueDate = &due
	}
	return task
}

func TestBuildDailyReviewRowsSectionsInOrder(t *testing.T) {
	proj := &models.Project{ID: "p1", Name: "Proj"}

	overdue := dueTask("o1", "Overdue", models.StateNext, intPtr(-5))
	today := dueTask("t1", "Today", models.StateNext, intPtr(0))
	soon := dueTask("s1", "Soon", models.StateNext, intPtr(3))
	later := dueTask("l1", "Later", models.StateNext, intPtr(15))
	future := dueTask("f1", "Future", models.StateNext, intPtr(45))
	inProg := dueTask("ip1", "InProgress", models.StateInProgress, nil)
	blocked := dueTask("b1", "Blocked", models.StateWaiting, nil)
	inReview := dueTask("r1", "Review", models.StateReview, nil)
	standby := dueTask("sb1", "Standby", models.StatePaused, nil)

	old := dueTask("old1", "Old", models.StateNext, nil)
	old.UpdatedAt = time.Now().AddDate(0, 0, -30)

	done := dueTask("d1", "Done", models.StateNext, nil)
	done.Complete()

	proj.Tasks = []*models.Task{overdue, today, soon, later, future, inProg, blocked, inReview, standby, old, done}

	m := reviewTestEnv(proj)
	m.buildDailyReviewRows()

	var headers []string
	for _, row := range m.rows {
		if row.IsSectionHeader {
			headers = append(headers, row.SectionName)
		}
	}

	want := []string{
		getSectionName(SectionOverdue), getSectionName(SectionDueToday), getSectionName(SectionDueSoon),
		getSectionName(SectionDueLater), getSectionName(SectionFuture), getSectionName(SectionInProgress),
		getSectionName(SectionBlocked), getSectionName(SectionInReview), getSectionName(SectionOnStandby),
		getSectionName(SectionOld), getSectionName(SectionCompleted),
	}
	if len(headers) != len(want) {
		t.Fatalf("headers = %v, want %v", headers, want)
	}
	for i := range want {
		if headers[i] != want[i] {
			t.Fatalf("header[%d] = %q, want %q (all: %v)", i, headers[i], want[i], headers)
		}
	}
}

func TestDailyReviewSectionCollapseHidesTasks(t *testing.T) {
	proj := &models.Project{ID: "p1", Name: "Proj"}
	proj.Tasks = []*models.Task{
		dueTask("o1", "Overdue A", models.StateNext, intPtr(-3)),
		dueTask("o2", "Overdue B", models.StateNext, intPtr(-2)),
	}

	m := reviewTestEnv(proj)
	m.buildDailyReviewRows()

	taskRows := 0
	var collapseKey string
	for _, row := range m.rows {
		if row.IsSectionHeader {
			collapseKey = row.CollapseKey
		} else if row.isTaskRow() {
			taskRows++
		}
	}
	if taskRows != 2 {
		t.Fatalf("expanded task rows = %d, want 2", taskRows)
	}
	if !isReviewCollapseKey(collapseKey) {
		t.Fatalf("collapse key %q is not a review key", collapseKey)
	}

	m.reviewCollapsedSections = map[string]bool{collapseKey: true}
	m.rows = m.rows[:0]
	m.buildDailyReviewRows()

	headers, tasks := 0, 0
	for _, row := range m.rows {
		if row.IsSectionHeader {
			headers++
		} else if row.isTaskRow() {
			tasks++
		}
	}
	if headers != 1 || tasks != 0 {
		t.Fatalf("collapsed: headers=%d tasks=%d, want 1/0", headers, tasks)
	}
}

func TestBuildActivityRowsSplitsAndOrders(t *testing.T) {
	proj := &models.Project{ID: "p1", Name: "Proj"}

	a := dueTask("a1", "Active newest", models.StateNext, nil)
	a.UpdatedAt = time.Now()
	b := dueTask("a2", "Active older", models.StateNext, nil)
	b.UpdatedAt = time.Now().AddDate(0, 0, -2)
	c := dueTask("c1", "Completed", models.StateNext, nil)
	c.Complete()
	c.UpdatedAt = time.Now().AddDate(0, 0, -1)

	proj.Tasks = []*models.Task{b, c, a}

	m := reviewTestEnv(proj)
	m.buildActivityRows()

	var headers []string
	var firstActive string
	inActive := false
	for _, row := range m.rows {
		if row.IsSectionHeader {
			headers = append(headers, row.SectionName)
			inActive = row.SectionName == "Active"
			continue
		}
		if inActive && firstActive == "" {
			firstActive = row.task.Title
		}
	}
	if len(headers) != 2 || headers[0] != "Active" || headers[1] != "Completed" {
		t.Fatalf("activity headers = %v, want [Active Completed]", headers)
	}
	if firstActive != "Active newest" {
		t.Fatalf("first active row = %q, want %q", firstActive, "Active newest")
	}
}

func TestBuildActivityRowsRespectsLimit(t *testing.T) {
	proj := &models.Project{ID: "p1", Name: "Proj"}
	for i := 0; i < 30; i++ {
		task := dueTask("t"+string(rune('a'+i)), "Task", models.StateNext, nil)
		task.UpdatedAt = time.Now().AddDate(0, 0, -i)
		proj.Tasks = append(proj.Tasks, task)
	}

	m := reviewTestEnv(proj)
	m.buildActivityRows()

	tasks := 0
	for _, row := range m.rows {
		if row.isTaskRow() {
			tasks++
		}
	}
	// Default recent-activity limit is 20.
	if tasks != 20 {
		t.Fatalf("activity task rows = %d, want 20 (limit)", tasks)
	}
}

func TestSelectReviewModeSetsAndClearsFlag(t *testing.T) {
	proj := &models.Project{ID: "p1", Name: "Proj"}
	proj.Tasks = []*models.Task{dueTask("o1", "Overdue", models.StateNext, intPtr(-1))}

	m := reviewTestEnv(proj)

	m.selectOutlinerMode(ModeDailyReview)
	if m.reviewMode != reviewDaily {
		t.Fatalf("reviewMode = %d, want reviewDaily", m.reviewMode)
	}
	if got := m.activeOutlinerMode(); got != ModeDailyReview {
		t.Fatalf("activeOutlinerMode = %q, want daily-review", got)
	}
	if !m.isComputedOrderMode() {
		t.Fatal("review mode should report computed order")
	}

	m.selectOutlinerMode(ModeActivity)
	if m.reviewMode != reviewActivity {
		t.Fatalf("reviewMode = %d, want reviewActivity", m.reviewMode)
	}

	// Switching to a perspective mode clears review state entirely.
	m.selectOutlinerMode(ModeAll)
	if m.reviewMode != reviewNone {
		t.Fatalf("reviewMode = %d after ModeAll, want reviewNone", m.reviewMode)
	}
}

func TestReviewCompletionGuardBlocksNarrowedScope(t *testing.T) {
	m := reviewTestEnv(&models.Project{ID: "p1", Name: "Proj"})
	m.reviewMode = reviewDaily

	// Full workspace scope with no filters: R emits the completion message.
	_, cmd := m.handleReviewCompletionToggle()
	if cmd == nil {
		t.Fatal("expected a completion command at full scope")
	}
	if _, ok := cmd().(MarkDailyReviewDoneMsg); !ok {
		t.Fatalf("expected MarkDailyReviewDoneMsg at full scope")
	}

	// A project scope must block completion with an explanatory status.
	m.scopedProjectID = "p1"
	prevStatus := m.statusMessage
	model, cmd := m.handleReviewCompletionToggle()
	if cmd != nil {
		t.Fatal("scoped review must not emit a completion command")
	}
	if om, ok := model.(*OutlinerModel); ok && om.statusMessage == prevStatus {
		t.Fatal("scoped review should set an explanatory status")
	}
}

func TestReviewCompletionOnlyInDailyMode(t *testing.T) {
	m := reviewTestEnv(&models.Project{ID: "p1", Name: "Proj"})
	m.reviewMode = reviewActivity

	_, cmd := m.handleReviewCompletionToggle()
	if cmd != nil {
		t.Fatal("R should not complete the review outside Daily Review mode")
	}
}

func TestReviewModesSelectableInPicker(t *testing.T) {
	daily, _ := OutlinerModeByID(ModeDailyReview)
	activity, _ := OutlinerModeByID(ModeActivity)
	if modePickerModeUnavailable(daily) || modePickerModeUnavailable(activity) {
		t.Fatal("review modes must be selectable in the mode picker")
	}

	rows := modePickerRows("review")
	found := false
	for _, r := range rows {
		if r.isMode && r.mode.ID == ModeDailyReview {
			found = true
			if r.disabled {
				t.Fatal("Daily Review must not be disabled in the picker")
			}
		}
	}
	if !found {
		t.Fatal("Daily Review not listed in filtered picker rows")
	}
}
