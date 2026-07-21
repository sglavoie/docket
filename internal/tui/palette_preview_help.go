package tui

import (
	"fmt"
	"strings"
)

// renderHelpReference shows the command palette syntax reference.
func (m *CommandPaletteModel) renderHelpReference() string {
	var b strings.Builder

	b.WriteString(TaskJumperPreviewHeaderStyle.Render("Command Palette Help"))
	b.WriteString("\n\n")

	b.WriteString(SubtitleStyle.Render("Category Prefixes"))
	b.WriteString("\n")
	b.WriteString("  :cmd    Commands only\n")
	b.WriteString("  :task   Tasks only\n")
	b.WriteString("  :prj    Projects only\n")
	b.WriteString("  :tag    Tags only\n")
	b.WriteString("  :sec    Sections only\n")
	b.WriteString("  :sv     Saved filters\n")
	b.WriteString("  :note   Notes only\n")
	b.WriteString("  :view   Views only\n")
	b.WriteString("  :recent Recent commands\n")
	b.WriteString("\n")

	b.WriteString(SubtitleStyle.Render("Task Filters"))
	b.WriteString("\n")
	b.WriteString("  priority:1     Exact priority\n")
	b.WriteString("  priority:1-3   Priority range\n")
	b.WriteString("  status:todo    Exact status\n")
	b.WriteString("  status:a|b     Multiple statuses\n")
	b.WriteString("  tag:bug        Has tag\n")
	b.WriteString("  tag:a|b        Has any tag\n")
	b.WriteString("  section:name   In section\n")
	b.WriteString("  @overdue       Overdue tasks\n")
	b.WriteString("  @today         Due today\n")
	b.WriteString("  @tomorrow      Due tomorrow\n")
	b.WriteString("  @this-week     Due this week\n")
	b.WriteString("\n")

	b.WriteString(SubtitleStyle.Render("Search Syntax"))
	b.WriteString("\n")
	b.WriteString("  term1 term2    AND (both match)\n")
	b.WriteString("  term1|term2    OR (either match)\n")
	b.WriteString("  -term          Exclude term\n")
	b.WriteString("  \"exact\"        Exact match\n")
	b.WriteString("  #123           Find task by ID\n")
	b.WriteString("\n")

	b.WriteString(SubtitleStyle.Render("Multi-Select"))
	b.WriteString("\n")
	b.WriteString("  Space   Toggle selection\n")
	b.WriteString("  Tab     Bulk actions\n")

	return b.String()
}

// renderActionPreview shows help for the current action.
func (m *CommandPaletteModel) renderActionPreview() string {
	var b strings.Builder

	b.WriteString(SubtitleStyle.Render("Acting on:"))
	b.WriteString("\n")
	b.WriteString(BoldStyle.Render(m.actionItem.Name))
	b.WriteString("\n")
	if m.actionItem.Description != "" {
		b.WriteString(MutedStyle.Render(m.actionItem.Description))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString(SubtitleStyle.Render("Available Actions:"))
	b.WriteString("\n")
	for _, action := range m.availableActions {
		hint := ""
		if action.KeyHint != "" {
			hint = fmt.Sprintf(" [%s]", action.KeyHint)
		}
		fmt.Fprintf(&b, "  %s %s%s\n",
			action.Icon,
			action.Name,
			MutedStyle.Render(hint))
	}

	return b.String()
}
