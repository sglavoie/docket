package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/planner"
)

// ZoomLevel controls the time resolution of the roadmap Gantt chart.
type ZoomLevel int

const (
	ZoomWeek    ZoomLevel = iota // ~2 months visible, day columns
	ZoomMonth                    // ~6 months visible, week columns
	ZoomQuarter                  // ~1 year visible, month columns
)

// RoadmapTab represents which roadmap tab is active.
type RoadmapTab int

const (
	RoadmapTabTimeline     RoadmapTab = iota // Gantt bar chart
	RoadmapTabCriticalPath                   // Critical path task list
	RoadmapTabForecasts                      // Forecast completion dates
)

var roadmapTabNames = []string{"Timeline", "Critical Path", "Forecasts"}

// TimelineRow represents a single row in the roadmap timeline view.
type TimelineRow struct {
	Task           *models.Task
	Project        *models.Project
	Indent         int     // nesting depth for display
	Progress       float64 // 0.0–1.0 fraction of children complete
	BarStart       int     // column offset for bar start
	BarEnd         int     // column offset for bar end (exclusive)
	IsCritical     bool
	IsBlocked      bool
	Slack          float64 // slack days from critical path analysis
	ForecastDate   time.Time
	Confidence     planner.Confidence
	Collapsed      bool
	ScheduledStart time.Time
	ScheduledEnd   time.Time
}

// TimelineAxis describes the time axis of the roadmap chart.
type TimelineAxis struct {
	Start     time.Time
	End       time.Time
	UnitDays  float64  // days per column unit
	TotalCols int      // total column count for the bar area
	Labels    []string // column label strings (one per unit)
	TodayCol  int      // column index of today (may be negative if before axis start)
}

// ShowRoadmapMsg opens the roadmap view.
type ShowRoadmapMsg struct {
	Project *models.Project // nil = global (all projects)
}

// CloseRoadmapMsg closes the roadmap view.
type CloseRoadmapMsg struct{ PopToAnchorMsg }

// RoadmapSelectMsg navigates to a task from the roadmap view.
type RoadmapSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}
