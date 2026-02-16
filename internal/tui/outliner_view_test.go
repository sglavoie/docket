package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestBuildTreePrefix(t *testing.T) {
	tests := []struct {
		name           string
		ancestorIsLast []bool
		isLast         bool
		expected       string
	}{
		{"root non-last", nil, false, "├── "},
		{"root last", nil, true, "└── "},
		{"depth 1, parent not last, non-last", []bool{false}, false, "│   ├── "},
		{"depth 1, parent not last, last", []bool{false}, true, "│   └── "},
		{"depth 1, parent last, non-last", []bool{true}, false, "    ├── "},
		{"depth 1, parent last, last", []bool{true}, true, "    └── "},
		{"depth 3 mixed", []bool{false, true, false}, false, "│       │   ├── "},
		{"depth 3 mixed last", []bool{false, true, false}, true, "│       │   └── "},
		{"depth 5 all not last", []bool{false, false, false, false, false}, false, "│   │   │   │   │   ├── "},
		{"depth 5 all last", []bool{true, true, true, true, true}, true, "                    └── "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTreePrefix(tt.ancestorIsLast, tt.isLast)
			if got != tt.expected {
				t.Errorf("buildTreePrefix(%v, %v)\ngot:  %q\nwant: %q", tt.ancestorIsLast, tt.isLast, got, tt.expected)
			}
		})
	}
}

func TestFlattenTaskTree(t *testing.T) {
	// Build a 3-level hierarchy:
	//   epic1
	//     story1
	//       task1
	//       task2
	//     story2
	//   epic2
	epic1 := models.NewTask("e1", 1, models.TaskTypeEpic, "Epic 1")
	story1 := models.NewTask("s1", 2, models.TaskTypeStory, "Story 1")
	task1 := models.NewTask("t1", 3, models.TaskTypeTask, "Task 1")
	task2 := models.NewTask("t2", 4, models.TaskTypeTask, "Task 2")
	story2 := models.NewTask("s2", 5, models.TaskTypeStory, "Story 2")
	epic2 := models.NewTask("e2", 6, models.TaskTypeEpic, "Epic 2")

	story1.Children = []*models.Task{task1, task2}
	epic1.Children = []*models.Task{story1, story2}

	roots := []*models.Task{epic1, epic2}

	var rows []outlinerRow
	flattenTaskTree(roots, nil, &rows)

	if len(rows) != 6 {
		t.Fatalf("expected 6 rows, got %d", len(rows))
	}

	expected := []struct {
		title  string
		prefix string
	}{
		{"Epic 1", "├── "},
		{"Story 1", "│   ├── "},
		{"Task 1", "│   │   ├── "},
		{"Task 2", "│   │   └── "},
		{"Story 2", "│   └── "},
		{"Epic 2", "└── "},
	}

	for i, exp := range expected {
		if rows[i].task.Title != exp.title {
			t.Errorf("row %d: expected title %q, got %q", i, exp.title, rows[i].task.Title)
		}
		if rows[i].prefix != exp.prefix {
			t.Errorf("row %d (%s): expected prefix %q, got %q", i, exp.title, exp.prefix, rows[i].prefix)
		}
	}
}

func TestFlattenTaskTreeDeep(t *testing.T) {
	// Build a 6-level deep chain where each node is the last child.
	tasks := make([]*models.Task, 6)
	for i := range tasks {
		tasks[i] = models.NewTask(
			"id"+string(rune('0'+i)), i+1, models.TaskTypeTask, "Level "+string(rune('1'+i)),
		)
	}
	for i := 0; i < 5; i++ {
		tasks[i].Children = []*models.Task{tasks[i+1]}
	}

	var rows []outlinerRow
	flattenTaskTree([]*models.Task{tasks[0]}, nil, &rows)

	if len(rows) != 6 {
		t.Fatalf("expected 6 rows, got %d", len(rows))
	}

	// When each node is the only/last child, all ancestors produce spaces.
	expectedPrefixes := []string{
		"└── ",
		"    └── ",
		"        └── ",
		"            └── ",
		"                └── ",
		"                    └── ",
	}

	for i, exp := range expectedPrefixes {
		if rows[i].prefix != exp {
			t.Errorf("row %d: expected prefix %q, got %q", i, exp, rows[i].prefix)
		}
	}
}

func TestFlattenTaskTreeLeafAndParentConnectors(t *testing.T) {
	// Build a mixed hierarchy with both leaf and parent nodes at various
	// positions (last and non-last siblings) at multiple depths:
	//
	//   epic1 (parent, non-last root)
	//     story1 (parent, non-last child of epic1)
	//       task1 (leaf, non-last child of story1)
	//       task2 (leaf, last child of story1)
	//     leaf1 (leaf, last child of epic1)
	//   epic2 (parent, last root)
	//     story2 (parent, non-last child of epic2)
	//       task3 (leaf, only/last child of story2)
	//     leaf2 (leaf, last child of epic2)

	epic1 := models.NewTask("e1", 1, models.TaskTypeEpic, "Epic 1")
	story1 := models.NewTask("s1", 2, models.TaskTypeStory, "Story 1")
	task1 := models.NewTask("t1", 3, models.TaskTypeTask, "Task 1")
	task2 := models.NewTask("t2", 4, models.TaskTypeTask, "Task 2")
	leaf1 := models.NewTask("l1", 5, models.TaskTypeTask, "Leaf 1")
	epic2 := models.NewTask("e2", 6, models.TaskTypeEpic, "Epic 2")
	story2 := models.NewTask("s2", 7, models.TaskTypeStory, "Story 2")
	task3 := models.NewTask("t3", 8, models.TaskTypeTask, "Task 3")
	leaf2 := models.NewTask("l2", 9, models.TaskTypeTask, "Leaf 2")

	story1.Children = []*models.Task{task1, task2}
	epic1.Children = []*models.Task{story1, leaf1}
	story2.Children = []*models.Task{task3}
	epic2.Children = []*models.Task{story2, leaf2}

	roots := []*models.Task{epic1, epic2}

	var rows []outlinerRow
	flattenTaskTree(roots, nil, &rows)

	if len(rows) != 9 {
		t.Fatalf("expected 9 rows, got %d", len(rows))
	}

	expected := []struct {
		title    string
		prefix   string
		isParent bool // true if node has children
	}{
		{"Epic 1", "├── ", true},          // parent, non-last root
		{"Story 1", "│   ├── ", true},     // parent, non-last at depth 1
		{"Task 1", "│   │   ├── ", false}, // leaf, non-last at depth 2
		{"Task 2", "│   │   └── ", false}, // leaf, last at depth 2
		{"Leaf 1", "│   └── ", false},     // leaf, last at depth 1
		{"Epic 2", "└── ", true},          // parent, last root
		{"Story 2", "    ├── ", true},     // parent, non-last at depth 1 (under last root)
		{"Task 3", "    │   └── ", false}, // leaf, last at depth 2 (under last root)
		{"Leaf 2", "    └── ", false},     // leaf, last at depth 1 (under last root)
	}

	for i, exp := range expected {
		if rows[i].task.Title != exp.title {
			t.Errorf("row %d: expected title %q, got %q", i, exp.title, rows[i].task.Title)
		}
		if rows[i].prefix != exp.prefix {
			t.Errorf("row %d (%s, parent=%v): expected prefix %q, got %q",
				i, exp.title, exp.isParent, exp.prefix, rows[i].prefix)
		}
		hasChildren := len(rows[i].task.Children) > 0
		if hasChildren != exp.isParent {
			t.Errorf("row %d (%s): expected parent=%v, got parent=%v",
				i, exp.title, exp.isParent, hasChildren)
		}
	}
}

func TestFlattenTaskTreeNoState(t *testing.T) {
	// Verify no connector state stored on tree nodes: run flatten twice
	// and confirm identical results.
	epic := models.NewTask("e1", 1, models.TaskTypeEpic, "Epic")
	child1 := models.NewTask("c1", 2, models.TaskTypeTask, "Child 1")
	child2 := models.NewTask("c2", 3, models.TaskTypeTask, "Child 2")
	epic.Children = []*models.Task{child1, child2}

	var rows1, rows2 []outlinerRow
	flattenTaskTree([]*models.Task{epic}, nil, &rows1)
	flattenTaskTree([]*models.Task{epic}, nil, &rows2)

	if len(rows1) != len(rows2) {
		t.Fatalf("different row counts: %d vs %d", len(rows1), len(rows2))
	}
	for i := range rows1 {
		if rows1[i].prefix != rows2[i].prefix {
			t.Errorf("row %d: prefix mismatch between runs: %q vs %q", i, rows1[i].prefix, rows2[i].prefix)
		}
	}
}
