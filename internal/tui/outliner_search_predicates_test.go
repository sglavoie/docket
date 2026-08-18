package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func TestCompileSearchDueSplitsBucketsFromExactDays(t *testing.T) {
	buckets := map[string]DueDateFilter{
		"overdue": DueDateOverdue,
		"TODAY":   DueDateToday,
		" week ":  DueDateThisWeek,
		"none":    DueDateNone,
	}
	for value, want := range buckets {
		got := compileSearchDue(value)
		if !got.hasBucket || got.hasDay || got.bucket != want {
			t.Errorf("compileSearchDue(%q) = %+v, want bucket %v", value, got, want)
		}
	}

	day := time.Now().AddDate(0, 0, 4)
	got := compileSearchDue(day.Format(util.DateFormat))
	if !got.hasDay || got.hasBucket || util.DaysBetween(got.day, day) != 0 {
		t.Fatalf("compileSearchDue(exact day) = %+v, want the calendar day", got)
	}

	if unresolvable := compileSearchDue("sarah"); unresolvable.hasBucket || unresolvable.hasDay {
		t.Fatalf("compileSearchDue(sarah) = %+v, want no predicate", unresolvable)
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
