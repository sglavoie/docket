package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
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
	required := max(
		config.WideMinWidth,
		config.ListMinWidth+listSurfacePaneGapWidth+config.PreviewMinWidth,
	)
	if config.Width < required {
		return ListSurfaceLayoutCompact, ListPreview{}
	}
	return ListSurfaceLayoutWide, preview
}

func renderCompactListSurface(
	model *ListSurfaceModel,
	config ListSurfaceRenderConfig,
) string {
	if model == nil {
		return MutedStyle.Render("List unavailable.")
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
	listWidth := max(config.Width*55/100, config.ListMinWidth)
	previewWidth := config.Width - listWidth - listSurfacePaneGapWidth
	if previewWidth < config.PreviewMinWidth {
		previewWidth = config.PreviewMinWidth
		listWidth = config.Width - previewWidth - listSurfacePaneGapWidth
	}

	showContinuations := sizeListSurfaceViewport(model, listWidth, config.Height)
	listPane := renderListSurfacePane(model, listWidth, showContinuations)
	previewPane := renderListSurfacePreview(preview, previewWidth, config.Height)
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
	lines := []string{MutedStyle.Render(truncateToVisualWidth(title, width))}
	if detail != "" {
		for _, line := range wrapText(detail, max(width, 1)) {
			lines = append(lines, MutedStyle.Render(truncateToVisualWidth(line, width)))
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
		return SectionHeaderStyle.Render(truncateToVisualWidth(label, width))
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
	markerWidth := lipgloss.Width(marker)
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
	// because SelectedLine owns the whole line there exactly as the Outliner
	// cursor row does, and a disabled row must not advertise a color it cannot
	// deliver.
	if item.PrimaryStyle != nil && item.Enabled() && !row.Cursor {
		chrome := func(text string) string { return text }
		if row.Selected {
			chrome = func(text string) string { return SelectionStyle.Render(text) }
		}
		styled := chrome(marker) + item.PrimaryStyle.Render(layout.Primary)
		if layout.Secondary != "" {
			styled += chrome(layout.Gap + layout.Secondary)
		}
		return truncateToVisualWidth(styled, width)
	}
	line := marker + layout.Text()
	if !item.Enabled() {
		line = MutedStyle.Render(line)
	}
	if row.Selected && !row.Cursor {
		line = SelectionStyle.Render(line)
	}
	if row.Cursor {
		line = SelectedLine(line, width)
	}
	return truncateToVisualWidth(line, width)
}

func renderListContinuation(direction string, count, width int) string {
	label := fmt.Sprintf("%s %d more", direction, count)
	return MutedStyle.Render(truncateToVisualWidth(label, width))
}

func renderListSurfacePreview(preview ListPreview, width, height int) string {
	lines := make([]string, 0, height)
	appendLine := func(line string) {
		if height > 0 && len(lines) >= height {
			return
		}
		lines = append(lines, truncateToVisualWidth(line, width))
	}
	appendWrapped := func(text string, style lipgloss.Style) {
		if strings.TrimSpace(text) == "" {
			return
		}
		for _, line := range wrapText(text, max(width, 1)) {
			appendLine(style.Render(line))
		}
	}

	if preview.Title != "" {
		appendWrapped(preview.Title, ListPreviewHeaderStyle)
	}
	if preview.Summary != "" {
		appendWrapped(preview.Summary, MutedStyle)
	}
	for _, field := range preview.Fields {
		label := strings.TrimSpace(field.Label)
		value := strings.TrimSpace(field.Value)
		switch {
		case label != "" && value != "":
			appendWrapped(label+": "+value, lipgloss.NewStyle())
		case label != "":
			appendWrapped(label, lipgloss.NewStyle())
		case value != "":
			appendWrapped(value, lipgloss.NewStyle())
		}
	}
	if preview.Body != "" {
		if len(lines) > 0 {
			appendLine("")
		}
		appendWrapped(preview.Body, lipgloss.NewStyle())
	}
	if len(lines) == 0 {
		appendLine(MutedStyle.Render("No preview available."))
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
			leftLine = truncateToVisualWidth(leftLines[index], leftWidth)
		}
		rightLine := ""
		if index < len(rightLines) {
			rightLine = truncateToVisualWidth(rightLines[index], rightWidth)
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
	helpLines := wrapText(help, max(width, 1))
	for index := range helpLines {
		helpLines[index] = HelpStyle.Render(helpLines[index])
	}
	if content == "" {
		return strings.Join(helpLines, "\n")
	}
	return content + "\n\n" + strings.Join(helpLines, "\n")
}
