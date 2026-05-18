package tui

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// ComputeTreeColumnLayout computes a ColumnLayout by scanning a slice of visible
// tree nodes. Returns nil when nodes is empty. The caller must guard on width
// (>= MinColumnAlignWidth) before calling.
func ComputeTreeColumnLayout(nodes []*TreeNode, columnConfig *ColumnConfig) *ColumnLayout {
	if len(nodes) == 0 {
		return nil
	}
	cl := &ColumnLayout{}
	for _, node := range nodes {
		task := node.Task

		// Priority (inline, before stage badge)
		if columnConfig.ShouldShowPriority(task.Priority) {
			badge := renderPriorityBadge(task.Priority)
			if w := lipgloss.Width(badge); w > 0 {
				cl.HasPriority = true
				if w > cl.PriorityWidth {
					cl.PriorityWidth = w
				}
			}
		}

		// Short ID (pre-title in the tree format string)
		if columnConfig.IsVisible(ColShortID) {
			cl.HasShortID = true
			if sw := lipgloss.Width(fmt.Sprintf("#%d", task.ShortID)); sw > cl.ShortIDWidth {
				cl.ShortIDWidth = sw
			}
		}
	}
	return cl
}

// ComputeOutlinerColumnLayout computes a ColumnLayout by scanning a slice of
// visible outliner rows. Section headers and separators are skipped. Returns
// nil when no task rows are present.
func ComputeOutlinerColumnLayout(rows []outlinerRow, columnConfig *ColumnConfig) *ColumnLayout {
	if len(rows) == 0 {
		return nil
	}
	cl := &ColumnLayout{}
	hasRows := false
	for _, row := range rows {
		if row.IsSectionHeader || row.IsSeparator || row.task == nil {
			continue
		}
		hasRows = true
		task := row.task

		// Priority (pre-title in outliner)
		if columnConfig.ShouldShowPriority(task.Priority) {
			badge := renderPriorityBadge(task.Priority)
			if w := lipgloss.Width(badge); w > 0 {
				cl.HasPriority = true
				if w > cl.PriorityWidth {
					cl.PriorityWidth = w
				}
			}
		}
	}
	if !hasRows {
		return nil
	}
	return cl
}

// ComputeTaskLineColumnLayout computes a ColumnLayout for TaskLine-based views
// (focus, priority, daily/weekly review, all tasks, etc.).
// hideLowPriority mirrors the HideLowPriority flag on TaskLine.
//
// Equivalent to ComputeBadgeWidths with IncludeStatus/IncludeType/IncludeStage
// all enabled. Callers that need a Project column should call ComputeBadgeWidths
// directly with a ProjectNameFor callback.
func ComputeTaskLineColumnLayout(tasks []*models.Task, hideLowPriority bool) *ColumnLayout {
	return ComputeBadgeWidths(tasks, BadgeWidthOpts{
		HideLowPriority: hideLowPriority,
		IncludeStatus:   true,
		IncludeType:     true,
		IncludeStage:    true,
	})
}

// crossProjectNameLookup returns a ProjectNameFor callback for a slice of
// CrossProjectTask. Tasks not in the slice produce empty strings (skipping
// them from project width measurement).
func crossProjectNameLookup(items []CrossProjectTask) func(task *models.Task) string {
	names := make(map[string]string, len(items))
	for _, it := range items {
		if it.Task != nil && it.Project != nil {
			names[it.Task.ID] = it.Project.Name
		}
	}
	return func(task *models.Task) string {
		if task == nil {
			return ""
		}
		return names[task.ID]
	}
}

// focusTaskNameLookup is the FocusTask counterpart to crossProjectNameLookup.
func focusTaskNameLookup(items []FocusTask) func(task *models.Task) string {
	names := make(map[string]string, len(items))
	for _, it := range items {
		if it.Task != nil && it.Project != nil {
			names[it.Task.ID] = it.Project.Name
		}
	}
	return func(task *models.Task) string {
		if task == nil {
			return ""
		}
		return names[task.ID]
	}
}

// BadgeWidthOpts configures which badge columns ComputeBadgeWidths should
// measure. Consumers explicitly enable each column so the helper can serve
// both single-project (no Project column) and cross-project surfaces.
type BadgeWidthOpts struct {
	// HideLowPriority mirrors TaskLine.HideLowPriority. When true, priorities
	// > 2 do not contribute to HasPriority/PriorityWidth.
	HideLowPriority bool

	// IncludeStatus measures the status badge column width.
	IncludeStatus bool
	// IncludeType measures the type badge column width.
	IncludeType bool
	// IncludeStage measures the stage badge column width.
	IncludeStage bool

	// ProjectNameFor, when non-nil, enables the Project column. The function
	// returns the project display name for a task; an empty return skips
	// that task. MaxProjectLen truncates the name (0 → default 12) and the
	// final rendered badge is "[name]".
	ProjectNameFor func(task *models.Task) string
	MaxProjectLen  int
}

// ComputeBadgeWidths walks tasks once and records the max rendered visual
// width per badge column requested by opts. Returns nil when tasks is empty.
// Session 7 will adopt the returned layout in list-row renderers; Session 6
// only provides the infrastructure.
func ComputeBadgeWidths(tasks []*models.Task, opts BadgeWidthOpts) *ColumnLayout {
	if len(tasks) == 0 {
		return nil
	}
	maxProjLen := opts.MaxProjectLen
	if maxProjLen <= 0 {
		maxProjLen = 12
	}
	cl := &ColumnLayout{}
	for _, task := range tasks {
		if task == nil {
			continue
		}

		showPriority := !opts.HideLowPriority || task.Priority <= 2
		if showPriority {
			badge := renderPriorityBadge(task.Priority)
			if w := lipgloss.Width(badge); w > 0 {
				cl.HasPriority = true
				if w > cl.PriorityWidth {
					cl.PriorityWidth = w
				}
			}
		}

		if opts.IncludeStatus {
			badge := GetStatusBadge(string(task.Status), task.Status.Icon())
			if w := lipgloss.Width(badge); w > 0 {
				cl.HasStatus = true
				if w > cl.StatusWidth {
					cl.StatusWidth = w
				}
			}
		}

		if opts.IncludeType {
			badge := GetTypeBadge(string(task.Type), task.Type.Icon())
			if w := lipgloss.Width(badge); w > 0 {
				cl.HasType = true
				if w > cl.TypeWidth {
					cl.TypeWidth = w
				}
			}
		}

		if opts.IncludeStage {
			badge := GetStageBadge(task.Stage)
			if w := lipgloss.Width(badge); w > 0 {
				cl.HasStage = true
				if w > cl.StageWidth {
					cl.StageWidth = w
				}
			}
		}

		if opts.ProjectNameFor != nil {
			name := opts.ProjectNameFor(task)
			if name != "" {
				truncated := truncateToVisualWidth(name, maxProjLen)
				badge := MutedStyle.Render(fmt.Sprintf("[%s]", truncated))
				if w := lipgloss.Width(badge); w > 0 {
					cl.HasProject = true
					if w > cl.ProjectWidth {
						cl.ProjectWidth = w
					}
				}
			}
		}
	}
	return cl
}
