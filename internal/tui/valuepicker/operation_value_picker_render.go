package valuepicker

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/keymap"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/projecthub"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// operationValuePickerInnerWidth is the overlay box width the picker claims out
// of the terminal.
func operationValuePickerInnerWidth(width int) int {
	return min(max(width*9/10, 46), max(width-6, 24))
}

// operationValuePickerExplainerWidth is the width the explainers are rendered
// at: the box content, inset from the border.
func operationValuePickerExplainerWidth(width int) int {
	return operationValuePickerInnerWidth(width) - 4
}

func (model *OperationValuePickerModel) GuidanceSpec() (chrome.GuidanceSpec, bool) {
	if model == nil || model.surface == nil {
		return chrome.GuidanceSpec{}, false
	}
	if model.provider.TagEntry() && model.tagAdder.AddTagMode {
		return chrome.GuidanceSpec{
			Hints: chrome.RequiredGuidanceHints(
				"Enter add "+model.tagAdderNoun(), "Esc cancel",
			),
			Rows: 2,
		}, true
	}
	if model.provider.DateEntry() && model.dateAdder.AddDateMode {
		return chrome.GuidanceSpec{
			Hints: chrome.RequiredGuidanceHints("Enter set date", "Esc cancel"),
			Rows:  2,
		}, true
	}
	if spec, ok := model.vocabularyRenameGuidanceSpec(); ok {
		return spec, true
	}
	count := len(model.surface.Matches())
	searching := model.searching
	navigationKeys := "j/k or ↑/↓"
	if searching {
		navigationKeys = "↑/↓"
	}
	enterLabel := "choose"
	if model.multi {
		enterLabel = "apply"
	}
	if searching {
		enterLabel = "apply search"
	}
	escapeLabel := "back"
	if searching {
		escapeLabel = "cancel search"
		if model.surface.Query().Raw != "" {
			escapeLabel = "clear search"
		}
	}
	km := &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("up", "Move selection", "Navigation",
			keymap.WithGate(func() bool { return count > 0 }),
			keymap.WithFooter(10, "navigate", false), keymap.WithFooterKey(navigationKeys)),
		keymap.Bind("/", "Search values", "Filters",
			keymap.WithGate(func() bool { return !searching }), keymap.WithFooter(20, "search", false)),
		keymap.Bind("enter", "Apply current state", "Actions",
			keymap.WithGate(func() bool { return searching || count > 0 }),
			keymap.WithFooter(1, enterLabel, true), keymap.WithFooterKey("Enter")),
		keymap.Bind("esc", "Unwind value picker", "Navigation",
			keymap.WithFooter(2, escapeLabel, true), keymap.WithFooterKey("Esc")),
	}}
	if model.sectionedDatePicker() {
		// Crossing between scheduling sections is navigation rather than an action, so
		// it is bound here rather than through `add`, which speaks for the Actions
		// category. The hint is optional, so it can never displace Enter or Esc.
		km.Entries = append(km.Entries, keymap.Bind(
			"tab", "Switch between Due, Defer, and Check-in sections", "Navigation",
			keymap.WithGate(func() bool { return !searching && count > 0 }),
			keymap.WithFooter(3, "switch field", false), keymap.WithFooterKey("Tab"),
		))
	}
	add := func(key, description, label string, rank int, gate keymap.Gate) {
		options := []keymap.BindOption{
			keymap.WithGate(gate), keymap.WithFooter(rank, label, false),
		}
		if key == "space" {
			options = append(options, keymap.WithFooterKey("Space"))
		}
		km.Entries = append(km.Entries, keymap.Bind(key, description, "Actions", options...))
	}
	if model.provider == operation.ValueProviderFilterProjectTags {
		add("v", "Toggle current facet", "toggle", 3, func() bool { return !searching && count > 0 })
		add("space", "Open tag actions", "tag actions", 4, func() bool { return !searching && count > 0 })
	} else if model.multi {
		add("space", "Toggle current value", "toggle", 3, func() bool { return !searching && count > 0 })
	}
	if model.provider.TagEntry() {
		noun := "tag"
		if model.contextRowUnderCursor() {
			noun = "context"
		}
		add("a", "Add a "+noun, "add "+noun, 4, func() bool { return !searching })
	}
	if model.provider == operation.ValueProviderTags {
		add("#", "Set the task context", "set context", 5, func() bool {
			return !searching && count > 0
		})
	}
	if model.provider.DateEntry() {
		model.appendDateGuidance(km, searching, count)
	}

	if vocabularyRenameProvider(model.provider) {
		add("e", "Rename or merge this "+model.vocabularyRenameNoun(), "rename", 5, func() bool {
			return !searching && count > 0
		})
	}
	return listsurface.ListSurfaceGuidanceSpec(model.surface.GuidanceState(searching), km, nil)
}

func (model *OperationValuePickerModel) Render(width int) string {
	return model.RenderWithin(width, 0)
}

// RenderWithin budgets the focusable list after reserving viewport rows for
// non-focusable explainers and modal chrome. A zero height keeps the natural
// list height for isolated render callers.
func (model *OperationValuePickerModel) RenderWithin(width, height int) string {
	if model == nil || model.surface == nil {
		return ""
	}
	innerWidth := operationValuePickerInnerWidth(width)
	if model.provider.TagEntry() && model.tagAdder.AddTagMode {
		noun := model.tagAdderNoun()
		content := theme.MutedStyle.Render(fmt.Sprintf(
			"Enter a %s name (max %d characters)",
			noun,
			model.tagAdder.AddTagInput.CharLimit,
		)) + "\n\n" + model.tagAdder.AddTagInput.View()
		if model.tagAdder.AddTagError != "" {
			content += "\n\n" + theme.BoldStyle.Foreground(theme.ColorError).Render(model.tagAdder.AddTagError)
		}
		title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render(
			"Add New " + strings.ToUpper(noun[:1]) + noun[1:],
		)
		guidance, _ := model.GuidanceSpec()
		return chrome.RenderOverlayBox(
			content,
			innerWidth,
			title,
			chrome.RenderGuidanceText(guidance, max(innerWidth-4, 1)),
			lipgloss.NewStyle().Foreground(theme.ColorPrimary),
		)
	}
	if model.provider.DateEntry() && model.dateAdder.AddDateMode {
		// An input with no width draws one cell of its placeholder, which reads
		// as a typed "e"; a render-local copy takes the box's width instead.
		input := model.dateAdder.AddDateInput
		input.SetWidth(max(innerWidth-4-lipgloss.Width(input.Prompt), 1))
		content := input.View()
		if preview := model.dateAdder.DatePreview(); preview != "" {
			content += "\n" + theme.MutedStyle.Render(preview)
		}
		content += "\n\n" + theme.MutedStyle.Render(strings.Join(model.dateAdder.DateAdderFormats(), "\n"))
		if model.dateAdder.AddDateError != "" {
			content += "\n\n" + theme.BoldStyle.Foreground(theme.ColorError).Render(model.dateAdder.AddDateError)
		}
		titleText := "Set due date"
		if model.dateDeferred {
			titleText = "Defer until"
		}
		title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render(titleText)
		guidance, _ := model.GuidanceSpec()
		return chrome.RenderOverlayBox(
			content,
			innerWidth,
			title,
			chrome.RenderGuidanceText(guidance, max(innerWidth-4, 1)),
			lipgloss.NewStyle().Foreground(theme.ColorPrimary),
		)
	}
	if rendered, ok := model.renderVocabularyRename(innerWidth, height); ok {
		return rendered
	}
	explainerWidth := operationValuePickerExplainerWidth(width)
	// The legends restate what the vocabulary means; the jump hint names keys
	// that are live right now. They are kept apart so the first can be given up
	// without the second.
	legends := make([]string, 0, 2)
	if explainer := projecthub.BehavioralProjectTagExplainer(model.provider, explainerWidth); explainer != "" {
		legends = append(legends, explainer)
	}
	if explainer := SemanticTaskTagExplainer(model.provider, explainerWidth); explainer != "" {
		legends = append(legends, explainer)
	}
	jumpHint := TaskTagCategoryJumpHint(model.provider, model.surface.Items(), explainerWidth)

	guidance, _ := model.GuidanceSpec()
	guidanceText := chrome.RenderGuidanceText(guidance, max(innerWidth-4, 1))
	// The context line wraps whole rather than eliding: a value it would cut is
	// one the rows below never show, so a truncated line would read as complete.
	contextLines := textfit.WrapText(model.contextLine, max(innerWidth-4, 1))

	rowCount := len(model.surface.Rows())
	listHeight := min(max(rowCount, 6), 15)
	explainerBlock := joinExplainerBlocks(legends, jumpHint)
	if height > 0 {
		// Halo, frame, title, search, and the search/list gap consume eight rows.
		// The footer is measured rather than assumed to be one, so a picker that
		// earns a second guidance row — an active field to name beside the keys
		// that move between them — takes that row out of the list instead of out
		// of the terminal. A present explainer adds its own rows plus one
		// separating row, and a context line adds the rows it wraps to.
		chromeRows := 8 + lipgloss.Height(guidanceText) + len(contextLines)
		available := height - chromeRows - explainerBlockRows(explainerBlock)
		// The list outranks the legends: a choice that scrolled out of a
		// one-row window is not recoverable from a sentence about the
		// vocabulary, while every row still spells its own summary. So a
		// budget that cannot show the rows the picker holds gives the legends'
		// rows to the list, and whatever is rendered is what is reserved.
		if len(legends) > 0 && available < min(rowCount, listHeight) {
			explainerBlock = joinExplainerBlocks(nil, jumpHint)
			available = height - chromeRows - explainerBlockRows(explainerBlock)
		}
		listHeight = min(listHeight, max(available, 1))
	}
	query := model.surface.Query().Raw
	search := ""
	if model.searching || query != "" {
		search = model.searchInput.View(query, model.searching, innerWidth-4)
	}
	content := listsurface.RenderCompactListSurface(model.surface, innerWidth-4, listHeight)
	if search != "" {
		content = search + "\n\n" + content
	}
	if len(contextLines) > 0 {
		content = theme.MutedStyle.Render(strings.Join(contextLines, "\n")) + "\n" + content
	}
	if explainerBlock != "" {
		content += "\n\n" + explainerBlock
	}
	titleText := chrome.FitOverlayTitle(model.title, model.subtitle, "", max(innerWidth-4, 1))
	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render(titleText)
	// The rendered footer is the one the budget above reserved for, so the two
	// cannot disagree about how many rows the guidance costs.
	return chrome.RenderOverlayBox(
		content,
		innerWidth,
		title,
		guidanceText,
		lipgloss.NewStyle().Foreground(theme.ColorPrimary),
	)
}

func (model *OperationValuePickerModel) appendDateGuidance(km *keymap.KeyMap, searching bool, count int) {
	// Check-in opens its own editor, so only Due and Defer advertise date typing
	// and immediate presets in the shared picker.
	noun := model.dateEntryNoun()
	if noun == "" {
		return
	}
	km.Entries = append(km.Entries,
		keymap.Bind("a", "Type a specific "+noun, "Actions", keymap.WithGate(func() bool { return !searching }), keymap.WithFooter(4, "set "+noun, false)),
		keymap.Bind("0", "Apply a date preset by its key", "Actions", keymap.WithGate(func() bool { return !searching && count > 0 }), keymap.WithFooter(6, "pick", false), keymap.WithFooterKey("preset key")),
	)
}

// joinExplainerBlocks stacks the blocks drawn beneath a picker's list, legends
// first and the jump hint last, skipping whichever is absent.
func joinExplainerBlocks(legends []string, jumpHint string) string {
	blocks := append([]string(nil), legends...)
	if jumpHint != "" {
		blocks = append(blocks, jumpHint)
	}
	return strings.Join(blocks, "\n\n")
}

// explainerBlockRows is what a block beneath the list costs the height budget:
// its own rows plus the one separating it from the list, or nothing when absent.
func explainerBlockRows(block string) int {
	if block == "" {
		return 0
	}
	return 1 + lipgloss.Height(block)
}
