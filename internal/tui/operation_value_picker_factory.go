package tui

import (
	"fmt"
	"strconv"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/entityops"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/operationhost"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskdetail"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/valuepicker"
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
) (*valuepicker.OperationValuePickerModel, error) {
	// Filter pickers refine the whole view, so they must open without any task
	// under the cursor and never resolve task targets.
	switch provider {
	case operation.ValueProviderFilterStates:
		return m.filters.NewFilterStatesPicker(m, request)
	case operation.ValueProviderFilterTags:
		return m.filters.NewFilterTagsPicker(m, request)
	case operation.ValueProviderFilterContexts:
		return m.filters.NewFilterContextsPicker(m, request)
	case operation.ValueProviderFilterDue:
		return m.filters.NewFilterDuePicker(m, request)
	case operation.ValueProviderFilterCompletionAge:
		return m.filters.NewFilterCompletionAgePicker(m, request)
	case operation.ValueProviderFilterHierarchy:
		return m.filters.NewFilterHierarchyPicker(m, request)
	case operation.ValueProviderFilterDependency:
		return m.filters.NewFilterDependencyPicker(m, request)
	case operation.ValueProviderFilterProjectTags:
		return m.filters.NewFilterProjectTagsPicker(m, request)
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
		return valuepicker.NewTaskStatePickerForTargets(request, states, subtitle)
	case operation.ValueProviderPriority:
		priorities := []chrome.PriorityInfo{
			{Priority: 0, Description: "Unset"},
			{Priority: 1, Description: "High"},
			{Priority: 2, Description: "Medium"},
			{Priority: 3, Description: "Low"},
		}
		selected := []listsurface.ListItemID(nil)
		if priority, ok := valuepicker.CommonTaskPriority(refs); ok {
			selected = []listsurface.ListItemID{listsurface.ListItemID("priority:" + strconv.Itoa(priority))}
		}
		return valuepicker.NewOperationValuePickerModel(
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
		due, dueShared := valuepicker.CommonTaskDate(refs, false)
		deferred, deferShared := valuepicker.CommonTaskDate(refs, true)
		if dueShared && deferShared {
			selected = valuepicker.OperationDateSelectedID(due, deferred)
		}
		return valuepicker.NewOperationValuePickerModel(
			request,
			provider,
			"Set Due or Defer",
			subtitle,
			valuepicker.OperationDateListSource(deferRefusalReason(refs)),
			false,
			selected,
		)
	case operation.ValueProviderDueDate:
		selected := []listsurface.ListItemID(nil)
		if date, ok := valuepicker.CommonTaskDate(refs, false); ok {
			selected = valuepicker.DueDateSelectedID(date)
		}
		return valuepicker.NewOperationValuePickerModel(
			request,
			provider,
			"Set Due Date",
			subtitle,
			valuepicker.OperationDueDateListSource(),
			false,
			selected,
		)
	case operation.ValueProviderParent:
		return valuepicker.NewOperationValuePickerModel(
			request,
			provider,
			"Set Parent",
			subtitle,
			valuepicker.OperationParentListSource(refs[0].Task, refs[0].Project),
			false,
			valuepicker.ParentSelectedID(refs[0].Task),
		)
	case operation.ValueProviderChild:
		// Nothing is preselected: the checked set is the adoption, so a preselected
		// row would make unchecking it read as a detach the `<child` sigil cannot
		// express.
		return valuepicker.NewOperationValuePickerModel(
			request,
			provider,
			"Adopt Children",
			subtitle+" · adds children, never detaches",
			valuepicker.OperationChildListSource(refs[0].Task, refs[0].Project),
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
		source := valuepicker.OperationTaskReferenceListSource(tasks, m.crossOutliner.DependencyDetailProjects())
		picker, err := valuepicker.NewOperationValuePickerModel(
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
		byID := make(map[listsurface.ListItemID]valuepicker.OperationValueMembership, len(source.Items))
		for _, item := range source.Items {
			byID[item.ID] = valuepicker.OperationValueNone
		}
		for id, state := range valuepicker.TaskReferenceMembership(tasks) {
			if _, ok := byID[id]; ok {
				byID[id] = state
			}
		}
		if err := picker.ConfigureValueMembership(byID); err != nil {
			return nil, err
		}
		return picker, nil
	case operation.ValueProviderProject:
		return valuepicker.NewOperationValuePickerModel(
			request,
			provider,
			"Move to Project",
			subtitle,
			valuepicker.OperationProjectListSource(m.domain.runtime.AvailableProjects(), valuepicker.CommonTaskProjectID(refs)),
			false,
			nil,
		)
	case operation.ValueProviderTags:
		projects := make([]*models.Project, 0, len(refs))
		for _, ref := range refs {
			projects = append(projects, ref.Project)
		}
		tagList := query.BuildProjectsTagList(projects, false)
		source := valuepicker.TaskTagListSource(tagList, true)
		// The context rows lead the CONTEXTS section, ahead of the tag rows the
		// source recategorized there by context usage.
		source.Items = append(
			valuepicker.TaskContextListItems(query.CollectContextNamesForProjects(projects)), source.Items...,
		)
		picker, err := valuepicker.NewOperationValuePickerModel(
			request, provider, "Set Tags & Context", subtitle, source, true, nil,
		)
		if err != nil {
			return nil, err
		}
		// Membership is configured at every cardinality, so the picker always
		// emits the patch payload the context field rides on. For one task the
		// patch and a replacement describe the same edit.
		byID := make(map[listsurface.ListItemID]valuepicker.OperationValueMembership, len(source.Items))
		for _, item := range source.Items {
			byID[item.ID] = valuepicker.OperationValueNone
		}
		for tag, state := range valuepicker.TaskTagMembership(refs) {
			id := listsurface.ListItemID("tag:" + tag)
			if _, ok := byID[id]; ok {
				byID[id] = state
			}
		}
		contextInitial, contextStates := valuepicker.TaskContextMembership(refs)
		for id, state := range contextStates {
			if _, ok := byID[id]; ok {
				byID[id] = state
			}
		}
		picker.ContextInitial = contextInitial
		if err := picker.ConfigureValueMembership(byID); err != nil {
			return nil, err
		}
		return picker, nil
	default:
		return nil, fmt.Errorf("unknown operation value provider %q", provider)
	}
}

// newOperationTextPrompt builds the prompt from the frozen target identity, so a
// target that already vanished fails here instead of after the user types.
func (m *AppModel) newOperationTextPrompt(
	request operation.OperationExecutionRequest,
	provider operation.ValueProviderID,
) (*operationhost.OperationTextPromptModel, error) {
	identity := operation.CurrentEntityIdentity(request.Context)
	label := string(provider)
	if descriptor, ok := m.GetOperationRegistry().Descriptor(request.ID); ok {
		label = descriptor.Label
	}
	switch provider {
	case operation.ValueProviderNoteTitle:
		note, err := m.resolveNoteTarget(identity)
		if err != nil {
			return nil, err
		}
		return operationhost.NewOperationTextPromptModel(
			request, label, note.Note.Title, note.Note.Title, "new note title",
			operationhost.NameTextPromptOptions(),
		), nil
	case operation.ValueProviderTagName:
		// A tag name applies across a whole selection, so there is no single
		// current value to prefill or show.
		return operationhost.NewOperationTextPromptModel(
			request, label, "", "", "tag name", operationhost.NameTextPromptOptions(),
		), nil
	case operation.ValueProviderChecklistItem:
		// Adding appends a new item, so only an edit has a current value; the
		// request's own operation decides which, not the cursor.
		current := ""
		if request.ID == operation.OperationChecklistEditItem {
			current = m.entities.ChecklistItemTextForContext(m, request.Context)
		}
		return operationhost.NewOperationTextPromptModel(
			request, label, current, current, "checklist item", operationhost.NameTextPromptOptions(),
		), nil
	case operation.ValueProviderResumeNote:
		refs, err := m.operationTaskRefs(request.Context)
		if err != nil {
			return nil, err
		}
		task := refs[0].Task
		// The task title is the subtitle rather than the stored note: the note is
		// already prefilled into the buffer, so echoing it above would show the
		// same sentence twice and leave the row it belongs to unnamed.
		return operationhost.NewOperationTextPromptModel(
			request, label, task.Title, task.ResumeNote, "where you left off (blank clears)",
			operationhost.ResumeNoteTextPromptOptions(),
		), nil
	case operation.ValueProviderComment:
		refs, err := m.operationTaskRefs(request.Context)
		if err != nil {
			return nil, err
		}
		// The subtitle names the task this comment is being written on and the
		// buffer opens empty, because appending never edits an existing comment.
		// Resolving the target here is what freezes it: a refresh or a deletion
		// while the popup is open cannot redirect the text somewhere else.
		return operationhost.NewOperationTextPromptModel(
			request, label, refs[0].Task.Title, "", "comment text",
			operationhost.CommentTextPromptOptions(taskdetail.CommentCharLimit),
		), nil
	case operation.ValueProviderProjectComment:
		// The project is resolved here for the same reason the task is above: that
		// is what freezes the owner, so a refresh, a rename, or a delete while the
		// popup is open cannot redirect the text to another project. The identity
		// carried by the request is what a retry reopens onto, so a project that has
		// already vanished refuses here rather than silently borrowing the cursor's.
		if m.projectByID(identity.ProjectID) == nil {
			return nil, fmt.Errorf("a project comment needs a project target")
		}
		return m.projectCommentPrompt(request), nil
	case operation.ValueProviderLensName:
		// Saving the current view mints a new record, so only a rename has a
		// current value. The request's own operation decides which, because the
		// cursor names a lens in both cases.
		current := ""
		if request.ID == lens.OperationLensRename {
			lens, err := m.resolveLensTarget(identity)
			if err != nil {
				return nil, err
			}
			current = lens.Name
		}
		return operationhost.NewOperationTextPromptModel(
			request, label, current, current, "lens name", operationhost.NameTextPromptOptions(),
		), nil
	default:
		return nil, fmt.Errorf("unknown operation text value provider %q", provider)
	}
}

// surfaceOperationGates is the inventory of entity surfaces the one-operation
// grammar governs.
//
// It stays in the shell rather than beside ValidateOperationOnePath or inside
// the operation host, because every entry names a surface strategy and a
// surface validator and those live above both — validateOutlinerOperationOnePath
// reads the Outliner manifests the shell declares. The host takes the table as
// an argument instead of reaching for it.
func surfaceOperationGates() []operationhost.SurfaceGate {
	return []operationhost.SurfaceGate{
		{Strategy: SimplifiedInputStrategy(), Validate: validateOutlinerOperationOnePath},
		{Strategy: projecthub.ProjectHubInputStrategy(), Validate: projecthub.ValidateProjectOperationOnePath},
		{Strategy: notes.NotesInputStrategy(), Validate: notes.ValidateNotesOperationOnePath},
		{Strategy: entityops.AttachmentsInputStrategy(), Validate: entityops.ValidateAttachmentOperationOnePath},
		{Strategy: entityops.CommentsInputStrategy(), Validate: entityops.ValidateCommentOperationOnePath},
	}
}

// SimplifiedInputStrategy re-exports the accepted daily-loop grammar from the
// operation layer that now declares it. The shell keeps the name because it is
// one of the five identifiers `internal/tui`'s two external importers compile
// against, and a package's exported surface is a contract independent of which
// file inside the subtree happens to hold the declaration.
func SimplifiedInputStrategy() operation.InputStrategy {
	return operation.SimplifiedInputStrategy()
}
