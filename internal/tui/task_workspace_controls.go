package tui

import (
	"fmt"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

const (
	CapabilityTaskWorkspace         Capability = "route:task-workspace"
	CapabilityWorkspaceQuery        Capability = "task-workspace:query"
	CapabilityWorkspaceViewMenu     Capability = "task-workspace:view-menu"
	CapabilityWorkspaceOrganize     Capability = "task-workspace:organize"
	CapabilityWorkspaceMutation     Capability = "task-workspace:mutation"
	CapabilityWorkspaceTask         Capability = "task-workspace:task"
	CapabilityWorkspaceSelection    Capability = "task-workspace:selection"
	CapabilityWorkspaceOutline      Capability = "task-workspace:layout:outline"
	CapabilityWorkspaceList         Capability = "task-workspace:layout:list"
	CapabilityWorkspaceBoard        Capability = "task-workspace:layout:board"
	CapabilityWorkspaceBoardAxis    Capability = "task-workspace:board-axis"
	CapabilityWorkspaceProjectScope Capability = "task-workspace:project-scope"
)

// Capabilities exposes the typed route's declarative command surface. It does
// not add a legacy-view capability.
func (s *TaskWorkspaceState) Capabilities(snapshot TaskWorkspaceSnapshot) CapabilitySet {
	capabilities := NewCapabilitySet(
		CapabilityCommandDiscovery,
		CapabilityTaskWorkspace,
		CapabilityWorkspaceQuery,
		CapabilityWorkspaceViewMenu,
		CapabilityWorkspaceOrganize,
		CapabilityWorkspaceMutation,
	)
	if s.Scope.Kind == ScopeApp {
		capabilities.Add(CapabilityAppScope)
	} else {
		capabilities.Add(CapabilityProjectScope, CapabilityWorkspaceProjectScope)
	}
	switch s.Layout {
	case WorkspaceOutline:
		capabilities.Add(CapabilityWorkspaceOutline)
	case WorkspaceList:
		capabilities.Add(CapabilityWorkspaceList)
	case WorkspaceBoard:
		capabilities.Add(CapabilityWorkspaceBoard, CapabilityWorkspaceBoardAxis)
	}
	if cursor, ok := s.Cursor(); ok {
		if _, exists := snapshot.Item(cursor); exists {
			capabilities.Add(CapabilityWorkspaceTask)
		}
	}
	if len(s.Selected()) > 0 {
		capabilities.Add(CapabilityWorkspaceSelection)
	}
	return capabilities
}

func (s *TaskWorkspaceState) CommandContext(snapshot TaskWorkspaceSnapshot) CommandContext {
	ctx := CommandContext{Scope: s.Scope.Kind, Capabilities: s.Capabilities(snapshot)}
	for _, id := range s.Selected() {
		ctx.Selection.TaskIDs = append(ctx.Selection.TaskIDs, id.TaskID)
	}
	if cursor, ok := s.Cursor(); ok {
		if item, exists := snapshot.Item(cursor); exists {
			ctx.Selection.ProjectID = cursor.ProjectID
			ctx.Entity = EntityContext{
				ProjectID: cursor.ProjectID,
				TaskID:    cursor.TaskID,
				State:     item.Task.State().String(),
				Mutable:   true,
				Archived:  item.Project.Archived,
			}
		}
	}
	return ctx
}

type TaskWorkspaceAction string

const (
	WorkspaceActionCursorDown    TaskWorkspaceAction = "cursor-down"
	WorkspaceActionCursorUp      TaskWorkspaceAction = "cursor-up"
	WorkspaceActionJumpFirst     TaskWorkspaceAction = "jump-first"
	WorkspaceActionJumpLast      TaskWorkspaceAction = "jump-last"
	WorkspaceActionPageUp        TaskWorkspaceAction = "page-up"
	WorkspaceActionPageDown      TaskWorkspaceAction = "page-down"
	WorkspaceActionLayoutLeft    TaskWorkspaceAction = "layout-left"
	WorkspaceActionLayoutRight   TaskWorkspaceAction = "layout-right"
	WorkspaceActionNextGroup     TaskWorkspaceAction = "next-group"
	WorkspaceActionPreviousGroup TaskWorkspaceAction = "previous-group"
	WorkspaceActionZoom          TaskWorkspaceAction = "zoom"
	WorkspaceActionZoomOut       TaskWorkspaceAction = "zoom-out"
	WorkspaceActionToggleSidebar TaskWorkspaceAction = "toggle-sidebar"
	WorkspaceActionOpenDetail    TaskWorkspaceAction = "open-detail"
	WorkspaceActionOpenPeek      TaskWorkspaceAction = "open-peek"
	WorkspaceActionSelect        TaskWorkspaceAction = "toggle-selection"
	WorkspaceActionSearch        TaskWorkspaceAction = "search"
	WorkspaceActionViewMenu      TaskWorkspaceAction = "view-menu"
	WorkspaceActionAdd           TaskWorkspaceAction = "add"
	WorkspaceActionEdit          TaskWorkspaceAction = "edit"
	WorkspaceActionComplete      TaskWorkspaceAction = "complete"
	WorkspaceActionState         TaskWorkspaceAction = "state"
	WorkspaceActionPin           TaskWorkspaceAction = "pin"
	WorkspaceActionPriority      TaskWorkspaceAction = "priority"
	WorkspaceActionDueDate       TaskWorkspaceAction = "due-date"
	WorkspaceActionTags          TaskWorkspaceAction = "tags"
	WorkspaceActionSnooze        TaskWorkspaceAction = "snooze"
	WorkspaceActionMove          TaskWorkspaceAction = "move"
	WorkspaceActionMoveDown      TaskWorkspaceAction = "move-down"
	WorkspaceActionMoveUp        TaskWorkspaceAction = "move-up"
	WorkspaceActionPark          TaskWorkspaceAction = "park"
	WorkspaceActionCopy          TaskWorkspaceAction = "copy"
	WorkspaceActionUndo          TaskWorkspaceAction = "undo"
	WorkspaceActionRedo          TaskWorkspaceAction = "redo"
)

// TaskWorkspaceBinding is one declarative key/action pair. Required
// capabilities filter both dispatch and Help, so there are no hidden aliases.
type TaskWorkspaceBinding struct {
	Key                  string
	Action               TaskWorkspaceAction
	Description          string
	Category             string
	RequiredCapabilities CapabilitySet
}

func taskWorkspaceBinding(key string, action TaskWorkspaceAction, description, category string, required ...Capability) TaskWorkspaceBinding {
	return TaskWorkspaceBinding{
		Key: key, Action: action, Description: description, Category: category,
		RequiredCapabilities: NewCapabilitySet(required...),
	}
}

// TaskWorkspaceBindingCatalog is the exact shared grammar for Tasks and
// Project. Tab is intentionally absent.
func TaskWorkspaceBindingCatalog() []TaskWorkspaceBinding {
	task := []Capability{CapabilityTaskWorkspace, CapabilityWorkspaceTask}
	return []TaskWorkspaceBinding{
		taskWorkspaceBinding("j", WorkspaceActionCursorDown, "Move down", "Navigation", CapabilityTaskWorkspace),
		taskWorkspaceBinding("down", WorkspaceActionCursorDown, "Move down", "Navigation", CapabilityTaskWorkspace),
		taskWorkspaceBinding("k", WorkspaceActionCursorUp, "Move up", "Navigation", CapabilityTaskWorkspace),
		taskWorkspaceBinding("up", WorkspaceActionCursorUp, "Move up", "Navigation", CapabilityTaskWorkspace),
		taskWorkspaceBinding("g", WorkspaceActionJumpFirst, "Jump to first task", "Navigation", CapabilityTaskWorkspace),
		taskWorkspaceBinding("G", WorkspaceActionJumpLast, "Jump to last task", "Navigation", CapabilityTaskWorkspace),
		taskWorkspaceBinding("ctrl+u", WorkspaceActionPageUp, "Page up", "Navigation", CapabilityTaskWorkspace),
		taskWorkspaceBinding("ctrl+d", WorkspaceActionPageDown, "Page down", "Navigation", CapabilityTaskWorkspace),
		taskWorkspaceBinding("h", WorkspaceActionLayoutLeft, "Move left in layout", "Navigation", CapabilityTaskWorkspace),
		taskWorkspaceBinding("l", WorkspaceActionLayoutRight, "Move right in layout", "Navigation", CapabilityTaskWorkspace),
		taskWorkspaceBinding("n", WorkspaceActionNextGroup, "Next group", "Navigation", CapabilityTaskWorkspace),
		taskWorkspaceBinding("N", WorkspaceActionPreviousGroup, "Previous group", "Navigation", CapabilityTaskWorkspace),
		taskWorkspaceBinding("z", WorkspaceActionZoom, "Zoom into task", "Navigation", task...),
		taskWorkspaceBinding("backspace", WorkspaceActionZoomOut, "Zoom out", "Navigation", CapabilityTaskWorkspace),
		taskWorkspaceBinding("\\", WorkspaceActionToggleSidebar, "Toggle sidebar", "View", CapabilityTaskWorkspace),
		taskWorkspaceBinding("enter", WorkspaceActionOpenDetail, "Open task detail", "Open", task...),
		taskWorkspaceBinding("shift+enter", WorkspaceActionOpenPeek, "Peek at task", "Open", task...),
		taskWorkspaceBinding("space", WorkspaceActionSelect, "Toggle selection", "Selection", task...),
		taskWorkspaceBinding("/", WorkspaceActionSearch, "Search tasks", "Open", CapabilityTaskWorkspace),
		taskWorkspaceBinding("v", WorkspaceActionViewMenu, "Open View menu", "View", CapabilityTaskWorkspace, CapabilityWorkspaceViewMenu),
		taskWorkspaceBinding("a", WorkspaceActionAdd, "Add task", "Mutation", CapabilityTaskWorkspace, CapabilityWorkspaceMutation),
		taskWorkspaceBinding("e", WorkspaceActionEdit, "Edit task", "Mutation", task...),
		taskWorkspaceBinding("x", WorkspaceActionComplete, "Complete task", "Mutation", task...),
		taskWorkspaceBinding("s", WorkspaceActionState, "Change state", "Mutation", task...),
		taskWorkspaceBinding("p", WorkspaceActionPin, "Toggle pin", "Mutation", task...),
		taskWorkspaceBinding("!", WorkspaceActionPriority, "Change priority", "Mutation", task...),
		taskWorkspaceBinding("d", WorkspaceActionDueDate, "Set due date", "Mutation", task...),
		taskWorkspaceBinding("t", WorkspaceActionTags, "Manage tags", "Mutation", task...),
		taskWorkspaceBinding("Z", WorkspaceActionSnooze, "Snooze task", "Mutation", task...),
		taskWorkspaceBinding("m", WorkspaceActionMove, "Move task", "Mutation", task...),
		taskWorkspaceBinding("J", WorkspaceActionMoveDown, "Move task down", "Mutation", task...),
		taskWorkspaceBinding("K", WorkspaceActionMoveUp, "Move task up", "Mutation", task...),
		taskWorkspaceBinding("P", WorkspaceActionPark, "Park task", "Mutation", task...),
		taskWorkspaceBinding("y", WorkspaceActionCopy, "Copy task", "Mutation", task...),
		taskWorkspaceBinding("u", WorkspaceActionUndo, "Undo task change", "Mutation", CapabilityTaskWorkspace, CapabilityWorkspaceMutation),
		taskWorkspaceBinding("ctrl+r", WorkspaceActionRedo, "Redo task change", "Mutation", CapabilityTaskWorkspace, CapabilityWorkspaceMutation),
	}
}

func TaskWorkspaceBindings(capabilities CapabilitySet) []TaskWorkspaceBinding {
	result := make([]TaskWorkspaceBinding, 0)
	for _, binding := range TaskWorkspaceBindingCatalog() {
		if capabilities.HasAll(binding.RequiredCapabilities) {
			result = append(result, binding)
		}
	}
	return result
}

func BuildTaskWorkspaceKeyMap(owner tea.Model, capabilities CapabilitySet, handle func(TaskWorkspaceAction) tea.Cmd) *keymap.KeyMap {
	bindings := TaskWorkspaceBindings(capabilities)
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
	return &keymap.KeyMap{ViewName: "Task Workspace", Entries: entries}
}

// TaskWorkspaceCommands projects the same grammar into capability-scoped
// palette descriptors. Views is always nil: typed capabilities are the only
// availability mechanism for these commands.
func TaskWorkspaceCommands(handler func(TaskWorkspaceAction) tea.Msg) []Command {
	seen := make(map[TaskWorkspaceAction]struct{})
	commands := make([]Command, 0)
	for _, binding := range TaskWorkspaceBindingCatalog() {
		if _, duplicate := seen[binding.Action]; duplicate {
			continue
		}
		seen[binding.Action] = struct{}{}
		action := binding.Action
		commands = append(commands, Command{
			OperationID:          OperationID("tui.workspace." + string(action)),
			Name:                 binding.Description,
			Description:          binding.Description + " in Task Workspace",
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

type TaskWorkspaceMenuSection string

const (
	WorkspaceMenuLayout   TaskWorkspaceMenuSection = "Layout"
	WorkspaceMenuFocus    TaskWorkspaceMenuSection = "Focus"
	WorkspaceMenuSmart    TaskWorkspaceMenuSection = "Smart"
	WorkspaceMenuOrganize TaskWorkspaceMenuSection = "Organize"
)

type TaskWorkspaceMenuItem struct {
	ID                   string
	Label                string
	Section              TaskWorkspaceMenuSection
	RequiredCapabilities CapabilitySet
	Active               func(*TaskWorkspaceState) bool
	Apply                func(*TaskWorkspaceState) error
}

func (item TaskWorkspaceMenuItem) IsActive(state *TaskWorkspaceState) bool {
	return item.Active != nil && item.Active(state)
}

func workspaceLayoutMenuItem(id, label string, layout TaskWorkspaceLayout) TaskWorkspaceMenuItem {
	return TaskWorkspaceMenuItem{
		ID: id, Label: label, Section: WorkspaceMenuLayout,
		RequiredCapabilities: NewCapabilitySet(CapabilityTaskWorkspace, CapabilityWorkspaceViewMenu),
		Active:               func(state *TaskWorkspaceState) bool { return state.Layout == layout },
		Apply:                func(state *TaskWorkspaceState) error { return state.SetLayout(layout) },
	}
}

func workspaceFocusMenuItem(id, label string, section TaskWorkspaceMenuSection, focus TaskWorkspaceFocus) TaskWorkspaceMenuItem {
	return TaskWorkspaceMenuItem{
		ID: id, Label: label, Section: section,
		RequiredCapabilities: NewCapabilitySet(CapabilityTaskWorkspace, CapabilityWorkspaceViewMenu),
		Active:               func(state *TaskWorkspaceState) bool { return state.Focus == focus },
		Apply:                func(state *TaskWorkspaceState) error { return state.SetFocus(focus, "") },
	}
}

func workspaceBoardAxisMenuItem(id, label string, axis TaskWorkspaceBoardAxis) TaskWorkspaceMenuItem {
	return TaskWorkspaceMenuItem{
		ID: id, Label: label, Section: WorkspaceMenuOrganize,
		RequiredCapabilities: NewCapabilitySet(CapabilityWorkspaceBoardAxis),
		Active:               func(state *TaskWorkspaceState) bool { return state.BoardAxis == axis },
		Apply:                func(state *TaskWorkspaceState) error { return state.SetBoardAxis(axis) },
	}
}

// TaskWorkspaceViewMenu returns one declarative menu filtered by the active
// layout capabilities. Saved filters are data-driven Smart entries; Board axis
// is present only in Board.
func TaskWorkspaceViewMenu(state *TaskWorkspaceState, snapshot TaskWorkspaceSnapshot, savedFilters []TaskWorkspaceSavedFilterOption) []TaskWorkspaceMenuItem {
	capabilities := state.Capabilities(snapshot)
	items := []TaskWorkspaceMenuItem{
		workspaceLayoutMenuItem("layout:outline", "Outline", WorkspaceOutline),
		workspaceLayoutMenuItem("layout:list", "List", WorkspaceList),
		workspaceLayoutMenuItem("layout:board", "Board", WorkspaceBoard),
		workspaceFocusMenuItem("focus:active", "Active", WorkspaceMenuFocus, WorkspaceActive),
		workspaceFocusMenuItem("focus:all", "All", WorkspaceMenuFocus, WorkspaceAll),
		workspaceFocusMenuItem("focus:inbox", "Inbox", WorkspaceMenuFocus, WorkspaceInbox),
		workspaceFocusMenuItem("focus:next", "Next", WorkspaceMenuFocus, WorkspaceNext),
		workspaceFocusMenuItem("focus:waiting", "Waiting", WorkspaceMenuFocus, WorkspaceWaiting),
		workspaceFocusMenuItem("focus:review", "Review", WorkspaceMenuFocus, WorkspaceReview),
		workspaceFocusMenuItem("focus:someday", "Someday", WorkspaceMenuFocus, WorkspaceSomeday),
		workspaceFocusMenuItem("focus:completed", "Completed", WorkspaceMenuFocus, WorkspaceCompleted),
		workspaceFocusMenuItem("smart:due-soon", "Due Soon", WorkspaceMenuSmart, WorkspaceDueSoon),
		workspaceFocusMenuItem("smart:high-priority", "High Priority", WorkspaceMenuSmart, WorkspaceHighPriority),
		workspaceFocusMenuItem("smart:blocked", "Blocked", WorkspaceMenuSmart, WorkspaceBlocked),
		workspaceFocusMenuItem("smart:parked", "Parked", WorkspaceMenuSmart, WorkspaceParked),
		workspaceFocusMenuItem("smart:snoozed", "Snoozed", WorkspaceMenuSmart, WorkspaceSnoozed),
		workspaceFocusMenuItem("smart:pinned", "Pinned", WorkspaceMenuSmart, WorkspacePinned),
		workspaceFocusMenuItem("smart:recent", "Recent", WorkspaceMenuSmart, WorkspaceRecent),
		{ID: "organize:grouping", Label: "Grouping", Section: WorkspaceMenuOrganize, RequiredCapabilities: NewCapabilitySet(CapabilityWorkspaceOrganize)},
		{ID: "organize:sorting", Label: "Sorting", Section: WorkspaceMenuOrganize, RequiredCapabilities: NewCapabilitySet(CapabilityWorkspaceOrganize)},
		{ID: "organize:columns", Label: "Columns", Section: WorkspaceMenuOrganize, RequiredCapabilities: NewCapabilitySet(CapabilityWorkspaceOrganize)},
		workspaceBoardAxisMenuItem("organize:board-axis:status", "Board axis: Status", WorkspaceBoardByStatus),
		workspaceBoardAxisMenuItem("organize:board-axis:section", "Board axis: Section", WorkspaceBoardBySection),
		workspaceBoardAxisMenuItem("organize:board-axis:priority", "Board axis: Priority", WorkspaceBoardByPriority),
		workspaceBoardAxisMenuItem("organize:board-axis:score", "Board axis: Score Buckets", WorkspaceBoardByScore),
		{ID: "organize:sidebar", Label: "Sidebar", Section: WorkspaceMenuOrganize, RequiredCapabilities: NewCapabilitySet(CapabilityWorkspaceOrganize), Active: func(s *TaskWorkspaceState) bool { return s.SidebarVisible }, Apply: func(s *TaskWorkspaceState) error { s.SidebarVisible = !s.SidebarVisible; return nil }},
		{ID: "organize:preview", Label: "Preview", Section: WorkspaceMenuOrganize, RequiredCapabilities: NewCapabilitySet(CapabilityWorkspaceOrganize), Active: func(s *TaskWorkspaceState) bool { return s.PreviewVisible }, Apply: func(s *TaskWorkspaceState) error { s.PreviewVisible = !s.PreviewVisible; return nil }},
		{ID: "organize:advanced-filters", Label: "Advanced filters", Section: WorkspaceMenuOrganize, RequiredCapabilities: NewCapabilitySet(CapabilityWorkspaceOrganize)},
	}
	for _, filter := range savedFilters {
		filter := filter
		items = append(items, TaskWorkspaceMenuItem{
			ID: "smart:saved:" + filter.ID, Label: filter.Name, Section: WorkspaceMenuSmart,
			RequiredCapabilities: NewCapabilitySet(CapabilityTaskWorkspace, CapabilityWorkspaceViewMenu),
			Active: func(state *TaskWorkspaceState) bool {
				return state.Focus == WorkspaceSavedFilter && state.SavedFilterID == filter.ID
			},
			Apply: func(state *TaskWorkspaceState) error { return state.SetFocus(WorkspaceSavedFilter, filter.ID) },
		})
	}
	return slices.DeleteFunc(items, func(item TaskWorkspaceMenuItem) bool {
		return !capabilities.HasAll(item.RequiredCapabilities)
	})
}

// TaskWorkspaceMutationAdapter is the layout-neutral transactional mutation
// port. A selected set is passed as one slice so bulk changes remain one
// application ChangeSet and one workspace-ledger entry.
type TaskWorkspaceMutationAdapter interface {
	Transition([]string, models.WorkflowState) tea.Cmd
	Edit(app.EditRequest) tea.Cmd
	SetPriority([]string, int) tea.Cmd
	SetDueDate([]string, *time.Time, bool) tea.Cmd
	Snooze([]string, *time.Time) tea.Cmd
	SetPinned(string, bool) tea.Cmd
	MoveUp(string) tea.Cmd
	MoveDown(string) tea.Cmd
}

type FeatureTaskWorkspaceMutationAdapter struct {
	task TaskItemCoordinator
	bulk BulkCoordinator
}

func NewFeatureTaskWorkspaceMutationAdapter(task TaskItemCoordinator, bulk BulkCoordinator) FeatureTaskWorkspaceMutationAdapter {
	return FeatureTaskWorkspaceMutationAdapter{task: task, bulk: bulk}
}

func (a FeatureTaskWorkspaceMutationAdapter) Transition(ids []string, target models.WorkflowState) tea.Cmd {
	if len(ids) == 1 {
		return a.task.Transition(ids[0], target)
	}
	if len(ids) > 1 {
		return a.bulk.Transition(ids, target)
	}
	return nil
}

func (a FeatureTaskWorkspaceMutationAdapter) Edit(req app.EditRequest) tea.Cmd {
	if req.TaskID == "" {
		return nil
	}
	return a.task.Edit(req)
}

func (a FeatureTaskWorkspaceMutationAdapter) SetPriority(ids []string, priority int) tea.Cmd {
	if len(ids) == 1 {
		return a.task.SetPriority(ids[0], priority)
	}
	if len(ids) > 1 {
		return a.bulk.SetPriority(ids, priority)
	}
	return nil
}

func (a FeatureTaskWorkspaceMutationAdapter) SetDueDate(ids []string, due *time.Time, hasTime bool) tea.Cmd {
	if len(ids) == 1 {
		return a.task.SetDueDate(ids[0], due, hasTime)
	}
	if len(ids) > 1 && due != nil {
		return a.bulk.SetDueDate(ids, *due, hasTime)
	}
	if len(ids) > 1 {
		return a.bulk.ClearDueDate(ids)
	}
	return nil
}

func (a FeatureTaskWorkspaceMutationAdapter) Snooze(ids []string, until *time.Time) tea.Cmd {
	if len(ids) == 1 {
		return a.task.Snooze(ids[0], until)
	}
	if len(ids) > 1 {
		return a.bulk.Snooze(ids, until)
	}
	return nil
}

func (a FeatureTaskWorkspaceMutationAdapter) SetPinned(id string, pinned bool) tea.Cmd {
	if id == "" {
		return nil
	}
	return a.task.SetPinned(id, pinned)
}

func (a FeatureTaskWorkspaceMutationAdapter) MoveUp(id string) tea.Cmd {
	if id == "" {
		return nil
	}
	return a.task.MoveUp(id)
}

func (a FeatureTaskWorkspaceMutationAdapter) MoveDown(id string) tea.Cmd {
	if id == "" {
		return nil
	}
	return a.task.MoveDown(id)
}

// TaskWorkspaceController owns shared state, the latest query snapshot, layout
// adapters, and the transactional mutation port.
type TaskWorkspaceController struct {
	State     *TaskWorkspaceState
	query     TaskWorkspaceQuery
	mutations TaskWorkspaceMutationAdapter
	snapshot  TaskWorkspaceSnapshot
	loaded    bool
	modes     map[TaskWorkspaceLayout]TaskWorkspaceMode
}

func NewTaskWorkspaceController(params TaskWorkspaceParams, query TaskWorkspaceQuery, mutations TaskWorkspaceMutationAdapter) (*TaskWorkspaceController, error) {
	state, err := NewTaskWorkspaceState(params)
	if err != nil {
		return nil, err
	}
	return &TaskWorkspaceController{State: state, query: query, mutations: mutations, modes: make(map[TaskWorkspaceLayout]TaskWorkspaceMode)}, nil
}

func (c *TaskWorkspaceController) RegisterMode(mode TaskWorkspaceMode) error {
	if mode == nil || mode.WorkspaceLayout() > WorkspaceBoard {
		return fmt.Errorf("invalid task workspace mode")
	}
	c.modes[mode.WorkspaceLayout()] = mode
	if c.loaded && mode.WorkspaceLayout() == c.State.Layout {
		mode.ApplyWorkspaceSnapshot(c.State, c.Snapshot())
	}
	return nil
}

func (c *TaskWorkspaceController) HasMode(layout TaskWorkspaceLayout) bool {
	if c == nil {
		return false
	}
	return c.modes[layout] != nil
}

func (c *TaskWorkspaceController) Refresh() error {
	if c == nil || c.State == nil {
		return fmt.Errorf("task workspace controller is unavailable")
	}
	snapshot, err := c.query.Run(c.State)
	if err != nil {
		return err
	}
	c.snapshot, c.loaded = snapshot, true
	if mode := c.modes[c.State.Layout]; mode != nil {
		mode.ApplyWorkspaceSnapshot(c.State, c.Snapshot())
	}
	return nil
}

func (c *TaskWorkspaceController) RefreshIfLoaded() error {
	if c == nil || !c.loaded {
		return nil
	}
	return c.Refresh()
}

func (c *TaskWorkspaceController) SetRouteParams(params TaskWorkspaceParams) error {
	if c == nil || c.State == nil {
		return fmt.Errorf("task workspace controller is unavailable")
	}
	if err := c.State.SetRouteParams(params); err != nil {
		return err
	}
	return c.RefreshIfLoaded()
}

func (c *TaskWorkspaceController) Snapshot() TaskWorkspaceSnapshot {
	result := TaskWorkspaceSnapshot{
		Items: slices.Clone(c.snapshot.Items), Groups: make([]TaskWorkspaceGroup, len(c.snapshot.Groups)),
		SavedFilters: slices.Clone(c.snapshot.SavedFilters), Automation: c.snapshot.Automation,
	}
	for i, group := range c.snapshot.Groups {
		result.Groups[i] = group
		result.Groups[i].Items = slices.Clone(group.Items)
	}
	return result
}

func (c *TaskWorkspaceController) ViewMenu() []TaskWorkspaceMenuItem {
	if c == nil || c.State == nil {
		return nil
	}
	snapshot := c.Snapshot()
	items := TaskWorkspaceViewMenu(c.State, snapshot, snapshot.SavedFilters)
	return slices.DeleteFunc(items, func(item TaskWorkspaceMenuItem) bool {
		switch item.ID {
		case "layout:outline":
			return !c.HasMode(WorkspaceOutline)
		case "layout:list":
			return !c.HasMode(WorkspaceList)
		case "layout:board":
			return !c.HasMode(WorkspaceBoard)
		default:
			return false
		}
	})
}

func (c *TaskWorkspaceController) targetIDs() []string {
	identities := c.State.Targets()
	result := make([]string, len(identities))
	for i, id := range identities {
		result[i] = id.TaskID
	}
	return result
}

func (c *TaskWorkspaceController) Transition(target models.WorkflowState) tea.Cmd {
	if c == nil || c.mutations == nil {
		return nil
	}
	return c.mutations.Transition(c.targetIDs(), target)
}

func (c *TaskWorkspaceController) Edit(req app.EditRequest) tea.Cmd {
	if c == nil || c.mutations == nil {
		return nil
	}
	return c.mutations.Edit(req)
}

func (c *TaskWorkspaceController) SetPriority(priority int) tea.Cmd {
	if c == nil || c.mutations == nil {
		return nil
	}
	return c.mutations.SetPriority(c.targetIDs(), priority)
}

func (c *TaskWorkspaceController) SetDueDate(due *time.Time, hasTime bool) tea.Cmd {
	if c == nil || c.mutations == nil {
		return nil
	}
	return c.mutations.SetDueDate(c.targetIDs(), due, hasTime)
}

func (c *TaskWorkspaceController) Snooze(until *time.Time) tea.Cmd {
	if c == nil || c.mutations == nil {
		return nil
	}
	return c.mutations.Snooze(c.targetIDs(), until)
}

func (c *TaskWorkspaceController) SetPinned(pinned bool) tea.Cmd {
	if c == nil || c.mutations == nil {
		return nil
	}
	targets := c.State.Targets()
	if len(targets) != 1 {
		return nil
	}
	return c.mutations.SetPinned(targets[0].TaskID, pinned)
}

func (c *TaskWorkspaceController) MoveUp() tea.Cmd {
	if c == nil || c.mutations == nil {
		return nil
	}
	targets := c.State.Targets()
	if len(targets) != 1 {
		return nil
	}
	return c.mutations.MoveUp(targets[0].TaskID)
}

func (c *TaskWorkspaceController) MoveDown() tea.Cmd {
	if c == nil || c.mutations == nil {
		return nil
	}
	targets := c.State.Targets()
	if len(targets) != 1 {
		return nil
	}
	return c.mutations.MoveDown(targets[0].TaskID)
}
