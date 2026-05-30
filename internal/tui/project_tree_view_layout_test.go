package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestTreeRenderLineCountIncludesGroupSeparators(t *testing.T) {
	visible := []*TreeNode{
		{TopGroupIndex: 0},
		{TopGroupIndex: 0},
		{TopGroupIndex: 1},
		{TopGroupIndex: 1},
		{TopGroupIndex: 2},
	}

	if got := treeRenderLineCount(visible, 0, len(visible)); got != 7 {
		t.Fatalf("treeRenderLineCount(full) = %d, want 7", got)
	}
	if got := treeRenderLineCount(visible, 1, 4); got != 4 {
		t.Fatalf("treeRenderLineCount(slice) = %d, want 4", got)
	}
}

func TestTreeWindowRangeByLinesAvoidsSingleRowCollapse(t *testing.T) {
	visible := make([]*TreeNode, 0, 21)
	for i := range 21 {
		visible = append(visible, &TreeNode{TopGroupIndex: i})
	}

	start, end := treeWindowRangeByLines(visible, 10, 10)
	if start > 10 || end <= 10 {
		t.Fatalf("window [%d,%d) does not include cursor 10", start, end)
	}
	if got := treeRenderLineCount(visible, start, end); got > 10 {
		t.Fatalf("window line count = %d, exceeds max 10", got)
	}
	if rows := end - start; rows <= 1 {
		t.Fatalf("window rows = %d, expected multiple rows", rows)
	}
}

func TestProjectTreeAllViewRendersMultipleRowsWithManyTopGroups(t *testing.T) {
	tasks := make([]*models.Task, 0, 25)
	sections := []string{
		models.SectionNext,
		models.SectionInbox,
		models.SectionWaiting,
		models.SectionSomeday,
		models.SectionReview,
		models.SectionStandby,
	}
	for i := 1; i <= 25; i++ {
		task := newTestProjectTreeTask(t, fmt.Sprintf("Task %02d", i))
		task.SetSection(sections[(i-1)%len(sections)])
		tasks = append(tasks, task)
	}

	project := newTestProjectTreeProject(t, "Layout Test", tasks)
	project.BuildTaskTree()

	m := NewProjectTreeModel(project, newTestProjectTreeConfig(), nil)
	m.viewMode = ViewModeAll
	m.updateVisibility()
	m.SetSize(120, 24)

	output := m.View().Content
	seen := 0
	for i := 1; i <= 5; i++ {
		if strings.Contains(output, fmt.Sprintf("Task %02d", i)) {
			seen++
		}
	}
	if seen < 2 {
		t.Fatalf("expected multiple rendered task rows, saw %d task titles in output", seen)
	}
}

func TestBuildProjectTreePrefix(t *testing.T) {
	tests := []struct {
		level int
		want  string
	}{
		{level: -1, want: ""},
		{level: 0, want: ""},
		{level: 1, want: "  "},
		{level: 2, want: "    "},
		{level: 3, want: "      "},
	}

	for _, tt := range tests {
		if got := buildProjectTreePrefix(tt.level); got != tt.want {
			t.Fatalf("buildProjectTreePrefix(%d) = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestProjectTreeBuildTreeUsesDepthOnlyPrefixes(t *testing.T) {
	parent := &models.Task{
		ID:     "parent",
		Title:  "Parent",
		Type:   models.TaskTypeEpic,
		Status: models.StatusTodo,
	}
	child := &models.Task{
		ID:       "child",
		Title:    "Child",
		Type:     models.TaskTypeStory,
		Status:   models.StatusTodo,
		ParentID: &parent.ID,
		Parent:   parent,
	}
	grandchild := &models.Task{
		ID:       "grandchild",
		Title:    "Grandchild",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		ParentID: &child.ID,
		Parent:   child,
	}
	sibling := &models.Task{
		ID:       "sibling",
		Title:    "Sibling",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		ParentID: &parent.ID,
		Parent:   parent,
	}

	child.Children = []*models.Task{grandchild}
	parent.Children = []*models.Task{child, sibling}

	project := &models.Project{
		Name:  "Depth Prefix Test",
		Tasks: []*models.Task{parent, child, grandchild, sibling},
	}

	model := &ProjectTreeModel{project: project}
	model.buildTree()

	want := map[string]string{
		"parent":     "",
		"child":      "  ",
		"grandchild": "    ",
		"sibling":    "  ",
	}

	if len(model.nodes) != len(want) {
		t.Fatalf("expected %d nodes, got %d", len(want), len(model.nodes))
	}

	for _, node := range model.nodes {
		expectedPrefix, ok := want[node.Task.ID]
		if !ok {
			t.Fatalf("unexpected task in tree: %s", node.Task.ID)
		}
		if node.TreePrefix != expectedPrefix {
			t.Fatalf("task %s prefix = %q, want %q", node.Task.ID, node.TreePrefix, expectedPrefix)
		}
		if strings.Contains(node.TreePrefix, "├──") || strings.Contains(node.TreePrefix, "└──") || strings.Contains(node.TreePrefix, "│") {
			t.Fatalf("task %s prefix should be depth-only indent, got %q", node.Task.ID, node.TreePrefix)
		}
	}
}

func TestProjectTreeRenderUsesChevronsWithoutOutlinerConnectors(t *testing.T) {
	parent := &models.Task{
		ID:     "parent",
		Title:  "Parent",
		Type:   models.TaskTypeEpic,
		Status: models.StatusTodo,
	}
	child := &models.Task{
		ID:       "child",
		Title:    "Child",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		ParentID: &parent.ID,
		Parent:   parent,
	}
	parent.Children = []*models.Task{child}

	project := &models.Project{
		Name:  "Render Prefix Test",
		Tasks: []*models.Task{parent, child},
	}

	model := NewProjectTreeModel(project, newTestProjectTreeConfig(), nil)
	model.SetSize(100, 20)

	out := model.View().Content
	if !strings.Contains(out, IconExpanded+" ") {
		t.Fatalf("expected parent rows to render expanded chevron, output: %q", out)
	}
	if strings.Contains(out, "├── ") || strings.Contains(out, "└── ") || strings.Contains(out, "│   ") {
		t.Fatalf("expected project tree output without outliner connectors, output: %q", out)
	}
}

func TestOutlinerConnectorPrefixUnaffected(t *testing.T) {
	if got := buildTreePrefix([]bool{false}, true); got != "│   └── " {
		t.Fatalf("buildTreePrefix([]bool{false}, true) = %q, want %q", got, "│   └── ")
	}
}
