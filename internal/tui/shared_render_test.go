package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestRenderDescriptionIndicator(t *testing.T) {
	t.Run("empty description returns empty string", func(t *testing.T) {
		task := &models.Task{Description: ""}
		if got := renderDescriptionIndicator(task); got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("non-empty description contains icon", func(t *testing.T) {
		task := &models.Task{Description: "Some details"}
		got := renderDescriptionIndicator(task)
		icon := IconDescription()
		if !strings.Contains(got, icon) {
			t.Errorf("expected output to contain icon %q, got %q", icon, got)
		}
	})
}

func TestRenderCrossProjectTaskLineShowDescription(t *testing.T) {
	proj := &models.Project{ID: "p1", Name: "Proj"}
	icon := IconDescription()

	t.Run("ShowDescription off omits icon even when description set", func(t *testing.T) {
		task := &models.Task{ID: "t1", Title: "Title", Description: "details"}
		got := RenderCrossProjectTaskLine(task, proj, nil, false, 120, 40, CrossProjectTaskLineOpts{
			ShowDescription: false,
		})
		if strings.Contains(got, icon) {
			t.Errorf("expected no description icon when ShowDescription=false, got %q", got)
		}
	})

	t.Run("ShowDescription on with description renders icon", func(t *testing.T) {
		task := &models.Task{ID: "t1", Title: "Title", Description: "details"}
		got := RenderCrossProjectTaskLine(task, proj, nil, false, 120, 40, CrossProjectTaskLineOpts{
			ShowDescription: true,
		})
		if !strings.Contains(got, icon) {
			t.Errorf("expected description icon when ShowDescription=true and task has description, got %q", got)
		}
	})

	t.Run("ShowDescription on without description omits icon", func(t *testing.T) {
		task := &models.Task{ID: "t1", Title: "Title", Description: ""}
		got := RenderCrossProjectTaskLine(task, proj, nil, false, 120, 40, CrossProjectTaskLineOpts{
			ShowDescription: true,
		})
		if strings.Contains(got, icon) {
			t.Errorf("expected no description icon when task has empty description, got %q", got)
		}
	})
}
