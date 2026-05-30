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
