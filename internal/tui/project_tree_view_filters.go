package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/icons"
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
	return fmt.Sprintf("%s %d %s over section limit (%d)", icons.Get("warning"), over, word, limit)
}

// renderViewModeIndicator returns a styled indicator for the current view mode with counts.
func (m *ProjectTreeModel) renderViewModeIndicator() string {
	counts := m.viewModeCounts()
	parts := []string{
		renderViewModeCount(ViewModeInbox, "Inbox", counts.inbox, m.viewMode == ViewModeInbox || counts.inbox > 0),
		renderViewModeCount(ViewModeActive, "Active", counts.active, m.viewMode == ViewModeActive),
		renderViewModeCount(ViewModeNext, "Next", counts.next, m.viewMode == ViewModeNext),
		renderViewModeCount(ViewModeWaiting, "Waiting", counts.waiting, m.viewMode == ViewModeWaiting),
		renderViewModeCount(ViewModeSomeday, "Someday", counts.someday, m.viewMode == ViewModeSomeday),
	}

	if m.Width() < 100 {
		parts = append(parts, m.renderCollapsedViewModes(counts))
	} else {
		parts = append(parts,
			renderViewModeCount(ViewModeReview, "Review", counts.review, m.viewMode == ViewModeReview || counts.review > 0),
			renderViewModeCount(ViewModeStandby, "Standby", counts.standby, m.viewMode == ViewModeStandby),
		)
		if counts.snoozed > 0 {
			parts = append(parts, renderViewModeCount(ViewModeSnoozed, "Snoozed", counts.snoozed, m.viewMode == ViewModeSnoozed))
		}
	}

	parts = append(parts, renderViewModeCount(ViewModeAll, "All", counts.all(), m.viewMode == ViewModeAll))

	// Blockers indicator (only show if there are active blockers)
	blockerCount := storage.GetActiveBlockerCount(m.project)
	if blockerCount > 0 {
		blockerLabel := fmt.Sprintf("%s%d", icons.Get("blockers"), blockerCount)
		parts = append(parts, DueSoonStyle.Render(blockerLabel))
	}

	return strings.Join(parts, " ")
}

type projectTreeViewModeCounts struct {
	active    int
	next      int
	inbox     int
	waiting   int
	someday   int
	review    int
	standby   int
	completed int
	snoozed   int
}

func (m *ProjectTreeModel) viewModeCounts() projectTreeViewModeCounts {
	var counts projectTreeViewModeCounts
	for _, task := range m.project.Tasks {
		switch {
		case task.IsSnoozed():
			counts.snoozed++
		case task.IsComplete():
			counts.completed++
		default:
			m.incrementSectionCount(&counts, task)
		}
	}
	return counts
}

func (m *ProjectTreeModel) incrementSectionCount(counts *projectTreeViewModeCounts, task *models.Task) {
	switch task.Section {
	case models.SectionNext:
		counts.next++
	case models.SectionInbox:
		counts.inbox++
	case models.SectionWaiting:
		counts.waiting++
	case models.SectionSomeday:
		counts.someday++
	case models.SectionReview:
		counts.review++
	case models.SectionStandby:
		counts.standby++
	default:
		counts.active++
	}
}

func (m *ProjectTreeModel) renderCollapsedViewModes(counts projectTreeViewModeCounts) string {
	switch m.viewMode {
	case ViewModeReview:
		return renderViewModeCount(ViewModeReview, "Review", counts.review, true)
	case ViewModeStandby:
		return renderViewModeCount(ViewModeStandby, "Standby", counts.standby, true)
	case ViewModeSnoozed:
		return renderViewModeCount(ViewModeSnoozed, "Snoozed", counts.snoozed, true)
	case ViewModeActive, ViewModeNext, ViewModeInbox, ViewModeWaiting, ViewModeSomeday, ViewModeCompleted, ViewModeAll:
		fallthrough
	default:
		return MutedStyle.Render(fmt.Sprintf("More[%d]", counts.review+counts.standby+counts.snoozed))
	}
}

func renderViewModeCount(mode ViewMode, label string, count int, active bool) string {
	text := fmt.Sprintf("%s[%d]", label, count)
	if !active {
		return MutedStyle.Render(text)
	}
	return renderViewModeText(mode, text)
}

func renderViewModeText(mode ViewMode, text string) string {
	switch mode {
	case ViewModeActive:
		return ViewModeActiveStyle.Render(text)
	case ViewModeNext:
		return ViewModeNextStyle.Render(text)
	case ViewModeInbox:
		return ViewModeInboxStyle.Render(text)
	case ViewModeWaiting:
		return ViewModeWaitingStyle.Render(text)
	case ViewModeSomeday:
		return ViewModeSomedayStyle.Render(text)
	case ViewModeCompleted:
		return ViewModeCompletedStyle.Render(text)
	case ViewModeReview:
		return ViewModeReviewStyle.Render(text)
	case ViewModeStandby:
		return ViewModeStandbyStyle.Render(text)
	case ViewModeSnoozed:
		return ViewModeSnoozedStyle.Render(text)
	case ViewModeAll:
		return ViewModeAllStyle.Render(text)
	default:
		return MutedStyle.Render(text)
	}
}

func (c projectTreeViewModeCounts) all() int {
	return c.active + c.next + c.inbox + c.waiting + c.someday + c.review + c.standby
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

	// Add common filter parts. commonFilterParts already emits "ready" for
	// Actionable; do not re-append it as a view-specific part below.
	parts = append(parts, commonFilterParts(&m.filter.CommonFilterState)...)

	// Add view-specific filter parts
	if len(m.filter.Sections) > 0 {
		secNames := make([]string, 0, len(m.filter.Sections))
		for sec := range m.filter.Sections {
			secNames = append(secNames, sec)
		}
		sort.Strings(secNames)
		for i, sec := range secNames {
			secNames[i] = "@" + sec
		}
		parts = append(parts, strings.Join(secNames, " "))
	}

	return parts
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *ProjectTreeModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.Dispatcher == nil || !m.Dispatcher.HasPendingPrefix() {
		return nil
	}
	prefix := m.Dispatcher.PendingPrefix()
	var currentStage string
	var currentPriority int
	if task := m.getCursorTask(); task != nil {
		currentStage = task.Stage
		currentPriority = task.Priority
	}
	hints := CommonTaskActionPrefixHints(m.config.Stages, currentStage, currentPriority)
	hints["f"] = FoldPrefixHints
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
	hints["FS"] = BuildFilterScorePrefixHints(m.config, true)
	hints["F."] = BuildFilterStagePrefixHints(m.config.Stages, m.filter.Stage)
	hints["Fv"] = SavedFilterPrefixHints
	return &PrefixPopupInfo{
		Prefix: prefix,
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

	issueDetail := "off"
	if m.filter.GitHubIssue {
		issueDetail = "on"
	}

	parkedDetail := "off"
	if m.filter.Parked {
		parkedDetail = "on"
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
		{Key: "i", Description: "issues", Detail: issueDetail, Active: m.filter.GitHubIssue},
		{Key: "k", Description: "parked", Detail: parkedDetail, Active: m.filter.Parked},
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

// singleActiveTag returns the single active tag filter name, or "" if zero or multiple tags are filtered.
func (m *ProjectTreeModel) singleActiveTag() string {
	if len(m.filter.Tags) != 1 {
		return ""
	}
	for tag := range m.filter.Tags {
		return tag
	}
	return ""
}

// singleActiveSection returns the section to suggest for quick capture based on active filters or view mode.
// Prefers an explicit single-section filter; falls back to the current view mode's section.
func (m *ProjectTreeModel) singleActiveSection() string {
	if len(m.filter.Sections) == 1 {
		for name := range m.filter.Sections {
			return name
		}
	}
	return viewModeToSection(m.viewMode)
}
