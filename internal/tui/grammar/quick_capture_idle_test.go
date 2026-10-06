package grammar

import (
	"testing"
	"time"

	"charm.land/bubbles/v2/cursor"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func TestCaptureSkipsIdleParsingButRefreshesDatesAndWorkspace(t *testing.T) {
	project := &models.Project{ID: "p", Slug: "p"}
	q := NewQuickCaptureInput(project, []*models.Project{project}, "", false)
	q.Input.SetValue("Task @tomorrow\nAnother")
	q.UpdatePreview()
	preview := q.Preview
	q.UpdateInput(cursor.BlinkMsg{})
	if q.Preview != preview {
		t.Fatal("idle blink reparsed draft")
	}
	q.previewMinute = time.Now().Add(-time.Minute).Truncate(time.Minute)
	q.UpdateInput(cursor.BlinkMsg{})
	if q.Preview == preview {
		t.Fatal("minute change did not refresh relative dates")
	}
	preview = q.Preview
	q.UpdateInput(tea.WindowSizeMsg{Width: 100, Height: 30})
	if q.Preview == preview {
		t.Fatal("non-idle update did not refresh")
	}
	q.Input.SetValue("Task %p")
	q.UpdatePreview()
	if q.PreviewErr != "" {
		t.Fatal(q.PreviewErr)
	}
	q.allProjects = nil
	q.Project = nil
	q.UpdateInput(tea.WindowSizeMsg{Width: 100, Height: 30})
	if q.PreviewErr == "" {
		t.Fatal("workspace invalidation retained a removed project")
	}
	q.Input.SetValue("New task")
	q.UpdateInput(tea.KeyPressMsg{Code: '!', Text: "!"})
	if q.Preview == preview {
		t.Fatal("edit did not refresh")
	}
}
