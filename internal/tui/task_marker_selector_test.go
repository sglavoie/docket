package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
)

func TestTagSelectorKeepsSemanticTagsIndependent(t *testing.T) {
	task, err := models.NewGenericTask("task", 1, "Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Tags = []string{"bug", "keep"}
	other, err := models.NewGenericTask("other", 2, "Other")
	if err != nil {
		t.Fatal(err)
	}
	other.Tags = []string{"idea"}
	project := &models.Project{Tasks: []*models.Task{task, other}}

	selector := listsurface.NewTagSelectorModel(task, project, task.Tags)
	selector.SetCursorToID("idea")
	updated, _ := selector.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	selector = updated.(*listsurface.TagSelectorModel)
	selected := selector.SelectedIDs()
	if !slices.Contains(selected, "idea") || !slices.Contains(selected, "keep") || !slices.Contains(selected, "bug") {
		t.Fatalf("selected tags = %v, want bug+idea+keep", selected)
	}
}
