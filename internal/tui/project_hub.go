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

// ProjectHubModel is the browse-first surface for selecting and administering
// every active and archived project. The cursor is live on press one and `/`
// opens the query, so the Hub's project vocabulary answers immediately instead
// of typing into a search input.
type ProjectHubModel struct {
	BaseSearchableView
	runtime          *config.RuntimeState
	taskCounts       map[string]int
	surface          *ListSurfaceModel
	actions          *ContextActionSheetModel
	registry         *OperationRegistry
	executor         *OperationExecutor
	dispatch         operationKeyDispatch
	focusedProjectID string
	notice           string
	taskFilter       projectHubTaskFilter
	// pendingTopChord holds the first g of gg. The shell's chord state serves the
	// Outliner only, so the Hub keeps its own one-key prefix.
	pendingTopChord bool
}

func NewProjectHubModel(runtime *config.RuntimeState) *ProjectHubModel {
	if runtime == nil {
		runtime = config.EmptyRuntimeState("")
	}
	m := &ProjectHubModel{runtime: runtime}
	m.InitSearchInput("Search projects", 64, 32)
	m.rebuildProjectSurface("")
	return m
}

func (m *ProjectHubModel) Init() tea.Cmd { return nil }

func (m *ProjectHubModel) SetOperationSystem(
	registry *OperationRegistry,
	executor *OperationExecutor,
	dispatch operationKeyDispatch,
) {
	m.registry = registry
	m.executor = executor
	m.dispatch = dispatch
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
		m.cycleProjectHubTaskFilter()
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
	if m.completeProjectHubTopChord(key.String()) {
		return m, nil
	}
	switch key.String() {
	case "j", "down":
		m.surface.MoveCursor(1)
	case "k", "up":
		m.surface.MoveCursor(-1)
	case "g":
		m.pendingTopChord = true
	case "home":
		m.surface.MoveCursorToStart()
	case "G", "end":
		m.surface.MoveCursorToEnd()
	case "ctrl+d":
		m.moveProjectHubHalfPage(1)
	case "ctrl+u":
		m.moveProjectHubHalfPage(-1)
	case "esc":
		// The shell only routes Esc here while ShouldCaptureEsc holds it, so this
		// is the selection rung of the unwind and never the close.
		m.surface.ClearSelection()
	case "/", "a", "e", "*", "#", "ctrl+n", "v", "ctrl+a":
		return m, m.dispatchProjectHubKey(key.String())
	case " ", "space":
		return m, m.dispatchProjectHubKey("space")
	case "enter":
		return m, m.activateProjectHubCurrent()
	}
	return m, nil
}

// completeProjectHubTopChord answers the second g of gg and reports the press as
// consumed. Any other key abandons the prefix silently and is then handled
// normally, so a stray g never swallows the key after it.
func (m *ProjectHubModel) completeProjectHubTopChord(key string) bool {
	if !m.pendingTopChord {
		return false
	}
	m.pendingTopChord = false
	if key != "g" {
		return false
	}
	m.surface.MoveCursorToStart()
	return true
}

// dispatchProjectHubKey resolves one browse key through the Hub's own strategy
// and runs whatever operation it reaches. The cursor's project is frozen into
// the context here because every project operation resolves its target from the
// context rather than from a host value.
func (m *ProjectHubModel) dispatchProjectHubKey(key string) tea.Cmd {
	if m.dispatch == nil {
		return nil
	}
	_, cmd := m.dispatch(
		InputStrategyProjectHub,
		[]string{key},
		m.projectHubCommandContext(m.cursorProjectID()),
	)
	return cmd
}

func (m *ProjectHubModel) cycleProjectHubTaskFilter() {
	m.taskFilter = m.taskFilter.next()
	m.rebuildProjectSurface(m.currentProjectHubItemID())
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

func (m *ProjectHubModel) selectEveryMatchingProject() {
	if m.surface == nil {
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
				operationScopeArchiveProject,
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
			Detail: "Press a to create a project.",
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

// activateProjectHubCurrent is the one Enter path: the search input and the
// browse grammar both reach it, so the key means the same thing whether or not
// the query is focused. The archived refusal stays here because it names the row
// the cursor sits on, which the operation's own applicability can only report as
// a missing action.
func (m *ProjectHubModel) activateProjectHubCurrent() tea.Cmd {
	entry, ok := m.selectedProjectHubEntry()
	if !ok || entry.Project == nil {
		return nil
	}
	if entry.Kind == projectHubEntryArchived {
		m.notice = "Archived projects must be unarchived from Space actions before they can be focused"
		return nil
	}
	return m.dispatchProjectHubKey("enter")
}

func (m *ProjectHubModel) projectHubCommandContext(projectID string) CommandContext {
	ctx := PresentationCommandContext(ViewProjectHub)
	ctx.Capabilities.Add(CapabilityProjectHub)
	if projectID != "" {
		ctx.CurrentEntity = ProjectEntity(projectID)
	}
	return ctx
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
	for _, entry := range projectOperationManifest() {
		if entry.ID == descriptor.ID {
			return entry.Category
		}
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

// ShouldCaptureEsc claims Esc for the Hub's own unwind rungs: the action sheet,
// the query, then the selection. A selection rung exists because retiring `A`
// left no other way to drop every checkbox at once.
func (m *ProjectHubModel) ShouldCaptureEsc() bool {
	return m.actions != nil || m.SearchShouldCaptureEsc() || m.selectionCount() > 0
}

func (m *ProjectHubModel) IsFullHeight() bool { return true }

func (m *ProjectHubModel) StatusBarLabel() string { return "Projects" }

// GetHelpItems documents the Hub grammar from the two sources that own it: the
// registry describes every operation the strategy binds, and the keymap names
// the cursor and unwind keys whose descriptors answer only to the Outliner.
// Ctrl+T is in both because the footer needs its cycling label, so the merge
// keeps the registry's spelling and drops the keymap's duplicate.
func (m *ProjectHubModel) GetHelpItems() []HelpItem {
	if m.registry == nil {
		return nil
	}
	items := m.registry.HelpItemsForInputStrategy(
		InputStrategyProjectHub,
		m.projectHubCommandContext(m.cursorProjectID()),
	)
	documented := make(map[string]bool, len(items))
	for _, item := range items {
		documented[strings.ToLower(item.Key)] = true
	}
	for _, item := range helpItemsFromKeyMap(m.projectHubGuidanceKeyMap(m.guidanceState())) {
		if !documented[strings.ToLower(item.Key)] {
			items = append(items, item)
		}
	}
	return items
}

// guidanceState reads the list state both the footer and `?` help project.
func (m *ProjectHubModel) guidanceState() ListGuidanceState {
	if m.surface == nil {
		return ListGuidanceState{}
	}
	return m.surface.GuidanceState(m.HasActiveSearch())
}

// cursorProjectID names the project the direct grammar would act on, so
// applicability drops Enter, e, *, # and Ctrl+N when the list has no cursor.
func (m *ProjectHubModel) cursorProjectID() string {
	entry, ok := m.selectedProjectHubEntry()
	if !ok || entry.Project == nil {
		return ""
	}
	return entry.Project.ID
}

func (m *ProjectHubModel) RefreshProjectHub() {
	m.rebuildProjectSurface(m.currentProjectHubItemID())
}

// projectHubOperationDescriptors declares everything the Hub grammar reaches
// that no project row carries: creating a project, and the list vocabulary the
// surface applies to its own cursor, query, and selection.
func projectHubOperationDescriptors() []OperationDescriptor {
	surface := func(
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
			RequiredCapabilities: NewCapabilitySet(CapabilityProjectHub),
			Risk:                 risk,
		}
	}
	// needsRow gates the vocabulary that only means something against a project
	// row. Search and the task filter stay ungated because either can empty the
	// list and is then the only way back.
	needsRow := func(descriptor OperationDescriptor) OperationDescriptor {
		descriptor.Applicability = projectOperationApplicable
		return descriptor
	}
	return []OperationDescriptor{
		surface(operationScopeCreateProject, "New project", OperationCategoryProject, OperationRiskMutation, "create project"),
		surface(operationProjectHubSearch, "Search projects", OperationCategoryFilter, OperationRiskReadOnly, "filter the project list"),
		needsRow(surface(operationProjectHubToggleSelection, "Select project", OperationCategoryProject, OperationRiskReadOnly, "toggle project selection")),
		needsRow(surface(operationProjectHubSelectAll, "Select every matching project", OperationCategoryProject, OperationRiskReadOnly, "select all projects")),
		needsRow(surface(operationProjectHubActions, "Project actions", OperationCategoryProject, OperationRiskReadOnly, "project action sheet")),
		surface(operationProjectHubTaskFilter, "Cycle every project, only those holding tasks, only those holding none", OperationCategoryFilter, OperationRiskReadOnly, "projects holding tasks", "projects holding no tasks"),
	}
}

func (m *AppModel) registerProjectHubOperations() error {
	for _, descriptor := range projectHubOperationDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities:     NewCapabilitySet(CapabilityProjectHub),
			Handler:                  m.executeProjectHubOperation,
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

// executeProjectHubOperation applies a Hub surface operation to the live Hub.
// The list vocabulary mutates the model the user is looking at, exactly as the
// Outliner's own core operations do, so the handler reaches for the mounted
// module rather than rebuilding one.
//
//nolint:exhaustive // Only Project Hub surface operations are registered with this handler.
func (m *AppModel) executeProjectHubOperation(invocation OperationInvocation) OperationHandlerResult {
	if invocation.Descriptor.ID == operationScopeCreateProject {
		return OperationHandlerResult{Message: CreateProjectMsg{}}
	}
	hub := m.activeProjectHub()
	if hub == nil {
		return OperationHandlerResult{Err: fmt.Errorf("the Project Hub is not open")}
	}
	switch invocation.Descriptor.ID {
	case operationProjectHubSearch:
		return OperationHandlerResult{Message: commandMessage(hub.ActivateSearch())}
	case operationProjectHubToggleSelection:
		hub.toggleProjectHubSelection()
	case operationProjectHubSelectAll:
		hub.selectEveryMatchingProject()
	case operationProjectHubActions:
		hub.openProjectHubActions()
	case operationProjectHubTaskFilter:
		hub.cycleProjectHubTaskFilter()
	}
	return OperationHandlerResult{}
}

// activeProjectHub returns the mounted Project Hub, or nil when Manage is on
// another module.
func (m *AppModel) activeProjectHub() *ProjectHubModel {
	if m == nil || m.manageHub == nil || m.manageHub.Params().Module != ManageProjects {
		return nil
	}
	hub, _ := m.manageHub.ActiveModuleView().(*ProjectHubModel)
	return hub
}

func (m *AppModel) refreshProjectHubIfActive() {
	hub := m.activeProjectHub()
	if hub == nil {
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
