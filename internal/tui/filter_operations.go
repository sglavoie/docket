package tui

import (
	"fmt"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

const (
	operationFilterPriority      OperationID = "tui.filter.priority"
	operationFilterStates        OperationID = "tui.filter.states"
	operationFilterTags          OperationID = "tui.filter.tags"
	operationFilterDue           OperationID = "tui.filter.due"
	operationFilterCompletionAge OperationID = "tui.filter.completion-age"
	operationFilterSnoozed       OperationID = "tui.filter.snoozed"
	operationFilterClear         OperationID = "tui.filter.clear"
)

func filterOperationDescriptors() []OperationDescriptor {
	descriptor := func(
		id OperationID,
		label string,
		provider ValueProviderID,
		aliases ...string,
	) OperationDescriptor {
		return OperationDescriptor{
			ID:                   id,
			Label:                label,
			Aliases:              aliases,
			Category:             OperationCategoryFilter,
			TargetCardinality:    TargetCardinalityNone,
			RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
			Applicability:        coreOutlinerApplicability,
			Risk:                 OperationRiskReadOnly,
			ValueProviderID:      provider,
		}
	}
	return []OperationDescriptor{
		descriptor(operationFilterPriority, "Filter by Priority", ValueProviderFilterPriority, "priority threshold"),
		descriptor(operationFilterStates, "Filter by State", ValueProviderFilterStates, "state filter"),
		descriptor(operationFilterTags, "Filter by Tags", ValueProviderFilterTags, "tag filter"),
		descriptor(operationFilterDue, "Filter by Due Date", ValueProviderFilterDue, "due filter"),
		descriptor(operationFilterCompletionAge, "Filter by Completion Age", ValueProviderFilterCompletionAge, "completion age", "cleanup age"),
		descriptor(operationFilterSnoozed, "Filter Snoozed Tasks", "", "snoozed only"),
		descriptor(operationFilterClear, "Clear Filters", "", "reset filters"),
	}
}

func (m *AppModel) registerFilterOperations() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	// Priority is the only preset-only facet: each fp0-3 binding carries its
	// threshold, so reaching the provider means a caller bypassed the grammar.
	if err := m.operationExecutor.RegisterValueProvider(ValueProviderFilterPriority,
		func(OperationInvocation) OperationValueProviderResult {
			return OperationValueProviderResult{
				Err: fmt.Errorf("this filter takes its value from the fp0-3 chord"),
			}
		}); err != nil {
		return err
	}
	for _, provider := range []ValueProviderID{
		ValueProviderFilterStates,
		ValueProviderFilterTags,
		ValueProviderFilterDue,
		ValueProviderFilterCompletionAge,
	} {
		if err := m.operationExecutor.RegisterValueProvider(provider,
			func(OperationInvocation) OperationValueProviderResult {
				// Returning no immediate value asks the shell to open the provider's
				// picker seeded from the current filter.
				return OperationValueProviderResult{}
			}); err != nil {
			return err
		}
	}
	for _, descriptor := range filterOperationDescriptors() {
		descriptor.Applicability = func(ctx CommandContext) bool {
			return coreOutlinerApplicability(ctx) &&
				(m.crossOutliner == nil || !m.crossOutliner.hasSearchProjection())
		}
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities:     NewCapabilitySet(CapabilitySimplifiedInput),
			Applicability:            descriptor.Applicability,
			Handler:                  m.executeFilterOperation,
			DelegateTargetResolution: true,
			DelegateConfirmation:     true,
			// Filters mutate session view state only, so no workspace transaction
			// is opened.
			DelegateTransaction: true,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (m *AppModel) executeFilterOperation(invocation OperationInvocation) OperationHandlerResult {
	out := m.crossOutliner
	if out == nil {
		return OperationHandlerResult{Err: fmt.Errorf("the Outliner is unavailable")}
	}
	if out.hasSearchProjection() {
		return OperationHandlerResult{Status: "Exit search before changing filters"}
	}
	filter := &out.filter.CommonFilterState

	var status string
	switch invocation.Descriptor.ID {
	case operationFilterPriority:
		threshold, ok := invocation.Value.(int)
		if !ok || threshold < 0 || threshold > models.PriorityMax {
			return OperationHandlerResult{Err: fmt.Errorf("invalid priority threshold")}
		}
		filter.Priority = threshold
		if threshold == 0 {
			status = "Priority filter cleared"
		} else {
			status = "Filtering " + priorityThresholdLabel(threshold)
		}
	case operationFilterDue:
		due, ok := invocation.Value.(DueDateFilter)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid due-date filter value")}
		}
		filter.DueDate = due
		status = "Filtering due: " + dueDateFilterLabel(due)
	case operationFilterCompletionAge:
		age, ok := invocation.Value.(CompletionAgeFilter)
		if !ok || age < CompletionAgeAll || age > CompletionAge90Days {
			return OperationHandlerResult{Err: fmt.Errorf("invalid completion-age filter value")}
		}
		filter.setCompletionAgeFilter(age)
		status = "Filtering completed: " + completionAgeFilterLabel(age)
	case operationFilterStates:
		names, ok := invocation.Value.([]string)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid state filter value")}
		}
		states := make([]models.WorkflowState, 0, len(names))
		for _, name := range names {
			state := models.WorkflowState(name)
			if !state.IsValid() {
				return OperationHandlerResult{Err: fmt.Errorf("invalid workflow state %q", name)}
			}
			states = append(states, state)
		}
		set, err := models.NewWorkflowStateSet(states...)
		if err != nil {
			return OperationHandlerResult{Err: err}
		}
		filter.setStateFilter(set, len(states) > 0)
		status = fmt.Sprintf("Filtering %d state(s)", len(states))
		if len(states) == 0 {
			status = "State filter cleared"
		}
	case operationFilterTags:
		names, ok := invocation.Value.([]string)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid tag filter value")}
		}
		tags := make(map[string]bool, len(names))
		for _, name := range names {
			tags[name] = true
		}
		filter.Tags = tags
		status = fmt.Sprintf("Filtering %d tag(s)", len(names))
		if len(names) == 0 {
			status = "Tag filter cleared"
		}
	case operationFilterSnoozed:
		filter.Snoozed = true
		status = "Filtering snoozed tasks"
	case operationFilterClear:
		clearCommonFilters(filter)
		status = "Filters cleared"
	default:
		return OperationHandlerResult{Err: fmt.Errorf("unknown filter operation %s", invocation.Descriptor.ID)}
	}

	out.crossForceRebuild = true
	rebuildOutlinerPreservingCursor(out)
	return OperationHandlerResult{Status: status}
}

func priorityThresholdLabel(threshold int) string {
	switch threshold {
	case models.PriorityHigh:
		return "High priority"
	case models.PriorityMedium:
		return "Medium priority or higher"
	case models.PriorityLow:
		return "Low priority or higher"
	default:
		return "priority"
	}
}

func dueDateFilterLabel(filter DueDateFilter) string {
	switch filter {
	case DueDateOverdue:
		return "overdue"
	case DueDateToday:
		return "today"
	case DueDateThisWeek:
		return "this week"
	case DueDateNone:
		return "none"
	default:
		return "all"
	}
}

func completionAgeFilterLabel(filter CompletionAgeFilter) string {
	if days := filter.days(); days > 0 {
		return fmt.Sprintf("at least %d days ago", days)
	}
	return "all ages"
}

// newFilterDuePicker is the fd single-select picker over the four public
// due-date facets, seeded from the active facet when one is set.
func (m *AppModel) newFilterDuePicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	options := []struct {
		id        string
		label     string
		secondary string
		value     DueDateFilter
	}{
		{id: "overdue", label: "Overdue", secondary: "before today", value: DueDateOverdue},
		{id: "today", label: "Today", secondary: "due today", value: DueDateToday},
		{id: "week", label: "This week", secondary: "within seven days", value: DueDateThisWeek},
		{id: "none", label: "No due date", secondary: "unscheduled tasks", value: DueDateNone},
	}
	items := make([]ListItem, 0, len(options))
	for _, option := range options {
		items = append(items, ListItem{
			ID:            ListItemID("filter-due:" + option.id),
			PrimaryText:   option.label,
			SecondaryText: option.secondary,
			Aliases:       []string{option.id, option.label, option.secondary},
			Data:          option.value,
		})
	}
	selected := []ListItemID(nil)
	if m.crossOutliner != nil {
		switch m.crossOutliner.filter.DueDate {
		case DueDateAll:
			selected = nil
		case DueDateOverdue:
			selected = []ListItemID{"filter-due:overdue"}
		case DueDateToday:
			selected = []ListItemID{"filter-due:today"}
		case DueDateThisWeek:
			selected = []ListItemID{"filter-due:week"}
		case DueDateNone:
			selected = []ListItemID{"filter-due:none"}
		}
	}
	return newOperationValuePickerModel(
		request,
		ValueProviderFilterDue,
		"Filter by Due Date",
		"keep matching tasks",
		ListSource{Items: items},
		false,
		selected,
	)
}

// newFilterCompletionAgePicker is the fc single-select picker. It changes only
// the completion-age facet: users must choose Done, Canceled, or both through
// fs before cleanup records become visible.
func (m *AppModel) newFilterCompletionAgePicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	options := []struct {
		id        string
		label     string
		secondary string
		value     CompletionAgeFilter
	}{
		{id: "all", label: "All ages", secondary: "any valid completion date", value: CompletionAgeAll},
		{id: "7", label: "7+ days", secondary: "completed at least 7 local days ago", value: CompletionAge7Days},
		{id: "30", label: "30+ days", secondary: "completed at least 30 local days ago", value: CompletionAge30Days},
		{id: "90", label: "90+ days", secondary: "completed at least 90 local days ago", value: CompletionAge90Days},
	}
	items := make([]ListItem, 0, len(options))
	for _, option := range options {
		items = append(items, ListItem{
			ID:            ListItemID("filter-completion-age:" + option.id),
			PrimaryText:   option.label,
			SecondaryText: option.secondary,
			Aliases:       []string{option.id, option.label, option.secondary},
			Data:          option.value,
		})
	}
	selected := []ListItemID(nil)
	if m.crossOutliner != nil && m.crossOutliner.filter.hasCompletionAgeFilter() {
		suffix := "all"
		switch m.crossOutliner.filter.CompletionAge {
		case CompletionAge7Days:
			suffix = "7"
		case CompletionAge30Days:
			suffix = "30"
		case CompletionAge90Days:
			suffix = "90"
		default:
		}
		selected = []ListItemID{ListItemID("filter-completion-age:" + suffix)}
	}
	return newOperationValuePickerModel(
		request,
		ValueProviderFilterCompletionAge,
		"Filter by Completion Age",
		"combine with fs before cleanup",
		ListSource{Items: items},
		false,
		selected,
	)
}

// newFilterStatesPicker is the fs multi-select picker over the canonical
// workflow states, seeded from the active state filter facet.
func (m *AppModel) newFilterStatesPicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	source := ListSourceFromWorkflowStates(models.AllWorkflowStates())
	counts := m.prospectiveStateFilterCounts()
	for index := range source.Items {
		if state, ok := source.Items[index].Data.(models.WorkflowState); ok {
			source.Items[index].SecondaryText += " " + renderProspectiveFilterCount(counts[state])
			source.Items[index].Data = state.String()
		}
	}
	selected := []ListItemID(nil)
	if m.crossOutliner != nil {
		for _, state := range m.crossOutliner.filter.States.Values() {
			selected = append(selected, ListItemID("state:"+state.String()))
		}
	}
	return newOperationValuePickerModel(
		request,
		ValueProviderFilterStates,
		"Filter by State",
		"keep matching tasks",
		source,
		true,
		selected,
	)
}

// prospectiveStateFilterCounts returns the matching-task buckets available to
// the state facet. The existing state selection is replaced because workflow
// states compose with OR; every other active facet, search, mode,
// Context/project scope, and zoom remain in force.
func (m *AppModel) prospectiveStateFilterCounts() map[models.WorkflowState]int {
	counts := make(map[models.WorkflowState]int, len(models.AllWorkflowStates()))
	if m == nil || m.crossOutliner == nil {
		return counts
	}

	filter := copyCommonFilterState(&m.crossOutliner.filter.CommonFilterState)
	states, err := models.NewWorkflowStateSet(models.AllWorkflowStates()...)
	if err != nil {
		return counts
	}
	filter.setStateFilter(states, true)
	return prospectiveOutlinerStateCounts(m.crossOutliner, filter)
}

// prospectiveOutlinerStateCounts runs the canonical row projection on an
// isolated model copy. Expanding task trees and sections keeps presentation
// folds from changing facet totals, while rebuilding through the normal mode
// pipeline preserves all actual scope and visibility rules.
func prospectiveOutlinerStateCounts(
	outliner *OutlinerModel,
	filter *CommonFilterState,
) map[models.WorkflowState]int {
	counts := make(map[models.WorkflowState]int, len(models.AllWorkflowStates()))
	if outliner == nil || filter == nil {
		return counts
	}

	projection := *outliner
	projection.filter.CommonFilterState = *copyCommonFilterState(filter)
	projection.rows = nil
	projection.OpenAtTask = nil
	projection.expandedNodes = make(map[TaskIdentity]bool)
	projection.collapsedSections = make(map[string]bool)
	projection.zoomStack = append([]ZoomEntry(nil), outliner.zoomStack...)
	projection.crossZoomStack = append(
		[]CrossProjectZoomEntry(nil),
		outliner.crossZoomStack...,
	)
	projection.crossForceRebuild = true
	projection.rebuildRows()

	// A matching descendant can retain a non-matching ancestor as hierarchy
	// context. Count only rows that satisfy the prospective predicate themselves.
	now := time.Now()
	matches := projection.workModeTaskFilterAt(now)
	seen := make(map[TaskIdentity]struct{})
	for _, row := range projection.rows {
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		project := projection.resolveProjectForRow(row)
		if !matches(row.task, project) {
			continue
		}
		projectID := row.ProjectID
		if projectID == "" && project != nil {
			projectID = project.ID
		}
		identity := TaskIdentity{ProjectID: projectID, TaskID: row.task.ID}
		if _, duplicate := seen[identity]; duplicate {
			continue
		}
		seen[identity] = struct{}{}
		counts[row.task.State()]++
	}
	return counts
}

func renderProspectiveFilterCount(count int) string {
	if count == 0 {
		zero := lipgloss.NewStyle().
			Foreground(ColorError).
			Faint(true).
			Render("0")
		return "⋅ " + zero
	}
	return fmt.Sprintf("⋅ %d", count)
}

// newFilterTagsPicker is the ft multi-select picker over the active scope's
// tags, seeded from the active tag filter facet.
func (m *AppModel) newFilterTagsPicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	projects := m.activeProjects()
	if m.crossOutliner != nil && len(m.crossOutliner.loadedProjects) > 0 {
		projects = m.crossOutliner.workModeScopeProjects()
	}
	source := ListSourceFromTags(BuildProjectsTagList(projects, false))
	for index := range source.Items {
		if tag, ok := source.Items[index].Data.(TagInfo); ok {
			source.Items[index].Data = tag.Name
		}
	}
	selected := []ListItemID(nil)
	if m.crossOutliner != nil {
		for tag := range m.crossOutliner.filter.Tags {
			selected = append(selected, ListItemID("tag:"+tag))
		}
	}
	return newOperationValuePickerModel(
		request,
		ValueProviderFilterTags,
		"Filter by Tags",
		"keep matching tasks",
		source,
		true,
		selected,
	)
}
