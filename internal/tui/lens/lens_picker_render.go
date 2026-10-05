package lens

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// NamedLensPickerInnerWidth is the overlay box width the picker claims out of
// the terminal.
func NamedLensPickerInnerWidth(width int) int {
	return min(max(width*4/5, 46), max(width-6, 24))
}

func (model *NamedLensPickerModel) Render(width int) string {
	return model.RenderWithin(width, 0)
}

// RenderWithin budgets the list after reserving rows for the modal chrome, so a
// minimum-supported terminal drops list rows rather than losing the search line,
// the state line, or the footer.
func (model *NamedLensPickerModel) RenderWithin(width, height int) string {
	if model == nil || model.Surface == nil {
		return ""
	}
	innerWidth := NamedLensPickerInnerWidth(width)
	contentWidth := max(innerWidth-4, 12)

	header := make([]string, 0, 2)
	if model.Searching || model.Query != "" {
		header = append(header, model.searchInput.View(model.Query, model.Searching, contentWidth))
	}
	if state := model.StateLine(); state != "" {
		header = append(header, theme.MutedStyle.Render(textfit.TruncateToVisualWidth(state, contentWidth)))
	}

	guidance := chrome.RenderGuidanceText(model.GuidanceSpec(), contentWidth)
	if model.notice != "" {
		lines := textfit.WrapText(model.notice, contentWidth)
		if height > 0 {
			// Reserve the chrome, other header lines, and one list row before
			// giving a refusal the remaining space. Only an oversized notice
			// is shortened; ordinary refusals wrap in full.
			budget := max(height-8-lipgloss.Height(guidance)-len(header)-1, 1)
			if len(lines) > budget {
				lines = lines[:budget]
				lines[budget-1] = textfit.TruncateToVisualWidth(lines[budget-1]+"…", contentWidth)
			}
		}
		header = append([]string{theme.ErrorStatusStyle.Render(strings.Join(lines, "\n"))}, header...)
	}
	headerText := strings.Join(header, "\n")
	listHeight := min(max(len(model.Surface.Rows()), 4), 14)
	bodyHeight := listHeight + 6
	if height > 0 {
		// Halo, frame, title, the header block's separating row, and the footer
		// consume eight rows plus the measured footer and header lines.
		headerHeight := 0
		if headerText != "" {
			headerHeight = lipgloss.Height(headerText)
		}
		reserved := 8 + lipgloss.Height(guidance) + headerHeight
		bodyHeight = max(height-reserved, 1)
	}
	details := model.renderDetails(contentWidth, max(bodyHeight-3, 0))
	if details != "" {
		bodyHeight -= lipgloss.Height(details)
	}
	listHeight = min(listHeight, bodyHeight)
	content := listsurface.RenderCompactListSurface(model.Surface, contentWidth, listHeight)
	if details != "" {
		content += "\n" + details
	}
	if len(header) > 0 {
		content = headerText + "\n\n" + content
	}

	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render("Named Lenses")
	return chrome.RenderOverlayBox(
		content,
		innerWidth,
		title,
		guidance,
		lipgloss.NewStyle().Foreground(theme.ColorPrimary),
	)
}

// renderDetails borrows rows from the list while keeping three rows for choosing
// a lens. The shared preview renderer wraps the complete definition; paging
// windows those lines instead of shortening the saved query.
func (model *NamedLensPickerModel) renderDetails(width, available int) string {
	model.detailPageRows, model.detailMaxOffset = 0, 0
	selected, ok := model.CurrentLens()
	if !ok || available < 3 {
		return ""
	}
	document := listsurface.RenderListSurfacePreview(listsurface.ListPreview{
		Summary: NamedLensSummary(selected, model.snapshot.ProjectSlugs),
	}, width, 0, nil)
	lines := strings.Split(document, "\n")
	capacity := min(4, available-2)
	model.detailPageRows = capacity
	model.detailMaxOffset = max(len(lines)-capacity, 0)
	start := min(model.detailOffset, model.detailMaxOffset)
	end := min(start+capacity, len(lines))
	heading := "Details"
	if model.detailMaxOffset > 0 {
		heading += " · Ctrl+F/B page"
	}
	result := theme.MutedStyle.Render(heading) + "\n" + strings.Join(lines[start:end], "\n")
	if model.detailMaxOffset > 0 {
		result += "\n" + theme.MutedStyle.Render(fmt.Sprintf("%d above · %d below", start, len(lines)-end))
	}
	return result
}

// StateLine names an empty population or a query that matched none of it. Both
// action rows are always listed, so the surface itself is never empty and cannot
// carry this copy on the list's behalf.
func (model *NamedLensPickerModel) StateLine() string {
	switch {
	case len(model.sorted) == 0:
		return "No saved lenses yet — save this view or add a built-in."
	case model.Listed == 0:
		if model.Searching {
			return "No matching lenses — Esc clears the search."
		}
		return "No matching lenses — / edits the search."
	default:
		return ""
	}
}

// GuidanceSpec returns the footer spec alone: this picker always draws a footer,
// so there is no "no guidance" case for a caller to branch on.
func (model *NamedLensPickerModel) GuidanceSpec() chrome.GuidanceSpec {
	if model == nil || model.Surface == nil {
		return chrome.GuidanceSpec{}
	}
	searching := model.Searching
	navigationKeys := "j/k or ↑/↓"
	if searching {
		navigationKeys = "↑/↓"
	}
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
		enterLabel = LensPickerCreateEnterLabel
	case model.cursorIs(LensPickerAddTemplateItemID):
		enterLabel = LensPickerAddTemplateEnterLabel
	}
	escapeLabel := "close"
	if searching {
		escapeLabel = "cancel search"
		if model.Query != "" {
			escapeLabel = "clear search"
		}
	}
	km := &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("up", "Move selection", "Navigation",
			keymap.WithFooter(10, "navigate", false), keymap.WithFooterKey(navigationKeys)),
		keymap.Bind("enter", "Apply the current row", "Actions",
			keymap.WithFooter(1, enterLabel, true), keymap.WithFooterKey("Enter")),
		keymap.Bind("1", "Apply the numbered lens", "Actions",
			keymap.WithGate(func() bool { return !searching && model.Listed > 0 }),
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
	spec, _ := listsurface.ListSurfaceGuidanceSpec(model.Surface.GuidanceState(searching), km, nil)
	return spec
}

// NamedLensSummary renders one saved definition as a single compact line: the
// mode and project it opens, the canonical clauses it narrows with, whatever prose the
// canonical vocabulary cannot spell, and the manual sort it restores. An axis
// left at its default contributes nothing, because a summary that names every
// axis names nothing.
func NamedLensSummary(lens config.NamedLens, projectSlugs map[string]string) string {
	parts := make([]string, 0, 12)
	parts = append(parts, namedLensModeLabel(lens.Mode))
	if lens.Scope != "" {
		slug := projectSlugs[lens.Scope]
		if slug == "" {
			slug = lens.Scope + " (unavailable)"
		}
		parts = append(parts, "Project: "+slug)
	}
	facets, remainder := models.PartitionQueryString(lens.Query)
	// The clauses are rendered by the shared chip renderer, so a saved lens and
	// the live filter chips spell the same narrowing with the same characters.
	parts = append(parts, query.FilterClauseParts(facets.Clauses)...)
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
	id, err := ModeAxis.Value(token)
	if err != nil {
		return token
	}
	if mode, ok := chrome.OutlinerModeByID(id); ok {
		return mode.Label
	}
	return token
}

func namedLensSortLabel(sorted config.LensSort) string {
	field, err := SortAxis.Value(sorted.Field)
	if err != nil {
		return "sort: " + sorted.Field
	}
	if field == query.SortByDefault {
		return ""
	}
	direction := "↓"
	if sorted.Ascending {
		direction = "↑"
	}
	return "sort: " + field.String() + " " + direction
}
