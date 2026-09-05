package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/valuepicker"
)

// TestUnavailableReferenceWordingIsSharedByEverySurface ties the three places a
// reference whose target this view cannot read is described to one fact. The
// picker is an editing surface and the other two are read-only, so nothing in
// the type system stops them drifting apart; what stops it is that all three
// read dependencyDetailUnavailable and the same durable locator, and this test
// is where that is checked rather than assumed.
//
// The row indicator carries no wording of its own — its whole statement is a
// color — so its share of the contract is that it reads the same reference as
// unreadable rather than settled, which is asserted here beside the words.
func TestUnavailableReferenceWordingIsSharedByEverySurface(t *testing.T) {
	alpha, _ := dependencyPresentationFixture(t)
	source := alpha.GetTaskByID("source")
	// Beta is deliberately left out of the population: proj-b:remote is then a
	// reference this view cannot read, which is the only case that produces the
	// wording under test.
	population := []*models.Project{alpha}
	unreadable := models.TaskReference{ProjectID: "proj-b", TaskID: "remote"}

	detail := outliner.DependencyDetailUnresolved(unreadable)
	if detail.State != outliner.DependencyDetailUnavailable {
		t.Fatalf("detail state = %q, want %q", detail.State, outliner.DependencyDetailUnavailable)
	}

	item := listsurface.ListItem{}
	for _, candidate := range valuepicker.OperationTaskReferenceListSource([]*models.Task{source}, population).Items {
		if candidate.ID == listsurface.ListItemID("reference:"+unreadable.Locator()) {
			item = candidate
			break
		}
	}
	if item.ID == "" {
		t.Fatal("the picker dropped the unreadable reference instead of listing it")
	}
	if item.PrimaryText != detail.Label {
		t.Errorf("picker label = %q, detail label = %q", item.PrimaryText, detail.Label)
	}
	if item.SecondaryText != detail.State {
		t.Errorf("picker status = %q, detail status = %q", item.SecondaryText, detail.State)
	}
	if got := item.PrimaryText + " (" + item.SecondaryText + ")"; got != detail.String() {
		t.Errorf("picker row reads %q, detail row reads %q", got, detail.String())
	}

	// Peek paints that same detail through outgoingDependencyDetails, so the
	// rendered line must carry the picker's words verbatim.
	outlinerView := scopeTestOutliner(alpha)
	row, ok := findPlanningTaskRow(outlinerView, "source")
	if !ok {
		t.Fatal("fixture produced no row for the referring task")
	}
	peek := ansi.Strip(strings.Join(outlinerView.InlinePeekLinesForRow(row, "  ", 200, nil), "\n"))
	if !strings.Contains(peek, detail.String()) {
		t.Fatalf("Peek does not carry %q:\n%s", detail.String(), peek)
	}

	// And the indicator agrees about the underlying fact rather than claiming the
	// reference is settled. The same-project target is closed first so the
	// unreadable one is the only thing left for the outlook to read.
	alpha.GetTaskByID("sibling").WorkflowState = models.StateDone
	if got := outliner.DependencyCensusFor(source, dependencyTestIndex(alpha)).Outlook(); got != outliner.DependencyOutlookUnreadable {
		t.Errorf("outlook = %v, want the unreadable tier", got)
	}
}

// dependencyTestIndex builds the reference index the render path reads. A test
// builds it explicitly for the same reason the model caches one: the index is
// the population's reading of the graph, so naming the population is naming the
// answer.
func dependencyTestIndex(projects ...*models.Project) *app.DependencyIndex {
	return app.NewDependencyIndex(projects)
}

// dependencyPresentationFixture seeds a referenced project and a referring one
// so both directions of the detail, and the unavailable fallback, have a real
// subject. beta is deliberately returned separately: a caller that omits it from
// the loaded population is exercising exactly the "project not loaded" case.
func dependencyPresentationFixture(t *testing.T) (*models.Project, *models.Project) {
	t.Helper()
	source := renderFixtureTask(t, "source", 1, "Wire the importer", models.StateDoing, 0, models.StateDoing)
	sibling := renderFixtureTask(t, "sibling", 2, "Agree the schema", models.StateBacklog, 0, models.StateBacklog)
	remote := renderFixtureTask(t, "remote", 1, "Publish the contract", models.StateWaiting, 0, models.StateWaiting)
	alpha := renderFixtureProject(t, "proj-a", "alpha", "Alpha", []*models.Task{source, sibling})
	beta := renderFixtureProject(t, "proj-b", "beta", "Beta", []*models.Task{remote})
	source.Dependencies = []models.TaskReference{
		{ProjectID: "proj-a", TaskID: "sibling"},
		{ProjectID: "proj-b", TaskID: "remote"},
	}
	return alpha, beta
}
