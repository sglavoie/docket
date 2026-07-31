package tui

import (
	"fmt"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

const projectHubFooterLabel = "Projects"

// The operation ID predates the dedicated Project Hub and remains stable for
// operation-catalog compatibility.
const operationScopeCreateProject OperationID = "tui.scope.create-project"

const (
	projectHubCategoryActive   ListCategoryID = "project-hub:active"
	projectHubCategoryArchived ListCategoryID = "project-hub:archived"

	projectHubActionCategoryProject  ListCategoryID = "project-hub-actions:project"
	projectHubActionCategoryRecovery ListCategoryID = "project-hub-actions:recovery"
	projectHubActionCategoryDanger   ListCategoryID = "project-hub-actions:danger"
)

var projectHubCategories = []ListCategory{
	{ID: projectHubCategoryActive, Label: "Active Projects"},
	{ID: projectHubCategoryArchived, Label: "Archived Projects"},
}

var projectHubActionCategories = []ListCategory{
	{ID: projectHubActionCategoryProject, Label: "Project"},
	{ID: projectHubActionCategoryRecovery, Label: "Recovery"},
	{ID: projectHubActionCategoryDanger, Label: "Danger"},
}

type projectHubEntryKind uint8

const (
	projectHubEntryActive projectHubEntryKind = iota
	projectHubEntryArchived
)

type projectHubEntry struct {
	Kind    projectHubEntryKind
	Project *models.Project
}

// projectHubActionTarget freezes project identity before an action sheet or
// direct project operation executes.
type projectHubActionTarget struct {
	ProjectID string
}

// ProjectHubModel is the search-first surface for selecting and administering
// every active and archived project.
type ProjectHubModel struct {
	BaseSearchableView
	runtime          *config.RuntimeState
	taskCounts       map[string]int
	surface          *ListSurfaceModel
	actions          *ContextActionSheetModel
	registry         *OperationRegistry
	executor         *OperationExecutor
	focusedProjectID string
	notice           string
}

func NewProjectHubModel(runtime *config.RuntimeState) *ProjectHubModel {
	if runtime == nil {
		runtime = config.EmptyRuntimeState("")
	}
	m := &ProjectHubModel{runtime: runtime}
	m.InitSearchInput("Search projects", 64, 32)
	m.rebuildProjectSurface("")
	// Search is active before Init so direct model tests and the first routed
	// keypress both behave as search input.
	_ = m.ActivateSearch()
	return m
}

func (m *ProjectHubModel) Init() tea.Cmd {
	return m.ActivateSearch()
}

func (m *ProjectHubModel) SetOperationSystem(registry *OperationRegistry, executor *OperationExecutor) {
	m.registry = registry
	m.executor = executor
}

func (m *ProjectHubModel) SetProjectFocus(projectID string) {
	if m.focusedProjectID == projectID {
		return
	}
	m.focusedProjectID = projectID
	m.rebuildProjectSurface(m.currentProjectHubItemID())
}

func (m *ProjectHubModel) SetLoadedProjects(projects []*models.Project) {
	counts := make(map[string]int, len(projects))
	for _, project := range projects {
		if project != nil {
			counts[project.ID] = project.CountTasks()
		}
	}
	if len(counts) == 0 {
		m.taskCounts = nil
	} else {
		m.taskCounts = counts
	}
	m.rebuildProjectSurface(m.currentProjectHubItemID())
}

func (m *ProjectHubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if m.actions != nil {
		return m.updateProjectHubActions(key)
	}
	if m.HasActiveSearch() {
		switch key.String() {
		case "enter":
			return m, m.activateProjectHubCurrent()
		case "up", "ctrl+k":
			m.surface.MoveCursor(-1)
			return m, nil
		case "down", "ctrl+j":
			m.surface.MoveCursor(1)
			return m, nil
		case "ctrl+d":
			m.moveProjectHubHalfPage(1)
			return m, nil
		case "ctrl+u":
			m.moveProjectHubHalfPage(-1)
			return m, nil
		}
		if model, cmd, handled := m.HandleSearchMode(key, m, m.applyProjectFilter); handled {
			return model, cmd
		}
	}

	m.notice = ""
	switch key.String() {
	case "/":
		return m, m.ActivateSearch()
	case "esc":
		if m.SearchQuery() != "" {
			m.ClearSearch()
			m.applyProjectFilter()
		}
	case "j", "down":
		m.surface.MoveCursor(1)
	case "k", "up":
		m.surface.MoveCursor(-1)
	case "g", "home":
		m.surface.MoveCursorToStart()
	case "G", "end":
		m.surface.MoveCursorToEnd()
	case "ctrl+d":
		m.moveProjectHubHalfPage(1)
	case "ctrl+u":
		m.moveProjectHubHalfPage(-1)
	case "n":
		return m, m.executeProjectHubCreate()
	case " ", "space":
		m.openProjectHubActions()
	case "enter":
		return m, m.activateProjectHubCurrent()
	}
	return m, nil
}

func (m *ProjectHubModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
}

func (m *ProjectHubModel) applyProjectFilter() {
	if m.surface != nil {
		m.surface.SetQuery(m.SearchQuery())
	}
}

func (m *ProjectHubModel) moveProjectHubHalfPage(direction int) {
	if m.surface == nil || direction == 0 {
		return
	}
	m.surface.MoveCursor(direction * HalfPage(m.surface.Viewport().Height))
}

func (m *ProjectHubModel) currentProjectHubItemID() ListItemID {
	if m.surface == nil {
		return ""
	}
	item, ok := m.surface.CurrentItem()
	if !ok {
		return ""
	}
	return item.ID
}

func (m *ProjectHubModel) rebuildProjectSurface(prefer ListItemID) {
	items := m.projectHubItems()
	if m.surface == nil {
		surface, err := NewListSurfaceModel(items, projectHubCategories, ListSurfaceConfig{
			SelectionMode: ListSelectionSingle,
			EmptyState: ListEmptyState{
				Title:  "No projects are available.",
				Detail: "Press n to create a project.",
			},
			NoMatchState: ListEmptyState{
				Title:  "No projects match this search.",
				Detail: "Press Esc to clear the query and browse every project.",
			},
		})
		if err != nil {
			m.notice = err.Error()
			return
		}
		m.surface = surface
	} else if err := m.surface.Reload(items, projectHubCategories); err != nil {
		m.notice = err.Error()
		return
	}
	m.surface.SetQuery(m.SearchQuery())
	if prefer != "" {
		m.surface.SetCursorToID(prefer)
	}
}

func (m *ProjectHubModel) projectHubItems() []ListItem {
	items := make([]ListItem, 0, len(m.runtime.AllProjects()))
	for _, project := range m.runtime.ActiveProjects() {
		if project == nil {
			continue
		}
		label := project.Name
		if project.ID == m.focusedProjectID {
			label += " (focused)"
		}
		items = append(items, ListItem{
			ID:            ListItemID("project-hub:active:" + project.ID),
			CategoryID:    projectHubCategoryActive,
			PrimaryText:   label,
			SecondaryText: m.projectSecondary(project),
			Aliases:       []string{project.Name, project.Slug, project.Context, "active project"},
			Preview:       m.projectPreview(project, false),
			OperationIDs: []OperationID{
				operationProjectOpen,
				operationScopeCreateProject,
				operationScopeRenameProject,
				operationScopePinProject,
				operationScopeArchiveProject,
				operationScopeOpenProjectNote,
			},
			Data: projectHubEntry{Kind: projectHubEntryActive, Project: project},
		})
	}
	for _, project := range m.runtime.ArchivedProjects() {
		if project == nil {
			continue
		}
		items = append(items, ListItem{
			ID:            ListItemID("project-hub:archived:" + project.ID),
			CategoryID:    projectHubCategoryArchived,
			PrimaryText:   project.Name,
			SecondaryText: "archived · " + project.Context,
			Aliases:       []string{project.Name, project.Slug, project.Context, "archived project"},
			Preview:       m.projectPreview(project, true),
			OperationIDs: []OperationID{
				operationScopeCreateProject,
				operationScopeRenameProject,
				operationScopeRestoreProject,
				operationScopeOpenProjectNote,
			},
			Data: projectHubEntry{Kind: projectHubEntryArchived, Project: project},
		})
	}
	return items
}

func (m *ProjectHubModel) projectPreview(project *models.Project, archived bool) ListPreview {
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
			{Label: "Tasks", Value: strconv.Itoa(m.projectTaskCount(project))},
			{Label: "Pinned", Value: strconv.FormatBool(project.Pinned)},
			{Label: "Slug", Value: project.Slug},
		},
	}
}

func (m *ProjectHubModel) projectTaskCount(project *models.Project) int {
	if project == nil {
		return 0
	}
	if count, ok := m.taskCounts[project.ID]; ok {
		return count
	}
	return project.CountTasks()
}

func (m *ProjectHubModel) projectSecondary(project *models.Project) string {
	context := project.Context
	if context == "" {
		context = "no context"
	}
	return fmt.Sprintf("%s · %d tasks", context, m.projectTaskCount(project))
}

func (m *ProjectHubModel) selectedProjectHubEntry() (projectHubEntry, bool) {
	if m.surface == nil {
		return projectHubEntry{}, false
	}
	item, ok := m.surface.CurrentItem()
	if !ok {
		return projectHubEntry{}, false
	}
	entry, ok := item.Data.(projectHubEntry)
	return entry, ok
}

func (m *ProjectHubModel) activateProjectHubCurrent() tea.Cmd {
	entry, ok := m.selectedProjectHubEntry()
	if !ok || entry.Project == nil {
		return nil
	}
	if entry.Kind == projectHubEntryArchived {
		m.notice = "Archived projects must be unarchived from Space actions before they can be focused"
		return nil
	}
	return m.executeProjectOperation(operationProjectOpen, entry.Project.ID)
}

func (m *ProjectHubModel) projectHubCommandContext(projectID string) CommandContext {
	ctx := PresentationCommandContext(ViewProjectHub)
	ctx.Capabilities.Add(CapabilityProjectHub)
	if projectID != "" {
		ctx.CurrentEntity = ProjectEntity(projectID)
		ctx.FocusedProjectID = projectID
	}
	return ctx
}

func (m *ProjectHubModel) executeProjectOperation(id OperationID, projectID string) tea.Cmd {
	if m.executor == nil {
		m.notice = "Project actions are unavailable"
		return nil
	}
	result := m.executor.Execute(OperationExecutionRequest{
		ID:      id,
		Context: m.projectHubCommandContext(projectID),
		Source:  OperationSourceDirectKey,
	})
	return m.applyProjectHubExecutionResult(result)
}

func (m *ProjectHubModel) executeProjectHubCreate() tea.Cmd {
	if m.executor == nil {
		m.notice = "Project actions are unavailable"
		return nil
	}
	target := projectHubActionTarget{}
	result := m.executor.Execute(OperationExecutionRequest{
		ID:       operationScopeCreateProject,
		Context:  m.projectHubCommandContext(""),
		Source:   OperationSourceDirectKey,
		Value:    target,
		HasValue: true,
	})
	return m.applyProjectHubExecutionResult(result)
}

func (m *ProjectHubModel) openProjectHubActions() {
	entry, ok := m.selectedProjectHubEntry()
	if !ok || entry.Project == nil || m.surface == nil {
		return
	}
	target := projectHubActionTarget{ProjectID: entry.Project.ID}
	sheet, err := NewContextActionSheetModel(
		m.registry,
		m.executor,
		m.projectHubCommandContext(target.ProjectID),
		m.surface.ActionState().OperationIDs,
		ContextActionSheetConfig{
			Host:         ContextActionHostProjectHub,
			Completion:   ContextActionCompleteCloseSheet,
			Title:        "Actions",
			Subtitle:     entry.Project.Name,
			Categories:   projectHubActionCategories,
			CategoryFor:  projectHubActionCategory,
			SecondaryFor: func(descriptor OperationDescriptor) string { return string(descriptor.Risk) },
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
		m.notice = "No administration actions are available for this project"
		return
	}
	m.actions = sheet
}

func projectHubActionCategory(descriptor OperationDescriptor) ListCategoryID {
	if descriptor.Risk == OperationRiskDestructive {
		return projectHubActionCategoryDanger
	}
	if descriptor.ID == operationScopeRestoreProject {
		return projectHubActionCategoryRecovery
	}
	return projectHubActionCategoryProject
}

func (m *ProjectHubModel) updateProjectHubActions(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if !m.actions.searching && (key.String() == " " || key.String() == "space") {
		key = tea.KeyPressMsg{Code: tea.KeyEnter}
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
		return m, m.applyProjectHubExecutionResult(typed.Result)
	case StatusMsg:
		m.notice = string(typed)
	}
	return m, cmd
}

func (m *ProjectHubModel) applyProjectHubExecutionResult(result OperationExecutionResult) tea.Cmd {
	if result.Err != nil {
		m.notice = result.Err.Error()
		return nil
	}
	message := result.AsMessage()
	if message == nil {
		return nil
	}
	return func() tea.Msg { return message }
}

func (m *ProjectHubModel) View() tea.View {
	width := max(m.width, 1)
	height := max(m.height, 1)
	title := renderFullscreenTitle("Select Project", m.subtitle(), m.AccentColor())
	header := title
	if search := m.renderSearchBar(); search != "" {
		header = lipgloss.JoinVertical(lipgloss.Left, title, search)
	}
	footer := renderStatusBarFooter(
		HelpStyle.Render(m.footerHelp(width)),
		MutedStyle.Render(projectHubFooterLabel),
		width,
		m.AccentColor(),
	)
	contentHeight := fullscreenContentHeightForBlocks(height, header, footer)
	var content string
	if m.actions != nil {
		content = m.actions.render(width)
	} else {
		content = NewListSurfaceRenderer(ListSurfaceRenderConfig{
			Width:       width,
			Height:      contentHeight,
			Layout:      ListSurfaceLayoutAuto,
			ShowPreview: true,
			HideHelp:    true,
		}).Render(m.surface).Content
	}
	if m.notice != "" {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			MutedStyle.Render(truncateToVisualWidth(m.notice, width)),
			content,
		)
	}
	return tea.NewView(lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		fitBlockHeight(content, contentHeight),
		footer,
	))
}

func (m *ProjectHubModel) subtitle() string {
	return fmt.Sprintf(
		"%d active · %d archived",
		len(m.runtime.ActiveProjects()),
		len(m.runtime.ArchivedProjects()),
	)
}

func (m *ProjectHubModel) renderSearchBar() string {
	if m.HasActiveSearch() {
		return "/" + m.SearchInputView()
	}
	if m.SearchQuery() == "" {
		return ""
	}
	return "/" + m.SearchQuery() + MutedStyle.Render(fmt.Sprintf("  (%d matches)", len(m.surface.Matches())))
}

func (m *ProjectHubModel) footerHelp(width int) string {
	var variants []string
	switch {
	case m.actions != nil && m.actions.searching:
		variants = []string{"Type to filter · Enter keep · Esc clear"}
	case m.actions != nil:
		variants = []string{"j/k choose · Enter run · / search · Esc back", "Enter run · Esc back"}
	case m.HasActiveSearch():
		variants = []string{
			"Type to filter · ↑↓/Ctrl+j/k move · Ctrl+d/u half page · Enter select · Esc clear and browse",
			"↑↓/Ctrl+j/k move · Enter select · Esc browse",
			"Enter select · Esc browse",
		}
	case m.SearchQuery() != "":
		variants = []string{"j/k choose · Enter select · Space actions · Esc clear", "Enter · Space · Esc clear"}
	default:
		variants = []string{
			"j/k choose · Enter select · Space actions · n new · / search",
			"j/k · Enter · Space · n · /",
			"Enter · Space · /",
		}
	}
	budget := width - lipgloss.Width(projectHubFooterLabel) - 1
	for _, variant := range variants {
		if lipgloss.Width(variant) <= budget {
			return variant
		}
	}
	return variants[len(variants)-1]
}

func (m *ProjectHubModel) HasTextInput() bool {
	return m.actions != nil || m.DefaultHasTextInput()
}

func (m *ProjectHubModel) ShouldCaptureEsc() bool {
	return m.actions != nil || m.SearchShouldCaptureEsc() || m.SearchQuery() != ""
}

func (m *ProjectHubModel) IsFullHeight() bool { return true }

func (m *ProjectHubModel) StatusBarLabel() string { return "Projects" }

func (m *ProjectHubModel) GetHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "j/k", Description: "Move selection", Category: "Navigation"},
		{Key: "Ctrl+d/u", Description: "Move half a page down/up", Category: "Navigation"},
		{Key: "Enter", Description: "Focus the selected active project", Category: "Actions"},
		{Key: "Space", Description: "Open project administration actions", Category: "Actions"},
		{Key: "n", Description: "Create a project", Category: "Actions"},
		{Key: "/", Description: "Search projects", Category: "Filters"},
		{Key: "Esc", Description: "Clear search, then close", Category: "Navigation"},
	}
}

func (m *ProjectHubModel) RefreshProjectHub() {
	m.rebuildProjectSurface(m.currentProjectHubItemID())
}

func projectHubOperationDescriptors() []OperationDescriptor {
	return []OperationDescriptor{{
		ID:                   operationScopeCreateProject,
		Label:                "New project",
		Aliases:              []string{"create project"},
		Category:             OperationCategoryProject,
		TargetSource:         OperationTargetGlobal,
		TargetCardinality:    TargetCardinalityNone,
		RequiredCapabilities: NewCapabilitySet(CapabilityProjectHub),
		Risk:                 OperationRiskMutation,
	}}
}

func (m *AppModel) registerProjectHubOperations() error {
	for _, descriptor := range projectHubOperationDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities: NewCapabilitySet(CapabilityProjectHub),
			Handler: func(invocation OperationInvocation) OperationHandlerResult {
				if _, ok := invocation.Value.(projectHubActionTarget); !ok {
					return OperationHandlerResult{Err: fmt.Errorf("project creation has no frozen hub target")}
				}
				return OperationHandlerResult{Message: CreateProjectMsg{}}
			},
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

func (m *AppModel) refreshProjectHubIfActive() {
	if m == nil || m.manageHub == nil || m.manageHub.Params().Module != ManageProjects {
		return
	}
	hub, ok := m.manageHub.ActiveModuleView().(*ProjectHubModel)
	if !ok {
		return
	}
	hub.RefreshProjectHub()
	if m.crossOutliner != nil {
		hub.SetLoadedProjects(m.crossOutliner.loadedProjects)
		hub.SetProjectFocus(m.crossOutliner.scopedProjectID)
	}
}
