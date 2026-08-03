package tui

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
)

// ListItemID is the stable identity used for cursor and selection preservation.
type ListItemID string

// ListCategoryID is the stable identity of a list category.
type ListCategoryID string

// ListSelectionMode controls whether a surface stores one or many selections.
type ListSelectionMode uint8

const (
	ListSelectionSingle ListSelectionMode = iota
	ListSelectionMulti
)

// ListEmptyKind distinguishes an empty source from a query with no matches.
type ListEmptyKind uint8

const (
	ListEmptySource ListEmptyKind = iota + 1
	ListEmptyNoMatches
)

// ListRowKind distinguishes selectable item rows from category headers.
type ListRowKind uint8

const (
	ListRowItem ListRowKind = iota
	ListRowCategory
)

var (
	ErrListItemIDRequired       = errors.New("list item ID is required")
	ErrListItemDuplicateID      = errors.New("list item ID is duplicated")
	ErrListCategoryIDRequired   = errors.New("list category ID is required")
	ErrListCategoryDuplicateID  = errors.New("list category ID is duplicated")
	ErrListCategoryUnknown      = errors.New("list item references an unknown category")
	ErrListItemDisabled         = errors.New("list item is disabled")
	ErrListItemNotFound         = errors.New("list item was not found")
	ErrListSelectionMode        = errors.New("invalid list selection mode")
	ErrListSelectionCardinality = errors.New("single-selection list accepts at most one selection")
	ErrListActionUnavailable    = errors.New("list action is unavailable")
)

// ListPreviewField is one renderer-neutral label/value pair in an item preview.
type ListPreviewField struct {
	Label string
	Value string
}

// ListPreview is text-only preview content that compact and wide renderers can
// arrange without depending on a picker-specific model.
type ListPreview struct {
	Title   string
	Summary string
	Body    string
	Fields  []ListPreviewField
}

// Empty reports whether the preview has any displayable content.
func (preview ListPreview) Empty() bool {
	return preview.Title == "" &&
		preview.Summary == "" &&
		preview.Body == "" &&
		len(preview.Fields) == 0
}

// ListItem is the complete renderer-neutral representation of one row.
// Data is an adapter-owned payload; the behavior model never inspects it.
type ListItem struct {
	ID             ListItemID
	CategoryID     ListCategoryID
	PrimaryText    string
	SecondaryText  string
	Aliases        []string
	DisabledReason string
	// Indeterminate renders the third state used by bulk membership editors.
	// It is presentation metadata only; adapters still own how toggling the row
	// changes the underlying value.
	Indeterminate bool
	Preview       ListPreview
	OperationIDs  []OperationID
	Data          any
}

// Enabled reports whether the item may be selected or activated.
func (item ListItem) Enabled() bool {
	return strings.TrimSpace(item.DisabledReason) == ""
}

// ListCategory describes one stable group. Slice order supplied to the model
// is the category display order.
type ListCategory struct {
	ID    ListCategoryID
	Label string
}

// ListEmptyState is renderer-neutral copy for an empty surface.
type ListEmptyState struct {
	Title  string
	Detail string
}

// ListSurfaceConfig contains behavior that varies between list callers.
type ListSurfaceConfig struct {
	SelectionMode ListSelectionMode
	EmptyState    ListEmptyState
	NoMatchState  ListEmptyState
}

// ListQuery retains both the caller's input and its whitespace-normalized fuzzy
// form. Terms are exposed for renderers that display query diagnostics.
type ListQuery struct {
	Raw        string
	Normalized string
	Terms      []string
}

// NewListQuery builds the canonical fuzzy query used by ListSurfaceModel.
func NewListQuery(raw string) ListQuery {
	terms := strings.Fields(raw)
	return ListQuery{
		Raw:        raw,
		Normalized: strings.Join(terms, " "),
		Terms:      append([]string(nil), terms...),
	}
}

// ListMatch exposes the fuzzy score while retaining stable source identity.
type ListMatch struct {
	Item ListItem
}

// ListSurfaceRow is one headless row. Category rows are never cursor targets.
type ListSurfaceRow struct {
	Kind     ListRowKind
	Category ListCategory
	Match    ListMatch
	Cursor   bool
	Selected bool
}

// ListViewport is a snapshot of the bounded list window.
type ListViewport struct {
	Width         int
	Height        int
	Offset        int
	TotalRows     int
	CanScrollUp   bool
	CanScrollDown bool
}

// ListEmptyContext pairs the configured copy with the reason it is displayed.
type ListEmptyContext struct {
	Kind  ListEmptyKind
	State ListEmptyState
}

// ListActionState describes the operations common to the active item set.
// In multi-select mode explicit selections win; otherwise the cursor item is
// the target. Adapters remain responsible for mapping item IDs into the
// CommandContext identities consumed by OperationExecutor.
type ListActionState struct {
	ItemIDs        []ListItemID
	OperationIDs   []OperationID
	DisabledReason string
}

// Enabled reports whether the active item set can offer contextual actions.
func (state ListActionState) Enabled() bool {
	return len(state.ItemIDs) > 0 && state.DisabledReason == ""
}

// Descriptors resolves available action metadata without invoking handlers.
func (state ListActionState) Descriptors(
	registry *OperationRegistry,
	ctx CommandContext,
) ([]OperationDescriptor, error) {
	if registry == nil {
		return nil, fmt.Errorf("list action state requires an operation registry")
	}
	if !state.Enabled() {
		return []OperationDescriptor{}, nil
	}

	descriptors := make([]OperationDescriptor, 0, len(state.OperationIDs))
	for _, id := range state.OperationIDs {
		descriptor, ok := registry.Descriptor(id)
		if !ok {
			return nil, fmt.Errorf("%w: operation %s is not registered", ErrListActionUnavailable, id)
		}
		if descriptor.Applicable(ctx) {
			descriptors = append(descriptors, descriptor)
		}
	}
	return descriptors, nil
}

// ListTextLayout is a visual-width-safe primary/secondary row layout.
type ListTextLayout struct {
	Primary   string
	Secondary string
	Gap       string
}

// Text returns the fitted inline text.
func (layout ListTextLayout) Text() string {
	if layout.Secondary == "" {
		return layout.Primary
	}
	return layout.Primary + layout.Gap + layout.Secondary
}

type listSurfaceMatch struct {
	item        ListItem
	score       int
	sourceIndex int
}

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

// Cursor returns the selectable match index, or -1 when no items match.
func (model *ListSurfaceModel) Cursor() int {
	if model == nil {
		return -1
	}
	return model.cursor
}

// CurrentItem returns the item under the cursor.
func (model *ListSurfaceModel) CurrentItem() (ListItem, bool) {
	if model == nil || model.cursor < 0 || model.cursor >= len(model.matches) {
		return ListItem{}, false
	}
	return cloneListItem(model.matches[model.cursor].item), true
}

// SetCursorToID moves to a visible item and updates the preservation anchor.
func (model *ListSurfaceModel) SetCursorToID(id ListItemID) bool {
	if model == nil {
		return false
	}
	for index, match := range model.matches {
		if match.item.ID != id {
			continue
		}
		model.cursor = index
		model.cursorAnchor = id
		model.ensureCursorVisible()
		return true
	}
	return false
}

// MoveCursor moves without wrapping and skips category headers by indexing only
// matches. It returns whether the cursor changed.
func (model *ListSurfaceModel) MoveCursor(delta int) bool {
	if model == nil || len(model.matches) == 0 || delta == 0 {
		return false
	}
	next := min(max(model.cursor+delta, 0), len(model.matches)-1)
	if next == model.cursor {
		return false
	}
	model.cursor = next
	model.cursorAnchor = model.matches[next].item.ID
	model.ensureCursorVisible()
	return true
}

// MoveCursorToStart selects the first matching item.
func (model *ListSurfaceModel) MoveCursorToStart() bool {
	if model == nil || len(model.matches) == 0 {
		return false
	}
	return model.setCursorIndex(0)
}

// MoveCursorToEnd selects the last matching item.
func (model *ListSurfaceModel) MoveCursorToEnd() bool {
	if model == nil || len(model.matches) == 0 {
		return false
	}
	return model.setCursorIndex(len(model.matches) - 1)
}

// PageCursor moves by the selectable capacity of each page.
func (model *ListSurfaceModel) PageCursor(pages int) bool {
	if model == nil || pages == 0 {
		return false
	}
	pageSize := max(model.height-1, 1)
	return model.MoveCursor(pages * pageSize)
}

// SetSize updates the headless viewport and keeps its offset in bounds.
func (model *ListSurfaceModel) SetSize(width, height int) {
	if model == nil {
		return
	}
	model.width = max(width, 0)
	model.height = max(height, 0)
	model.ensureCursorVisible()
}

// Viewport returns current scroll boundaries.
func (model *ListSurfaceModel) Viewport() ListViewport {
	if model == nil {
		return ListViewport{}
	}
	total := len(model.buildRows())
	maxOffset := max(total-model.height, 0)
	offset := min(max(model.scrollOffset, 0), maxOffset)
	return ListViewport{
		Width:         model.width,
		Height:        model.height,
		Offset:        offset,
		TotalRows:     total,
		CanScrollUp:   offset > 0,
		CanScrollDown: offset < maxOffset,
	}
}

// Rows returns every category and item row.
func (model *ListSurfaceModel) Rows() []ListSurfaceRow {
	if model == nil {
		return []ListSurfaceRow{}
	}
	return model.buildRows()
}

// VisibleRows returns a bounded copy of the current viewport.
func (model *ListSurfaceModel) VisibleRows() []ListSurfaceRow {
	if model == nil || model.height <= 0 {
		return []ListSurfaceRow{}
	}
	rows := model.buildRows()
	if len(rows) == 0 {
		return []ListSurfaceRow{}
	}
	start := min(max(model.scrollOffset, 0), max(len(rows)-model.height, 0))
	end := min(start+model.height, len(rows))
	return append([]ListSurfaceRow(nil), rows[start:end]...)
}

// EmptyContext returns contextual copy only while the result set is empty.
func (model *ListSurfaceModel) EmptyContext() (ListEmptyContext, bool) {
	if model == nil || len(model.matches) > 0 {
		return ListEmptyContext{}, false
	}
	if len(model.items) == 0 {
		return ListEmptyContext{
			Kind:  ListEmptySource,
			State: model.config.EmptyState,
		}, true
	}
	return ListEmptyContext{
		Kind:  ListEmptyNoMatches,
		State: model.config.NoMatchState,
	}, true
}

// SelectCurrent stores the enabled cursor item. Single selection replaces the
// prior value; multi selection adds to it.
func (model *ListSurfaceModel) SelectCurrent() error {
	item, ok := model.CurrentItem()
	if !ok {
		return ErrListItemNotFound
	}
	if !item.Enabled() {
		return fmt.Errorf("%w: %s", ErrListItemDisabled, item.DisabledReason)
	}
	if model.config.SelectionMode == ListSelectionSingle {
		clear(model.selected)
	}
	model.selected[item.ID] = struct{}{}
	return nil
}

// ToggleCurrentSelection toggles in multi-select mode and selects in
// single-select mode. It returns the resulting selected state.
func (model *ListSurfaceModel) ToggleCurrentSelection() (bool, error) {
	item, ok := model.CurrentItem()
	if !ok {
		return false, ErrListItemNotFound
	}
	if !item.Enabled() {
		return false, fmt.Errorf("%w: %s", ErrListItemDisabled, item.DisabledReason)
	}
	if model.config.SelectionMode == ListSelectionSingle {
		clear(model.selected)
		model.selected[item.ID] = struct{}{}
		return true, nil
	}
	if _, selected := model.selected[item.ID]; selected {
		delete(model.selected, item.ID)
		return false, nil
	}
	model.selected[item.ID] = struct{}{}
	return true, nil
}

// SetSelectedIDs atomically restores explicit stable-ID selections.
func (model *ListSurfaceModel) SetSelectedIDs(ids []ListItemID) error {
	if model == nil {
		return nil
	}
	normalized := make([]ListItemID, 0, len(ids))
	seen := make(map[ListItemID]struct{}, len(ids))
	for _, id := range ids {
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		item, ok := model.sourceItem(id)
		if !ok {
			return fmt.Errorf("%w: %s", ErrListItemNotFound, id)
		}
		if !item.Enabled() {
			return fmt.Errorf("%w: %s", ErrListItemDisabled, item.DisabledReason)
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	if model.config.SelectionMode == ListSelectionSingle && len(normalized) > 1 {
		return ErrListSelectionCardinality
	}

	next := make(map[ListItemID]struct{}, len(normalized))
	for _, id := range normalized {
		next[id] = struct{}{}
	}
	model.selected = next
	return nil
}

// ClearSelection removes every explicit selection.
func (model *ListSurfaceModel) ClearSelection() {
	if model != nil {
		clear(model.selected)
	}
}

// IsSelected reports whether a stable ID is explicitly selected.
func (model *ListSurfaceModel) IsSelected(id ListItemID) bool {
	if model == nil {
		return false
	}
	_, ok := model.selected[id]
	return ok
}

// SelectedIDs returns IDs in source order rather than map iteration order.
func (model *ListSurfaceModel) SelectedIDs() []ListItemID {
	if model == nil {
		return []ListItemID{}
	}
	ids := make([]ListItemID, 0, len(model.selected))
	for _, item := range model.items {
		if model.IsSelected(item.ID) {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

// SelectedItems returns selected items in source order.
func (model *ListSurfaceModel) SelectedItems() []ListItem {
	if model == nil {
		return []ListItem{}
	}
	items := make([]ListItem, 0, len(model.selected))
	for _, item := range model.items {
		if model.IsSelected(item.ID) {
			items = append(items, cloneListItem(item))
		}
	}
	return items
}

// CurrentPreview returns preview content for the cursor item.
func (model *ListSurfaceModel) CurrentPreview() (ListPreview, bool) {
	item, ok := model.CurrentItem()
	if !ok || item.Preview.Empty() {
		return ListPreview{}, false
	}
	return cloneListPreview(item.Preview), true
}

// ActionState returns operations common to the active item set.
func (model *ListSurfaceModel) ActionState() ListActionState {
	if model == nil {
		return ListActionState{}
	}
	items := model.actionItems()
	if len(items) == 0 {
		return ListActionState{}
	}

	state := ListActionState{
		ItemIDs: make([]ListItemID, 0, len(items)),
	}
	for _, item := range items {
		state.ItemIDs = append(state.ItemIDs, item.ID)
		if !item.Enabled() {
			state.DisabledReason = item.DisabledReason
			state.OperationIDs = nil
			return state
		}
	}

	state.OperationIDs = append([]OperationID(nil), items[0].OperationIDs...)
	for _, item := range items[1:] {
		state.OperationIDs = intersectOperationIDs(state.OperationIDs, item.OperationIDs)
	}
	return state
}

// ExecuteAction enforces item-declared action availability, then delegates all
// execution policy to the centralized OperationExecutor.
func (model *ListSurfaceModel) ExecuteAction(
	executor *OperationExecutor,
	request OperationExecutionRequest,
) OperationExecutionResult {
	if executor == nil {
		return OperationExecutionResult{
			Err: fmt.Errorf("%w: operation executor is nil", ErrListActionUnavailable),
		}
	}
	state := model.ActionState()
	if !state.Enabled() || !slices.Contains(state.OperationIDs, request.ID) {
		detail := string(request.ID)
		if state.DisabledReason != "" {
			detail = state.DisabledReason
		}
		return OperationExecutionResult{
			Err: fmt.Errorf("%w: %s", ErrListActionUnavailable, detail),
		}
	}
	return executor.Execute(request)
}

// LayoutListText fits primary and secondary text by terminal display cells.
// Primary text gets preference; secondary text uses at most one third of a
// constrained row unless the primary leaves spare capacity.
func LayoutListText(primary, secondary string, width int) ListTextLayout {
	layout := ListTextLayout{}
	if width <= 0 {
		return layout
	}

	primaryWidth := lipgloss.Width(primary)
	secondaryWidth := lipgloss.Width(secondary)
	if secondary == "" {
		layout.Primary = truncateToVisualWidth(primary, width)
		return layout
	}

	const gap = "  "
	gapWidth := lipgloss.Width(gap)
	if primaryWidth+gapWidth+secondaryWidth <= width {
		layout.Primary = primary
		layout.Secondary = secondary
		layout.Gap = gap
		return layout
	}
	if width <= gapWidth+1 {
		layout.Primary = truncateToVisualWidth(primary, width)
		return layout
	}

	available := width - gapWidth
	secondaryBudget := min(secondaryWidth, max(available/3, 1))
	primaryBudget := available - secondaryBudget
	if primaryWidth < primaryBudget {
		primaryBudget = primaryWidth
		secondaryBudget = available - primaryBudget
	}
	if secondaryWidth < secondaryBudget {
		secondaryBudget = secondaryWidth
		primaryBudget = available - secondaryBudget
	}

	layout.Primary = truncateToVisualWidth(primary, primaryBudget)
	layout.Secondary = truncateToVisualWidth(secondary, secondaryBudget)
	layout.Gap = gap
	return layout
}

func validListSelectionMode(mode ListSelectionMode) bool {
	return mode == ListSelectionSingle || mode == ListSelectionMulti
}

func normalizeListCategories(
	categories []ListCategory,
) ([]ListCategory, map[ListCategoryID]ListCategory, error) {
	normalized := make([]ListCategory, 0, len(categories))
	index := make(map[ListCategoryID]ListCategory, len(categories))
	for _, category := range categories {
		category.ID = ListCategoryID(strings.TrimSpace(string(category.ID)))
		category.Label = strings.TrimSpace(category.Label)
		if category.ID == "" {
			return nil, nil, ErrListCategoryIDRequired
		}
		if _, duplicate := index[category.ID]; duplicate {
			return nil, nil, fmt.Errorf("%w: %s", ErrListCategoryDuplicateID, category.ID)
		}
		index[category.ID] = category
		normalized = append(normalized, category)
	}
	return normalized, index, nil
}

func normalizeListItems(
	items []ListItem,
	categories map[ListCategoryID]ListCategory,
) ([]ListItem, error) {
	normalized := make([]ListItem, 0, len(items))
	seen := make(map[ListItemID]struct{}, len(items))
	for _, item := range items {
		item.ID = ListItemID(strings.TrimSpace(string(item.ID)))
		item.CategoryID = ListCategoryID(strings.TrimSpace(string(item.CategoryID)))
		item.DisabledReason = strings.TrimSpace(item.DisabledReason)
		if item.ID == "" {
			return nil, ErrListItemIDRequired
		}
		if _, duplicate := seen[item.ID]; duplicate {
			return nil, fmt.Errorf("%w: %s", ErrListItemDuplicateID, item.ID)
		}
		if item.CategoryID != "" {
			if _, ok := categories[item.CategoryID]; !ok {
				return nil, fmt.Errorf("%w: item %s category %s", ErrListCategoryUnknown, item.ID, item.CategoryID)
			}
		}
		seen[item.ID] = struct{}{}
		normalized = append(normalized, cloneListItem(item))
	}
	return normalized, nil
}

func cloneListItem(item ListItem) ListItem {
	item.Aliases = append([]string(nil), item.Aliases...)
	item.OperationIDs = append([]OperationID(nil), item.OperationIDs...)
	item.Preview = cloneListPreview(item.Preview)
	return item
}

func cloneListItems(items []ListItem) []ListItem {
	cloned := make([]ListItem, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, cloneListItem(item))
	}
	return cloned
}

func cloneListPreview(preview ListPreview) ListPreview {
	preview.Fields = append([]ListPreviewField(nil), preview.Fields...)
	return preview
}

func cloneListQuery(query ListQuery) ListQuery {
	query.Terms = append([]string(nil), query.Terms...)
	return query
}

func listCategoryIndex(categories []ListCategory) map[ListCategoryID]ListCategory {
	index := make(map[ListCategoryID]ListCategory, len(categories))
	for _, category := range categories {
		index[category.ID] = category
	}
	return index
}

func (model *ListSurfaceModel) rebuildMatches() {
	if model == nil {
		return
	}
	buckets := make(map[ListCategoryID][]listSurfaceMatch, len(model.categories)+1)
	for sourceIndex, item := range model.items {
		score := listItemScore(item, model.query.Normalized)
		if score == 0 {
			continue
		}
		buckets[item.CategoryID] = append(buckets[item.CategoryID], listSurfaceMatch{
			item:        item,
			score:       score,
			sourceIndex: sourceIndex,
		})
	}

	model.matches = model.matches[:0]
	for _, category := range model.categories {
		ranked := buckets[category.ID]
		sortListMatches(ranked)
		model.matches = append(model.matches, ranked...)
	}
	uncategorized := buckets[""]
	sortListMatches(uncategorized)
	model.matches = append(model.matches, uncategorized...)
}

func listItemScore(item ListItem, query string) int {
	if query == "" {
		return 1
	}
	parts := make([]string, 0, 2+len(item.Aliases))
	parts = append(parts, item.PrimaryText, item.SecondaryText)
	parts = append(parts, item.Aliases...)
	return fuzzy.ScoreTerms(strings.Join(parts, " "), query)
}

func sortListMatches(matches []listSurfaceMatch) {
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].sourceIndex < matches[j].sourceIndex
	})
}

func exportListMatch(match listSurfaceMatch) ListMatch {
	return ListMatch{
		Item: cloneListItem(match.item),
	}
}

func (model *ListSurfaceModel) restoreCursor(oldCursor int, replaceAnchor bool) {
	if len(model.matches) == 0 {
		model.cursor = -1
		model.scrollOffset = 0
		if replaceAnchor {
			model.cursorAnchor = ""
		}
		return
	}
	for index, match := range model.matches {
		if model.cursorAnchor != "" && match.item.ID == model.cursorAnchor {
			model.cursor = index
			model.ensureCursorVisible()
			return
		}
	}

	model.cursor = min(max(oldCursor, 0), len(model.matches)-1)
	if replaceAnchor {
		model.cursorAnchor = model.matches[model.cursor].item.ID
	}
	model.ensureCursorVisible()
}

func (model *ListSurfaceModel) setCursorIndex(index int) bool {
	index = min(max(index, 0), len(model.matches)-1)
	if index == model.cursor {
		return false
	}
	model.cursor = index
	model.cursorAnchor = model.matches[index].item.ID
	model.ensureCursorVisible()
	return true
}

func (model *ListSurfaceModel) buildRows() []ListSurfaceRow {
	if model == nil || len(model.matches) == 0 {
		return []ListSurfaceRow{}
	}
	categoryIndex := listCategoryIndex(model.categories)
	rows := make([]ListSurfaceRow, 0, len(model.matches)+len(model.categories))
	var previousCategory ListCategoryID
	havePrevious := false
	for matchIndex, match := range model.matches {
		if match.item.CategoryID != "" &&
			(!havePrevious || match.item.CategoryID != previousCategory) {
			rows = append(rows, ListSurfaceRow{
				Kind:     ListRowCategory,
				Category: categoryIndex[match.item.CategoryID],
			})
		}
		rows = append(rows, ListSurfaceRow{
			Kind:     ListRowItem,
			Category: categoryIndex[match.item.CategoryID],
			Match:    exportListMatch(match),
			Cursor:   matchIndex == model.cursor,
			Selected: model.IsSelected(match.item.ID),
		})
		previousCategory = match.item.CategoryID
		havePrevious = true
	}
	return rows
}

func (model *ListSurfaceModel) ensureCursorVisible() {
	rows := model.buildRows()
	if model.height <= 0 || len(rows) == 0 || model.cursor < 0 {
		model.scrollOffset = 0
		return
	}
	cursorRow := -1
	for index, row := range rows {
		if row.Kind == ListRowItem && row.Cursor {
			cursorRow = index
			break
		}
	}
	if cursorRow < 0 {
		model.scrollOffset = 0
		return
	}
	if cursorRow < model.scrollOffset {
		model.scrollOffset = cursorRow
	}
	if cursorRow >= model.scrollOffset+model.height {
		model.scrollOffset = cursorRow - model.height + 1
	}
	model.scrollOffset = min(
		max(model.scrollOffset, 0),
		max(len(rows)-model.height, 0),
	)
}

func (model *ListSurfaceModel) pruneSelection() {
	for id := range model.selected {
		item, ok := model.sourceItem(id)
		if !ok || !item.Enabled() {
			delete(model.selected, id)
		}
	}
}

func (model *ListSurfaceModel) sourceContains(id ListItemID) bool {
	if id == "" {
		return false
	}
	_, ok := model.sourceItem(id)
	return ok
}

func (model *ListSurfaceModel) sourceItem(id ListItemID) (ListItem, bool) {
	for _, item := range model.items {
		if item.ID == id {
			return item, true
		}
	}
	return ListItem{}, false
}

func (model *ListSurfaceModel) actionItems() []ListItem {
	if model.config.SelectionMode == ListSelectionMulti && len(model.selected) > 0 {
		return model.SelectedItems()
	}
	item, ok := model.CurrentItem()
	if !ok {
		return nil
	}
	return []ListItem{item}
}

func intersectOperationIDs(left, right []OperationID) []OperationID {
	if len(left) == 0 || len(right) == 0 {
		return []OperationID{}
	}
	rightSet := make(map[OperationID]struct{}, len(right))
	for _, id := range right {
		rightSet[id] = struct{}{}
	}
	intersection := make([]OperationID, 0, min(len(left), len(right)))
	seen := make(map[OperationID]struct{})
	for _, id := range left {
		if _, ok := rightSet[id]; !ok {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		intersection = append(intersection, id)
	}
	return intersection
}
