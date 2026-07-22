package alerts

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// AlertCategory identifies the type of proactive alert.
type AlertCategory string

const (
	CategoryDailyReview   AlertCategory = "daily_review"
	CategoryWeeklyReview  AlertCategory = "weekly_review"
	CategoryOverdue       AlertCategory = "overdue"
	CategoryDueSoon       AlertCategory = "due_soon"
	CategoryStale         AlertCategory = "stale"
	CategoryInbox         AlertCategory = "inbox"
	CategoryBlocked       AlertCategory = "blocked"
	CategoryProjectHealth AlertCategory = "project_health"
)

// Severity levels for alerts.
const (
	SeverityInfo    = 0
	SeverityWarning = 1
	SeverityUrgent  = 2
)

// Alert represents a single proactive alert.
type Alert struct {
	Category    AlertCategory
	Title       string
	Details     string
	TaskID      string
	ProjectID   string
	ProjectName string // Display label for project-scoped alerts; empty means global.
	Count       int
	Severity    int // 0=info, 1=warning, 2=urgent
}

// AlertGroup groups alerts of the same category for display.
type AlertGroup struct {
	Category AlertCategory
	Label    string // Display string like "Overdue (3)"
	Alerts   []Alert
}

// AlertSnapshot holds the result of a single evaluation pass.
type AlertSnapshot struct {
	Groups      []AlertGroup
	TotalCount  int
	EvaluatedAt time.Time
}

// DismissalKey returns a stable identity string for this alert, used to track
// dismissals across sessions. Task-specific keys include their owning project
// so duplicate task UUIDs in different projects remain independent.
func (a Alert) DismissalKey() string {
	if a.TaskID != "" {
		if a.ProjectID != "" {
			return string(a.Category) + ":" + a.ProjectID + ":" + a.TaskID
		}
		return string(a.Category) + ":" + a.TaskID
	}
	if a.ProjectID != "" {
		return string(a.Category) + ":" + a.ProjectID
	}
	return string(a.Category)
}

const GlobalProjectDisplayName = "Global"

// IsGlobal reports whether the alert is not tied to a specific project.
func (a Alert) IsGlobal() bool {
	return a.ProjectID == "" && a.ProjectName == ""
}

// ProjectDisplayName returns the display label for project/group headers.
func (a Alert) ProjectDisplayName() string {
	if a.ProjectName != "" {
		return a.ProjectName
	}
	if a.ProjectID != "" {
		return a.ProjectID
	}
	return GlobalProjectDisplayName
}

// ProjectData is a lightweight struct holding project metadata and parsed tasks,
// used to avoid importing TUI types into the alerts package.
type ProjectData struct {
	Project *models.Project
	Tasks   []*models.Task
}
