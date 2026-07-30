package tui

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// testFlattenTaskTree flattens a task tree using the production
// flattenTaskTreeWithFolds method with an empty expandedNodes map
// (all nodes expanded), matching the removed flattenTaskTree behavior.
func testFlattenTaskTree(tasks []*models.Task, ancestorIsLast []bool, rows *[]outlinerRow) {
	m := &OutlinerModel{}
	m.flattenTaskTreeWithFolds(tasks, ancestorIsLast, rows)
}

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

func TestRenderTaskPrefixKeepsLeafConnectorsCompact(t *testing.T) {
	prefix, fold := renderTaskPrefix("├── ", "")
	if got, want := ansi.Strip(prefix), "├── "; got != want {
		t.Fatalf("leaf connector = %q, want compact %q", got, want)
	}
	if fold != "" {
		t.Fatalf("leaf connector should not add a filler segment, got %q", ansi.Strip(fold))
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
	epic1, err := models.NewGenericTask("e1", 1, "Epic 1")
	if err != nil {
		t.Fatal(err)
	}
	story1, err := models.NewGenericTask("s1", 2, "Story 1")
	if err != nil {
		t.Fatal(err)
	}
	task1, err := models.NewGenericTask("t1", 3, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := models.NewGenericTask("t2", 4, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	story2, err := models.NewGenericTask("s2", 5, "Story 2")
	if err != nil {
		t.Fatal(err)
	}
	epic2, err := models.NewGenericTask("e2", 6, "Epic 2")
	if err != nil {
		t.Fatal(err)
	}

	story1.Children = []*models.Task{task1, task2}
	epic1.Children = []*models.Task{story1, story2}

	roots := []*models.Task{epic1, epic2}

	var rows []outlinerRow
	testFlattenTaskTree(roots, nil, &rows)

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

func TestOutlinerPersistentScrollKeepsFirstRowMovingUpWithinViewport(t *testing.T) {
	for _, tt := range []struct {
		name    string
		project *models.Project
	}{
		{name: "single project", project: &models.Project{Name: "Single"}},
		{name: "cross project", project: nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows := make([]outlinerRow, 10)
			for i := range rows {
				rows[i] = outlinerRow{
					task: &models.Task{
						ID:      fmt.Sprintf("task-%02d", i),
						ShortID: i + 1,
						Title:   fmt.Sprintf("Task %02d", i),
					},
				}
			}
			m := &OutlinerModel{
				project:           tt.project,
				rows:              rows,
				pinnedTagTab:      make(map[string]int),
				collapsedSections: make(map[string]bool),
			}
			m.SetSize(100, 6) // One header line leaves five row slots.

			m.cursor = 5
			viewport := m.computeOutlinerViewport(100, 0)
			start := viewport.startIdx
			if start == 0 {
				t.Fatalf("expected initial viewport to scroll down, start stayed at 0")
			}
			firstTitle := m.rows[start].task.Title

			m.cursor = 4
			viewport = m.computeOutlinerViewport(100, 0)
			if viewport.startIdx != start {
				t.Fatalf("moving up inside viewport changed first rendered row: start=%d want %d", viewport.startIdx, start)
			}
			if got := m.rows[viewport.startIdx].task.Title; got != firstTitle {
				t.Fatalf("first rendered row = %q, want %q", got, firstTitle)
			}
		})
	}
}

func TestFlattenTaskTreeDeep(t *testing.T) {
	// Build a 6-level deep chain where each node is the last child.
	tasks := make([]*models.Task, 6)
	for i := range tasks {
		var err error
		tasks[i], err = models.NewGenericTask(
			"id"+string(rune('0'+i)), i+1, "Level "+string(rune('1'+i)))

		if err != nil {
			t.Fatal(err)
		}
	}
	for i := range 5 {
		tasks[i].Children = []*models.Task{tasks[i+1]}
	}

	var rows []outlinerRow
	testFlattenTaskTree([]*models.Task{tasks[0]}, nil, &rows)

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

	epic1, err := models.NewGenericTask("e1", 1, "Epic 1")
	if err != nil {
		t.Fatal(err)
	}
	story1, err := models.NewGenericTask("s1", 2, "Story 1")
	if err != nil {
		t.Fatal(err)
	}
	task1, err := models.NewGenericTask("t1", 3, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := models.NewGenericTask("t2", 4, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	leaf1, err := models.NewGenericTask("l1", 5, "Leaf 1")
	if err != nil {
		t.Fatal(err)
	}
	epic2, err := models.NewGenericTask("e2", 6, "Epic 2")
	if err != nil {
		t.Fatal(err)
	}
	story2, err := models.NewGenericTask("s2", 7, "Story 2")
	if err != nil {
		t.Fatal(err)
	}
	task3, err := models.NewGenericTask("t3", 8, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	leaf2, err := models.NewGenericTask("l2", 9, "Leaf 2")
	if err != nil {
		t.Fatal(err)
	}

	story1.Children = []*models.Task{task1, task2}
	epic1.Children = []*models.Task{story1, leaf1}
	story2.Children = []*models.Task{task3}
	epic2.Children = []*models.Task{story2, leaf2}

	roots := []*models.Task{epic1, epic2}

	var rows []outlinerRow
	testFlattenTaskTree(roots, nil, &rows)

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

func TestFlattenTaskTreeSliceAliasing(t *testing.T) {
	// Regression test for slice aliasing in tree flattening.
	// When sibling nodes at depth >= 2 each have children, the recursive
	// append(ancestorIsLast, isLast) could share a backing array if the
	// slice had spare capacity, corrupting connector prefixes.
	//
	// Tree structure:
	//   root (non-last root)
	//     parent1 (non-last child of root)
	//       sibling1 (non-last child of parent1) — has child
	//         grandchild1
	//       sibling2 (last child of parent1) — has child
	//         grandchild2
	//     parent2 (last child of root)
	//   root2 (last root)
	//
	// The bug would cause grandchild2's prefix to reflect sibling1's
	// "non-last" state as "last" because the second sibling's append
	// overwrote the first sibling's ancestor trail in the shared array.

	root, _ := models.NewGenericTask("r", 1, "Root")
	parent1, _ := models.NewGenericTask("p1", 2, "Parent 1")
	parent2, _ := models.NewGenericTask("p2", 3, "Parent 2")
	sibling1, _ := models.NewGenericTask("s1", 4, "Sibling 1")
	sibling2, _ := models.NewGenericTask("s2", 5, "Sibling 2")
	grandchild1, _ := models.NewGenericTask("gc1", 6, "Grandchild 1")
	grandchild2, _ := models.NewGenericTask("gc2", 7, "Grandchild 2")
	root2, _ := models.NewGenericTask("r2", 8, "Root 2")

	sibling1.Children = []*models.Task{grandchild1}
	sibling2.Children = []*models.Task{grandchild2}
	parent1.Children = []*models.Task{sibling1, sibling2}
	root.Children = []*models.Task{parent1, parent2}

	roots := []*models.Task{root, root2}

	var rows []outlinerRow
	testFlattenTaskTree(roots, nil, &rows)

	expected := []struct {
		title  string
		prefix string
	}{
		{"Root", "├── "},
		{"Parent 1", "│   ├── "},
		{"Sibling 1", "│   │   ├── "},
		{"Grandchild 1", "│   │   │   └── "},
		{"Sibling 2", "│   │   └── "},
		{"Grandchild 2", "│   │       └── "},
		{"Parent 2", "│   └── "},
		{"Root 2", "└── "},
	}

	if len(rows) != len(expected) {
		t.Fatalf("expected %d rows, got %d", len(expected), len(rows))
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

func TestFlattenTaskTreeNoState(t *testing.T) {
	// Verify no connector state stored on tree nodes: run flatten twice
	// and confirm identical results.
	epic, err := models.NewGenericTask("e1", 1, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	child1, err := models.NewGenericTask("c1", 2, "Child 1")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := models.NewGenericTask("c2", 3, "Child 2")
	if err != nil {
		t.Fatal(err)
	}
	epic.Children = []*models.Task{child1, child2}

	var rows1, rows2 []outlinerRow
	testFlattenTaskTree([]*models.Task{epic}, nil, &rows1)
	testFlattenTaskTree([]*models.Task{epic}, nil, &rows2)

	if len(rows1) != len(rows2) {
		t.Fatalf("different row counts: %d vs %d", len(rows1), len(rows2))
	}
	for i := range rows1 {
		if rows1[i].prefix != rows2[i].prefix {
			t.Errorf("row %d: prefix mismatch between runs: %q vs %q", i, rows1[i].prefix, rows2[i].prefix)
		}
	}
}
