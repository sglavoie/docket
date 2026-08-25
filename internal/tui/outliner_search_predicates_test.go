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
	predicates := &searchPredicates{priority: models.PriorityMedium, now: time.Now()}
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
