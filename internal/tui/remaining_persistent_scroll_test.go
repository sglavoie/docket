package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/planner"
)

func makeRemainingScrollTasks(count int) []*models.Task {
	tasks := make([]*models.Task, 0, count)
	for i := range count {
		tasks = append(tasks, &models.Task{
			ID:            fmt.Sprintf("task-%02d", i),
			ShortID:       i + 1,
			Title:         fmt.Sprintf("Task %02d", i),
			Type:          models.TaskTypeTask,
			WorkflowState: models.StateReady,
		})
	}
	return tasks
}

func TestRoadmapPersistentScrollKeepsWindowOnMoveUp(t *testing.T) {
	tasks := makeRemainingScrollTasks(8)
	project := &models.Project{Name: "Roadmap"}

	tests := []struct {
		name  string
		model *RoadmapModel
		view  func(*RoadmapModel)
	}{
		{
			name: "timeline",
			model: &RoadmapModel{
				BaseView:   BaseView{width: 83, height: 10},
				activeTab:  RoadmapTabTimeline,
				axis:       TimelineAxis{TotalCols: 10, Labels: []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}},
				labelWidth: 20,
			},
			view: func(m *RoadmapModel) {
				_ = m.renderTimelineTab(5, roadmapTimelineLayout{
					labelWidth:      20,
					chartWidth:      60,
					annotationWidth: 3,
				})
			},
		},
		{
			name: "critical path",
			model: &RoadmapModel{
				activeTab: RoadmapTabCriticalPath,
				criticalPath: &planner.CriticalPathResult{
					Path:  tasks,
					Slack: map[string]float64{},
				},
			},
			view: func(m *RoadmapModel) { _ = m.renderCriticalPathTab(6) },
		},
		{
			name:  "forecasts",
			model: &RoadmapModel{activeTab: RoadmapTabForecasts},
			view:  func(m *RoadmapModel) { _ = m.renderForecastsTab(6) },
		},
	}

	for _, task := range tasks {
		task.Children = []*models.Task{{ID: task.ID + "-child"}}
	}

	rows := make([]TimelineRow, 0, len(tasks))
	for _, task := range tasks {
		rows = append(rows, TimelineRow{Task: task, Project: project})
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.model
			m.rows = rows
			m.cursor = 5

			tc.view(m)
			start := m.scrollOff
			if start == 0 {
				t.Fatalf("expected roadmap %s to scroll down, offset stayed at 0", tc.name)
			}

			m.cursor = 4
			tc.view(m)
			if m.scrollOff != start {
				t.Fatalf("moving up inside viewport changed first rendered row: offset=%d want %d", m.scrollOff, start)
			}
		})
	}
}

func TestNotesViewPersistentScrollKeepsWindowOnMoveUp(t *testing.T) {
	notes := make([]*models.Note, 0, 8)
	for i := range 8 {
		notes = append(notes, &models.Note{
			Title:    fmt.Sprintf("Note %02d", i),
			Filename: fmt.Sprintf("note-%02d.md", i),
		})
	}

	m := newTestNotesView(notes)
	m.SetSize(80, 10)

	m.cursor = 5
	_ = m.renderListPane()
	start := m.scrollOffset
	if start == 0 {
		t.Fatalf("expected notes list to scroll down, offset stayed at 0")
	}

	m.cursor = 4
	_ = m.renderListPane()
	if m.scrollOffset != start {
		t.Fatalf("moving up inside viewport changed first rendered row: offset=%d want %d", m.scrollOffset, start)
	}
}

func TestNotesSearchPersistentScrollKeepsWindowOnMoveUp(t *testing.T) {
	filtered := make([]notesSearchResult, 0, 8)
	for i := range 8 {
		filtered = append(filtered, notesSearchResult{
			note: &models.Note{
				Title:    fmt.Sprintf("Note %02d", i),
				Filename: fmt.Sprintf("note-%02d.md", i),
			},
		})
	}

	m := &NotesSearchModel{
		filtered:  filtered,
		listWidth: 80,
	}

	m.cursor = 5
	_ = m.renderNoteList(3)
	start := m.scrollOffset
	if start == 0 {
		t.Fatalf("expected notes search to scroll down, offset stayed at 0")
	}

	m.cursor = 4
	_ = m.renderNoteList(3)
	if m.scrollOffset != start {
		t.Fatalf("moving up inside viewport changed first rendered row: offset=%d want %d", m.scrollOffset, start)
	}
}

func TestBlockersPersistentScrollKeepsWindowOnMoveUp(t *testing.T) {
	project := &models.Project{Name: "Blockers"}
	m := &BlockersModel{
		project: project,
		state:   newBlockerViewState(""),
	}
	m.state.setSize(100, 16)
	for i := range 8 {
		m.blockers = append(m.blockers, models.NewBlocker(fmt.Sprintf("Blocker %02d", i)))
	}

	m.state.cursor = 5
	var b strings.Builder
	blockerListView(&m.state, m, m.getVisibleItems(m.state.tabs.Active), &b)
	start := m.state.scrollOffset
	if start == 0 {
		t.Fatalf("expected blockers list to scroll down, offset stayed at 0")
	}

	m.state.cursor = 4
	b.Reset()
	blockerListView(&m.state, m, m.getVisibleItems(m.state.tabs.Active), &b)
	if m.state.scrollOffset != start {
		t.Fatalf("moving up inside viewport changed first rendered row: offset=%d want %d", m.state.scrollOffset, start)
	}
}
