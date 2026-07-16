package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestOutlinerAllGroupsEachTaskByOwnSection(t *testing.T) {
	parent := &models.Task{ID: "parent", Title: "Epic", Type: models.TaskTypeEpic, WorkflowState: models.StateNext}
	child := &models.Task{ID: "child", Title: "Story", Type: models.TaskTypeStory, WorkflowState: models.StateInbox, ParentID: &parent.ID}
	grandchild := &models.Task{ID: "grandchild", Title: "Task", Type: models.TaskTypeTask, WorkflowState: models.StateInbox, ParentID: &child.ID}
	project := &models.Project{ID: "p", Name: "Project", Tasks: []*models.Task{parent, child, grandchild}}
	project.BuildTaskTree()

	m := minimalOutliner()
	m.project = project
	m.activePerspective = 1
	m.rebuildRows()

	seen := map[string]int{}
	section := ""
	for _, row := range m.rows {
		if row.IsSectionHeader {
			section = row.SectionName
			continue
		}
		if row.task == nil {
			continue
		}
		seen[row.task.ID]++
		switch row.task.ID {
		case parent.ID:
			if section != models.SectionNext {
				t.Errorf("parent rendered in %q, want Next", section)
			}
		case child.ID:
			if section != models.SectionInbox {
				t.Errorf("child rendered in %q, want Inbox", section)
			}
			if !strings.Contains(row.Breadcrumb, parent.Title) {
				t.Errorf("child breadcrumb = %q, want parent title", row.Breadcrumb)
			}
		case grandchild.ID:
			if section != models.SectionInbox || row.Breadcrumb != "" {
				t.Errorf("same-section grandchild section/breadcrumb = %q/%q", section, row.Breadcrumb)
			}
		}
	}
	for _, task := range project.Tasks {
		if seen[task.ID] != 1 {
			t.Errorf("task %s rendered %d times, want once", task.ID, seen[task.ID])
		}
	}

	// A fold in Next must not hide the Inbox subtree.
	m.expandedNodes[parent.ID] = false
	m.rebuildRows()
	if findTaskRow(m.rows, child.ID) < 0 || findTaskRow(m.rows, grandchild.ID) < 0 {
		t.Fatal("cross-section Inbox descendants disappeared when Next parent was folded")
	}
}

func findTaskRow(rows []outlinerRow, taskID string) int {
	for i, row := range rows {
		if row.task != nil && row.task.ID == taskID {
			return i
		}
	}
	return -1
}
