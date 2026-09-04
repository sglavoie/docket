package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
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

func (model *OperationValuePickerModel) guidanceSpec() (chrome.GuidanceSpec, bool) {
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
	count := len(model.surface.Matches())
	searching := model.searching
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
		keymap.Bind("j", "Move selection", "Navigation",
			keymap.WithGate(func() bool { return count > 0 }),
			keymap.WithFooter(10, "navigate", false), keymap.WithFooterKey("j/k or ↑/↓")),
		keymap.Bind("/", "Search values", "Filters",
			keymap.WithGate(func() bool { return !searching }), keymap.WithFooter(20, "search", false)),
		keymap.Bind("enter", "Apply current state", "Actions",
			keymap.WithGate(func() bool { return searching || count > 0 }),
			keymap.WithFooter(1, enterLabel, true), keymap.WithFooterKey("Enter")),
		keymap.Bind("esc", "Unwind value picker", "Navigation",
			keymap.WithFooter(2, escapeLabel, true), keymap.WithFooterKey("Esc")),
	}}
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
		add("a", "Type a specific date", "set date", 4, func() bool { return !searching })
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
		guidance, _ := model.guidanceSpec()
		return chrome.RenderOverlayBox(
			content,
			innerWidth,
			title,
			chrome.RenderGuidanceText(guidance, max(innerWidth-4, 1)),
			lipgloss.NewStyle().Foreground(theme.ColorPrimary),
		)
	}
	if model.provider.DateEntry() && model.dateAdder.AddDateMode {
		content := model.dateAdder.AddDateInput.View() + "\n\n" +
			theme.MutedStyle.Render(strings.Join(model.dateAdder.DateAdderFormats(), "\n"))
		if model.dateAdder.AddDateError != "" {
			content += "\n\n" + theme.BoldStyle.Foreground(theme.ColorError).Render(model.dateAdder.AddDateError)
		}
		title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render("Set a Specific Date")
		guidance, _ := model.guidanceSpec()
		return chrome.RenderOverlayBox(
			content,
			innerWidth,
			title,
			chrome.RenderGuidanceText(guidance, max(innerWidth-4, 1)),
			lipgloss.NewStyle().Foreground(theme.ColorPrimary),
		)
	}
	explainerWidth := operationValuePickerExplainerWidth(width)
	explainers := make([]string, 0, 3)
	if explainer := projecthub.BehavioralProjectTagExplainer(model.provider, explainerWidth); explainer != "" {
		explainers = append(explainers, explainer)
	}
	if explainer := semanticTaskTagExplainer(model.provider, explainerWidth); explainer != "" {
		explainers = append(explainers, explainer)
	}
	if hint := taskTagCategoryJumpHint(
		model.provider, model.surface.Items(), explainerWidth,
	); hint != "" {
		explainers = append(explainers, hint)
	}
	explainerBlock := strings.Join(explainers, "\n\n")

	listHeight := min(max(len(model.surface.Rows()), 6), 15)
	if height > 0 {
		// Halo, frame, title, search, search/list gap, and footer consume nine
		// rows. A present explainer adds its own rows plus one separating row.
		reservedRows := 9
		if explainerBlock != "" {
			reservedRows += 1 + lipgloss.Height(explainerBlock)
		}
		listHeight = min(listHeight, max(height-reservedRows, 1))
	}
	query := model.surface.Query().Raw
	search := ""
	if model.searching || query != "" {
		cursor := ""
		if model.searching {
			cursor = "▏"
		}
		search = "Search: " + query + cursor
	}
	content := listsurface.RenderCompactListSurface(model.surface, innerWidth-4, listHeight)
	if search != "" {
		content = theme.MutedStyle.Render(search) + "\n\n" + content
	}
	if explainerBlock != "" {
		content += "\n\n" + explainerBlock
	}
	titleText := model.title
	if model.subtitle != "" {
		titleText += ": " + textfit.TruncateToVisualWidth(model.subtitle, max(innerWidth-20, 12))
	}
	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render(titleText)
	guidance, _ := model.guidanceSpec()
	return chrome.RenderOverlayBox(
		content,
		innerWidth,
		title,
		chrome.RenderGuidanceText(guidance, max(innerWidth-4, 1)),
		lipgloss.NewStyle().Foreground(theme.ColorPrimary),
	)
}
