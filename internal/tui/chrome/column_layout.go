package chrome

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// ColumnLayout holds pre-computed fixed-width column measurements for a single
// render pass. When non-nil, badge columns are padded to fixed widths so
// metadata aligns vertically across all rows. Nil = current variable behavior.
type ColumnLayout struct {
	PriorityWidth int // 1 when any row shows priority badge; 0 if all hidden
	StatusWidth   int // max rendered status badge visual width; 0 when column inactive
	ProjectWidth  int // max rendered project and context label width; 0 when not in use
	HasPriority   bool
	HasStatus     bool
	// HasFold reports that some row of the projection draws a fold glyph, so
	// every other task row reserves that column blank and the titles share one.
	// ComputeBadgeWidths never sets it: a fold is a fact about a row, not about
	// a task, so the caller walking the rows does.
	HasFold bool
}

// BadgeWidthOpts configures which badge columns ComputeBadgeWidths should
// measure. Consumers explicitly enable each column so the helper can serve
// both single-project (no Project column) and cross-project surfaces.
type BadgeWidthOpts struct {
	// IncludeStatus measures the status badge column width.
	IncludeStatus bool

	// ProjectNameFor, when non-nil, enables the Project column. The function
	// returns the project display name for a task; an empty return skips
	// that task. MaxProjectLen truncates the name and the final rendered badge
	// is "[name]"; the caller owns that cap, so there is no default here.
	ProjectNameFor func(task *models.Task) string
	MaxProjectLen  int
}

// ComputeBadgeWidths walks tasks once and records the max rendered visual
// width per badge column requested by opts. Returns nil when tasks is empty.
func ComputeBadgeWidths(tasks []*models.Task, opts BadgeWidthOpts) *ColumnLayout {
	if len(tasks) == 0 {
		return nil
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
					truncated := textfit.TruncateToVisualWidth(name, opts.MaxProjectLen)
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
