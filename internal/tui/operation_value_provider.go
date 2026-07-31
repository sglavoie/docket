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

// OperationDateValue is the picker-neutral due/snooze payload. Nil clears the field.
type OperationDateValue struct {
	Date *time.Time
}

// OperationRecurrenceValue distinguishes an explicit clear (nil Pattern) from
// canceling the editor, which never resumes the frozen operation request.
type OperationRecurrenceValue struct {
	Pattern *models.RecurrencePattern
}

// OperationParentValue distinguishes "no parent" from an invalid/missing value.
type OperationParentValue struct {
	ParentID *string
}

// OperationTagsValue is the tri-state bulk tag edit emitted by the value
// picker. Add and Remove are uniform patches; tags in neither set preserve
// their existing per-task membership.
type OperationTagsValue struct {
	Add    []string
	Remove []string
}

type operationTagMembership uint8

const (
	operationTagNone operationTagMembership = iota
	operationTagSome
	operationTagAll
)

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
	surface    *ListSurfaceModel
	request    OperationExecutionRequest
	provider   ValueProviderID
	title      string
	subtitle   string
	multi      bool
	searching  bool
	tagInitial map[ListItemID]operationTagMembership
	tagCurrent map[ListItemID]operationTagMembership
	tagAdder   TagAdder
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
	model := &OperationValuePickerModel{
		surface:  surface,
		request:  request,
		provider: provider,
		title:    title,
		subtitle: subtitle,
		multi:    multi,
	}
	if provider == ValueProviderTags {
		model.tagAdder = NewTagAdder()
	}
	return model, nil
}

//nolint:gocyclo // Centralized key dispatch preserves picker, freeform, and tag-input precedence.
func (model *OperationValuePickerModel) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	if model == nil || model.surface == nil {
		return nil, true, OperationValueCanceledMsg{}
	}
	if model.provider == ValueProviderTags && model.tagAdder.AddTagMode {
		handled, cmd := model.tagAdder.handleAddTagInputKey(msg, model.addOrSelectTag)
		if handled {
			return cmd, false, nil
		}
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
	case "a":
		if model.provider == ValueProviderTags {
			return model.tagAdder.EnterAddMode(), false, nil
		}
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
			if err := model.toggleCurrentMultiValue(); err != nil {
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

func (model *OperationValuePickerModel) addOrSelectTag(tag string) error {
	id := ListItemID("tag:" + tag)
	items := model.surface.Items()
	for _, item := range items {
		if item.ID == id {
			model.surface.SetQuery("")
			return model.selectTagForAll(id)
		}
	}

	source := ListSourceFromTags([]TagInfo{{Name: tag}})
	if len(source.Items) != 1 {
		return fmt.Errorf("building tag row for %q", tag)
	}
	item := source.Items[0]
	item.Data = tag
	items = append(items, item)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].PrimaryText < items[j].PrimaryText
	})

	model.surface.SetQuery("")
	if err := model.surface.SetItems(items); err != nil {
		return err
	}
	if model.tagInitial != nil {
		model.tagInitial[id] = operationTagNone
		model.tagCurrent[id] = operationTagAll
	}
	return model.selectTagForAll(id)
}

func (model *OperationValuePickerModel) selectTagForAll(id ListItemID) error {
	if model.tagCurrent != nil {
		model.tagCurrent[id] = operationTagAll
	}
	if !model.surface.IsSelected(id) {
		selected := model.surface.SelectedIDs()
		selected = append(selected, id)
		if err := model.surface.SetSelectedIDs(selected); err != nil {
			return err
		}
	}
	if err := model.enforceExclusiveTaskMarker(id); err != nil {
		return err
	}
	if model.tagCurrent != nil {
		if err := model.syncTagMembershipRows(); err != nil {
			return err
		}
	}
	if !model.surface.SetCursorToID(id) {
		return fmt.Errorf("%w: %s", ErrListItemNotFound, id)
	}
	return nil
}

func (model *OperationValuePickerModel) toggleCurrentMultiValue() error {
	if model.provider != ValueProviderTags || len(model.tagCurrent) == 0 {
		_, err := model.surface.ToggleCurrentSelection()
		return err
	}
	item, ok := model.surface.CurrentItem()
	if !ok {
		return ErrListItemNotFound
	}
	state := model.tagCurrent[item.ID]
	if state == operationTagAll {
		if _, err := model.surface.ToggleCurrentSelection(); err != nil {
			return err
		}
		model.tagCurrent[item.ID] = operationTagNone
	} else {
		if !model.surface.IsSelected(item.ID) {
			if _, err := model.surface.ToggleCurrentSelection(); err != nil {
				return err
			}
		}
		model.tagCurrent[item.ID] = operationTagAll
		if err := model.enforceExclusiveTaskMarker(item.ID); err != nil {
			return err
		}
	}
	return model.syncTagMembershipRows()
}

// enforceExclusiveTaskMarker clears every other semantic marker when target is
// selected for all tasks. In the tri-state bulk picker, changing Some/All to
// None also emits the removals needed to keep each resulting task valid.
func (model *OperationValuePickerModel) enforceExclusiveTaskMarker(target ListItemID) error {
	items := model.surface.Items()
	var targetIsMarker bool
	markerIDs := make(map[ListItemID]bool)
	for _, item := range items {
		tag, ok := item.Data.(string)
		if !ok {
			continue
		}
		if _, semantic := models.ParseTaskMarkerTag(tag); semantic {
			markerIDs[item.ID] = true
			if item.ID == target {
				targetIsMarker = true
			}
		}
	}
	if !targetIsMarker {
		return nil
	}
	if model.tagCurrent != nil {
		for id := range markerIDs {
			if id != target {
				model.tagCurrent[id] = operationTagNone
			}
		}
	}

	selected := model.surface.SelectedIDs()
	next := make([]ListItemID, 0, len(selected))
	for _, id := range selected {
		if id == target {
			next = append(next, id)
			continue
		}
		if markerIDs[id] {
			continue
		}
		next = append(next, id)
	}
	return model.surface.SetSelectedIDs(next)
}

func (model *OperationValuePickerModel) configureTagMembership(
	membership map[ListItemID]operationTagMembership,
) error {
	if model == nil || model.surface == nil {
		return nil
	}
	model.tagInitial = make(map[ListItemID]operationTagMembership, len(membership))
	model.tagCurrent = make(map[ListItemID]operationTagMembership, len(membership))
	selected := make([]ListItemID, 0, len(membership))
	for id, state := range membership {
		model.tagInitial[id] = state
		model.tagCurrent[id] = state
		if state == operationTagAll {
			selected = append(selected, id)
		}
	}
	if err := model.surface.SetSelectedIDs(selected); err != nil {
		return err
	}
	return model.syncTagMembershipRows()
}

func (model *OperationValuePickerModel) syncTagMembershipRows() error {
	items := model.surface.Items()
	for index := range items {
		items[index].Indeterminate = model.tagCurrent[items[index].ID] == operationTagSome
	}
	return model.surface.SetItems(items)
}

func (model *OperationValuePickerModel) selectedValue() (any, error) {
	if model.provider == ValueProviderTags && len(model.tagCurrent) > 0 {
		value := OperationTagsValue{}
		for _, item := range model.surface.Items() {
			initial := model.tagInitial[item.ID]
			current := model.tagCurrent[item.ID]
			switch {
			case current == operationTagAll && initial != operationTagAll:
				tag, ok := item.Data.(string)
				if !ok {
					return nil, fmt.Errorf("selected value %s has invalid payload", item.ID)
				}
				value.Add = append(value.Add, tag)
			case current == operationTagNone && initial != operationTagNone:
				tag, ok := item.Data.(string)
				if !ok {
					return nil, fmt.Errorf("selected value %s has invalid payload", item.ID)
				}
				value.Remove = append(value.Remove, tag)
			}
		}
		sort.Strings(value.Add)
		sort.Strings(value.Remove)
		return value, nil
	}
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
	if model.provider == ValueProviderTags && model.tagAdder.AddTagMode {
		content := MutedStyle.Render(fmt.Sprintf(
			"Enter a tag name (max %d characters)",
			model.tagAdder.AddTagInput.CharLimit,
		)) + "\n\n" + model.tagAdder.AddTagInput.View()
		if model.tagAdder.AddTagError != "" {
			content += "\n\n" + BoldStyle.Foreground(ColorError).Render(model.tagAdder.AddTagError)
		}
		title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Add New Tag")
		return renderOverlayBox(
			content,
			innerWidth,
			title,
			HelpStyle.Render("Enter add tag · Esc cancel"),
			lipgloss.NewStyle().Foreground(ColorPrimary),
		)
	}
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
		RenderCompactListSurface(model.surface, innerWidth-4, listHeight)
	titleText := model.title
	if model.subtitle != "" {
		titleText += ": " + truncateToVisualWidth(model.subtitle, max(innerWidth-20, 12))
	}
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(titleText)
	help := "Enter choose · / search · Esc back"
	if model.multi {
		help = "Space toggle · Enter apply · Esc back"
	}
	if model.provider == ValueProviderTags {
		help = "Space toggle · a add tag · Enter apply · Esc back"
	}
	return renderOverlayBox(
		content,
		innerWidth,
		title,
		HelpStyle.Render(help),
		lipgloss.NewStyle().Foreground(ColorPrimary),
	)
}

func (m *AppModel) taskActionOperationDescriptors() []OperationDescriptor {
	manifest := taskOperationDescriptorManifest()
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
			ID:                   id,
			Label:                entry.Label,
			Aliases:              entry.Aliases,
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
	// Direct operations describe a stable grammar, not a menu projection. They
	// stay resolvable over headers and otherwise invalid cursor positions so the
	// executor can report the canonical missing-target error. A visual
	// selection still suppresses single-target operations instead of silently
	// falling back to the cursor.
	if entry.DirectOnly {
		if context.Selection.Visual {
			return entry.Bulk
		}
		return true
	}
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
		models.StateWaiting, models.StatePaused, models.StateSomeday:
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
		ValueProviderRecurrence,
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
		if _, ok := value.([]string); ok {
			return true
		}
		_, ok := value.(OperationTagsValue)
		return id == ValueProviderTags && ok
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
	if invocation.Descriptor.ID == operationTaskTogglePin {
		return m.executeTaskTogglePinOperation(invocation)
	}
	if request, action, summary, handled, err := applicationTaskOperationRequest(invocation); handled {
		if err != nil {
			return OperationHandlerResult{Err: err}
		}
		return OperationHandlerResult{Message: OperationCommandResultMsg{
			Cmd: m.secondaryFeatures.TaskItem.ExecuteOperation(request, action, summary),
		}}
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

// executeTaskTogglePinOperation applies one deterministic toggle to the whole
// frozen target set: if every target is pinned it unpins all of them, otherwise
// it pins all of them. The application operation validates and commits the
// project-qualified set atomically.
func (m *AppModel) executeTaskTogglePinOperation(
	invocation OperationInvocation,
) OperationHandlerResult {
	locators := make([]app.TaskLocator, 0, len(invocation.Targets.Targets))
	allPinned := true
	for _, target := range invocation.Targets.Targets {
		if !target.Task.Valid() {
			return OperationHandlerResult{Err: fmt.Errorf(
				"operation %s resolved an invalid task",
				invocation.Descriptor.ID,
			)}
		}
		task, _ := m.taskAndProjectForIdentity(target.Task)
		if task == nil {
			return OperationHandlerResult{Err: fmt.Errorf("task is no longer available")}
		}
		allPinned = allPinned && task.Pinned
		locators = append(locators, target.Task.Locator())
	}
	pinned := !allPinned
	action, verb := "Pin Task", "Pinned"
	if !pinned {
		action, verb = "Unpin Task", "Unpinned"
	}
	return OperationHandlerResult{Message: OperationCommandResultMsg{
		Cmd: m.secondaryFeatures.TaskItem.ExecuteOperation(
			app.TaskOperationRequest{
				Operation: app.TaskOperationSetPinned,
				Targets:   locators,
				Pinned:    pinned,
			},
			action,
			fmt.Sprintf("%s %d task(s)", verb, len(locators)),
		),
	}}
}

func applicationTaskOperationRequest(
	invocation OperationInvocation,
) (app.TaskOperationRequest, string, string, bool, error) {
	locators := make([]app.TaskLocator, 0, len(invocation.Targets.Targets))
	for _, target := range invocation.Targets.Targets {
		if !target.Task.Valid() {
			return app.TaskOperationRequest{}, "", "", true, fmt.Errorf(
				"operation %s resolved an invalid task",
				invocation.Descriptor.ID,
			)
		}
		locators = append(locators, target.Task.Locator())
	}
	request := app.TaskOperationRequest{Targets: locators}
	count := len(locators)

	switch invocation.Descriptor.ID {
	case operationTaskSetState:
		state, ok := invocation.Value.(models.WorkflowState)
		if !ok || !state.IsValid() {
			return request, "", "", true, fmt.Errorf("invalid state value")
		}
		request.Operation = app.TaskOperationSetState
		request.State = state
		return request, "Change State",
			fmt.Sprintf("Change State · Changed %d task(s) to %s", count, state), true, nil

	case operationTaskSetPriority:
		priority, ok := invocation.Value.(int)
		if !ok || priority < 0 || priority > models.PriorityMax {
			return request, "", "", true, fmt.Errorf("invalid priority value")
		}
		request.Operation = app.TaskOperationSetPriority
		request.Priority = priority
		return request, "Set Priority",
			fmt.Sprintf("Set Priority · Set priority on %d task(s)", count), true, nil

	case operationTaskSetDueDate:
		value, ok := invocation.Value.(OperationDateValue)
		if !ok {
			return request, "", "", true, fmt.Errorf("invalid due-date value")
		}
		request.Operation = app.TaskOperationSetDueDate
		request.Date = cloneOperationDate(value.Date)
		verb := "Set due date on"
		if value.Date == nil {
			verb = "Cleared due date on"
		}
		return request, "Set Due Date",
			fmt.Sprintf("Set Due Date · %s %d task(s)", verb, count), true, nil

	case operationTaskSetSnooze:
		value, ok := invocation.Value.(OperationDateValue)
		if !ok {
			return request, "", "", true, fmt.Errorf("invalid snooze value")
		}
		request.Operation = app.TaskOperationSetSnooze
		request.Date = cloneOperationDate(value.Date)
		verb := "Snoozed"
		if value.Date == nil {
			verb = "Cleared snooze on"
		}
		return request, "Set Snooze",
			fmt.Sprintf("Set Snooze · %s %d task(s)", verb, count), true, nil

	case operationTaskSetTags:
		switch value := invocation.Value.(type) {
		case []string:
			request.Tags = append([]string(nil), value...)
		case OperationTagsValue:
			request.TagPatch = true
			request.TagsToAdd = append([]string(nil), value.Add...)
			request.TagsToRemove = append([]string(nil), value.Remove...)
		default:
			return request, "", "", true, fmt.Errorf("invalid tag value")
		}
		request.Operation = app.TaskOperationSetTags
		return request, "Set Tags",
			fmt.Sprintf("Set Tags · Tagged %d task(s)", count), true, nil

	case operationTaskPin, operationTaskUnpin:
		request.Operation = app.TaskOperationSetPinned
		request.Pinned = invocation.Descriptor.ID == operationTaskPin
		action := "Unpin Task"
		verb := "Unpinned"
		if request.Pinned {
			action = "Pin Task"
			verb = "Pinned"
		}
		return request, action, fmt.Sprintf("%s %d task(s)", verb, count), true, nil

	case operationTaskCopyTitle:
		request.Operation = app.TaskOperationCopyTitle
		return request, "Copy Title", "", true, nil

	case operationTaskCopyDetails:
		request.Operation = app.TaskOperationCopyDescription
		return request, "Copy Description", "", true, nil

	case operationTaskCopyTitleDescription:
		request.Operation = app.TaskOperationCopyTitleDescription
		return request, "Copy Title and Description", "", true, nil

	default:
		return app.TaskOperationRequest{}, "", "", false, nil
	}
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
		return cmdResult(m.secondaryFeatures.TaskItem.SetDueDate(identity, cloneOperationDate(value.Date)))
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
	case operationTaskRecurrence:
		value, ok := invocation.Value.(OperationRecurrenceValue)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid recurrence value")}
		}
		if value.Pattern == nil {
			return cmdResult(m.secondaryFeatures.TaskItem.ClearRecurrence(identity))
		}
		if err := value.Pattern.Validate(); err != nil {
			return OperationHandlerResult{Err: fmt.Errorf("invalid recurrence value: %w", err)}
		}
		return cmdResult(m.secondaryFeatures.TaskItem.SetRecurrence(identity, value.Pattern))
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
		return cmdResult(m.secondaryFeatures.Bulk.TransitionLocators(
			locators,
			state,
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

//nolint:gocyclo // One factory exhaustively maps typed operation values to their picker contracts.
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
	case ValueProviderFilterDue:
		return m.newFilterDuePicker(request)
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
		membership := taskTagMembership(refs)
		selected := make([]ListItemID, 0, len(membership))
		for tag, state := range membership {
			if state == operationTagAll {
				selected = append(selected, ListItemID("tag:"+tag))
			}
		}
		picker, err := newOperationValuePickerModel(
			request, provider, "Set Tags", subtitle, source, true, selected,
		)
		if err != nil {
			return nil, err
		}
		if request.Context.Selection.Visual {
			byID := make(map[ListItemID]operationTagMembership, len(membership))
			for tag, state := range membership {
				byID[ListItemID("tag:"+tag)] = state
			}
			for _, item := range picker.surface.Items() {
				if _, ok := byID[item.ID]; !ok {
					byID[item.ID] = operationTagNone
				}
			}
			if err := picker.configureTagMembership(byID); err != nil {
				return nil, err
			}
		}
		return picker, nil
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
		source, selected := m.dependencyOperationListSource(task, project)
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

func taskTagMembership(refs []TaskProjectRef) map[string]operationTagMembership {
	counts := make(map[string]int)
	for _, ref := range refs {
		if ref.Task == nil {
			continue
		}
		for _, tag := range models.DedupeTags(ref.Task.Tags) {
			counts[tag]++
		}
	}
	membership := make(map[string]operationTagMembership, len(counts))
	for tag, count := range counts {
		if count == len(refs) {
			membership[tag] = operationTagAll
		} else if count > 0 {
			membership[tag] = operationTagSome
		}
	}
	return membership
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

func (m *AppModel) dependencyOperationListSource(
	task *models.Task,
	owner *models.Project,
) (ListSource, []ListItemID) {
	if task == nil || owner == nil {
		return ListSource{}, nil
	}
	projects := m.activeProjects()
	resolver := app.NewWorkspaceResolver(app.NewSnapshot(projects, nil, nil))
	selectedIdentities, unresolved := dependencySelectionIdentities(task.DependsOn, owner, projects)
	descendants := taskDescendantIDs(task)
	items := make([]ListItem, 0)
	selected := make([]ListItemID, 0, len(task.DependsOn))
	available := make(map[string]bool)

	for _, project := range projects {
		if project == nil || project.Archived {
			continue
		}
		for _, candidate := range project.Tasks {
			if candidate == nil || (project.ID == owner.ID && candidate.ID == task.ID) {
				continue
			}
			if project.ID == owner.ID && descendants[candidate.ID] {
				continue
			}
			ref := dependencyReference(owner, project, candidate.ID)
			identity := TaskIdentity{ProjectID: project.ID, TaskID: candidate.ID}
			isSelected := selectedIdentities[identity]
			if !isSelected && (candidate.IsComplete() ||
				resolver.WouldCreateDependencyCycle(owner, task.ID, ref)) {
				continue
			}
			id := ListItemID("dependency:" + ref)
			available[ref] = true
			secondary := project.Name + " · " + workflowStateListLabel(candidate.State())
			items = append(items, ListItem{
				ID:            id,
				PrimaryText:   fmt.Sprintf("#%d %s", candidate.ShortID, candidate.Title),
				SecondaryText: secondary,
				Aliases: append(
					[]string{candidate.Title, candidate.State().String(), project.Name, project.Slug},
					candidate.Tags...,
				),
				Preview: ListPreview{
					Title:   project.Name + " › " + candidate.Title,
					Summary: candidate.Description,
				},
				Data: ref,
			})
			if isSelected {
				selected = append(selected, id)
			}
		}
	}

	// Surface unresolved pre-existing references so the user can explicitly
	// remove them. The application boundary rejects them if they remain selected.
	for _, dependency := range unresolved {
		if available[dependency] {
			continue
		}
		items = append(items, ListItem{
			ID:            ListItemID("dependency:" + dependency),
			PrimaryText:   dependency,
			SecondaryText: "unresolved existing dependency",
			Aliases:       []string{dependency},
			Data:          dependency,
		})
		selected = append(selected, ListItemID("dependency:"+dependency))
	}
	return ListSource{Items: items}, selected
}

func dependencyReference(owner, target *models.Project, taskID string) string {
	if owner != nil && target != nil && owner.ID == target.ID {
		return taskID
	}
	return models.TaskRef{ProjectID: target.ID, TaskID: taskID}.String()
}

func dependencySelectionIdentities(
	dependencies []string,
	owner *models.Project,
	projects []*models.Project,
) (map[TaskIdentity]bool, []string) {
	selected := make(map[TaskIdentity]bool, len(dependencies))
	var unresolved []string
	for _, dependency := range dependencies {
		parsed := models.ParseTaskRef(dependency)
		project := owner
		if parsed.IsCrossProject() {
			project = nil
			for _, candidate := range projects {
				if candidate != nil && (candidate.ID == parsed.ProjectID || candidate.Slug == parsed.ProjectID) {
					project = candidate
					break
				}
			}
		}
		if project == nil || project.GetTaskByID(parsed.TaskID) == nil {
			unresolved = append(unresolved, dependency)
			continue
		}
		selected[TaskIdentity{ProjectID: project.ID, TaskID: parsed.TaskID}] = true
	}
	return selected, unresolved
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
	if request.Provider == ValueProviderRecurrence {
		refs, err := m.operationTaskRefs(request.Request.Context)
		if err != nil || len(refs) != 1 {
			if err == nil {
				err = fmt.Errorf("recurrence is available for one task at a time")
			}
			m.err = err
			return statusTimeout(5 * time.Second)
		}
		frozenRequest := request.Request
		m.pendingTaskRecurrenceRequest = &frozenRequest
		m.suspendedTaskActionSheet = m.contextActionSheet
		m.contextActionSheet = nil
		task := refs[0].Task
		m.showView(ViewRecurrenceEditor, NewRecurrenceEditorModel(task.Recurrence, task.DueDate))
		return nil
	}
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
	return operationResultCmd(msg.Result)
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
