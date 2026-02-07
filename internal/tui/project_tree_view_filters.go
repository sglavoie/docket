package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// renderViewModeIndicator returns a styled indicator for the current view mode with counts.
func (m *ProjectTreeModel) renderViewModeIndicator() string {
	// Count by section: active, next, inbox, waiting, someday, completed
	var activeCount, nextCount, inboxCount, waitingCount, somedayCount, reviewCount, completedCount, snoozedCount int
	for _, task := range m.project.Tasks {
		if task.IsSnoozed() {
			snoozedCount++
			continue
		}
		if task.IsComplete() {
			completedCount++
			continue
		}
		switch task.Section {
		case models.SectionNext:
			nextCount++
		case models.SectionInbox:
			inboxCount++
		case models.SectionWaiting:
			waitingCount++
		case models.SectionSomeday:
			somedayCount++
		case models.SectionReview:
			reviewCount++
		default:
			// Empty section = Active
			activeCount++
		}
	}

	// Build indicator for each view with counts
	var parts []string

	// Active view
	activeLabel := fmt.Sprintf("Active[%d]", activeCount)
	if m.viewMode == ViewModeActive {
		parts = append(parts, ViewModeActiveStyle.Render(activeLabel))
	} else {
		parts = append(parts, MutedStyle.Render(activeLabel))
	}

	// Next view
	nextLabel := fmt.Sprintf("Next[%d]", nextCount)
	if m.viewMode == ViewModeNext {
		parts = append(parts, ViewModeNextStyle.Render(nextLabel))
	} else {
		parts = append(parts, MutedStyle.Render(nextLabel))
	}

	// Inbox view
	inboxLabel := fmt.Sprintf("Inbox[%d]", inboxCount)
	if m.viewMode == ViewModeInbox {
		parts = append(parts, ViewModeInboxStyle.Render(inboxLabel))
	} else {
		parts = append(parts, MutedStyle.Render(inboxLabel))
	}

	// Waiting view
	waitingLabel := fmt.Sprintf("Waiting[%d]", waitingCount)
	if m.viewMode == ViewModeWaiting {
		parts = append(parts, ViewModeWaitingStyle.Render(waitingLabel))
	} else {
		parts = append(parts, MutedStyle.Render(waitingLabel))
	}

	// Someday view
	somedayLabel := fmt.Sprintf("Someday[%d]", somedayCount)
	if m.viewMode == ViewModeSomeday {
		parts = append(parts, ViewModeSomedayStyle.Render(somedayLabel))
	} else {
		parts = append(parts, MutedStyle.Render(somedayLabel))
	}

	// Review view
	reviewLabel := fmt.Sprintf("Review[%d]", reviewCount)
	if m.viewMode == ViewModeReview {
		parts = append(parts, ViewModeReviewStyle.Render(reviewLabel))
	} else {
		parts = append(parts, MutedStyle.Render(reviewLabel))
	}

	// Snoozed indicator (only show if there are snoozed tasks)
	if snoozedCount > 0 {
		snoozedLabel := fmt.Sprintf("Snoozed[%d]", snoozedCount)
		if m.viewMode == ViewModeSnoozed {
			parts = append(parts, ViewModeSnoozedStyle.Render(snoozedLabel))
		} else {
			parts = append(parts, MutedStyle.Render(snoozedLabel))
		}
	}

	// All view indicator
	allCount := activeCount + nextCount + inboxCount + waitingCount + somedayCount + reviewCount
	allLabel := fmt.Sprintf("All[%d]", allCount)
	if m.viewMode == ViewModeAll {
		parts = append(parts, ViewModeAllStyle.Render(allLabel))
	} else {
		parts = append(parts, MutedStyle.Render(allLabel))
	}

	// Blockers indicator (only show if there are active blockers)
	blockerCount := storage.GetActiveBlockerCount(m.project)
	if blockerCount > 0 {
		blockerLabel := fmt.Sprintf("🚧%d", blockerCount)
		parts = append(parts, DueSoonStyle.Render(blockerLabel))
	}

	return strings.Join(parts, " ")
}

func (m *ProjectTreeModel) renderFilterBar() string {
	var result strings.Builder

	// Active filter indicator line (only shown when filters are active)
	if indicator := m.renderActiveFilterIndicator(); indicator != "" {
		result.WriteString(indicator)
		result.WriteString("\n")
	}

	// View mode indicator (always shown)
	result.WriteString(m.renderViewModeIndicator())
	result.WriteString("\n")

	return result.String()
}

// hasActiveFilters returns true if any filter is currently active.
func (m *ProjectTreeModel) hasActiveFilters() bool {
	return m.activeSavedView != "" ||
		m.viewMode == ViewModeCompleted ||
		m.filter.Priority != 0 ||
		m.filter.Type != "" ||
		m.filter.Status != "" ||
		len(m.filter.Tags) > 0 ||
		len(m.filter.Sections) > 0 ||
		m.filter.DueDate != DueDateAll ||
		m.filter.Actionable ||
		len(m.filter.Attrs) > 0 ||
		m.filter.ScoreMin > 0
}

// renderActiveFilterIndicator returns a compact indicator of all active filters.
// Returns empty string when no filters are active.
func (m *ProjectTreeModel) renderActiveFilterIndicator() string {
	var parts []string

	// Show saved view badge if active
	if m.activeSavedView != "" {
		parts = append(parts, SavedViewBadgeStyle.Render("View: "+m.activeSavedView))
	}

	if m.viewMode == ViewModeCompleted {
		parts = append(parts, "completed")
	}
	if m.filter.Priority == 1 {
		parts = append(parts, "!Hi")
	}
	if m.filter.Type != "" {
		parts = append(parts, "type:"+string(m.filter.Type))
	}
	if m.filter.Status != "" {
		parts = append(parts, "status:"+string(m.filter.Status))
	}
	if len(m.filter.Tags) > 0 {
		var tagNames []string
		for tag := range m.filter.Tags {
			tagNames = append(tagNames, "#"+tag)
		}
		parts = append(parts, strings.Join(tagNames, " "))
	}
	if len(m.filter.Sections) > 0 {
		var secNames []string
		for sec := range m.filter.Sections {
			secNames = append(secNames, "@"+sec)
		}
		parts = append(parts, strings.Join(secNames, " "))
	}
	switch m.filter.DueDate {
	case DueDateOverdue:
		parts = append(parts, "due:overdue")
	case DueDateToday:
		parts = append(parts, "due:today")
	case DueDateThisWeek:
		parts = append(parts, "due:week")
	case DueDateNone:
		parts = append(parts, "due:none")
	}
	if m.filter.Actionable {
		parts = append(parts, "ready")
	}
	if len(m.filter.Attrs) > 0 {
		var attrParts []string
		for k, v := range m.filter.Attrs {
			attrParts = append(attrParts, k+"="+v)
		}
		parts = append(parts, "attr:"+strings.Join(attrParts, ","))
	}
	if m.filter.ScoreMin > 0 {
		parts = append(parts, fmt.Sprintf("score>=%.0f", m.filter.ScoreMin))
	}

	if len(parts) == 0 {
		return ""
	}

	return FilterBadgeStyle.Render("Filters: " + strings.Join(parts, " "))
}

// renderChildrenInOtherSectionsBadge returns a badge showing how many children
// are in sections hidden by the current view mode.
func (m *ProjectTreeModel) renderChildrenInOtherSectionsBadge(task *models.Task) string {
	hiddenSections := m.getHiddenSectionsForViewMode()
	if len(hiddenSections) == 0 {
		return ""
	}

	counts, ok := m.filterBar.GetChildrenInOtherSections()[task.ID]
	if !ok || len(counts) == 0 {
		return ""
	}

	var parts []string
	for _, section := range hiddenSections {
		if count, exists := counts[section]; exists && count > 0 {
			parts = append(parts, fmt.Sprintf("+%d %s", count, section))
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return ChildrenBadgeStyle.Render("[" + strings.Join(parts, ", ") + "]")
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *ProjectTreeModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints: PrefixHints{
			",":   StatusPrefixHints,
			"z":   FoldPrefixHints,
			"g":   ProjectTreeNavigationPrefixHints,
			"m":   ProjectTreeMovePrefixHints,
			"!":   PriorityPrefixHints,
			"d":   DueDatePrefixHints,
			"s":   SnoozePrefixHints,
			"c":   CreateLinkedPrefixHints,
			"t":   TaskPrefixHints,
			"b":   BlockerPrefixHints,
			"F":   m.buildFilterPrefixHints(),
			"Ft":  FilterTypePrefixHints,
			"Fs":  FilterStatusPrefixHints,
			"Fd":  FilterDuePrefixHints,
			"Fa":  FilterAttrPrefixHints,
			"Fas": FilterAttrSeverityPrefixHints,
			"Far": FilterAttrReproduciblePrefixHints,
			"Fae": FilterAttrEffortPrefixHints,
			"Fac": FilterAttrCategoryPrefixHints,
			"Fav": FilterAttrValuePrefixHints,
			"Faw": FilterAttrWorktypePrefixHints,
			"Fao": FilterAttrScopePrefixHints,
			"Fap": FilterAttrPointsPrefixHints,
			"FS":  BuildFilterScorePrefixHints(m.config),
			"Fv":  SavedViewPrefixHints,
		},
	}
}

// buildFilterPrefixHints builds dynamic filter hints showing current filter state.
func (m *ProjectTreeModel) buildFilterPrefixHints() []PrefixHint {
	completedDetail := "off"
	if m.viewMode == ViewModeCompleted {
		completedCount := m.project.CountCompletedTasks()
		completedDetail = fmt.Sprintf("[%d]", completedCount)
	}

	priorityDetail := "(all)"
	if m.filter.Priority == 1 {
		priorityDetail = "!Hi"
	}

	typeDetail := "(all)"
	if m.filter.Type != "" {
		typeDetail = string(m.filter.Type)
	}

	statusDetail := "(all)"
	if m.filter.Status != "" {
		statusDetail = string(m.filter.Status)
	}

	tagDetail := "(none)"
	if len(m.filter.Tags) > 0 {
		tagDetail = fmt.Sprintf("%d tags", len(m.filter.Tags))
	}

	sectionDetail := "(none)"
	if len(m.filter.Sections) > 0 {
		sectionDetail = fmt.Sprintf("%d sections", len(m.filter.Sections))
	}

	dueDetail := "(all)"
	switch m.filter.DueDate {
	case DueDateOverdue:
		dueDetail = "overdue"
	case DueDateToday:
		dueDetail = "today"
	case DueDateThisWeek:
		dueDetail = "week"
	case DueDateNone:
		dueDetail = "no-date"
	}

	readyDetail := "off"
	if m.filter.Actionable {
		readyDetail = "on"
	}

	attrDetail := "(none)"
	if len(m.filter.Attrs) > 0 {
		attrDetail = fmt.Sprintf("%d attrs", len(m.filter.Attrs))
	}

	scoreDetail := "(off)"
	if m.filter.ScoreMin > 0 {
		scoreDetail = fmt.Sprintf(">=%.0f", m.filter.ScoreMin)
	}

	return []PrefixHint{
		{Key: "c", Description: "completed", Detail: completedDetail, Active: m.viewMode == ViewModeCompleted},
		{Key: "p", Description: "priority", Detail: priorityDetail, Active: m.filter.Priority != 0},
		{Key: "t", Description: "type", Detail: typeDetail, Active: m.filter.Type != ""},
		{Key: "s", Description: "status", Detail: statusDetail, Active: m.filter.Status != ""},
		{Key: "#", Description: "tags", Detail: tagDetail, Active: len(m.filter.Tags) > 0},
		{Key: "@", Description: "section", Detail: sectionDetail, Active: len(m.filter.Sections) > 0},
		{Key: "d", Description: "due-date", Detail: dueDetail, Active: m.filter.DueDate != DueDateAll},
		{Key: "a", Description: "attribute", Detail: attrDetail, Active: len(m.filter.Attrs) > 0},
		{Key: "S", Description: "score", Detail: scoreDetail, Active: m.filter.ScoreMin > 0},
		{Key: "r", Description: "ready", Detail: readyDetail, Active: m.filter.Actionable},
		{Key: "v", Description: "saved views"},
		{Key: "x", Description: "clear-all"},
	}
}

// getViewModeTitle returns the card title based on current view mode.
// If a single section is filtered, appends the section name.
func (m *ProjectTreeModel) getViewModeTitle() string {
	var base string
	switch m.viewMode {
	case ViewModeActive:
		base = "Active"
	case ViewModeNext:
		base = "Next"
	case ViewModeInbox:
		base = "Inbox"
	case ViewModeWaiting:
		base = "Waiting"
	case ViewModeSomeday:
		base = "Someday"
	case ViewModeCompleted:
		base = "Completed"
	case ViewModeReview:
		base = "Review"
	case ViewModeSnoozed:
		base = "Snoozed"
	case ViewModeAll:
		base = "All"
	default:
		base = "Tasks"
	}

	// Append section name if exactly one section is filtered
	if len(m.filter.Sections) == 1 {
		for name := range m.filter.Sections {
			base = base + " · @" + name
			break
		}
	}

	return base
}

// getSingleFilteredSectionColor returns the section color if exactly one section
// is being filtered, otherwise returns empty string.
func (m *ProjectTreeModel) getSingleFilteredSectionColor() string {
	if len(m.filter.Sections) != 1 {
		return ""
	}
	// Get the single filtered section name
	var sectionName string
	for name := range m.filter.Sections {
		sectionName = name
		break
	}
	return m.project.GetSectionColor(sectionName)
}
