package outliner

import (
	"testing"

	"github.com/sglavoie/docket/internal/models"
)

func pruneTestTree() []*models.Task {
	leaf := &models.Task{ID: "leaf"}
	gone := &models.Task{ID: "gone", Children: []*models.Task{leaf}}
	kept := &models.Task{ID: "kept", Children: []*models.Task{{ID: "kept-child"}}}
	root := &models.Task{ID: "root", Children: []*models.Task{gone}}
	return []*models.Task{kept, root}
}

func treeShape(tasks []*models.Task) string {
	shape := ""
	for _, task := range tasks {
		shape += task.ID + "(" + treeShape(task.Children) + ")"
	}
	return shape
}

// TestPruneOwnedTreeSharesWhatItLeavesAlone pins pruneOwnedTree against the
// copying pass it replaced: the same tree shape, the untouched subtree handed
// back as it stood, only the path to a removal copied, and the input unmoved.
func TestPruneOwnedTreeSharesWhatItLeavesAlone(t *testing.T) {
	removeGone := func(task *models.Task) bool { return task.ID == "gone" }
	input := pruneTestTree()
	pruned, changed := pruneOwnedTree(input, removeGone)

	want := treeShape(excludeTaskTreePromotingChildren(pruneTestTree(), removeGone))
	if got := treeShape(pruned); !changed || got != want {
		t.Fatalf("pruned shape = %q (changed %v), want the copying pass's %q", got, changed, want)
	}
	if pruned[0] != input[0] {
		t.Fatal("a subtree that lost nothing was copied")
	}
	if pruned[1] == input[1] {
		t.Fatal("the path down to a removal was shared, so the input would have changed")
	}
	if treeShape(input) != "kept(kept-child())root(gone(leaf()))" {
		t.Fatalf("the input moved: %q", treeShape(input))
	}

	same, changed := pruneOwnedTree(input, func(*models.Task) bool { return false })
	if changed || &same[0] != &input[0] {
		t.Fatal("a pass that removes nothing did not hand its input back")
	}
}
