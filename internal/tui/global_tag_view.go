package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// View renders the global tag manager view.
func (m *GlobalTagManagerModel) View() tea.View {
	var b strings.Builder

	if m.showProjectPicker {
		return tea.NewView(m.renderProjectPicker())
	}

	b.WriteString(renderFullscreenTitle("Global Tag Manager", "across all projects"))
	b.WriteString("\n\n")

	if len(m.tags) == 0 {
		b.WriteString(MutedStyle.Render("No tags found in any project"))
		b.WriteString("\n")
	} else {
		for i, tag := range m.tags {
			cursor := "  "
			style := TagManagerItemStyle
			if i == m.cursor {
				cursor = "  "
				style = TagManagerItemSelectedStyle
			}

			if m.renameMode && tag.Name == m.renameTarget {
				b.WriteString(cursor)
				b.WriteString("Rename to: ")
				b.WriteString(m.renameInput.View())
				b.WriteString("\n")
			} else {
				line := fmt.Sprintf("%s%s ", cursor, tag.Name)
				b.WriteString(style.Render(line))
				countStr := fmt.Sprintf("(%d)", tag.TaskCount)
				b.WriteString(TagManagerCountStyle.Render(countStr))
				projectStr := fmt.Sprintf(" [%d project", len(tag.Projects))
				if len(tag.Projects) != 1 {
					projectStr += "s"
				}
				projectStr += "]"
				b.WriteString(MutedStyle.Render(projectStr))
				b.WriteString("\n")
			}
		}
	}

	if m.errorMsg != "" {
		b.WriteString("\n")
		b.WriteString(TagManagerErrorStyle.Render(m.errorMsg))
		b.WriteString("\n")
	}

	var footerHelp string
	if m.renameMode {
		footerHelp = "enter confirm · esc cancel"
	} else if len(m.tags) > 0 {
		footerHelp = "j/k nav · enter filter · r rename · Esc close"
	} else {
		footerHelp = "Esc close"
	}
	b.WriteString(renderStatusBarFooter(HelpStyle.Render(footerHelp), "", m.width))

	return tea.NewView(b.String())
}

// renderProjectPicker renders the project picker overlay.
func (m *GlobalTagManagerModel) renderProjectPicker() string {
	var b strings.Builder

	var projects []TagProjectUsage
	for _, tag := range m.tags {
		if tag.Name == m.selectedTag {
			projects = tag.Projects
			break
		}
	}

	b.WriteString(renderFullscreenTitle("Select Project", fmt.Sprintf("for tag \"%s\"", m.selectedTag)))
	b.WriteString("\n\n")

	for i, usage := range projects {
		cursor := "  "
		style := TagManagerItemStyle
		if i == m.projectCursor {
			cursor = "  "
			style = TagManagerItemSelectedStyle
		}

		line := fmt.Sprintf("%s%s ", cursor, usage.Project.Name)
		b.WriteString(style.Render(line))
		countStr := fmt.Sprintf("(%d tasks)", usage.TaskCount)
		b.WriteString(TagManagerCountStyle.Render(countStr))
		b.WriteString("\n")
	}

	b.WriteString(renderStatusBarFooter(HelpStyle.Render("j/k nav · enter select · esc back"), "", m.width))

	return b.String()
}
