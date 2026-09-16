package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
)

// deleteRefs qualifies the named tasks of one project the way a task surface
// hands them to the canonical delete request.
func deleteRefs(project *models.Project, taskIDs ...string) []TaskProjectRef {
	refs := make([]TaskProjectRef, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		refs = append(refs, TaskProjectRef{Task: project.GetTaskByID(taskID), Project: project})
	}
	return refs
}

func TestSoleTaskDeleteTargetNamesOnlyAOneTaskGesture(t *testing.T) {
	project := scopeTestProject("alpha", "Alpha", "one", "two")

	target := soleTaskDeleteTarget(deleteRefs(project, "one"))
	if want := (app.TaskLocator{ProjectID: "alpha", TaskID: "one"}); target != want {
		t.Fatalf("one requested task = %+v, want %+v", target, want)
	}
	if target := soleTaskDeleteTarget(deleteRefs(project, "one", "two")); target.Valid() {
		t.Fatalf("two requested tasks = %+v, want the uncorrelated zero locator", target)
	}
	if target := soleTaskDeleteTarget(nil); target.Valid() {
		t.Fatalf("no requested task = %+v, want the uncorrelated zero locator", target)
	}
}

func TestSoleTaskDeleteTargetLeavesANestedSelectionUncorrelated(t *testing.T) {
	project := scopeTestProject("alpha", "Alpha", "parent", "child")
	parentID := "parent"
	project.GetTaskByID("child").ParentID = &parentID
	project.BuildTaskTree()

	requested := canonicalizeTaskDeleteRoots(deleteRefs(project, "parent", "child"))
	roots := collapseNestedTaskDeleteRoots(requested)
	if len(roots) != 1 {
		t.Fatalf("collapsed roots = %d, want the one ancestor", len(roots))
	}
	// The subtree collapses to one root, but the user named two rows. The stamp
	// reads the requested refs for exactly that reason: a gesture over several
	// tasks stays uncorrelated even when one transaction carries all of them.
	if target := soleTaskDeleteTarget(requested); target.Valid() {
		t.Fatalf("nested two-task selection = %+v, want the uncorrelated zero locator", target)
	}
}

func TestStampCommittedFocusTargetOnlyDecoratesACommittedBatch(t *testing.T) {
	target := app.TaskLocator{ProjectID: "alpha", TaskID: "one"}

	committed := coordinator.StampCommittedFocusTarget(func() tea.Msg {
		return coordinator.BatchTransactionResultMsg{TransactionID: "tx"}
	}, target)
	batch, ok := committed().(coordinator.BatchTransactionResultMsg)
	if !ok || batch.CommittedTarget != target {
		t.Fatalf("committed batch = %+v, want CommittedTarget %+v", batch, target)
	}

	// A canceled or refused deletion resolves to TransactionErrorMsg, which the
	// decorator passes through untouched: nothing may correlate a transaction
	// that never committed.
	refused := coordinator.StampCommittedFocusTarget(func() tea.Msg {
		return coordinator.TransactionErrorMsg{}
	}, target)
	if _, ok := refused().(coordinator.TransactionErrorMsg); !ok {
		t.Fatalf("refused gesture = %T, want the untouched TransactionErrorMsg", refused())
	}

	unnamed := coordinator.StampCommittedFocusTarget(func() tea.Msg {
		return coordinator.BatchTransactionResultMsg{TransactionID: "tx"}
	}, app.TaskLocator{})
	bare, ok := unnamed().(coordinator.BatchTransactionResultMsg)
	if !ok || bare.CommittedTarget.Valid() {
		t.Fatalf("multi-target batch = %+v, want no CommittedTarget", bare)
	}
}
