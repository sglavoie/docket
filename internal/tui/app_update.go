package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle global key messages first
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Clear temporary status/error messages on any keypress
		m.statusMessage = ""
		m.err = nil

		switch msg.String() {
		case "ctrl+c":
			m.saveProjectTreeState()
			m.quitting = true
			return m, tea.Quit
		case "esc":
			if handled := m.handleEscKey(); handled {
				return m, nil
			}
		case "q":
			// Don't intercept q if there's an active text input
			if m.hasActiveTextInput() {
				break
			}
			// On dashboard, q quits the app
			if m.currentView() == ViewDashboard {
				m.saveProjectTreeState()
				m.quitting = true
				return m, tea.Quit
			}
			if handled := m.handleEscKey(); handled {
				return m, nil
			}
		case ":":
			// Open command palette if no view has an active text input
			if !m.hasActiveTextInput() {
				return m, func() tea.Msg {
					return ShowCommandPaletteMsg{}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.handleWindowSize(msg)

	case QuitMsg:
		m.saveProjectTreeState()
		m.quitting = true
		return m, tea.Quit

	case spinner.TickMsg:
		// Only update spinner if we're in loading state
		if m.isLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case SaveStartMsg:
		m.isLoading = true
		m.loadingMessage = msg.Message
		m.err = nil
		return m, m.spinner.Tick()

	case SaveCompleteMsg:
		m.isLoading = false
		m.loadingMessage = ""
		m.statusMessage = msg.SuccessMessage
		m.refreshProjectTree()
		return m, statusTimeout(2 * time.Second)

	case SaveErrorMsg:
		m.isLoading = false
		m.loadingMessage = ""
		m.err = msg.Err
		return m, statusTimeout(5 * time.Second)
	}

	// Try each handler group in sequence
	if handled, cmd := m.handleProjectMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleTaskMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleViewMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleTagMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleSectionMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleInboxMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleClipboardMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleProjectCycleMessages(msg); handled {
		return m, cmd
	}

	// Delegate to current view
	cmd := m.delegateToCurrentView(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// hasActiveTextInput returns true if the current view has an active text input
// that should receive keyboard input (like search boxes, text fields).
// This is used to determine whether global keyboard shortcuts should be intercepted.
func (m *AppModel) hasActiveTextInput() bool {
	// Check if the current view has an active search input
	switch m.currentView() {
	case ViewProjectTree:
		if m.projectTree != nil && m.projectTree.HasActiveSearch() {
			return true
		}
	case ViewDashboard:
		if m.dashboard != nil && m.dashboard.HasActiveSearch() {
			return true
		}
	case ViewHelp:
		if m.helpView != nil && m.helpView.HasActiveSearch() {
			return true
		}
	case ViewTaskJumper:
		// Task jumper always has active search
		return true
	case ViewGlobalSearch:
		// Global search always has active search
		return true
	case ViewCommandPalette:
		// Command palette always has active search
		return true
	case ViewTagSelector:
		if selector, ok := m.views.Get(ViewTagSelector).(*TagSelectorModel); ok && selector != nil && selector.HasActiveInput() {
			return true
		}
	case ViewBulkTagSelector:
		if selector, ok := m.views.Get(ViewBulkTagSelector).(*BulkTagSelectorModel); ok && selector != nil && selector.HasActiveInput() {
			return true
		}
	case ViewParentSelector:
		// Parent selector always has active search
		return true
	case ViewProjectSelector:
		// Project selector always has active search
		return true
	case ViewTaskCreator, ViewNoteCreator, ViewQuickCapture, ViewBulkCapture, ViewTextEditor:
		// Form views always have active text input
		return true
	case ViewTaskEditor:
		// Task editor has active text input
		return true
	case ViewTaskComments:
		if comments, ok := m.views.Get(ViewTaskComments).(*TaskCommentsModel); ok && comments != nil {
			return comments.addMode || comments.editMode
		}
	}
	return false
}

// handleEscKey handles the escape/q key for view navigation.
// Returns true if the key was handled (should not propagate to views).
func (m *AppModel) handleEscKey() bool {
	// Let views handle esc/q if they have an active search, focus mode, sidebar focus, or pending prefix
	if m.currentView() == ViewProjectTree && m.projectTree != nil &&
		(m.projectTree.HasActiveSearch() || m.projectTree.IsFocusMode() || m.projectTree.sidebarFocused || m.projectTree.multiSelectMode || m.projectTree.pendingPrefix != "") {
		return false
	}
	if m.currentView() == ViewDashboard && m.dashboard != nil && (m.dashboard.HasActiveSearch() || m.dashboard.HasPendingPrefix()) {
		return false
	}
	if m.currentView() == ViewPriorityView {
		if pv, ok := m.views.Get(ViewPriorityView).(*PriorityViewModel); ok && pv != nil && pv.HasPendingPrefix() {
			return false
		}
	}
	if m.currentView() == ViewDailyReview {
		if dr, ok := m.views.Get(ViewDailyReview).(*DailyReviewModel); ok && dr != nil && (dr.HasPendingPrefix() || dr.HasFocusedSection()) {
			return false
		}
	}
	if m.currentView() == ViewGlobalInboxProcessor {
		if gip, ok := m.views.Get(ViewGlobalInboxProcessor).(*GlobalInboxProcessorModel); ok && gip != nil && gip.HasPendingPrefix() {
			return false
		}
	}
	if m.currentView() == ViewFocusView {
		if fv, ok := m.views.Get(ViewFocusView).(*FocusViewModel); ok && fv != nil && fv.HasPendingPrefix() {
			return false
		}
	}
	// Inbox processor handles its own q/esc to properly reload project and refresh tree
	if m.currentView() == ViewInboxProcessor {
		return false
	}
	if m.currentView() == ViewHelp && m.helpView != nil && m.helpView.HasActiveSearch() {
		return false
	}
	if m.currentView() == ViewTaskJumper && m.taskJumper != nil {
		return false
	}
	if m.currentView() == ViewTagSelector {
		if selector, ok := m.views.Get(ViewTagSelector).(*TagSelectorModel); ok && selector != nil && selector.HasActiveInput() {
			return false
		}
	}
	if m.currentView() == ViewBulkTagSelector {
		if selector, ok := m.views.Get(ViewBulkTagSelector).(*BulkTagSelectorModel); ok && selector != nil && selector.HasActiveInput() {
			return false
		}
	}
	if m.currentView() == ViewCommandPalette {
		if palette, ok := m.views.Get(ViewCommandPalette).(*CommandPaletteModel); ok && palette != nil && palette.IsActionMode() {
			return false
		}
	}
	// Workflow rule creator/editor handle their own esc for picker overlays
	if m.currentView() == ViewWorkflowRuleCreator || m.currentView() == ViewWorkflowRuleEditor {
		return false
	}
	// Recurrence editor handles its own esc to properly signal cancellation
	if m.currentView() == ViewRecurrenceEditor {
		return false
	}
	// Workflow rules view handles its own esc/q when in expanded view or delete confirmation
	if m.currentView() == ViewWorkflowRules {
		if rules, ok := m.views.Get(ViewWorkflowRules).(*WorkflowRulesModel); ok && rules != nil && (rules.expandedView || rules.confirmDelete) {
			return false
		}
	}
	// Task comments view handles its own esc when in expanded view, add/edit mode, or delete confirmation
	if m.currentView() == ViewTaskComments {
		if comments, ok := m.views.Get(ViewTaskComments).(*TaskCommentsModel); ok && comments != nil {
			if comments.expandedView || comments.confirmDelete || comments.addMode || comments.editMode {
				return false
			}
		}
	}
	if m.currentView() != ViewDashboard {
		closingView := m.currentView()
		m.popView()
		// Rebuild project tree when closing ProjectCycleManager since it reloads
		// all projects in buildProjectInfoList(), invalidating tree's task pointers
		if closingView == ViewProjectCycleManager && m.projectTree != nil {
			m.projectTree.buildTree()
		}
		return true
	}
	return false
}

// handleWindowSize updates dimensions for all views.
func (m *AppModel) handleWindowSize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height
	m.views.SetSize(msg.Width, msg.Height-2)
}

// delegateToCurrentView forwards messages to the active view model.
func (m *AppModel) delegateToCurrentView(msg tea.Msg) tea.Cmd {
	currentView := m.currentView()
	cmd := m.views.Update(currentView, msg)

	// Keep typed pointers in sync for views that need direct access
	switch currentView {
	case ViewDashboard:
		if v := m.views.Get(ViewDashboard); v != nil {
			m.dashboard = v.(*DashboardModel)
		}
	case ViewProjectTree:
		if v := m.views.Get(ViewProjectTree); v != nil {
			m.projectTree = v.(*ProjectTreeModel)
		}
	case ViewHelp:
		if v := m.views.Get(ViewHelp); v != nil {
			m.helpView = v.(*HelpViewModel)
		}
	case ViewTaskJumper:
		if v := m.views.Get(ViewTaskJumper); v != nil {
			m.taskJumper = v.(*TaskJumperModel)
		}
	case ViewInboxProcessor:
		if v := m.views.Get(ViewInboxProcessor); v != nil {
			m.inboxProcessor = v.(*InboxProcessorModel)
		}
	case ViewGlobalInboxProcessor:
		if v := m.views.Get(ViewGlobalInboxProcessor); v != nil {
			m.globalInboxProcessor = v.(*GlobalInboxProcessorModel)
		}
	}

	return cmd
}
