package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/docket/internal/config"
)

func TestTUIPlanningKeysPreserveIncomingPlansOnUndoRedo(t *testing.T) {
	driver, workspace, project := newQuickCaptureInterruptFixture(t)
	openFixtureOutliner(t, driver, project)
	targetID := project.Tasks[0].ID
	otherID := project.Tasks[1].ID
	runChecklistActions(t, driver, []UITestAction{
		{Type: "key", Key: "w"},
		{Type: "key", Key: "t"},
	})
	store := config.NewDomainFileStore(workspace.DataDir)
	target := config.ProjectTaskLocator{ProjectID: project.ID, TaskID: targetID}
	other := config.ProjectTaskLocator{ProjectID: project.ID, TaskID: otherID}
	for _, step := range []struct {
		key   string
		today bool
	}{
		{"u", false}, {"ctrl+r", true}, {"u", false},
	} {
		planning, err := store.LoadPlanningReadOnly()
		if err != nil {
			t.Fatal(err)
		}
		if step.key == "u" {
			planning.RemoveFromTodayInProject(otherID, project.ID)
			planning.AddToNext(otherID, project.ID)
		} else {
			planning.RemoveFromNextInProject(otherID, project.ID)
			planning.AddToToday(otherID, project.ID)
		}
		if saveErr := store.SavePlanning(planning); saveErr != nil {
			t.Fatal(saveErr)
		}
		runChecklistActions(t, driver, []UITestAction{
			{Type: "key", Key: step.key},
			{Type: "assert", Assert: "no_layout_issues"},
		})
		got, err := store.LoadPlanningReadOnly()
		if err != nil {
			t.Fatal(err)
		}
		if slices.Contains(got.Today, target) != step.today {
			t.Fatalf("after %s: target Today = %v, want %v", step.key, got.Today, step.today)
		}
		if slices.Contains(got.Today, other) != slices.Contains(planning.Today, other) ||
			slices.Contains(got.Next, other) != slices.Contains(planning.Next, other) {
			t.Fatalf("after %s: incoming plan lost: %+v", step.key, got)
		}
		runtime := driver.app.domain.runtime.Planning()
		if !slices.Equal(got.Today, runtime.Today) || !slices.Equal(got.Next, runtime.Next) {
			t.Fatalf("after %s: UI runtime did not adopt restored planning", step.key)
		}
	}
}
