package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func requireChordDimmed(t *testing.T, app *AppModel, prefix []string, key string, want bool) {
	t.Helper()
	app.operations.Chord = prefix
	spec, ok := app.pendingChordGuidance()
	if !ok {
		t.Fatalf("no guidance for %v", prefix)
	}
	for _, hint := range spec.Hints {
		if hint.Key == key {
			if hint.Dimmed != want {
				t.Errorf("%v %s (%s) dimmed = %v, want %v", prefix, key, hint.Label, hint.Dimmed, want)
			}
			return
		}
	}
	t.Fatalf("%v guidance omitted %s", prefix, key)
}

func TestChordDimmingReadsLiveNavigationState(t *testing.T) {
	app := newFilterChordTestApp(t)
	out := app.crossOutliner
	count := 0
	out.SetOutstandingConflicts(func() int { return count })
	prefix := []string{"g"}
	for _, countValue := range []int{0, 2, 0} {
		count = countValue
		requireChordDimmed(t, app, prefix, "c", count == 0)
		requireChordDimmed(t, app, prefix, "Esc", false)
	}
	out.SelectOutlinerMode(chrome.ModeAll)
	requireChordDimmed(t, app, prefix, "a", true)
	requireChordDimmed(t, app, prefix, "i", false)
	out.SelectOutlinerMode(chrome.ModeInbox)
	requireChordDimmed(t, app, prefix, "a", false)
	requireChordDimmed(t, app, prefix, "i", true)
	out.SelectOutlinerMode(chrome.ModeRecent)
	requireChordDimmed(t, app, prefix, "r", false) // Refresh remains useful.
	out.SelectOutlinerMode(chrome.ModeAll)
	out.Cursor = 0
	requireChordDimmed(t, app, prefix, "g", true)
	out.Cursor = len(out.Rows) - 1
	requireChordDimmed(t, app, prefix, "g", false)
	for _, row := range out.Rows {
		if row.IsSectionHeader && out.EnterSectionZoom(row) {
			requireChordDimmed(t, app, prefix, "a", false)
			out.ExitSectionZoom()
			break
		}
	}
}

func TestChordDimmingTaskNavigation(t *testing.T) {
	app := newFilterChordTestApp(t)
	out := app.crossOutliner
	project := out.Population.Projects[0]
	task := project.Tasks[0]
	out.NavigateToTaskInProject(task, project)
	requireChordDimmed(t, app, []string{"g"}, "p", false)
	requireChordDimmed(t, app, []string{"g"}, "~", true)
	task.Dependencies = []models.TaskReference{{ProjectID: project.ID, TaskID: project.Tasks[1].ID}}
	out.Population.Invalidate()
	out.RebuildRows()
	out.NavigateToTaskInProject(task, project)
	requireChordDimmed(t, app, []string{"g"}, "~", false)
	out.PushProjectScope(project.ID)
	out.SelectOutlinerMode(chrome.ModeAll)
	out.NavigateToTaskInProject(task, project)
	requireChordDimmed(t, app, []string{"g"}, "p", true)
}

func TestChordDimmingReadsFiltersAndKeepsTogglesUseful(t *testing.T) {
	app := newFilterChordTestApp(t)
	out := app.crossOutliner
	requireChordDimmed(t, app, []string{"f"}, "x", true)
	requireChordDimmed(t, app, []string{"f", "!"}, "0", true)
	out.Filter.Priority = 2
	requireChordDimmed(t, app, []string{"f", "!"}, "0", false)
	requireChordDimmed(t, app, []string{"f", "!"}, "2", true)
	requireChordDimmed(t, app, []string{"f"}, "x", false)
	out.Filter.Review = true
	requireChordDimmed(t, app, []string{"f"}, "=", false)
	requireChordDimmed(t, app, []string{"F"}, "x", true)
	out.ProjectFilter.ParkedMode = query.ProjectPopulationInclude
	requireChordDimmed(t, app, []string{"F"}, "x", false)
	requireChordDimmed(t, app, []string{"F"}, "p", false)
	out.Sort.Reset()
	out.TreeSort.Reset()
	requireChordDimmed(t, app, []string{"o"}, "x", true)
	out.TreeSort.Toggle(query.SortByPriority)
	requireChordDimmed(t, app, []string{"o"}, "x", false)
	out.Sort = out.TreeSort
	requireChordDimmed(t, app, []string{"o"}, "!", false) // Reverses direction.
}

func TestChordDimmingReadsTaskValuesAndWholeSelection(t *testing.T) {
	app := newFilterChordTestApp(t)
	out := app.crossOutliner
	project := out.Population.Projects[0]
	first, second := project.Tasks[0], project.Tasks[1]
	out.NavigateToTaskInProject(first, project)
	requireChordDimmed(t, app, []string{"!"}, "1", true)
	requireChordDimmed(t, app, []string{"!"}, "2", false)
	requireChordDimmed(t, app, []string{"w"}, "b", true)
	app.domain.runtime.Planning().SetToday([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: first.ID}})
	requireChordDimmed(t, app, []string{"w"}, "b", false)
	requireChordDimmed(t, app, []string{"w"}, "t", false)
	requireChordDimmed(t, app, []string{"w"}, "n", false)
	first.ApplyWorkflowState(models.StateWaiting)
	requireChordDimmed(t, app, []string{"w"}, "w", true)
	requireChordDimmed(t, app, []string{"w"}, "i", false)
	first.ApplyWorkflowState(models.StateDone)
	requireChordDimmed(t, app, []string{"w"}, "g", true)
	requireChordDimmed(t, app, []string{"w"}, "d", true)
	requireChordDimmed(t, app, []string{"w"}, "i", false)

	first.ApplyWorkflowState(models.StateWaiting)
	out.ReplaceSelection([]taskref.TaskIdentity{taskref.For(first, project), taskref.For(second, project)})
	requireChordDimmed(t, app, []string{"!"}, "1", false)
	second.Priority = 1
	requireChordDimmed(t, app, []string{"!"}, "1", true)
	// A bulk transition rejects the whole batch if any member is already set.
	requireChordDimmed(t, app, []string{"w"}, "w", true)
	requireChordDimmed(t, app, []string{"w"}, "g", false)
	second.ParentID = &first.ID
	requireChordDimmed(t, app, []string{"w"}, "d", true)
}

func TestChordDimmingScrollAgreesWithDispatch(t *testing.T) {
	app, out, _, _ := scrollAnchorApp(t)
	for _, key := range []string{"t", "z", "b"} {
		app.operations.Chord = nil
		result := executeDirectPath(t, app, app.operationCommandContext(), []string{"z", key})
		if result.Err != nil {
			t.Fatal(result.Err)
		}
		out.ComputeOutlinerViewport(100, 0)
		requireChordDimmed(t, app, []string{"z"}, key, true)
	}
}
