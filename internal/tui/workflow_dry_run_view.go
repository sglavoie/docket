package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *WorkflowDryRunModel) View() string {
	var b strings.Builder

	// Title
	subtitle := ""
	if m.singleRule != nil {
		subtitle = fmt.Sprintf("testing rule: %s", m.singleRule.Name)
	} else {
		subtitle = fmt.Sprintf("%d rules · %d tasks scanned", m.ruleCount, m.taskCount)
	}
	b.WriteString(renderFullscreenTitle("Workflow Dry Run", subtitle))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(MutedStyle.Render("Evaluating rules against all tasks..."))
		return b.String()
	}

	if m.loadErr != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorError).Render(fmt.Sprintf("Error: %s", m.loadErr.Error())))
		b.WriteString("\n\n")
		b.WriteString(renderStatusBarFooter(HelpStyle.Render("q: close"), "", m.Width()))
		return b.String()
	}

	// Show search input when active
	if m.searchActive || m.searchQuery != "" {
		b.WriteString("/")
		b.WriteString(m.SearchInputView())
		if m.searchQuery != "" && !m.searchActive {
			b.WriteString(MutedStyle.Render(fmt.Sprintf(" (%d matches)", len(m.filtered))))
		}
		b.WriteString("\n\n")
	}

	// Summary line
	affectedCount := len(m.results)
	if affectedCount == 0 {
		b.WriteString(MutedStyle.Render("No tasks would be affected by the current rules."))
		b.WriteString("\n")
	} else {
		summaryText := fmt.Sprintf("%d task", affectedCount)
		if affectedCount != 1 {
			summaryText += "s"
		}
		summaryText += " would be affected"
		b.WriteString(DoneStyle.Render(summaryText))
		b.WriteString("\n\n")

		// Calculate available height for content
		// Title(1) + blank(1) + search(2 if active) + summary(2) + conflicts(variable) + footer(2) = ~8 lines overhead
		availableHeight := m.Height() - 8
		if m.searchActive || m.searchQuery != "" {
			availableHeight -= 2
		}
		if availableHeight < 5 {
			availableHeight = 5
		}

		// Wide screen: split layout
		if m.isWideScreen() {
			listContent := m.renderTaskList(availableHeight)
			detailContent := m.renderDetailPane()

			listWidth := m.Width() * 45 / 100
			detailWidth := m.Width() - listWidth - 3

			listPane := lipgloss.NewStyle().
				Width(listWidth).
				Height(availableHeight).
				Render(listContent)

			detailPane := lipgloss.NewStyle().
				Width(detailWidth).
				Height(availableHeight).
				Render(detailContent)

			separator := SeparatorStyle.Render(strings.Repeat("┃\n", availableHeight))
			mainContent := lipgloss.JoinHorizontal(lipgloss.Top, listPane, " ", separator, " ", detailPane)
			b.WriteString(mainContent)
		} else {
			// Narrow screen: list with inline details
			b.WriteString(m.renderTaskListWithDetails(availableHeight))
		}
	}

	// Conflicts section
	if len(m.conflicts) > 0 {
		b.WriteString("\n")
		b.WriteString(PreviewWarningStyle.Render(fmt.Sprintf("Conflicts (%d)", len(m.conflicts))))
		b.WriteString("\n")
		for _, c := range m.conflicts {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  %s: %s conflict between %s",
				c.TaskTitle, c.Field, strings.Join(c.Rules, ", "))))
			b.WriteString("\n")
		}
	}

	// Help footer
	var helpParts []string
	helpParts = append(helpParts, "/: search", "j/k: nav", "q: close")
	b.WriteString("\n")
	b.WriteString(renderStatusBarFooter(HelpStyle.Render(strings.Join(helpParts, " · ")), "", m.Width()))

	return b.String()
}

// renderTaskList renders the scrollable task list for split layout.
func (m *WorkflowDryRunModel) renderTaskList(paneHeight int) string {
	if len(m.filtered) == 0 {
		if m.searchQuery != "" {
			return MutedStyle.Render("No tasks match your search.")
		}
		return MutedStyle.Render("No tasks affected.")
	}

	var b strings.Builder

	maxVisible := paneHeight
	if maxVisible < 3 {
		maxVisible = 3
	}

	startIdx := 0
	if m.cursor >= maxVisible {
		startIdx = m.cursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
	}

	listWidth := m.Width()*45/100 - 4
	if listWidth < 20 {
		listWidth = 20
	}

	var rowBuilder strings.Builder
	for i := startIdx; i < endIdx; i++ {
		result := m.filtered[i]
		title := result.Task.Title
		if len(title) > listWidth-4 {
			title = title[:listWidth-7] + "..."
		}

		projectTag := MutedStyle.Render("[" + result.ProjectName + "]")
		changeCount := MutedStyle.Render(fmt.Sprintf("(%d)", len(result.Changes)))

		if i == m.cursor {
			line := SelectedItemTextStyle.Render(fmt.Sprintf("▸ %s", title))
			rowBuilder.WriteString(line)
			rowBuilder.WriteString(" ")
			rowBuilder.WriteString(changeCount)
			rowBuilder.WriteString("\n")
			rowBuilder.WriteString("  ")
			rowBuilder.WriteString(projectTag)
			rowBuilder.WriteString("\n")
		} else {
			rowBuilder.WriteString(fmt.Sprintf("  %s %s\n", title, changeCount))
		}
	}

	scrollbar := RenderScrollbar(len(m.filtered), maxVisible, startIdx)
	b.WriteString(AttachScrollbar(rowBuilder.String(), scrollbar))

	return b.String()
}

// renderDetailPane renders the detail pane for the selected task in split layout.
func (m *WorkflowDryRunModel) renderDetailPane() string {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return MutedStyle.Render("No task selected")
	}

	result := m.filtered[m.cursor]
	var b strings.Builder

	keywordStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	operatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("13"))

	// Task info
	b.WriteString(BoldStyle.Foreground(ColorPrimary).Render(result.Task.Title))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(fmt.Sprintf("Project: %s  Status: %s  Priority: %d",
		result.ProjectName, string(result.Task.Status), result.Task.Priority)))
	b.WriteString("\n\n")

	// Matched rules
	b.WriteString(BoldStyle.Render("Matched Rules"))
	b.WriteString("\n")
	for _, name := range result.MatchedRules {
		b.WriteString(fmt.Sprintf("  %s %s\n", keywordStyle.Render("▸"), name))
	}
	b.WriteString("\n")

	// Changes
	b.WriteString(BoldStyle.Render("Changes"))
	b.WriteString("\n")
	for _, c := range result.Changes {
		if c.Field == "comment" {
			b.WriteString(fmt.Sprintf("  %s %s %s\n",
				c.Field,
				operatorStyle.Render("+"),
				c.NewValue))
		} else {
			b.WriteString(fmt.Sprintf("  %s%s %s %s %s\n",
				c.Field,
				operatorStyle.Render(":"),
				c.OldValue,
				operatorStyle.Render("→"),
				c.NewValue))
		}
	}

	return b.String()
}

// renderTaskListWithDetails renders the task list with inline details for narrow screens.
func (m *WorkflowDryRunModel) renderTaskListWithDetails(paneHeight int) string {
	if len(m.filtered) == 0 {
		if m.searchQuery != "" {
			return MutedStyle.Render("No tasks match your search.")
		}
		return MutedStyle.Render("No tasks affected.")
	}

	var b strings.Builder
	operatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("13"))

	maxVisible := paneHeight
	if maxVisible < 3 {
		maxVisible = 3
	}

	startIdx := 0
	if m.cursor >= maxVisible {
		startIdx = m.cursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
	}

	maxTitleLen := m.Width() - 8
	if maxTitleLen < 20 {
		maxTitleLen = 20
	}

	for i := startIdx; i < endIdx; i++ {
		result := m.filtered[i]
		title := result.Task.Title
		if len(title) > maxTitleLen {
			title = title[:maxTitleLen-3] + "..."
		}

		if i == m.cursor {
			b.WriteString(SelectedItemTextStyle.Render(fmt.Sprintf("▸ %s", title)))
			b.WriteString(" ")
			b.WriteString(MutedStyle.Render("[" + result.ProjectName + "]"))
			b.WriteString("\n")
			// Show rules and changes for selected task
			for _, name := range result.MatchedRules {
				b.WriteString(MutedStyle.Render(fmt.Sprintf("    Rule: %s", name)))
				b.WriteString("\n")
			}
			for _, c := range result.Changes {
				if c.Field == "comment" {
					b.WriteString(MutedStyle.Render(fmt.Sprintf("    %s %s %s",
						c.Field, operatorStyle.Render("+"), c.NewValue)))
				} else {
					b.WriteString(MutedStyle.Render(fmt.Sprintf("    %s%s %s %s %s",
						c.Field, operatorStyle.Render(":"), c.OldValue, operatorStyle.Render("→"), c.NewValue)))
				}
				b.WriteString("\n")
			}
		} else {
			b.WriteString(fmt.Sprintf("  %s", title))
			b.WriteString("\n")
		}
	}

	if len(m.filtered) > maxVisible {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  %d of %d tasks", m.cursor+1, len(m.filtered))))
	}

	return b.String()
}
