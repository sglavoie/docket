package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// viewModeToSection maps a ViewMode to its GTD section name string for limit warnings.
// Returns "" for ViewModeActive, ViewModeAll, ViewModeCompleted, ViewModeSnoozed
// (no project-tree warning for those — the outliner handles Active via section headers).
func viewModeToSection(vm ViewMode) string {
	switch vm {
	case ViewModeNext:
		return models.SectionNext
	case ViewModeInbox:
		return models.SectionInbox
	case ViewModeWaiting:
		return models.SectionWaiting
	case ViewModeSomeday:
		return models.SectionSomeday
	case ViewModeReview:
		return models.SectionReview
	case ViewModeStandby:
		return models.SectionStandby
	default:
		// ViewModeAll, ViewModeCompleted, ViewModeSnoozed — no limit warning
		return ""
	}
}

// sectionTaskCount returns the number of non-completed, non-snoozed tasks in
// the given section (where sectionName="" means Active).
func sectionTaskCount(tasks []*models.Task, sectionName string) int {
	count := 0
	for _, t := range tasks {
		if t.IsComplete() || t.IsSnoozed() {
			continue
		}
		if t.Section == sectionName {
			count++
		}
	}
	return count
}

// sectionLimitWarning returns a warning string like "⚠ 3 tasks over section limit (5)"
// with correct singular/plural form.
func sectionLimitWarning(over, limit int) string {
	word := "tasks"
	if over == 1 {
		word = "task"
	}
	return fmt.Sprintf("⚠ %d %s over section limit (%d)", over, word, limit)
}

// renderViewModeIndicator returns a styled indicator for the current view mode with counts.
func (m *ProjectTreeModel) renderViewModeIndicator() string {
	// Count by section: active, next, inbox, waiting, someday, completed
	var activeCount, nextCount, inboxCount, waitingCount, somedayCount, reviewCount, standbyCount, completedCount, snoozedCount int
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
		case models.SectionStandby:
			standbyCount++
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

	// Standby view
	standbyLabel := fmt.Sprintf("Standby[%d]", standbyCount)
	if m.viewMode == ViewModeStandby {
		parts = append(parts, ViewModeStandbyStyle.Render(standbyLabel))
	} else {
		parts = append(parts, MutedStyle.Render(standbyLabel))
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
	allCount := activeCount + nextCount + inboxCount + waitingCount + somedayCount + reviewCount + standbyCount
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

	// View mode indicator (always shown)
	result.WriteString(m.renderViewModeIndicator())
	result.WriteString("\n")

	return result.String()
}

// ActiveFilterCount returns the number of active filters for the status bar pill.
func (m *ProjectTreeModel) ActiveFilterCount() int {
	return len(m.activeFilterParts())
}

// activeFilterParts returns the list of active filter description strings.
func (m *ProjectTreeModel) activeFilterParts() []string {
	var parts []string

	// Show saved filter badge if active
	if m.activeSavedFilter != "" {
		parts = append(parts, SavedFilterBadgeStyle.Render("View: "+m.activeSavedFilter))
	}

	if m.viewMode == ViewModeCompleted {
		parts = append(parts, "completed")
	}

	// Add common filter parts
	parts = append(parts, commonFilterParts(&m.filter.CommonFilterState)...)

	// Add view-specific filter parts
	if len(m.filter.Sections) > 0 {
		var secNames []string
		for sec := range m.filter.Sections {
			secNames = append(secNames, "@"+sec)
		}
		parts = append(parts, strings.Join(secNames, " "))
	}
	if m.filter.Actionable {
		parts = append(parts, "ready")
	}

	return parts
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
	var currentStage string
	var currentPriority int
	if task := m.getCursorTask(); task != nil {
		currentStage = task.Stage
		currentPriority = task.Priority
	}
	hints := CommonTaskActionPrefixHints(m.config.Stages, currentStage, currentPriority)
	hints["z"] = FoldPrefixHints
	hints["g"] = ProjectTreeNavigationPrefixHints
	hints["m"] = ProjectTreeMovePrefixHints // override: tree uses tree moves
	hints["t"] = TaskPrefixHints
	hints["b"] = BlockerPrefixHints
	hints["F"] = m.buildFilterPrefixHints()
	hints["Ft"] = FilterTypePrefixHints
	hints["Fs"] = FilterStatusPrefixHints
	hints["Fd"] = FilterDuePrefixHints
	hints["Fa"] = FilterAttrPrefixHints
	hints["Fas"] = FilterAttrSeverityPrefixHints
	hints["Far"] = FilterAttrReproduciblePrefixHints
	hints["Fae"] = FilterAttrEffortPrefixHints
	hints["Fac"] = FilterAttrCategoryPrefixHints
	hints["Fav"] = FilterAttrValuePrefixHints
	hints["Faw"] = FilterAttrWorktypePrefixHints
	hints["Fao"] = FilterAttrScopePrefixHints
	hints["Fap"] = FilterAttrPointsPrefixHints
	hints["FS"] = BuildFilterScorePrefixHints(m.config)
	hints["F."] = BuildFilterStagePrefixHints(m.config.Stages, m.filter.Stage)
	hints["Fv"] = SavedFilterPrefixHints
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  hints,
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
	default:
	}

	readyDetail := "off"
	if m.filter.Actionable {
		readyDetail = "on"
	}

	dimDetail := "off"
	if m.filter.HideDimmed {
		dimDetail = "on"
	}

	attrDetail := "(none)"
	if len(m.filter.Attrs) > 0 {
		attrDetail = fmt.Sprintf("%d attrs", len(m.filter.Attrs))
	}

	scoreDetail := "(off)"
	if m.filter.ScoreMin > 0 {
		scoreDetail = fmt.Sprintf(">=%.0f", m.filter.ScoreMin)
	}

	stageDetail := "(all)"
	if m.filter.Stage != "" {
		stageDetail = m.filter.Stage
	}

	hints := []PrefixHint{
		{Key: "c", Description: "completed", Detail: completedDetail, Active: m.viewMode == ViewModeCompleted},
		{Key: "p", Description: "priority", Detail: priorityDetail, Active: m.filter.Priority != 0},
		{Key: "t", Description: "type", Detail: typeDetail, Active: m.filter.Type != ""},
		{Key: "s", Description: "status", Detail: statusDetail, Active: m.filter.Status != ""},
		{Key: "#", Description: "tags", Detail: tagDetail, Active: len(m.filter.Tags) > 0},
		{Key: "@", Description: "section", Detail: sectionDetail, Active: len(m.filter.Sections) > 0},
		{Key: "d", Description: "due-date", Detail: dueDetail, Active: m.filter.DueDate != DueDateAll},
		{Key: "a", Description: "attribute", Detail: attrDetail, Active: len(m.filter.Attrs) > 0},
		{Key: ".", Description: "stage", Detail: stageDetail, Active: m.filter.Stage != ""},
		{Key: "S", Description: "score", Detail: scoreDetail, Active: m.filter.ScoreMin > 0},
		{Key: "r", Description: "ready", Detail: readyDetail, Active: m.filter.Actionable},
		{Key: "h", Description: "dimmed", Detail: dimDetail, Active: m.filter.HideDimmed},
		{Key: "v", Description: "saved filters"},
		{Key: "D", Description: "columns"},
		{Key: "x", Description: "clear-all"},
	}
	sortPrefixHintsLowercaseFirst(hints)
	return hints
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
	case ViewModeStandby:
		base = "Standby"
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
