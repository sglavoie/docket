package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

const (
	operationTaskSetState        OperationID = "tui.task.set-state"
	operationTaskSetPriority     OperationID = "tui.task.set-priority"
	operationTaskSetDueDate      OperationID = "tui.task.set-due-date"
	operationTaskSetSnooze       OperationID = "tui.task.set-snooze"
	operationTaskSetTags         OperationID = "tui.task.set-tags"
	operationTaskMoveProject     OperationID = "tui.task.move-project"
	operationTaskSetDependencies OperationID = "tui.task.set-dependencies"
	operationTaskSetParent       OperationID = "tui.task.set-parent"
	operationTaskIndent          OperationID = "tui.task.indent"
	operationTaskOutdent         OperationID = "tui.task.outdent"
)

// OperationDateValue is the picker-neutral due/snooze payload. Nil clears the
// field; HasTime is retained for future time-bearing due presets.
type OperationDateValue struct {
	Date    *time.Time
	HasTime bool
}

// OperationParentValue distinguishes "no parent" from an invalid/missing value.
type OperationParentValue struct {
	ParentID *string
}

// OperationValueSelectedMsg resumes a pending operation with the chosen value.
type OperationValueSelectedMsg struct {
	Request  OperationExecutionRequest
	Provider ValueProviderID
	Value    any
}

// OperationValueCanceledMsg closes the value picker and returns to its action
// sheet without executing the operation.
type OperationValueCanceledMsg struct{}

// OperationValuePickerModel is the shared ListSurface value-picker control.
type OperationValuePickerModel struct {
	surface   *ListSurfaceModel
	request   OperationExecutionRequest
	provider  ValueProviderID
	title     string
	subtitle  string
	multi     bool
	searching bool
}

func newOperationValuePickerModel(
	request OperationExecutionRequest,
	provider ValueProviderID,
	title string,
	subtitle string,
	source ListSource,
	multi bool,
	selected []ListItemID,
) (*OperationValuePickerModel, error) {
	mode := ListSelectionSingle
	if multi {
		mode = ListSelectionMulti
	}
	surface, err := source.NewModel(ListSurfaceConfig{
		SelectionMode: mode,
		EmptyState: ListEmptyState{
			Title: "No values available.",
		},
		NoMatchState: ListEmptyState{
			Title: "No matching values.",
		},
	})
	if err != nil {
		return nil, err
	}
	if len(selected) > 0 {
		if multi {
			if err := surface.SetSelectedIDs(selected); err != nil {
				return nil, err
			}
		} else {
			surface.SetCursorToID(selected[0])
		}
	}
	return &OperationValuePickerModel{
		surface:  surface,
		request:  request,
		provider: provider,
		title:    title,
		subtitle: subtitle,
		multi:    multi,
	}, nil
}

func (model *OperationValuePickerModel) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	if model == nil || model.surface == nil {
		return nil, true, OperationValueCanceledMsg{}
	}
	key := msg.String()
	if model.searching {
		switch key {
		case "esc":
			if model.surface.Query().Raw != "" {
				model.surface.SetQuery("")
				return nil, false, nil
			}
			model.searching = false
			return nil, false, nil
		case "enter":
			model.searching = false
			return nil, false, nil
		case "backspace":
			query := model.surface.Query().Raw
			if query != "" {
				_, size := utf8.DecodeLastRuneInString(query)
				model.surface.SetQuery(query[:len(query)-size])
			}
			return nil, false, nil
		default:
			if msg.Text != "" {
				model.surface.SetQuery(model.surface.Query().Raw + msg.Text)
			}
			return nil, false, nil
		}
	}

	switch key {
	case "esc":
		return nil, true, OperationValueCanceledMsg{}
	case "/":
		model.searching = true
	case "up", "k", "ctrl+p":
		model.surface.MoveCursor(-1)
	case "down", "j", "ctrl+n":
		model.surface.MoveCursor(1)
	case "home", "g":
		model.surface.MoveCursorToStart()
	case "end", "G":
		model.surface.MoveCursorToEnd()
	case "pgup":
		model.surface.PageCursor(-1)
	case "pgdown":
		model.surface.PageCursor(1)
	case "space":
		if model.multi {
			if _, err := model.surface.ToggleCurrentSelection(); err != nil {
				return nil, false, StatusMsg(err.Error())
			}
		}
	case "enter":
		value, err := model.selectedValue()
		if err != nil {
			return nil, false, StatusMsg(err.Error())
		}
		return nil, true, OperationValueSelectedMsg{
			Request:  model.request,
			Provider: model.provider,
			Value:    value,
		}
	default:
		if model.provider == ValueProviderPriority && len(key) == 1 && key[0] >= '1' && key[0] <= '5' {
			if model.surface.SetCursorToID(ListItemID("priority:" + key)) {
				value, err := model.selectedValue()
				if err == nil {
					return nil, true, OperationValueSelectedMsg{
						Request: model.request, Provider: model.provider, Value: value,
					}
				}
			}
		}
	}
	return nil, false, nil
}

func (model *OperationValuePickerModel) selectedValue() (any, error) {
	if model.multi {
		items := model.surface.SelectedItems()
		values := make([]string, 0, len(items))
		for _, item := range items {
			value, ok := item.Data.(string)
			if !ok {
				return nil, fmt.Errorf("selected value %s has invalid payload", item.ID)
			}
			values = append(values, value)
		}
		sort.Strings(values)
		return values, nil
	}
	item, ok := model.surface.CurrentItem()
	if !ok {
		return nil, ErrListItemNotFound
	}
	if !item.Enabled() {
		return nil, fmt.Errorf("%w: %s", ErrListItemDisabled, item.DisabledReason)
	}
	return item.Data, nil
}

func (model *OperationValuePickerModel) render(width int) string {
	if model == nil || model.surface == nil {
		return ""
	}
	innerWidth := min(max(width*3/5, 46), max(width-6, 24))
	listHeight := min(max(len(model.surface.Rows()), 6), 15)
	query := model.surface.Query().Raw
	search := "Press / to search"
	if model.searching || query != "" {
		cursor := ""
		if model.searching {
			cursor = "▏"
		}
		search = "Search: " + query + cursor
	}
	content := MutedStyle.Render(search) + "\n\n" +
		RenderCompactListSurface(model.surface, innerWidth-2, listHeight)
	titleText := model.title
	if model.subtitle != "" {
		titleText += ": " + truncateToVisualWidth(model.subtitle, max(innerWidth-20, 12))
	}
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(titleText)
	help := "Enter choose · / search · Esc back"
	if model.multi {
		help = "Space toggle · Enter apply · Esc back"
	}
	return renderTitledBox(
		content,
		innerWidth,
		title,
		HelpStyle.Render(help),
		lipgloss.NewStyle().Foreground(ColorPrimary),
	)
}

func (m *AppModel) taskActionOperationDescriptors() []OperationDescriptor {
	manifest := taskOperationManifest()
	descriptors := make([]OperationDescriptor, 0, len(manifest))
	for _, entry := range manifest {
		if entry.Core {
			continue
		}
		source, cardinality := OperationTargetCurrentTask, TargetCardinalityOne
		if entry.Bulk {
			source, cardinality = OperationTargetSelectedTasks, TargetCardinalityOneOrMore
		}
		id := entry.ID
		descriptors = append(descriptors, OperationDescriptor{
			ID:      id,
			Label:   entry.Label,
			Aliases: entry.Aliases,
			// A manifest action reaches a user through the row it acts on, so it
			// is contextual: the Outliner sheet and the Finder task sheet both
			// project the manifest, and Finder never lists it a second time as a
			// standalone operation aimed at whatever the cursor happens to be on.
			Placement:            PlacementContextual,
			Category:             OperationCategoryTask,
			TargetSource:         source,
			TargetCardinality:    cardinality,
			RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
			Applicability: func(context CommandContext) bool {
				return m.taskActionOperationApplicable(id, context)
			},
			Risk:            entry.Risk,
			ValueProviderID: entry.Provider,
		})
	}
	return descriptors
}

func (m *AppModel) taskActionOperationApplicable(id OperationID, context CommandContext) bool {
	if !coreOutlinerApplicability(context) {
		return false
	}
	entry, _ := taskOperationManifestEntryFor(id)
	entities := []EntityContext{context.Entity}
	if context.Selection.Visual {
		selected := normalizedTaskIdentities(context.Selection.Tasks)
		if len(selected) == 0 || len(context.Selection.Entities) != len(selected) {
			return false
		}
		if !entry.Bulk {
			return false
		}
		entities = context.Selection.Entities
	}
	for _, entity := range entities {
		if entity.TaskID == "" || entity.ProjectID == "" {
			return false
		}
		if entity.Archived && entry.Risk != OperationRiskReadOnly {
			return false
		}
		if context.Selection.Visual && id == operationTaskSetSnooze && !taskStateAllowsSnooze(entity.State) {
			return false
		}
		if !m.taskStructureAllowsOperation(id, entity) {
			return false
		}
		if !m.taskAuxiliaryAllowsOperation(id, entity) {
			return false
		}
	}
	return true
}

func taskStateAllowsSnooze(state string) bool {
	switch models.WorkflowState(state) {
	case models.StateInbox, models.StateReady, models.StateNext,
		models.StateWaiting, models.StatePaused, models.StateReview, models.StateSomeday:
		return true
	default:
		return false
	}
}

// taskStructureAllowsOperation hides destination-dependent actions that the
// domain would reject: an indent with no legal parent above, an outdent at root,
// a reparent with no candidate, or a move with no other active project. A target
// that no longer resolves stays applicable so the handler reports the single
// authoritative "no longer available" error instead of the row vanishing.
func (m *AppModel) taskStructureAllowsOperation(id OperationID, entity EntityContext) bool {
	switch id {
	case operationTaskMoveProject, operationTaskSetParent, operationTaskIndent, operationTaskOutdent:
	default:
		return true
	}
	task, project := m.taskAndProjectForIdentity(TaskIdentity{
		ProjectID: entity.ProjectID,
		TaskID:    entity.TaskID,
	})
	if task == nil || project == nil {
		return true
	}
	switch id {
	case operationTaskMoveProject:
		return len(m.moveProjectCandidates(project)) > 0
	case operationTaskSetParent:
		return task.ParentID != nil || len(buildReparentValidParents(project, task)) > 0
	case operationTaskIndent:
		above := project.GetTaskAbove(task)
		if above == nil {
			return false
		}
		allowed, _ := task.CanIndent(above)
		return allowed
	case operationTaskOutdent:
		allowed, _ := task.CanOutdent()
		return allowed
	default:
		return true
	}
}

// moveProjectCandidates is the single destination set shared by move-project
// applicability and its value picker.
func (m *AppModel) moveProjectCandidates(source *models.Project) []*models.Project {
	candidates := make([]*models.Project, 0)
	for _, candidate := range m.activeProjects() {
		if candidate == nil || candidate.Archived {
			continue
		}
		if source != nil && candidate.ID == source.ID {
			continue
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func (m *AppModel) registerTaskActionOperations() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	executor := m.operationExecutor
	for _, provider := range []ValueProviderID{
		ValueProviderState,
		ValueProviderPriority,
		ValueProviderDueDate,
		ValueProviderSnooze,
		ValueProviderTags,
		ValueProviderProject,
		ValueProviderDependencies,
		ValueProviderParent,
	} {
		if err := executor.RegisterValueProvider(provider, func(OperationInvocation) OperationValueProviderResult {
			// Returning no immediate value asks the shell to construct the
			// provider's ListSurface picker from the frozen task identity.
			return OperationValueProviderResult{}
		}); err != nil {
			return err
		}
	}
	for _, descriptor := range m.taskActionOperationDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
			Applicability:        descriptor.Applicability,
			Handler:              m.executeTaskOperation,
			// Every handler below dispatches to an application-layer coordinator
			// or the existing atomic move message, which already owns commit and
			// rollback policy.
			DelegateTransaction: true,
			// The canonical delete request prompts before it mutates, so a generic
			// executor confirmation in front of it would ask the same question twice.
			DelegateConfirmation: descriptor.ID == operationTaskDelete,
		}); err != nil {
			return err
		}
	}
	return nil
}

// acceptsPresetValue reports whether a key binding may deliver value directly
// instead of opening the provider's picker. It mirrors the type assertions the
// task handlers below make, so a mistyped preset is rejected when the input
// strategy is registered rather than when the user presses the chord.
func (id ValueProviderID) acceptsPresetValue(value any) bool {
	switch id {
	case ValueProviderState:
		_, ok := value.(models.WorkflowState)
		return ok
	case ValueProviderPriority, ValueProviderFilterPriority:
		_, ok := value.(int)
		return ok
	case ValueProviderFilterDue:
		_, ok := value.(DueDateFilter)
		return ok
	case ValueProviderFilterStates, ValueProviderFilterTags:
		_, ok := value.([]string)
		return ok
	case ValueProviderDueDate, ValueProviderSnooze:
		_, ok := value.(OperationDateValue)
		return ok
	case ValueProviderTags, ValueProviderDependencies:
		_, ok := value.([]string)
		return ok
	case ValueProviderProject, ValueProviderNoteTitle, ValueProviderTagName:
		_, ok := value.(string)
		return ok
	case ValueProviderParent:
		_, ok := value.(OperationParentValue)
		return ok
	default:
		return false
	}
}

func (m *AppModel) executeTaskOperation(
	invocation OperationInvocation,
) OperationHandlerResult {
	if len(invocation.Targets.Targets) == 0 {
		return OperationHandlerResult{Err: fmt.Errorf("operation %s requires at least one task", invocation.Descriptor.ID)}
	}
	if invocation.Descriptor.ValueProviderID != "" && !invocation.HasValue {
		return OperationHandlerResult{Err: fmt.Errorf("operation %s requires a value", invocation.Descriptor.ID)}
	}
	// Deletion takes every resolved target at once because one atomic
	// transaction removes single and multi-root selections alike.
	if invocation.Descriptor.ID == operationTaskDelete {
		return m.executeTaskDeleteOperation(invocation)
	}
	if len(invocation.Targets.Targets) > 1 {
		return m.executeBulkTaskValueOperation(invocation)
	}
	target := invocation.Targets.Targets[0]
	identity := target.Task
	if !identity.Valid() {
		return OperationHandlerResult{Err: fmt.Errorf("operation %s resolved an invalid task", invocation.Descriptor.ID)}
	}
	return m.executeSingleTaskOperation(invocation, identity)
}

func (m *AppModel) executeSingleTaskOperation(
	invocation OperationInvocation,
	identity TaskIdentity,
) OperationHandlerResult {
	cmdResult := func(cmd tea.Cmd) OperationHandlerResult {
		return OperationHandlerResult{Message: OperationCommandResultMsg{Cmd: cmd}}
	}

	switch invocation.Descriptor.ID {
	case operationTaskSetState:
		state, ok := invocation.Value.(models.WorkflowState)
		if !ok || !state.IsValid() {
			return OperationHandlerResult{Err: fmt.Errorf("invalid state value")}
		}
		return cmdResult(m.secondaryFeatures.TaskItem.Transition(identity, state))
	case operationTaskSetPriority:
		priority, ok := invocation.Value.(int)
		if !ok || priority < 0 || priority > models.PriorityMax {
			return OperationHandlerResult{Err: fmt.Errorf("invalid priority value")}
		}
		return cmdResult(m.secondaryFeatures.TaskItem.SetPriority(identity, priority))
	case operationTaskSetDueDate:
		value, ok := invocation.Value.(OperationDateValue)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid due-date value")}
		}
		return cmdResult(m.secondaryFeatures.TaskItem.SetDueDate(identity, cloneOperationDate(value.Date), value.HasTime))
	case operationTaskSetSnooze:
		value, ok := invocation.Value.(OperationDateValue)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid snooze value")}
		}
		return cmdResult(m.secondaryFeatures.TaskItem.Snooze(identity, cloneOperationDate(value.Date)))
	case operationTaskSetTags:
		tags, ok := invocation.Value.([]string)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid tag value")}
		}
		return cmdResult(m.secondaryFeatures.Bulk.SetTagsLocators(
			[]app.TaskLocator{identity.Locator()},
			tags,
		))
	case operationTaskMoveProject:
		destination, ok := invocation.Value.(string)
		if !ok || strings.TrimSpace(destination) == "" {
			return OperationHandlerResult{Err: fmt.Errorf("invalid destination project")}
		}
		task, project := m.taskAndProjectForIdentity(identity)
		if task == nil || project == nil {
			return OperationHandlerResult{Err: fmt.Errorf("task is no longer available")}
		}
		return OperationHandlerResult{Message: MoveTaskToProjectMsg{
			Task:          task,
			SourceProject: project,
			DestProjectID: destination,
			FromGlobal:    true,
		}}
	case operationTaskSetDependencies:
		dependencies, ok := invocation.Value.([]string)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid dependencies value")}
		}
		return cmdResult(m.secondaryFeatures.TaskItem.SetDependencies(identity, dependencies))
	case operationTaskSetParent:
		parent, ok := invocation.Value.(OperationParentValue)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid parent value")}
		}
		return cmdResult(m.secondaryFeatures.TaskItem.Reparent(identity, parent.ParentID))
	case operationTaskIndent:
		return cmdResult(m.secondaryFeatures.TaskItem.Indent(identity))
	case operationTaskOutdent:
		return cmdResult(m.secondaryFeatures.TaskItem.Outdent(identity))
	default:
		return m.executeAuxiliaryTaskOperation(invocation, identity)
	}
}

func (m *AppModel) executeBulkTaskValueOperation(
	invocation OperationInvocation,
) OperationHandlerResult {
	locators := make([]app.TaskLocator, 0, len(invocation.Targets.Targets))
	for _, target := range invocation.Targets.Targets {
		if !target.Task.Valid() {
			return OperationHandlerResult{Err: fmt.Errorf("operation %s resolved an invalid task", invocation.Descriptor.ID)}
		}
		locators = append(locators, target.Task.Locator())
	}
	cmdResult := func(cmd tea.Cmd) OperationHandlerResult {
		return OperationHandlerResult{Message: OperationCommandResultMsg{Cmd: cmd}}
	}
	count := len(locators)

	switch invocation.Descriptor.ID {
	case operationTaskSetState:
		state, ok := invocation.Value.(models.WorkflowState)
		if !ok || !state.IsValid() {
			return OperationHandlerResult{Err: fmt.Errorf("invalid state value")}
		}
		req := app.BulkTransitionRequest{
			TaskLocators: locators,
			Target:       state,
			UndoAction:   "Set State",
			UndoContext:  fmt.Sprintf("%d tasks", count),
			SkipInvalid:  true,
		}
		if m.runtime != nil {
			req.Rules = m.runtime.Automation().GetEnabledWorkflowRules()
		}
		return cmdResult(m.secondaryFeatures.Bulk.TransitionWithWorkflow(
			req,
			func(changed int) string {
				return fmt.Sprintf("Changed %d task(s) to %s", changed, state)
			},
		))
	case operationTaskSetPriority:
		priority, ok := invocation.Value.(int)
		if !ok || priority < 0 || priority > models.PriorityMax {
			return OperationHandlerResult{Err: fmt.Errorf("invalid priority value")}
		}
		return cmdResult(m.secondaryFeatures.Bulk.SetPriorityLocators(locators, priority))
	case operationTaskSetDueDate:
		value, ok := invocation.Value.(OperationDateValue)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid due-date value")}
		}
		if value.Date == nil {
			return cmdResult(m.secondaryFeatures.Bulk.ClearDueDateLocators(locators))
		}
		return cmdResult(m.secondaryFeatures.Bulk.SetDueDateLocators(
			locators,
			*cloneOperationDate(value.Date),
			value.HasTime,
		))
	case operationTaskSetSnooze:
		value, ok := invocation.Value.(OperationDateValue)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid snooze value")}
		}
		return cmdResult(m.secondaryFeatures.Bulk.SnoozeLocators(locators, cloneOperationDate(value.Date)))
	case operationTaskSetTags:
		tags, ok := invocation.Value.([]string)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid tag value")}
		}
		return cmdResult(m.secondaryFeatures.Bulk.SetTagsLocators(locators, tags))
	default:
		return OperationHandlerResult{
			Err: fmt.Errorf("operation %s is not available for %d selected tasks", invocation.Descriptor.ID, count),
		}
	}
}

func cloneOperationDate(date *time.Time) *time.Time {
	if date == nil {
		return nil
	}
	cloned := *date
	return &cloned
}

func (m *AppModel) taskAndProjectForIdentity(identity TaskIdentity) (*models.Task, *models.Project) {
	if !identity.Valid() {
		return nil, nil
	}
	project := m.projectByID(identity.ProjectID)
	if project == nil {
		return nil, nil
	}
	return project.GetTaskByID(identity.TaskID), project
}

func (m *AppModel) newOperationValuePicker(
	request OperationExecutionRequest,
	provider ValueProviderID,
) (*OperationValuePickerModel, error) {
	// Filter pickers refine the whole view, so they must open without any task
	// under the cursor and never resolve targets.
	switch provider {
	case ValueProviderFilterStates:
		return m.newFilterStatesPicker(request)
	case ValueProviderFilterTags:
		return m.newFilterTagsPicker(request)
	default:
	}
	refs, err := m.operationTaskRefs(request.Context)
	if err != nil {
		return nil, err
	}
	task, project := refs[0].Task, refs[0].Project
	subtitle := task.Title
	if len(refs) > 1 || request.Context.Selection.Visual {
		subtitle = fmt.Sprintf("%d selected task(s)", len(refs))
	}

	switch provider {
	case ValueProviderState:
		states := make([]models.WorkflowState, 0, len(refs))
		for _, ref := range refs {
			states = append(states, ref.Task.State())
		}
		return NewTaskStatePickerForTargets(request, states, subtitle)
	case ValueProviderPriority:
		priorities := []PriorityInfo{
			{Priority: 1, Description: "Critical"},
			{Priority: 2, Description: "High"},
			{Priority: 3, Description: "Normal"},
			{Priority: 4, Description: "Low"},
			{Priority: 5, Description: "Someday"},
		}
		selected := []ListItemID(nil)
		if priority, ok := commonTaskPriority(refs); ok {
			selected = []ListItemID{ListItemID("priority:" + strconv.Itoa(priority))}
		}
		return newOperationValuePickerModel(
			request,
			provider,
			"Set Priority",
			subtitle,
			ListSourceFromPriorities(priorities),
			false,
			selected,
		)
	case ValueProviderDueDate:
		selected := []ListItemID(nil)
		if date, ok := commonTaskDate(refs, false); ok {
			selected = dueDateSelectedID(date)
		}
		return newOperationValuePickerModel(
			request,
			provider,
			"Set Due Date",
			subtitle,
			operationDateListSource(false),
			false,
			selected,
		)
	case ValueProviderSnooze:
		selected := []ListItemID(nil)
		if date, ok := commonTaskDate(refs, true); ok {
			selected = snoozeSelectedID(date)
		}
		return newOperationValuePickerModel(
			request,
			provider,
			"Set Snooze",
			subtitle,
			operationDateListSource(true),
			false,
			selected,
		)
	case ValueProviderTags:
		projects := make([]*models.Project, 0, len(refs))
		for _, ref := range refs {
			projects = append(projects, ref.Project)
		}
		source := ListSourceFromTags(BuildProjectsTagList(projects, false))
		for index := range source.Items {
			if tag, ok := source.Items[index].Data.(TagInfo); ok {
				source.Items[index].Data = tag.Name
			}
		}
		commonTags := commonTaskTags(refs)
		selected := make([]ListItemID, 0, len(commonTags))
		for _, tag := range commonTags {
			selected = append(selected, ListItemID("tag:"+tag))
		}
		return newOperationValuePickerModel(
			request, provider, "Set Tags", subtitle, source, true, selected,
		)
	case ValueProviderProject:
		if len(refs) != 1 {
			return nil, fmt.Errorf("move project is available for one task at a time")
		}
		source := ListSourceFromProjects(m.moveProjectCandidates(project))
		for index := range source.Items {
			candidate, _ := source.Items[index].Data.(*models.Project)
			if candidate != nil {
				source.Items[index].Data = candidate.ID
			}
		}
		return newOperationValuePickerModel(
			request, provider, "Move Project", subtitle, source, false, nil,
		)
	case ValueProviderDependencies:
		if len(refs) != 1 {
			return nil, fmt.Errorf("dependencies are available for one task at a time")
		}
		source, selected := dependencyOperationListSource(task, project)
		return newOperationValuePickerModel(
			request, provider, "Set Dependencies", subtitle, source, true, selected,
		)
	case ValueProviderParent:
		if len(refs) != 1 {
			return nil, fmt.Errorf("reparenting is available for one task at a time")
		}
		source, selected := parentOperationListSource(task, project)
		return newOperationValuePickerModel(
			request, provider, "Set Parent", subtitle, source, false, selected,
		)
	default:
		return nil, fmt.Errorf("unknown operation value provider %q", provider)
	}
}

func (m *AppModel) operationTaskRefs(context CommandContext) ([]TaskProjectRef, error) {
	identities := normalizedTaskIdentities(context.Selection.Tasks)
	if len(identities) == 0 {
		if context.Selection.Visual {
			return nil, fmt.Errorf("select at least one task")
		}
		if current := currentTaskIdentity(context); current.Valid() {
			identities = []TaskIdentity{current}
		}
	}
	refs := make([]TaskProjectRef, 0, len(identities))
	for _, identity := range identities {
		task, project := m.taskAndProjectForIdentity(identity)
		if task == nil || project == nil {
			return nil, fmt.Errorf("selected task is no longer available")
		}
		refs = append(refs, TaskProjectRef{Task: task, Project: project})
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("select a task")
	}
	return refs, nil
}

func commonTaskPriority(refs []TaskProjectRef) (int, bool) {
	if len(refs) == 0 || refs[0].Task == nil {
		return 0, false
	}
	priority := refs[0].Task.Priority
	for _, ref := range refs[1:] {
		if ref.Task == nil || ref.Task.Priority != priority {
			return 0, false
		}
	}
	return priority, true
}

func commonTaskDate(refs []TaskProjectRef, snooze bool) (*time.Time, bool) {
	if len(refs) == 0 || refs[0].Task == nil {
		return nil, false
	}
	dateFor := func(task *models.Task) *time.Time {
		if snooze {
			return task.SnoozedUntil
		}
		return task.DueDate
	}
	first := dateFor(refs[0].Task)
	for _, ref := range refs[1:] {
		if ref.Task == nil || !sameOperationDate(first, dateFor(ref.Task)) {
			return nil, false
		}
	}
	return cloneOperationDate(first), true
}

func sameOperationDate(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func commonTaskTags(refs []TaskProjectRef) []string {
	if len(refs) == 0 || refs[0].Task == nil {
		return nil
	}
	common := make(map[string]bool)
	for _, tag := range models.DedupeTags(refs[0].Task.Tags) {
		common[tag] = true
	}
	for _, ref := range refs[1:] {
		present := make(map[string]bool)
		if ref.Task != nil {
			for _, tag := range models.DedupeTags(ref.Task.Tags) {
				present[tag] = true
			}
		}
		for tag := range common {
			if !present[tag] {
				delete(common, tag)
			}
		}
	}
	tags := make([]string, 0, len(common))
	for tag := range common {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

func operationDateListSource(snooze bool) ListSource {
	today := util.StartOfDay(time.Now())
	type preset struct {
		id    string
		label string
		days  int
		clear bool
	}
	presets := []preset{
		{id: "clear", label: "Clear", clear: true},
		{id: "today", label: "Today"},
		{id: "tomorrow", label: "Tomorrow", days: 1},
		{id: "week", label: "In one week", days: 7},
		{id: "month", label: "In one month", days: 30},
	}
	if snooze {
		presets = []preset{
			{id: "clear", label: "Clear snooze", clear: true},
			{id: "tomorrow", label: "Until tomorrow", days: 1},
			{id: "week", label: "For one week", days: 7},
			{id: "month", label: "For one month", days: 30},
		}
	}
	items := make([]ListItem, 0, len(presets))
	for _, preset := range presets {
		var date *time.Time
		secondary := "remove date"
		if !preset.clear {
			value := today.AddDate(0, 0, preset.days)
			date = &value
			secondary = value.Format("Mon Jan 2")
		}
		items = append(items, ListItem{
			ID:            ListItemID("date:" + preset.id),
			PrimaryText:   preset.label,
			SecondaryText: secondary,
			Aliases:       []string{preset.id, secondary},
			Data:          OperationDateValue{Date: date},
		})
	}
	return ListSource{Items: items}
}

func dueDateSelectedID(date *time.Time) []ListItemID {
	if date == nil {
		return []ListItemID{"date:clear"}
	}
	return relativeDateSelectedID(date, false)
}

func snoozeSelectedID(date *time.Time) []ListItemID {
	if date == nil {
		return []ListItemID{"date:clear"}
	}
	return relativeDateSelectedID(date, true)
}

func relativeDateSelectedID(date *time.Time, snooze bool) []ListItemID {
	today := util.StartOfDay(time.Now())
	days := int(util.StartOfDay(*date).Sub(today).Hours() / 24)
	switch days {
	case 0:
		if !snooze {
			return []ListItemID{"date:today"}
		}
	case 1:
		return []ListItemID{"date:tomorrow"}
	case 7:
		return []ListItemID{"date:week"}
	case 30:
		return []ListItemID{"date:month"}
	}
	return nil
}

func dependencyOperationListSource(
	task *models.Task,
	project *models.Project,
) (ListSource, []ListItemID) {
	candidates := buildDependencyCandidates(task, project)
	items := make([]ListItem, 0, len(candidates))
	available := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		if candidate.Task == nil {
			continue
		}
		ref := candidate.Task.ID
		available[ref] = true
		items = append(items, ListItem{
			ID:            ListItemID("dependency:" + ref),
			PrimaryText:   fmt.Sprintf("#%d %s", candidate.Task.ShortID, candidate.Task.Title),
			SecondaryText: workflowStateListLabel(candidate.Task.State()),
			Aliases:       append([]string{candidate.Task.Title, candidate.Task.State().String()}, candidate.Task.Tags...),
			Preview: ListPreview{
				Title:   candidate.Task.Title,
				Summary: candidate.Task.Description,
			},
			Data: ref,
		})
	}
	// Keep qualified or otherwise non-local existing dependencies selectable so
	// confirming the picker without edits cannot silently drop an external edge.
	for _, dependency := range task.DependsOn {
		if available[dependency] {
			continue
		}
		available[dependency] = true
		items = append(items, ListItem{
			ID:            ListItemID("dependency:" + dependency),
			PrimaryText:   dependency,
			SecondaryText: "existing external dependency",
			Aliases:       []string{dependency},
			Data:          dependency,
		})
	}
	selected := make([]ListItemID, 0, len(task.DependsOn))
	for _, dependency := range task.DependsOn {
		selected = append(selected, ListItemID("dependency:"+dependency))
	}
	return ListSource{Items: items}, selected
}

func parentOperationListSource(
	task *models.Task,
	project *models.Project,
) (ListSource, []ListItemID) {
	items := []ListItem{{
		ID:            "parent:none",
		PrimaryText:   "No parent",
		SecondaryText: "top level",
		Data:          OperationParentValue{},
	}}
	for _, parent := range buildReparentValidParents(project, task) {
		parentID := parent.ID
		items = append(items, ListItem{
			ID:            ListItemID("parent:" + parent.ID),
			PrimaryText:   fmt.Sprintf("#%d %s", parent.ShortID, parent.Title),
			SecondaryText: workflowStateListLabel(parent.State()),
			Aliases:       []string{parent.Title, parent.State().String()},
			Data:          OperationParentValue{ParentID: &parentID},
		})
	}
	selected := []ListItemID{"parent:none"}
	if task.ParentID != nil {
		selected = []ListItemID{ListItemID("parent:" + *task.ParentID)}
	}
	return ListSource{Items: items}, selected
}

func (m *AppModel) executeOperationOverlayRequest(
	request OperationExecutionRequest,
) tea.Cmd {
	if m.operationExecutor == nil {
		m.err = fmt.Errorf("operation registry is unavailable")
		return statusTimeout(5 * time.Second)
	}
	result := m.operationExecutor.Execute(request)
	if result.Err != nil {
		m.err = result.Err
		return statusTimeout(5 * time.Second)
	}
	if result.ValueRequired != nil {
		return m.openOperationValueStep(*result.ValueRequired)
	}
	return operationResultCmd(result)
}

// openOperationValueStep opens the value overlay one provider needs: a typed
// prompt for text values, the ListSurface picker for everything else.
func (m *AppModel) openOperationValueStep(request OperationValueRequestMsg) tea.Cmd {
	if request.Provider.textEntry() {
		prompt, err := m.newOperationTextPrompt(request.Request, request.Provider)
		if err != nil {
			m.err = err
			return statusTimeout(5 * time.Second)
		}
		m.operationTextPrompt = prompt
		return prompt.initCmd()
	}
	picker, err := m.newOperationValuePicker(request.Request, request.Provider)
	if err != nil {
		m.err = err
		return statusTimeout(5 * time.Second)
	}
	m.operationValuePicker = picker
	return nil
}

func (m *AppModel) clearOperationValueStep() {
	m.operationValuePicker = nil
	m.operationTextPrompt = nil
}

// operationResultCmd converts a completed execution outcome into the Bubble Tea
// command seam.
func operationResultCmd(result OperationExecutionResult) tea.Cmd {
	switch typed := result.AsMessage().(type) {
	case nil:
		return nil
	case OperationCommandResultMsg:
		return typed.Cmd
	default:
		return func() tea.Msg { return typed }
	}
}

func (m *AppModel) handleOperationOverlayMessage(msg tea.Msg) (bool, tea.Cmd) {
	switch typed := msg.(type) {
	case ContextActionSheetResultMsg:
		return true, m.applyContextActionSheetResult(typed)
	case ContextActionSheetClosedMsg:
		m.contextActionSheet = nil
		return true, nil
	case OperationConfirmationRequestMsg:
		m.showOperationConfirmation(typed)
		return true, nil
	case OperationValueRequestMsg:
		return true, m.openOperationValueStep(typed)
	case OperationValueSelectedMsg:
		m.clearOperationValueStep()
		request := typed.Request
		request.Value = typed.Value
		request.HasValue = true
		return true, m.resumeContextActionRequest(request)
	case OperationValueCanceledMsg:
		m.clearOperationValueStep()
		return true, nil
	default:
		return false, nil
	}
}

// applyContextActionSheetResult applies the nested step or completion produced
// by one centralized sheet execution. A failure is already retained on the
// sheet, a pending value or confirmation opens beneath it, and only a completed
// action reports the host lifecycle.
func (m *AppModel) applyContextActionSheetResult(msg ContextActionSheetResultMsg) tea.Cmd {
	switch {
	case msg.Result.Err != nil:
		return nil
	case msg.Result.ValueRequired != nil:
		return m.openOperationValueStep(*msg.Result.ValueRequired)
	case msg.Result.ConfirmationRequired != nil:
		m.showOperationConfirmation(*msg.Result.ConfirmationRequired)
		return nil
	}
	cmd := operationResultCmd(msg.Result)
	if msg.Completion != ContextActionCompleteCloseHost {
		return cmd
	}
	completed := ContextActionSheetCompletedMsg{Host: msg.Host, Cmd: cmd}
	return func() tea.Msg { return completed }
}

// resumeContextActionRequest re-enters the executor after a nested value or
// confirmation step. An open sheet keeps ownership of the outcome so a failure
// stays visible on it and only a completed action dismisses it.
func (m *AppModel) resumeContextActionRequest(request OperationExecutionRequest) tea.Cmd {
	sheet := m.contextActionSheet
	if sheet == nil {
		return m.executeOperationOverlayRequest(request)
	}
	result, dismissed := sheet.Execute(request)
	if dismissed {
		m.contextActionSheet = nil
	}
	return m.applyContextActionSheetResult(ContextActionSheetResultMsg{
		Host:       sheet.config.Host,
		Completion: sheet.config.Completion,
		Request:    request,
		Result:     result,
	})
}

const operationConfirmAction = "contextual-operation"

// showOperationConfirmation prompts for a destructive contextual action and
// freezes the executor-supplied request, which already carries Confirmed.
func (m *AppModel) showOperationConfirmation(request OperationConfirmationRequestMsg) {
	m.pendingOperationRequest = request.Request
	prompt := request.Label + "?"
	if count := len(request.Targets.Entities()); count > 1 {
		prompt = fmt.Sprintf("%s · %d targets?", request.Label, count)
	}
	m.operationConfirm.Show(operationConfirmAction, prompt)
}

// handleOperationConfirmKey gives the contextual destructive confirmation modal
// key priority: it consumes every key while active so nothing leaks into the
// sheet beneath, and cancellation leaves that sheet exactly as it was.
func (m *AppModel) handleOperationConfirmKey(key string) (bool, tea.Cmd) {
	if !m.operationConfirm.IsActive() {
		return false, nil
	}
	confirmed, handled := m.operationConfirm.HandleKey(key)
	if !handled {
		return false, nil
	}
	if confirmed {
		request := m.pendingOperationRequest
		m.pendingOperationRequest = OperationExecutionRequest{}
		return true, m.resumeContextActionRequest(request)
	}
	switch key {
	case "n", "N", "esc":
		m.pendingOperationRequest = OperationExecutionRequest{}
	}
	return true, nil
}
