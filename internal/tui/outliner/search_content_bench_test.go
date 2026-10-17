package outliner

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/notes"
)

// Exercise title, description, comment, and no-match searches over substantial
// prose. Every key rebuilds and renders; setup and correctness checks are untimed.
func BenchmarkLiveSearchContent(b *testing.B) { benchmarkLiveSearchContent(b, false, false) }

// A first query in a fresh session, with descriptions and comments unique to
// each task. This prevents repeated fixture prose from overstating cache wins.
func BenchmarkLiveSearchColdUniqueContent(b *testing.B) { benchmarkLiveSearchContent(b, true, true) }

func benchmarkLiveSearchContent(b *testing.B, cold, unique bool) {
	for _, count := range []int{1000, 10000} {
		for _, query := range []string{"release", "handoff", "zzzzzzz"} {
			b.Run(fmt.Sprintf("tasks=%d/query=%s", count, query), func(b *testing.B) {
				projects := make([]*models.Project, 20)
				for i := range projects {
					id := fmt.Sprintf("project-%d", i)
					projects[i] = &models.Project{ID: id, Slug: id, Name: id}
				}
				prose := strings.Repeat("Investigate the customer portal timeout. Check logs, reproduce the issue, and verify the fix with the team. 確認.\n", 10)
				now := time.Now()
				for i := range count {
					task := &models.Task{ID: fmt.Sprintf("task-%d", i), ShortID: i + 1,
						Title: fmt.Sprintf("Review release item %d", i), Description: prose,
						CreatedAt: now, UpdatedAt: now, StateChangedAt: now,
						WorkflowState: models.StateBacklog,
						Comments:      []*models.Comment{{Text: prose[:len(prose)/2]}, {Text: "Record the handoff after verification. " + prose[:len(prose)/2]}},
					}
					if i%2 == 0 {
						task.Title = fmt.Sprintf("Check delivery %d", i)
						task.Description += "\nPrepare the release."
					}
					if unique {
						task.Description = fmt.Sprintf("Document %d. ", i) + task.Description
						for j, comment := range task.Comments {
							comment.Text = fmt.Sprintf("Comment %d/%d. ", i, j) + comment.Text
						}
					}
					project := projects[i%len(projects)]
					project.Tasks = append(project.Tasks, task)
				}
				var index []notes.LocatedNoteRow
				for i := range count / 5 {
					project := projects[i%len(projects)]
					index = append(index, notes.LocatedNoteRow{
						Location: app.NoteLocation{ProjectID: project.ID}, OwnerLabel: project.Name,
						Note: &models.Note{Title: "Release and handoff notes", Filename: fmt.Sprintf("note-%d.md", i), Body: prose},
					})
				}
				m := noteTierOutliner(index, projects...)
				m.SetSize(100, 33)
				m.ActivateSearch()
				var keys []tea.KeyPressMsg
				for _, key := range query {
					keys = append(keys, tea.KeyPressMsg{Code: key, Text: string(key)})
				}
				for range query {
					keys = append(keys, tea.KeyPressMsg{Code: tea.KeyBackspace})
				}
				for _, key := range keys[:len(query)] {
					m.Update(key)
					settleBackgroundSearch(m)
					m.View()
				}
				if query == "zzzzzzz" {
					if m.Search.drawn != 0 {
						b.Fatal("no-match workload returned rows")
					}
				} else {
					if m.Search.drawn != count+len(index) {
						b.Fatalf("workload lost results: got %d, want %d", m.Search.drawn, count+len(index))
					}
					if query == "handoff" {
						for _, row := range m.Rows {
							if row.IsTaskRow() && row.MatchSource != MatchSourceComment {
								b.Fatal("comment workload matched a different field")
							}
						}
					}
				}
				for _, key := range keys[len(query):] {
					m.Update(key)
					settleBackgroundSearch(m)
					m.View()
				}
				b.ReportAllocs()
				latencies := make([]int64, 0, b.N*len(keys))
				b.ResetTimer()
				for range b.N {
					if cold {
						m.Search.scoreCache = nil
					}
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
				benchmarkSearchEscape(b, m, keys[:len(query)])
			})
		}
	}
}
