package outliner

import (
	"fmt"
	"slices"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/notes"
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

// Each operation types a query, renders after every key, and deletes it again.
// Unlike the applied-search navigation benchmark above, every key recompiles,
// ranks, and paints the live results. Report ns/key alongside the whole cycle.
func BenchmarkLiveSearchTyping(b *testing.B) {
	for _, count := range []int{100, 1000, 10000} {
		for _, projectCount := range []int{1, 20} {
			for _, withNotes := range []bool{false, true} {
				b.Run(fmt.Sprintf("tasks=%d/projects=%d/notes=%t", count, projectCount, withNotes), func(b *testing.B) {
					projects := make([]*models.Project, projectCount)
					now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
					for i := range projects {
						id := fmt.Sprintf("project-%d", i)
						projects[i] = &models.Project{ID: id, Slug: id, Name: id}
					}
					for i := range count {
						p := projects[i%projectCount]
						task := &models.Task{ID: fmt.Sprintf("task-%d", i), ShortID: i + 1,
							Title: fmt.Sprintf("Review release item %d", i), CreatedAt: now, UpdatedAt: now, StateChangedAt: now}
						if i%2 == 0 {
							task.Title = fmt.Sprintf("Check delivery %d", i)
							task.Description = "Check the release checklist before publishing the documentation."
						}
						task.ApplyWorkflowState(models.StateBacklog)
						p.Tasks = append(p.Tasks, task)
					}
					var index []notes.LocatedNoteRow
					if withNotes {
						for i := range count / 5 {
							p := projects[i%projectCount]
							index = append(index, notes.LocatedNoteRow{
								Location: app.NoteLocation{ProjectID: p.ID}, OwnerLabel: p.Name,
								Note: &models.Note{Title: fmt.Sprintf("Release notes %d", i), Filename: fmt.Sprintf("note-%d.md", i),
									Body: "Review the release checklist and delivery schedule."},
							})
						}
					}
					m := noteTierOutliner(index, projects...)
					m.SetSize(100, 33)
					m.ActivateSearch()
					keys := make([]tea.KeyPressMsg, 0, 14)
					for _, key := range "release" {
						keys = append(keys, tea.KeyPressMsg{Code: key, Text: string(key)})
					}
					for range "release" {
						keys = append(keys, tea.KeyPressMsg{Code: tea.KeyBackspace})
					}
					// Prove the workload reaches both result populations before timing it.
					for _, key := range keys[:7] {
						m.Update(key)
						settleBackgroundSearch(m)
						m.View()
					}
					tasks, noteCount := 0, 0
					for _, row := range m.Rows {
						if row.IsTaskRow() {
							tasks++
						}
						if row.IsNoteRow {
							noteCount++
						}
					}
					if tasks == 0 || (withNotes && noteCount == 0) {
						b.Fatalf("missing live results: tasks=%d notes=%d", tasks, noteCount)
					}
					for _, key := range keys[7:] {
						m.Update(key)
						settleBackgroundSearch(m)
						m.View()
					}
					b.ReportAllocs()
					latencies := make([]int64, 0, b.N*len(keys))
					b.ResetTimer()
					for range b.N {
						for _, key := range keys {
							started := time.Now()
							m.Update(key)
							settleBackgroundSearch(m)
							m.View()
							latencies = append(latencies, time.Since(started).Nanoseconds())
						}
					}
					b.StopTimer()
					reportSearchTailLatency(b, latencies)
					b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(keys)), "ns/key")
					if m.SearchInputValue() != "" {
						b.Fatal("deletion did not return to the empty query")
					}
					benchmarkSearchEscape(b, m, keys[:7])
				})
			}
		}
	}
}

// Record tails from actual input-to-render cycles, not scorer-only timings.
func reportSearchTailLatency(b *testing.B, latencies []int64) {
	slices.Sort(latencies)
	b.ReportMetric(float64(latencies[(len(latencies)*95+99)/100-1]), "p95-ns/key")
	b.ReportMetric(float64(latencies[len(latencies)-1]), "max-ns/key")
}

// Search setup is outside both measured intervals. Escape must restore the
// original projection promptly even after the most expensive query.
func benchmarkSearchEscape(b *testing.B, m *OutlinerModel, typing []tea.KeyPressMsg) {
	b.Helper()
	var slowest time.Duration
	for range b.N {
		m.ActivateSearch()
		for _, key := range typing {
			m.Update(key)
			settleBackgroundSearch(m)
			m.View()
		}
		started := time.Now()
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		m.View()
		slowest = max(slowest, time.Since(started))
		if m.HasSearchProjection() {
			b.Fatal("Escape did not restore the original projection")
		}
	}
	b.ReportMetric(float64(slowest.Nanoseconds()), "escape-ns")
}
