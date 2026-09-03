package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// ListSurfaceLayout is the concrete layout selected by a renderer.
type ListSurfaceLayout uint8

const (
	// ListSurfaceLayoutAuto uses a preview pane only when the terminal can
	// satisfy both panes' minimum widths.
	ListSurfaceLayoutAuto ListSurfaceLayout = iota
	// ListSurfaceLayoutCompact renders only the list.
	ListSurfaceLayoutCompact
	// ListSurfaceLayoutWide requests a side-by-side list and preview. It still
	// falls back to compact when the minimum-width contract cannot be met.
	ListSurfaceLayoutWide
)

const (
	ListSurfaceDefaultWideMinWidth    = 72
	ListSurfaceDefaultListMinWidth    = 24
	ListSurfaceDefaultPreviewMinWidth = 24
	listSurfacePaneGapWidth           = 3
	// listSurfacePreviewBodyLines caps the wrapped Body lines the preview header
	// may spend. Body is a description, and a long one would otherwise swallow the
	// rows PreviewBody exists to fill.
	listSurfacePreviewBodyLines = 2
)

// ListSurfaceRenderConfig configures presentation only. Query, cursor,
// selection, and scrolling continue to belong to ListSurfaceModel.
type ListSurfaceRenderConfig struct {
	Width           int
	Height          int
	Layout          ListSurfaceLayout
	ShowPreview     bool
	HideHelp        bool
	HelpText        string
	WideMinWidth    int
	ListMinWidth    int
	PreviewMinWidth int
	// PreviewBody renders pane-owned lines below the preview header, at the
	// resolved pane width and within the rows the header left. It returns lines
	// already fitted to width; the renderer appends them verbatim. ListPreview
	// cannot carry them: it is a renderer-neutral value snapshotted into each item
	// long before a pane width exists, and its text is wrapped rather than
	// truncated, which no height budget survives. Compact layout ignores the hook.
	PreviewBody func(width, height int) []string
}

// ListSurfaceRenderer renders a headless ListSurfaceModel into a Bubble Tea
// view. The renderer is intentionally stateless and cheap to construct.
type ListSurfaceRenderer struct {
	Config ListSurfaceRenderConfig
}

// NewListSurfaceRenderer creates a renderer with the supplied presentation
// configuration.
func NewListSurfaceRenderer(config ListSurfaceRenderConfig) ListSurfaceRenderer {
	return ListSurfaceRenderer{Config: config}
}

// Render selects compact or wide presentation, applying the minimum-width
// fallback before mutating the model's viewport.
func (renderer ListSurfaceRenderer) Render(model *ListSurfaceModel) string {
	config := normalizeListSurfaceRenderConfig(renderer.Config, model)
	layout, preview := listSurfaceRenderLayout(model, config)
	if layout == ListSurfaceLayoutWide {
		return renderWideListSurface(model, config, preview)
	}
	return renderCompactListSurface(model, config)
}

// RenderCompactListSurface is the convenience entry point for embedding a
// compact list without constructing a renderer.
func RenderCompactListSurface(model *ListSurfaceModel, width, height int) string {
	return NewListSurfaceRenderer(ListSurfaceRenderConfig{
		Width:    width,
		Height:   height,
		Layout:   ListSurfaceLayoutCompact,
		HideHelp: true,
	}).Render(model)
}

func normalizeListSurfaceRenderConfig(
	config ListSurfaceRenderConfig,
	model *ListSurfaceModel,
) ListSurfaceRenderConfig {
	viewport := ListViewport{}
	if model != nil {
		viewport = model.Viewport()
	}
	if config.Width <= 0 {
		config.Width = viewport.Width
	}
	if config.Height <= 0 {
		config.Height = viewport.Height
	}
	if config.Width <= 0 {
		config.Width = 80
	}
	if config.Height <= 0 {
		config.Height = 10
	}
	config.Width = max(config.Width, 0)
	config.Height = max(config.Height, 0)
	if config.Layout > ListSurfaceLayoutWide {
		config.Layout = ListSurfaceLayoutAuto
	}
	if config.WideMinWidth <= 0 {
		config.WideMinWidth = ListSurfaceDefaultWideMinWidth
	}
	if config.ListMinWidth <= 0 {
		config.ListMinWidth = ListSurfaceDefaultListMinWidth
	}
	if config.PreviewMinWidth <= 0 {
		config.PreviewMinWidth = ListSurfaceDefaultPreviewMinWidth
	}
	return config
}

func listSurfaceRenderLayout(
	model *ListSurfaceModel,
	config ListSurfaceRenderConfig,
) (ListSurfaceLayout, ListPreview) {
	if model == nil || !config.ShowPreview || config.Layout == ListSurfaceLayoutCompact {
		return ListSurfaceLayoutCompact, ListPreview{}
	}
	preview, ok := model.CurrentPreview()
	if !ok {
		return ListSurfaceLayoutCompact, ListPreview{}
	}
	if _, previewWidth := listSurfacePreviewGeometry(config); previewWidth <= 0 {
		return ListSurfaceLayoutCompact, ListPreview{}
	}
	return ListSurfaceLayoutWide, preview
}

// listSurfacePreviewGeometry is the one copy of the two-pane split. A preview
// width of zero means the terminal cannot satisfy both panes' minimums and the
// layout falls back to the list alone, so a caller deciding whether to advertise
// a key that only acts inside the preview asks this rather than re-deriving the
// 55%/gap formula and drifting from what the renderer draws.
func listSurfacePreviewGeometry(config ListSurfaceRenderConfig) (int, int) {
	config = normalizeListSurfaceRenderConfig(config, nil)
	required := max(
		config.WideMinWidth,
		config.ListMinWidth+listSurfacePaneGapWidth+config.PreviewMinWidth,
	)
	if config.Width < required {
		return config.Width, 0
	}
	listWidth := max(config.Width*55/100, config.ListMinWidth)
	previewWidth := config.Width - listWidth - listSurfacePaneGapWidth
	if previewWidth < config.PreviewMinWidth {
		previewWidth = config.PreviewMinWidth
		listWidth = config.Width - previewWidth - listSurfacePaneGapWidth
	}
	return listWidth, previewWidth
}

func renderCompactListSurface(
	model *ListSurfaceModel,
	config ListSurfaceRenderConfig,
) string {
	if model == nil {
		return theme.MutedStyle.Render("List unavailable.")
	}
	showContinuations := sizeListSurfaceViewport(model, config.Width, config.Height)
	content := renderListSurfacePane(model, config.Width, showContinuations)
	if !config.HideHelp {
		content = appendListSurfaceHelp(content, listSurfaceHelp(model, config.HelpText), config.Width)
	}
	return content
}

func renderWideListSurface(
	model *ListSurfaceModel,
	config ListSurfaceRenderConfig,
	preview ListPreview,
) string {
	listWidth, previewWidth := listSurfacePreviewGeometry(config)

	showContinuations := sizeListSurfaceViewport(model, listWidth, config.Height)
	listPane := renderListSurfacePane(model, listWidth, showContinuations)
	previewPane := renderListSurfacePreview(preview, previewWidth, config.Height, config.PreviewBody)
	content := joinListSurfacePanes(listPane, previewPane, listWidth, previewWidth)
	if !config.HideHelp {
		content = appendListSurfaceHelp(content, listSurfaceHelp(model, config.HelpText), config.Width)
	}
	return content
}

// sizeListSurfaceViewport reserves room inside the renderer's height for the
// continuation rows it adds around the model's item rows. Reducing the item
// capacity can reveal a second continuation, so converge from the full height
// until the indicator count is stable. Without this accounting, an upper
// continuation at the bottom of a list made the pane one row too tall and the
// embedding view clipped the cursor row.
func sizeListSurfaceViewport(model *ListSurfaceModel, width, height int) bool {
	model.SetSize(width, height)
	// At one or two rows, keeping the cursor visible is more useful than spending
	// the entire pane on scroll indicators.
	if height < 3 {
		return false
	}
	for {
		viewport := model.Viewport()
		continuationRows := 0
		if viewport.CanScrollUp {
			continuationRows++
		}
		if viewport.CanScrollDown {
			continuationRows++
		}
		itemRows := max(height-continuationRows, 1)
		if itemRows == viewport.Height {
			return true
		}
		model.SetSize(width, itemRows)
	}
}

func renderListSurfacePane(model *ListSurfaceModel, width int, showContinuations bool) string {
	if empty, ok := model.EmptyContext(); ok {
		return renderListSurfaceEmpty(empty, width)
	}

	rows := model.VisibleRows()
	viewport := model.Viewport()
	lines := make([]string, 0, len(rows)+2)
	if showContinuations && viewport.CanScrollUp {
		lines = append(lines, renderListContinuation("↑", viewport.Offset, width))
	}
	for _, row := range rows {
		lines = append(lines, renderListSurfaceRow(row, model.Config().SelectionMode, width))
	}
	if showContinuations && viewport.CanScrollDown {
		remaining := max(viewport.TotalRows-viewport.Offset-len(rows), 0)
		lines = append(lines, renderListContinuation("↓", remaining, width))
	}
	return strings.Join(lines, "\n")
}

func renderListSurfaceEmpty(empty ListEmptyContext, width int) string {
	title := strings.TrimSpace(empty.State.Title)
	detail := strings.TrimSpace(empty.State.Detail)
	if title == "" {
		if empty.Kind == ListEmptyNoMatches {
			title = "No matching items."
		} else {
			title = "No items."
		}
	}
	lines := []string{theme.MutedStyle.Render(textfit.TruncateToVisualWidth(title, width))}
	if detail != "" {
		for _, line := range textfit.WrapText(detail, max(width, 1)) {
			lines = append(lines, theme.MutedStyle.Render(textfit.TruncateToVisualWidth(line, width)))
		}
	}
	return strings.Join(lines, "\n")
}

func renderListSurfaceRow(
	row ListSurfaceRow,
	selectionMode ListSelectionMode,
	width int,
) string {
	if row.Kind == ListRowCategory {
		label := strings.ToUpper(row.Category.Label)
		return theme.SectionHeaderStyle.Render(textfit.TruncateToVisualWidth(label, width))
	}

	item := row.Match.Item
	cursor := "  "
	if row.Cursor {
		cursor = "› "
	}
	marker := cursor
	if selectionMode == ListSelectionMulti {
		checkbox := "[ ] "
		if item.Indeterminate {
			checkbox = "[-] "
		} else if row.Selected {
			checkbox = "[x] "
		}
		marker += checkbox
	}
	// PrimaryMarker is charged to markerWidth rather than to the primary text, so
	// LayoutListText budgets around chrome it never sees and the marker survives
	// the truncation the text takes.
	markerWidth := lipgloss.Width(marker) + lipgloss.Width(item.PrimaryMarker)
	secondary := item.SecondaryText
	if !item.Enabled() {
		disabled := "disabled"
		if item.DisabledReason != "" {
			disabled += ": " + item.DisabledReason
		}
		if secondary == "" {
			secondary = disabled
		} else {
			secondary += " · " + disabled
		}
	}
	layout := LayoutListText(item.PrimaryText, secondary, max(width-markerWidth, 0))
	// A checked row keeps its color because "applied" is the state worth
	// previewing; the [x] still says it is applied. The cursor row does not,
	// because SelectedLine owns the whole line on every list surface, and a
	// disabled row must not advertise a color it cannot deliver. The Outliner
	// cursor row is the deliberate exception: it has structural margins to paint
	// instead, which a flat list row does not.
	if item.PrimaryStyle != nil && item.Enabled() && !row.Cursor {
		chrome := func(text string) string { return text }
		if row.Selected {
			chrome = func(text string) string { return theme.SelectionStyle.Render(text) }
		}
		styled := chrome(marker) + item.PrimaryMarker + item.PrimaryStyle.Render(layout.Primary)
		if layout.Secondary != "" {
			styled += chrome(layout.Gap + layout.Secondary)
		}
		return textfit.TruncateToVisualWidth(styled, width)
	}
	line := marker + item.PrimaryMarker + layout.Text()
	if !item.Enabled() {
		line = theme.MutedStyle.Render(line)
	}
	if row.Selected && !row.Cursor {
		line = theme.SelectionStyle.Render(line)
	}
	if row.Cursor {
		line = SelectedLine(line, width)
	}
	return textfit.TruncateToVisualWidth(line, width)
}

func renderListContinuation(direction string, count, width int) string {
	label := fmt.Sprintf("%s %d more", direction, count)
	return theme.MutedStyle.Render(textfit.TruncateToVisualWidth(label, width))
}

// listPreviewScrollWindow places the visible slice inside height rows, reserving
// a row for each continuation it ends up needing. Reserving shrinks the
// capacity, which can reveal a continuation that was not needed before, so it
// converges instead of subtracting once — the same accounting
// sizeListSurfaceViewport does, for the same reason. It sits beside
// renderListContinuation because every pane that draws those rows must budget
// for them identically; the Project Hub task pane and the Notes body pane are
// both callers.
func listPreviewScrollWindow(total, offset, height int) (start, count int, above, below bool) {
	if total <= 0 || height <= 0 {
		return 0, 0, false, false
	}
	capacity := height
	for {
		start = max(0, min(offset, total-capacity))
		above = start > 0
		below = start+capacity < total
		reserved := 0
		if above {
			reserved++
		}
		if below {
			reserved++
		}
		next := max(height-reserved, 1)
		if next == capacity {
			return start, min(capacity, total-start), above, below
		}
		capacity = next
	}
}

func renderListSurfacePreview(
	preview ListPreview,
	width, height int,
	body func(width, height int) []string,
) string {
	lines := make([]string, 0, height)
	appendLine := func(line string) {
		if height > 0 && len(lines) >= height {
			return
		}
		lines = append(lines, textfit.TruncateToVisualWidth(line, width))
	}
	appendWrapped := func(text string, style lipgloss.Style, limit int) {
		if strings.TrimSpace(text) == "" {
			return
		}
		wrapped := textfit.WrapText(text, max(width, 1))
		if limit > 0 && len(wrapped) > limit {
			wrapped = wrapped[:limit]
		}
		for _, line := range wrapped {
			appendLine(style.Render(line))
		}
	}

	if preview.Title != "" {
		appendWrapped(preview.Title, theme.ListPreviewHeaderStyle, 0)
	}
	if preview.Summary != "" {
		appendWrapped(preview.Summary, theme.MutedStyle, 0)
	}
	for _, field := range preview.Fields {
		label := strings.TrimSpace(field.Label)
		value := strings.TrimSpace(field.Value)
		switch {
		case label != "" && value != "":
			appendWrapped(label+": "+value, lipgloss.NewStyle(), 0)
		case label != "":
			appendWrapped(label, lipgloss.NewStyle(), 0)
		case value != "":
			appendWrapped(value, lipgloss.NewStyle(), 0)
		}
	}
	if preview.Body != "" {
		if len(lines) > 0 {
			appendLine("")
		}
		appendWrapped(preview.Body, lipgloss.NewStyle(), listSurfacePreviewBodyLines)
	}
	if body != nil && height > 0 {
		// The rule costs a row of its own, so draw it only when a pane row would
		// still follow it.
		if capacity := height - len(lines) - 1; capacity > 0 {
			if rows := body(width, capacity); len(rows) > 0 {
				appendLine(theme.MutedStyle.Render(strings.Repeat("─", max(width, 1))))
				for _, row := range rows {
					appendLine(row)
				}
			}
		}
	}
	if len(lines) == 0 {
		appendLine(theme.MutedStyle.Render("No preview available."))
	}
	if height > 0 && len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func joinListSurfacePanes(left, right string, leftWidth, rightWidth int) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	lineCount := max(len(leftLines), len(rightLines))
	lines := make([]string, 0, lineCount)
	for index := range lineCount {
		leftLine := ""
		if index < len(leftLines) {
			leftLine = textfit.TruncateToVisualWidth(leftLines[index], leftWidth)
		}
		rightLine := ""
		if index < len(rightLines) {
			rightLine = textfit.TruncateToVisualWidth(rightLines[index], rightWidth)
		}
		leftLine = lipgloss.NewStyle().Width(leftWidth).Render(leftLine)
		lines = append(lines, leftLine+" │ "+rightLine)
	}
	return strings.Join(lines, "\n")
}

func listSurfaceHelp(model *ListSurfaceModel, override string) string {
	if override != "" {
		return override
	}
	if model != nil && model.Config().SelectionMode == ListSelectionMulti {
		return "j/k navigate · space toggle · enter apply · / search · esc cancel"
	}
	return "j/k navigate · enter select · / search · esc cancel"
}

func appendListSurfaceHelp(content, help string, width int) string {
	helpLines := textfit.WrapText(help, max(width, 1))
	for index := range helpLines {
		helpLines[index] = theme.HelpStyle.Render(helpLines[index])
	}
	if content == "" {
		return strings.Join(helpLines, "\n")
	}
	return content + "\n\n" + strings.Join(helpLines, "\n")
}
