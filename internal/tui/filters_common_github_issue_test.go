package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCommonFilterGitHubIssueMatching(t *testing.T) {
	issueTask := mustCommonFilterTask(t, "issue-task")
	issueTask.URL = "https://github.com/sglavoie/tt/issues/123"
	plainTask := mustCommonFilterTask(t, "plain-task")

	filter := newFilterState()
	filter.GitHubIssue = true

	if !passesCommonFilters(issueTask, &filter, nil, nil) {
		t.Fatal("task with a GitHub issue URL should pass the GitHub issue filter")
	}
	if passesCommonFilters(plainTask, &filter, nil, nil) {
		t.Fatal("task without a GitHub issue URL should not pass the GitHub issue filter")
	}
}

func TestCommonFilterGitHubIssueBadgeCountAndClear(t *testing.T) {
	filter := newFilterState()
	filter.GitHubIssue = true

	if !hasActiveCommonFilters(&filter) {
		t.Fatal("GitHub issue filter should count as an active common filter")
	}
	if got := commonFilterParts(&filter); len(got) != 1 || got[0] != "issue" {
		t.Fatalf("commonFilterParts() = %v, want [issue]", got)
	}
	if got := renderCommonFilterIndicator(&filter); !strings.Contains(got, "issue") {
		t.Fatalf("renderCommonFilterIndicator() = %q, want to contain issue", got)
	}

	mixin := FilterMixin{filter: filter}
	if got := mixin.ActiveFilterCount(); got != 1 {
		t.Fatalf("FilterMixin.ActiveFilterCount() = %d, want 1", got)
	}

	copied := copyCommonFilterState(&filter)
	if !copied.GitHubIssue {
		t.Fatal("copyCommonFilterState should preserve GitHubIssue")
	}

	clearCommonFilters(&filter)
	if filter.GitHubIssue {
		t.Fatal("clearCommonFilters should clear GitHubIssue")
	}
	if hasActiveCommonFilters(&filter) {
		t.Fatal("cleared GitHub issue filter should not remain active")
	}
}

func TestHandleCommonFilterPrefixGitHubIssue(t *testing.T) {
	filter := newFilterState()

	result := handleCommonFilterPrefix("i", &filter, nil)
	if !result.Handled || !result.Changed {
		t.Fatalf("handleCommonFilterPrefix(i) = %+v, want handled changed result", result)
	}
	if !filter.GitHubIssue {
		t.Fatal("Fi should enable the GitHub issue filter")
	}

	result = handleCommonFilterPrefix("i", &filter, nil)
	if !result.Handled || !result.Changed {
		t.Fatalf("second handleCommonFilterPrefix(i) = %+v, want handled changed result", result)
	}
	if filter.GitHubIssue {
		t.Fatal("Fi should toggle the GitHub issue filter off")
	}
}

func TestCommonFilterPrefixHintsGitHubIssue(t *testing.T) {
	filter := newFilterState()

	hint, ok := findPrefixHint(buildCommonFilterPrefixHints(&filter, true, true), "i")
	if !ok {
		t.Fatal("expected common filter hints to include Fi")
	}
	if hint.Description != "issues" || hint.Detail != "off" || hint.Active {
		t.Fatalf("inactive Fi hint = %+v, want issues/off/inactive", hint)
	}

	filter.GitHubIssue = true
	hint, ok = findPrefixHint(buildCommonFilterPrefixHints(&filter, true, true), "i")
	if !ok {
		t.Fatal("expected common filter hints to include Fi when active")
	}
	if hint.Description != "issues" || hint.Detail != "on" || !hint.Active {
		t.Fatalf("active Fi hint = %+v, want issues/on/active", hint)
	}
}

func mustCommonFilterTask(t *testing.T, id string) *models.Task {
	t.Helper()
	task, err := models.NewTask(id, 1, models.TaskTypeTask, "Task "+id)
	if err != nil {
		t.Fatalf("NewTask(%q): %v", id, err)
	}
	return task
}

func findPrefixHint(hints []PrefixHint, key string) (PrefixHint, bool) {
	for _, hint := range hints {
		if hint.Key == key {
			return hint, true
		}
	}
	return PrefixHint{}, false
}
