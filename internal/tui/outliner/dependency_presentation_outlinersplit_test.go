package outliner

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
)

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
	withRefs := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 120, 120, 0, nil))
	stored := row.Task.Dependencies
	row.Task.Dependencies = nil
	withoutRefs := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 120, 120, 0, nil))
	row.Task.Dependencies = stored
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

	dependents := incomingDependencyDetails(taskref.TaskIdentity{ProjectID: "proj-b", TaskID: "remote"}, dependencyTestIndex(alpha, beta))
	if len(dependents) != 1 || dependents[0].String() != "Alpha #1 Wire the importer (doing)" {
		t.Fatalf("dependents = %#v, want the referring task named", dependents)
	}

	// Drop the referring project from the population and the incoming view is
	// empty rather than wrong: dependents are derived, and a derivation over a
	// smaller population is honestly smaller.
	if got := incomingDependencyDetails(taskref.TaskIdentity{ProjectID: "proj-b", TaskID: "remote"}, dependencyTestIndex(beta)); len(got) != 0 {
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
	// The document rather than the page: both entries are in Peek whatever page
	// the pane happens to be showing, which is the whole of what paging bought.
	lines := ansi.Strip(strings.Join(peekDocumentBody(outliner, row, 200), "\n"))
	if !strings.Contains(lines, "Dependencies 0/2: Alpha #2 Agree the schema (backlog)") {
		t.Fatalf("Peek did not name the first outgoing reference:\n%s", lines)
	}
	if !strings.Contains(lines, "Beta #1 Publish the contract (waiting)") {
		t.Fatalf("Peek did not name the second outgoing reference:\n%s", lines)
	}

	target, ok := findPlanningTaskRow(outliner, "remote")
	if !ok {
		t.Fatal("fixture produced no row for the referenced task")
	}
	targetLines := ansi.Strip(strings.Join(peekDocumentBody(outliner, target, 200), "\n"))
	if !strings.Contains(targetLines, "Dependents 1: Alpha #1 Wire the importer (doing)") {
		t.Fatalf("Peek did not name the incoming references:\n%s", targetLines)
	}
	// The referenced task stores nothing, so it carries no Dependencies line and
	// no `~N` count of its own.
	if strings.Contains(targetLines, "Dependencies ") {
		t.Fatalf("a task with no stored references grew a Dependencies line:\n%s", targetLines)
	}
}

// TestInlinePeekReferenceLinesNameEveryEntry is the claim that replaced the
// old overflow count: a reference list is complete in the document, one entry
// per row, and the page is what bounds what is on screen. An unavailable entry
// keeps its stored locator, because a reference whose target cannot be read is
// exactly the one worth seeing.
func TestInlinePeekReferenceLinesNameEveryEntry(t *testing.T) {
	details := []DependencyDetail{
		{Label: "Alpha #1 One", State: "doing"},
		{Label: "Alpha #2 Two", State: "backlog"},
		{Label: "Alpha #3 Three", State: "backlog"},
		{Label: "proj-b:remote", State: DependencyDetailUnavailable},
	}
	lines := inlinePeekReferenceLines("Dependencies: ", details, "", 200)
	if len(lines) != len(details) {
		t.Fatalf("reference list drew %d rows for %d entries:\n%s",
			len(lines), len(details), ansi.Strip(strings.Join(lines, "\n")))
	}
	if got := ansi.Strip(lines[0]); got != "Dependencies: Alpha #1 One (doing)" {
		t.Fatalf("first row = %q", got)
	}
	if got := ansi.Strip(lines[3]); got != "              proj-b:remote (unavailable)" {
		t.Fatalf("unavailable row = %q", got)
	}
	if inlinePeekReferenceLines("Dependencies: ", nil, "", 200) != nil {
		t.Fatal("an empty reference list produced a line")
	}
}
