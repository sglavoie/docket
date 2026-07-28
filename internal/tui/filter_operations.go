package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

const (
	operationFilterPriority OperationID = "tui.filter.priority"
	operationFilterStates   OperationID = "tui.filter.states"
	operationFilterTags     OperationID = "tui.filter.tags"
	operationFilterDue      OperationID = "tui.filter.due"
	operationFilterClear    OperationID = "tui.filter.clear"
)

func filterOperationDescriptors() []OperationDescriptor {
	descriptor := func(
		id OperationID,
		label string,
		provider ValueProviderID,
		aliases ...string,
	) OperationDescriptor {
		return OperationDescriptor{
			ID:      id,
			Label:   label,
			Aliases: aliases,
			// The filter family is a chord-only view refinement: it narrows the one
			// Outliner in place, so Finder and the action sheets never list it.
			Placement:            PlacementHidden,
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
		descriptor(operationFilterClear, "Clear Filters", "", "reset filters"),
	}
}

func (m *AppModel) registerFilterOperations() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	// The preset-only providers reject a picker request outright: every priority
	// and due binding carries its value, so reaching the provider means a caller
	// bypassed the chord grammar.
	presetOnly := map[ValueProviderID]string{
		ValueProviderFilterPriority: "fp0-5",
		ValueProviderFilterDue:      "fd",
	}
	for provider, chord := range presetOnly {
		if err := m.operationExecutor.RegisterValueProvider(provider,
			func(OperationInvocation) OperationValueProviderResult {
				return OperationValueProviderResult{
					Err: fmt.Errorf("this filter takes its value from the %s chord", chord),
				}
			}); err != nil {
			return err
		}
	}
	for _, provider := range []ValueProviderID{ValueProviderFilterStates, ValueProviderFilterTags} {
		if err := m.operationExecutor.RegisterValueProvider(provider,
			func(OperationInvocation) OperationValueProviderResult {
				// Returning no immediate value asks the shell to open the provider's
				// multi-select picker seeded from the current filter.
				return OperationValueProviderResult{}
			}); err != nil {
			return err
		}
	}
	for _, descriptor := range filterOperationDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities:     NewCapabilitySet(CapabilitySimplifiedInput),
			Applicability:            coreOutlinerApplicability,
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
	filter := &out.filter.CommonFilterState

	var status string
	switch invocation.Descriptor.ID {
	case operationFilterPriority:
		threshold, ok := invocation.Value.(int)
		if !ok || threshold < 0 || threshold > models.PriorityMax {
			return OperationHandlerResult{Err: fmt.Errorf("invalid priority threshold")}
		}
		filter.Priority = threshold
		status = fmt.Sprintf("Filtering priority <= %d", threshold)
		if threshold == 0 {
			status = "Priority filter cleared"
		}
	case operationFilterDue:
		due, ok := invocation.Value.(DueDateFilter)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid due-date filter value")}
		}
		filter.DueDate = due
		status = "Filtering due: " + dueDateFilterLabel(due)
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
	case operationFilterClear:
		clearCommonFilters(filter)
		status = "Filters cleared"
	default:
		return OperationHandlerResult{Err: fmt.Errorf("unknown filter operation %s", invocation.Descriptor.ID)}
	}

	out.crossForceRebuild = true
	out.rebuildRows()
	out.clampCursor()
	return OperationHandlerResult{Status: status}
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

// newFilterStatesPicker is the fs multi-select picker over the canonical
// workflow states, seeded from the active state filter facet.
func (m *AppModel) newFilterStatesPicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	source := ListSourceFromWorkflowStates(models.AllWorkflowStates())
	for index := range source.Items {
		if state, ok := source.Items[index].Data.(models.WorkflowState); ok {
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

// newFilterTagsPicker is the ft multi-select picker over the active scope's
// tags, seeded from the active tag filter facet.
func (m *AppModel) newFilterTagsPicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	projects := m.activeProjects()
	if m.crossOutliner != nil && len(m.crossOutliner.loadedProjects) > 0 {
		projects = m.crossOutliner.loadedProjects
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
