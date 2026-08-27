package tui

import (
	"fmt"
	"slices"
	"strings"
)

const (
	operationSortDueDate   OperationID = "tui.sort.due"
	operationSortPriority  OperationID = "tui.sort.priority"
	operationSortCreatedAt OperationID = "tui.sort.created"
	operationSortUpdatedAt OperationID = "tui.sort.updated"
	operationSortTitle     OperationID = "tui.sort.title"
	operationSortClear     OperationID = "tui.sort.clear"
)

// sortFieldOperations maps each order-axis operation to the field it installs.
// One descriptor per field rather than one preset-valued operation, so the axis
// needs no value provider and cannot grow a sort picker.
var sortFieldOperations = map[OperationID]SortField{
	operationSortDueDate:   SortByDueDate,
	operationSortPriority:  SortByPriority,
	operationSortCreatedAt: SortByCreatedAt,
	operationSortUpdatedAt: SortByUpdatedAt,
	operationSortTitle:     SortByTitle,
	operationSortClear:     SortByDefault,
}

// sortOperationDescriptor is the shape every order-axis key shares: no target,
// no value, no transaction, and applicability decided by the Outliner alone.
func sortOperationDescriptor(id OperationID, label string, aliases ...string) OperationDescriptor {
	return OperationDescriptor{
		ID:                   id,
		Label:                label,
		Aliases:              aliases,
		Category:             OperationCategorySort,
		TargetCardinality:    TargetCardinalityNone,
		RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
		Applicability:        coreOutlinerApplicability,
		Risk:                 OperationRiskReadOnly,
	}
}

func sortAxisOperationDescriptors() []OperationDescriptor {
	descriptor := sortOperationDescriptor
	return []OperationDescriptor{
		descriptor(operationSortDueDate, "Sort by Due Date", "order by due date"),
		descriptor(operationSortPriority, "Sort by Priority", "order by priority"),
		descriptor(operationSortCreatedAt, "Sort by Created", "order by creation date"),
		descriptor(operationSortUpdatedAt, "Sort by Updated", "order by last touch"),
		descriptor(operationSortTitle, "Sort by Title", "order alphabetically"),
		descriptor(operationSortClear, "Clear Sort", "restore the mode's own order"),
	}
}

func (m *AppModel) registerSortOperations() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	for _, descriptor := range sortAxisOperationDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities:     NewCapabilitySet(CapabilitySimplifiedInput),
			Applicability:            descriptor.Applicability,
			Handler:                  m.executeSortOperation,
			DelegateTargetResolution: true,
			DelegateConfirmation:     true,
			// The order axis mutates session view state only, so no workspace
			// transaction is opened.
			DelegateTransaction: true,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (m *AppModel) executeSortOperation(invocation OperationInvocation) OperationHandlerResult {
	out := m.crossOutliner
	if out == nil {
		return OperationHandlerResult{Err: fmt.Errorf("the Outliner is unavailable")}
	}
	field, known := sortFieldOperations[invocation.Descriptor.ID]
	if !known {
		return OperationHandlerResult{Err: fmt.Errorf("unknown sort operation %s", invocation.Descriptor.ID)}
	}

	if invocation.Descriptor.ID == operationSortClear {
		out.sort.Reset()
		out.treeSort.Reset()
		out.crossForceRebuild = true
		rebuildOutlinerPreservingCursor(out)
		return OperationHandlerResult{Status: "Sort cleared"}
	}

	// A fixed-order mode owns its ordering, so the key records the choice on the
	// durable tree sort and the mode transition installs it on return. A search
	// projection is flat and owns no mode order at all, so the mode underneath it
	// is irrelevant there and the sort installs directly.
	mode := out.activeOutlinerModeEntry()
	if mode.FixedOrder() && !out.hasSearchProjection() {
		out.treeSort.Toggle(field)
		return OperationHandlerResult{Status: fmt.Sprintf(
			"%s owns its order · sort applies in All, Inbox, Backlog", mode.Label)}
	}

	out.sort.Toggle(field)
	out.treeSort = out.sort
	out.crossForceRebuild = true
	rebuildOutlinerPreservingCursor(out)
	status := "Sorting by " + sortStateLabel(out.sort)
	if exempt := out.manualOrderSectionNames(); len(exempt) > 0 {
		status += " · " + strings.Join(exempt, ", ") + " keep manual order"
	}
	return OperationHandlerResult{Status: status}
}

// manualOrderSectionNames names the sections the freshly rebuilt projection
// holds that the sort deliberately skipped. It reads the rows rather than the
// mode catalog, so the status can never promise an exemption the screen does
// not show.
func (m *OutlinerModel) manualOrderSectionNames() []string {
	if m == nil {
		return nil
	}
	names := make([]string, 0, 4)
	for _, row := range m.rows {
		if !row.IsSectionHeader || !rowKeepsManualOrder(row) || row.SectionName == "" {
			continue
		}
		if !slices.Contains(names, row.SectionName) {
			names = append(names, row.SectionName)
		}
	}
	return names
}

func sortStateLabel(state SortState) string {
	direction := "↑"
	if !state.Ascending {
		direction = "↓"
	}
	return state.Field.String() + " " + direction
}
