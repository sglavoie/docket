package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

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

	// projectHubCategoryBoundaryPrefix namespaces one list category per custom
	// boundary. The durable boundary ID is the suffix, so a rename relabels a
	// group without moving any row out of it.
	projectHubCategoryBoundaryPrefix = "project-hub:boundary:"

	projectHubActionCategoryProject  ListCategoryID = "project-hub-actions:project"
	projectHubActionCategoryRecovery ListCategoryID = "project-hub-actions:recovery"
	projectHubActionCategoryDanger   ListCategoryID = "project-hub-actions:danger"
)

// Row IDs are keyed by lifecycle rather than boundary so a project keeps the
// same identity — and therefore the same cursor anchor — when it is parked.
const (
	projectHubItemPrefix         = "project-hub:project:"
	projectHubArchivedItemPrefix = "project-hub:archived:"
)

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
	filterPicker     *projectHubFilterPicker
	boundaries       *projectBoundaryManager
	registry         *OperationRegistry
	executor         *OperationExecutor
	focusedProjectID string
	// boundaryFilter narrows the listing to one group for as long as this Hub
	// instance lives. It is keyed by durable boundary ID (plus the two sentinels
	// for All and Archived) so a rename cannot invalidate it, and it is never
	// persisted: the Manage host rebuilds the Hub on every open, which is what
	// makes each visit start at All.
	boundaryFilter string
	notice         string
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
	if m.boundaries != nil {
		return m.updateBoundaryManager(key)
	}
	if m.filterPicker != nil {
		return m.updateProjectHubFilterPicker(key)
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
	case "f":
		m.openProjectHubFilterPicker()
	case "B":
		m.openBoundaryManager()
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
	if m.boundaries != nil {
		m.boundaries.resize(width)
	}
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
	m.reconcileBoundaryFilter()
	categories := m.projectHubCategories()
	items := m.projectHubItems()
	if m.surface == nil {
		surface, err := NewListSurfaceModel(items, categories, ListSurfaceConfig{
			SelectionMode: ListSelectionSingle,
			EmptyState:    m.projectHubEmptyState(),
			NoMatchState:  m.projectHubNoMatchState(),
		})
		if err != nil {
			m.notice = err.Error()
			return
		}
		m.surface = surface
	} else if err := m.surface.Reload(items, categories); err != nil {
		m.notice = err.Error()
		return
	}
	m.surface.SetQuery(m.SearchQuery())
	if prefer != "" {
		m.surface.SetCursorToID(prefer)
	}
}

// projectHubCategories orders the listing groups: the base task workspace
// first, then every custom boundary by name, then the archived tail. Building
// them from the catalog on each rebuild is what lets a created, renamed, or
// deleted boundary reach the Hub without any separate invalidation.
func (m *ProjectHubModel) projectHubCategories() []ListCategory {
	boundaries := m.customBoundaries()
	categories := make([]ListCategory, 0, len(boundaries)+2)
	categories = append(categories, ListCategory{ID: projectHubCategoryActive, Label: "Active Projects"})
	for _, boundary := range boundaries {
		categories = append(categories, ListCategory{
			ID:    projectHubBoundaryCategory(boundary.ID),
			Label: boundary.Name,
		})
	}
	return append(categories, ListCategory{ID: projectHubCategoryArchived, Label: "Archived Projects"})
}

// customBoundaries returns every boundary except the reserved Active one,
// ordered case-insensitively by name with the durable ID breaking ties so two
// boundaries that differ only by case still order deterministically.
func (m *ProjectHubModel) customBoundaries() []config.ProjectBoundary {
	catalog := m.runtime.Catalog()
	boundaries := make([]config.ProjectBoundary, 0, len(catalog.Boundaries))
	for _, boundary := range catalog.Boundaries {
		if boundary.ID == models.ActiveBoundaryID {
			continue
		}
		boundaries = append(boundaries, boundary)
	}
	slices.SortFunc(boundaries, func(a, b config.ProjectBoundary) int {
		if order := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	return boundaries
}

func projectHubBoundaryCategory(boundaryID string) ListCategoryID {
	if boundaryID == "" || boundaryID == models.ActiveBoundaryID {
		return projectHubCategoryActive
	}
	return ListCategoryID(projectHubCategoryBoundaryPrefix + boundaryID)
}

// projectBoundary resolves the group one project is listed under. An unassigned
// project and one naming a boundary the catalog no longer defines both resolve
// to Active, so a row can never reference a category the surface lacks.
func (m *ProjectHubModel) projectBoundary(projectID string) config.ProjectBoundary {
	active := config.ProjectBoundary{ID: models.ActiveBoundaryID, Name: config.ActiveBoundaryName}
	catalog := m.runtime.Catalog()
	boundaryID := catalog.ProjectBoundaryID(projectID)
	if boundaryID == "" || boundaryID == models.ActiveBoundaryID {
		return active
	}
	boundary, ok := catalog.Boundary(boundaryID)
	if !ok || boundary.Name == "" {
		return active
	}
	return boundary
}

func (m *ProjectHubModel) projectHubItems() []ListItem {
	items := make([]ListItem, 0, len(m.runtime.AllProjects()))
	for _, project := range m.runtime.AvailableProjects() {
		if project == nil {
			continue
		}
		boundary := m.projectBoundary(project.ID)
		if !m.boundaryFilterAdmits(boundary.ID, false) {
			continue
		}
		label := project.Name
		if project.ID == m.focusedProjectID {
			label += " (focused)"
		}
		items = append(items, ListItem{
			ID:            ListItemID(projectHubItemPrefix + project.ID),
			CategoryID:    projectHubBoundaryCategory(boundary.ID),
			PrimaryText:   label,
			SecondaryText: m.projectSecondary(project),
			Aliases:       []string{project.Name, project.Slug, project.Context, boundary.Name, "active project"},
			Preview:       m.projectPreview(project, boundary, false),
			OperationIDs: []OperationID{
				operationProjectOpen,
				operationScopeCreateProject,
				operationScopeRenameProject,
				operationProjectSetBoundary,
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
		boundary := m.projectBoundary(project.ID)
		if !m.boundaryFilterAdmits(boundary.ID, true) {
			continue
		}
		items = append(items, ListItem{
			ID:          ListItemID(projectHubArchivedItemPrefix + project.ID),
			CategoryID:  projectHubCategoryArchived,
			PrimaryText: project.Name,
			SecondaryText: "archived" + scopeBoundarySeparator +
				projectHubContextLabel(project) + scopeBoundarySeparator + boundary.Name,
			Aliases: []string{project.Name, project.Slug, project.Context, boundary.Name, "archived project"},
			Preview: m.projectPreview(project, boundary, true),
			OperationIDs: []OperationID{
				operationScopeCreateProject,
				operationScopeRenameProject,
				operationProjectSetBoundary,
				operationScopeRestoreProject,
			},
			Data: projectHubEntry{Kind: projectHubEntryArchived, Project: project},
		})
	}
	return items
}

func (m *ProjectHubModel) projectPreview(
	project *models.Project,
	boundary config.ProjectBoundary,
	archived bool,
) ListPreview {
	lifecycle := "Active"
	if archived {
		lifecycle = "Archived"
	}
	return ListPreview{
		Title:   project.Name,
		Summary: project.Description,
		Fields: []ListPreviewField{
			{Label: "Lifecycle", Value: lifecycle},
			{Label: "Boundary", Value: boundary.Name},
			{Label: "Context", Value: project.Context},
			{Label: "Tasks", Value: m.projectTaskCount(project)},
			{Label: "Pinned", Value: strconv.FormatBool(project.Pinned)},
			{Label: "Slug", Value: project.Slug},
		},
	}
}

// projectTaskCount reports the loaded task count, or unknown. A project outside
// the base task workspace is not loaded, and its rehydrated catalog record
// counts zero tasks; reporting that zero would read as "empty" rather than "not
// counted", so an unloaded project is never given a number.
func (m *ProjectHubModel) projectTaskCount(project *models.Project) string {
	if project == nil {
		return projectTaskCountUnknown
	}
	if count, ok := m.taskCounts[project.ID]; ok {
		return strconv.Itoa(count)
	}
	return projectTaskCountLabel(project)
}

func (m *ProjectHubModel) projectSecondary(project *models.Project) string {
	context := projectHubContextLabel(project)
	count := m.projectTaskCount(project)
	if count == projectTaskCountUnknown {
		return context + scopeBoundarySeparator + "tasks not loaded"
	}
	return fmt.Sprintf("%s%s%s tasks", context, scopeBoundarySeparator, count)
}

func projectHubContextLabel(project *models.Project) string {
	if project.Context == "" {
		return "no context"
	}
	return project.Context
}

// boundaryFilterAdmits reports whether one row survives the active filter.
// Archived is a lifecycle group beside the boundary groups rather than inside
// one: an archived project keeps its retained boundary on the row, but a
// boundary filter lists only the projects that boundary currently holds.
func (m *ProjectHubModel) boundaryFilterAdmits(boundaryID string, archived bool) bool {
	if m.boundaryFilter == projectHubFilterAll {
		return true
	}
	if m.boundaryFilter == projectHubFilterArchived {
		return archived
	}
	return !archived && boundaryID == m.boundaryFilter
}

// reconcileBoundaryFilter drops a filter whose boundary is gone. Deleting a
// boundary moves its members elsewhere, so keeping the filter would leave the
// Hub permanently empty with no visible cause.
func (m *ProjectHubModel) reconcileBoundaryFilter() {
	switch m.boundaryFilter {
	case projectHubFilterAll, projectHubFilterArchived, models.ActiveBoundaryID:
		return
	}
	if _, ok := m.runtime.Catalog().Boundary(m.boundaryFilter); ok {
		return
	}
	m.boundaryFilter = projectHubFilterAll
	m.notice = "Boundary filter cleared: that boundary no longer exists"
}

// boundaryFilterLabel names the active filter for Hub chrome.
func (m *ProjectHubModel) boundaryFilterLabel() string {
	if m.boundaryFilter == projectHubFilterAll {
		return "All"
	}
	if m.boundaryFilter == projectHubFilterArchived {
		return "Archived"
	}
	if m.boundaryFilter == models.ActiveBoundaryID {
		return config.ActiveBoundaryName
	}
	if boundary, ok := m.runtime.Catalog().Boundary(m.boundaryFilter); ok {
		return boundary.Name
	}
	return "All"
}

func (m *ProjectHubModel) projectHubEmptyState() ListEmptyState {
	if m.boundaryFilter == projectHubFilterAll {
		return ListEmptyState{
			Title:  "No projects are available.",
			Detail: "Press n to create a project.",
		}
	}
	return ListEmptyState{
		Title:  fmt.Sprintf("No projects are in %s.", m.boundaryFilterLabel()),
		Detail: "Press f to choose another boundary.",
	}
}

func (m *ProjectHubModel) projectHubNoMatchState() ListEmptyState {
	if m.boundaryFilter == projectHubFilterAll {
		return ListEmptyState{
			Title:  "No projects match this search.",
			Detail: "Press Esc to clear the query and browse every project.",
		}
	}
	return ListEmptyState{
		Title:  fmt.Sprintf("No projects in %s match this search.", m.boundaryFilterLabel()),
		Detail: "Press Esc to clear the query or f to widen the boundary filter.",
	}
}

// boundaryFilterOptions builds the picker rows: All, the base task workspace,
// every custom boundary in listing order, then Archived.
func (m *ProjectHubModel) boundaryFilterOptions() []projectHubFilterOption {
	boundaries := m.customBoundaries()
	counts := make(map[string]int, len(boundaries)+1)
	available := m.runtime.AvailableProjects()
	for _, project := range available {
		if project != nil {
			counts[m.projectBoundary(project.ID).ID]++
		}
	}
	archived := len(m.runtime.ArchivedProjects())

	options := make([]projectHubFilterOption, 0, len(boundaries)+3)
	options = append(options,
		projectHubFilterOption{
			ID:    projectHubFilterAll,
			Label: "All",
			Count: len(available) + archived,
		},
		projectHubFilterOption{
			ID:    models.ActiveBoundaryID,
			Label: config.ActiveBoundaryName,
			Count: counts[models.ActiveBoundaryID],
		},
	)
	for _, boundary := range boundaries {
		options = append(options, projectHubFilterOption{
			ID:    boundary.ID,
			Label: boundary.Name,
			Count: counts[boundary.ID],
		})
	}
	return append(options, projectHubFilterOption{
		ID:    projectHubFilterArchived,
		Label: "Archived",
		Count: archived,
	})
}

func (m *ProjectHubModel) openProjectHubFilterPicker() {
	picker, err := newProjectHubFilterPicker(
		"Boundary",
		"Type filter · ↑↓/Ctrl+j/k move · Enter apply · Esc close",
		m.boundaryFilterOptions(),
		m.boundaryFilter,
	)
	if err != nil {
		m.notice = err.Error()
		return
	}
	m.filterPicker = picker
}

func (m *ProjectHubModel) updateProjectHubFilterPicker(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	option, chosen, dismissed := m.filterPicker.handleKey(key)
	if chosen {
		m.filterPicker = nil
		m.applyBoundaryFilter(option.ID)
		return m, nil
	}
	if dismissed {
		m.filterPicker = nil
	}
	return m, nil
}

// applyBoundaryFilter narrows the listing without touching any stored state:
// the surface is rebuilt from the same catalog, only with fewer rows admitted.
// The surface is discarded rather than reloaded so its empty-state copy is
// rebuilt for the new filter.
func (m *ProjectHubModel) applyBoundaryFilter(boundaryID string) {
	if m.boundaryFilter == boundaryID {
		return
	}
	prefer := m.currentProjectHubItemID()
	m.boundaryFilter = boundaryID
	m.notice = ""
	m.surface = nil
	m.rebuildProjectSurface(prefer)
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
	title := m.renderProjectHubTitle(width)
	header := title
	if search := m.renderSearchBar(); search != "" {
		header = lipgloss.JoinVertical(lipgloss.Left, title, search)
	}
	footer := renderStatusBarFooter(
		HelpStyle.Render(m.footerHelp(width)),
		MutedStyle.Render(m.footerLabel()),
		width,
		m.AccentColor(),
	)
	contentHeight := fullscreenContentHeightForBlocks(height, header, footer)
	var content string
	if m.boundaries != nil {
		content = m.boundaries.view(width, contentHeight)
	} else if m.filterPicker != nil {
		content = m.filterPicker.render(width)
	} else if m.actions != nil {
		content = m.actions.render(width)
	} else {
		content = NewListSurfaceRenderer(ListSurfaceRenderConfig{
			Width:       width,
			Height:      contentHeight,
			Layout:      ListSurfaceLayoutAuto,
			ShowPreview: true,
			HideHelp:    true,
		}).Render(m.surface)
	}
	if notice := m.noticeText(); notice != "" {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			MutedStyle.Render(truncateToVisualWidth(notice, width)),
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

// footerLabel names the surface currently holding the key stream, so the status
// bar says Boundaries while the manager owns it.
func (m *ProjectHubModel) footerLabel() string {
	if m.boundaries != nil {
		return boundaryManagerFooterLabel
	}
	return projectHubFooterLabel
}

// noticeText prefers the manager's own notice while it is open, because the Hub
// underneath is not the surface that produced or can act on it.
func (m *ProjectHubModel) noticeText() string {
	if m.boundaries != nil {
		return m.boundaries.notice
	}
	return m.notice
}

func (m *ProjectHubModel) subtitle() string {
	counts := fmt.Sprintf(
		"%d active · %d archived",
		len(m.runtime.AvailableProjects()),
		len(m.runtime.ArchivedProjects()),
	)
	if m.boundaryFilter == projectHubFilterAll {
		return counts
	}
	return counts + scopeBoundarySeparator + m.boundaryFilterChrome()
}

// boundaryFilterChrome is the shortest statement of the active filter, so a
// narrow Hub can still say which boundary it is showing after the workspace
// counts have been dropped.
func (m *ProjectHubModel) boundaryFilterChrome() string {
	return "showing " + m.boundaryFilterLabel()
}

// renderProjectHubTitle degrades the title from the full subtitle to the filter
// alone to nothing, so a narrow Hub loses workspace counts before it loses the
// only chrome that explains why rows are missing.
func (m *ProjectHubModel) renderProjectHubTitle(width int) string {
	heading := "Select Project"
	subtitles := []string{m.subtitle()}
	if m.boundaries != nil {
		heading = "Boundaries"
		subtitles = []string{m.boundaries.subtitle()}
	} else if m.boundaryFilter != projectHubFilterAll {
		subtitles = append(subtitles, m.boundaryFilterChrome())
	}
	subtitles = append(subtitles, "")
	for _, subtitle := range subtitles {
		title := renderFullscreenTitle(heading, subtitle, m.AccentColor())
		if lipgloss.Width(title) <= width {
			return title
		}
	}
	return fitStyledChromeToWidth(
		"project hub title",
		renderFullscreenTitle(heading, "", m.AccentColor()),
		width,
	)
}

func (m *ProjectHubModel) renderSearchBar() string {
	if m.boundaries != nil {
		return m.boundaries.searchBar()
	}
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
	case m.boundaries != nil:
		variants = m.boundaries.footerHelpVariants()
	case m.filterPicker != nil:
		variants = []string{
			"Type to filter · ↑↓/Ctrl+j/k move · Enter apply · Esc close",
			"↑↓ move · Enter apply · Esc close",
			"Enter · Esc",
		}
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
			"j/k choose · Enter select · Space actions · f boundary · n new · / search",
			"j/k · Enter · Space · f · n · /",
			"Enter · Space · f · /",
		}
	}
	budget := width - lipgloss.Width(m.footerLabel()) - 1
	for _, variant := range variants {
		if lipgloss.Width(variant) <= budget {
			return variant
		}
	}
	return variants[len(variants)-1]
}

// HasTextInput reports the whole Boundary Manager as text input because it owns
// the Hub's entire key stream while open, so no shell interceptor may claim a
// key the manager is waiting for.
func (m *ProjectHubModel) HasTextInput() bool {
	return m.boundaries != nil || m.actions != nil || m.filterPicker != nil || m.DefaultHasTextInput()
}

func (m *ProjectHubModel) ShouldCaptureEsc() bool {
	return m.boundaries != nil || m.actions != nil || m.filterPicker != nil ||
		m.SearchShouldCaptureEsc() || m.SearchQuery() != ""
}

// GetConfirmDialog surfaces the manager's confirmation through the Manage host,
// so a boundary deletion is answered by the same overlay every other destructive
// operation uses.
func (m *ProjectHubModel) GetConfirmDialog() *ConfirmDialog {
	if m.boundaries == nil {
		return nil
	}
	return m.boundaries.confirmDialog()
}

func (m *ProjectHubModel) IsFullHeight() bool { return true }

func (m *ProjectHubModel) StatusBarLabel() string { return "Projects" }

func (m *ProjectHubModel) GetHelpItems() []HelpItem {
	if m.boundaries != nil {
		return m.boundaries.helpItems()
	}
	return []HelpItem{
		{Key: "j/k", Description: "Move selection", Category: "Navigation"},
		{Key: "Ctrl+d/u", Description: "Move half a page down/up", Category: "Navigation"},
		{Key: "Enter", Description: "Focus the selected active project", Category: "Actions"},
		{Key: "Space", Description: "Open project administration actions", Category: "Actions"},
		{Key: "n", Description: "Create a project", Category: "Actions"},
		{Key: "B", Description: "Manage boundaries", Category: "Actions"},
		{Key: "f", Description: "Filter the list by boundary", Category: "Filters"},
		{Key: "/", Description: "Search projects", Category: "Filters"},
		{Key: "Esc", Description: "Clear search, then close", Category: "Navigation"},
	}
}

func (m *ProjectHubModel) RefreshProjectHub() {
	m.rebuildProjectSurface(m.currentProjectHubItemID())
	if m.boundaries != nil {
		m.boundaries.refresh()
	}
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
	return m.registerProjectBoundaryOperations()
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
