package outliner

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/notes"
)

func backgroundSearchFixture(count int) *OutlinerModel {
	project := &models.Project{ID: "p", Slug: "p", Name: "Project"}
	for i := range count {
		task := tierTask()
		task.ID = fmt.Sprintf("task-%d", i)
		task.ShortID = i + 1
		task.Title = fmt.Sprintf("Task %d", i)
		task.Description = strings.Repeat("ordinary prose ", 80) + "release"
		task.Comments = []*models.Comment{{Text: "handoff"}}
		project.Tasks = append(project.Tasks, task)
	}
	m := noteTierOutliner([]notes.LocatedNoteRow{}, project)
	m.SetSize(100, 33)
	m.ActivateSearch()
	return m
}

func TestBackgroundSearchMatchesSynchronousRules(t *testing.T) {
	for _, text := range []string{"release", "handoff", "RELEASE", "zzzzzzz", "#missing release", "-#missing release", "?leaf", "%p:12"} {
		t.Run(text, func(t *testing.T) {
			m := backgroundSearchFixture(1000)
			m.BaseSearchableView.SetSearchQuery(text)
			expected := backgroundSearchFixture(1000)
			expected.BaseSearchableView.SetSearchQuery(text)
			expected.Rows = nil
			expected.buildSearchProjectionRows()
			cmd := m.rebuildLiveSearch()
			if cmd == nil || m.Search.flight == nil {
				t.Fatal("large search ran synchronously")
			}
			if !strings.Contains(m.View().Content, "Searching") {
				t.Fatal("pending search presented itself as no matches")
			}
			m.Update(cmd())
			if len(m.Rows) != len(expected.Rows) {
				t.Fatalf("rows %d, want %d", len(m.Rows), len(expected.Rows))
			}
			for i, row := range m.Rows {
				want := expected.Rows[i]
				if row.Task.ID != want.Task.ID || row.MatchSource != want.MatchSource {
					t.Fatalf("row %d differs", i)
				}
				if row.Task != m.Population.Projects[0].GetTaskByID(row.Task.ID) {
					t.Fatal("row kept a worker task")
				}
			}
			if m.Search.caseRelaxed != expected.Search.caseRelaxed || m.Search.withheld != expected.Search.withheld || m.Search.eliminationHint != expected.Search.eliminationHint {
				t.Fatal("search metadata differs")
			}
		})
	}
}

func TestBackgroundSearchSupersessionEscapeAndMutation(t *testing.T) {
	m := backgroundSearchFixture(1000)
	m.BaseSearchableView.SetSearchQuery("release")
	old := m.rebuildLiveSearch()
	oldFlight := m.Search.flight
	// Produce a result but deliver it only after a newer query.
	stale := old()
	m.BaseSearchableView.SetSearchQuery("handoff")
	current := m.rebuildLiveSearch()
	m.Update(stale)
	if m.Search.flight == oldFlight || m.Search.flight.result != nil {
		t.Fatal("old query published")
	}
	m.Update(current())
	if m.Search.drawn != 1000 {
		t.Fatal("latest query did not publish")
	}
	m.BaseSearchableView.SetSearchQuery("release")
	escaped := m.rebuildLiveSearch()
	late := escaped()
	m.ClearSearch()
	rows := append([]OutlinerRow(nil), m.Rows...)
	m.Update(late)
	if m.HasSearchProjection() || !reflect.DeepEqual(rows, m.Rows) {
		t.Fatal("late result undid Escape")
	}

	m.ActivateSearch()
	m.BaseSearchableView.SetSearchQuery("release")
	invalidated := m.rebuildLiveSearch()
	late = invalidated()
	m.Population.Projects[0].Tasks[0].Description = "changed"
	m.Population.Invalidate()
	m.Update(late)
	m.View()
	if m.Search.drawn != 999 {
		t.Fatalf("stale snapshot applied: %d", m.Search.drawn)
	}
}

func TestBackgroundSearchDetachesAndCancelsWithoutBlockingInput(t *testing.T) {
	m := backgroundSearchFixture(1000)
	m.BaseSearchableView.SetSearchQuery("handoff")
	cmd := m.rebuildLiveSearch()
	// The running command reads no live pointers, including comments.
	result := make(chan tea.Msg, 1)
	go func() { result <- cmd() }()
	for _, task := range m.Population.Projects[0].Tasks {
		task.Comments[0].Text = "edited"
	}
	m.RebuildRows()
	m.Update(<-result)
	if m.Search.drawn != 0 {
		t.Fatal("detached stale comments were published")
	}
	m.BaseSearchableView.SetSearchQuery("release")
	canceled := m.rebuildLiveSearch()
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if msg := canceled(); msg != nil {
		t.Fatal("canceled scoring ran")
	}
	if m.HasSearchProjection() {
		t.Fatal("Escape waited for search")
	}
}

// Benchmarks wait for the same command production runs, without executing the
// unrelated text cursor's blink timers. This measures settled results, not
// merely the cheap frame which says Searching.
func settleBackgroundSearch(m *OutlinerModel) {
	if flight := m.Search.flight; flight != nil && flight.result == nil {
		m.Update(flight.run())
	}
}

// Deliver a burst while scoring is actually running; Escape is measured before
// waiting for workers. A queued query must never hold input behind its work.
func BenchmarkLiveSearchResponsive(b *testing.B) {
	m := backgroundSearchFixture(10000)
	prose := strings.Repeat("Investigate the customer portal timeout. Check logs, reproduce the issue, and verify the fix with the team. 確認.\n", 10)
	for i, task := range m.Population.Projects[0].Tasks {
		task.Description = fmt.Sprintf("Document %d. %s", i, prose)
		task.Comments = []*models.Comment{{Text: fmt.Sprintf("Comment %d. Record the handoff. %s", i, prose)}}
	}
	var latencies []int64
	var escape time.Duration
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		m.ActivateSearch()
		m.Search.scoreCache = nil
		done := make(chan tea.Msg, 7)
		jobs := 0
		b.StartTimer()
		for _, key := range "handoff" {
			started := time.Now()
			m.Update(tea.KeyPressMsg{Code: key, Text: string(key)})
			if flight := m.Search.flight; flight != nil && flight.result == nil {
				run := flight.run
				jobs++
				go func() { done <- run() }()
			}
			m.View()
			latencies = append(latencies, time.Since(started).Nanoseconds())
		}
		started := time.Now()
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		m.View()
		escape = max(escape, time.Since(started))
		b.StopTimer()
		if m.HasSearchProjection() {
			b.Fatal("Escape did not restore the view")
		}
		for range jobs {
			m.Update(<-done)
		}
		if m.HasSearchProjection() {
			b.Fatal("late result reinstated search")
		}
		b.StartTimer()
	}
	b.StopTimer()
	reportSearchTailLatency(b, latencies)
	var total int64
	for _, elapsed := range latencies {
		total += elapsed
	}
	b.ReportMetric(float64(total)/float64(len(latencies)), "ns/key")
	b.ReportMetric(float64(escape.Nanoseconds()), "escape-ns")
}

func TestBackgroundSearchInputDoesNotWaitForWorker(t *testing.T) {
	m := backgroundSearchFixture(1000)
	m.Search.worker = &sync.Mutex{}
	m.Search.worker.Lock()
	defer m.Search.worker.Unlock()
	m.BaseSearchableView.SetSearchQuery("release")
	cmd := m.rebuildLiveSearch()
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if m.SearchQuery() != "x" {
		t.Fatal("typing did not advance while the worker was occupied")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.HasSearchProjection() {
		t.Fatal("Escape waited for the occupied worker")
	}
	select {
	case <-done:
		t.Fatal("worker unexpectedly completed behind the locked gate")
	default:
	}
}

func TestBackgroundSearchReplacesNoteSnapshot(t *testing.T) {
	m := backgroundSearchFixture(1000)
	index := []notes.LocatedNoteRow{{Note: &models.Note{Filename: "note.md", Title: "old note", Body: "release"}}}
	m.SetNoteSearchIndex(func() []notes.LocatedNoteRow { return index })
	m.BaseSearchableView.SetSearchQuery("release")
	m.Update(m.rebuildLiveSearch()())
	if m.Search.drawn != 1001 {
		t.Fatalf("initial matches = %d", m.Search.drawn)
	}
	index[0].Note.Body = "changed"
	m.SetNoteSearchIndex(func() []notes.LocatedNoteRow { return index })
	m.Update(m.rebuildLiveSearch()())
	if m.Search.drawn != 1000 {
		t.Fatalf("replacement retained old note: %d", m.Search.drawn)
	}
}
