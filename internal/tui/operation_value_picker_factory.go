package tui

import (
	"fmt"
	"strconv"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func (m *AppModel) taskAndProjectForIdentity(identity taskref.TaskIdentity) (*models.Task, *models.Project) {
	if !identity.Valid() {
		return nil, nil
	}
	project := m.projectByID(identity.ProjectID)
	if project == nil {
		return nil, nil
	}
	return project.GetTaskByID(identity.TaskID), project
}

//nolint:gocyclo // One factory exhaustively maps typed operation values to their picker contracts.
func (m *AppModel) newOperationValuePicker(
	request operation.OperationExecutionRequest,
	provider operation.ValueProviderID,
) (*OperationValuePickerModel, error) {
	// Filter pickers refine the whole view, so they must open without any task
	// under the cursor and never resolve task targets.
	switch provider {
	case operation.ValueProviderFilterStates:
		return m.newFilterStatesPicker(request)
	case operation.ValueProviderFilterTags:
		return m.newFilterTagsPicker(request)
	case operation.ValueProviderFilterContexts:
		return m.newFilterContextsPicker(request)
	case operation.ValueProviderFilterDue:
		return m.newFilterDuePicker(request)
	case operation.ValueProviderFilterCompletionAge:
		return m.newFilterCompletionAgePicker(request)
	case operation.ValueProviderFilterHierarchy:
		return m.newFilterHierarchyPicker(request)
	case operation.ValueProviderFilterDependency:
		return m.newFilterDependencyPicker(request)
	case operation.ValueProviderFilterProjectTags:
		return m.newFilterProjectTagsPicker(request)
	case operation.ValueProviderProjectTagDestination:
		return m.newProjectTagDestinationPicker(request)
	case operation.ValueProviderProjectTags:
		// A project tag edit resolves a project, not a task, so it must open from
		// a project header where no task ref exists.
		return m.newProjectTagsPicker(request)
	case operation.ValueProviderLensTemplate:
		// The built-in view catalog is global, so the gallery opens from the lens
		// picker where there is no task under the cursor at all.
		return m.newLensTemplatePicker(request)
	default:
	}
	refs, err := m.operationTaskRefs(request.Context)
	if err != nil {
		return nil, err
	}
	task := refs[0].Task
	subtitle := task.Title
	if len(refs) > 1 || request.Context.Selection.Visual {
		subtitle = fmt.Sprintf("%d selected task(s)", len(refs))
	}

	switch provider {
	case operation.ValueProviderState:
		states := make([]models.WorkflowState, 0, len(refs))
		for _, ref := range refs {
			states = append(states, ref.Task.State())
		}
		return NewTaskStatePickerForTargets(request, states, subtitle)
	case operation.ValueProviderPriority:
		priorities := []chrome.PriorityInfo{
			{Priority: 0, Description: "Unset"},
			{Priority: 1, Description: "High"},
			{Priority: 2, Description: "Medium"},
			{Priority: 3, Description: "Low"},
		}
		selected := []listsurface.ListItemID(nil)
		if priority, ok := commonTaskPriority(refs); ok {
			selected = []listsurface.ListItemID{listsurface.ListItemID("priority:" + strconv.Itoa(priority))}
		}
		return newOperationValuePickerModel(
			request,
			provider,
			"Set Priority",
			subtitle,
			listsurface.ListSourceFromPriorities(priorities),
			false,
			selected,
		)
	case operation.ValueProviderDate:
		selected := []listsurface.ListItemID(nil)
		due, dueShared := commonTaskDate(refs, false)
		deferred, deferShared := commonTaskDate(refs, true)
		if dueShared && deferShared {
			selected = operationDateSelectedID(due, deferred)
		}
		return newOperationValuePickerModel(
			request,
			provider,
			"Set Due or Defer",
			subtitle,
			operationDateListSource(deferRefusalReason(refs)),
			false,
			selected,
		)
	case operation.ValueProviderDueDate:
		selected := []listsurface.ListItemID(nil)
		if date, ok := commonTaskDate(refs, false); ok {
			selected = dueDateSelectedID(date)
		}
		return newOperationValuePickerModel(
			request,
			provider,
			"Set Due Date",
			subtitle,
			operationDueDateListSource(),
			false,
			selected,
		)
	case operation.ValueProviderParent:
		return newOperationValuePickerModel(
			request,
			provider,
			"Set Parent",
			subtitle,
			operationParentListSource(refs[0].Task, refs[0].Project),
			false,
			parentSelectedID(refs[0].Task),
		)
	case operation.ValueProviderChild:
		// Nothing is preselected: the checked set is the adoption, so a preselected
		// row would make unchecking it read as a detach the `<child` sigil cannot
		// express.
		return newOperationValuePickerModel(
			request,
			provider,
			"Adopt Children",
			subtitle+" · adds children, never detaches",
			operationChildListSource(refs[0].Task, refs[0].Project),
			true,
			nil,
		)
	case operation.ValueProviderTaskReference:
		// The stored set is preselected where the adoption picker above preselects
		// nothing, and the asymmetry is a grammar fact rather than a preference:
		// `~` declares both `~0` and `-~task`, so unchecking a row is a removal the
		// sigil has a form for, while `<child` declares no Clears at all.
		//
		// The population is the one Peek and `~` autocomplete already read, not
		// AvailableProjects, whose task trees are not loaded — so the keyboard and
		// the typed routes can never disagree about what is addressable.
		tasks := make([]*models.Task, 0, len(refs))
		for _, ref := range refs {
			tasks = append(tasks, ref.Task)
		}
		source := operationTaskReferenceListSource(tasks, m.crossOutliner.DependencyDetailProjects())
		picker, err := newOperationValuePickerModel(
			request, provider, "Set Dependencies", subtitle, source, true, nil,
		)
		if err != nil {
			return nil, err
		}
		// Membership is configured at every cardinality, so the picker always emits
		// a difference rather than a set. For one task the two describe the same
		// edit; for a selection only the difference can leave a row the tasks
		// disagree about alone. The preselection is therefore the baseline the
		// commit diffs against rather than something merely displayed: the overlay
		// outlives the snapshot it was built from, and the difference is what
		// survives that.
		byID := make(map[listsurface.ListItemID]operationValueMembership, len(source.Items))
		for _, item := range source.Items {
			byID[item.ID] = operationValueNone
		}
		for id, state := range taskReferenceMembership(tasks) {
			if _, ok := byID[id]; ok {
				byID[id] = state
			}
		}
		if err := picker.configureValueMembership(byID); err != nil {
			return nil, err
		}
		return picker, nil
	case operation.ValueProviderProject:
		return newOperationValuePickerModel(
			request,
			provider,
			"Move to Project",
			subtitle,
			operationProjectListSource(m.runtime.AvailableProjects(), commonTaskProjectID(refs)),
			false,
			nil,
		)
	case operation.ValueProviderTags:
		projects := make([]*models.Project, 0, len(refs))
		for _, ref := range refs {
			projects = append(projects, ref.Project)
		}
		tagList := query.BuildProjectsTagList(projects, false)
		source := taskTagListSource(tagList, true)
		// The context rows lead the CONTEXTS section, ahead of the tag rows the
		// source recategorized there by context usage.
		source.Items = append(
			taskContextListItems(query.CollectContextNamesForProjects(projects)), source.Items...,
		)
		picker, err := newOperationValuePickerModel(
			request, provider, "Set Tags & Context", subtitle, source, true, nil,
		)
		if err != nil {
			return nil, err
		}
		// Membership is configured at every cardinality, so the picker always
		// emits the patch payload the context field rides on. For one task the
		// patch and a replacement describe the same edit.
		byID := make(map[listsurface.ListItemID]operationValueMembership, len(source.Items))
		for _, item := range source.Items {
			byID[item.ID] = operationValueNone
		}
		for tag, state := range taskTagMembership(refs) {
			id := listsurface.ListItemID("tag:" + tag)
			if _, ok := byID[id]; ok {
				byID[id] = state
			}
		}
		contextInitial, contextStates := taskContextMembership(refs)
		for id, state := range contextStates {
			if _, ok := byID[id]; ok {
				byID[id] = state
			}
		}
		picker.contextInitial = contextInitial
		if err := picker.configureValueMembership(byID); err != nil {
			return nil, err
		}
		return picker, nil
	default:
		return nil, fmt.Errorf("unknown operation value provider %q", provider)
	}
}
