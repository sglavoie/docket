package tui

import (
	"fmt"
	"maps"
)

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *OutlinerModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  m.buildPrefixHints(),
	}
}

// OutlinerNavigationPrefixHints contains "g" prefix hints for the outliner view.
var OutlinerNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to first item"},
	{Key: "h", Description: "history", Detail: "Show task activity log"},
	{Key: "l", Description: "activity", Detail: "Show project recent activity"},
	{Key: "d", Description: "due date", Detail: "Cycle due date filter"},
	{Key: "z", Description: "snoozed", Detail: "Jump to Snoozed perspective"},
}

// CrossProjectNavigationPrefixHints contains "g" prefix hints for the cross-project outliner.
var CrossProjectNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to first item"},
	{Key: "o", Description: "drill down", Detail: "Open task's project outliner"},
	{Key: "d", Description: "due date", Detail: "Cycle due date filter"},
	{Key: "z", Description: "snoozed", Detail: "Jump to Snoozed perspective"},
}

// OutlinerPerspectivePrefixHints provides the hint list for the p-prefix popup overlay.
var OutlinerPerspectivePrefixHints = []PrefixHint{
	{Key: "1", Description: "All"},
	{Key: "2", Description: "Active"},
	{Key: "3", Description: "Next"},
	{Key: "4", Description: "Inbox"},
	{Key: "5", Description: "Standby"},
	{Key: "6", Description: "Waiting"},
	{Key: "7", Description: "Someday"},
	{Key: "8", Description: "Completed"},
	{Key: "9", Description: "Review"},
	{Key: "0", Description: "Snoozed"},
}

// buildPrefixHints returns the hints map for all supported prefix keys.
func (m *OutlinerModel) buildPrefixHints() PrefixHints {
	var currentStage string
	var currentPriority int
	if len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].task != nil {
		currentStage = m.rows[m.cursor].task.Stage
		currentPriority = m.rows[m.cursor].task.Priority
	}
	hints := CommonTaskActionPrefixHints(m.config.Stages, currentStage, currentPriority)
	hints["t"] = TaskPrefixHints
	if m.project == nil {
		annotated := make([]PrefixHint, len(TaskPrefixHints))
		copy(annotated, TaskPrefixHints)
		for i := range annotated {
			if annotated[i].Key == "n" || annotated[i].Key == "N" {
				annotated[i].Disabled = true
			}
		}
		hints["t"] = annotated
	}
	hints["b"] = BlockerPrefixHints
	if m.project == nil {
		hints["g"] = CrossProjectNavigationPrefixHints
	} else {
		hints["g"] = OutlinerNavigationPrefixHints
	}
	hints["m"] = ProjectTreeMovePrefixHints // override: outliner uses tree moves
	hints["z"] = OutlinerFoldPrefixHints

	// Add perspective hints with active annotation
	if m.pendingPrefix == "P" {
		annotated := make([]PrefixHint, len(OutlinerPerspectivePrefixHints))
		copy(annotated, OutlinerPerspectivePrefixHints)
		idx := m.activePerspective
		if idx < 0 {
			idx = 1
		}
		for i := range annotated {
			if annotated[i].Key == fmt.Sprintf("%d", idx) {
				annotated[i].Active = true
			}
		}
		hints["P"] = annotated
	} else {
		hints["P"] = OutlinerPerspectivePrefixHints
	}

	// Add filter hints
	filterHints := buildCommonFilterPrefixPopupHints(&m.filter.CommonFilterState, m.config, true, true)
	maps.Copy(hints, filterHints)

	// Add view-specific filter hints to F prefix
	if fHints, ok := hints["F"]; ok {
		completedDetail := "off"
		if m.activePerspective == PerspectiveCompleted {
			completedDetail = "on"
		}
		actionableDetail := "off"
		if m.filter.Actionable {
			actionableDetail = "on"
		}
		fHints = append(fHints,
			PrefixHint{Key: "c", Description: "completed", Detail: completedDetail, Active: m.activePerspective == PerspectiveCompleted},
			PrefixHint{Key: "r", Description: "actionable", Detail: actionableDetail, Active: m.filter.Actionable},
			PrefixHint{Key: "@", Description: "section", Detail: "selector"},
			PrefixHint{Key: "v", Description: "saved filters"},
		)
		if m.project == nil {
			projDetail := "all"
			if len(m.filterProjects) > 0 {
				projDetail = fmt.Sprintf("%d selected", len(m.filterProjects))
			}
			fHints = append(fHints, PrefixHint{Key: "P", Description: "projects", Detail: projDetail, Active: len(m.filterProjects) > 0})
		}
		sortPrefixHintsLowercaseFirst(fHints)
		hints["F"] = fHints
	}

	// Add saved filter sub-prefix hints
	hints["Fv"] = SavedFilterPrefixHints

	// Annotate current task's status as active in status hints and dim recurrence hints
	if m.pendingPrefix == "," && len(m.rows) > 0 && m.cursor < len(m.rows) && m.rows[m.cursor].task != nil {
		task := m.rows[m.cursor].task
		currentStatus := string(task.Status)
		notRecurring := task.Recurrence == nil
		annotated := make([]PrefixHint, len(StatusPrefixHints))
		copy(annotated, StatusPrefixHints)
		for i := range annotated {
			if annotated[i].Key == "k" || annotated[i].Key == "D" {
				if notRecurring {
					annotated[i].Disabled = true
				}
			} else if statusForHint, ok := StatusForPrefixKey(annotated[i].Key); ok {
				if string(statusForHint) == currentStatus {
					annotated[i].Active = true
				}
			}
		}
		hints[","] = annotated
	}

	return hints
}

// HandledKeys returns all keys OutlinerModel can handle (ViewOutliner set).
// ViewCrossProjectOutliner is a subset handled by the same model; cross-project
// unique keys (go) are excluded here and tested separately.
func (m *OutlinerModel) HandledKeys() []string {
	return concatKeys(
		[]string{"j", "k", "G", "ctrl+u", "ctrl+d"}, // Outliner registry has j/k and ctrl+d/u but not ↑/↓
		// commonPrimaryViewKeys() minus ctrl+o (ctrl+o opens outliner but isn't registered for ViewOutliner)
		[]string{
			"ctrl+p",
			"`", "~", "@",
			"B", "p", "w", "W",
			"i", "I",
			"\\", "{", "}",
			"R", "ctrl+r", "ctrl+l",
			"u",
			"S", "ctrl+.", "ctrl+m",
		},
		statusPrefixKeys(),     // ,s ,d ,b ,S ,o ,x ,r
		priorityPrefixKeys(),   // !1-!5
		dueDatePrefixKeys(),    // dt dm dw dn d0 dd
		snoozePrefixKeys(),     // st sw sm sn s0 sd
		stagePrefixKeys(),      // .1-.9 .0 F.
		linkedTaskPrefixKeys(), // cb cd cs cf cn cp
		blockerPrefixKeySet(),  // bb bm
		taskPrefixKeySet(),     // tn tN tr ts to tp td tt tc ta tl te tm tD
		movePrefixKeys(),       // ma mn mi mw mr ms mK mJ
		foldPrefixKeys(),       // za zo zc zR zM zv zp
		typeChangeKeys(),       // < > H L
		filterSubKeys(),        // Fp FP Ft Fs F# Fa FS Fr Fc F@ Fx Fd Fvs Fvl Fvd
		sortKeys(),             // 1 2 3 4 5 0
		[]string{
			"h", "l",
			"enter", "space", "tab",
			"g", "backspace",
			"n", "N",
			"f",
			"K", "J", "right", "left",
			"o", "e", "E", "D",
			"*",
			"gz", "gh", "gl", "gd", "gX", "gk",
			"A", "C", "T", "v", "a",
			"F", "/",
			"ctrl+f", "ctrl+t", "ctrl+s", "ctrl+a", "ctrl+e", "ctrl+v",
			"shift+left", "shift+right",
			"x",
			// P0-P9 perspective switching
			"P0", "P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9",
		},
	)
}

// GetHelpItems returns help items for the outliner view.
// In cross-project mode (project == nil), returns bindings for ViewCrossProjectOutliner.
func (m *OutlinerModel) GetHelpItems() []HelpItem {
	if m.project == nil {
		return GetKeybindingRegistry().GetForView(ViewCrossProjectOutliner)
	}
	return GetKeybindingRegistry().GetForView(ViewOutliner)
}
