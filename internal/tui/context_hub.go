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

// contextHubWideWidth is the width at which the hub splits into a browse list
// and a scope preview. Below it the preview is dropped rather than squeezed.
const contextHubWideWidth = 81

// contextHubFooterLabel is the footer's right-hand label. Help variants are
// measured against it so the pair always shares a single line: the shell swaps a
// full-height view's last two lines for the status bar, so help wrapped onto a
// third line would leave an orphaned separator behind that swap.
const contextHubFooterLabel = "Contexts"

// ContextHubModel is the Manage-hosted surface for browsing, applying, and
// editing the active project context. It renders the shared
// ContextScopeSummaries list and reports a selection through
// ContextScopeSelectedMsg so the shell applies scope through the single existing
// path. Create/edit/rename/delete for both context kinds run inline here and
// commit through ContextMutator.
type ContextHubModel struct {
	BaseSearchableView
	runtime    *config.RuntimeState
	mutator    *ContextMutator
	summaries  []ContextScopeSummary
	filtered   []ContextScopeSummary
	active     ContextScope
	cursor     int
	scroll     int
	taskCounts map[string]int

	editor         *contextEditor
	confirm        ConfirmDialog
	pendingDelete  ContextScope
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
	m.filtered = m.summaries
	for i, summary := range m.filtered {
		if summary.Scope == active {
			m.cursor = i
			break
		}
	}
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
		return
	}
	m.taskCounts = counts
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
	if confirmed, handled := m.confirm.HandleKey(key.String()); handled {
		if !confirmed {
			m.pendingDelete = ContextScope{}
			return m, nil
		}
		return m, m.deleteConfirmedContext()
	}
	if m.editor != nil {
		return m.updateEditor(key)
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
		m.setCursor(m.cursor + 1)
	case "k", "up":
		m.setCursor(m.cursor - 1)
	case "g", "home":
		m.setCursor(0)
	case "G", "end":
		m.setCursor(len(m.filtered) - 1)
	case "n":
		m.openEditor("")
	case "e":
		m.editSelected()
	case "r":
		m.renameSelected()
	case "d":
		m.requestDeleteSelected()
	case "enter":
		if m.cursor < 0 || m.cursor >= len(m.filtered) {
			return m, nil
		}
		scope := m.filtered[m.cursor].Scope
		return m, func() tea.Msg {
			return ContextScopeSelectedMsg{
				SelectedContext: scope.Name,
				IsCustomContext: scope.IsCustom(),
			}
		}
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

func (m *ContextHubModel) setCursor(index int) {
	m.cursor = min(max(index, 0), max(len(m.filtered)-1, 0))
}

// applyFilter narrows the browse list to the scopes whose own name or whose
// member project names match the query, keeping the highlighted scope selected
// when it survives the filter.
func (m *ContextHubModel) applyFilter() {
	var selected ContextScope
	hadSelection := m.cursor >= 0 && m.cursor < len(m.filtered)
	if hadSelection {
		selected = m.filtered[m.cursor].Scope
	}

	query := strings.ToLower(strings.TrimSpace(m.SearchQuery()))
	if query == "" {
		m.filtered = m.summaries
	} else {
		filtered := make([]ContextScopeSummary, 0, len(m.summaries))
		for _, summary := range m.summaries {
			if m.scopeMatches(summary.Scope, query) {
				filtered = append(filtered, summary)
			}
		}
		m.filtered = filtered
	}

	m.cursor = 0
	m.scroll = 0
	if !hadSelection {
		return
	}
	for i, summary := range m.filtered {
		if summary.Scope == selected {
			m.cursor = i
			return
		}
	}
}

func (m *ContextHubModel) scopeMatches(scope ContextScope, query string) bool {
	if strings.Contains(strings.ToLower(contextScopeDisplayName(scope)), query) {
		return true
	}
	for _, project := range scope.Projects(m.runtime) {
		if project != nil && strings.Contains(strings.ToLower(project.Name), query) {
			return true
		}
	}
	return false
}

func (m *ContextHubModel) View() tea.View {
	width := max(m.width, 1)
	height := max(m.height, 1)

	title := renderFullscreenTitle("Contexts", m.subtitle(), m.AccentColor())
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
	if width < contextHubWideWidth {
		return m.renderList(width, height)
	}
	listWidth := max(width*45/100, 1)
	previewWidth := max(width-listWidth-SeparatorWidth, 1)

	listPane := lipgloss.NewStyle().Width(listWidth).Height(height).
		Render(m.renderList(listWidth, height))
	previewPane := lipgloss.NewStyle().Width(previewWidth).Height(height).
		Render(fitBlockHeight(m.renderPreview(previewWidth), height))
	separator := SeparatorStyle.Render(strings.TrimRight(strings.Repeat("│\n", height), "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, listPane, " ", separator, " ", previewPane)
}

// renderList renders the grouped browse list: All, then custom contexts, then
// regular project-context labels, with a bounded window around the cursor.
func (m *ContextHubModel) renderList(paneWidth, paneHeight int) string {
	if len(m.filtered) == 0 {
		if m.SearchQuery() != "" {
			return renderEmptyState(
				fmt.Sprintf("No contexts match %q.", m.SearchQuery()),
				"Esc clears the search.",
			)
		}
		return renderEmptyState("No contexts defined.", "Every project without a context is still in All.")
	}

	lines := make([]string, 0, len(m.filtered)+6)
	cursorLine := 0
	group := ContextScopeKind(-1)
	for i, summary := range m.filtered {
		if summary.Scope.Kind != group {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			group = summary.Scope.Kind
			lines = append(lines, SectionHeaderStyle.Render(contextHubGroupTitle(group)))
		}
		if i == m.cursor {
			cursorLine = len(lines)
		}
		lines = append(lines, m.renderRow(i, summary, paneWidth))
	}
	visible, scroll := contextHubWindow(lines, cursorLine, m.scroll, paneHeight)
	m.scroll = scroll
	return strings.Join(visible, "\n")
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

func (m *ContextHubModel) renderRow(index int, summary ContextScopeSummary, paneWidth int) string {
	name := contextScopeDisplayName(summary.Scope)
	if summary.Scope == m.active {
		name += " (active)"
	}
	detail := "  " + MutedStyle.Render(contextHubRowDetail(summary))
	if index != m.cursor {
		return truncateToVisualWidth("  "+BoldStyle.Render(name)+detail, paneWidth)
	}
	line := IconCursorRight + " " + SelectedItemTextStyle.Render(name) + detail
	return SelectionRowStyle.Width(paneWidth).Render(truncateToVisualWidth(line, paneWidth))
}

// renderPreview describes the highlighted scope: what it is, how many projects
// it resolves to, and which projects the Outliner would show.
func (m *ContextHubModel) renderPreview(paneWidth int) string {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return MutedStyle.Render("No context selected.")
	}
	summary := m.filtered[m.cursor]

	var b strings.Builder
	b.WriteString(BoldStyle.Render(truncateToVisualWidth(contextScopeDisplayName(summary.Scope), paneWidth)))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(contextHubScopeKindLabel(summary.Scope)))
	b.WriteString("\n\n")
	// The counts are the preview's whole point, so they wrap rather than clip:
	// at 100 columns the custom-context line (active, archived, missing, tasks)
	// is wider than the preview pane.
	b.WriteString(MutedStyle.Render(strings.Join(wrapText(m.previewCounts(summary), paneWidth), "\n")))
	b.WriteString("\n\n")

	projects := summary.Scope.Projects(m.runtime)
	if len(projects) == 0 {
		b.WriteString(renderEmptyState(
			"No active projects in this context.",
			"Applying it leaves the Outliner empty.",
		))
		return b.String()
	}

	b.WriteString(SectionHeaderStyle.Render("Projects"))
	b.WriteString("\n")
	for _, project := range projects {
		b.WriteString(m.renderPreviewProject(project, paneWidth))
		b.WriteString("\n")
	}
	return b.String()
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

func (m *ContextHubModel) renderPreviewProject(project *models.Project, paneWidth int) string {
	if project == nil {
		return ""
	}
	if m.taskCounts == nil {
		return truncateToVisualWidth("  "+project.Name, paneWidth)
	}
	count := fmt.Sprintf("  %d task(s)", m.taskCounts[project.ID])
	name := truncateToVisualWidth("  "+project.Name, max(paneWidth-lipgloss.Width(count), 1))
	return name + MutedStyle.Render(count)
}

func (m *ContextHubModel) renderSearchBar() string {
	switch {
	case m.HasActiveSearch():
		return "/" + m.SearchInputView()
	case m.SearchQuery() != "":
		return "/" + m.SearchQuery() + MutedStyle.Render(fmt.Sprintf("  (%d matches)", len(m.filtered)))
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
	return fmt.Sprintf("%d custom · %d project contexts", custom, regular)
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
	case m.HasActiveSearch():
		return []string{"Enter keep filter · Esc clear"}
	case m.SearchQuery() != "":
		return []string{"j/k choose · Enter apply · Esc clear search"}
	}
	return []string{
		"j/k choose · Enter apply · n new · e edit · r rename · d delete · / search",
		"j/k · Enter apply · n new · e edit · r rename · d delete · / search",
		"j/k · Enter apply · n/e/r/d · / search",
	}
}

func contextScopeDisplayName(scope ContextScope) string {
	if scope.IsAll() {
		return "All"
	}
	return scope.Name
}

func contextHubGroupTitle(kind ContextScopeKind) string {
	switch kind {
	case ContextScopeCustom:
		return "Custom Contexts"
	case ContextScopeRegular:
		return "Regular Contexts"
	default:
		return "All"
	}
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

// HasTextInput reports true for the whole editor, not just its focused input:
// the shell's q/w/u/y/#/|/: interceptors and the Manage hub's `v` interceptor
// would otherwise swallow the editor's own single-letter keys.
func (m *ContextHubModel) HasTextInput() bool {
	if m.editor != nil {
		return true
	}
	return m.DefaultHasTextInput()
}

func (m *ContextHubModel) ShouldCaptureEsc() bool {
	return m.editor != nil || m.confirm.IsActive() || m.confirmDiscard ||
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

func (m *ContextHubModel) StatusBarLabel() string { return "Contexts" }

func (m *ContextHubModel) GetHelpItems() []HelpItem {
	if m.editor != nil {
		return m.editorHelpItems()
	}
	return []HelpItem{
		{Key: "j/k", Description: "Move selection", Category: "Navigation"},
		{Key: "g/G", Description: "Jump to first or last context", Category: "Navigation"},
		{Key: "/", Description: "Search contexts and project names", Category: "Filters"},
		{Key: "enter", Description: "Apply the selected context", Category: "Actions"},
		{Key: "n", Description: "New custom context", Category: "Actions"},
		{Key: "e", Description: "Edit the selected context's membership", Category: "Actions"},
		{Key: "r", Description: "Rename the selected context", Category: "Actions"},
		{Key: "d", Description: "Delete the selected context", Category: "Actions"},
		{Key: "esc", Description: "Return to the Outliner", Category: "Navigation"},
	}
}
