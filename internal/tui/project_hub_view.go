package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/workspace"
)

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
			footer = chrome.RenderGuidance(guidance, width, m.AccentColor())
		}
	}
	contentHeight := chrome.FullscreenContentHeightForBlocks(height, header, footer)
	var content string
	if m.actions != nil {
		content = m.actions.render(width)
	} else {
		content = NewListSurfaceRenderer(m.projectHubRenderConfig(contentHeight)).Render(m.surface)
	}
	if notice := m.notice; notice != "" {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			theme.MutedStyle.Render(textfit.TruncateToVisualWidth(notice, width)),
			content,
		)
	}
	return tea.NewView(lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		chrome.FitBlockHeight(content, contentHeight),
		footer,
	))
}

// projectHubRenderConfig is the one description of the Hub's list surface, so
// the footer gate asking whether a preview pane fits reads the same widths the
// frame is about to draw. Height is the caller's because only the render pass
// has measured the header and footer around the list.
func (m *ProjectHubModel) projectHubRenderConfig(height int) ListSurfaceRenderConfig {
	return ListSurfaceRenderConfig{
		Width:       max(m.width, 1),
		Height:      height,
		Layout:      ListSurfaceLayoutAuto,
		ShowPreview: true,
		HideHelp:    true,
		PreviewBody: m.renderProjectPreviewTasks,
	}
}

func (m *ProjectHubModel) subtitle() string {
	var subtitle string
	if m.mode == route.ProjectHubModePinned {
		subtitle = fmt.Sprintf("%d pinned active", m.pinnedActiveProjectCount())
	} else {
		subtitle = fmt.Sprintf(
			"%d active · %d archived",
			len(m.runtime.AvailableProjects()),
			len(m.runtime.ArchivedProjects()),
		)
	}
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

func (m *ProjectHubModel) pinnedActiveProjectCount() int {
	count := 0
	for _, project := range m.runtime.AvailableProjects() {
		if project != nil && project.Pinned {
			count++
		}
	}
	return count
}

// renderProjectHubTitle delegates pulse and context fitting to the shared
// operational title renderer.
func (m *ProjectHubModel) renderProjectHubTitle(width int) string {
	title := "Select Project"
	if m.mode == route.ProjectHubModePinned {
		title = "Select Pinned Project"
	}
	return chrome.RenderFullscreenTitleWithPulse(
		title,
		m.subtitle(),
		m.CurrentWorkspacePulse(),
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
	return "/" + m.SearchQuery() + theme.MutedStyle.Render(fmt.Sprintf("  (%d matches)", len(m.surface.Matches())))
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
func (m *ProjectHubModel) GetHelpItems() []operation.HelpItem {
	if m.registry == nil {
		return nil
	}
	items := m.registry.HelpItemsForInputStrategy(
		operation.InputStrategyProjectHub,
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
func projectHubOperationDescriptors() []operation.OperationDescriptor {
	surface := func(
		id operation.OperationID,
		label string,
		category operation.OperationCategory,
		risk operation.OperationRisk,
		aliases ...string,
	) operation.OperationDescriptor {
		return operation.OperationDescriptor{
			ID:                   id,
			Label:                label,
			Aliases:              aliases,
			Category:             category,
			TargetSource:         operation.OperationTargetGlobal,
			TargetCardinality:    operation.TargetCardinalityNone,
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilityProjectHub),
			Risk:                 risk,
		}
	}
	// needsRow gates the vocabulary that only means something against a project
	// row. Search and the task filter stay ungated because either can empty the
	// list and is then the only way back.
	needsRow := func(descriptor operation.OperationDescriptor) operation.OperationDescriptor {
		descriptor.Applicability = projectOperationApplicable
		return descriptor
	}
	return []operation.OperationDescriptor{
		surface(operationScopeCreateProject, "New project", operation.OperationCategoryProject, operation.OperationRiskMutation, "create project"),
		surface(operationProjectHubSearch, "Search projects", operation.OperationCategoryFilter, operation.OperationRiskReadOnly, "filter the project list"),
		needsRow(surface(operationProjectHubToggleSelection, "Select project", operation.OperationCategoryProject, operation.OperationRiskReadOnly, "toggle project selection")),
		needsRow(surface(operationProjectHubSelectAll, "Select every matching project", operation.OperationCategoryProject, operation.OperationRiskReadOnly, "select all projects")),
		needsRow(surface(operationProjectHubActions, "Project actions", operation.OperationCategoryProject, operation.OperationRiskReadOnly, "project action sheet")),
		surface(operationProjectHubTaskFilter, "Cycle every project, only those holding open tasks, only those holding none open", operation.OperationCategoryFilter, operation.OperationRiskReadOnly, "projects holding open tasks", "projects holding no open tasks"),
		needsRow(surface(operationProjectHubPreviewPageDown, "Page project tasks down", operation.OperationCategoryNavigation, operation.OperationRiskReadOnly, "scroll project preview down")),
		needsRow(surface(operationProjectHubPreviewPageUp, "Page project tasks up", operation.OperationCategoryNavigation, operation.OperationRiskReadOnly, "scroll project preview up")),
	}
}

func (m *AppModel) registerProjectHubOperations() error {
	for _, descriptor := range projectHubOperationDescriptors() {
		if err := m.registerOperation(descriptor, operation.OperationBinding{
			RequiredCapabilities:     operation.NewCapabilitySet(operation.CapabilityProjectHub),
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
func (m *AppModel) executeProjectHubOperation(invocation operation.OperationInvocation) operation.OperationHandlerResult {
	if invocation.Descriptor.ID == operationScopeCreateProject {
		return operation.OperationHandlerResult{Message: CreateProjectMsg{}}
	}
	hub := m.activeProjectHub()
	if hub == nil {
		return operation.OperationHandlerResult{Err: fmt.Errorf("the Project Hub is not open")}
	}
	switch invocation.Descriptor.ID {
	case operationProjectHubSearch:
		return operation.OperationHandlerResult{Message: commandMessage(hub.ActivateSearch())}
	case operationProjectHubToggleSelection:
		hub.toggleProjectHubSelection()
	case operationProjectHubSelectAll:
		hub.selectEveryMatchingProject()
	case operationProjectHubActions:
		hub.openProjectHubActions()
	case operationProjectHubTaskFilter:
		hub.cycleProjectHubTaskFilter()
	case operationProjectHubPreviewPageDown:
		hub.pageProjectPreview(1)
	case operationProjectHubPreviewPageUp:
		hub.pageProjectPreview(-1)
	}
	return operation.OperationHandlerResult{}
}

// activeProjectHub returns the mounted Project Hub, or nil when Manage is on
// another module.
func (m *AppModel) activeProjectHub() *ProjectHubModel {
	if m == nil || m.manageHub == nil || m.manageHub.Params().Module != route.ManageProjects {
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
func (m *AppModel) projectHubTaskCounts() map[string]projectTaskTally {
	counts := make(map[string]projectTaskTally)
	if m == nil {
		return counts
	}
	var loader *workspace.Loader
	if m.crossOutliner != nil {
		for _, project := range m.crossOutliner.loadedProjects {
			if project != nil {
				counts[project.ID] = projectTaskTallyOf(project)
			}
		}
		if m.crossOutliner.projectService != nil {
			loader = m.crossOutliner.projectService.WorkspaceLoader()
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
			counts[project.ID] = projectTaskTallyOf(project)
		}
	}
	return counts
}
