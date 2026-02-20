package tui

import "fmt"

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
	hints := PrefixHints{
		",": StatusPrefixHints,
		"t": TaskPrefixHints,
		"s": SnoozePrefixHints,
		"!": PriorityPrefixHints,
		"d": DueDatePrefixHints,
		"b": BlockerPrefixHints,
		"c": CreateLinkedPrefixHints,
		"g": OutlinerNavigationPrefixHints,
		"m": ProjectTreeMovePrefixHints,
		"z": KanbanFoldPrefixHints,
	}

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
	filterHints := buildCommonFilterPrefixPopupHints(&m.filter, m.config, true, true)
	for k, v := range filterHints {
		hints[k] = v
	}

	// Add view-specific filter hints to F prefix
	if fHints, ok := hints["F"]; ok {
		completedDetail := "off"
		if m.activePerspective == 8 {
			completedDetail = "on"
		}
		actionableDetail := "off"
		if m.actionableFilter {
			actionableDetail = "on"
		}
		fHints = append(fHints,
			PrefixHint{Key: "c", Description: "completed", Detail: completedDetail, Active: m.activePerspective == 8},
			PrefixHint{Key: "r", Description: "actionable", Detail: actionableDetail, Active: m.actionableFilter},
			PrefixHint{Key: "@", Description: "section", Detail: "selector"},
		)
		hints["F"] = fHints
	}

	// Annotate current task's status as active in status hints
	if m.pendingPrefix == "," && len(m.rows) > 0 && m.cursor < len(m.rows) {
		task := m.rows[m.cursor].task
		currentStatus := string(task.Status)
		annotated := make([]PrefixHint, len(StatusPrefixHints))
		copy(annotated, StatusPrefixHints)
		for i := range annotated {
			if statusForHint, ok := StatusForPrefixKey(annotated[i].Key); ok {
				if string(statusForHint) == currentStatus {
					annotated[i].Active = true
				}
			}
		}
		hints[","] = annotated
	}

	return hints
}

// GetHelpItems returns help items for the outliner view.
func (m *OutlinerModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewOutliner)
}
