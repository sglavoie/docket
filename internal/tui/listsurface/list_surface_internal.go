package listsurface

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/textfit"
)

// ListTextGap is the one separator LayoutListText puts between primary and
// secondary text. A caller that measures a row before handing it here must
// charge this same width, so it is named once rather than spelled per site.
const ListTextGap = "  "

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
		layout.Primary = textfit.TruncateToVisualWidth(primary, width)
		return layout
	}

	const gap = ListTextGap
	gapWidth := lipgloss.Width(gap)
	if primaryWidth+gapWidth+secondaryWidth <= width {
		layout.Primary = primary
		layout.Secondary = secondary
		layout.Gap = gap
		return layout
	}
	if width <= gapWidth+1 {
		layout.Primary = textfit.TruncateToVisualWidth(primary, width)
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

	layout.Primary = textfit.TruncateToVisualWidth(primary, primaryBudget)
	layout.Secondary = textfit.TruncateToVisualWidth(secondary, secondaryBudget)
	layout.Gap = gap
	return layout
}

func validListSelectionMode(mode ListSelectionMode) bool {
	return mode == ListSelectionSingle || mode.Many()
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
		item.GroupID = ListCategoryID(strings.TrimSpace(string(item.GroupID)))
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
		// A group is drawn as a section, so it is declared like one: a promotion
		// naming no category would head a block with a blank label.
		if item.GroupID != "" {
			if _, ok := categories[item.GroupID]; !ok {
				return nil, fmt.Errorf("%w: item %s group %s", ErrListCategoryUnknown, item.ID, item.GroupID)
			}
		}
		seen[item.ID] = struct{}{}
		normalized = append(normalized, cloneListItem(item))
	}
	return normalized, nil
}

func cloneListItem(item ListItem) ListItem {
	item.PrimaryMatchPositions = append([]int(nil), item.PrimaryMatchPositions...)
	item.SecondaryMatchPositions = append([]int(nil), item.SecondaryMatchPositions...)
	item.Aliases = append([]string(nil), item.Aliases...)
	item.OperationIDs = append([]operation.OperationID(nil), item.OperationIDs...)
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
		group := item.GroupOrCategory()
		buckets[group] = append(buckets[group], listSurfaceMatch{
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
		group := match.item.GroupOrCategory()
		if group != "" && (!havePrevious || group != previousCategory) {
			rows = append(rows, ListSurfaceRow{
				Kind:     ListRowCategory,
				Category: categoryIndex[group],
			})
		}
		rows = append(rows, ListSurfaceRow{
			Kind:     ListRowItem,
			Category: categoryIndex[group],
			Match:    exportListMatch(match),
			Cursor:   matchIndex == model.cursor,
			Selected: model.IsSelected(match.item.ID),
		})
		previousCategory = group
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
	if model.config.SelectionMode.Many() && len(model.selected) > 0 {
		return model.SelectedItems()
	}
	item, ok := model.CurrentItem()
	if !ok {
		return nil
	}
	return []ListItem{item}
}

func intersectOperationIDs(left, right []operation.OperationID) []operation.OperationID {
	if len(left) == 0 || len(right) == 0 {
		return []operation.OperationID{}
	}
	rightSet := make(map[operation.OperationID]struct{}, len(right))
	for _, id := range right {
		rightSet[id] = struct{}{}
	}
	intersection := make([]operation.OperationID, 0, min(len(left), len(right)))
	seen := make(map[operation.OperationID]struct{})
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
