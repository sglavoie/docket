package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

const (
	operationFilterPriority      OperationID = "tui.filter.priority"
	operationFilterStates        OperationID = "tui.filter.states"
	operationFilterTags          OperationID = "tui.filter.tags"
	operationFilterContexts      OperationID = "tui.filter.contexts"
	operationFilterReview        OperationID = "tui.filter.review"
	operationFilterDue           OperationID = "tui.filter.due"
	operationFilterCompletionAge OperationID = "tui.filter.completion-age"
	operationFilterHierarchy     OperationID = "tui.filter.hierarchy"
	operationFilterStale         OperationID = "tui.filter.stale"
	operationFilterClear         OperationID = "tui.filter.clear"
)

// filterDeferredOnly is the value the due picker's deferred row carries. It is
// its own type because that row writes CommonFilterState.Deferred rather than
// the DueDate facet the other four rows write.
type filterDeferredOnly struct{}

// filterOperationDescriptor is the shape every facet shares: no target, no
// transaction, and applicability decided by the Outliner alone.
func filterOperationDescriptor(
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

func filterOperationDescriptors() []OperationDescriptor {
	descriptor := filterOperationDescriptor
	return []OperationDescriptor{
		descriptor(operationFilterPriority, "Filter by Priority", ValueProviderFilterPriority, "priority threshold"),
		descriptor(operationFilterStates, "Filter by State", ValueProviderFilterStates, "state filter"),
		descriptor(operationFilterTags, "Filter by Tags", ValueProviderFilterTags, "tag filter"),
		descriptor(operationFilterContexts, "Filter by Context", ValueProviderFilterContexts, "context filter"),
		descriptor(operationFilterReview, "Filter Review", "", "review tag filter"),
		descriptor(operationFilterDue, "Filter by Due or Deferred", ValueProviderFilterDue, "due filter", "deferred", "hidden"),
		descriptor(operationFilterCompletionAge, "Filter by Completion Age", ValueProviderFilterCompletionAge, "completion age", "cleanup age"),
		descriptor(operationFilterHierarchy, "Filter by Hierarchy", ValueProviderFilterHierarchy, "hierarchy position", "parent or child"),
		descriptor(operationFilterStale, "Filter Stale", "", "stale work", "untouched work"),
		descriptor(operationFilterClear, "Clear Filters", "", "reset filters"),
	}
}

func (m *AppModel) registerFilterOperations() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	// Priority is the only preset-only facet: each f!0-3 binding carries its
	// threshold, so reaching the provider means a caller bypassed the grammar.
	if err := m.operationExecutor.RegisterValueProvider(ValueProviderFilterPriority,
		func(OperationInvocation) OperationValueProviderResult {
			return OperationValueProviderResult{
				Err: fmt.Errorf("this filter takes its value from the f!0-3 chord"),
			}
		}); err != nil {
		return err
	}
	for _, provider := range []ValueProviderID{
		ValueProviderFilterStates,
		ValueProviderFilterTags,
		ValueProviderFilterContexts,
		ValueProviderFilterDue,
		ValueProviderFilterCompletionAge,
		ValueProviderFilterHierarchy,
		ValueProviderFilterProjectTags,
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
	// One closure body serves both families. Which of them refuses a search
	// projection is not declared here: it is read off the clause fields the
	// operation writes, so a key is advertised exactly when its handler will
	// accept it.
	register := func(descriptors []OperationDescriptor, handler OperationHandler) error {
		for _, descriptor := range descriptors {
			stale := descriptor.ID == operationFilterStale
			refusesSearchProjection := filterOperationRequiresReload(descriptor.ID)
			descriptor.Applicability = func(ctx CommandContext) bool {
				if !coreOutlinerApplicability(ctx) {
					return false
				}
				if refusesSearchProjection &&
					m.crossOutliner != nil && m.crossOutliner.hasSearchProjection() {
					return false
				}
				return !stale || m.crossOutliner == nil ||
					m.crossOutliner.activeOutlinerMode() != ModeRecent
			}
			if err := m.registerOperation(descriptor, OperationBinding{
				RequiredCapabilities:     NewCapabilitySet(CapabilitySimplifiedInput),
				Applicability:            descriptor.Applicability,
				Handler:                  handler,
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
	if err := register(filterOperationDescriptors(), m.executeFilterOperation); err != nil {
		return err
	}
	return register(projectFilterOperationDescriptors(), m.executeProjectFilterOperation)
}

// filterOperationFields names the clause fields one facet owns. A facet writes
// these fields outright and nothing else, so applying it can neither preserve a
// value it just replaced nor disturb a facet beside it.
func filterOperationFields(id OperationID) []models.QueryField {
	switch id {
	case operationFilterPriority:
		return []models.QueryField{models.QueryFieldPriority}
	case operationFilterStates:
		return []models.QueryField{models.QueryFieldState}
	case operationFilterTags, operationFilterReview:
		return []models.QueryField{models.QueryFieldTag}
	case operationFilterContexts:
		return []models.QueryField{models.QueryFieldContext}
	case operationFilterDue:
		// The due picker's deferred row and its four bucket rows are one facet:
		// choosing either clears the other, which is only true if both fields
		// belong to the same operation.
		return []models.QueryField{models.QueryFieldDue, models.QueryFieldDeferred}
	case operationFilterCompletionAge:
		// The cleanup facet installs the terminal-state predicate itself, which is
		// why commonFilterFacetCount reads the pair as one clearable facet.
		return []models.QueryField{models.QueryFieldCompleted, models.QueryFieldState}
	case operationFilterHierarchy:
		return filterQueryHierarchyFields
	case operationFilterStale:
		return []models.QueryField{models.QueryFieldStale}
	case operationFilterClear:
		return filterQueryTaskFields()
	case operationFilterProjectTags:
		// The F# picker carries the parked row in the same list of names, so the
		// tag restriction and the parked axis move together.
		return []models.QueryField{models.QueryFieldProjectTag, models.QueryFieldParked}
	case operationFilterShowParked, operationFilterOnlyParked:
		return []models.QueryField{models.QueryFieldParked}
	case operationFilterIncludeArchived, operationFilterOnlyArchived:
		return []models.QueryField{models.QueryFieldArchived}
	case operationFilterProjectClear:
		return filterQueryPopulationFields
	default:
		return nil
	}
}

// filterOperationRequiresReload reports whether applying a facet reloads the
// population rather than rebuilding the rows. It is computed from the fields the
// operation writes rather than declared beside its handler, so a facet cannot be
// registered claiming one obligation and implemented honoring another.
func filterOperationRequiresReload(id OperationID) bool {
	for _, field := range filterOperationFields(id) {
		if models.QueryFieldRequiresReload(field) {
			return true
		}
	}
	return false
}

// filterClauseEdit is what one facet constructor produces: the clauses its own
// fields now carry, and the status the surface reports. Narrowing nothing is an
// empty clause list, not a separate clearing verb.
type filterClauseEdit struct {
	clauses []models.QueryClause
	status  string
	// declined reports a facet that changed nothing and only has something to
	// say, such as a clear with nothing to clear.
	declined bool
}

// applyFilterClauseEdit is the one path every facet mutation takes. It writes the
// operation's fields into the Outliner's single filter query and lets both
// carriers be re-derived from the result, so a picker, a chip, a count, and the
// row predicate cannot disagree about what is filtered.
//
// A population facet decides which projects load at all, so applying one needs a
// reload rather than a rebuild, and projectFilterReloadCmd lands the cursor at
// the top of a freshly loaded scope. That same obligation is why it refuses under
// a search projection: the projection is scored over the snapshot the reload
// replaces.
func (m *AppModel) applyFilterClauseEdit(id OperationID, edit filterClauseEdit) OperationHandlerResult {
	out := m.crossOutliner
	if out == nil {
		return OperationHandlerResult{Err: fmt.Errorf("the Outliner is unavailable")}
	}
	reload := filterOperationRequiresReload(id)
	if reload && out.hasSearchProjection() {
		return OperationHandlerResult{Status: "Exit search before changing filters"}
	}
	if edit.declined {
		return OperationHandlerResult{Status: edit.status}
	}
	out.replaceFilterClauses(filterOperationFields(id), edit.clauses)
	if reload {
		return OperationHandlerResult{
			Message: OperationCommandResultMsg{Cmd: m.projectFilterReloadCmd(edit.status)},
		}
	}
	out.crossForceRebuild = true
	rebuildOutlinerPreservingCursor(out)
	return OperationHandlerResult{Status: edit.status}
}

func (m *AppModel) executeFilterOperation(invocation OperationInvocation) OperationHandlerResult {
	out := m.crossOutliner
	if out == nil {
		return OperationHandlerResult{Err: fmt.Errorf("the Outliner is unavailable")}
	}
	edit, err := out.rowFilterClauseEdit(invocation)
	if err != nil {
		return OperationHandlerResult{Err: err}
	}
	return m.applyFilterClauseEdit(invocation.Descriptor.ID, edit)
}

// rowFilterClauseEdit is one clause constructor per row facet. Each reads the
// picker's value and spells what its fields now say; none of them mutates
// anything, so the whole f chord has exactly one writer.
//
//nolint:gocyclo // One constructor per facet, kept together so the set is auditable.
func (m *OutlinerModel) rowFilterClauseEdit(invocation OperationInvocation) (filterClauseEdit, error) {
	switch invocation.Descriptor.ID {
	case operationFilterPriority:
		threshold, ok := invocation.Value.(int)
		if !ok || threshold < 0 || threshold > models.PriorityMax {
			return filterClauseEdit{}, fmt.Errorf("invalid priority threshold")
		}
		if threshold == 0 {
			return filterClauseEdit{status: "Priority filter cleared"}, nil
		}
		return filterClauseEdit{
			clauses: filterClauseList(models.QueryFieldPriority,
				models.QueryOperatorAtLeast, strconv.Itoa(threshold)),
			status: "Filtering " + priorityThresholdLabel(threshold),
		}, nil
	case operationFilterDue:
		if _, deferred := invocation.Value.(filterDeferredOnly); deferred {
			return filterClauseEdit{
				clauses: filterClauseList(models.QueryFieldDeferred, models.QueryOperatorPresent, ""),
				status:  "Filtering deferred tasks",
			}, nil
		}
		due, ok := invocation.Value.(DueDateFilter)
		if !ok {
			return filterClauseEdit{}, fmt.Errorf("invalid due-date filter value")
		}
		return filterClauseEdit{
			clauses: dueFilterClauses(due),
			status:  "Filtering due: " + dueDateFilterLabel(due),
		}, nil
	case operationFilterCompletionAge:
		age, ok := invocation.Value.(CompletionAgeFilter)
		if !ok || age < CompletionAgeAll || age > CompletionAge90Days {
			return filterClauseEdit{}, fmt.Errorf("invalid completion-age filter value")
		}
		return filterClauseEdit{
			clauses: completionAgeClauses(age),
			status:  "Filtering completed: " + completionAgeFilterLabel(age),
		}, nil
	case operationFilterHierarchy:
		hierarchy, ok := invocation.Value.(HierarchyFilter)
		if !ok || hierarchy < HierarchyParent || hierarchy > HierarchyLeaf {
			return filterClauseEdit{}, fmt.Errorf("invalid hierarchy filter value")
		}
		return filterClauseEdit{
			clauses: hierarchyFilterClauses(hierarchy),
			status:  "Filtering hierarchy: " + hierarchyFilterLabel(hierarchy),
		}, nil
	case operationFilterStates:
		names, ok := invocation.Value.([]string)
		if !ok {
			return filterClauseEdit{}, fmt.Errorf("invalid state filter value")
		}
		clauses := make([]models.QueryClause, 0, len(names))
		for _, name := range names {
			if !models.WorkflowState(name).IsValid() {
				return filterClauseEdit{}, fmt.Errorf("invalid workflow state %q", name)
			}
			clauses = appendFilterClause(clauses, models.QueryFieldState,
				models.QueryOperatorIs, name)
		}
		status := fmt.Sprintf("Filtering %d state(s)", len(names))
		if len(names) == 0 {
			status = "State filter cleared"
		}
		return filterClauseEdit{clauses: clauses, status: status}, nil
	case operationFilterTags:
		names, ok := invocation.Value.([]string)
		if !ok {
			return filterClauseEdit{}, fmt.Errorf("invalid tag filter value")
		}
		clauses := make([]models.QueryClause, 0, len(names))
		for _, name := range names {
			clauses = appendFilterClause(clauses, models.QueryFieldTag,
				models.QueryOperatorIs, name)
		}
		status := fmt.Sprintf("Filtering %d tag(s)", len(names))
		if len(names) == 0 {
			status = "Tag filter cleared"
		}
		return filterClauseEdit{clauses: clauses, status: status}, nil
	case operationFilterContexts:
		names, ok := invocation.Value.([]string)
		if !ok {
			return filterClauseEdit{}, fmt.Errorf("invalid context filter value")
		}
		clauses := contextFilterClauses(names)
		status := "Context filter cleared"
		if len(clauses) > 0 {
			status = "Filtering context: " + strings.Join(filterClauseParts(clauses), " ")
		}
		return filterClauseEdit{clauses: clauses, status: status}, nil
	case operationFilterReview:
		return m.reviewFilterClauseEdit(), nil
	case operationFilterStale:
		if m.filter.Stale {
			return filterClauseEdit{status: "Stale filter cleared"}, nil
		}
		return filterClauseEdit{
			clauses: filterClauseList(models.QueryFieldStale, models.QueryOperatorPresent, ""),
			status:  "Filtering Stale",
		}, nil
	case operationFilterClear:
		return filterClauseEdit{status: "Filters cleared"}, nil
	default:
		return filterClauseEdit{}, fmt.Errorf("unknown filter operation %s", invocation.Descriptor.ID)
	}
}

// completionAgeClauses spells the cleanup facet. It carries the terminal-state
// predicate itself, because a completion age asked of open work would name a
// population that cannot answer it.
func completionAgeClauses(age CompletionAgeFilter) []models.QueryClause {
	clauses := filterClauseList(models.QueryFieldState,
		models.QueryOperatorIs, string(models.StateDone))
	clauses = appendFilterClause(clauses, models.QueryFieldState,
		models.QueryOperatorIs, string(models.StateCanceled))
	if days := age.days(); days > 0 {
		return appendFilterClause(clauses, models.QueryFieldCompleted,
			models.QueryOperatorAtLeast, fmt.Sprintf("%dd", days))
	}
	return appendFilterClause(clauses, models.QueryFieldCompleted,
		models.QueryOperatorPresent, "")
}

// contextFilterClauses spells one context picker result. The no-context
// pseudo-entry travels in the same list of names and becomes the none operator,
// so the facet needs no second value channel.
func contextFilterClauses(names []string) []models.QueryClause {
	clauses := make([]models.QueryClause, 0, len(names))
	for _, name := range names {
		if name == noContextFilterKey {
			clauses = appendFilterClause(clauses, models.QueryFieldContext,
				models.QueryOperatorNone, "")
			continue
		}
		clauses = appendFilterClause(clauses, models.QueryFieldContext,
			models.QueryOperatorIs, name)
	}
	return clauses
}

// reviewFilterClauseEdit toggles the review tag inside the tag facet, leaving
// every other tag the picker chose in place.
func (m *OutlinerModel) reviewFilterClauseEdit() filterClauseEdit {
	current := queryClausesForField(m.filterQuery, models.QueryFieldTag)
	kept := make([]models.QueryClause, 0, len(current)+1)
	found := false
	for _, clause := range current {
		if clause.Value == reviewTaskTag {
			found = true
			continue
		}
		kept = append(kept, clause)
	}
	if found {
		return filterClauseEdit{clauses: kept, status: "Review filter cleared"}
	}
	kept = appendFilterClause(kept, models.QueryFieldTag,
		models.QueryOperatorIs, reviewTaskTag)
	return filterClauseEdit{clauses: kept, status: "Filtering Review"}
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

func hierarchyFilterLabel(filter HierarchyFilter) string {
	switch filter {
	case HierarchyParent:
		return "parent"
	case HierarchyChild:
		return "child"
	case HierarchyRoot:
		return "root"
	case HierarchyLeaf:
		return "leaf"
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

// newFilterDuePicker is the f@ single-select picker over the four public
// due-date facets plus the deferred-only facet, seeded from the active facet
// when one is set. Deferral is a scheduling instant like a due date, so it is a
// row here rather than a chord of its own.
func (m *AppModel) newFilterDuePicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	options := []struct {
		id        string
		label     string
		secondary string
		value     any
	}{
		{id: "overdue", label: "Overdue", secondary: "before today", value: DueDateOverdue},
		{id: "today", label: "Today", secondary: "due today", value: DueDateToday},
		{id: "week", label: "This week", secondary: "within seven days", value: DueDateThisWeek},
		{id: "none", label: "No due date", secondary: "unscheduled tasks", value: DueDateNone},
		{id: "deferred", label: "Deferred only", secondary: "waking later", value: filterDeferredOnly{}},
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
		switch {
		case m.crossOutliner.filter.Deferred:
			selected = []ListItemID{"filter-due:deferred"}
		case m.crossOutliner.filter.DueDate == DueDateOverdue:
			selected = []ListItemID{"filter-due:overdue"}
		case m.crossOutliner.filter.DueDate == DueDateToday:
			selected = []ListItemID{"filter-due:today"}
		case m.crossOutliner.filter.DueDate == DueDateThisWeek:
			selected = []ListItemID{"filter-due:week"}
		case m.crossOutliner.filter.DueDate == DueDateNone:
			selected = []ListItemID{"filter-due:none"}
		}
	}
	return newOperationValuePickerModel(
		request,
		ValueProviderFilterDue,
		"Filter by Due or Deferred",
		"keep matching tasks",
		ListSource{Items: items},
		false,
		selected,
	)
}

// newFilterHierarchyPicker is the fh single-select picker over the four
// hierarchy positions, seeded from the active facet when one is set.
func (m *AppModel) newFilterHierarchyPicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	options := []struct {
		id        string
		label     string
		secondary string
		value     HierarchyFilter
	}{
		{id: "parent", label: "Parent", secondary: "has open children", value: HierarchyParent},
		{id: "child", label: "Child", secondary: "sits under a parent", value: HierarchyChild},
		{id: "root", label: "Root", secondary: "has no parent", value: HierarchyRoot},
		{id: "leaf", label: "Leaf", secondary: "has no open children", value: HierarchyLeaf},
	}
	items := make([]ListItem, 0, len(options))
	for _, option := range options {
		items = append(items, ListItem{
			ID:            ListItemID("filter-hierarchy:" + option.id),
			PrimaryText:   option.label,
			SecondaryText: option.secondary,
			Aliases:       []string{option.id, option.label, option.secondary},
			Data:          option.value,
		})
	}
	selected := []ListItemID(nil)
	if m.crossOutliner != nil {
		switch m.crossOutliner.filter.Hierarchy {
		case HierarchyAll:
			selected = nil
		case HierarchyParent:
			selected = []ListItemID{"filter-hierarchy:parent"}
		case HierarchyChild:
			selected = []ListItemID{"filter-hierarchy:child"}
		case HierarchyRoot:
			selected = []ListItemID{"filter-hierarchy:root"}
		case HierarchyLeaf:
			selected = []ListItemID{"filter-hierarchy:leaf"}
		}
	}
	return newOperationValuePickerModel(
		request,
		ValueProviderFilterHierarchy,
		"Filter by Hierarchy",
		"keep matching tasks",
		ListSource{Items: items},
		false,
		selected,
	)
}

// newFilterCompletionAgePicker is the fc single-select picker. Applying a
// completion-age filter also selects both terminal workflow states.
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
		"includes done and canceled tasks",
		ListSource{Items: items},
		false,
		selected,
	)
}

// newFilterStatesPicker is the f$ multi-select picker over the canonical
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
// Context/project scope, and focus remain in force.
func (m *AppModel) prospectiveStateFilterCounts() map[models.WorkflowState]int {
	counts := make(map[models.WorkflowState]int, len(models.AllWorkflowStates()))
	if m == nil || m.crossOutliner == nil {
		return counts
	}

	clauses := make([]models.QueryClause, 0, len(models.AllWorkflowStates()))
	for _, state := range models.AllWorkflowStates() {
		clauses = appendFilterClause(clauses, models.QueryFieldState,
			models.QueryOperatorIs, state.String())
	}
	query := queryWithFields(m.crossOutliner.filterQuery,
		[]models.QueryField{models.QueryFieldState}, clauses)
	return prospectiveOutlinerStateCounts(m.crossOutliner, query)
}

// prospectiveOutlinerStateCounts runs the canonical row projection on an
// isolated model copy. Expanding task trees and sections keeps presentation
// folds from changing facet totals, while rebuilding through the normal mode
// pipeline preserves all actual scope and visibility rules.
func prospectiveOutlinerStateCounts(
	outliner *OutlinerModel,
	query models.Query,
) map[models.WorkflowState]int {
	counts := make(map[models.WorkflowState]int, len(models.AllWorkflowStates()))
	if outliner == nil {
		return counts
	}

	projection := *outliner
	// The copy takes the prospective narrowing through the one writer, so its
	// carriers are derived from the query the count is asked about rather than
	// the one the Outliner is still showing.
	projection.setFilterQuery(query)
	projection.rows = nil
	projection.OpenAtTask = nil
	projection.expandedNodes = make(map[TaskIdentity]bool)
	projection.collapsedSections = make(map[string]bool)
	projection.expandedChecklists = make(map[TaskIdentity]bool)
	projection.focusStack = append([]FocusEntry(nil), outliner.focusStack...)
	projection.crossFocusStack = append(
		[]CrossProjectFocusEntry(nil),
		outliner.crossFocusStack...,
	)
	projection.crossForceRebuild = true
	projection.rebuildRows()

	// A matching descendant can retain a non-matching ancestor as hierarchy
	// context. Count only rows that satisfy the prospective predicate themselves.
	// The facet half alone. Under a search projection the isolated copy carries
	// searchProjection != nil, so its rebuild yields fuzzy-scored rows that the
	// full work-mode predicate would then substring-gate with searchRemainder,
	// undercounting; facetPredicateAt is correct in both branches and applies the
	// facets exactly once.
	now := time.Now()
	matches := projection.facetPredicateAt(now)
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

// newFilterTagsPicker is the f# multi-select picker over the active scope's tag
// vocabulary, seeded from the active tag filter facet. Contexts are a separate
// axis on fc, so the census here reads tags alone and the picker's CONTEXTS
// section drops out with them.
func (m *AppModel) newFilterTagsPicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	tagList := BuildProjectsTagList(m.filterScopeProjects(), false)
	source := taskTagListSource(tagList, false)
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

// filterScopeProjects is the population a row facet's vocabulary is drawn from:
// the loaded scope once there is one, and the whole task workspace before the
// first load.
func (m *AppModel) filterScopeProjects() []*models.Project {
	if m.crossOutliner != nil && len(m.crossOutliner.loadedProjects) > 0 {
		return m.crossOutliner.workModeScopeProjects()
	}
	return m.taskWorkspaceProjects()
}

// newFilterContextsPicker is the fc multi-select picker over the active scope's
// context vocabulary, seeded from the active context filter facet. The rows
// compose with OR because a task carries at most one context, and the leading
// taskCountLabel is the census wording both context rows carry.
func taskCountLabel(count int) string {
	return fmt.Sprintf("%d %s", count, pluralize(count, "task", "tasks"))
}

// no-context row names the tasks the vocabulary does not cover.
func (m *AppModel) newFilterContextsPicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	projects := m.filterScopeProjects()
	facets := BuildTagFacetList(projects, false)
	items := make([]ListItem, 0, len(facets)+1)
	items = append(items, ListItem{
		ID:            ListItemID("filter-context:" + noContextFilterKey),
		PrimaryText:   "No context",
		SecondaryText: taskCountLabel(countTasksWithoutContext(projects)),
		Aliases:       []string{"no context", "none", "##0"},
		Data:          noContextFilterKey,
	})
	for _, facet := range facets {
		if facet.ContextCount == 0 {
			continue
		}
		items = append(items, ListItem{
			ID:            ListItemID("filter-context:" + facet.Name),
			PrimaryText:   "##" + facet.Name,
			SecondaryText: taskCountLabel(facet.ContextCount),
			Aliases:       []string{facet.Name, "##" + facet.Name},
			Data:          facet.Name,
		})
	}
	selected := []ListItemID(nil)
	if m.crossOutliner != nil {
		filter := &m.crossOutliner.filter.CommonFilterState
		if filter.NoContext {
			selected = append(selected, ListItemID("filter-context:"+noContextFilterKey))
		}
		for name := range filter.Contexts {
			selected = append(selected, ListItemID("filter-context:"+name))
		}
	}
	return newOperationValuePickerModel(
		request,
		ValueProviderFilterContexts,
		"Filter by Context",
		"keep matching tasks",
		ListSource{Items: items},
		true,
		selected,
	)
}

// countTasksWithoutContext counts the gap the no-context row names, over the
// same population BuildTagFacetList reads for the rows beside it.
func countTasksWithoutContext(projects []*models.Project) int {
	count := 0
	for _, project := range uniqueProjects(projects) {
		for _, task := range project.Tasks {
			if task != nil && task.Context == "" {
				count++
			}
		}
	}
	return count
}
