package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// DiscoverySurfaceKind identifies the provider behind the shared Finder/List
// Surface interaction model.
type DiscoverySurfaceKind uint8

const (
	DiscoverySurfaceFinder DiscoverySurfaceKind = iota
	DiscoverySurfaceModes
)

const (
	finderResultCategoryRecent ListCategoryID = "finder-result:recent"
	finderResultCategoryPinned ListCategoryID = "finder-result:pinned"
)

// ShowFinderMsg opens the unified Finder.
type ShowFinderMsg struct {
	PreFilter string
}

type CloseFinderMsg struct{}

// ShowFinderContextActionsMsg requests the shared contextual sheet for the
// selected Finder row, carrying the row and the frozen context it was ranked in.
type ShowFinderContextActionsMsg struct {
	Item    ListItem
	Context CommandContext
}

// ShowModeListMsg opens the ten-mode catalog through the same searchable List
// Surface interaction used by the Finder.
type ShowModeListMsg struct{}

// ActivateOutlinerModeMsg switches the discovery Outliner to one catalog mode.
type ActivateOutlinerModeMsg struct {
	Mode OutlinerModeID
}

// DiscoveryListSelectMsg carries the selected typed ListItem and the frozen
// operation context from the route that opened discovery.
type DiscoveryListSelectMsg struct {
	Item    ListItem
	Context CommandContext
	Kind    DiscoverySurfaceKind
}

// DiscoveryListModel is the Bubble Tea adapter shared by the global Finder and
// the prefiltered mode catalog. Ranking and source construction stay in Finder
// and ListSource; this type owns only input, cursor movement, and rendering.
type DiscoveryListModel struct {
	kind           DiscoverySurfaceKind
	finder         *Finder
	finderCategory ListCategoryID
	surface        *ListSurfaceModel
	context        CommandContext
	searchInput    textinput.Model
	width          int
	height         int
	notice         string
}

// NewFinderDiscoveryModel projects the headless Finder into ListSurface.
func NewFinderDiscoveryModel(
	finder *Finder,
	context CommandContext,
	preFilter string,
) (*DiscoveryListModel, error) {
	category, query := parseFinderPreFilter(preFilter)
	input := newDiscoverySearchInput("Search tasks, projects, modes, tools, tags, filters, and more")
	if category != "" {
		input.Placeholder = "Search " + finderCategoryLabel(category)
	}
	input.SetValue(query)
	input.CursorEnd()
	model := &DiscoveryListModel{
		kind:           DiscoverySurfaceFinder,
		finder:         finder,
		finderCategory: category,
		context:        cloneCommandContext(context),
		searchInput:    input,
	}
	if err := model.rebuildFinderResults(); err != nil {
		return nil, err
	}
	return model, nil
}

// NewModeDiscoveryModel projects exactly the canonical mode catalog through
// the same ListSurface interaction and renderer as the Finder.
func NewModeDiscoveryModel(context CommandContext) (*DiscoveryListModel, error) {
	source := ListSourceFromOutlinerModes(OutlinerModeCatalog())
	surface, err := source.NewModel(ListSurfaceConfig{
		SelectionMode: ListSelectionSingle,
		EmptyState:    ListEmptyState{Title: "No modes available."},
		NoMatchState:  ListEmptyState{Title: "No matching modes.", Detail: "Clear or change the search."},
	})
	if err != nil {
		return nil, fmt.Errorf("build mode discovery: %w", err)
	}
	return &DiscoveryListModel{
		kind:        DiscoverySurfaceModes,
		surface:     surface,
		context:     cloneCommandContext(context),
		searchInput: newDiscoverySearchInput("Search modes..."),
	}, nil
}

func newDiscoverySearchInput(placeholder string) textinput.Model {
	input := textinput.New()
	input.Placeholder = placeholder
	input.CharLimit = 120
	input.SetWidth(50)
	input.Focus()
	applyThemedTextInput(&input)
	return input
}

// FinderResultListSource projects ranked result metadata into visible Recent
// and Pinned groups. The underlying item identity and typed FinderTarget stay
// unchanged, so activation and durable recency never depend on presentation.
func FinderResultListSource(resultSet FinderResultSet) ListSource {
	categories := []ListCategory{
		{ID: finderResultCategoryRecent, Label: "Recent"},
		{ID: finderResultCategoryPinned, Label: "Pinned"},
	}
	categories = append(categories, FinderCategories()...)
	items := make([]ListItem, 0, len(resultSet.Results))
	for _, result := range resultSet.Results {
		item := cloneListItem(result.Item)
		switch {
		case result.Recent:
			item.CategoryID = finderResultCategoryRecent
		case result.Pinned:
			item.CategoryID = finderResultCategoryPinned
		}
		items = append(items, item)
	}
	return ListSource{Items: items, Categories: categories}
}

func (model *DiscoveryListModel) rebuildFinderResults() error {
	if model == nil || model.kind != DiscoverySurfaceFinder {
		return nil
	}
	category := model.finderCategory
	query := model.searchInput.Value()
	if typedCategory, typedQuery := parseFinderPreFilter(query); typedCategory != "" {
		category = typedCategory
		query = typedQuery
	}
	resultSet := FinderResultSet{}
	if model.finder != nil {
		resultSet = model.finder.Results(query)
	}
	if category != "" {
		resultSet = model.finderCategoryResults(category, query)
	}
	source := FinderResultListSource(resultSet)
	if model.surface == nil {
		surface, err := source.NewModel(ListSurfaceConfig{
			SelectionMode: ListSelectionSingle,
			EmptyState:    ListEmptyState{Title: "No finder results available."},
			NoMatchState:  ListEmptyState{Title: "No matching results.", Detail: "Clear or change the search."},
		})
		if err != nil {
			return fmt.Errorf("build finder discovery: %w", err)
		}
		model.surface = surface
		return nil
	}
	if err := model.surface.Reload(source.Items, source.Categories); err != nil {
		return fmt.Errorf("refresh finder discovery: %w", err)
	}
	return nil
}

func (model *DiscoveryListModel) finderCategoryResults(
	category ListCategoryID,
	rawQuery string,
) FinderResultSet {
	query := strings.TrimSpace(rawQuery)
	if model.finder == nil {
		return FinderResultSet{Query: NewListQuery(query)}
	}
	var candidates []FinderResult
	if query == "" {
		for _, item := range model.finder.Items() {
			candidates = append(candidates, FinderResult{
				Item:   item,
				Pinned: finderItemPinned(item),
			})
		}
	} else {
		candidates = model.finder.Results(query).Results
	}
	filtered := make([]FinderResult, 0, len(candidates))
	for _, result := range candidates {
		if result.Item.CategoryID == category {
			filtered = append(filtered, result)
		}
	}
	return FinderResultSet{
		Query:   NewListQuery(query),
		Results: filtered,
	}
}

func parseFinderPreFilter(raw string) (ListCategoryID, string) {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, ":") {
		return "", trimmed
	}
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return "", ""
	}
	categories := map[string]ListCategoryID{
		":cmd":   FinderCategoryOperations,
		":task":  FinderCategoryTasks,
		":prj":   FinderCategoryProjects,
		":note":  FinderCategoryNotes,
		":view":  FinderCategoryTools,
		":mode":  FinderCategoryModes,
		":scope": FinderCategoryScopes,
		":tag":   FinderCategoryTags,
		":sv":    FinderCategorySavedFilters,
	}
	category, ok := categories[strings.ToLower(parts[0])]
	if !ok {
		return "", trimmed
	}
	return category, strings.Join(parts[1:], " ")
}

func finderCategoryLabel(id ListCategoryID) string {
	for _, category := range FinderCategories() {
		if category.ID == id {
			return strings.ToLower(category.Label) + "..."
		}
	}
	return "finder..."
}

// Init implements tea.Model.
func (model *DiscoveryListModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (model *DiscoveryListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		model.SetSize(msg.Width, msg.Height)
		return model, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return model, func() tea.Msg { return CloseFinderMsg{} }
		case "enter":
			item, ok := model.surface.CurrentItem()
			if !ok {
				return model, nil
			}
			if !item.Enabled() {
				model.notice = item.DisabledReason
				return model, nil
			}
			return model, func() tea.Msg {
				return DiscoveryListSelectMsg{
					Item:    item,
					Context: cloneCommandContext(model.context),
					Kind:    model.kind,
				}
			}
		case "tab":
			return model, model.contextActionsCmd()
		case "up", "ctrl+k", "ctrl+p", "shift+tab":
			model.surface.MoveCursor(-1)
			return model, nil
		case "down", "ctrl+j", "ctrl+n":
			model.surface.MoveCursor(1)
			return model, nil
		case "home":
			model.surface.MoveCursorToStart()
			return model, nil
		case "end":
			model.surface.MoveCursorToEnd()
			return model, nil
		case "pgup":
			model.surface.PageCursor(-1)
			return model, nil
		case "pgdown":
			model.surface.PageCursor(1)
			return model, nil
		}

		before := model.searchInput.Value()
		var cmd tea.Cmd
		model.searchInput, cmd = model.searchInput.Update(msg)
		if model.searchInput.Value() != before {
			model.notice = ""
			if model.kind == DiscoverySurfaceFinder {
				if err := model.rebuildFinderResults(); err != nil {
					model.notice = err.Error()
				}
			} else {
				model.surface.SetQuery(model.searchInput.Value())
			}
		}
		return model, cmd
	}
	return model, nil
}

// contextActionsCmd asks the host for the selected row's contextual sheet. The
// Finder keeps its query, cursor, and viewport because the sheet opens as an
// app-owned overlay above this model instead of replacing it.
func (model *DiscoveryListModel) contextActionsCmd() tea.Cmd {
	if model.kind != DiscoverySurfaceFinder {
		return nil
	}
	item, ok := model.surface.CurrentItem()
	if !ok {
		return nil
	}
	if !item.Enabled() {
		model.notice = item.DisabledReason
		return nil
	}
	return func() tea.Msg {
		return ShowFinderContextActionsMsg{
			Item:    item,
			Context: cloneCommandContext(model.context),
		}
	}
}

// SetSize implements the shared view sizing contract.
func (model *DiscoveryListModel) SetSize(width, height int) {
	if model == nil {
		return
	}
	model.width = width
	model.height = height
	model.searchInput.SetWidth(max(width-8, 20))
}

// HasTextInput reports that discovery always owns printable input.
func (model *DiscoveryListModel) HasTextInput() bool { return true }

// HasActiveSearch reports that discovery is a search-first surface.
func (model *DiscoveryListModel) HasActiveSearch() bool { return true }

// ShouldCaptureEsc allows the shell to close the Finder overlay normally.
func (model *DiscoveryListModel) ShouldCaptureEsc() bool { return false }

// IsFullHeight reports that discovery renders its own footer.
func (model *DiscoveryListModel) IsFullHeight() bool { return true }

// StatusBarLabel implements StatusBarLabeler.
func (model *DiscoveryListModel) StatusBarLabel() string {
	if model != nil && model.kind == DiscoverySurfaceModes {
		return "Modes"
	}
	return "Finder"
}

// View implements tea.Model.
func (model *DiscoveryListModel) View() tea.View {
	if model == nil || model.surface == nil {
		return tea.NewView("")
	}
	width := max(model.width, 40)
	height := max(model.height, 12)
	innerWidth := max(width-4, 24)
	listHeight := max(height-8, 5)
	title := model.StatusBarLabel()
	subtitle := fmt.Sprintf("%d results", len(model.surface.Matches()))

	var content strings.Builder
	content.WriteString(renderFullscreenTitle(title, subtitle))
	content.WriteString("\n")
	content.WriteString(model.searchInput.View())
	content.WriteString("\n\n")
	content.WriteString(NewListSurfaceRenderer(ListSurfaceRenderConfig{
		Width:       innerWidth,
		Height:      listHeight,
		Layout:      ListSurfaceLayoutAuto,
		ShowPreview: true,
		HideHelp:    true,
	}).Render(model.surface).Content)
	content.WriteString("\n")
	if model.notice != "" {
		content.WriteString(MutedStyle.Render(model.notice))
		content.WriteString("\n")
	}
	content.WriteString(renderStatusBarFooter(
		HelpStyle.Render("Type to search · Enter open · Tab actions · Esc close"),
		"",
		width,
	))
	return tea.NewView(content.String())
}
