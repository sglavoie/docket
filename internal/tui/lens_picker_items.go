package tui

import (
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
)

// items renders both action rows plus every saved lens the query admits.
// Filtering happens here rather than on the surface so the two action rows are
// exempt from it and the saved rows keep their alphabetical order instead of a
// relevance order.
//
// Ordinals are assigned here too, in this one pass, and therefore follow the
// filtered visible list: the digit is printed on the row, so numbering the
// unfiltered population would put digits on screen that apply lenses nobody can
// see. Unfiltered numbering stays one Esc away.
func (model *NamedLensPickerModel) items() []ListItem {
	actions := lensPickerActionRows(model.snapshot)
	items := make([]ListItem, 0, len(model.sorted)+len(actions))
	items = append(items, actions...)
	listed := 0
	model.ordinals = model.ordinals[:0]
	for _, lens := range model.sorted {
		if !namedLensMatchesQuery(lens, model.query) {
			continue
		}
		listed++
		ordinal := 0
		if len(model.ordinals) < lensPickerMaxOrdinal {
			model.ordinals = append(model.ordinals, lens.ID)
			ordinal = len(model.ordinals)
		}
		items = append(items, namedLensPickerItem(lens, model.snapshot, ordinal))
	}
	model.listed = listed
	return items
}

// lensPickerOrdinalMarker draws the quick-apply digit, or the blank slot of the
// same width that says this row has none. It leads PrimaryMarker on every row,
// so the number column stays put behind an action row, a tenth saved lens, and
// either spelling of the active-lens dot alike.
func lensPickerOrdinalMarker(ordinal int) string {
	if ordinal < 1 || ordinal > lensPickerMaxOrdinal {
		return strings.Repeat(" ", lensPickerOrdinalMarkerWidth)
	}
	return MutedStyle.Render(strconv.Itoa(ordinal)) + " "
}

// lensPickerActionRows is the query-exempt head of the list, in display order.
// Both rows carry no Data, which is what CurrentLens reads to tell an action row
// from a saved one; their identity travels as the stable ListItemID instead.
// Both carry the blank ordinal slot: it shares the one text column with the
// numbered rows below, and the empty slot is the statement that an act nobody
// repeats is not worth a digit.
func lensPickerActionRows(snapshot NamedLensPickerSnapshot) []ListItem {
	return []ListItem{
		{
			ID:            lensPickerCreateItemID,
			PrimaryText:   lensPickerCreateLabel,
			PrimaryMarker: lensPickerOrdinalMarker(0),
			Aliases:       []string{"save", "new lens", "create lens"},
			SecondaryText: snapshot.CurrentSummary,
			OperationIDs:  []OperationID{operationLensCreate},
		},
		{
			ID:            lensPickerAddTemplateItemID,
			PrimaryText:   lensPickerAddTemplateLabel,
			PrimaryMarker: lensPickerOrdinalMarker(0),
			// The aliases are the words someone reaches for before they know what
			// this product calls a lens, so the row answers to all three.
			Aliases:       []string{"template", "preset", "built-in view"},
			SecondaryText: lensPickerAddTemplateSecondary,
			OperationIDs:  []OperationID{operationLensAddTemplate},
		},
	}
}

func namedLensPickerItem(
	lens config.NamedLens,
	snapshot NamedLensPickerSnapshot,
	ordinal int,
) ListItem {
	marker := lensPickerOrdinalMarker(ordinal)
	if lens.ID == snapshot.ActiveID {
		label := "●"
		if snapshot.Modified {
			label = "●*"
		}
		marker += MutedStyle.Render(label) + " "
	}
	item := ListItem{
		ID:            lensPickerItemID(lens.ID),
		PrimaryText:   lens.Name,
		PrimaryMarker: marker,
		SecondaryText: namedLensSummary(lens),
		OperationIDs:  append([]OperationID{operationLensApply}, namedLensActionSheetOperations...),
		Data:          lens,
	}
	// The accent is read from the definition every time a row is built, which is
	// why it belongs here rather than beside any one caller: initial
	// construction, a query change, and Refresh after rename, replace, create, or
	// delete all rebuild through items(). A neutral definition sets no style at
	// all, leaving the row the list's ordinary text.
	if style, ok := namedLensPrimaryStyle(lens); ok {
		item.PrimaryStyle = &style
	}
	return item
}

func lensPickerItemID(id string) ListItemID {
	return ListItemID(lensPickerLensPrefix + id)
}

// namedLensMatchesQuery scores a saved lens against the picker's own query. The
// summary participates so a search for "waiting" finds a lens whose facets say
// waiting even when its name does not.
func namedLensMatchesQuery(lens config.NamedLens, query string) bool {
	normalized := strings.Join(strings.Fields(query), " ")
	if normalized == "" {
		return true
	}
	return fuzzy.ScoreTerms(lens.Name+" "+namedLensSummary(lens), normalized) != 0
}
