package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

const (
	scopeHubCategoryBase     ListCategoryID = "scope-hub:base"
	scopeHubCategoryActive   ListCategoryID = "scope-hub:active"
	scopeHubCategoryArchived ListCategoryID = "scope-hub:archived"

	scopeHubActionCategoryContext  ListCategoryID = "scope-hub-actions:context"
	scopeHubActionCategoryProject  ListCategoryID = "scope-hub-actions:project"
	scopeHubActionCategoryRecovery ListCategoryID = "scope-hub-actions:recovery"
	scopeHubActionCategoryDanger   ListCategoryID = "scope-hub-actions:danger"
)

var scopeHubCategories = []ListCategory{
	{ID: scopeHubCategoryBase, Label: "Base Scopes"},
	{ID: scopeHubCategoryActive, Label: "Active Projects"},
	{ID: scopeHubCategoryArchived, Label: "Archived Projects"},
}

var scopeHubActionCategories = []ListCategory{
	{ID: scopeHubActionCategoryContext, Label: "Context"},
	{ID: scopeHubActionCategoryProject, Label: "Project"},
	{ID: scopeHubActionCategoryRecovery, Label: "Recovery"},
	{ID: scopeHubActionCategoryDanger, Label: "Danger"},
}

type scopeHubEntryKind uint8

const (
	scopeHubEntryBase scopeHubEntryKind = iota
	scopeHubEntryActiveProject
	scopeHubEntryArchivedProject
)

// scopeHubEntry is the typed, lifecycle-safe payload behind one ListSurface row.
type scopeHubEntry struct {
	Kind    scopeHubEntryKind
	Scope   ContextScope
	Project *models.Project
}

// scopeHubActionTarget freezes stable identity at the moment Space opens the
// action sheet. No later cursor movement can retarget the selected operation.
type scopeHubActionTarget struct {
	Scope     ContextScope
	ProjectID string
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
	operationScopeCreateProject OperationID = "tui.scope.create-project"
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
		descriptor(operationScopeCreateContext, "New custom context", OperationCategoryScope, OperationRiskMutation, "create context"),
		descriptor(operationScopeEditContext, "Edit membership", OperationCategoryScope, OperationRiskMutation, "edit context"),
		descriptor(operationScopeRenameContext, "Rename context", OperationCategoryScope, OperationRiskMutation),
		descriptor(operationScopeDeleteContext, "Delete context", OperationCategoryScope, OperationRiskDestructive),
		descriptor(operationScopeCreateProject, "New project", OperationCategoryProject, OperationRiskMutation, "create project"),
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
			IsCustomContext: target.Scope.IsCustom(),
		}
	case operationScopeCreateContext,
		operationScopeEditContext,
		operationScopeRenameContext,
		operationScopeDeleteContext:
		return scopeHubInlineActionMsg{OperationID: id, Target: target}
	case operationScopeCreateProject:
		return CreateProjectMsg{}
	default:
	}
	return ErrorMsg{Err: fmt.Errorf("scope action target is no longer available")}
}

func (m *ContextHubModel) SetOperationSystem(registry *OperationRegistry, executor *OperationExecutor) {
	m.registry = registry
	m.executor = executor
}

func (m *ContextHubModel) SetProjectFocus(projectID string) {
	if m.focusedProjectID == projectID {
		return
	}
	m.focusedProjectID = projectID
	m.rebuildScopeSurface(m.currentScopeHubItemID())
}

func (m *ContextHubModel) scopeHubCommandContext() CommandContext {
	ctx := PresentationCommandContext(ViewContextHub)
	ctx.Capabilities.Add(CapabilityScopeHub)
	return ctx
}

// scopeHubActionContext carries the frozen row identity into the shared
// operation pipeline. Project operations resolve their target from it rather
// than from the live cursor, so a later rebuild cannot retarget them.
func (m *ContextHubModel) scopeHubActionContext(target scopeHubActionTarget) CommandContext {
	ctx := m.scopeHubCommandContext()
	if target.ProjectID != "" {
		ctx.CurrentEntity = ProjectEntity(target.ProjectID)
		ctx.FocusedProjectID = target.ProjectID
	}
	return ctx
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

// rebuildScopeSurface reloads the scope and lean project lifecycle categories while preserving a
// stable cursor identity and the current fuzzy query.
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
				Title:  "No scopes or projects are available.",
				Detail: "Create a project to start using workspace scopes.",
			},
			NoMatchState: ListEmptyState{
				Title:  "No contexts match this search.",
				Detail: "Change the query to search scopes and projects.",
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
	items := make([]ListItem, 0, len(m.summaries)+len(m.runtime.AllProjects()))
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
		operations := []OperationID{operationScopeCreateContext, operationScopeCreateProject}
		if !scope.IsAll() {
			operations = append(operations,
				operationScopeEditContext,
				operationScopeRenameContext,
			)
			if scope.IsCustom() || !strings.EqualFold(scope.Name, m.runtime.DefaultContext()) {
				operations = append(operations, operationScopeDeleteContext)
			}
		}
		items = append(items, ListItem{
			ID:            contextScopeListItemID(scope),
			CategoryID:    scopeHubCategoryBase,
			PrimaryText:   label,
			SecondaryText: contextHubRowDetail(summary),
			Aliases:       aliases,
			Preview:       m.scopeHubContextPreview(summary),
			OperationIDs:  operations,
			Data:          scopeHubEntry{Kind: scopeHubEntryBase, Scope: scope},
		})
	}

	for _, project := range m.runtime.ActiveProjects() {
		if project == nil {
			continue
		}
		label := project.Name
		if m.scopeHubProjectFocused(project.ID) {
			label += " (focused)"
		}
		items = append(items, ListItem{
			ID:            ListItemID("scope-hub:active:" + project.ID),
			CategoryID:    scopeHubCategoryActive,
			PrimaryText:   label,
			SecondaryText: m.scopeHubProjectSecondary(project),
			Aliases:       []string{project.Name, project.Slug, project.Context, "active project"},
			Preview:       m.scopeHubProjectPreview(project, false),
			OperationIDs: []OperationID{
				operationProjectOpen,
				operationScopeCreateProject,
				operationScopeRenameProject,
				operationScopePinProject,
				operationScopeArchiveProject,
				operationScopeOpenProjectNote,
			},
			Data: scopeHubEntry{Kind: scopeHubEntryActiveProject, Project: project},
		})
	}
	for _, project := range m.runtime.ArchivedProjects() {
		if project == nil {
			continue
		}
		items = append(items, ListItem{
			ID:            ListItemID("scope-hub:archived:" + project.ID),
			CategoryID:    scopeHubCategoryArchived,
			PrimaryText:   project.Name,
			SecondaryText: "archived · " + project.Context,
			Aliases:       []string{project.Name, project.Slug, project.Context, "archived project"},
			Preview:       m.scopeHubProjectPreview(project, true),
			OperationIDs: []OperationID{
				operationScopeRenameProject,
				operationScopeRestoreProject,
				operationScopeOpenProjectNote,
			},
			Data: scopeHubEntry{Kind: scopeHubEntryArchivedProject, Project: project},
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

func (m *ContextHubModel) scopeHubProjectPreview(project *models.Project, archived bool) ListPreview {
	lifecycle := "Active"
	if archived {
		lifecycle = "Archived"
	}
	return ListPreview{
		Title:   project.Name,
		Summary: project.Description,
		Fields: []ListPreviewField{
			{Label: "Lifecycle", Value: lifecycle},
			{Label: "Context", Value: project.Context},
			{Label: "Tasks", Value: strconv.Itoa(m.scopeHubTaskCount(project))},
			{Label: "Pinned", Value: strconv.FormatBool(project.Pinned)},
			{Label: "Slug", Value: project.Slug},
		},
	}
}

func (m *ContextHubModel) scopeHubTaskCount(project *models.Project) int {
	if project == nil {
		return 0
	}
	if count, ok := m.taskCounts[project.ID]; ok {
		return count
	}
	return project.CountTasks()
}

func (m *ContextHubModel) scopeHubProjectSecondary(project *models.Project) string {
	context := project.Context
	if context == "" {
		context = "no context"
	}
	return fmt.Sprintf("%s · %d tasks", context, m.scopeHubTaskCount(project))
}

func (m *ContextHubModel) scopeHubProjectFocused(projectID string) bool {
	return m.activeProjectFocusID() == projectID
}

// activeProjectFocusID is overridden by SetProjectFocus below; keeping it on
// the hub avoids reaching into the Outliner from this renderer.
func (m *ContextHubModel) activeProjectFocusID() string {
	return m.focusedProjectID
}

func (m *ContextHubModel) scopeHubLifecycleCounts() (active, archived int) {
	if m.surface == nil {
		return len(m.runtime.ActiveProjects()), len(m.runtime.ArchivedProjects())
	}
	for _, item := range m.surface.Items() {
		entry, ok := item.Data.(scopeHubEntry)
		if !ok {
			continue
		}
		switch entry.Kind {
		case scopeHubEntryActiveProject:
			active++
		case scopeHubEntryArchivedProject:
			archived++
		default:
		}
	}
	return active, archived
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
				IsCustomContext: scope.IsCustom(),
			}
		}
	case scopeHubEntryActiveProject:
		if entry.Project != nil {
			return m.executeScopeHubProjectOperation(operationProjectOpen, entry)
		}
	case scopeHubEntryArchivedProject:
		m.notice = "Archived projects must be unarchived from Space actions before they can be focused"
	}
	return nil
}

// executeScopeHubProjectOperation runs a project operation from a browse key
// through the same executor path the action sheet uses, so Enter and Space
// share one handler instead of emitting their own message.
func (m *ContextHubModel) executeScopeHubProjectOperation(id OperationID, entry scopeHubEntry) tea.Cmd {
	if m.executor == nil {
		m.notice = "Scope actions are unavailable"
		return nil
	}
	target := scopeHubTarget(entry)
	return m.applyScopeHubExecutionResult(m.executor.Execute(OperationExecutionRequest{
		ID:      id,
		Context: m.scopeHubActionContext(target),
		Source:  OperationSourceDirectKey,
	}))
}

func scopeHubTarget(entry scopeHubEntry) scopeHubActionTarget {
	target := scopeHubActionTarget{Scope: entry.Scope}
	if entry.Project != nil {
		target.ProjectID = entry.Project.ID
	}
	return target
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
	case operationScopeRestoreProject:
		return scopeHubActionCategoryRecovery
	default:
		return scopeHubActionCategoryProject
	}
}

func scopeHubEntryLabel(entry scopeHubEntry) string {
	switch entry.Kind {
	case scopeHubEntryBase:
		return contextScopeDisplayName(entry.Scope)
	case scopeHubEntryActiveProject, scopeHubEntryArchivedProject:
		if entry.Project != nil {
			return entry.Project.Name
		}
	}
	return "Scope item"
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
// centralized executor with a target captured before execution. Creating a
// context is global and therefore works with an empty target when no row is
// visible; the edit, rename, and delete actions remain gated by the selected
// row.
func (m *ContextHubModel) executeScopeHubBrowseAction(id OperationID) tea.Cmd {
	entry, ok := m.selectedScopeHubEntry()
	if id != operationScopeCreateContext && (!ok || m.surface == nil) {
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
	var result OperationExecutionResult
	if id == operationScopeCreateContext {
		result = m.executor.Execute(request)
	} else {
		result = m.surface.ExecuteAction(m.executor, request)
	}
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
		m.openEditor("")
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

// RefreshScopeHub re-reads contexts and project lifecycle after an external
// project editor or checked lifecycle mutation completes.
func (m *ContextHubModel) RefreshScopeHub() {
	m.rebuildScopeSurface(m.currentScopeHubItemID())
}

func (m *AppModel) refreshScopeHubIfActive() {
	if m == nil || m.manageHub == nil || m.manageHub.Params().Module != ManageContexts {
		return
	}
	hub, ok := m.manageHub.ActiveModuleView().(*ContextHubModel)
	if !ok {
		return
	}
	hub.RefreshScopeHub()
	if m.crossOutliner != nil {
		hub.SetLoadedProjects(m.crossOutliner.loadedProjects)
		hub.SetProjectFocus(m.crossOutliner.scopedProjectID)
	}
}
