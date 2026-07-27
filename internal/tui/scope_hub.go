package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

const (
	scopeHubCategoryBase     ListCategoryID = "scope-hub:base"
	scopeHubCategoryActive   ListCategoryID = "scope-hub:active"
	scopeHubCategoryArchived ListCategoryID = "scope-hub:archived"
	scopeHubCategoryTrash    ListCategoryID = "scope-hub:trash"

	scopeHubActionCategoryContext  ListCategoryID = "scope-hub-actions:context"
	scopeHubActionCategoryProject  ListCategoryID = "scope-hub-actions:project"
	scopeHubActionCategoryRecovery ListCategoryID = "scope-hub-actions:recovery"
	scopeHubActionCategoryDanger   ListCategoryID = "scope-hub-actions:danger"

	scopeHubConfirmPurge = "scope-hub-purge"
)

var scopeHubCategories = []ListCategory{
	{ID: scopeHubCategoryBase, Label: "Base Scopes"},
	{ID: scopeHubCategoryActive, Label: "Active Projects"},
	{ID: scopeHubCategoryArchived, Label: "Archived Projects"},
	{ID: scopeHubCategoryTrash, Label: "Trash"},
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
	scopeHubEntryTrash
)

// scopeHubEntry is the typed, lifecycle-safe payload behind one ListSurface
// row. Trashed generations never masquerade as live projects.
type scopeHubEntry struct {
	Kind    scopeHubEntryKind
	Scope   ContextScope
	Project *models.Project
	Trash   storage.TrashEntry
}

// scopeHubActionTarget freezes stable identity at the moment Space opens the
// action sheet. No later cursor movement can retarget the selected operation.
type scopeHubActionTarget struct {
	Scope        ContextScope
	ProjectID    string
	TrashEntryID string
	TrashSlug    string
}

type scopeHubInlineActionMsg struct {
	OperationID OperationID
	Target      scopeHubActionTarget
}

type scopeHubActionSheet struct {
	surface   *ListSurfaceModel
	target    scopeHubActionTarget
	label     string
	input     textinput.Model
	searching bool
}

const (
	operationScopeCreateContext   OperationID = "tui.scope.create-context"
	operationScopeEditContext     OperationID = "tui.scope.edit-context"
	operationScopeRenameContext   OperationID = "tui.scope.rename-context"
	operationScopeDeleteContext   OperationID = "tui.scope.delete-context"
	operationScopeCreateProject   OperationID = "tui.scope.create-project"
	operationScopeRenameProject   OperationID = "tui.scope.rename-project"
	operationScopeStatusProject   OperationID = "tui.scope.status-project"
	operationScopePinProject      OperationID = "tui.scope.pin-project"
	operationScopeArchiveProject  OperationID = "tui.scope.archive-project"
	operationScopeRestoreProject  OperationID = "tui.scope.restore-project"
	operationScopeOpenProjectNote OperationID = "tui.scope.open-project-notes"
	operationScopeRestoreTrash    OperationID = "tui.scope.restore-trash"
	operationScopePurgeTrash      OperationID = "tui.scope.purge-trash"
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
		descriptor(operationScopeCreateContext, "New custom context", OperationCategoryScope, OperationRiskMutation, "create context"),
		descriptor(operationScopeEditContext, "Edit membership", OperationCategoryScope, OperationRiskMutation, "edit context"),
		descriptor(operationScopeRenameContext, "Rename context", OperationCategoryScope, OperationRiskMutation),
		descriptor(operationScopeDeleteContext, "Delete context", OperationCategoryScope, OperationRiskDestructive),
		descriptor(operationScopeCreateProject, "New project", OperationCategoryProject, OperationRiskMutation, "create project"),
		descriptor(operationScopeRenameProject, "Rename project", OperationCategoryProject, OperationRiskMutation, "edit project"),
		descriptor(operationScopeStatusProject, "Set project status", OperationCategoryProject, OperationRiskMutation),
		descriptor(operationScopePinProject, "Toggle project pin", OperationCategoryProject, OperationRiskMutation, "pin project", "unpin project"),
		descriptor(operationScopeArchiveProject, "Archive project", OperationCategoryProject, OperationRiskDestructive, "unarchive project"),
		descriptor(operationScopeRestoreProject, "Unarchive project", OperationCategoryProject, OperationRiskMutation, "restore archived project"),
		descriptor(operationScopeOpenProjectNote, "Open project notes", OperationCategoryNote, OperationRiskReadOnly),
		descriptor(operationScopeRestoreTrash, "Restore from trash", OperationCategoryProject, OperationRiskMutation, "recover project", "undelete"),
		descriptor(operationScopePurgeTrash, "Permanently delete", OperationCategoryProject, OperationRiskDestructive, "purge trash"),
	}
}

// registerScopeHubOperations makes the same stable operations available to the
// Scope Hub action sheet, harness discovery, and future finder projections.
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
	project := m.projectByID(target.ProjectID)
	switch id {
	case operationScopeCreateContext,
		operationScopeEditContext,
		operationScopeRenameContext,
		operationScopeDeleteContext,
		operationScopePurgeTrash:
		return scopeHubInlineActionMsg{OperationID: id, Target: target}
	case operationScopeCreateProject:
		return CreateProjectMsg{}
	case operationScopeRenameProject:
		if project != nil {
			return EditProjectMsg{Project: project}
		}
	case operationScopeStatusProject:
		if project != nil {
			return ShowStatusIconPickerMsg{Project: project}
		}
	case operationScopePinProject:
		if project != nil {
			return ProjectPinRequestMsg{Project: project, Pinned: !project.Pinned}
		}
	case operationScopeArchiveProject:
		if project != nil {
			return ProjectArchiveRequestMsg{Project: project, Archive: true}
		}
	case operationScopeRestoreProject:
		if project != nil {
			return ProjectArchiveRequestMsg{Project: project, Archive: false}
		}
	case operationScopeOpenProjectNote:
		if project != nil {
			return ShowNotesMsg{Project: project}
		}
	case operationScopeRestoreTrash:
		if target.TrashEntryID != "" {
			return RestoreFromTrashMsg{EntryID: target.TrashEntryID, Slug: target.TrashSlug}
		}
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

// rebuildScopeSurface reloads all four lifecycle categories while preserving a
// stable cursor identity and the current fuzzy query.
func (m *ContextHubModel) rebuildScopeSurface(prefer ListItemID) {
	if m.runtime == nil {
		return
	}
	m.summaries = ContextScopeSummaries(m.runtime)
	trash, err := storage.ListTrashEntries(m.runtime.DataDir())
	if err != nil {
		m.notice = "Trash is unavailable: " + err.Error()
		trash = nil
	}
	items := m.scopeHubItems(trash)
	if m.surface == nil {
		surface, surfaceErr := NewListSurfaceModel(items, scopeHubCategories, ListSurfaceConfig{
			SelectionMode: ListSelectionSingle,
			EmptyState: ListEmptyState{
				Title:  "No scopes or projects are available.",
				Detail: "Create a project to start using workspace scopes.",
			},
			NoMatchState: ListEmptyState{
				Title:  "No contexts match this search.",
				Detail: "Change the query to search scopes, projects, and trash.",
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

func (m *ContextHubModel) scopeHubItems(trash []storage.TrashEntry) []ListItem {
	items := make([]ListItem, 0, len(m.summaries)+len(m.runtime.AllProjects())+len(trash))
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
				operationScopeCreateProject,
				operationScopeRenameProject,
				operationScopeStatusProject,
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
			},
			Data: scopeHubEntry{Kind: scopeHubEntryArchivedProject, Project: project},
		})
	}
	for _, entry := range trash {
		name := entry.Name
		if name == "" {
			name = entry.Slug
		}
		items = append(items, ListItem{
			ID:            ListItemID("scope-hub:trash:" + entry.EntryID),
			CategoryID:    scopeHubCategoryTrash,
			PrimaryText:   name,
			SecondaryText: "deleted " + storage.FormatRelativeTime(entry.DeletedAt),
			Aliases:       []string{name, entry.Slug, entry.EntryID, "trash", "deleted"},
			Preview: ListPreview{
				Title:   name,
				Summary: "Deleted project generation",
				Fields: []ListPreviewField{
					{Label: "Slug", Value: entry.Slug},
					{Label: "Deleted", Value: storage.FormatRelativeTime(entry.DeletedAt)},
					{Label: "Reason", Value: entry.Reason},
					{Label: "Legacy", Value: strconv.FormatBool(entry.Legacy)},
				},
				Body: "Restore keeps the captured project metadata and machine references. Permanent deletion cannot be undone.",
			},
			OperationIDs: []OperationID{operationScopeRestoreTrash, operationScopePurgeTrash},
			Data:         scopeHubEntry{Kind: scopeHubEntryTrash, Trash: entry},
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

func (m *ContextHubModel) scopeHubLifecycleCounts() (active, archived, trash int) {
	if m.surface == nil {
		return len(m.runtime.ActiveProjects()), len(m.runtime.ArchivedProjects()), 0
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
		case scopeHubEntryTrash:
			trash++
		default:
		}
	}
	return active, archived, trash
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
			projectID := entry.Project.ID
			return func() tea.Msg { return ApplyProjectScopeMsg{ProjectID: projectID} }
		}
	case scopeHubEntryArchivedProject:
		m.notice = "Archived projects must be unarchived from Space actions before they can be focused"
	case scopeHubEntryTrash:
		m.notice = "Deleted projects must be restored from Space actions before they can be focused"
	}
	return nil
}

func scopeHubTarget(entry scopeHubEntry) scopeHubActionTarget {
	target := scopeHubActionTarget{Scope: entry.Scope}
	if entry.Project != nil {
		target.ProjectID = entry.Project.ID
	}
	if entry.Kind == scopeHubEntryTrash {
		target.TrashEntryID = entry.Trash.EntryID
		target.TrashSlug = entry.Trash.Slug
	}
	return target
}

func (m *ContextHubModel) openScopeHubActions() {
	entry, ok := m.selectedScopeHubEntry()
	if !ok || m.surface == nil {
		return
	}
	state := m.surface.ActionState()
	descriptors, err := state.Descriptors(m.registry, m.scopeHubCommandContext())
	if err != nil {
		m.notice = err.Error()
		return
	}
	if len(descriptors) == 0 {
		m.notice = "No administration actions are available for this item"
		return
	}
	items := make([]ListItem, 0, len(descriptors))
	for _, descriptor := range descriptors {
		items = append(items, ListItem{
			ID:            ListItemID("scope-hub-action:" + string(descriptor.ID)),
			CategoryID:    scopeHubActionCategory(descriptor),
			PrimaryText:   descriptor.Label,
			SecondaryText: string(descriptor.Risk),
			Aliases:       append([]string(nil), descriptor.Aliases...),
			Preview: ListPreview{
				Title: descriptor.Label,
				Fields: []ListPreviewField{
					{Label: "Risk", Value: string(descriptor.Risk)},
				},
			},
			Data: descriptor,
		})
	}
	surface, err := NewListSurfaceModel(items, scopeHubActionCategories, ListSurfaceConfig{
		SelectionMode: ListSelectionSingle,
		EmptyState:    ListEmptyState{Title: "No actions available."},
		NoMatchState:  ListEmptyState{Title: "No matching actions.", Detail: "Clear or change the search."},
	})
	if err != nil {
		m.notice = err.Error()
		return
	}
	input := textinput.New()
	input.Placeholder = "Search actions..."
	input.CharLimit = 64
	applyThemedTextInput(&input)
	m.actions = &scopeHubActionSheet{
		surface: surface,
		target:  scopeHubTarget(entry),
		label:   scopeHubEntryLabel(entry),
		input:   input,
	}
}

func scopeHubActionCategory(descriptor OperationDescriptor) ListCategoryID {
	if descriptor.Risk == OperationRiskDestructive {
		return scopeHubActionCategoryDanger
	}
	switch descriptor.ID {
	case operationScopeCreateContext, operationScopeEditContext, operationScopeRenameContext:
		return scopeHubActionCategoryContext
	case operationScopeRestoreProject, operationScopeRestoreTrash:
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
	case scopeHubEntryTrash:
		if entry.Trash.Name != "" {
			return entry.Trash.Name
		}
		return entry.Trash.Slug
	}
	return "Scope item"
}

func (m *ContextHubModel) updateScopeHubActions(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	actions := m.actions
	if actions == nil {
		return m, nil
	}
	if actions.searching {
		switch key.String() {
		case "esc":
			actions.searching = false
			actions.input.Blur()
			actions.input.SetValue("")
			actions.surface.SetQuery("")
			return m, nil
		case "enter":
			actions.searching = false
			actions.input.Blur()
			return m, nil
		case "up", "ctrl+k", "ctrl+p":
			actions.surface.MoveCursor(-1)
			return m, nil
		case "down", "ctrl+j", "ctrl+n":
			actions.surface.MoveCursor(1)
			return m, nil
		}
		previous := actions.input.Value()
		var cmd tea.Cmd
		actions.input, cmd = actions.input.Update(key)
		if actions.input.Value() != previous {
			actions.surface.SetQuery(actions.input.Value())
		}
		return m, cmd
	}

	switch key.String() {
	case "esc":
		if actions.surface.Query().Raw != "" {
			actions.input.SetValue("")
			actions.surface.SetQuery("")
			return m, nil
		}
		m.actions = nil
	case "/":
		actions.searching = true
		actions.input.SetValue(actions.surface.Query().Raw)
		actions.input.CursorEnd()
		return m, actions.input.Focus()
	case "j", "down":
		actions.surface.MoveCursor(1)
	case "k", "up":
		actions.surface.MoveCursor(-1)
	case "g", "home":
		actions.surface.MoveCursorToStart()
	case "G", "end":
		actions.surface.MoveCursorToEnd()
	case "enter", " ", "space":
		return m, m.executeScopeHubAction()
	}
	return m, nil
}

func (m *ContextHubModel) executeScopeHubAction() tea.Cmd {
	if m.actions == nil {
		return nil
	}
	item, ok := m.actions.surface.CurrentItem()
	if !ok {
		return nil
	}
	descriptor, ok := item.Data.(OperationDescriptor)
	if !ok {
		m.notice = "Selected scope action is invalid"
		return nil
	}
	target := m.actions.target
	m.actions = nil

	if m.executor == nil {
		m.notice = "Scope actions are unavailable"
		return nil
	}
	result := m.executor.Execute(OperationExecutionRequest{
		ID:       descriptor.ID,
		Context:  m.scopeHubCommandContext(),
		Source:   OperationSourceActionSheet,
		Value:    target,
		HasValue: true,
	})
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
	case operationScopePurgeTrash:
		if msg.Target.TrashEntryID == "" {
			m.notice = "Trash entry is no longer available"
			return
		}
		m.pendingPurge = msg.Target
		m.confirm.Show(
			scopeHubConfirmPurge,
			fmt.Sprintf("Permanently delete %q from trash? This cannot be undone.", msg.Target.TrashSlug),
		)
	default:
	}
}

func (m *ContextHubModel) focusScopeHubContext(scope ContextScope) {
	if m.surface != nil {
		m.surface.SetCursorToID(contextScopeListItemID(scope))
	}
}

func (m *ContextHubModel) renderScopeHubActions(width, height int) string {
	if m.actions == nil {
		return ""
	}
	header := SectionHeaderStyle.Render("Actions for " + m.actions.label)
	if m.actions.searching || m.actions.surface.Query().Raw != "" {
		header += "\n/" + m.actions.input.View()
	}
	contentHeight := max(height-2, 1)
	body := RenderCompactListSurface(m.actions.surface, width, contentHeight)
	return header + "\n\n" + body
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

// RefreshScopeHub re-reads contexts, project lifecycle, and trash after an
// external project editor or checked lifecycle mutation completes.
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
