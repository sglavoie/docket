package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// namedLensPickerInnerWidth is the overlay box width the picker claims out of
// the terminal.
func namedLensPickerInnerWidth(width int) int {
	return min(max(width*4/5, 46), max(width-6, 24))
}

func (model *NamedLensPickerModel) render(width int) string {
	return model.renderWithin(width, 0)
}

// renderWithin budgets the list after reserving rows for the modal chrome, so a
// minimum-supported terminal drops list rows rather than losing the search line,
// the state line, or the footer.
func (model *NamedLensPickerModel) renderWithin(width, height int) string {
	if model == nil || model.surface == nil {
		return ""
	}
	innerWidth := namedLensPickerInnerWidth(width)
	contentWidth := max(innerWidth-4, 12)

	header := make([]string, 0, 2)
	if model.notice != "" {
		header = append(header, theme.ErrorStatusStyle.Render(
			textfit.TruncateToVisualWidth(model.notice, contentWidth),
		))
	}
	if model.searching || model.query != "" {
		cursor := ""
		if model.searching {
			cursor = "▏"
		}
		header = append(header, theme.MutedStyle.Render("Search: "+model.query+cursor))
	}
	if state := model.stateLine(); state != "" {
		header = append(header, theme.MutedStyle.Render(textfit.TruncateToVisualWidth(state, contentWidth)))
	}

	listHeight := min(max(len(model.surface.Rows()), 4), 14)
	if height > 0 {
		// Halo, frame, title, the header block's separating row, and the footer
		// consume nine rows before any header line of its own.
		reserved := 9 + len(header)
		listHeight = min(listHeight, max(height-reserved, 1))
	}
	content := RenderCompactListSurface(model.surface, contentWidth, listHeight)
	if len(header) > 0 {
		content = strings.Join(header, "\n") + "\n\n" + content
	}

	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render("Named Lenses")
	guidance := model.guidanceSpec()
	return renderOverlayBox(
		content,
		innerWidth,
		title,
		renderGuidanceText(guidance, max(innerWidth-4, 1)),
		lipgloss.NewStyle().Foreground(theme.ColorPrimary),
	)
}

// stateLine names an empty population or a query that matched none of it. Both
// action rows are always listed, so the surface itself is never empty and cannot
// carry this copy on the list's behalf.
func (model *NamedLensPickerModel) stateLine() string {
	switch {
	case len(model.sorted) == 0:
		return "No saved lenses yet — save this view or add a built-in."
	case model.listed == 0:
		return "No matching lenses — Esc clears the search."
	default:
		return ""
	}
}

// guidanceSpec returns the footer spec alone: this picker always draws a footer,
// so there is no "no guidance" case for a caller to branch on.
func (model *NamedLensPickerModel) guidanceSpec() GuidanceSpec {
	if model == nil || model.surface == nil {
		return GuidanceSpec{}
	}
	searching := model.searching
	onLens := func() bool {
		_, ok := model.CurrentLens()
		return ok
	}
	// Browse-mode Enter is row-specific: the two action rows do different things
	// and the footer would lie if it named only one of them.
	enterLabel := "apply"
	switch {
	case searching:
		enterLabel = "apply search"
	case model.cursorIs(lensPickerCreateItemID):
		enterLabel = lensPickerCreateEnterLabel
	case model.cursorIs(lensPickerAddTemplateItemID):
		enterLabel = lensPickerAddTemplateEnterLabel
	}
	escapeLabel := "close"
	if searching {
		escapeLabel = "cancel search"
		if model.query != "" {
			escapeLabel = "clear search"
		}
	}
	km := &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("j", "Move selection", "Navigation",
			keymap.WithGate(func() bool { return !searching }),
			keymap.WithFooter(10, "navigate", false), keymap.WithFooterKey("j/k or ↑/↓")),
		keymap.Bind("enter", "Apply the current row", "Actions",
			keymap.WithFooter(1, enterLabel, true), keymap.WithFooterKey("Enter")),
		keymap.Bind("1", "Apply the numbered lens", "Actions",
			keymap.WithGate(func() bool { return !searching && model.listed > 0 }),
			keymap.WithFooter(4, "apply by number", false), keymap.WithFooterKey("1-9")),
		keymap.Bind("space", "Open lens actions", "Actions",
			keymap.WithGate(func() bool { return !searching && onLens() }),
			keymap.WithFooter(3, "manage", false), keymap.WithFooterKey("Space")),
		keymap.Bind("/", "Search lenses", "Filters",
			keymap.WithGate(func() bool { return !searching }),
			keymap.WithFooter(20, "search", false)),
		keymap.Bind("esc", "Unwind the lens picker", "Navigation",
			keymap.WithFooter(2, escapeLabel, true), keymap.WithFooterKey("Esc")),
	}}
	spec, _ := listSurfaceGuidanceSpec(model.surface.GuidanceState(searching), km, nil)
	return spec
}

// namedLensSummary renders one saved definition as a single compact line: the
// mode it opens, the canonical clauses it narrows with, whatever prose the
// canonical vocabulary cannot spell, and the manual sort it restores. An axis
// left at its default contributes nothing, because a summary that names every
// axis names nothing.
func namedLensSummary(lens config.NamedLens) string {
	parts := make([]string, 0, 12)
	parts = append(parts, namedLensModeLabel(lens.Mode))
	facets, remainder := models.PartitionQueryString(lens.Query)
	// The clauses are rendered by the shared chip renderer, so a saved lens and
	// the live filter chips spell the same narrowing with the same characters.
	parts = append(parts, filterClauseParts(facets.Clauses)...)
	if prose := strings.TrimSpace(remainder); prose != "" {
		parts = append(parts, "/"+prose)
	}
	if sorted := namedLensSortLabel(lens.Sort); sorted != "" {
		parts = append(parts, sorted)
	}
	return strings.Join(parts, " · ")
}

// namedLensModeLabel prefers the catalog label and degrades to the stored token,
// so a machine file written by a newer build still names something.
func namedLensModeLabel(token string) string {
	id, err := lensModeAxis.value(token)
	if err != nil {
		return token
	}
	if mode, ok := OutlinerModeByID(id); ok {
		return mode.Label
	}
	return token
}

func namedLensSortLabel(sorted config.LensSort) string {
	field, err := lensSortAxis.value(sorted.Field)
	if err != nil {
		return "sort: " + sorted.Field
	}
	if field == SortByDefault {
		return ""
	}
	direction := "↓"
	if sorted.Ascending {
		direction = "↑"
	}
	return "sort: " + field.String() + " " + direction
}
