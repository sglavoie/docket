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
		return chrome.GuidanceSpec{
			Hints: chrome.RequiredGuidanceHints("y/Enter apply", "n/Esc back"),
			Rows:  2,
		}, true
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

// renderVocabularyRename paints the open step, if there is one.
func (model *OperationValuePickerModel) renderVocabularyRename(innerWidth int) (string, bool) {
	bodyWidth := max(innerWidth-4, 1)
	var title, content string
	switch model.rename.stage {
	case vocabularyRenameNaming:
		title, content = model.renderVocabularyRenameNaming(bodyWidth)
	case vocabularyRenameConfirming:
		title, content = model.renderVocabularyRenamePlan(bodyWidth)
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

func (model *OperationValuePickerModel) renderVocabularyRenamePlan(width int) (string, string) {
	lines := make([]string, 0, len(model.rename.plan.Lines))
	for _, line := range model.rename.plan.Lines {
		lines = append(lines, textfit.TruncateToVisualWidth(line, width))
	}
	return model.rename.plan.Heading, strings.Join(lines, "\n")
}

func (model *OperationValuePickerModel) renderVocabularyRenameOutcome(width int) (string, string) {
	all := model.rename.outcome.Lines
	offset := min(max(model.rename.offset, 0), model.vocabularyRenameMaxOffset())
	end := min(offset+vocabularyRenameReportLines, len(all))
	lines := make([]string, 0, vocabularyRenameReportLines+1)
	for _, line := range all[offset:end] {
		lines = append(lines, textfit.TruncateToVisualWidth(line, width))
	}
	if remaining := len(all) - end; remaining > 0 {
		lines = append(lines, theme.MutedStyle.Render(fmt.Sprintf("… %d more", remaining)))
	}
	return model.rename.outcome.Heading, strings.Join(lines, "\n")
}
