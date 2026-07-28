package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// contextHubFooterLabel is the footer's right-hand label. Help variants are
// measured against it so the pair always shares a single line: the shell swaps a
// full-height view's last two lines for the status bar, so help wrapped onto a
// third line would leave an orphaned separator behind that swap.
const contextHubFooterLabel = "Scope Hub"

// ContextHubModel is the Manage-hosted surface for browsing, applying, and
// editing the active project context. It renders the shared
// ContextScopeSummaries list and reports a selection through
// ContextScopeSelectedMsg so the shell applies scope through the single existing
// path. Create/edit/rename/delete for both context kinds run inline here and
// commit through ContextMutator.
type ContextHubModel struct {
	BaseSearchableView
	runtime          *config.RuntimeState
	mutator          *ContextMutator
	summaries        []ContextScopeSummary
	active           ContextScope
	taskCounts       map[string]int
	surface          *ListSurfaceModel
	actions          *ContextActionSheetModel
	registry         *OperationRegistry
	executor         *OperationExecutor
	focusedProjectID string

	editor         *contextEditor
	confirm        ConfirmDialog
	pendingDelete  ContextScope
	pendingPurge   scopeHubActionTarget
	confirmDiscard bool
	discardOverlay ConfirmDialog
	notice         string
}

// NewContextHubModel takes the manager and service so regular-context edits
// commit through the same catalog transaction boundary the rest of the shell
// uses. Both may be nil in tests, where ContextMutator falls back to an
// in-memory catalog.
func NewContextHubModel(runtime *config.RuntimeState, manager *config.Manager, service *app.Service, active ContextScope) *ContextHubModel {
	if runtime == nil {
		runtime = config.EmptyRuntimeState("")
	}
	m := &ContextHubModel{
		runtime:   runtime,
		mutator:   NewContextMutator(runtime, manager, service),
		summaries: ContextScopeSummaries(runtime),
		active:    active,
	}
	m.InitSearchInput("Search contexts and projects", 64, 32)
	m.rebuildScopeSurface(contextScopeListItemID(active))
	return m
}

// SetLoadedProjects supplies the task-bearing projects the preview counts
// against. Catalog projects carry metadata only, so without this seam the
// preview lists project names with no task counts rather than reporting zero.
func (m *ContextHubModel) SetLoadedProjects(projects []*models.Project) {
	counts := make(map[string]int, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		counts[project.ID] = project.CountTasks()
	}
	if len(counts) == 0 {
		m.taskCounts = nil
		m.rebuildScopeSurface(m.currentScopeHubItemID())
		return
	}
	m.taskCounts = counts
	m.rebuildScopeSurface(m.currentScopeHubItemID())
}

func (m *ContextHubModel) Init() tea.Cmd { return nil }

func (m *ContextHubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if m.confirmDiscard {
		switch key.String() {
		case "y", "Y":
			m.closeEditor()
		case "n", "N", "esc":
			m.confirmDiscard = false
			m.discardOverlay.Cancel()
		}
		return m, nil
	}
	confirmAction := m.confirm.Action()
	if confirmed, handled := m.confirm.HandleKey(key.String()); handled {
		if !confirmed {
			m.pendingDelete = ContextScope{}
			m.pendingPurge = scopeHubActionTarget{}
			return m, nil
		}
		if confirmAction == scopeHubConfirmPurge {
			target := m.pendingPurge
			m.pendingPurge = scopeHubActionTarget{}
			if target.TrashEntryID == "" {
				return m, nil
			}
			return m, func() tea.Msg {
				return PurgeFromTrashMsg{EntryID: target.TrashEntryID, Slug: target.TrashSlug}
			}
		}
		return m, m.deleteConfirmedContext()
	}
	if m.editor != nil {
		return m.updateEditor(key)
	}
	if m.actions != nil {
		return m.updateScopeHubActions(key)
	}
	if model, cmd, handled := m.HandleSearchMode(key, m, m.applyFilter); handled {
		return model, cmd
	}
	m.notice = ""
	switch key.String() {
	case "/":
		return m, m.ActivateSearch()
	case "esc":
		if m.SearchQuery() != "" {
			m.ClearSearch()
			m.applyFilter()
		}
	case "j", "down":
		if m.surface != nil {
			m.surface.MoveCursor(1)
		}
	case "k", "up":
		if m.surface != nil {
			m.surface.MoveCursor(-1)
		}
	case "g", "home":
		if m.surface != nil {
			m.surface.MoveCursorToStart()
		}
	case "G", "end":
		if m.surface != nil {
			m.surface.MoveCursorToEnd()
		}
	case "n":
		return m, m.executeScopeHubBrowseAction(operationScopeCreateContext)
	case "e":
		return m, m.executeScopeHubBrowseAction(operationScopeEditContext)
	case "r":
		return m, m.executeScopeHubBrowseAction(operationScopeRenameContext)
	case "d":
		return m, m.executeScopeHubBrowseAction(operationScopeDeleteContext)
	case " ", "space":
		m.openScopeHubActions()
	case "enter":
		return m, m.activateScopeHubCurrent()
	}
	return m, nil
}

// SetSize keeps the inline editor's inputs sized with the hub itself; the hub
// is rebuilt on every open, so this is the only place the editor learns its
// width.
func (m *ContextHubModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
	if m.editor != nil {
		m.editor.resize(width)
	}
}

// applyFilter delegates query ranking and stable cursor preservation to the
// shared List Surface.
func (m *ContextHubModel) applyFilter() {
	if m.surface != nil {
		m.surface.SetQuery(m.SearchQuery())
	}
}

func (m *ContextHubModel) View() tea.View {
	width := max(m.width, 1)
	height := max(m.height, 1)

	title := renderFullscreenTitle("Scope Hub", m.subtitle(), m.AccentColor())
	footer := renderStatusBarFooter(
		HelpStyle.Render(m.footerHelp(width)),
		MutedStyle.Render(contextHubFooterLabel),
		width,
		m.AccentColor(),
	)

	header := title
	if searchBar := m.renderSearchBar(); searchBar != "" {
		header = lipgloss.JoinVertical(lipgloss.Left, title, searchBar)
	}
	contentHeight := fullscreenContentHeightForBlocks(height, header, footer)

	content := m.renderContent(width, contentHeight)
	if m.notice != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, MutedStyle.Render(truncateToVisualWidth(m.notice, width)), content)
	}
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, fitBlockHeight(content, contentHeight), footer))
}

func (m *ContextHubModel) renderContent(width, height int) string {
	if m.editor != nil {
		return m.editorView(width, height)
	}
	if m.actions != nil {
		return m.renderScopeHubActions(width)
	}
	rendered := NewListSurfaceRenderer(ListSurfaceRenderConfig{
		Width:       width,
		Height:      height,
		Layout:      ListSurfaceLayoutAuto,
		ShowPreview: true,
		HideHelp:    true,
	}).Render(m.surface)
	return rendered.Content
}

// contextHubWindow returns the rows of lines visible in a height-row viewport
// scrolled to keep cursorLine in view, plus the scroll offset it settled on.
// When rows are clipped it spends a row on an "N more above/below" hint so the
// hidden rows stay discoverable at the minimum supported terminal height.
func contextHubWindow(lines []string, cursorLine, scroll, height int) ([]string, int) {
	if height <= 0 || len(lines) <= height {
		return lines, 0
	}
	// Each hint costs a row, so they are affordable only while enough list rows
	// remain visible underneath them to navigate by.
	if height < 4 {
		scroll = clampWindowScroll(scroll, cursorLine, height, len(lines))
		return lines[scroll : scroll+height], scroll
	}

	window := height - 1
	scroll = clampWindowScroll(scroll, cursorLine, window, len(lines))
	if scroll > 0 && scroll+window < len(lines) {
		window = height - 2
		scroll = clampWindowScroll(scroll, cursorLine, window, len(lines))
	}

	visible := make([]string, 0, height)
	if scroll > 0 {
		visible = append(visible, MutedStyle.Render(fmt.Sprintf("  ↑ %d more above", scroll)))
	}
	visible = append(visible, lines[scroll:scroll+window]...)
	if hidden := len(lines) - scroll - window; hidden > 0 {
		visible = append(visible, MutedStyle.Render(fmt.Sprintf("  ↓ %d more below", hidden)))
	}
	return visible, scroll
}

func clampWindowScroll(scroll, cursorLine, window, total int) int {
	if cursorLine < scroll {
		scroll = cursorLine
	}
	if cursorLine >= scroll+window {
		scroll = cursorLine - window + 1
	}
	return min(max(scroll, 0), max(total-window, 0))
}

func (m *ContextHubModel) previewCounts(summary ContextScopeSummary) string {
	parts := []string{fmt.Sprintf("%d active project(s)", summary.ActiveProjectCount)}
	if summary.Scope.IsCustom() {
		parts = append(parts, fmt.Sprintf("%d archived", summary.ArchivedProjectCount))
		// Sandbox and internal members are rare enough that a permanent "0
		// hidden" would be noise in a pane that already wraps.
		if summary.HiddenProjectCount > 0 {
			parts = append(parts, fmt.Sprintf("%d hidden", summary.HiddenProjectCount))
		}
		parts = append(parts, fmt.Sprintf("%d missing", summary.StaleReferenceCount))
	}
	if m.taskCounts != nil {
		total := 0
		for _, project := range summary.Scope.Projects(m.runtime) {
			if project != nil {
				total += m.taskCounts[project.ID]
			}
		}
		parts = append(parts, fmt.Sprintf("%d task(s)", total))
	}
	return strings.Join(parts, " · ")
}

func (m *ContextHubModel) renderSearchBar() string {
	switch {
	case m.HasActiveSearch():
		return "/" + m.SearchInputView()
	case m.SearchQuery() != "":
		matches := 0
		if m.surface != nil {
			matches = len(m.surface.Matches())
		}
		return "/" + m.SearchQuery() + MutedStyle.Render(fmt.Sprintf("  (%d matches)", matches))
	default:
		return ""
	}
}

func (m *ContextHubModel) subtitle() string {
	if m.editor != nil {
		if m.editor.isNew() {
			return "new custom context"
		}
		return "editing " + m.editor.originalName
	}
	custom, regular := 0, 0
	for _, summary := range m.summaries {
		if summary.Scope.IsCustom() {
			custom++
		} else if !summary.Scope.IsAll() {
			regular++
		}
	}
	activeProjects, archivedProjects, trash := m.scopeHubLifecycleCounts()
	return fmt.Sprintf(
		"%d base scopes · %d active · %d archived · %d trash",
		1+custom+regular,
		activeProjects,
		archivedProjects,
		trash,
	)
}

// footerHelp returns the widest help variant that still fits beside the footer
// label at the given width, falling back to the shortest one.
func (m *ContextHubModel) footerHelp(width int) string {
	variants := m.footerHelpVariants()
	budget := width - lipgloss.Width(contextHubFooterLabel) - 1
	for _, help := range variants {
		if lipgloss.Width(help) <= budget {
			return help
		}
	}
	return variants[len(variants)-1]
}

// footerHelpVariants lists the help strings for the current mode from widest to
// narrowest. Every variant keeps the keys a user needs to leave the mode.
func (m *ContextHubModel) footerHelpVariants() []string {
	switch {
	case m.editor != nil:
		return m.editorFooterHelpVariants()
	case m.actions != nil:
		return m.scopeHubActionFooterHelpVariants()
	case m.HasActiveSearch():
		return []string{"Enter keep filter · Esc clear"}
	case m.SearchQuery() != "":
		return []string{
			"j/k choose · Enter activate · Space actions · Esc clear search",
			"j/k · Enter · Space actions · Esc clear",
			"Enter · Space · Esc clear",
		}
	}
	return []string{
		"j/k choose · Enter activate · Space actions · / search",
		"j/k · Enter activate · Space actions · / search",
		"j/k · Enter · Space · /",
	}
}

func contextScopeDisplayName(scope ContextScope) string {
	if scope.IsAll() {
		return "All"
	}
	return scope.Name
}

func contextHubScopeKindLabel(scope ContextScope) string {
	switch scope.Kind {
	case ContextScopeCustom:
		return "Custom context · saved project group"
	case ContextScopeRegular:
		return "Regular context · project label"
	default:
		return "Every active project"
	}
}

func contextHubRowDetail(summary ContextScopeSummary) string {
	detail := fmt.Sprintf("%d project(s)", summary.ActiveProjectCount)
	if summary.Scope.IsCustom() {
		detail = "custom · " + detail
		if summary.ArchivedProjectCount > 0 {
			detail += fmt.Sprintf(" · %d archived", summary.ArchivedProjectCount)
		}
		if summary.HiddenProjectCount > 0 {
			detail += fmt.Sprintf(" · %d hidden", summary.HiddenProjectCount)
		}
		if summary.StaleReferenceCount > 0 {
			detail += fmt.Sprintf(" · %d missing", summary.StaleReferenceCount)
		}
	}
	return detail
}

func (m *ContextHubModel) IsFullHeight() bool { return true }

// HasTextInput reports true for the whole editor or action sheet, not just a
// focused input: the shell's q/w/u/y/#/|/: interceptors and the Manage hub's
// `v` interceptor must not mutate or replace the view beneath either modal.
func (m *ContextHubModel) HasTextInput() bool {
	if m.editor != nil || m.actions != nil {
		return true
	}
	return m.DefaultHasTextInput()
}

func (m *ContextHubModel) ShouldCaptureEsc() bool {
	return m.editor != nil || m.actions != nil || m.confirm.IsActive() || m.confirmDiscard ||
		m.SearchShouldCaptureEsc() || m.SearchQuery() != ""
}

func (m *ContextHubModel) GetConfirmDialog() *ConfirmDialog {
	if m.confirmDiscard {
		m.discardOverlay.Show("discard", "Discard unsaved changes?")
		return &m.discardOverlay
	}
	m.discardOverlay.Cancel()
	return &m.confirm
}

func (m *ContextHubModel) StatusBarLabel() string { return "Scope Hub" }

func (m *ContextHubModel) GetHelpItems() []HelpItem {
	if m.editor != nil {
		return m.editorHelpItems()
	}
	return []HelpItem{
		{Key: "j/k", Description: "Move selection", Category: "Navigation"},
		{Key: "g/G", Description: "Jump to the first or last scope item", Category: "Navigation"},
		{Key: "/", Description: "Search scopes, projects, and trash", Category: "Filters"},
		{Key: "enter", Description: "Activate a base scope or project focus", Category: "Actions"},
		{Key: "space", Description: "Open contextual administration actions", Category: "Actions"},
		{Key: "n", Description: "Create a custom context", Category: "Actions"},
		{Key: "e", Description: "Edit context membership", Category: "Actions"},
		{Key: "r", Description: "Rename a context", Category: "Actions"},
		{Key: "d", Description: "Delete a context", Category: "Actions"},
		{Key: "esc", Description: "Return to the Outliner", Category: "Navigation"},
	}
}
