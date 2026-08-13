package tui

import (
	"fmt"
	"sort"
	"strings"
)

const (
	taskActionCategoryPlan          ListCategoryID = "task-actions:plan"
	taskActionCategoryOrganize      ListCategoryID = "task-actions:organize"
	taskActionCategoryContent       ListCategoryID = "task-actions:content"
	taskActionCategoryRelationships ListCategoryID = "task-actions:relationships"
	taskActionCategoryDanger        ListCategoryID = "task-actions:danger"
)

var taskActionCategories = []ListCategory{
	{ID: taskActionCategoryContent, Label: "Content"},
	{ID: taskActionCategoryDanger, Label: "Danger"},
}

// NewTaskActionSheetModel builds the Outliner task sheet on the shared
// contextual engine. The CommandContext carries stable task and scope
// identities, so cursor changes beneath the modal cannot retarget the eventual
// operation.
func NewTaskActionSheetModel(
	registry *OperationRegistry,
	executor *OperationExecutor,
	context CommandContext,
	taskTitle string,
) (*ContextActionSheetModel, error) {
	descriptors := TaskActionSheetDescriptors(registry, context)
	ids := make([]OperationID, 0, len(descriptors))
	for _, descriptor := range descriptors {
		ids = append(ids, descriptor.ID)
	}
	title := "Task Actions"
	if context.Selection.Visual {
		title = "Bulk Actions"
	}
	return NewContextActionSheetModel(registry, executor, context, ids, ContextActionSheetConfig{
		Host:         ContextActionHostOutliner,
		Completion:   ContextActionCompleteCloseSheet,
		Title:        title,
		Subtitle:     strings.TrimSpace(taskTitle),
		ItemIDPrefix: "task-action:",
		Categories:   taskActionCategories,
		CategoryFor:  taskActionCategory,
		SecondaryFor: operationActionSecondaryText,
		DisabledFor: func(descriptor OperationDescriptor) string {
			return taskActionDisabledReason(descriptor, context)
		},
		ImmediateFilter: true,
		Targets:         taskActionTargetCount(context),
		Bulk:            context.Selection.Visual,
		EmptyState: ListEmptyState{
			Title:  "No task actions available.",
			Detail: "Select a task that can be acted on.",
		},
		NoMatchState: ListEmptyState{
			Title:  "No matching actions.",
			Detail: "Clear or change the search.",
		},
	})
}

// TaskActionSheetDescriptors returns the deterministic selected-task operation
// projection used by the sheet and its tests.
func TaskActionSheetDescriptors(
	registry *OperationRegistry,
	context CommandContext,
) []OperationDescriptor {
	if registry == nil || taskActionTargetCount(context) == 0 {
		return []OperationDescriptor{}
	}
	descriptors := make([]OperationDescriptor, 0)
	for _, descriptor := range registry.All() {
		if !taskActionSheetEligible(descriptor) ||
			!taskActionDescriptorApplicableToEveryTarget(descriptor, context) {
			continue
		}
		// The manifest Bulk flag is the single bulk-eligibility rule: an action
		// reaches a multi-target sheet only when a multi-task coordinator or the
		// canonical delete transaction can carry every frozen selected task.
		if context.Selection.Visual && !taskOperationSupportsBulk(descriptor.ID) {
			continue
		}
		if _, err := (OperationTargetResolver{}).Resolve(descriptor, context); err != nil {
			continue
		}
		descriptors = append(descriptors, descriptor)
	}
	sortTaskActionDescriptors(descriptors)
	return descriptors
}

func taskActionTargetCount(context CommandContext) int {
	if context.Selection.Visual {
		return len(normalizedTaskIdentities(context.Selection.Tasks))
	}
	if currentTaskIdentity(context).Valid() {
		return 1
	}
	return 0
}

func taskActionDescriptorApplicableToEveryTarget(
	descriptor OperationDescriptor,
	context CommandContext,
) bool {
	if !descriptor.Applicable(context) {
		return false
	}
	if !context.Selection.Visual {
		return true
	}
	selected := normalizedTaskIdentities(context.Selection.Tasks)
	if len(selected) == 0 || len(context.Selection.Entities) != len(selected) {
		return false
	}
	for _, entity := range context.Selection.Entities {
		targetContext := cloneCommandContext(context)
		targetContext.Entity = entity
		if !descriptor.Applicable(targetContext) {
			return false
		}
	}
	return true
}

func taskActionSheetEligible(descriptor OperationDescriptor) bool {
	// The task manifest is an allowlist, not merely an ordering hint. Keeping
	// eligibility closed over its three menu entries prevents newly registered
	// task operations from silently expanding the Space menu.
	entry, manifested := taskOperationManifestEntryFor(descriptor.ID)
	return manifested && !entry.DirectOnly
}

func taskActionCategory(descriptor OperationDescriptor) ListCategoryID {
	if descriptor.Risk == OperationRiskDestructive {
		return taskActionCategoryDanger
	}
	if entry, manifested := taskOperationManifestEntryFor(descriptor.ID); manifested {
		return entry.Category
	}
	return taskActionCategoryContent
}

func operationActionSecondaryText(descriptor OperationDescriptor) string {
	switch {
	case descriptor.ValueProviderID != "":
		return "choose " + strings.ReplaceAll(string(descriptor.ValueProviderID), "-", " ")
	case descriptor.Risk == OperationRiskDestructive:
		return "confirmation required"
	case descriptor.DirectKey != "":
		return descriptor.DirectKey
	default:
		return ""
	}
}

func taskActionDisabledReason(
	descriptor OperationDescriptor,
	context CommandContext,
) string {
	if descriptor.ID == operationTaskSetDefer {
		switch context.Entity.State {
		case "done", "canceled":
			return "terminal tasks cannot be deferred"
		}
	}
	if descriptor.Risk == OperationRiskMutation && context.Entity.Archived {
		return "project is archived"
	}
	return ""
}

// sortTaskActionDescriptors groups by action-sheet category, then puts manifest
// operations in their authoritative order ahead of any other action in the same
// category.
func sortTaskActionDescriptors(descriptors []OperationDescriptor) {
	categoryRank := func(id ListCategoryID) int {
		for index, category := range taskActionCategories {
			if category.ID == id {
				return index
			}
		}
		return len(taskActionCategories)
	}
	sort.SliceStable(descriptors, func(i, j int) bool {
		left, right := descriptors[i], descriptors[j]
		leftCategory := categoryRank(taskActionCategory(left))
		rightCategory := categoryRank(taskActionCategory(right))
		if leftCategory != rightCategory {
			return leftCategory < rightCategory
		}
		leftManifest := taskOperationManifestRank(left.ID)
		rightManifest := taskOperationManifestRank(right.ID)
		if leftManifest != rightManifest {
			return leftManifest < rightManifest
		}
		leftLabel, rightLabel := strings.ToLower(left.Label), strings.ToLower(right.Label)
		if leftLabel != rightLabel {
			return leftLabel < rightLabel
		}
		return left.ID < right.ID
	})
}

func cloneCommandContext(context CommandContext) CommandContext {
	context.Capabilities = cloneCapabilitySet(context.Capabilities)
	context.Selection.Tasks = append([]TaskIdentity(nil), context.Selection.Tasks...)
	context.Selection.Entities = append([]EntityContext(nil), context.Selection.Entities...)
	context.Selection.EntityIdentities = append(
		[]EntityIdentity(nil),
		context.Selection.EntityIdentities...,
	)
	context.ScopeProjectIDs = append([]string(nil), context.ScopeProjectIDs...)
	return context
}

func (m *AppModel) openTaskActionSheet(context CommandContext) error {
	if context.Selection.Visual {
		refs, err := m.operationTaskRefs(context)
		if err != nil {
			return err
		}
		if len(refs) == 0 {
			return fmt.Errorf("select at least one task to open bulk actions")
		}
		context.Selection.Tasks = make([]TaskIdentity, 0, len(refs))
		context.Selection.Entities = make([]EntityContext, 0, len(refs))
		for _, ref := range refs {
			context.Selection.Tasks = append(context.Selection.Tasks, ref.Identity())
			context.Selection.Entities = append(
				context.Selection.Entities,
				taskOperationEntity(ref.Task, ref.Project),
			)
		}
		// operationTaskRefs can drop rows the selection still names, so the typed
		// identities are rederived rather than inherited; otherwise the frozen
		// sheet would act on a wider set than it lists.
		context.Selection.EntityIdentities = taskEntityIdentities(context.Selection.Tasks)
		context.Entity = context.Selection.Entities[0]
		subtitle := fmt.Sprintf("%d tasks", len(refs))
		if context.Selection.VisibleCount+context.Selection.HiddenCount == len(refs) {
			subtitle = fmt.Sprintf(
				"%d tasks · %d visible · %d hidden",
				len(refs),
				context.Selection.VisibleCount,
				context.Selection.HiddenCount,
			)
		}
		sheet, err := NewTaskActionSheetModel(
			m.GetOperationRegistry(),
			m.operationExecutor,
			context,
			subtitle,
		)
		if err != nil {
			return err
		}
		m.contextActionSheet = sheet
		return nil
	}
	task, _ := m.getCurrentCursorTaskAndProject()
	if task == nil {
		return fmt.Errorf("select a task to open actions")
	}
	sheet, err := NewTaskActionSheetModel(
		m.GetOperationRegistry(),
		m.operationExecutor,
		context,
		task.Title,
	)
	if err != nil {
		return err
	}
	m.contextActionSheet = sheet
	return nil
}
