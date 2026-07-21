package tui

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

const plannerViewMenuOverlayKey = "planner-view-menu"

const (
	CapabilityPlanner          Capability = "route:planner"
	CapabilityPlannerTask      Capability = "planner:task"
	CapabilityPlannerSelection Capability = "planner:selection"
	CapabilityPlannerMutation  Capability = "planner:mutation"
	CapabilityPlannerViewMenu  Capability = "planner:view-menu"
	CapabilityPlannerReorder   Capability = "planner:reorder"
)

const (
	PlannerActionNextTab     TaskWorkspaceAction = "planner-next-tab"
	PlannerActionPreviousTab TaskWorkspaceAction = "planner-previous-tab"
)

type plannerViewMenuMsg struct{ Open bool }

// PlannerRouteChangedMsg keeps the typed Planner route synchronized with the
// tab selected inside its renderer.
type PlannerRouteChangedMsg struct{ Tab PlannerTab }

// PlannerBindingCatalog projects the shared Task Workspace task grammar onto
// the Planner capabilities. Layout-only bindings are omitted; Planner adds only
// its two tab bindings and gives J/K their horizon-ordering semantics.
func PlannerBindingCatalog() []TaskWorkspaceBinding {
	allowed := map[TaskWorkspaceAction]bool{
		WorkspaceActionCursorDown: true, WorkspaceActionCursorUp: true,
		WorkspaceActionJumpFirst: true, WorkspaceActionJumpLast: true,
		WorkspaceActionPageUp: true, WorkspaceActionPageDown: true,
		WorkspaceActionNextGroup: true, WorkspaceActionPreviousGroup: true,
		WorkspaceActionOpenDetail: true, WorkspaceActionOpenPeek: true,
		WorkspaceActionSelect: true, WorkspaceActionSearch: true,
		WorkspaceActionViewMenu: true, WorkspaceActionAdd: true,
		WorkspaceActionEdit: true, WorkspaceActionComplete: true,
		WorkspaceActionState: true, WorkspaceActionPin: true,
		WorkspaceActionPriority: true, WorkspaceActionDueDate: true,
		WorkspaceActionTags: true, WorkspaceActionSnooze: true,
		WorkspaceActionMove: true, WorkspaceActionMoveDown: true,
		WorkspaceActionMoveUp: true, WorkspaceActionPark: true,
		WorkspaceActionCopy: true, WorkspaceActionUndo: true,
		WorkspaceActionRedo: true,
	}
	bindings := make([]TaskWorkspaceBinding, 0, len(TaskWorkspaceBindingCatalog())+2)
	for _, binding := range TaskWorkspaceBindingCatalog() {
		if !allowed[binding.Action] {
			continue
		}
		binding.RequiredCapabilities = plannerCapabilitiesFor(binding.RequiredCapabilities)
		switch binding.Action { //nolint:exhaustive // Only reorder bindings need Planner capability overrides.
		case WorkspaceActionMoveDown:
			binding.RequiredCapabilities.Add(CapabilityPlannerReorder)
			binding.Description = "Reorder down within Today or Next"
		case WorkspaceActionMoveUp:
			binding.RequiredCapabilities.Add(CapabilityPlannerReorder)
			binding.Description = "Reorder up within Today or Next"
		}
		bindings = append(bindings, binding)
	}
	bindings = append(bindings,
		taskWorkspaceBinding("tab", PlannerActionNextTab, "Next Planner tab", "Navigation", CapabilityPlanner),
		taskWorkspaceBinding("shift+tab", PlannerActionPreviousTab, "Previous Planner tab", "Navigation", CapabilityPlanner),
	)
	return bindings
}

func plannerCapabilitiesFor(required CapabilitySet) CapabilitySet {
	result := NewCapabilitySet()
	for capability := range required {
		switch capability {
		case CapabilityTaskWorkspace:
			result.Add(CapabilityPlanner)
		case CapabilityWorkspaceTask:
			result.Add(CapabilityPlannerTask)
		case CapabilityWorkspaceMutation:
			result.Add(CapabilityPlannerMutation)
		case CapabilityWorkspaceViewMenu:
			result.Add(CapabilityPlannerViewMenu)
		case CapabilityWorkspaceSelection:
			result.Add(CapabilityPlannerSelection)
		default:
			result.Add(capability)
		}
	}
	return result
}

func PlannerBindings(capabilities CapabilitySet) []TaskWorkspaceBinding {
	return slices.DeleteFunc(PlannerBindingCatalog(), func(binding TaskWorkspaceBinding) bool {
		return !capabilities.HasAll(binding.RequiredCapabilities)
	})
}

func BuildPlannerKeyMap(owner tea.Model, capabilities CapabilitySet, handle func(TaskWorkspaceAction) tea.Cmd) *keymap.KeyMap {
	bindings := PlannerBindings(capabilities)
	entries := make([]keymap.Entry, 0, len(bindings))
	for _, binding := range bindings {
		binding := binding
		entries = append(entries, keymap.Bind(binding.Key, binding.Description, binding.Category, func() (tea.Model, tea.Cmd) {
			if handle == nil {
				return owner, nil
			}
			return owner, handle(binding.Action)
		}))
	}
	return &keymap.KeyMap{ViewName: "Planner", Entries: entries}
}

// PlannerCommands projects the same capability-scoped grammar into the command
// palette. It deliberately has no PresentationID scoping.
func PlannerCommands(handler func(TaskWorkspaceAction) tea.Msg) []Command {
	seen := make(map[TaskWorkspaceAction]struct{})
	commands := make([]Command, 0)
	for _, binding := range PlannerBindingCatalog() {
		if _, duplicate := seen[binding.Action]; duplicate {
			continue
		}
		seen[binding.Action] = struct{}{}
		action := binding.Action
		commands = append(commands, Command{
			OperationID:          operationIDForName("Planner " + binding.Description),
			Name:                 "Planner " + binding.Description,
			Description:          binding.Description + " in Planner",
			Shortcut:             binding.Key,
			Category:             CategoryTasks,
			RequiredCapabilities: binding.RequiredCapabilities,
			Handler: func() tea.Msg {
				if handler == nil {
					return nil
				}
				return handler(action)
			},
		})
	}
	return commands
}

func (m *FocusBoardModel) Capabilities() CapabilitySet {
	capabilities := NewCapabilitySet(
		CapabilityCommandDiscovery,
		CapabilityAppScope,
		CapabilityPlanner,
		CapabilityPlannerMutation,
		CapabilityPlannerViewMenu,
	)
	if task, _ := m.getTaskRef(); task != nil {
		capabilities.Add(CapabilityPlannerTask)
	}
	if len(m.selection) > 0 {
		capabilities.Add(CapabilityPlannerSelection)
	}
	if m.tabs.Active == boardTabToday || m.tabs.Active == boardTabNext {
		capabilities.Add(CapabilityPlannerReorder)
	}
	return capabilities
}

func (m *FocusBoardModel) CommandContext() CommandContext {
	ctx := CommandContext{View: ViewFocusBoard, Scope: ScopeApp, Capabilities: m.Capabilities()}
	for _, ref := range m.selectedTaskRefs() {
		ctx.Selection.TaskIDs = append(ctx.Selection.TaskIDs, ref.Task.ID)
	}
	if task, project := m.getTaskRef(); task != nil && project != nil {
		ctx.Selection.ProjectID = project.ID
		ctx.Entity = EntityContext{
			ProjectID: project.ID, TaskID: task.ID, State: task.State().String(), Mutable: true, Archived: project.Archived,
		}
	}
	return ctx
}

func (m *FocusBoardModel) rebuildPlannerDispatcher() {
	m.Dispatcher = keymap.NewDispatcher(BuildPlannerKeyMap(m, m.Capabilities(), m.handlePlannerAction))
}

func (m *FocusBoardModel) handlePlannerAction(action TaskWorkspaceAction) tea.Cmd { //nolint:gocyclo
	switch action { //nolint:exhaustive // Planner deliberately excludes Workspace-only layout actions.
	case WorkspaceActionCursorDown:
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			m.skipGroupHeaderForward()
		}
	case WorkspaceActionCursorUp:
		if m.cursor > 0 {
			m.cursor--
			m.skipGroupHeaderBackward()
		}
	case WorkspaceActionJumpFirst:
		return m.JumpToTop()
	case WorkspaceActionJumpLast:
		old := m.cursor
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
			m.skipGroupHeaderBackward()
		}
		return flashIfJumped(old, m.cursor, &m.cursorFlash)
	case WorkspaceActionPageUp:
		old := m.cursor
		jumpCursor(&m.cursor, len(m.filtered), -HalfPage(m.Height()))
		m.skipGroupHeaderBackward()
		return flashIfJumped(old, m.cursor, &m.cursorFlash)
	case WorkspaceActionPageDown:
		old := m.cursor
		jumpCursor(&m.cursor, len(m.filtered), HalfPage(m.Height()))
		m.skipGroupHeaderForward()
		return flashIfJumped(old, m.cursor, &m.cursorFlash)
	case WorkspaceActionNextGroup:
		return m.focusBoardGoToNextParent()
	case WorkspaceActionPreviousGroup:
		return m.focusBoardGoToPrevParent()
	case WorkspaceActionOpenDetail, WorkspaceActionOpenPeek:
		_, cmd := m.handleEnter()
		return cmd
	case WorkspaceActionSelect:
		m.togglePlannerSelection()
	case WorkspaceActionSearch:
		m.searchInput.SetValue(m.searchQuery)
		return m.ActivateSearch()
	case WorkspaceActionViewMenu:
		m.menuOpen, m.menuCursor = true, 0
		return func() tea.Msg { return plannerViewMenuMsg{Open: true} }
	case WorkspaceActionAdd:
		m.addInput.SetValue("")
		m.addInput.Focus()
		m.addMode = true
	case WorkspaceActionEdit:
		if row := m.getRowAtCursor(); row != nil && row.Task != nil {
			m.editMode, m.editIdx = true, m.cursor
			m.editInput.SetValue(row.Task.Title)
			m.editInput.Focus()
		}
	case WorkspaceActionComplete:
		return m.completePlannerTargets()
	case WorkspaceActionState:
		return m.currentTaskCommand(func(task *models.Task, project *models.Project) tea.Msg {
			return ShowSectionPickerMsg{Task: task, Project: project}
		})
	case WorkspaceActionPin:
		if task, _ := m.getTaskRef(); task != nil {
			return m.taskItem.SetPinned(task.ID, !task.Pinned)
		}
	case WorkspaceActionPriority:
		return m.currentTaskCommand(func(task *models.Task, project *models.Project) tea.Msg {
			return ShowPriorityPickerMsg{Task: task, Project: project}
		})
	case WorkspaceActionDueDate:
		refs := m.plannerTargetRefs()
		return m.currentTaskCommand(func(task *models.Task, project *models.Project) tea.Msg {
			return SetDueDateMsg{Task: task, Project: project, TaskRefs: refs}
		})
	case WorkspaceActionTags:
		refs := m.plannerTargetRefs()
		if len(refs) > 1 {
			return func() tea.Msg { return ShowBulkTagSelectorMsg{TaskRefs: refs} }
		}
		return m.currentTaskCommand(func(task *models.Task, project *models.Project) tea.Msg {
			return ShowTagSelectorMsg{Task: task, Project: project, CurrentTags: slices.Clone(task.Tags)}
		})
	case WorkspaceActionSnooze:
		refs := m.plannerTargetRefs()
		return m.currentTaskCommand(func(task *models.Task, project *models.Project) tea.Msg {
			return SetSnoozeDateMsg{Task: task, Project: project, TaskRefs: refs}
		})
	case WorkspaceActionMove:
		return m.currentTaskCommand(func(task *models.Task, project *models.Project) tea.Msg {
			return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project}
		})
	case WorkspaceActionMoveDown:
		_, cmd := m.handleFocusBoardReorder(1)
		return cmd
	case WorkspaceActionMoveUp:
		_, cmd := m.handleFocusBoardReorder(-1)
		return cmd
	case WorkspaceActionPark:
		return m.currentTaskCommand(func(task *models.Task, project *models.Project) tea.Msg {
			return ShowParkInputMsg{Task: task, Project: project}
		})
	case WorkspaceActionCopy:
		tasks := m.getMultiSelectedTasks()
		if len(tasks) > 1 {
			return func() tea.Msg { return CopyTasksTitleMsg{Tasks: tasks} }
		}
		if task, _ := m.getTaskRef(); task != nil {
			return func() tea.Msg { return CopyTaskTitleMsg{Task: task} }
		}
	case PlannerActionNextTab:
		return m.cyclePlannerTab(1)
	case PlannerActionPreviousTab:
		return m.cyclePlannerTab(-1)
	case WorkspaceActionUndo, WorkspaceActionRedo:
		// AppModel owns durable ledger undo/redo and intercepts these bindings.
	}
	m.rebuildPlannerDispatcher()
	return nil
}

func (m *FocusBoardModel) completePlannerTargets() tea.Cmd {
	refs := m.plannerTargetRefs()
	if len(refs) <= 1 {
		_, cmd := m.markDone()
		return cmd
	}
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.Task.ID)
	}
	return m.bulk.TransitionWithWorkflow(app.BulkTransitionRequest{
		TaskIDs: ids, Target: models.StateDone, Rules: m.workflowRules(),
		UndoAction: "Complete Tasks", UndoContext: fmt.Sprintf("%d Planner tasks", len(ids)), SkipInvalid: true,
	}, func(changed int) string { return fmt.Sprintf("Completed %d task(s)", changed) })
}

func (m *FocusBoardModel) currentTaskCommand(build func(*models.Task, *models.Project) tea.Msg) tea.Cmd {
	task, project := m.getTaskRef()
	if task == nil || project == nil || build == nil {
		return nil
	}
	return func() tea.Msg { return build(task, project) }
}

func (m *FocusBoardModel) currentTaskIdentity() (TaskIdentity, bool) {
	row := m.getRowAtCursor()
	if row == nil || row.Task == nil || row.Project == nil {
		return TaskIdentity{}, false
	}
	return TaskIdentity{ProjectID: row.Project.ID, TaskID: row.Task.ID}, true
}

func (m *FocusBoardModel) togglePlannerSelection() {
	id, ok := m.currentTaskIdentity()
	if !ok {
		return
	}
	if m.selection == nil {
		m.selection = make(map[TaskIdentity]struct{})
	}
	if _, selected := m.selection[id]; selected {
		delete(m.selection, id)
	} else {
		m.selection[id] = struct{}{}
	}
	m.rebuildPlannerDispatcher()
}

func (m *FocusBoardModel) selectedTaskRefs() []TaskProjectRef {
	refs := make([]TaskProjectRef, 0, len(m.selection))
	for _, row := range m.filtered {
		if row.Task == nil || row.Project == nil {
			continue
		}
		id := TaskIdentity{ProjectID: row.Project.ID, TaskID: row.Task.ID}
		if _, selected := m.selection[id]; selected {
			refs = append(refs, TaskProjectRef{Task: row.Task, Project: row.Project})
		}
	}
	return refs
}

func (m *FocusBoardModel) plannerTargetRefs() []TaskProjectRef {
	if refs := m.selectedTaskRefs(); len(refs) > 0 {
		return refs
	}
	if task, project := m.getTaskRef(); task != nil && project != nil {
		return []TaskProjectRef{{Task: task, Project: project}}
	}
	return nil
}

func (m *FocusBoardModel) getCursorTask() *models.Task {
	task, _ := m.getTaskRef()
	return task
}

func (m *FocusBoardModel) getMultiSelectedTasks() []*models.Task {
	if len(m.selection) == 0 {
		return nil
	}
	refs := m.selectedTaskRefs()
	tasks := make([]*models.Task, 0, len(refs))
	for _, ref := range refs {
		tasks = append(tasks, ref.Task)
	}
	return tasks
}

func (m *FocusBoardModel) clearMultiSelect() {
	clear(m.selection)
	m.rebuildPlannerDispatcher()
}

func (m *FocusBoardModel) IsMultiSelectActive() bool { return len(m.selection) > 0 }

func (m *FocusBoardModel) cyclePlannerTab(delta int) tea.Cmd {
	m.tabs.Cycle(delta)
	m.cursor = 0
	clear(m.selection)
	m.applyFilter()
	m.clampCursor()
	m.rebuildPlannerDispatcher()
	tab, _ := plannerTabFromBoard(m.tabs.Active)
	return func() tea.Msg { return PlannerRouteChangedMsg{Tab: tab} }
}

func plannerTabFromBoard(tab int) (PlannerTab, bool) {
	switch tab {
	case boardTabToday:
		return PlannerToday, true
	case boardTabNext:
		return PlannerNext, true
	case boardTabBacklog:
		return PlannerBacklog, true
	default:
		return PlannerToday, false
	}
}

type plannerMenuItem struct {
	label  string
	active bool
	apply  func() tea.Cmd
}

func (m *FocusBoardModel) plannerMenuItems() []plannerMenuItem {
	items := []plannerMenuItem{
		{label: "Preview", active: m.previewMode, apply: func() tea.Cmd { m.previewMode = !m.previewMode; return nil }},
	}
	for _, option := range []struct {
		label string
		field SortField
	}{
		{"Sort: Planner order", SortByDefault},
		{"Sort: Priority", SortByPriority},
		{"Sort: Due date", SortByDueDate},
		{"Sort: Created", SortByCreatedAt},
		{"Sort: Status", SortByStatus},
		{"Sort: Score", SortByScore},
	} {
		option := option
		items = append(items, plannerMenuItem{
			label: option.label, active: m.sort.Field == option.field,
			apply: func() tea.Cmd {
				m.sort = SortState{Field: option.field, Ascending: true}
				m.applyFilter()
				m.clampCursor()
				return nil
			},
		})
	}
	if m.tabs.Active == boardTabBacklog {
		items = append(items, plannerMenuItem{label: "Filter Backlog by project…", active: len(m.backlogProjectFilter) > 0, apply: func() tea.Cmd {
			_, cmd := m.focusBoardFilterByProject()
			return cmd
		}})
	}
	items = append(items, plannerMenuItem{label: "Advanced filters…", active: hasActiveCommonFilters(&m.filter), apply: func() tea.Cmd {
		return func() tea.Msg { return ShowAttrFilterPickerMsg{Source: ViewFocusBoard} }
	}})
	if m.HasActiveFilters() {
		items = append(items, plannerMenuItem{label: "Clear filters", apply: func() tea.Cmd {
			m.ClearAllFilters()
			m.applyFilter()
			m.clampCursor()
			return nil
		}})
	}
	return items
}

func (m *FocusBoardModel) handlePlannerMenuKey(msg tea.KeyPressMsg) tea.Cmd {
	items := m.plannerMenuItems()
	switch msg.String() {
	case "esc", "v":
		m.menuOpen = false
		return func() tea.Msg { return plannerViewMenuMsg{Open: false} }
	case "j", "down":
		m.menuCursor = min(m.menuCursor+1, max(len(items)-1, 0))
	case "k", "up":
		m.menuCursor = max(m.menuCursor-1, 0)
	case "enter":
		if m.menuCursor < 0 || m.menuCursor >= len(items) {
			return nil
		}
		cmd := items[m.menuCursor].apply()
		m.menuOpen = false
		m.rebuildPlannerDispatcher()
		return tea.Batch(func() tea.Msg { return plannerViewMenuMsg{Open: false} }, cmd)
	}
	return nil
}

func (m *FocusBoardModel) renderPlannerViewMenu(width int) string {
	if !m.menuOpen {
		return ""
	}
	items := m.plannerMenuItems()
	lines := make([]string, 0, len(items))
	for i, item := range items {
		marker, active := "  ", "  "
		if i == m.menuCursor {
			marker = "› "
		}
		if item.active {
			active = "✓ "
		}
		line := marker + active + item.label
		if i == m.menuCursor {
			line = SelectedTaskTitleStyle.Render(line)
		}
		lines = append(lines, line)
	}
	innerWidth := min(max(width-8, 28), 54)
	return renderTitledBox(stringsJoin(lines, "\n"), innerWidth, "View", "↑/↓ choose · enter apply · esc close", lipgloss.NewStyle().Foreground(ColorPrimary))
}

// stringsJoin exists only to keep planner_controls imports focused on behavior.
func stringsJoin(parts []string, sep string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += sep
		}
		result += part
	}
	return result
}
