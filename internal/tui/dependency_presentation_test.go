package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

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

// TestDependencyIsolationPresentationIsStateIndependent is the positive half of
// the presentation isolation contract. A soft reference changes nothing about
// how the subject is drawn beyond adding a count and a detail line, and the
// count does not vary with what the reference points at: it is over stored
// locators, not over open ones. Only the indicator's color reads the targets,
// which is why every assertion here is over ansi.Strip output;
// TestDependencyIndicatorColorReadsTargetOutlook below owns the color.
func TestDependencyIsolationPresentationIsStateIndependent(t *testing.T) {
	alpha, beta := dependencyPresentationFixture(t)
	source := alpha.GetTaskByID("source")
	population := []*models.Project{alpha, beta}

	base := ansi.Strip(renderDependencyIndicator(source, population))
	if base != "~2" {
		t.Fatalf("indicator = %q, want ~2", base)
	}

	// The subject's own state badge, priority, and title styling are decided
	// without ever reading the reference list, so dropping the list changes the
	// row by exactly the count and nothing else.
	outliner := scopeTestOutliner(alpha, beta)
	row, ok := findPlanningTaskRow(outliner, "source")
	if !ok {
		t.Fatal("fixture produced no row for the referring task")
	}
	withRefs := ansi.Strip(outliner.renderOutlinerTaskRow(row, false, 120, 120, 0, nil))
	stored := row.task.Dependencies
	row.task.Dependencies = nil
	withoutRefs := ansi.Strip(outliner.renderOutlinerTaskRow(row, false, 120, 120, 0, nil))
	row.task.Dependencies = stored
	if !strings.Contains(withRefs, "~2") {
		t.Fatalf("row omitted the reference count: %q", withRefs)
	}
	stripped := strings.Join(strings.Fields(strings.ReplaceAll(withRefs, "~2", "")), " ")
	if stripped != strings.Join(strings.Fields(withoutRefs), " ") {
		t.Fatalf("row differs beyond the reference count:\nwith:    %q\nwithout: %q", withRefs, withoutRefs)
	}

	// Close one target and leave the subject itself terminal. A hard-dependency
	// model would have to change the count for at least one of these; a soft one
	// cannot change it for any, however the color reads afterwards.
	beta.GetTaskByID("remote").WorkflowState = models.StateDone
	source.WorkflowState = models.StateDone
	if got := ansi.Strip(renderDependencyIndicator(source, population)); got != base {
		t.Fatalf("indicator = %q after the target closed and the subject completed, want %q", got, base)
	}
}

// TestDependencyIndicatorColorReadsTargetOutlook pins the one thing the row says
// about its targets. The count is fixed at `~2` in all three cases; the color
// carries the aggregate reading, and the unreadable tier stays muted rather than
// green because a view that did not load a project cannot claim its task is
// finished.
func TestDependencyIndicatorColorReadsTargetOutlook(t *testing.T) {
	alpha, beta := dependencyPresentationFixture(t)
	source := alpha.GetTaskByID("source")
	sibling := alpha.GetTaskByID("sibling")
	remote := beta.GetTaskByID("remote")

	// Both targets open: one outstanding reference is enough.
	got := renderDependencyIndicator(source, []*models.Project{alpha, beta})
	if want := DependencyIndicatorOutstandingStyle.Render("~2"); got != want {
		t.Fatalf("open targets rendered %q, want %q", got, want)
	}

	// Close the same-project target only. An outstanding target outranks a settled
	// one, so the row still says there is something left to wait on.
	sibling.WorkflowState = models.StateDone
	got = renderDependencyIndicator(source, []*models.Project{alpha, beta})
	if want := DependencyIndicatorOutstandingStyle.Render("~2"); got != want {
		t.Fatalf("one open target rendered %q, want %q", got, want)
	}

	// Every target terminal.
	remote.WorkflowState = models.StateDone
	got = renderDependencyIndicator(source, []*models.Project{alpha, beta})
	if want := DependencyIndicatorSettledStyle.Render("~2"); got != want {
		t.Fatalf("settled targets rendered %q, want %q", got, want)
	}

	// Beta unloaded, Alpha's target done: nothing outstanding is readable, but the
	// second locator points somewhere this view cannot read.
	got = renderDependencyIndicator(source, []*models.Project{alpha})
	if want := MutedStyle.Render("~2"); got != want {
		t.Fatalf("unreadable target rendered %q, want %q", got, want)
	}
	if stripped := ansi.Strip(got); stripped != "~2" {
		t.Fatalf("count = %q across every outlook, want ~2", stripped)
	}

	// A loaded project whose task is gone is settled, not unreadable: the view did
	// read that project and found nothing outstanding in it.
	alpha.Tasks = []*models.Task{source}
	got = renderDependencyIndicator(source, []*models.Project{alpha, beta})
	if want := DependencyIndicatorSettledStyle.Render("~2"); got != want {
		t.Fatalf("absent target rendered %q, want %q", got, want)
	}
}

func TestOutgoingDependencyDetailsNameLoadedTargetsAndFallBackToLocators(t *testing.T) {
	alpha, beta := dependencyPresentationFixture(t)
	source := alpha.GetTaskByID("source")

	details := outgoingDependencyDetails(source, []*models.Project{alpha, beta})
	if len(details) != 2 {
		t.Fatalf("details = %#v, want two entries in stored order", details)
	}
	if got := details[0].String(); got != "Alpha #2 Agree the schema (backlog)" {
		t.Fatalf("same-project entry = %q", got)
	}
	if got := details[1].String(); got != "Beta #1 Publish the contract (waiting)" {
		t.Fatalf("cross-project entry = %q", got)
	}

	// With Beta unloaded the second entry keeps its durable locator rather than
	// disappearing, and says only that it is unavailable — not that it is missing,
	// which is a fact this view cannot establish.
	fallback := outgoingDependencyDetails(source, []*models.Project{alpha})
	if len(fallback) != 2 {
		t.Fatalf("fallback dropped an entry: %#v", fallback)
	}
	if got := fallback[1].String(); got != "proj-b:remote (unavailable)" {
		t.Fatalf("unavailable entry = %q", got)
	}
}

func TestIncomingDependencyDetailsAreComputedFromLoadedProjectsOnly(t *testing.T) {
	alpha, beta := dependencyPresentationFixture(t)

	dependents := incomingDependencyDetails(TaskIdentity{ProjectID: "proj-b", TaskID: "remote"}, []*models.Project{alpha, beta})
	if len(dependents) != 1 || dependents[0].String() != "Alpha #1 Wire the importer (doing)" {
		t.Fatalf("dependents = %#v, want the referring task named", dependents)
	}

	// Drop the referring project from the population and the incoming view is
	// empty rather than wrong: dependents are derived, and a derivation over a
	// smaller population is honestly smaller.
	if got := incomingDependencyDetails(TaskIdentity{ProjectID: "proj-b", TaskID: "remote"}, []*models.Project{beta}); len(got) != 0 {
		t.Fatalf("dependents = %#v over an unloaded referrer, want none", got)
	}
}

func TestInlinePeekNamesBothReferenceDirections(t *testing.T) {
	alpha, beta := dependencyPresentationFixture(t)
	outliner := scopeTestOutliner(alpha, beta)

	row, ok := findPlanningTaskRow(outliner, "source")
	if !ok {
		t.Fatal("fixture produced no row for the referring task")
	}
	lines := strings.Join(outliner.inlinePeekLinesForRow(row, "  ", 200, nil), "\n")
	if !strings.Contains(ansi.Strip(lines), "Dependencies: Alpha #2 Agree the schema (backlog) · Beta #1 Publish the contract (waiting)") {
		t.Fatalf("Peek did not name the outgoing references:\n%s", ansi.Strip(lines))
	}

	target, ok := findPlanningTaskRow(outliner, "remote")
	if !ok {
		t.Fatal("fixture produced no row for the referenced task")
	}
	targetLines := ansi.Strip(strings.Join(outliner.inlinePeekLinesForRow(target, "  ", 200, nil), "\n"))
	if !strings.Contains(targetLines, "Dependents: Alpha #1 Wire the importer (doing)") {
		t.Fatalf("Peek did not name the incoming references:\n%s", targetLines)
	}
	// The referenced task stores nothing, so it carries no Dependencies line and
	// no `~N` count of its own.
	if strings.Contains(targetLines, "Dependencies:") {
		t.Fatalf("a task with no stored references grew a Dependencies line:\n%s", targetLines)
	}
}

func TestInlinePeekReferenceLineCountsTheOverflow(t *testing.T) {
	details := []dependencyDetail{
		{label: "Alpha #1 One", state: "doing"},
		{label: "Alpha #2 Two", state: "backlog"},
		{label: "Alpha #3 Three", state: "backlog"},
		{label: "Alpha #4 Four", state: "backlog"},
	}
	got := ansi.Strip(inlinePeekReferenceLine("Dependencies", details, 200))
	want := "Dependencies: Alpha #1 One (doing) · Alpha #2 Two (backlog) · +2 more"
	if got != want {
		t.Fatalf("line = %q, want %q", got, want)
	}
	if inlinePeekReferenceLine("Dependencies", nil, 200) != "" {
		t.Fatal("an empty reference list produced a line")
	}
}
