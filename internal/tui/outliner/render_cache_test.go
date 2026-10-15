package outliner

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

func TestSingleProjectRepaintsMatchFreshProjection(t *testing.T) {
	newModel := func() *OutlinerModel {
		project := &models.Project{ID: "p", Slug: "p", Name: "Project", Tasks: []*models.Task{
			{ID: "short", Title: "Short", WorkflowState: models.StateBacklog},
			{ID: "long", Title: "Long", WorkflowState: models.StateBacklog},
		}}
		model := NewOutlinerModel(project, nil, newTestConfig())
		model.SetSize(100, 33)
		return model
	}
	m := newModel()
	checks := []struct {
		name   string
		change func()
	}{
		{"initial", func() {}},
		{"cursor", func() { m.Cursor = 1 }},
		{"peek", func() { m.PeekMode = true }},
		{"resize", func() { m.SetSize(60, 18) }},
		{"mode", func() { m.PrimaryMode = chrome.ModeInbox }},
		{"return mode", func() { m.PrimaryMode = chrome.ModeAll }},
		{"search", func() { m.SetSearchQuery("Short") }},
		{"clear search", func() { m.ClearSearch() }},
		{"mutation", func() { m.Project.Tasks[0].Title = "Changed title"; m.RebuildRows() }},
		{"time invalidation", func() { m.Project.Tasks[0].DeferredUntil = nil; m.Population.Invalidate() }},
		{"append", func() {
			m.Project.Tasks = append(m.Project.Tasks, &models.Task{ID: "new", Title: "New task", WorkflowState: models.StateInbox, CreatedAt: time.Now()})
		}},
		{"replace project", func() {
			copy := *m.Project
			copy.Tasks = []*models.Task{{ID: "replacement", Title: "Replacement", WorkflowState: models.StateInbox}}
			m.Project = &copy
		}},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			m = newModel()
			m.View()
			check.change()
			got := m.View().Content
			m.RebuildRows()
			want := m.View().Content
			if got != want {
				t.Fatalf("cached frame differs from fresh projection\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}
