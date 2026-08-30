package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func TestCompileSearchDueSplitsBucketsFromExactDays(t *testing.T) {
	now := time.Now()
	buckets := map[string]DueDateFilter{
		"overdue": DueDateOverdue,
		"TODAY":   DueDateToday,
		" week ":  DueDateThisWeek,
		"none":    DueDateNone,
	}
	for value, want := range buckets {
		got := compileSearchDue(value, now)
		if !got.hasBucket || got.hasDay || got.hasHourWindow || got.bucket != want {
			t.Errorf("compileSearchDue(%q) = %+v, want bucket %v", value, got, want)
		}
	}

	day := now.AddDate(0, 0, 4)
	got := compileSearchDue(day.Format(util.DateFormat), now)
	if !got.hasDay || got.hasBucket || got.hasHourWindow || util.DaysBetween(got.day, day) != 0 {
		t.Fatalf("compileSearchDue(exact day) = %+v, want the calendar day", got)
	}

	if unresolvable := compileSearchDue("sarah", now); unresolvable.hasBucket || unresolvable.hasDay || unresolvable.hasHourWindow {
		t.Fatalf("compileSearchDue(sarah) = %+v, want no predicate", unresolvable)
	}
}

func TestCompileSearchDueHourWindow(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.Local)
	got := compileSearchDue("+4h", now)
	if !got.hasHourWindow || got.hasBucket || got.hasDay {
		t.Fatalf("compileSearchDue(+4h) = %+v, want an hour window", got)
	}
	if want := now.Add(4 * time.Hour); !got.window.Equal(want) {
		t.Errorf("compileSearchDue(+4h).window = %v, want %v", got.window, want)
	}

	if upper := compileSearchDue("+12H", now); !upper.hasHourWindow || !upper.window.Equal(now.Add(12*time.Hour)) {
		t.Errorf("compileSearchDue(+12H) = %+v, want a case-insensitive hour window", upper)
	}

	// A minute/day offset or a bucket must not be read as an hour window.
	if minutes := compileSearchDue("+3m", now); minutes.hasHourWindow {
		t.Errorf("compileSearchDue(+3m) = %+v, want no hour window", minutes)
	}
	if today := compileSearchDue("today", now); today.hasHourWindow || !today.hasBucket {
		t.Errorf("compileSearchDue(today) = %+v, want the today bucket unchanged", today)
	}
}

func TestSearchPredicatesAdmitsDueHourWindow(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.Local)
	predicates := &searchPredicates{
		now: now,
		due: compileSearchDue("+4h", now),
	}
	project := &models.Project{ID: "alpha", Slug: "alpha"}

	overdue := now.Add(-3 * time.Hour)
	soon := now.Add(2 * time.Hour)
	later := now.Add(6 * time.Hour)
	dateOnlyToday := time.Date(2026, 8, 25, 0, 0, 0, 0, time.Local)

	cases := map[string]struct {
		task *models.Task
		want bool
	}{
		"overdue instant":   {&models.Task{ID: "a", WorkflowState: models.StateBacklog, DueDate: &overdue}, true},
		"due within window": {&models.Task{ID: "b", WorkflowState: models.StateBacklog, DueDate: &soon}, true},
		"date-only today":   {&models.Task{ID: "c", WorkflowState: models.StateBacklog, DueDate: &dateOnlyToday}, true},
		"beyond window":     {&models.Task{ID: "d", WorkflowState: models.StateBacklog, DueDate: &later}, false},
		"no due date":       {&models.Task{ID: "e", WorkflowState: models.StateBacklog}, false},
	}
	for name, tc := range cases {
		if got := predicates.admits(tc.task, project); got != tc.want {
			t.Errorf("%s: admits = %v, want %v", name, got, tc.want)
		}
	}
}

func TestSearchTagPrefixReadsNormalizedTagsOnly(t *testing.T) {
	task := &models.Task{Tags: []string{"Backend", "source:GitHub"}, Context: "Client-Acme"}
	for _, prefix := range []string{"back", "backend", "source-git"} {
		if !taskCarriesTagPrefix(task, prefix) {
			t.Errorf("taskCarriesTagPrefix(%q) = false, want a normalized prefix match", prefix)
		}
	}
	if !taskCarriesTagPrefix(task, normalizeSearchTagPrefix("source:GitHub")) {
		t.Error("a namespaced prefix did not reach the kebab-normalized stored tag")
	}
	for _, prefix := range []string{"end", "client", "sourcegit"} {
		if taskCarriesTagPrefix(task, prefix) {
			t.Errorf("taskCarriesTagPrefix(%q) = true, want tags only and prefixes only", prefix)
		}
	}

	predicates := &searchPredicates{contexts: []string{"client"}}
	if !predicates.admitsContext(task) {
		t.Error("context prefix did not match the normalized task context")
	}
	predicates.contexts = []string{"acme"}
	if predicates.admitsContext(task) {
		t.Error("context matched a non-prefix substring")
	}
}

func TestSearchPredicatesAreNilSafeAndOpen(t *testing.T) {
	var predicates *searchPredicates
	task := &models.Task{ID: "task", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "alpha", Slug: "alpha"}
	if predicates.invalid() || predicates.allowsTerminal() || predicates.guidanceText() != "" {
		t.Fatal("a nil gate reported a state")
	}
	if !predicates.admits(task, project) || !predicates.admitsProject(project) {
		t.Fatal("a nil gate rejected a task")
	}
}

func TestSearchPriorityGateIsEqualityRatherThanThreshold(t *testing.T) {
	predicates := &searchPredicates{priorities: []int{models.PriorityMedium}, now: time.Now()}
	project := &models.Project{ID: "alpha", Slug: "alpha"}
	for priority, want := range map[int]bool{
		models.PriorityHigh:   false,
		models.PriorityMedium: true,
		models.PriorityLow:    false,
		models.PriorityUnset:  false,
	} {
		task := &models.Task{ID: "task", WorkflowState: models.StateBacklog, Priority: priority}
		if got := predicates.admits(task, project); got != want {
			t.Errorf("priority %d admitted = %v, want %v", priority, got, want)
		}
	}
}

// TestSearchPriorityThresholdWidensOnlyWhereTheSuffixIsTyped holds the split the
// threshold was added for: !N is still equality, and only !N+ opens the more
// urgent priorities, which are the smaller numbers.
func TestSearchPriorityThresholdWidensOnlyWhereTheSuffixIsTyped(t *testing.T) {
	project := &models.Project{ID: "alpha", Slug: "alpha"}
	tests := []struct {
		query string
		want  map[int]bool
	}{
		{"!2", map[int]bool{models.PriorityHigh: false, models.PriorityMedium: true, models.PriorityLow: false}},
		{"!2+", map[int]bool{models.PriorityHigh: true, models.PriorityMedium: true, models.PriorityLow: false}},
		{"-!2+", map[int]bool{models.PriorityHigh: false, models.PriorityMedium: false, models.PriorityLow: true}},
		{"-!1", map[int]bool{models.PriorityHigh: false, models.PriorityMedium: true, models.PriorityLow: true}},
	}
	for _, tt := range tests {
		outliner := scopeTestOutliner(project)
		_, gate := outliner.compileSearchQuery(tt.query, []*models.Project{project})
		if gate.invalid() {
			t.Fatalf("%s produced guidance %q", tt.query, gate.guidanceText())
		}
		for priority, want := range tt.want {
			task := &models.Task{ID: "task", WorkflowState: models.StateBacklog, Priority: priority}
			if got := gate.admits(task, project); got != want {
				t.Errorf("%s admitted priority %d = %v, want %v", tt.query, priority, got, want)
			}
		}
	}
}

// TestSearchNegationsSubtractWhatTheirTwinsSelect walks the negation set through
// the compiled gate. Each one must remove exactly the population its positive
// twin would have kept, and leave an unrelated task alone: a negation narrows,
// it never re-scopes.
func TestSearchNegationsSubtractWhatTheirTwinsSelect(t *testing.T) {
	now := time.Now()
	due := now
	marked := &models.Task{
		ID: "marked", ShortID: 1, Title: "Marked",
		WorkflowState: models.StateDoing, Priority: models.PriorityHigh,
		Tags: []string{"backend"}, Context: "client", Pinned: true, DueDate: &due,
	}
	plain := &models.Task{ID: "plain", ShortID: 2, Title: "Plain", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "alpha", Slug: "alpha", Name: "Alpha", Tasks: []*models.Task{marked, plain}}
	outliner := scopeTestOutliner(project)

	for _, query := range []string{"-!1", "-#backend", "-##client", "-@today", "-$doing", "-*pin"} {
		_, gate := outliner.compileSearchQuery(query, []*models.Project{project})
		if gate.invalid() {
			t.Fatalf("%s produced guidance %q", query, gate.guidanceText())
		}
		if gate.admits(marked, project) {
			t.Errorf("%s admitted the task it names", query)
		}
		if !gate.admits(plain, project) {
			t.Errorf("%s excluded a task it says nothing about", query)
		}
	}

	// The project negation is the one axis that answers about the project rather
	// than the task, so it is checked where it applies.
	_, scoped := outliner.compileSearchQuery("-%alpha", []*models.Project{project})
	if scoped.invalid() {
		t.Fatalf("-%%alpha produced guidance %q", scoped.guidanceText())
	}
	if scoped.admitsProject(project) {
		t.Error("-%alpha admitted the project it names")
	}
}

// TestSearchDerivedPredicatesAnswerComputedQuestions covers the ? set end to end
// through the same gate a query runs on. The two reference predicates are
// structural on purpose: one reads the task's own outgoing list and the other
// the incoming census, and neither looks at the other end's state.
func TestSearchDerivedPredicatesAnswerComputedQuestions(t *testing.T) {
	now := time.Now()
	old := now.AddDate(0, 0, -200)
	completedLongAgo := now.AddDate(0, 0, -120)
	later := now.Add(48 * time.Hour)

	stale := &models.Task{ID: "stale", ShortID: 1, Title: "Stale", WorkflowState: models.StateBacklog, UpdatedAt: old}
	child := &models.Task{ID: "child", ShortID: 2, Title: "Child", WorkflowState: models.StateBacklog, UpdatedAt: now}
	parent := &models.Task{
		ID: "parent", ShortID: 3, Title: "Parent", WorkflowState: models.StateBacklog,
		UpdatedAt: now, Children: []*models.Task{child},
	}
	deferred := &models.Task{
		ID: "deferred", ShortID: 4, Title: "Deferred", WorkflowState: models.StateBacklog,
		UpdatedAt: now, DeferredUntil: &later,
	}
	finished := &models.Task{
		ID: "finished", ShortID: 5, Title: "Finished", WorkflowState: models.StateDone,
		UpdatedAt: completedLongAgo, CompletedAt: &completedLongAgo,
	}
	target := &models.Task{ID: "target", ShortID: 6, Title: "Target", WorkflowState: models.StateBacklog, UpdatedAt: now}
	referrer := &models.Task{
		ID: "referrer", ShortID: 7, Title: "Referrer", WorkflowState: models.StateBacklog, UpdatedAt: now,
		Dependencies: []models.TaskReference{{ProjectID: "alpha", TaskID: "target"}},
	}
	project := &models.Project{
		ID: "alpha", Slug: "alpha", Name: "Alpha",
		Tasks: []*models.Task{stale, parent, child, deferred, finished, target, referrer},
	}
	outliner := scopeTestOutliner(project)

	cases := []struct {
		query string
		want  *models.Task
	}{
		{"?stale", stale},
		{"?deferred", deferred},
		{"?parent", parent},
		{"?completed:90d", finished},
		{"?blocked", referrer},
		{"?blocking", target},
	}
	for _, tc := range cases {
		_, gate := outliner.compileSearchQuery(tc.query, []*models.Project{project})
		if gate.invalid() {
			t.Fatalf("%s produced guidance %q", tc.query, gate.guidanceText())
		}
		for _, task := range project.Tasks {
			want := task == tc.want
			if got := gate.admits(task, project); got != want {
				t.Errorf("%s admitted %s = %v, want %v", tc.query, task.ID, got, want)
			}
		}
	}

	// Leaf is the complement of parent over the same census, so it is checked as
	// a set rather than as a single row.
	_, leaves := outliner.compileSearchQuery("?leaf", []*models.Project{project})
	if leaves.admits(parent, project) {
		t.Error("?leaf admitted a task with an open child")
	}
	if !leaves.admits(child, project) || !leaves.admits(stale, project) {
		t.Error("?leaf excluded a task with no open child")
	}

	// The completion question opens the terminal population; its negation asks
	// about live work and deliberately does not.
	_, notCompleted := outliner.compileSearchQuery("-?completed", []*models.Project{project})
	if notCompleted.allowsTerminal() {
		t.Error("-?completed opened the terminal population")
	}
	if notCompleted.admits(finished, project) {
		t.Error("-?completed admitted a completed task")
	}
}
