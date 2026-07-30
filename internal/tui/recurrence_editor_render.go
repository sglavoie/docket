package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *RecurrenceEditorModel) View() tea.View {
	var b strings.Builder

	// Interval row
	var intervalLabel string
	if m.focusSection == 0 {
		intervalLabel = FieldLabelFocusedStyle.Render("Interval: ")
	} else {
		intervalLabel = FieldLabelStyle.Render("Interval: ")
	}
	b.WriteString(intervalLabel)

	// Interval value
	intervalStr := fmt.Sprintf("%d", m.interval)
	if m.interval == 0 {
		intervalStr = " " // Show empty placeholder when cleared
	}
	if m.focusSection == 0 {
		b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", intervalStr)))
	} else {
		b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", intervalStr)))
	}
	b.WriteString("  ")

	// Unit selector
	var unitLabel string
	if m.focusSection == 1 {
		unitLabel = FieldLabelFocusedStyle.Render("Unit: ")
	} else {
		unitLabel = FieldLabelStyle.Render("Unit: ")
	}
	b.WriteString(unitLabel)
	b.WriteString(m.renderUnitSelector())
	b.WriteString("\n\n")

	// Separator
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("─────────────────────────────────────────"))
	b.WriteString("\n\n")

	// Preview
	pattern := m.buildPattern()
	b.WriteString(FieldLabelStyle.Render("Preview: "))
	if pattern != nil {
		b.WriteString(BoldStyle.Render(pattern.String()))
	}
	b.WriteString("\n")

	// Next occurrences
	tempTask := &models.Task{
		DueDate:    m.dueDate,
		Recurrence: pattern,
	}
	upcoming := tempTask.NextNDueDates(3)
	if len(upcoming) > 0 {
		for i, d := range upcoming {
			label := fmt.Sprintf("Next %d:  ", i+1)
			b.WriteString(FieldLabelStyle.Render(label))
			if i == 0 {
				b.WriteString(d.Format("Mon Jan 2, 2006"))
			} else {
				b.WriteString(MutedStyle.Render(d.Format("Mon Jan 2, 2006")))
			}
			b.WriteString("\n")
		}
	} else {
		b.WriteString(MutedStyle.Render("Set a due date to see next occurrences"))
		b.WriteString("\n")
	}

	left := HelpStyle.Render("Tab: nav · ←→: select · Enter: set · c: clear · Esc: cancel")

	var out strings.Builder
	out.WriteString(renderFullscreenTitle("Custom Recurrence", ""))
	out.WriteString("\n\n")
	out.WriteString(b.String())
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(left, "", m.width))

	return tea.NewView(out.String())
}

func (m *RecurrenceEditorModel) renderUnitSelector() string {
	var b strings.Builder

	unitNames := []string{"day", "week", "month"}

	for i, name := range unitNames {
		if i == m.unitCursor {
			if m.focusSection == 1 {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", name)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", name)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", name)))
		}
	}

	return b.String()
}
