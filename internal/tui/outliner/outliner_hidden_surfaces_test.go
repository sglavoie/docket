package outliner

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestHiddenFacetTogglesTheOnlyFormAndOwesAReload(t *testing.T) {
	outliner := hiddenPopulationFixture()
	invocation := operation.OperationInvocation{
		Descriptor: operation.OperationDescriptor{ID: operation.OperationFilterHidden},
	}
	edit, err := outliner.RowFilterClauseEdit(invocation)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(query.FilterClauseParts(edit.Clauses), " "); got != "+hidden:only" {
		t.Fatalf("fH spelled %q, want +hidden:only", got)
	}
	outliner.ReplaceFilterClauses(FilterOperationFields(operation.OperationFilterHidden), edit.Clauses)
	if !outliner.AdmitsHiddenPopulation() {
		t.Fatal("fH did not open the hidden population")
	}
	edit, err = outliner.RowFilterClauseEdit(invocation)
	if err != nil || len(edit.Clauses) != 0 || edit.Status != "Hidden filter cleared" {
		t.Fatalf("a second fH = %+v, %v; want the clause dropped", edit, err)
	}
	if !FilterOperationRequiresReload(operation.OperationFilterHidden) {
		t.Fatal("fH must owe a reload, which is what refuses it under the ranked list")
	}
	if FilterOperationRequiresReload(operation.OperationFilterClear) {
		t.Fatal("fx drops the hidden clause with a rebuild and must not owe a reload")
	}
}

func TestHiddenReadoutRowPeelsThroughAReload(t *testing.T) {
	outliner := hiddenPopulationFixture()
	outliner.SetFilterQuery(hiddenOnlyQuery(t))
	outliner.RebuildRows()
	rows := outliner.QueryReadoutRows()
	index := -1
	for i, row := range rows {
		if row.value == "+hidden:only" {
			index = i
		}
	}
	if index < 0 {
		t.Fatalf("readout rows %+v name no +hidden:only row", rows)
	}
	if !queryReadoutRowRequiresReload(rows[index]) {
		t.Fatal("the hidden row must peel through a reload")
	}
	cmd := outliner.PeelQueryReadoutRow(index)
	if cmd == nil {
		t.Fatal("peeling the hidden row returned no command")
	}
	if _, ok := cmd().(QueryReadoutReloadRequestMsg); !ok {
		t.Fatal("peeling the hidden row did not ask the shell to reload")
	}
	if outliner.AdmitsHiddenPopulation() {
		t.Fatal("the peel left the hidden clause standing")
	}
}

func TestHiddenFacetOverNothingHiddenNamesTheKeyThatHides(t *testing.T) {
	visible := primaryRowsTask("only-visible", models.StateBacklog)
	outliner := scopeTestOutliner(primaryRowsProject("alpha", visible))
	outliner.SetFilterQuery(hiddenOnlyQuery(t))
	outliner.RebuildRows()
	if outliner.HasContentRows() {
		t.Fatal("fH over no hidden work drew rows")
	}
	if got := outliner.OutlinerEmptyMessage(); got != OutlinerEmptyHiddenMessage {
		t.Fatalf("empty message = %q, want %q", got, OutlinerEmptyHiddenMessage)
	}

	// A second facet beside fH is an ordinary filtered view again.
	outliner.SetFilterQuery(models.Query{Clauses: append(
		hiddenOnlyQuery(t).Clauses,
		query.FilterClauseList(models.QueryFieldPinned, models.QueryOperatorPresent, "")...)})
	outliner.RebuildRows()
	if got := outliner.OutlinerEmptyMessage(); got == OutlinerEmptyHiddenMessage {
		t.Fatal("fH beside another facet should explain the filters, not hiding")
	}
}

func TestPeekStatusLineMarksInheritedHiding(t *testing.T) {
	parent := primaryRowsTask("parent", models.StateBacklog)
	parent.Hidden = true
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &parent.ID
	visible := primaryRowsTask("visible", models.StateBacklog)
	primaryRowsProject("alpha", parent, child, visible)
	now := time.Now()
	for _, task := range []*models.Task{parent, child} {
		if line := ansi.Strip(inlinePeekStatusLine(task, "", now)); !strings.Contains(line, InlinePeekHiddenMarker) {
			t.Fatalf("Peek status for %s = %q, want %s", task.ID, line, InlinePeekHiddenMarker)
		}
	}
	if line := ansi.Strip(inlinePeekStatusLine(visible, "", now)); strings.Contains(line, InlinePeekHiddenMarker) {
		t.Fatalf("a visible task wore the hidden marker: %q", line)
	}
	if inlinePeekStatusKey(parent, "") == inlinePeekStatusKey(visible, "") {
		t.Fatal("the status key must change with hiding")
	}
}

func TestRelationHopRefusesAHiddenTargetOutsideTheHiddenFacet(t *testing.T) {
	outliner := hiddenPopulationFixture()
	project := outliner.Population.Projects[0]
	target := project.GetTaskByID("needle-child")
	if refusal := outliner.jumpToRelation(target, project); refusal != RelationHiddenTargetRefusal {
		t.Fatalf("hop to a hidden target = %q, want %q", refusal, RelationHiddenTargetRefusal)
	}
	if outliner.HasRelationTrail() {
		t.Fatal("a refused hop published a return frame")
	}

	outliner.SetFilterQuery(hiddenOnlyQuery(t))
	outliner.RebuildRows()
	if refusal := outliner.jumpToRelation(target, project); refusal != "" {
		t.Fatalf("hop under fH refused: %q", refusal)
	}
	if got := outliner.CurrentTaskIdentity(); got != taskref.For(target, project) {
		t.Fatalf("the hop under fH landed on %+v, want the hidden target", got)
	}
	if !outliner.AdmitsHiddenPopulation() {
		t.Fatal("the hop under fH dropped the hidden facet it landed in")
	}
}

func TestFindMenuOffersShowHiddenAfterPinned(t *testing.T) {
	for _, menu := range OutlinerMenuSpecs("", false, false) {
		if menu.ID != MenuIDFind {
			continue
		}
		for i, entry := range menu.Entries {
			if entry.OperationID != operation.OperationFilterHidden {
				continue
			}
			if entry.Label != "Show hidden" {
				t.Fatalf("Find menu labels fH %q, want Show hidden", entry.Label)
			}
			if i == 0 || menu.Entries[i-1].OperationID != operation.OperationFilterPinned {
				t.Fatal("Show hidden should follow Filter Pinned")
			}
			return
		}
		t.Fatal("the Find menu has no Show hidden entry")
	}
	t.Fatal("no Find menu")
}

// TestHiddenRowsAdvertiseNoOwedReview keeps the row badge, the hero ladder, and
// Peek's check-in flag on the one attention predicate the Review lane and R read,
// so a hidden task drawn under fH names no review it cannot be given.
func TestHiddenRowsAdvertiseNoOwedReview(t *testing.T) {
	parent := primaryRowsTask("parent", models.StateBacklog)
	parent.Hidden = true
	parent.Review = true
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &parent.ID
	child.Review = true
	visible := primaryRowsTask("visible", models.StateBacklog)
	visible.Review = true
	primaryRowsProject("alpha", parent, child, visible)
	now := time.Now()
	for _, task := range []*models.Task{parent, child} {
		if got := formatReviewIndicator(task, now); got != "" {
			t.Fatalf("%s review badge = %q, want none while hidden", task.ID, got)
		}
		if got := chrome.PickHeroAttribute(task, now); got == chrome.HeroReview {
			t.Fatalf("%s took the review hero while hidden", task.ID)
		}
	}
	if formatReviewIndicator(visible, now) == "" || chrome.PickHeroAttribute(visible, now) != chrome.HeroReview {
		t.Fatal("a visible flagged task lost its review badge")
	}
}
