package models

import "testing"

func TestIncompleteChildrenMixedStatuses(t *testing.T) {
	epic, err := NewGenericTask("epic", 1, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	done, err := NewGenericTask("done", 2, "Done")
	if err != nil {
		t.Fatal(err)
	}
	open, err := NewGenericTask("open", 3, "Open")
	if err != nil {
		t.Fatal(err)
	}

	if err := epic.AddChild(done); err != nil {
		t.Fatal(err)
	}
	if err := epic.AddChild(open); err != nil {
		t.Fatal(err)
	}
	done.Complete()

	incomplete := epic.IncompleteChildren()
	if len(incomplete) != 1 {
		t.Fatalf("expected 1 incomplete child, got %d", len(incomplete))
	}
	if incomplete[0].ID != open.ID {
		t.Errorf("expected the open child, got %q", incomplete[0].ID)
	}
	if !epic.HasIncompleteChildren() {
		t.Error("expected HasIncompleteChildren to be true")
	}
}

func TestIncompleteChildrenLeaf(t *testing.T) {
	leaf, err := NewGenericTask("leaf", 1, "Leaf")
	if err != nil {
		t.Fatal(err)
	}

	if got := leaf.IncompleteChildren(); len(got) != 0 {
		t.Errorf("expected no incomplete children for leaf, got %d", len(got))
	}
	if leaf.HasIncompleteChildren() {
		t.Error("expected HasIncompleteChildren to be false for a leaf")
	}
}

func TestIncompleteChildrenAllComplete(t *testing.T) {
	epic, err := NewGenericTask("epic", 1, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := NewGenericTask("c1", 2, "Child 1")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := NewGenericTask("c2", 3, "Child 2")
	if err != nil {
		t.Fatal(err)
	}

	if err := epic.AddChild(c1); err != nil {
		t.Fatal(err)
	}
	if err := epic.AddChild(c2); err != nil {
		t.Fatal(err)
	}
	c1.Complete()
	c2.Complete()

	if got := epic.IncompleteChildren(); len(got) != 0 {
		t.Errorf("expected no incomplete children, got %d", len(got))
	}
	if epic.HasIncompleteChildren() {
		t.Error("expected HasIncompleteChildren to be false when all complete")
	}
}

func TestIncompleteChildrenCanceledCountsAsComplete(t *testing.T) {
	epic, err := NewGenericTask("epic", 1, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	canceled, err := NewGenericTask("canceled", 2, "Canceled")
	if err != nil {
		t.Fatal(err)
	}

	if err := epic.AddChild(canceled); err != nil {
		t.Fatal(err)
	}
	canceled.Cancel()

	if got := epic.IncompleteChildren(); len(got) != 0 {
		t.Errorf("expected canceled child to count as complete, got %d incomplete", len(got))
	}
	if epic.HasIncompleteChildren() {
		t.Error("expected HasIncompleteChildren to be false when only child is canceled")
	}
}
