package outliner

import (
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// Exercise the full input-to-frame path, including projection and Peek, rather
// than timing just the fuzzy matcher or Markdown loader.
func BenchmarkSingleProjectFrame(b *testing.B) {
	for _, count := range []int{100, 1000, 10000} {
		for _, mode := range []string{"navigate", "peek", "search"} {
			b.Run(fmt.Sprintf("%d/%s", count, mode), func(b *testing.B) {
				project := &models.Project{ID: "bench", Slug: "bench", Name: "Benchmark"}
				now := time.Now()
				for i := 0; i < count; i++ {
					project.Tasks = append(project.Tasks, &models.Task{ID: fmt.Sprintf("task-%d", i), ShortID: i + 1, Title: fmt.Sprintf("Review release item %d", i), CreatedAt: now, UpdatedAt: now, StateChangedAt: now})
				}
				for _, task := range project.Tasks {
					task.ApplyWorkflowState(models.StateBacklog)
				}
				m := NewOutlinerModel(project, nil, newTestConfig())
				m.SetSize(100, 33)
				m.PrimaryMode = chrome.ModeBacklog
				m.RebuildRows()
				m.Cursor = 1
				if mode == "peek" {
					m.PeekMode = true
				}
				if mode == "search" {
					m.ActivateSearch()
					m.SetSearchQuery("release")
					m.ConfirmSearch()
				}
				m.View()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					key := 'j'
					if i%2 != 0 {
						key = 'k'
					}
					m.Update(tea.KeyPressMsg{Code: key, Text: string(key)})
					m.Cursor = 1 + i%2
					m.View()
				}
			})
		}
	}
}
