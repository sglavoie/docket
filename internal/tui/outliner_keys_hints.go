package tui

import (
	"fmt"
	"maps"
)

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *OutlinerModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	var prefix string
	if m.Dispatcher != nil && m.Dispatcher.HasPendingPrefix() {
		prefix = m.Dispatcher.PendingPrefix()
	} else {
		prefix = m.pendingPrefix
	}
	if prefix == "" {
		return nil
	}
	return &PrefixPopupInfo{
		Prefix: prefix,
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
	currentStage, currentPriority := m.currentTaskPrefixContext()
	hints := CommonTaskActionPrefixHints(m.config.Stages, currentStage, currentPriority)
	hints["t"] = buildOutlinerTaskPrefixHints(m.project == nil)
	hints["b"] = BlockerPrefixHints
	hints["g"] = outlinerNavigationHints(m.project == nil)
	hints["m"] = ProjectTreeMovePrefixHints // override: outliner uses tree moves
	hints["f"] = OutlinerFoldPrefixHints

	activePrefix := m.currentPrefixHint()
	hints["P"] = m.buildOutlinerPerspectiveHints(activePrefix)

	// Add filter hints
	maps.Copy(hints, buildCommonFilterPrefixPopupHints(&m.filter.CommonFilterState, m.config, true, true, true))
	m.addOutlinerFilterHints(hints)

	// Add saved filter sub-prefix hints
	hints["Fv"] = SavedFilterPrefixHints

	m.addOutlinerStatusHints(hints, activePrefix)

	return hints
}

func (m *OutlinerModel) currentTaskPrefixContext() (string, int) {
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) || m.rows[m.cursor].task == nil {
		return "", 0
	}
	task := m.rows[m.cursor].task
	return task.Stage, task.Priority
}

func buildOutlinerTaskPrefixHints(crossProject bool) []PrefixHint {
	if !crossProject {
		return TaskPrefixHints
	}
	return annotatePrefixHints(TaskPrefixHints, func(hint *PrefixHint) {
		if hint.Key == "n" || hint.Key == "N" {
			hint.Disabled = true
		}
	})
}

func outlinerNavigationHints(crossProject bool) []PrefixHint {
	if crossProject {
		return CrossProjectNavigationPrefixHints
	}
	return OutlinerNavigationPrefixHints
}

func (m *OutlinerModel) currentPrefixHint() string {
	if m.Dispatcher != nil && m.Dispatcher.HasPendingPrefix() {
		return m.Dispatcher.PendingPrefix()
	}
	return m.pendingPrefix
}

func (m *OutlinerModel) buildOutlinerPerspectiveHints(activePrefix string) []PrefixHint {
	if activePrefix != "P" {
		return OutlinerPerspectivePrefixHints
	}
	idx := m.activePerspective
	if idx < 0 {
		idx = 1
	}
	activeKey := fmt.Sprintf("%d", idx)
	return annotatePrefixHints(OutlinerPerspectivePrefixHints, func(hint *PrefixHint) {
		if hint.Key == activeKey {
			hint.Active = true
		}
	})
}

func (m *OutlinerModel) addOutlinerFilterHints(hints PrefixHints) {
	fHints, ok := hints["F"]
	if !ok {
		return
	}

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
		PrefixHint{Key: "D", Description: "columns"},
		PrefixHint{Key: "v", Description: "saved filters"},
	)
	if m.project == nil {
		fHints = append(fHints, PrefixHint{
			Key:         "P",
			Description: "projects",
			Detail:      m.outlinerProjectFilterDetail(),
			Active:      len(m.filterProjects) > 0,
		})
	}
	sortPrefixHintsLowercaseFirst(fHints)
	hints["F"] = fHints
}

func (m *OutlinerModel) outlinerProjectFilterDetail() string {
	if len(m.filterProjects) == 0 {
		return "all"
	}
	return fmt.Sprintf("%d selected", len(m.filterProjects))
}

func (m *OutlinerModel) addOutlinerStatusHints(hints PrefixHints, activePrefix string) {
	if activePrefix != "," || len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) || m.rows[m.cursor].task == nil {
		return
	}

	task := m.rows[m.cursor].task
	currentStatus := string(task.Status)
	notRecurring := task.Recurrence == nil
	hints[","] = annotatePrefixHints(StatusPrefixHints, func(hint *PrefixHint) {
		switch hint.Key {
		case "k", "D":
			if notRecurring {
				hint.Disabled = true
			}
		default:
			if statusForHint, ok := StatusForPrefixKey(hint.Key); ok && string(statusForHint) == currentStatus {
				hint.Active = true
			}
		}
	})
}

func annotatePrefixHints(src []PrefixHint, annotate func(*PrefixHint)) []PrefixHint {
	annotated := make([]PrefixHint, len(src))
	copy(annotated, src)
	for i := range annotated {
		annotate(&annotated[i])
	}
	return annotated
}

// HandledKeys returns all keys OutlinerModel can handle.
func (m *OutlinerModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

// GetHelpItems returns help items for the outliner view.
// In cross-project mode (project == nil), returns bindings for ViewCrossProjectOutliner.
func (m *OutlinerModel) GetHelpItems() []HelpItem {
	if m.project == nil {
		return GetKeybindingRegistry().GetForView(ViewCrossProjectOutliner)
	}
	return GetKeybindingRegistry().GetForView(ViewOutliner)
}
