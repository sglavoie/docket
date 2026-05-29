package selector

import "testing"

func TestNewItemCopiesColumnsAndHeaders(t *testing.T) {
	columns := []string{"#1", "Task"}
	headers := []string{"ID", "Title"}

	item := NewItem("task-1", "payload", columns, headers)

	columns[0] = "#2"
	headers[1] = "Name"

	if item.Columns[0] != "#1" {
		t.Fatalf("Columns[0] = %q, want %q", item.Columns[0], "#1")
	}
	if item.Headers[1] != "Title" {
		t.Fatalf("Headers[1] = %q, want %q", item.Headers[1], "Title")
	}
}

func TestCloneCopiesColumnsAndHeaders(t *testing.T) {
	original := Item{
		ID:      "task-1",
		Data:    "payload",
		Columns: []string{"#1", "Task"},
		Headers: []string{"ID", "Title"},
	}

	clone := original.Clone()
	original.Columns[0] = "#2"
	original.Headers[1] = "Name"

	if clone.Columns[0] != "#1" {
		t.Fatalf("clone.Columns[0] = %q, want %q", clone.Columns[0], "#1")
	}
	if clone.Headers[1] != "Title" {
		t.Fatalf("clone.Headers[1] = %q, want %q", clone.Headers[1], "Title")
	}
	if clone.ID != original.ID {
		t.Fatalf("clone.ID = %q, want %q", clone.ID, original.ID)
	}
	if clone.Data != original.Data {
		t.Fatalf("clone.Data = %v, want %v", clone.Data, original.Data)
	}
}
