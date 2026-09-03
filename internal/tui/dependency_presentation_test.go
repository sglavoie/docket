package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
)

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

// TestDependencyIsolationPresentationIsStateIndependent is the positive half of
// the presentation isolation contract. A soft reference changes nothing about
// how the subject is drawn beyond one count segment and a detail line, and the
// denominator of that count never varies with what the references point at: it
// is over stored locators, whatever state they reach.
func TestDependencyIsolationPresentationIsStateIndependent(t *testing.T) {
	alpha, beta := dependencyPresentationFixture(t)
	source := alpha.GetTaskByID("source")
	population := dependencyTestIndex(alpha, beta)

	base := ansi.Strip(renderDependencyIndicator(source, population))
	if base != "~0/2" {
		t.Fatalf("indicator = %q, want ~0/2", base)
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
	if !strings.Contains(withRefs, "~0/2") {
		t.Fatalf("row omitted the reference count: %q", withRefs)
	}
	stripped := strings.Join(strings.Fields(strings.ReplaceAll(withRefs, "~0/2", "")), " ")
	if stripped != strings.Join(strings.Fields(withoutRefs), " ") {
		t.Fatalf("row differs beyond the reference count:\nwith:    %q\nwithout: %q", withRefs, withoutRefs)
	}

	// Close one target and leave the subject itself terminal. The settled half
	// moves, because that is what the enriched count is for; the denominator does
	// not, because a hard-dependency model is the only one that would drop a
	// reference on the way to done.
	beta.GetTaskByID("remote").WorkflowState = models.StateDone
	source.WorkflowState = models.StateDone
	got := ansi.Strip(renderDependencyIndicator(source, dependencyTestIndex(alpha, beta)))
	if got != "~1/2" {
		t.Fatalf("indicator = %q after the target closed and the subject completed, want ~1/2", got)
	}
}

// TestDependencyIndicatorColorReadsTargetOutlook pins the one thing the row says
// about its targets. The denominator is fixed at 2 in every case; the settled
// half and the color carry the aggregate reading, and the unreadable tier stays
// muted rather than green because a view that did not load a project cannot
// claim its task is finished.
func TestDependencyIndicatorColorReadsTargetOutlook(t *testing.T) {
	alpha, beta := dependencyPresentationFixture(t)
	source := alpha.GetTaskByID("source")
	sibling := alpha.GetTaskByID("sibling")
	remote := beta.GetTaskByID("remote")

	// Both targets open: one blocked reference is enough.
	got := renderDependencyIndicator(source, dependencyTestIndex(alpha, beta))
	if want := theme.DependencyIndicatorBlockedStyle.Render("~0/2"); got != want {
		t.Fatalf("open targets rendered %q, want %q", got, want)
	}

	// Close the same-project target only. A blocked target outranks a settled one,
	// so the row still says there is something left to wait on, and the settled
	// half moves to say how much of the wait is already over.
	sibling.WorkflowState = models.StateDone
	got = renderDependencyIndicator(source, dependencyTestIndex(alpha, beta))
	if want := theme.DependencyIndicatorBlockedStyle.Render("~1/2"); got != want {
		t.Fatalf("one open target rendered %q, want %q", got, want)
	}

	// Every target terminal. A fully settled census says its total alone: `S/N`
	// exists to name a remainder, and there is none.
	remote.WorkflowState = models.StateDone
	got = renderDependencyIndicator(source, dependencyTestIndex(alpha, beta))
	if want := theme.DependencyIndicatorSettledStyle.Render("~2"); got != want {
		t.Fatalf("settled targets rendered %q, want %q", got, want)
	}

	// Beta unloaded, Alpha's target done: nothing blocked is readable, but the
	// second locator points somewhere this view cannot read, which the trailing
	// `?` says and the muted tier colors.
	got = renderDependencyIndicator(source, dependencyTestIndex(alpha))
	if want := theme.MutedStyle.Render("~1/2?"); got != want {
		t.Fatalf("unreadable target rendered %q, want %q", got, want)
	}

	// A loaded project whose task is gone is settled, not unreadable: the view did
	// read that project and found nothing blocked in it.
	alpha.Tasks = []*models.Task{source}
	got = renderDependencyIndicator(source, dependencyTestIndex(alpha, beta))
	if want := theme.DependencyIndicatorSettledStyle.Render("~2"); got != want {
		t.Fatalf("absent target rendered %q, want %q", got, want)
	}
}

func TestOutgoingDependencyDetailsNameLoadedTargetsAndFallBackToLocators(t *testing.T) {
	alpha, beta := dependencyPresentationFixture(t)
	source := alpha.GetTaskByID("source")

	details := outgoingDependencyDetails(source, dependencyTestIndex(alpha, beta))
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
	fallback := outgoingDependencyDetails(source, dependencyTestIndex(alpha))
	if len(fallback) != 2 {
		t.Fatalf("fallback dropped an entry: %#v", fallback)
	}
	if got := fallback[1].String(); got != "proj-b:remote (unavailable)" {
		t.Fatalf("unavailable entry = %q", got)
	}
}

func TestIncomingDependencyDetailsAreComputedFromLoadedProjectsOnly(t *testing.T) {
	alpha, beta := dependencyPresentationFixture(t)

	dependents := incomingDependencyDetails(TaskIdentity{ProjectID: "proj-b", TaskID: "remote"}, dependencyTestIndex(alpha, beta))
	if len(dependents) != 1 || dependents[0].String() != "Alpha #1 Wire the importer (doing)" {
		t.Fatalf("dependents = %#v, want the referring task named", dependents)
	}

	// Drop the referring project from the population and the incoming view is
	// empty rather than wrong: dependents are derived, and a derivation over a
	// smaller population is honestly smaller.
	if got := incomingDependencyDetails(TaskIdentity{ProjectID: "proj-b", TaskID: "remote"}, dependencyTestIndex(beta)); len(got) != 0 {
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
	if !strings.Contains(ansi.Strip(lines), "Dependencies 0/2: Alpha #2 Agree the schema (backlog) · Beta #1 Publish the contract (waiting)") {
		t.Fatalf("Peek did not name the outgoing references:\n%s", ansi.Strip(lines))
	}

	target, ok := findPlanningTaskRow(outliner, "remote")
	if !ok {
		t.Fatal("fixture produced no row for the referenced task")
	}
	targetLines := ansi.Strip(strings.Join(outliner.inlinePeekLinesForRow(target, "  ", 200, nil), "\n"))
	if !strings.Contains(targetLines, "Dependents 1: Alpha #1 Wire the importer (doing)") {
		t.Fatalf("Peek did not name the incoming references:\n%s", targetLines)
	}
	// The referenced task stores nothing, so it carries no Dependencies line and
	// no `~N` count of its own.
	if strings.Contains(targetLines, "Dependencies ") {
		t.Fatalf("a task with no stored references grew a Dependencies line:\n%s", targetLines)
	}
}

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

	detail := dependencyDetailUnresolved(unreadable)
	if detail.state != dependencyDetailUnavailable {
		t.Fatalf("detail state = %q, want %q", detail.state, dependencyDetailUnavailable)
	}

	item := ListItem{}
	for _, candidate := range operationTaskReferenceListSource([]*models.Task{source}, population).Items {
		if candidate.ID == ListItemID("reference:"+unreadable.Locator()) {
			item = candidate
			break
		}
	}
	if item.ID == "" {
		t.Fatal("the picker dropped the unreadable reference instead of listing it")
	}
	if item.PrimaryText != detail.label {
		t.Errorf("picker label = %q, detail label = %q", item.PrimaryText, detail.label)
	}
	if item.SecondaryText != detail.state {
		t.Errorf("picker status = %q, detail status = %q", item.SecondaryText, detail.state)
	}
	if got := item.PrimaryText + " (" + item.SecondaryText + ")"; got != detail.String() {
		t.Errorf("picker row reads %q, detail row reads %q", got, detail.String())
	}

	// Peek paints that same detail through outgoingDependencyDetails, so the
	// rendered line must carry the picker's words verbatim.
	outliner := scopeTestOutliner(alpha)
	row, ok := findPlanningTaskRow(outliner, "source")
	if !ok {
		t.Fatal("fixture produced no row for the referring task")
	}
	peek := ansi.Strip(strings.Join(outliner.inlinePeekLinesForRow(row, "  ", 200, nil), "\n"))
	if !strings.Contains(peek, detail.String()) {
		t.Fatalf("Peek does not carry %q:\n%s", detail.String(), peek)
	}

	// And the indicator agrees about the underlying fact rather than claiming the
	// reference is settled. The same-project target is closed first so the
	// unreadable one is the only thing left for the outlook to read.
	alpha.GetTaskByID("sibling").WorkflowState = models.StateDone
	if got := dependencyCensusFor(source, dependencyTestIndex(alpha)).outlook(); got != dependencyOutlookUnreadable {
		t.Errorf("outlook = %v, want the unreadable tier", got)
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
