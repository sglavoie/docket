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
	for _, task := range tasks {
		if task == nil {
			continue
		}
		hasTasks = true

		badge := RenderPriorityBadge(task.Priority)
		if w := lipgloss.Width(badge); w > 0 {
			cl.HasPriority = true
			if w > cl.PriorityWidth {
				cl.PriorityWidth = w
			}
		}

		if opts.IncludeStatus {
			badge := theme.GetStateBadge(task.State())
			if w := lipgloss.Width(badge); w > 0 {
				cl.HasStatus = true
				if w > cl.StatusWidth {
					cl.StatusWidth = w
				}
			}
		}

		if opts.ProjectNameFor != nil {
			name := opts.ProjectNameFor(task)
			if name != "" {
				truncated := textfit.TruncateToVisualWidth(name, maxProjLen)
				badge := theme.MutedStyle.Render(fmt.Sprintf("[%s]", truncated))
				if w := lipgloss.Width(badge); w > 0 {
					if w > cl.ProjectWidth {
						cl.ProjectWidth = w
					}
				}
			}
		}
	}
	if !hasTasks {
		return nil
	}
	return cl
}
