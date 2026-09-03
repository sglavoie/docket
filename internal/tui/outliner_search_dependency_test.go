package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// searchDependencyTask builds an addressable task for the autocomplete pane. The
// compiled predicate itself is exercised in internal/tui/query, which owns it;
// what is left here is the shell's own vocabulary contract.
func searchDependencyTask(id string, shortID int, title string) *models.Task {
	task := primaryRowsTask(id, models.StateBacklog)
	task.Title = title
	task.ShortID = shortID
	return task
}

// TestSearchDependencyAutocompleteOffersReferencedTargets is the vocabulary
// contract: the pane completes what the live candidates already depend on, which
// is exactly the set this predicate can match.
func TestSearchDependencyAutocompleteOffersReferencedTargets(t *testing.T) {
	open := searchDependencyTask("open", 1, "Open target")
	closed := searchDependencyTask("closed", 2, "Closed target")
	closed.ApplyWorkflowState(models.StateDone)
	holder := searchDependencyTask("holder", 3, "Holder")
	lonely := searchDependencyTask("lonely", 4, "Nobody depends on me")
	holder.Dependencies = []models.TaskReference{
		{ProjectID: "alpha", TaskID: open.ID},
		{ProjectID: "alpha", TaskID: closed.ID},
		{ProjectID: "ghost", TaskID: "vanished"},
	}
	project := primaryRowsProject("alpha", open, closed, holder, lonely)
	project.Slug = "alpha"
	outliner := searchACOutliner(project)

	typeSearchAC(outliner, "~")
	values := searchACValues(outliner)
	for _, want := range []string{"1", "2", "ghost:vanished"} {
		if !containsSearchACValue(values, want) {
			t.Errorf("suggestions %v missing %q", values, want)
		}
	}
	// An open task nobody depends on would complete into a query with no results.
	if containsSearchACValue(values, "4") {
		t.Errorf("suggestions %v offered an unreferenced task", values)
	}

	// A value already accepted on the line is not offered twice: the sigil is
	// repeatable and the predicates AND.
	typeSearchAC(outliner, "~1 ~")
	if remaining := searchACValues(outliner); containsSearchACValue(remaining, "1") {
		t.Errorf("suggestions %v re-offered an accepted value", remaining)
	}
}

func containsSearchACValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
