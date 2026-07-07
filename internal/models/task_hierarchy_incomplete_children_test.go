package models

import "testing"

func TestIncompleteChildrenMixedStatuses(t *testing.T) {
	epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	done, err := NewTask("done", 2, TaskTypeTask, "Done")
	if err != nil {
		t.Fatal(err)
	}
	open, err := NewTask("open", 3, TaskTypeTask, "Open")
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
	leaf, err := NewTask("leaf", 1, TaskTypeTask, "Leaf")
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
	epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := NewTask("c1", 2, TaskTypeTask, "Child 1")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := NewTask("c2", 3, TaskTypeTask, "Child 2")
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
	epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	canceled, err := NewTask("canceled", 2, TaskTypeTask, "Canceled")
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
