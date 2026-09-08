package valuepicker

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// The three rename steps paint as the picker's whole body rather than as
// overlays above it, which is the visible half of the same rule the key
// dispatch states: the list is not shown while a step is open, and it is not
// rebuilt either, so dismissing a step reveals exactly the query, cursor, and
// pending selection the reader left behind.

// vocabularyRenameGuidanceSpec is the footer for whichever step is open.
func (model *OperationValuePickerModel) vocabularyRenameGuidanceSpec() (chrome.GuidanceSpec, bool) {
	switch model.rename.stage {
	case vocabularyRenameNaming:
		return chrome.GuidanceSpec{
			Hints: chrome.RequiredGuidanceHints("Enter review", "Esc cancel"),
			Rows:  2,
		}, true
	case vocabularyRenameConfirming:
		// The two answers are required, so a narrow footer drops the scroll hint
		// and never the apply and the way back out.
		hints := chrome.RequiredGuidanceHints("y/Enter apply", "n/Esc back")
		if model.vocabularyRenameMaxOffset() > 0 {
			hints = append(hints, chrome.OptionalGuidanceHints(10, "j/k scroll")...)
		}
		return chrome.GuidanceSpec{Hints: hints, Rows: 2}, true
	case vocabularyRenameReporting:
		hints := []string{"Esc close"}
		if model.vocabularyRenameMaxOffset() > 0 {
			hints = append([]string{"j/k scroll"}, hints...)
		}
		return chrome.GuidanceSpec{Hints: chrome.RequiredGuidanceHints(hints...), Rows: 2}, true
	case vocabularyRenameOff:
	}
	return chrome.GuidanceSpec{}, false
}

// renderVocabularyRename paints the open step, if there is one. A zero height
// is an isolated render caller with no viewport to stay inside, which keeps the
// natural body height.
func (model *OperationValuePickerModel) renderVocabularyRename(innerWidth, height int) (string, bool) {
	bodyWidth := max(innerWidth-4, 1)
	bodyHeight := 0
	if height > 0 {
		bodyHeight = max(height-vocabularyRenameChromeRows, 1)
	}
	var title, content string
	switch model.rename.stage {
	case vocabularyRenameNaming:
		title, content = model.renderVocabularyRenameNaming(bodyWidth)
	case vocabularyRenameConfirming:
		title, content = model.renderVocabularyRenamePlan(bodyWidth, bodyHeight)
	case vocabularyRenameReporting:
		title, content = model.renderVocabularyRenameOutcome(bodyWidth)
	case vocabularyRenameOff:
		return "", false
	}
	guidance, _ := model.GuidanceSpec()
	return chrome.RenderOverlayBox(
		content,
		innerWidth,
		lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render(title),
		chrome.RenderGuidanceText(guidance, bodyWidth),
		lipgloss.NewStyle().Foreground(theme.ColorPrimary),
	), true
}

func (model *OperationValuePickerModel) renderVocabularyRenameNaming(width int) (string, string) {
	noun := model.vocabularyRenameNoun()
	// The field admits the whole canonical namespace length, which is wider than a
	// narrow box can draw, so its window is set from the width it is painted at
	// rather than from what it may hold. Re-anchoring the cursor is what applies
	// that width: the input recomputes its window on a cursor move and not on a
	// resize, so a prefilled name would otherwise wrap on the first frame and only
	// start scrolling once a key was pressed.
	model.rename.input.SetWidth(max(width-lipgloss.Width(model.rename.input.Prompt), 10))
	model.rename.input.SetCursor(model.rename.input.Position())
	lead := fmt.Sprintf(
		"Rename %s across every unarchived project. Typing a name already in use merges the two.",
		model.rename.from,
	)
	content := strings.Join(textfit.WrapStyled(theme.MutedStyle.Render(lead), width), "\n") +
		"\n\n" + model.rename.input.View()
	if model.rename.failure != "" {
		content += "\n\n" + strings.Join(textfit.WrapStyled(
			theme.BoldStyle.Foreground(theme.ColorError).Render(model.rename.failure), width,
		), "\n")
	}
	return "Rename " + strings.ToUpper(noun[:1]) + noun[1:], content
}

// renderVocabularyRenamePlan budgets the confirmation against the box it is
// painted in. The summary is what the reader is answering for — the
// destination, the counts, the scope, and how many lenses are named — so it is
// pinned and the enumeration underneath is what scrolls.
func (model *OperationValuePickerModel) renderVocabularyRenamePlan(width, bodyHeight int) (string, string) {
	summary := model.rename.plan.Summary
	lines := model.rename.plan.Lines
	windowRows := len(lines)
	if bodyHeight > 0 {
		summary = summary[:min(len(summary), max(bodyHeight-vocabularyRenameConfirmMinWindowRows, 1))]
		windowRows = max(bodyHeight-len(summary), 1)
		if len(lines) > windowRows {
			// The row that says what is held back is one of the rows there is room
			// for, so it is only taken from the window when there is a remainder.
			windowRows = max(windowRows-1, 1)
		}
	}
	return model.rename.plan.Heading,
		model.renderVocabularyRenameWindow(summary, lines, width, windowRows)
}

func (model *OperationValuePickerModel) renderVocabularyRenameOutcome(width int) (string, string) {
	return model.rename.outcome.Heading,
		model.renderVocabularyRenameWindow(nil, model.rename.outcome.Lines, width, vocabularyRenameReportLines)
}

// renderVocabularyRenameWindow paints a pinned summary above one window of
// lines, and records the count and the window height because this is the only
// place either is known. The offset is clamped here rather than only at the key,
// so a window that shrank under a resize reseats what it can no longer hold.
func (model *OperationValuePickerModel) renderVocabularyRenameWindow(
	summary, lines []string, width, windowRows int,
) string {
	model.rename.rows = len(lines)
	model.rename.window = windowRows
	model.rename.offset = min(max(model.rename.offset, 0), model.vocabularyRenameMaxOffset())
	end := min(model.rename.offset+windowRows, len(lines))

	rendered := make([]string, 0, len(summary)+windowRows+1)
	for _, line := range summary {
		rendered = append(rendered, textfit.TruncateToVisualWidth(line, width))
	}
	for _, line := range lines[model.rename.offset:end] {
		rendered = append(rendered, textfit.TruncateToVisualWidth(line, width))
	}
	if remaining := len(lines) - end; remaining > 0 {
		rendered = append(rendered, theme.MutedStyle.Render(fmt.Sprintf("… %d more", remaining)))
	}
	return strings.Join(rendered, "\n")
}
