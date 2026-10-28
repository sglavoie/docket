package listsurface

// ListSurfaceModel owns reusable query, ranking, cursor, selection, category,
// preview, action, and viewport behavior. It has no Bubble Tea dependencies.
type ListSurfaceModel struct {
	config       ListSurfaceConfig
	categories   []ListCategory
	items        []ListItem
	query        ListQuery
	matches      []listSurfaceMatch
	cursor       int
	cursorAnchor ListItemID
	selected     map[ListItemID]struct{}
	width        int
	height       int
	scrollOffset int
	// pointerFrame is the pane the last render drew, which is the only frame a
	// click may be read against. See list_surface_pointer.go.
	pointerFrame ListPointerFrame
	// previewLeft is the first column the last render drew a preview pane in,
	// or zero when it drew the list alone.
	previewLeft int
}

// NewListSurfaceModel validates and copies all source data.
func NewListSurfaceModel(
	items []ListItem,
	categories []ListCategory,
	config ListSurfaceConfig,
) (*ListSurfaceModel, error) {
	if !validListSelectionMode(config.SelectionMode) {
		return nil, ErrListSelectionMode
	}
	normalizedCategories, categoryIndex, err := normalizeListCategories(categories)
	if err != nil {
		return nil, err
	}
	normalizedItems, err := normalizeListItems(items, categoryIndex)
	if err != nil {
		return nil, err
	}

	model := &ListSurfaceModel{
		config:     config,
		categories: normalizedCategories,
		items:      normalizedItems,
		cursor:     -1,
		selected:   make(map[ListItemID]struct{}),
	}
	model.rebuildMatches()
	if len(model.matches) > 0 {
		model.cursor = 0
		model.cursorAnchor = model.matches[0].item.ID
	}
	return model, nil
}

// Config returns the immutable behavior configuration.
func (model *ListSurfaceModel) Config() ListSurfaceConfig {
	if model == nil {
		return ListSurfaceConfig{}
	}
	return model.config
}

// Items returns source items in their caller-supplied order.
func (model *ListSurfaceModel) Items() []ListItem {
	if model == nil {
		return []ListItem{}
	}
	return cloneListItems(model.items)
}

// Categories returns categories in display order.
func (model *ListSurfaceModel) Categories() []ListCategory {
	if model == nil {
		return []ListCategory{}
	}
	return append([]ListCategory(nil), model.categories...)
}

// Query returns a defensive copy of the active query.
func (model *ListSurfaceModel) Query() ListQuery {
	if model == nil {
		return ListQuery{}
	}
	return cloneListQuery(model.query)
}

// Matches returns the current grouped and ranked item results.
func (model *ListSurfaceModel) Matches() []ListMatch {
	if model == nil {
		return []ListMatch{}
	}
	matches := make([]ListMatch, 0, len(model.matches))
	for _, match := range model.matches {
		matches = append(matches, exportListMatch(match))
	}
	return matches
}

// SetQuery fuzzy-filters the source while retaining the cursor's stable anchor.
// If the anchor is temporarily hidden, a deterministic clamped cursor is shown;
// clearing the query restores the anchor unless the user moved the cursor.
func (model *ListSurfaceModel) SetQuery(raw string) {
	if model == nil {
		return
	}
	query := NewListQuery(raw)
	if model.query.Raw == query.Raw {
		return
	}
	oldCursor := model.cursor
	model.query = query
	model.rebuildMatches()
	model.restoreCursor(oldCursor, false)
}

// SetItems atomically validates a reload and preserves cursor and selection by
// stable ID. Missing and newly disabled selections are removed.
func (model *ListSurfaceModel) SetItems(items []ListItem) error {
	if model == nil {
		return nil
	}
	categoryIndex := listCategoryIndex(model.categories)
	normalized, err := normalizeListItems(items, categoryIndex)
	if err != nil {
		return err
	}

	oldCursor := model.cursor
	model.items = normalized
	model.pruneSelection()
	model.rebuildMatches()
	model.restoreCursor(oldCursor, !model.sourceContains(model.cursorAnchor))
	return nil
}

// Reload atomically replaces categories and items while preserving identity.
func (model *ListSurfaceModel) Reload(
	items []ListItem,
	categories []ListCategory,
) error {
	if model == nil {
		return nil
	}
	normalizedCategories, categoryIndex, err := normalizeListCategories(categories)
	if err != nil {
		return err
	}
	normalizedItems, err := normalizeListItems(items, categoryIndex)
	if err != nil {
		return err
	}

	oldCursor := model.cursor
	model.categories = normalizedCategories
	model.items = normalizedItems
	model.pruneSelection()
	model.rebuildMatches()
	model.restoreCursor(oldCursor, !model.sourceContains(model.cursorAnchor))
	return nil
}

// SetEmptyStates replaces the contextual copy shown while the result set is
// empty. A surface whose own filtering can empty the source needs to say why,
// and the reason is not known at construction time.
func (model *ListSurfaceModel) SetEmptyStates(empty, noMatch ListEmptyState) {
	if model == nil {
		return
	}
	model.config.EmptyState = empty
	model.config.NoMatchState = noMatch
}
