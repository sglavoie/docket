package tui

import (
	"fmt"
	"os"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/external"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/help"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/projecthub"
	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskdetail"
	"github.com/sglavoie/docket/internal/tui/taskform"
	"github.com/sglavoie/docket/internal/tui/taskref"
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
		width, height := m.viewNav.viewSize()
		m.helpView = m.helpSession.Open(msg.Items, msg.ViewName, width, height)
		if msg.ActionContext != nil && m.currentView() == presentation.ViewCrossProjectOutliner {
			m.helpView.EnableActions(*msg.ActionContext, msg.SearchItems)
		}
		m.showView(presentation.ViewHelp, m.helpView)
		return true, nil

	case help.RunActionMsg:
		return true, m.runHelpAction(msg)

	case status.StatusClearMsg:
		// A failed save belongs to the retained draft. Timers (including older
		// success timers) cannot erase the reason it has not been saved.
		if m.feedback.err != nil && m.quitBlockReason() != "" {
			return true, nil
		}
		if m.feedback.stale(msg.ScheduledAt) {
			return true, nil
		}
		// An earlier plain-status tick may arrive after a newer setUndoableStatus
		// has installed a still-live toast (6s window). Don't dismiss a toast
		// that hasn't reached its own deadline yet — its dedicated
		// UndoToastTimeoutMsg will clear it on time.
		if m.feedback.undoLive(time.Now()) {
			return true, nil
		}
		m.clearStatus()
		return true, nil

	case status.ViewStatusClearMsg:
		// The message names its view, so it clears that view's status even when
		// something else is drawn over it by the time the tick lands; dropping
		// it there would leave a transient message standing forever.
		if m.viewNav.views == nil {
			return true, nil
		}
		if owner, ok := m.viewNav.views.Get(msg.View).(viewStatusOwner); ok && owner.StatusStamp() == msg.Seq {
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
		return true, m.statusReadingTimeout()

	case status.InfoStatusMsg:
		m.setInfoStatus(string(msg))
		return true, m.statusReadingTimeout()

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
		// Seeded after the reset, which clears every fold, and beside the launch
		// focus for the same reason: only starting a session owes it.
		m.crossOutliner.FoldQuietLanesForLaunch()
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
		return m.showViewInit(presentation.ViewTextEditor, chrome.NewTextEditorModel(msg.FieldName, msg.Subtitle, msg.CurrentValue))

	case chrome.ShowTaskTextEditorMsg:
		if msg.Task == nil || msg.Project == nil {
			return true, nil
		}
		var fieldName, currentValue string
		switch msg.Operation {
		case app.TaskOperationEditDescription:
			fieldName = chrome.TextEditorFieldDescription
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
		return m.showViewInit(presentation.ViewTextEditor, chrome.NewTextEditorModel(fieldName, msg.Task.Title, currentValue))

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
		// Forward to text editor view to update its textarea
		cmd := m.viewNav.views.Update(m.currentView(), msg)
		if msg.Err != nil {
			// A failed round trip is reported rather than wiped with the
			// "Opening in editor…" notice: the draft the view still holds is
			// unchanged, and the user needs to know the edit did not land.
			return true, tea.Batch(cmd, status.ErrorCmd("%s", msg.Err))
		}
		m.clearStatus()
		return true, cmd

	}
	return false, nil
}

// handleTextEditorExternal opens an external editor for text field editing.
func (m *AppModel) handleTextEditorExternal(msg chrome.TextEditorExternalMsg) (bool, tea.Cmd) {
	tmpPath, err := external.CreateTempFileWithContent(msg.CurrentValue, "docket-text-", ".md")
	if err != nil {
		return true, status.ErrorCmd("%s", err)
	}

	editor, editorErr := external.ResolveEditor("")
	if editorErr != nil {
		_ = os.Remove(tmpPath)
		return true, status.ErrorCmd("%s", editorErr)
	}
	c := external.EditorCommand(editor, tmpPath)

	m.setInfoStatus("Opening in editor…")
	fieldName := msg.FieldName
	return true, tea.ExecProcess(c, textEditorExternalDone(fieldName, tmpPath))
}

// textEditorExternalDone turns the editor's exit into the round trip's result.
// An editor that failed is reported and its buffer removed, as the task-content
// round trip does. A buffer that cannot be read back is kept on disk and named,
// since whatever the user wrote there is the only copy of it.
func textEditorExternalDone(fieldName, tmpPath string) func(error) tea.Msg {
	return func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return chrome.TextEditorExternalDoneMsg{FieldName: fieldName, Canceled: true,
				Err: fmt.Errorf("editor failed: %w", err)}
		}
		content, readErr := os.ReadFile(tmpPath)
		if readErr != nil {
			return chrome.TextEditorExternalDoneMsg{FieldName: fieldName, Canceled: true,
				Err: fmt.Errorf("could not read the edited text; it is kept at %s: %w", tmpPath, readErr)}
		}
		_ = os.Remove(tmpPath)
		return chrome.TextEditorExternalDoneMsg{FieldName: fieldName, Value: string(content), Canceled: false}
	}
}

// handleCaptureAndBulkMessages handles the Task Composer.
func (m *AppModel) handleCaptureAndBulkMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case taskform.ShowTaskComposerMsg:
		return true, m.openTaskComposer(msg.Preset)

	}

	return false, nil
}

func (m *AppModel) outlinerHelpMessage(ctx operation.CommandContext) help.ShowHelpMsg {
	ctx = operation.CloneCommandContext(ctx)
	items := m.coreHelpItemsForContext(ctx, true)
	strategy, _ := m.operations.Registry.InputStrategy(operation.InputStrategySimplified)
	// Sheets already declare the rare entity actions. Project their menu-only
	// entries here too, with the real menu key as the route to learn.
	for _, entry := range append(taskOperationManifest(), outlinerProjectOperationManifest()...) {
		if entry.DirectOnly {
			continue
		}
		if descriptor, ok := m.operations.Registry.Descriptor(entry.ID); ok {
			menu := operation.OperationCoreActions
			if descriptor.TargetSource == operation.OperationTargetCurrentProject {
				menu = operation.OperationProjectActions
			}
			items = append(items, operation.HelpItem{
				Key: operation.DisplayKeysForOperation(strategy, menu), Description: descriptor.Label, Aliases: descriptor.Aliases,
				Category: operation.OperationCategoryLabel(descriptor.Category),
				Action:   &operation.HelpAction{ID: descriptor.ID}, DisabledReason: operation.AvailabilityReason(descriptor, ctx),
			})
		}
	}
	for i := range items {
		if items[i].Action == nil {
			continue
		}
		descriptor, ok := m.operations.Registry.Descriptor(items[i].Action.ID)
		if ok {
			items[i].Target = m.helpActionTarget(descriptor.TargetSource, ctx)
		}
	}
	return help.ShowHelpMsg{
		ViewName: "Outliner", Items: m.coreHelpItemsForContext(ctx, false),
		ActionContext: &ctx, SearchItems: items,
	}
}

func (m *AppModel) helpActionTarget(source operation.OperationTargetSource, ctx operation.CommandContext) string {
	switch source {
	case operation.OperationTargetSelectedTasks, operation.OperationTargetSelectedEntities:
		if ctx.Selection.Visual || ctx.Selection.Count() > 0 {
			label := fmt.Sprintf("%d selected tasks", ctx.Selection.Count())
			if ctx.Selection.Count() == 1 {
				label = "1 selected task"
			}
			if ctx.Selection.HiddenCount > 0 {
				label += fmt.Sprintf(" (%d hidden)", ctx.Selection.HiddenCount)
			}
			return label
		}
		fallthrough
	case operation.OperationTargetCurrentTask, operation.OperationTargetCurrentEntity:
		if ctx.Entity.HasChecklistCursor {
			index := ctx.Entity.ChecklistIndex
			if index >= 0 && index < len(ctx.Entity.Checklist) {
				return "Checklist item: " + ctx.Entity.Checklist[index].Text
			}
		}
		if task, project := m.TaskAndProjectForIdentity(operation.CurrentTaskIdentity(ctx)); task != nil && project != nil {
			return fmt.Sprintf("[%s:%d] %s", project.Slug, task.ShortID, task.Title)
		}
		if !ctx.Entity.HasProjectHeaderCursor {
			return "No task selected"
		}
		fallthrough
	case operation.OperationTargetCurrentProject:
		if project := m.ProjectByID(ctx.Entity.ProjectID); project != nil {
			return "Project: " + project.Name
		}
		return "No project selected"
	case operation.OperationTargetScopedProjects:
		return fmt.Sprintf("Current scope (%d projects)", len(ctx.ScopeProjectIDs))
	default:
		return "Current view / workspace"
	}
}

func (m *AppModel) runHelpAction(msg help.RunActionMsg) tea.Cmd {
	// A delayed Enter from an earlier help instance cannot close a newer
	// presentation or run against its cursor.
	if m.currentView() != presentation.ViewHelp || msg.Source != m.helpView {
		return nil
	}
	descriptor, ok := m.operations.Registry.Descriptor(msg.Request.ID)
	if !ok || m.crossOutliner == nil {
		return nil
	}
	live := m.crossOutliner.OperationCommandContext()
	if reason := operation.AvailabilityReason(descriptor, live); reason != "" {
		m.helpView.RefuseAction(descriptor.ID, reason)
		return nil
	}
	resolver := operation.OperationTargetResolver{}
	before, beforeErr := resolver.Resolve(descriptor, msg.Request.Context)
	now, nowErr := resolver.Resolve(descriptor, live)
	if beforeErr != nil || nowErr != nil || !slices.Equal(before.Targets, now.Targets) {
		m.helpView.RefuseAction(descriptor.ID, "The task or selection changed. Close help and choose it again.")
		return nil
	}
	resume := m.popView()
	if m.currentView() != msg.Request.Context.View {
		return resume
	}
	return tea.Batch(resume, operation.ResultCmd(m.operations.Executor.Execute(msg.Request)))
}
