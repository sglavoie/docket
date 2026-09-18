package tui

import (
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskdetail"
	"github.com/sglavoie/tt/internal/tui/taskform"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// handleViewMessages handles view navigation and display messages.
// Returns true if the message was handled.
func (m *AppModel) handleViewMessages(msg tea.Msg) (bool, tea.Cmd) {
	// Delegate notes messages to dedicated handler
	if handled, cmd := m.handleNotesMessages(msg); handled {
		return handled, cmd
	}

	// Delegate editor messages to dedicated handler
	if handled, cmd := m.handleEditorMessages(msg); handled {
		return handled, cmd
	}

	if handled, cmd := m.handleViewCoreMessages(msg); handled {
		return handled, cmd
	}

	if handled, cmd := m.handleViewActionMessages(msg); handled {
		return handled, cmd
	}

	if handled, cmd := m.handleViewSupportMessages(msg); handled {
		return handled, cmd
	}

	if handled, cmd := m.handleProjectViewMessages(msg); handled {
		return handled, cmd
	}

	return false, nil
}

// handleViewCoreMessages handles generic view navigation, status, and utility messages.
func (m *AppModel) handleViewCoreMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case status.BackMsg:
		return true, m.popView()

	case status.ErrorMsg:
		m.feedback.err = msg.Err
		return true, tea.Batch(m.delegateToCurrentView(msg), status.StatusTimeout(5*time.Second))

	case help.ShowHelpMsg:
		m.helpView = help.NewHelpViewModel(msg.Items, msg.ViewName)
		m.showView(presentation.ViewHelp, m.helpView)
		return true, nil

	case status.StatusClearMsg:
		if m.feedback.stale(msg.ScheduledAt) {
			return true, nil
		}
		// A 2s plain-status tick may arrive after a newer setUndoableStatus
		// has installed a still-live toast (6s window). Don't dismiss a toast
		// that hasn't reached its own deadline yet — its dedicated
		// UndoToastTimeoutMsg will clear it on time.
		if m.feedback.undoLive(time.Now()) {
			return true, nil
		}
		m.clearStatus()
		return true, nil

	case status.ViewStatusClearMsg:
		if m.currentView() != msg.View {
			return true, nil
		}
		if owner, ok := m.activeView().(viewStatusOwner); ok && owner.StatusStamp() == msg.Seq {
			owner.ClearStatus()
		}
		return true, nil

	case FooterFlashTickMsg:
		return true, m.advanceFooterFlash(msg.Seq)

	case status.UndoToastTimeoutMsg:
		// Only clear if the deadline that fired this tick is still the
		// active one. A newer toast (with later expiresAt) may have
		// replaced this one — leave its callback alone.
		if m.feedback.undoLapsed(time.Now()) {
			m.clearStatus()
		}
		return true, nil

	case status.WorkspaceUndoMsg:
		// The panels reach the workspace ledger through the same entry point the
		// Outliner's u and Ctrl+R press, so a live undo toast still outranks the
		// ledger and an exhausted ledger stays silent here too.
		if msg.Redo {
			return true, m.coreRedo()
		}
		return true, m.coreUndo()

	case status.StatusMsg:
		m.setSuccessStatus(string(msg))
		return true, status.DefaultStatusTimeout()

	case status.InfoStatusMsg:
		m.setInfoStatus(string(msg))
		return true, status.DefaultStatusTimeout()

	case outliner.OutlinerPlanningReorderMsg:
		return true, m.handleOutlinerPlanningReorder(msg)
	}

	return false, nil
}

// handleViewActionMessages handles task action views such as priority, capture, and attachments.
func (m *AppModel) handleViewActionMessages(msg tea.Msg) (bool, tea.Cmd) {
	if scope, ok := msg.(outliner.ApplyProjectScopeMsg); ok {
		return m.applyProjectScope(scope)
	}
	return m.handleCaptureAndBulkMessages(msg)
}

// handleViewSupportMessages handles attachment views.
func (m *AppModel) handleViewSupportMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case taskdetail.ShowTaskAttachmentsMsg:
		if msg.Task == nil || msg.Project == nil {
			return true, nil
		}
		view := taskdetail.NewTaskAttachmentsModel(
			msg.Task,
			msg.Project,
			m.domain.runtime,
			m.secondaryFeatures.Attachment,
		)
		view.SetOperationDispatch(m.dispatchOperationKey)
		return m.showViewInit(presentation.ViewTaskAttachments, view)
	}

	return false, nil
}

// handleProjectViewMessages handles project tree and outliner view transitions.
func (m *AppModel) handleProjectViewMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case projecthub.ShowProjectHubMsg:
		return true, m.activateManageHub(route.ManageParams{
			Module:         route.ManageProjects,
			ProjectHubMode: msg.Mode,
		})

	case launchOutlinerMsg:
		presentationCmd := m.prepareCrossProjectOutliner()
		// The launch rule is armed here and not inside ResetToGlobalAll below,
		// which this message shares with the global reset: starting a session is
		// what owes the cursor a task, and a reset the user asked for is not.
		m.lifecycle.armLaunchFocus()
		m.clearTransientNavigationState()
		m.taskComposer = nil
		m.currentProject = nil
		m.projectStack = nil
		m.viewNav.navigation.ResetRoot()
		m.crossOutliner.ResetToGlobalAll()
		m.setAccentOnView(m.crossOutliner)
		// The reset drops any parked focus after presentationCmd captured it, so
		// supersede that load with one that requests the base workspace alone.
		return true, tea.Batch(
			m.activateTaskWorkspaceRoute(route.OutlinerRootRoute()),
			presentationCmd,
			m.crossOutliner.TakeParkedFocusReload(),
		)

	}

	return false, nil
}

func (m *AppModel) prepareCrossProjectOutliner() tea.Cmd {
	if m.crossOutliner == nil {
		m.crossOutliner = outliner.NewOutlinerModel(nil, m.domain.manager, m.domain.runtime)
		m.crossOutliner.SetSize(m.viewNav.viewSize())
		m.wireSecondaryCoordinators(m.crossOutliner)
		// The one live Outliner is the one surface that ranks notes. Every other
		// construction site leaves the accessor nil, so the tier is simply absent
		// there — correct for a test model and for the Hub's task preview.
		m.crossOutliner.SetNoteSearchIndex(m.noteSearchIndexRows)
		m.viewNav.views.Register(presentation.ViewCrossProjectOutliner, m.crossOutliner)
		return m.crossOutliner.Init()
	}
	return m.crossOutliner.ReloadCrossProjects()
}

// handleEditorMessages handles text editor, glow, and external editor messages.
// Returns true if the message was handled.
func (m *AppModel) handleEditorMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case chrome.ShowTextEditorMsg:
		return m.showViewInit(presentation.ViewTextEditor, chrome.NewTextEditorModel(msg.FieldName, msg.CurrentValue))

	case chrome.ShowTaskTextEditorMsg:
		if msg.Task == nil || msg.Project == nil {
			return true, nil
		}
		var fieldName, currentValue string
		switch msg.Operation {
		case app.TaskOperationEditDescription:
			fieldName = "description"
			currentValue = msg.Task.Description
		default:
			return true, status.ErrorCmd("unsupported task text operation %q", msg.Operation)
		}
		m.pendingTaskTextEdit = &struct {
			Identity  taskref.TaskIdentity
			Operation app.TaskOperation
			Original  string
		}{
			Identity:  taskref.For(msg.Task, msg.Project),
			Operation: msg.Operation,
			Original:  currentValue,
		}
		return m.showViewInit(presentation.ViewTextEditor, chrome.NewTextEditorModel(fieldName, currentValue))

	case chrome.TextEditorDoneMsg:
		m.popView()
		if m.pendingTaskTextEdit != nil {
			pending := m.pendingTaskTextEdit
			m.pendingTaskTextEdit = nil
			if msg.Canceled || msg.Value == pending.Original {
				return true, nil
			}
			action := "Edit Description"
			summary := "Description updated"
			return true, m.secondaryFeatures.TaskItem.ExecuteOperation(app.TaskOperationRequest{
				Operation: pending.Operation,
				Targets:   []app.TaskLocator{pending.Identity.Locator()},
				Text:      msg.Value,
			}, action, summary)
		}
		// Forward the message to the parent view (task creator or task editor)
		// so it can update the title/description
		cmd := m.viewNav.views.Update(m.currentView(), msg)
		return true, cmd

	case chrome.TextEditorExternalMsg:
		return m.handleTextEditorExternal(msg)

	case chrome.TextEditorExternalDoneMsg:
		m.clearStatus()
		// Forward to text editor view to update its textarea
		cmd := m.viewNav.views.Update(m.currentView(), msg)
		return true, cmd

	}
	return false, nil
}

// handleTextEditorExternal opens an external editor for text field editing.
func (m *AppModel) handleTextEditorExternal(msg chrome.TextEditorExternalMsg) (bool, tea.Cmd) {
	tmpPath, err := external.CreateTempFileWithContent(msg.CurrentValue, "tt-text-", ".md")
	if err != nil {
		return true, status.ErrorCmd("%s", err)
	}

	editor, editorErr := external.ResolveEditor("")
	if editorErr != nil {
		_ = os.Remove(tmpPath)
		return true, status.ErrorCmd("%s", editorErr)
	}
	c := external.EditorCommand(editor, tmpPath)

	m.setInfoStatus("Opening in editor...")
	fieldName := msg.FieldName
	return true, tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return chrome.TextEditorExternalDoneMsg{FieldName: fieldName, Value: "", Canceled: true}
		}
		content, readErr := os.ReadFile(tmpPath)
		_ = os.Remove(tmpPath)
		if readErr != nil {
			return chrome.TextEditorExternalDoneMsg{FieldName: fieldName, Value: "", Canceled: true}
		}
		return chrome.TextEditorExternalDoneMsg{FieldName: fieldName, Value: string(content), Canceled: false}
	})
}

// handleCaptureAndBulkMessages handles the Task Composer.
func (m *AppModel) handleCaptureAndBulkMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case taskform.ShowTaskComposerMsg:
		return true, m.openTaskComposer(msg.Preset)

	}

	return false, nil
}
