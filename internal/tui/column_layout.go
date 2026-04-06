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
			cl.HasPriority = true
			cl.PriorityWidth = 1
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
			cl.HasPriority = true
			cl.PriorityWidth = 1
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
func ComputeTaskLineColumnLayout(tasks []*models.Task, hideLowPriority bool) *ColumnLayout {
	if len(tasks) == 0 {
		return nil
	}
	cl := &ColumnLayout{}
	for _, task := range tasks {
		if task == nil {
			continue
		}

		// Priority
		showPriority := !hideLowPriority || task.Priority <= 2
		if showPriority {
			cl.HasPriority = true
			cl.PriorityWidth = 1
		}
	}
	return cl
}
