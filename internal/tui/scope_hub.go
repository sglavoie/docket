package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

const (
	scopeHubCategoryScopes ListCategoryID = "scope-hub:scopes"

	scopeHubActionCategoryContext ListCategoryID = "scope-hub-actions:context"
	scopeHubActionCategoryDanger  ListCategoryID = "scope-hub-actions:danger"
)

var scopeHubCategories = []ListCategory{
	{ID: scopeHubCategoryScopes, Label: "Scopes"},
}

var scopeHubActionCategories = []ListCategory{
	{ID: scopeHubActionCategoryContext, Label: "Context"},
	{ID: scopeHubActionCategoryDanger, Label: "Danger"},
}

type scopeHubEntryKind uint8

const (
	scopeHubEntryBase scopeHubEntryKind = iota
)

// scopeHubEntry is the typed payload behind one selectable scope row.
type scopeHubEntry struct {
	Kind  scopeHubEntryKind
	Scope ContextScope
}

// scopeHubActionTarget freezes stable identity at the moment Space opens the
// action sheet. No later cursor movement can retarget the selected operation.
type scopeHubActionTarget struct {
	Scope ContextScope
}

type scopeHubInlineActionMsg struct {
	OperationID OperationID
	Target      scopeHubActionTarget
}

const (
	operationScopeActivateContext OperationID = "tui.scope.activate-context"

	operationScopeCreateContext OperationID = "tui.scope.create-context"
	operationScopeEditContext   OperationID = "tui.scope.edit-context"
	operationScopeRenameContext OperationID = "tui.scope.rename-context"
	operationScopeDeleteContext OperationID = "tui.scope.delete-context"
)

func scopeHubOperationDescriptors() []OperationDescriptor {
	descriptor := func(
		id OperationID,
		label string,
		category OperationCategory,
		risk OperationRisk,
		aliases ...string,
	) OperationDescriptor {
		return OperationDescriptor{
			ID:                   id,
			Label:                label,
			Aliases:              aliases,
			Category:             category,
			TargetSource:         OperationTargetGlobal,
			TargetCardinality:    TargetCardinalityNone,
			RequiredCapabilities: NewCapabilitySet(CapabilityScopeHub),
			Risk:                 risk,
		}
	}
	return []OperationDescriptor{
		descriptor(operationScopeActivateContext, "Activate scope", OperationCategoryScope, OperationRiskReadOnly, "apply scope", "switch scope"),
		descriptor(operationScopeCreateContext, "New context", OperationCategoryScope, OperationRiskMutation, "create context"),
		descriptor(operationScopeEditContext, "Edit membership", OperationCategoryScope, OperationRiskMutation, "edit context"),
		descriptor(operationScopeRenameContext, "Rename context", OperationCategoryScope, OperationRiskMutation),
		descriptor(operationScopeDeleteContext, "Delete context", OperationCategoryScope, OperationRiskDestructive),
	}
}

// registerScopeHubOperations makes the same stable operations available to the
// Scope Hub action sheet and harness discovery.
func (m *AppModel) registerScopeHubOperations() error {
	for _, descriptor := range scopeHubOperationDescriptors() {
		id := descriptor.ID
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities: NewCapabilitySet(CapabilityScopeHub),
			Handler: func(invocation OperationInvocation) OperationHandlerResult {
				target, ok := invocation.Value.(scopeHubActionTarget)
				if !ok {
					return OperationHandlerResult{Err: fmt.Errorf("scope action %s has no frozen target", id)}
				}
				return OperationHandlerResult{Message: m.scopeHubOperationMessage(id, target)}
			},
			// The existing project/context lifecycle paths own their checked
			// commits and confirmations. The registry remains the single
			// invocation catalog without duplicating those policies here.
			DelegateTargetResolution: true,
			DelegateValueProvider:    true,
			DelegateConfirmation:     true,
			DelegateTransaction:      true,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (m *AppModel) scopeHubOperationMessage(id OperationID, target scopeHubActionTarget) tea.Msg {
	switch id {
	case operationScopeActivateContext:
		return ContextScopeSelectedMsg{
			SelectedContext: target.Scope.Name,
		}
	case operationScopeCreateContext,
		operationScopeEditContext,
		operationScopeRenameContext,
		operationScopeDeleteContext:
		return scopeHubInlineActionMsg{OperationID: id, Target: target}
	default:
	}
	return ErrorMsg{Err: fmt.Errorf("scope action target is no longer available")}
}

func (m *ContextHubModel) SetOperationSystem(registry *OperationRegistry, executor *OperationExecutor) {
	m.registry = registry
	m.executor = executor
}

func (m *ContextHubModel) scopeHubCommandContext() CommandContext {
	ctx := PresentationCommandContext(ViewContextHub)
	ctx.Capabilities.Add(CapabilityScopeHub)
	return ctx
}

// scopeHubActionContext carries the frozen row identity into the shared
// operation pipeline.
func (m *ContextHubModel) scopeHubActionContext(_ scopeHubActionTarget) CommandContext {
	return m.scopeHubCommandContext()
}

func (m *ContextHubModel) currentScopeHubItemID() ListItemID {
	if m.surface == nil {
		return ""
	}
	item, ok := m.surface.CurrentItem()
	if !ok {
		return ""
	}
	return item.ID
}

// rebuildScopeSurface reloads All Projects and regular contexts while
// preserving a stable cursor identity and the current fuzzy query.
func (m *ContextHubModel) rebuildScopeSurface(prefer ListItemID) {
	if m.runtime == nil {
		return
	}
	m.summaries = ContextScopeSummaries(m.runtime)
	items := m.scopeHubItems()
	if m.surface == nil {
		surface, surfaceErr := NewListSurfaceModel(items, scopeHubCategories, ListSurfaceConfig{
			SelectionMode: ListSelectionSingle,
			EmptyState: ListEmptyState{
				Title:  "No scopes are available.",
				Detail: "Add a project context to make it available here.",
			},
			NoMatchState: ListEmptyState{
				Title:  "No contexts match this search.",
				Detail: "Change the query to search scopes and contexts.",
			},
		})
		if surfaceErr != nil {
			m.notice = surfaceErr.Error()
			return
		}
		m.surface = surface
	} else if err := m.surface.Reload(items, scopeHubCategories); err != nil {
		m.notice = err.Error()
		return
	}
	m.surface.SetQuery(m.SearchQuery())
	if prefer != "" {
		m.surface.SetCursorToID(prefer)
	}
}

func (m *ContextHubModel) scopeHubItems() []ListItem {
	items := make([]ListItem, 0, len(m.summaries))
	for _, summary := range m.summaries {
		scope := summary.Scope
		label := contextScopeDisplayName(scope)
		if scope.IsAll() {
			label = "All Projects"
		}
		if scope == m.active {
			label += " (active)"
		}
		aliases := []string{scope.Name, "scope", "context", label}
		for _, project := range scope.Projects(m.runtime) {
			if project != nil {
				aliases = append(aliases, project.Name, project.Slug)
			}
		}
		operations := []OperationID{operationScopeCreateContext}
		if !scope.IsAll() {
			operations = append(operations, operationScopeEditContext)
			if !strings.EqualFold(scope.Name, m.runtime.DefaultContext()) {
				operations = append(operations, operationScopeRenameContext, operationScopeDeleteContext)
			}
		}
		items = append(items, ListItem{
			ID:            contextScopeListItemID(scope),
			CategoryID:    scopeHubCategoryScopes,
			PrimaryText:   label,
			SecondaryText: contextHubRowDetail(summary),
			Aliases:       aliases,
			Preview:       m.scopeHubContextPreview(summary),
			OperationIDs:  operations,
			Data:          scopeHubEntry{Kind: scopeHubEntryBase, Scope: scope},
		})
	}
	return items
}

func (m *ContextHubModel) scopeHubContextPreview(summary ContextScopeSummary) ListPreview {
	label := contextScopeDisplayName(summary.Scope)
	if summary.Scope.IsAll() {
		label = "All Projects"
	}
	body := m.previewCounts(summary)
	projects := summary.Scope.Projects(m.runtime)
	if len(projects) == 0 {
		body += "\n\nNo active projects in this context."
	} else {
		body += "\n\nProjects"
		for _, project := range projects {
			if project == nil {
				continue
			}
			body += "\n" + project.Name
			if m.taskCounts != nil {
				body += fmt.Sprintf("  %d task(s)", m.taskCounts[project.ID])
			}
		}
	}
	return ListPreview{
		Title:   label,
		Summary: contextHubScopeKindLabel(summary.Scope),
		Body:    body,
	}
}

func (m *ContextHubModel) selectedScopeHubEntry() (scopeHubEntry, bool) {
	if m.surface == nil {
		return scopeHubEntry{}, false
	}
	item, ok := m.surface.CurrentItem()
	if !ok {
		return scopeHubEntry{}, false
	}
	entry, ok := item.Data.(scopeHubEntry)
	return entry, ok
}

func (m *ContextHubModel) activateScopeHubCurrent() tea.Cmd {
	entry, ok := m.selectedScopeHubEntry()
	if !ok {
		return nil
	}
	switch entry.Kind {
	case scopeHubEntryBase:
		scope := entry.Scope
		return func() tea.Msg {
			return ContextScopeSelectedMsg{
				SelectedContext: scope.Name,
			}
		}
	}
	return nil
}

func scopeHubTarget(entry scopeHubEntry) scopeHubActionTarget {
	return scopeHubActionTarget{Scope: entry.Scope}
}

func (m *ContextHubModel) openScopeHubActions() {
	entry, ok := m.selectedScopeHubEntry()
	if !ok || m.surface == nil {
		return
	}
	target := scopeHubTarget(entry)
	sheet, err := NewContextActionSheetModel(
		m.registry,
		m.executor,
		m.scopeHubActionContext(target),
		m.surface.ActionState().OperationIDs,
		ContextActionSheetConfig{
			Host:         ContextActionHostScopeHub,
			Completion:   ContextActionCompleteCloseSheet,
			Title:        "Actions",
			Subtitle:     scopeHubEntryLabel(entry),
			Categories:   scopeHubActionCategories,
			CategoryFor:  scopeHubActionCategory,
			SecondaryFor: func(descriptor OperationDescriptor) string { return string(descriptor.Risk) },
			// The frozen row travels as the value every scope operation reads, so
			// no later rebuild or cursor move can retarget it.
			Value:        target,
			HasValue:     true,
			EmptyState:   ListEmptyState{Title: "No actions available."},
			NoMatchState: ListEmptyState{Title: "No matching actions.", Detail: "Clear or change the search."},
		},
	)
	if err != nil {
		m.notice = err.Error()
		return
	}
	if len(sheet.operationIDs()) == 0 {
		m.notice = "No administration actions are available for this item"
		return
	}
	m.actions = sheet
}

func scopeHubActionCategory(descriptor OperationDescriptor) ListCategoryID {
	if descriptor.Risk == OperationRiskDestructive {
		return scopeHubActionCategoryDanger
	}
	switch descriptor.ID {
	case operationScopeActivateContext,
		operationScopeCreateContext,
		operationScopeEditContext,
		operationScopeRenameContext:
		return scopeHubActionCategoryContext
	default:
		return scopeHubActionCategoryContext
	}
}

func scopeHubEntryLabel(entry scopeHubEntry) string {
	return contextScopeDisplayName(entry.Scope)
}

// updateScopeHubActions translates the shared sheet's compact key contract into
// the hub's own notice, confirmation, and inline-editor behavior. Space keeps
// running the highlighted action so the documented open-and-run rhythm survives
// the migration to the shared engine.
func (m *ContextHubModel) updateScopeHubActions(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.actions == nil {
		return m, nil
	}
	if !m.actions.searching {
		switch key.String() {
		case " ", "space":
			key = tea.KeyPressMsg{Code: tea.KeyEnter}
		}
	}
	cmd, dismissed, message := m.actions.handleKey(key)
	if dismissed {
		m.actions = nil
	}
	switch typed := message.(type) {
	case ContextActionSheetClosedMsg:
		m.actions = nil
		return m, cmd
	case ContextActionSheetResultMsg:
		return m, m.applyScopeHubExecutionResult(typed.Result)
	case StatusMsg:
		m.notice = string(typed)
		return m, cmd
	}
	return m, cmd
}

// executeScopeHubBrowseAction routes every documented browse key through the
// centralized executor with a target captured before execution.
func (m *ContextHubModel) executeScopeHubBrowseAction(id OperationID) tea.Cmd {
	entry, ok := m.selectedScopeHubEntry()
	if !ok || m.surface == nil {
		m.notice = "No scope item is selected"
		return nil
	}
	if m.executor == nil {
		m.notice = "Scope actions are unavailable"
		return nil
	}
	target := scopeHubActionTarget{}
	if ok {
		target = scopeHubTarget(entry)
	}
	request := OperationExecutionRequest{
		ID:       id,
		Context:  m.scopeHubActionContext(target),
		Source:   OperationSourceDirectKey,
		Value:    target,
		HasValue: true,
	}
	result := m.surface.ExecuteAction(m.executor, request)
	if errors.Is(result.Err, ErrListActionUnavailable) {
		m.notice = scopeHubBrowseActionUnavailableNotice(id)
		return nil
	}
	return m.applyScopeHubExecutionResult(result)
}

func scopeHubBrowseActionUnavailableNotice(id OperationID) string {
	switch id {
	case operationScopeEditContext:
		return "Edit membership is available only on an editable context row"
	case operationScopeRenameContext:
		return "Rename context is available only on an editable context row"
	case operationScopeDeleteContext:
		return "Delete context is available only on a deletable context row"
	default:
		return "That action is unavailable for this scope item"
	}
}

func (m *ContextHubModel) applyScopeHubExecutionResult(result OperationExecutionResult) tea.Cmd {
	if result.Err != nil {
		m.notice = result.Err.Error()
		return nil
	}
	message := result.AsMessage()
	if inline, ok := message.(scopeHubInlineActionMsg); ok {
		m.applyScopeHubInlineAction(inline)
		return nil
	}
	if message == nil {
		return nil
	}
	return func() tea.Msg { return message }
}

func (m *ContextHubModel) applyScopeHubInlineAction(msg scopeHubInlineActionMsg) {
	switch msg.OperationID {
	case operationScopeCreateContext:
		m.openEditor()
	case operationScopeEditContext:
		m.focusScopeHubContext(msg.Target.Scope)
		m.editSelected()
	case operationScopeRenameContext:
		m.focusScopeHubContext(msg.Target.Scope)
		m.renameSelected()
	case operationScopeDeleteContext:
		m.focusScopeHubContext(msg.Target.Scope)
		m.requestDeleteSelected()
	default:
	}
}

func (m *ContextHubModel) focusScopeHubContext(scope ContextScope) {
	if m.surface != nil {
		m.surface.SetCursorToID(contextScopeListItemID(scope))
	}
}

func (m *ContextHubModel) renderScopeHubActions(width int) string {
	if m.actions == nil {
		return ""
	}
	return m.actions.render(width)
}

func (m *ContextHubModel) scopeHubActionFooterHelpVariants() []string {
	if m.actions == nil {
		return nil
	}
	if m.actions.searching {
		return []string{"Type to filter · Enter keep · Esc clear"}
	}
	if m.actions.surface.Query().Raw != "" {
		return []string{"j/k choose · Enter run · / edit · Esc clear"}
	}
	return []string{
		"j/k choose · Enter run · / search · Esc back",
		"j/k · Enter run · / search · Esc back",
	}
}

// RefreshScopeHub re-reads contexts after an external checked mutation.
func (m *ContextHubModel) RefreshScopeHub() {
	m.rebuildScopeSurface(m.currentScopeHubItemID())
}

func (m *AppModel) refreshScopeHubIfActive() {
	if m == nil || m.manageHub == nil {
		return
	}
	if m.manageHub.Params().Module == ManageProjects {
		m.refreshProjectHubIfActive()
		return
	}
	if m.manageHub.Params().Module != ManageContexts {
		return
	}
	hub, ok := m.manageHub.ActiveModuleView().(*ContextHubModel)
	if !ok {
		return
	}
	hub.RefreshScopeHub()
	if m.crossOutliner != nil {
		hub.SetLoadedProjects(m.crossOutliner.loadedProjects)
	}
}
