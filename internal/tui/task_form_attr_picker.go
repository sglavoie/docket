package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// LabeledValue pairs a display label with a string value for generic pickers.
type LabeledValue struct {
	Label string
	Value string // empty string means "none" / cleared
}

// TaskFormAttrPickedMsg is emitted when the user selects a value in formAttrPicker.
type TaskFormAttrPickedMsg struct {
	Field string // which draft attr field to update
	Value string // selected string value (empty = cleared/none)
}

// formAttrPicker is a generic inline radio overlay for type-specific attributes.
// It mirrors the structure of formPriorityPicker.
type formAttrPicker struct {
	title   string
	field   string
	options []LabeledValue
	cursor  int
}

// newFormAttrPicker creates an attr picker with cursor pre-set to the entry whose
// Value matches current. If no match is found, cursor defaults to 0 (none).
func newFormAttrPicker(title, field string, options []LabeledValue, current string) *formAttrPicker {
	cursor := 0
	for i, o := range options {
		if o.Value == current {
			cursor = i
			break
		}
	}
	return &formAttrPicker{title: title, field: field, options: options, cursor: cursor}
}

// handleKey processes a key press. Returns (cmd, dismiss, resultMsg).
func (p *formAttrPicker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return nil, true, nil
	case "enter":
		return nil, true, TaskFormAttrPickedMsg{Field: p.field, Value: p.options[p.cursor].Value}
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
		return nil, false, nil
	case "down", "j":
		if p.cursor < len(p.options)-1 {
			p.cursor++
		}
		return nil, false, nil
	}
	return nil, false, nil
}

func (p *formAttrPicker) render(width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(p.title)
	footerText := HelpStyle.Render("j/k: navigate · Enter: select · Esc: cancel")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	maxWidth := min(max(width*2/5, 36), width-4)
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	var content strings.Builder
	for i, opt := range p.options {
		line := fmt.Sprintf(" %s", opt.Label)
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth-2 {
			line += strings.Repeat(" ", innerWidth-2-lineWidth)
		}
		if i == p.cursor {
			line = SelectedStyle.Render(line)
		}
		content.WriteString(line)
		if i < len(p.options)-1 {
			content.WriteString("\n")
		}
	}

	return renderTitledBox(content.String(), innerWidth, title, footerText, borderStyle)
}

// ── Option lists (first entry is always "none") ───────────────────────────────

var attrOptsBugReproducible = []LabeledValue{
	{"(none)", ""},
	{"yes", string(models.BugReproducibleYes)},
	{"no", string(models.BugReproducibleNo)},
	{"sometimes", string(models.BugReproducibleSometimes)},
}

var attrOptsBugSeverity = []LabeledValue{
	{"(none)", ""},
	{"critical", string(models.BugSeverityCritical)},
	{"major", string(models.BugSeverityMajor)},
	{"minor", string(models.BugSeverityMinor)},
	{"cosmetic", string(models.BugSeverityCosmetic)},
}

var attrOptsIdeaCategory = []LabeledValue{
	{"(none)", ""},
	{"feature", string(models.IdeaCategoryFeature)},
	{"improvement", string(models.IdeaCategoryImprovement)},
	{"refactor", string(models.IdeaCategoryRefactor)},
	{"research", string(models.IdeaCategoryResearch)},
}

var attrOptsEffort = []LabeledValue{
	{"(none)", ""},
	{"small", string(models.EffortSmall)},
	{"medium", string(models.EffortMedium)},
	{"large", string(models.EffortLarge)},
	{"unknown", string(models.EffortUnknown)},
}

var attrOptsIdeaValue = []LabeledValue{
	{"(none)", ""},
	{"low", string(models.IdeaValueLow)},
	{"medium", string(models.IdeaValueMedium)},
	{"high", string(models.IdeaValueHigh)},
}

var attrOptsIdeaValidated = []LabeledValue{
	{"not validated", ""},
	{"validated", "true"},
}

var attrOptsWorkType = []LabeledValue{
	{"(none)", ""},
	{"development", string(models.WorkTypeDevelopment)},
	{"documentation", string(models.WorkTypeDocumentation)},
	{"testing", string(models.WorkTypeTesting)},
	{"devops", string(models.WorkTypeDevOps)},
	{"design", string(models.WorkTypeDesign)},
}

var attrOptsStoryPoints = []LabeledValue{
	{"(none)", ""},
	{"1", "1"},
	{"2", "2"},
	{"3", "3"},
	{"5", "5"},
	{"8", "8"},
	{"13", "13"},
	{"21", "21"},
}

var attrOptsEpicScope = []LabeledValue{
	{"(none)", ""},
	{"defined", string(models.EpicScopeDefined)},
	{"evolving", string(models.EpicScopeEvolving)},
	{"undefined", string(models.EpicScopeUndefined)},
}
