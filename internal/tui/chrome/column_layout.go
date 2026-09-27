package chrome

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// BadgeWidthOpts configures which badge columns ComputeBadgeWidths should
// measure. Consumers explicitly enable each column so the helper can serve
// both single-project (no Project column) and cross-project surfaces.
type BadgeWidthOpts struct {
	// IncludeStatus measures the status badge column width.
	IncludeStatus bool

	// ProjectNameFor, when non-nil, enables the Project column. The function
	// returns the project display name for a task; an empty return skips
	// that task. MaxProjectLen truncates the name (0 → default 12) and the
	// final rendered badge is "[name]".
	ProjectNameFor func(task *models.Task) string
	MaxProjectLen  int
}

// ComputeBadgeWidths walks tasks once and records the max rendered visual
// width per badge column requested by opts. Returns nil when tasks is empty.
func ComputeBadgeWidths(tasks []*models.Task, opts BadgeWidthOpts) *ColumnLayout {
	if len(tasks) == 0 {
		return nil
	}
	maxProjLen := opts.MaxProjectLen
	if maxProjLen <= 0 {
		maxProjLen = 12
	}
	cl := &ColumnLayout{}
	hasTasks := false
	// A badge's width depends only on its value, and a projection holds a
	// handful of priorities and states over thousands of rows, so each distinct
	// value is rendered and measured once per call rather than once per task.
	priorityWidths := make(map[int]int, 4)
	statusWidths := make(map[models.WorkflowState]int, 8)
	projectWidths := make(map[string]int)
	for _, task := range tasks {
		if task == nil {
			continue
		}
		hasTasks = true

		w, ok := priorityWidths[task.Priority]
		if !ok {
			w = lipgloss.Width(RenderPriorityBadge(task.Priority))
			priorityWidths[task.Priority] = w
		}
		if w > 0 {
			cl.HasPriority = true
			if w > cl.PriorityWidth {
				cl.PriorityWidth = w
			}
		}

		if opts.IncludeStatus {
			state := task.State()
			w, ok := statusWidths[state]
			if !ok {
				w = lipgloss.Width(theme.GetStateBadge(state))
				statusWidths[state] = w
			}
			if w > 0 {
				cl.HasStatus = true
				if w > cl.StatusWidth {
					cl.StatusWidth = w
				}
			}
		}

		if opts.ProjectNameFor != nil {
			name := opts.ProjectNameFor(task)
			if name != "" {
				w, ok := projectWidths[name]
				if !ok {
					truncated := textfit.TruncateToVisualWidth(name, maxProjLen)
					w = lipgloss.Width(theme.MutedStyle.Render(fmt.Sprintf("[%s]", truncated)))
					projectWidths[name] = w
				}
				if w > cl.ProjectWidth {
					cl.ProjectWidth = w
				}
			}
		}
	}
	if !hasTasks {
		return nil
	}
	return cl
}
