package tui

import (
	"fmt"
	"image/color"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var taskDetailTabOrder = [...]TaskDetailTab{
	TaskOverview,
	TaskChecklist,
	TaskComments,
	TaskDecisions,
	TaskAttachments,
	TaskActivity,
}

var taskDetailTabNames = [...]string{
	"Overview",
	"Checklist",
	"Comments",
	"Decisions",
	"Attachments",
	"Activity",
}

func (t TaskDetailTab) String() string {
	if int(t) >= len(taskDetailTabNames) {
		return "Unknown"
	}
	return taskDetailTabNames[t]
}

// taskDetailPresentation maps the unified tab back to the temporary
// PresentationID observation used by command discovery and migration tests.
func taskDetailPresentation(tab TaskDetailTab) PresentationID {
	switch tab {
	case TaskChecklist:
		return ViewTaskChecklist
	case TaskComments:
		return ViewTaskComments
	case TaskDecisions:
		return ViewTaskDecisions
	case TaskAttachments:
		return ViewTaskAttachments
	case TaskActivity:
		return ViewTaskActivity
	case TaskOverview:
		return ViewTaskDetail
	default:
		return ViewTaskDetail
	}
}

// TaskDetailModel is the single tabbed task-scoped primary surface.
type TaskDetailModel struct {
	BaseView
	InteractiveViewState
	task    *models.Task
	project *models.Project
	// runtime backs the domain-native scoring read (project scoring profile).
	runtime *config.RuntimeState

	// taskItem routes the detail view's own status/priority mutations through the
	// application-layer single-task coordinator. The shell wires it via
	// SetTaskItemCoordinator (taskItemWireable) from showView; a zero value is
	// only reachable in isolated tests that never dispatch a mutation.
	taskItem TaskItemCoordinator

	tab         TaskDetailTab
	checklist   *TaskChecklistModel
	comments    *TaskCommentsModel
	decisions   *TaskDecisionsModel
	attachments *TaskAttachmentsModel
	activity    *TaskActivityModel

	scrollOffset      int
	totalLines        int
	mdRenderer        *glamour.TermRenderer
	renderedDesc      string
	renderedDescWidth int
	renderedDescTheme string

	// historyExpanded controls progressive disclosure of the History cluster
	// (Comments, Decisions, Attachments). When false, only a one-line summary
	// header is rendered. Toggled by `H`. Intentionally not persisted.
	historyExpanded bool

	// openLoops mirrors AppModel.openLoops for the title-row Zeigarnik counter.
	openLoops OpenLoops

	// Planning membership is shell-owned because Today and Focus Queue live in
	// planning state rather than on the task entity itself.
	inDailyPlan  bool
	inFocusQueue bool
}

// setOpenLoops injects the latest cross-project open-loop counts into the view.
func (m *TaskDetailModel) setOpenLoops(o OpenLoops) { m.openLoops = o }

func (m *TaskDetailModel) SetPlanningMembership(inDailyPlan, inFocusQueue bool) {
	m.inDailyPlan = inDailyPlan
	m.inFocusQueue = inFocusQueue
}

// SetTaskItemCoordinator injects the application-layer single-task coordinator
// so the detail view's status/priority mutations commit through the ledger
// (taskItemWireable). Wired by the shell from showView/replaceView.
func (m *TaskDetailModel) SetTaskItemCoordinator(c TaskItemCoordinator) { m.taskItem = c }

// refreshAfterReload reloads the detail view's project from disk and re-resolves
// the shown task after a coordinator commit. The single-task coordinator commits
// to disk and reloads the shell's shared project pointer, but the detail view may
// hold a distinct (e.g. cross-project) pointer, so it reloads its own copy here.
// Called by the shell's refreshTaskViewsAfterSave after every committed mutation.
func (m *TaskDetailModel) refreshAfterReload() bool {
	if m.project == nil || m.task == nil {
		return false
	}
	if err := storage.LoadProject(m.project); err != nil {
		logging.Warn("failed to reload project %s in task detail: %v", m.project.Name, err)
		return true
	}
	if fresh := m.project.GetTaskByID(m.task.ID); fresh != nil {
		m.task = fresh
		m.renderedDesc = ""
		m.syncEmbeddedTasks()
		return true
	}
	m.task = nil
	m.syncEmbeddedTasks()
	return false
}

// NewTaskDetailModel creates a new task detail view.
func NewTaskDetailModel(task *models.Task, project *models.Project, runtime *config.RuntimeState) *TaskDetailModel {
	return NewTaskDetailModelForTab(task, project, runtime, TaskOverview, "", SubviewCoordinator{}, AttachmentCoordinator{})
}

// NewTaskDetailModelForTab creates the unified detail surface and all embedded
// tab controllers. Existing child models retain their coordinator-backed
// mutation and editor behavior without owning navigation entries.
func NewTaskDetailModelForTab(
	task *models.Task,
	project *models.Project,
	runtime *config.RuntimeState,
	tab TaskDetailTab,
	editor string,
	subview SubviewCoordinator,
	attachment AttachmentCoordinator,
) *TaskDetailModel {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(0),
	)
	if err != nil {
		logging.Warn("glamour renderer init failed: %v", err)
	}
	if tab > TaskActivity {
		tab = TaskOverview
	}
	m := &TaskDetailModel{
		task:       task,
		project:    project,
		runtime:    runtime,
		mdRenderer: renderer,
		tab:        tab,
	}
	m.checklist = NewTaskChecklistModelWithEditor(task, project, editor, subview)
	m.comments = NewTaskCommentsModelWithEditor(task, project, editor, subview)
	m.decisions = NewTaskDecisionsModelWithEditor(task, project, editor, subview)
	m.attachments = NewTaskAttachmentsModel(task, project, runtime, attachment)
	m.activity = NewTaskActivityModel(task, project)
	return m
}

func (m *TaskDetailModel) Init() tea.Cmd {
	if child := m.activeTabView(); child != nil {
		return child.Init()
	}
	return nil
}

func (m *TaskDetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.shouldChangeTab(msg.String()) {
			return m.changeTab(msg.String())
		}
		if m.tab != TaskOverview {
			return m.updateActiveTab(msg)
		}
		return m.handleKeyMsg(msg)
	case TaskUpdatedMsg:
		// Re-fetch task pointer in case project was reloaded.
		// Only react to messages targeting the task currently shown — otherwise
		// a broadcast for an unrelated task would swap m.task out from under us.
		if msg.Task != nil && m.task != nil && msg.Task.ID == m.task.ID {
			if fresh := m.project.GetTaskByID(msg.Task.ID); fresh != nil {
				m.task = fresh
				m.renderedDesc = ""
				m.syncEmbeddedTasks()
			} else {
				m.task = nil
				m.syncEmbeddedTasks()
				return m, func() tea.Msg { return BackMsg{} }
			}
		}
		return m, nil
	case TaskPropertyChangedMsg:
		if msg.Task != nil && m.task != nil && msg.Task.ID == m.task.ID {
			if fresh := m.project.GetTaskByID(msg.Task.ID); fresh != nil {
				m.task = fresh
				m.renderedDesc = ""
				m.syncEmbeddedTasks()
			} else {
				m.task = nil
				m.syncEmbeddedTasks()
				return m, func() tea.Msg { return BackMsg{} }
			}
		}
		return m, nil
	default:
		if m.tab != TaskOverview {
			return m.updateActiveTab(msg)
		}
	}
	return m, nil
}

func (m *TaskDetailModel) updateActiveTab(msg tea.Msg) (tea.Model, tea.Cmd) {
	child := m.activeTabView()
	if child == nil {
		return m, nil
	}
	_, cmd := child.Update(msg)
	return m, cmd
}

func (m *TaskDetailModel) activeTabView() View {
	switch m.tab {
	case TaskChecklist:
		return m.checklist
	case TaskComments:
		return m.comments
	case TaskDecisions:
		return m.decisions
	case TaskAttachments:
		return m.attachments
	case TaskActivity:
		return m.activity
	default:
		return nil
	}
}

func (m *TaskDetailModel) shouldChangeTab(key string) bool {
	if key != "tab" && key != "shift+tab" {
		return false
	}
	if child := m.activeTabView(); child != nil {
		if caps, ok := child.(ViewCapabilities); ok && (caps.HasTextInput() || caps.ShouldCaptureEsc()) {
			return false
		}
	}
	return true
}

func (m *TaskDetailModel) changeTab(key string) (tea.Model, tea.Cmd) {
	delta := 1
	if key == "shift+tab" {
		delta = -1
	}
	index := int(m.tab) + delta
	if index < 0 {
		index = len(taskDetailTabOrder) - 1
	}
	index %= len(taskDetailTabOrder)
	m.SetTab(taskDetailTabOrder[index])
	return m, func() tea.Msg { return TaskDetailTabChangedMsg{Tab: m.tab} }
}

// SetTab changes the active mode without reconstructing embedded models, so
// cursor and in-progress tab-local state survive ordinary tab switches.
func (m *TaskDetailModel) SetTab(tab TaskDetailTab) {
	if tab > TaskActivity || tab == m.tab {
		return
	}
	m.tab = tab
	m.scrollOffset = 0
}

func (m *TaskDetailModel) Tab() TaskDetailTab { return m.tab }

// Matches verifies that the model and typed route describe the same entity and
// active tab.
func (m *TaskDetailModel) Matches(params TaskDetailParams) bool {
	return m != nil && m.task != nil && m.project != nil &&
		m.task.ID == params.TaskID && m.project.ID == params.ProjectID && m.tab == params.Tab
}

func (m *TaskDetailModel) syncEmbeddedTasks() {
	if m.checklist != nil {
		m.checklist.task = m.task
	}
	if m.comments != nil {
		m.comments.task = m.task
	}
	if m.decisions != nil {
		m.decisions.task = m.task
	}
	if m.attachments != nil {
		m.attachments.task = m.task
		if m.task != nil {
			m.attachments.refreshAfterReload()
		}
	}
	if m.activity != nil {
		m.activity.task = m.task
	}
}

func (m *TaskDetailModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	childHeight := max(height-1, 1)
	for _, child := range m.embeddedTabViews() {
		child.SetSize(width, childHeight)
	}
}

func (m *TaskDetailModel) SetAccentColor(accent color.Color) {
	m.BaseView.SetAccentColor(accent)
	for _, child := range m.embeddedTabViews() {
		if setter, ok := child.(AccentColorSetter); ok {
			setter.SetAccentColor(accent)
		}
	}
}

func (m *TaskDetailModel) embeddedTabViews() []View {
	views := make([]View, 0, len(taskDetailTabOrder)-1)
	if m.checklist != nil {
		views = append(views, m.checklist)
	}
	if m.comments != nil {
		views = append(views, m.comments)
	}
	if m.decisions != nil {
		views = append(views, m.decisions)
	}
	if m.attachments != nil {
		views = append(views, m.attachments)
	}
	if m.activity != nil {
		views = append(views, m.activity)
	}
	return views
}

func (m *TaskDetailModel) HasTextInput() bool {
	if child := m.activeTabView(); child != nil {
		if caps, ok := child.(ViewCapabilities); ok {
			return caps.HasTextInput()
		}
	}
	return false
}

func (m *TaskDetailModel) HasPendingPrefix() bool {
	if child := m.activeTabView(); child != nil {
		if pending, ok := child.(PendingPrefixView); ok {
			return pending.HasPendingPrefix()
		}
	}
	return m.InteractiveViewState.HasPendingPrefix()
}

// ShouldCaptureEsc returns true so the detail view handles esc/q itself.
func (m *TaskDetailModel) ShouldCaptureEsc() bool {
	if child := m.activeTabView(); child != nil {
		if caps, ok := child.(ViewCapabilities); ok {
			return caps.ShouldCaptureEsc()
		}
	}
	return m.DefaultShouldCaptureEsc()
}

func (m *TaskDetailModel) IsConfirmActive() bool {
	if child := m.activeTabView(); child != nil {
		if confirm, ok := child.(interface{ IsConfirmActive() bool }); ok && confirm.IsConfirmActive() {
			return true
		}
	}
	return m.InteractiveViewState.IsConfirmActive()
}

// GetConfirmDialog surfaces an active child tab's confirmation so the App shell
// renders it as a floating overlay (checklist/comment/decision deletes and
// discard prompts). It falls back to Task Detail's own confirm state when no
// child tab owns an active dialog.
func (m *TaskDetailModel) GetConfirmDialog() *ConfirmDialog {
	if child := m.activeTabView(); child != nil {
		if provider, ok := child.(ConfirmDialogProvider); ok {
			if cd := provider.GetConfirmDialog(); cd != nil && cd.IsActive() {
				return cd
			}
		}
	}
	return m.InteractiveViewState.GetConfirmDialog()
}

func (m *TaskDetailModel) OnExit() {
	if m.comments != nil {
		m.comments.OnExit()
	}
	if m.decisions != nil {
		m.decisions.OnExit()
	}
}

// IsFullHeight returns true because task detail renders its own footer.
func (m *TaskDetailModel) IsFullHeight() bool { return true }

// GetPrefixPopupInfo returns popup info when a prefix key is pending.
func (m *TaskDetailModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if child := m.activeTabView(); child != nil {
		if prefix, ok := child.(interface{ GetPrefixPopupInfo() *PrefixPopupInfo }); ok {
			return prefix.GetPrefixPopupInfo()
		}
	}
	if m.pendingPrefix == "" {
		return nil
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  m.prefixHints(),
	}
}

// prefixHints returns the prefix hint map for this view.
func (m *TaskDetailModel) prefixHints() PrefixHints {
	hints := PrefixHints{
		",": StatusPrefixHints,
		"!": BuildPriorityPrefixHints(0),
	}

	// Add status-aware active hints
	if m.task != nil {
		statusHints := make([]PrefixHint, len(StatusPrefixHints))
		copy(statusHints, StatusPrefixHints)
		state := m.task.State()
		for i := range statusHints {
			// Each "," prefix key maps directly to the workflow state it applies;
			// the hint is active when the task is already in that state.
			if target, ok := StateForPrefixKey(statusHints[i].Key); ok {
				statusHints[i].Active = state == target
			}
		}
		hints[","] = statusHints
		hints["!"] = BuildPriorityPrefixHints(m.task.Priority)
	}

	return hints
}

// viewHeight returns the number of visible content lines after reserving the
// title and the task detail footer's actual wrapped height.
func (m *TaskDetailModel) viewHeight() int {
	return max(m.height-2-m.footerHeight(), 1)
}

// clampScroll ensures scrollOffset is within valid bounds.
func (m *TaskDetailModel) clampScroll() {
	maxScroll := max(m.totalLines-m.viewHeight(), 0)
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m *TaskDetailModel) StatusBarLabel() string {
	if m.project != nil {
		return fmt.Sprintf("Task Detail / %s: %s", m.tab, m.project.Name)
	}
	return "Task Detail"
}
