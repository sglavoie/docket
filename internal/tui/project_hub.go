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
	"github.com/sglavoie/tt/internal/workspace"
)

// ShowProjectHubMsg opens the searchable project selection and administration
// surface.
type ShowProjectHubMsg struct{}

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

// projectHubTaskFilter is the Ctrl+T axis: every project, only the projects
// known to hold tasks, or only the projects known to hold none. It is one
// cycling axis rather than two independent toggles because the two narrowings
// are mutually exclusive readings of the same count.
type projectHubTaskFilter uint8

const (
	projectHubTaskFilterAll projectHubTaskFilter = iota
	projectHubTaskFilterWithTasks
	projectHubTaskFilterWithoutTasks
)

func (f projectHubTaskFilter) next() projectHubTaskFilter {
	switch f {
	case projectHubTaskFilterWithTasks:
		return projectHubTaskFilterWithoutTasks
	case projectHubTaskFilterWithoutTasks:
		return projectHubTaskFilterAll
	default:
		return projectHubTaskFilterWithTasks
	}
}

// shortLabel names the state for the footer hint, which reads as what the next
// press produces rather than as the state the axis currently holds.
func (f projectHubTaskFilter) shortLabel() string {
	switch f {
	case projectHubTaskFilterWithTasks:
		return "with tasks"
	case projectHubTaskFilterWithoutTasks:
		return "without tasks"
	default:
		return "all projects"
	}
}

// phrase names the state in prose, for the empty-list copy that has to explain
// where the next press lands.
func (f projectHubTaskFilter) phrase() string {
	switch f {
	case projectHubTaskFilterWithTasks:
		return "projects that hold tasks"
	case projectHubTaskFilterWithoutTasks:
		return "projects without tasks"
	default:
		return "every project"
	}
}

func (f projectHubTaskFilter) subtitleMarker() string {
	if f == projectHubTaskFilterAll {
		return ""
	}
	return f.shortLabel()
}

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
	taskFilter       projectHubTaskFilter
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

func (m *ProjectHubModel) SetTaskCounts(counts map[string]int) {
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
	// Above the search branch, so the cycle answers from the query input and
	// from browse alike; the action sheet above still owns its whole key stream.
	if key.String() == "ctrl+t" {
		m.notice = ""
		m.taskFilter = m.taskFilter.next()
		m.rebuildProjectSurface(m.currentProjectHubItemID())
		return m, nil
	}
	if m.HasActiveSearch() {
		switch key.String() {
		case "esc":
			// Leaving the input keeps the query so the matches stay browsable.
			m.ConfirmSearch()
			return m, nil
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
	case "v":
		m.toggleProjectHubSelection()
	case "a":
		m.selectEveryMatchingProject(true)
	case "A":
		m.selectEveryMatchingProject(false)
	case " ", "space":
		m.openProjectHubActions()
	case "enter":
		return m, m.activateProjectHubCurrent()
	}
	return m, nil
}

// toggleProjectHubSelection mirrors the Outliner's v: Enter already focuses a
// project and Space already opens actions, so selection needs its own key
// rather than the Enter/Space toggle the standalone bulk editor used.
func (m *ProjectHubModel) toggleProjectHubSelection() {
	if m.surface == nil {
		return
	}
	if _, err := m.surface.ToggleCurrentSelection(); err != nil {
		m.notice = err.Error()
	}
}

func (m *ProjectHubModel) selectEveryMatchingProject(selected bool) {
	if m.surface == nil {
		return
	}
	if !selected {
		m.surface.ClearSelection()
		return
	}
	matches := m.surface.Matches()
	ids := make([]ListItemID, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, match.Item.ID)
	}
	if err := m.surface.SetSelectedIDs(ids); err != nil {
		m.notice = err.Error()
	}
}

func (m *ProjectHubModel) selectionCount() int {
	if m.surface == nil {
		return 0
	}
	return len(m.surface.SelectedIDs())
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
	categories := m.projectHubCategories()
	items := m.projectHubItems()
	if m.surface == nil {
		surface, err := NewListSurfaceModel(items, categories, ListSurfaceConfig{
			SelectionMode: ListSelectionMulti,
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
	m.refreshProjectHubEmptyStates()
	m.surface.SetQuery(m.SearchQuery())
	if prefer != "" {
		m.surface.SetCursorToID(prefer)
	}
}

// projectHubCategories orders the listing groups: the base task workspace
// first, then the archived tail.
func (m *ProjectHubModel) projectHubCategories() []ListCategory {
	return []ListCategory{
		{ID: projectHubCategoryActive, Label: "Active Projects"},
		{ID: projectHubCategoryArchived, Label: "Archived Projects"},
	}
}

func (m *ProjectHubModel) projectHubItems() []ListItem {
	items := make([]ListItem, 0, len(m.runtime.AllProjects()))
	for _, project := range m.runtime.AvailableProjects() {
		if project == nil || m.projectHubFilterHides(project) {
			continue
		}
		label := project.Name
		if project.ID == m.focusedProjectID {
			label += " (focused)"
		}
		items = append(items, ListItem{
			ID:            ListItemID(projectHubItemPrefix + project.ID),
			CategoryID:    projectHubCategoryActive,
			PrimaryText:   label,
			SecondaryText: m.projectSecondary(project),
			Aliases:       []string{project.Name, project.Slug, "active project"},
			Preview:       m.projectPreview(project, false),
			OperationIDs: append([]OperationID{
				operationProjectOpen,
				operationScopeCreateProject,
				operationScopeRenameProject,
				operationScopePinProject,
				operationScopeArchiveProject,
				operationScopeOpenProjectNote,
			}, projectBulkOperations...),
			Data: projectHubEntry{Kind: projectHubEntryActive, Project: project},
		})
	}
	for _, project := range m.runtime.ArchivedProjects() {
		if project == nil || m.projectHubFilterHides(project) {
			continue
		}
		items = append(items, ListItem{
			ID:            ListItemID(projectHubArchivedItemPrefix + project.ID),
			CategoryID:    projectHubCategoryArchived,
			PrimaryText:   project.Name,
			SecondaryText: strings.TrimSuffix("archived · "+projectTagChips(project.Tags), " · "),
			Aliases:       []string{project.Name, project.Slug, "archived project"},
			Preview:       m.projectPreview(project, true),
			OperationIDs: []OperationID{
				operationScopeCreateProject,
				operationScopeRenameProject,
				operationScopeRestoreProject,
			},
			Data: projectHubEntry{Kind: projectHubEntryArchived, Project: project},
		})
	}
	return items
}

// projectHubFilterHides reports the Ctrl+T filter hiding a project. It hides a
// known count only: a project whose tasks could not be read is absent from the
// counts and is not known to be empty or to hold work, so neither narrowing may
// claim it.
func (m *ProjectHubModel) projectHubFilterHides(project *models.Project) bool {
	count, ok := m.taskCounts[project.ID]
	if !ok {
		return false
	}
	switch m.taskFilter {
	case projectHubTaskFilterWithTasks:
		return count == 0
	case projectHubTaskFilterWithoutTasks:
		return count > 0
	default:
		return false
	}
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

// projectSecondary carries the task count and the project's own tag chips. The
// chips are plain text rather than styled badges because the row layout measures
// this string, and they are what makes a parked project legible here: the Hub
// lists parked projects alongside every other active one.
func (m *ProjectHubModel) projectSecondary(project *models.Project) string {
	count := m.projectTaskCount(project)
	secondary := fmt.Sprintf("%s tasks", count)
	if count == projectTaskCountUnknown {
		secondary = "tasks not loaded"
	}
	if chips := projectTagChips(project.Tags); chips != "" {
		secondary += " · " + chips
	}
	return secondary
}

func projectTagChips(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	chips := make([]string, 0, len(tags))
	for _, tag := range tags {
		chips = append(chips, "#"+tag)
	}
	return strings.Join(chips, " ")
}

// refreshProjectHubEmptyStates keeps the empty copy naming whichever narrowing
// emptied the list. The Ctrl+T filter runs at item-build time, so a list it
// empties is source-empty rather than unmatched, and the stock "create a
// project" copy would be a lie there.
func (m *ProjectHubModel) refreshProjectHubEmptyStates() {
	if m.surface == nil {
		return
	}
	m.surface.SetEmptyStates(m.projectHubEmptyState(), m.projectHubNoMatchState())
}

func (m *ProjectHubModel) projectHubEmptyState() ListEmptyState {
	switch m.taskFilter {
	case projectHubTaskFilterWithTasks:
		return ListEmptyState{Title: "No projects hold tasks.", Detail: m.projectHubFilterAdvice()}
	case projectHubTaskFilterWithoutTasks:
		return ListEmptyState{Title: "Every project holds tasks.", Detail: m.projectHubFilterAdvice()}
	default:
		return ListEmptyState{
			Title:  "No projects are available.",
			Detail: "Press n to create a project.",
		}
	}
}

// projectHubFilterAdvice names where the next press lands, so a three-state
// cycle never claims one press returns to the whole list.
func (m *ProjectHubModel) projectHubFilterAdvice() string {
	return "Press Ctrl+T to show " + m.taskFilter.next().phrase() + "."
}

func (m *ProjectHubModel) projectHubNoMatchState() ListEmptyState {
	detail := "Press Esc to clear the query and browse every project."
	if m.taskFilter != projectHubTaskFilterAll {
		detail = "Press Esc to clear the query, or Ctrl+T to show " + m.taskFilter.next().phrase() + "."
	}
	return ListEmptyState{
		Title:  "No projects match this search.",
		Detail: detail,
	}
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

// openProjectHubActions offers the cursor's project, or the whole selection
// when one exists. The selection wins because it is the deliberate, visible
// state: acting on the cursor while checkboxes are set would silently ignore
// them.
func (m *ProjectHubModel) openProjectHubActions() {
	if m.selectionCount() > 0 {
		m.openProjectHubBulkActions()
		return
	}
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

// openProjectHubBulkActions freezes the selected project identities into the
// command context instead of a single host value, so the executor resolves the
// same target set when the tag prompt resumes the request.
func (m *ProjectHubModel) openProjectHubBulkActions() {
	selected := m.surface.SelectedItems()
	identities := make([]EntityIdentity, 0, len(selected))
	for _, item := range selected {
		entry, ok := item.Data.(projectHubEntry)
		if !ok || entry.Project == nil {
			continue
		}
		identities = append(identities, ProjectEntity(entry.Project.ID))
	}
	if len(identities) == 0 {
		return
	}
	ctx := m.projectHubCommandContext("")
	ctx.Selection.EntityIdentities = identities

	sheet, err := NewContextActionSheetModel(
		m.registry,
		m.executor,
		ctx,
		projectHubBulkOperationIDs(m.surface.ActionState().OperationIDs),
		ContextActionSheetConfig{
			Host:         ContextActionHostProjectHub,
			Completion:   ContextActionCompleteCloseSheet,
			Title:        "Bulk Actions",
			Categories:   projectHubActionCategories,
			CategoryFor:  projectHubActionCategory,
			SecondaryFor: func(descriptor OperationDescriptor) string { return string(descriptor.Risk) },
			Bulk:         true,
			Targets:      len(identities),
			EmptyState:   ListEmptyState{Title: "No bulk actions available."},
			NoMatchState: ListEmptyState{Title: "No matching actions.", Detail: "Clear or change the search."},
		},
	)
	if err != nil {
		m.notice = err.Error()
		return
	}
	if len(sheet.operationIDs()) == 0 {
		m.notice = "Bulk tag actions apply only to active projects"
		return
	}
	m.actions = sheet
}

// projectHubBulkOperationIDs keeps the bulk sheet closed over the four tag
// actions, in their declared order, and drops any the selection does not share.
// An archived row in the selection carries none of them, so a mixed selection
// offers nothing rather than an action half of it would refuse.
func projectHubBulkOperationIDs(available []OperationID) []OperationID {
	ids := make([]OperationID, 0, len(projectBulkOperations))
	for _, id := range projectBulkOperations {
		if slices.Contains(available, id) {
			ids = append(ids, id)
		}
	}
	return ids
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
	// A value step is owned by the shell overlay above the Hub and resumes from
	// the frozen request, so the sheet has nothing left to do and closing it keeps
	// one modal on screen instead of two.
	if result.ValueRequired != nil {
		m.actions = nil
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
	footer := ""
	if m.actions == nil {
		if guidance, ok := m.ContextualGuidance(); ok {
			footer = renderGuidance(guidance, width, m.AccentColor())
		}
	}
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
		}).Render(m.surface)
	}
	if notice := m.notice; notice != "" {
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

func (m *ProjectHubModel) subtitle() string {
	subtitle := fmt.Sprintf(
		"%d active · %d archived",
		len(m.runtime.AvailableProjects()),
		len(m.runtime.ArchivedProjects()),
	)
	// The counts above stay workspace truth; the marker names the narrowing the
	// list is under rather than restating it as a smaller total.
	if marker := m.taskFilter.subtitleMarker(); marker != "" {
		subtitle += " · " + marker
	}
	if count := m.selectionCount(); count > 0 {
		subtitle += fmt.Sprintf(" · %d selected", count)
	}
	return subtitle
}

// renderProjectHubTitle delegates pulse and context fitting to the shared
// operational title renderer.
func (m *ProjectHubModel) renderProjectHubTitle(width int) string {
	return renderFullscreenTitleWithPulse(
		"Select Project",
		m.subtitle(),
		m.currentWorkspacePulse(),
		width,
		m.AccentColor(),
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

// HasTextInput reports the action sheet as text input because it owns the Hub's
// entire key stream while open, so no shell interceptor may claim a key the
// sheet is waiting for.
func (m *ProjectHubModel) HasTextInput() bool {
	return m.actions != nil || m.DefaultHasTextInput()
}

func (m *ProjectHubModel) ShouldCaptureEsc() bool {
	return m.actions != nil || m.SearchShouldCaptureEsc()
}

func (m *ProjectHubModel) IsFullHeight() bool { return true }

func (m *ProjectHubModel) StatusBarLabel() string { return "Projects" }

func (m *ProjectHubModel) GetHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "j/k", Description: "Move selection", Category: "Navigation"},
		{Key: "Ctrl+d/u", Description: "Move half a page down/up", Category: "Navigation"},
		{Key: "Enter", Description: "Focus the selected active project", Category: "Actions"},
		{Key: "Space", Description: "Open actions for the selection, or the cursor when nothing is selected", Category: "Actions"},
		{Key: "n", Description: "Create a project", Category: "Actions"},
		{Key: "v", Description: "Select or deselect the project under the cursor", Category: "Selection"},
		{Key: "a/A", Description: "Select or deselect every matching project", Category: "Selection"},
		{Key: "/", Description: "Search projects", Category: "Filters"},
		{Key: "Ctrl+T", Description: "Cycle every project, only those holding tasks, only those holding none", Category: "Filters"},
		{Key: "Esc", Description: "Leave search, then close", Category: "Navigation"},
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
	hub.SetTaskCounts(m.projectHubTaskCounts())
	if m.crossOutliner != nil {
		hub.SetProjectFocus(m.crossOutliner.scopedProjectID)
	}
}

// projectHubTaskCounts counts tasks for every cataloged project, not just the
// base workspace the Outliner loads, so the Hub can report parked and archived
// rows on a real number instead of an unknown. The projects the Outliner already
// parsed seed the map for free; the remainder is loaded through the Outliner's
// own retained loader, which serves an unchanged project from its cache rather
// than reparsing it on every Hub open.
//
// A project that fails to load is absent from the map rather than counted zero,
// and its diagnostic is dropped: the Hub is a picker, and a project it cannot
// parse still needs to be listed and selectable.
func (m *AppModel) projectHubTaskCounts() map[string]int {
	counts := make(map[string]int)
	if m == nil {
		return counts
	}
	var loader *workspace.Loader
	if m.crossOutliner != nil {
		for _, project := range m.crossOutliner.loadedProjects {
			if project != nil {
				counts[project.ID] = project.CountTasks()
			}
		}
		if m.crossOutliner.projectService != nil {
			loader = m.crossOutliner.projectService.workspaceLoader
		}
	}
	if m.runtime == nil {
		return counts
	}

	seen := make(map[string]bool, len(counts))
	for id := range counts {
		seen[id] = true
	}
	cataloged := append(m.runtime.AvailableProjects(), m.runtime.ArchivedProjects()...)
	remainder := make([]*models.Project, 0, len(cataloged))
	for _, project := range cataloged {
		if project == nil || seen[project.ID] {
			continue
		}
		seen[project.ID] = true
		remainder = append(remainder, project)
	}
	if len(remainder) == 0 {
		return counts
	}
	_, loaded, _ := loadCrossScopeSelection(remainder, loader)
	for _, project := range loaded {
		if project != nil {
			counts[project.ID] = project.CountTasks()
		}
	}
	return counts
}
