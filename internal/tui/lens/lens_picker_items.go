package lens

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/theme"
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
func (model *NamedLensPickerModel) items() []listsurface.ListItem {
	actions := lensPickerActionRows(model.snapshot)
	items := make([]listsurface.ListItem, 0, len(model.sorted)+len(actions))
	items = append(items, actions...)
	listed := 0
	model.Ordinals = model.Ordinals[:0]
	for _, lens := range model.sorted {
		item := namedLensPickerItem(lens, model.snapshot)
		primary, secondary, matches := namedLensMatchPositions(item.PrimaryText, item.SecondaryText, model.Query)
		if !matches {
			continue
		}
		listed++
		ordinal := 0
		if len(model.Ordinals) < LensPickerMaxOrdinal {
			model.Ordinals = append(model.Ordinals, lens.ID)
			ordinal = len(model.Ordinals)
		}
		item.PrimaryMarker = lensPickerOrdinalMarker(ordinal) + item.PrimaryMarker
		item.PrimaryMatchPositions, item.SecondaryMatchPositions = primary, secondary
		items = append(items, item)
	}
	model.Listed = listed
	return items
}

// lensPickerOrdinalMarker draws the quick-apply digit, or the blank slot of the
// same width that says this row has none. It leads PrimaryMarker on every row,
// so the number column stays put behind an action row, a tenth saved lens, and
// either spelling of the active-lens dot alike.
func lensPickerOrdinalMarker(ordinal int) string {
	if ordinal < 1 || ordinal > LensPickerMaxOrdinal {
		return strings.Repeat(" ", LensPickerOrdinalMarkerWidth)
	}
	return theme.MutedStyle.Render(strconv.Itoa(ordinal)) + " "
}

// lensPickerActionRows is the query-exempt head of the list, in display order.
// Both rows carry no Data, which is what CurrentLens reads to tell an action row
// from a saved one; their identity travels as the stable ListItemID instead.
// Both carry the blank ordinal slot: it shares the one text column with the
// numbered rows below, and the empty slot is the statement that an act nobody
// repeats is not worth a digit.
func lensPickerActionRows(snapshot NamedLensPickerSnapshot) []listsurface.ListItem {
	return []listsurface.ListItem{
		{
			ID:            lensPickerCreateItemID,
			PrimaryText:   LensPickerCreateLabel,
			PrimaryMarker: lensPickerOrdinalMarker(0),
			Aliases:       []string{"save", "new lens", "create lens"},
			SecondaryText: snapshot.CurrentSummary,
			OperationIDs:  []operation.OperationID{OperationLensCreate},
		},
		{
			ID:            LensPickerAddTemplateItemID,
			PrimaryText:   LensPickerAddTemplateLabel,
			PrimaryMarker: lensPickerOrdinalMarker(0),
			// The aliases are the words someone reaches for before they know what
			// this product calls a lens, so the row answers to all three.
			Aliases:       []string{"template", "preset", "built-in view"},
			SecondaryText: LensPickerAddTemplateSecondary,
			OperationIDs:  []operation.OperationID{OperationLensAddTemplate},
		},
	}
}

func namedLensPickerItem(
	lens config.NamedLens,
	snapshot NamedLensPickerSnapshot,
) listsurface.ListItem {
	marker := ""
	if lens.ID == snapshot.ActiveID {
		label := "●"
		if snapshot.Modified {
			label = "●*"
		}
		marker += theme.MutedStyle.Render(label) + " "
	}
	item := listsurface.ListItem{
		ID:            lensPickerItemID(lens.ID),
		PrimaryText:   lens.Name,
		PrimaryMarker: marker,
		SecondaryText: NamedLensSummary(lens, snapshot.ProjectSlugs),
		OperationIDs:  append([]operation.OperationID{OperationLensApply}, NamedLensActionSheetOperations...),
		Data:          lens,
	}
	// The accent is read from the definition every time a row is built, which is
	// why it belongs here rather than beside any one caller: initial
	// construction, a query change, and Refresh after rename, replace, create, or
	// delete all rebuild through items(). A neutral definition sets no style at
	// all, leaving the row the list's ordinary text.
	if style, ok := NamedLensPrimaryStyle(lens); ok {
		item.PrimaryStyle = &style
	}
	return item
}

func lensPickerItemID(id string) listsurface.ListItemID {
	return listsurface.ListItemID(lensPickerLensPrefix + id)
}

// Filter and highlights share the exact combined string, so terms may span
// name and summary without either field inventing a second match.
func namedLensMatchPositions(name, summary, query string) (primary, secondary []int, matches bool) {
	score, positions := fuzzy.NewMatcher(false).ScoreTermsWithPositions(name+" "+summary, query)
	split := utf8.RuneCountInString(name)
	for _, position := range positions {
		switch {
		case position < split:
			primary = append(primary, position)
		case position > split:
			secondary = append(secondary, position-split-1)
		}
	}
	return primary, secondary, score > 0
}
