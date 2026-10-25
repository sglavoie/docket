package bridge

import (
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// WidgetSnapshotFormat names the document query.widget_snapshot answers and
// the app copies into the App Group for its widgets and extensions.
const WidgetSnapshotFormat = "tt-widget-snapshot/v1"

const (
	defaultWidgetLimit = 8
	maxWidgetLimit     = 20
)

type widgetSnapshotPayload struct {
	Limit int `json:"limit,omitempty"`
}

// widgetTask is one row a widget may draw and the identities its deep link
// carries.
type widgetTask struct {
	ProjectID   string     `json:"project_id"`
	TaskID      string     `json:"task_id"`
	ShortID     int        `json:"short_id"`
	Title       string     `json:"title"`
	ProjectName string     `json:"project_name"`
	State       string     `json:"state"`
	Priority    int        `json:"priority"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	Overdue     bool       `json:"overdue"`
	Pinned      bool       `json:"pinned"`
}

// widgetList is one view's first rows and how many it holds in all.
type widgetList struct {
	Count int          `json:"count"`
	Tasks []widgetTask `json:"tasks"`
}

// widgetSnapshotResult is what the app's widgets show while the app is not
// running. It is a cache: generated_at is when it was true, and valid_until
// the next local midnight, after which Today and every overdue mark describe
// a day that is over, so a widget must stop presenting it as current.
type widgetSnapshotResult struct {
	Format      string     `json:"format"`
	Workspace   string     `json:"workspace"`
	GeneratedAt time.Time  `json:"generated_at"`
	Day         string     `json:"day"`
	ValidUntil  time.Time  `json:"valid_until"`
	Today       widgetList `json:"today"`
	Inbox       widgetList `json:"inbox"`
}

// queryWidgetSnapshot answers Today and Inbox as the app's own tabs evaluate
// them, in the device's zone, bounded for a widget.
func (b *Bridge) queryWidgetSnapshot(request Request) (any, error) {
	var payload widgetSnapshotPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	limit := payload.Limit
	switch {
	case limit < 0 || limit > maxWidgetLimit:
		return nil, newError(CodeInvalidRequest, "limit must be between 0 and %d", maxWidgetLimit)
	case limit == 0:
		limit = defaultWidgetLimit
	}
	runtime := b.workspace.runtime
	if err := runtime.Refresh(); err != nil {
		return nil, err
	}
	now := b.now().In(time.Local)
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.Local)
	result := widgetSnapshotResult{
		Format:      WidgetSnapshotFormat,
		Workspace:   b.workspace.id,
		GeneratedAt: now.UTC(),
		Day:         now.Format(time.DateOnly),
		ValidUntil:  midnight.UTC(),
	}
	for _, view := range []struct {
		mode models.QueryMode
		into *widgetList
	}{{models.QueryMode("today"), &result.Today}, {models.QueryMode("inbox"), &result.Inbox}} {
		query, err := models.NewQuery(view.mode, "", models.QuerySort{Field: models.QuerySortManual, Ascending: true})
		if err != nil {
			return nil, err
		}
		answer, err := runtime.Query(query)
		if err != nil {
			return nil, err
		}
		view.into.Count = len(answer.Items)
		view.into.Tasks = make([]widgetTask, 0, min(limit, len(answer.Items)))
		for _, item := range answer.Items {
			if len(view.into.Tasks) == limit {
				break
			}
			task := item.Task
			row := widgetTask{
				ProjectID:   item.Project.ID,
				TaskID:      task.ID,
				ShortID:     task.ShortID,
				Title:       task.Title,
				ProjectName: item.Project.Name,
				State:       string(task.WorkflowState),
				Priority:    task.Priority,
				DueDate:     task.DueDate,
				Pinned:      task.Pinned,
			}
			if task.DueDate != nil && !task.IsTerminalState() {
				due := task.DueDate.In(time.Local)
				row.Overdue = time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, time.Local).
					Before(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local))
			}
			view.into.Tasks = append(view.into.Tasks, row)
		}
	}
	return result, nil
}
