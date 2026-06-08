package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/planner"
	"github.com/sglavoie/tt/internal/storage"
)

// TestRoadmapDateToCol verifies bar column position math.
func TestRoadmapDateToCol(t *testing.T) {
	axisStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	m := &RoadmapModel{
		zoom: ZoomMonth, // 7 days per column
		axis: TimelineAxis{
			Start:     axisStart,
			UnitDays:  7,
			TotalCols: 26,
		},
	}

	tests := []struct {
		name    string
		date    time.Time
		wantCol int
	}{
		{
			name:    "axis start = column 0",
			date:    axisStart,
			wantCol: 0,
		},
		{
			name:    "7 days later = column 1",
			date:    axisStart.AddDate(0, 0, 7),
			wantCol: 1,
		},
		{
			name:    "14 days later = column 2",
			date:    axisStart.AddDate(0, 0, 14),
			wantCol: 2,
		},
		{
			name:    "3.5 days later = column 0 (truncated)",
			date:    axisStart.AddDate(0, 0, 3),
			wantCol: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.dateToCol(tc.date)
			if got != tc.wantCol {
				t.Errorf("dateToCol(%v) = %d, want %d", tc.date, got, tc.wantCol)
			}
		})
	}
}

// TestRoadmapDateToCol_WeekZoom verifies column math at week zoom (1 day/col).
func TestRoadmapDateToCol_WeekZoom(t *testing.T) {
	axisStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	m := &RoadmapModel{
		zoom: ZoomWeek, // 1 day per column
		axis: TimelineAxis{
			Start:    axisStart,
			UnitDays: 1,
		},
	}

	got := m.dateToCol(axisStart.AddDate(0, 0, 5))
	if got != 5 {
		t.Errorf("expected column 5 at week zoom, got %d", got)
	}
}

// TestRoadmapBuildAxis_TodayCol verifies that today's column is computed correctly.
func TestRoadmapBuildAxis_TodayCol(t *testing.T) {
	m := &RoadmapModel{
		zoom:      ZoomMonth,
		collapsed: make(map[string]bool),
	}
	m.buildAxis()

	now := time.Now()
	expectedTodayCol := int(now.Sub(m.axis.Start).Hours() / 24 / m.axis.UnitDays)
	if m.axis.TodayCol != expectedTodayCol {
		t.Errorf("TodayCol = %d, want %d", m.axis.TodayCol, expectedTodayCol)
	}
}

// TestRoadmapBuildAxis_Labels verifies axis labels are generated for all columns.
func TestRoadmapBuildAxis_Labels(t *testing.T) {
	for _, zoom := range []ZoomLevel{ZoomWeek, ZoomMonth, ZoomQuarter} {
		m := &RoadmapModel{zoom: zoom, collapsed: make(map[string]bool)}
		m.buildAxis()
		if len(m.axis.Labels) != m.axis.TotalCols {
			t.Errorf("zoom %d: labels length %d != TotalCols %d", zoom, len(m.axis.Labels), m.axis.TotalCols)
		}
	}
}

// TestRoadmapCollapse verifies collapsed state is stored and retrieved per task ID.
func TestRoadmapCollapse(t *testing.T) {
	m := &RoadmapModel{
		collapsed: make(map[string]bool),
	}

	taskID := "test-task-id"
	if m.collapsed[taskID] {
		t.Error("task should not be collapsed initially")
	}

	m.collapsed[taskID] = true
	if !m.collapsed[taskID] {
		t.Error("task should be collapsed after setting")
	}
}

func TestRoadmapSelectedRowIgnoresNegativeCursor(t *testing.T) {
	task := &models.Task{ID: "task-1", Title: "Task 1"}
	project := &models.Project{ID: "project-1", Name: "Project 1"}

	tests := []struct {
		name string
		tab  RoadmapTab
	}{
		{name: "timeline", tab: RoadmapTabTimeline},
		{name: "forecasts", tab: RoadmapTabForecasts},
		{name: "critical path", tab: RoadmapTabCriticalPath},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &RoadmapModel{
				activeTab: tc.tab,
				cursor:    -1,
				rows: []TimelineRow{{
					Task:    task,
					Project: project,
				}},
				criticalPath: &planner.CriticalPathResult{
					Path: []*models.Task{task},
				},
			}

			gotTask, gotProject := m.selectedRow()
			if gotTask != nil || gotProject != nil {
				t.Fatalf("selectedRow() = (%v, %v), want nil row for negative cursor", gotTask, gotProject)
			}
		})
	}
}

func TestRoadmapToggleTimelineRowCollapsedIgnoresNegativeCursor(t *testing.T) {
	task := &models.Task{ID: "task-1", Title: "Task 1"}
	m := &RoadmapModel{
		activeTab:  RoadmapTabTimeline,
		cursor:     -1,
		collapsed:  make(map[string]bool),
		rows:       []TimelineRow{{Task: task}},
		labelWidth: 30,
	}

	m.toggleTimelineRowCollapsed()

	if m.collapsed[task.ID] {
		t.Fatal("negative cursor should not toggle a row")
	}
}

func TestRoadmapLoadsArchivedSummariesForVelocity(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "roadmap-summary")
	project, err := models.NewProject("roadmap-summary", "Roadmap Summary", projectPath)
	if err != nil {
		t.Fatalf("new project: %v", err)
	}
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("save project: %v", err)
	}

	completedAt := time.Now().Add(-24 * time.Hour)
	archived, err := models.NewTask("archived-roadmap-task", 1, models.TaskTypeTask, "Archived roadmap task")
	if err != nil {
		t.Fatalf("new archived task: %v", err)
	}
	archived.Status = models.StatusDone
	archived.CompletedAt = &completedAt
	archived.EnsureTaskAttrs().Effort = models.EffortLarge

	archivePath := filepath.Join(project.Path, "completed.md")
	if err := os.WriteFile(archivePath, []byte(storage.WriteArchivedTasks(project.Name, []*models.Task{archived})), storage.FilePermissions); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if err := storage.RebuildArchiveIndex(project); err != nil {
		t.Fatalf("rebuild archive index: %v", err)
	}
	if err := os.Chmod(archivePath, 0); err != nil {
		t.Fatalf("make archive unreadable: %v", err)
	}
	defer func() {
		_ = os.Chmod(archivePath, storage.FilePermissions)
	}()

	cfg := config.NewConfig()
	cfg.Projects = []*models.Project{project}
	model := NewRoadmapModel(cfg)

	if model.velocity < 0.09 || model.velocity > 0.11 {
		t.Fatalf("velocity = %v, want about 0.1 from archived large-effort summary", model.velocity)
	}
	if len(model.scheduled) != 0 {
		t.Fatalf("scheduled %d archived tasks, want archived tasks excluded from roadmap scheduling", len(model.scheduled))
	}
}
