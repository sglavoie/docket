package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/util"
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
	b.WriteString("  :saved  Saved searches\n")
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
	b.WriteString("  #123           Jump to task ID\n")
	b.WriteString("\n")

	b.WriteString(SubtitleStyle.Render("Create Task (+)"))
	b.WriteString("\n")
	b.WriteString("  +Title         Create task\n")
	b.WriteString("  +Title !2      With priority\n")
	b.WriteString("  +Title #bug    With tag\n")
	b.WriteString("  +Title @today  With due date\n")
	b.WriteString("\n")

	b.WriteString(SubtitleStyle.Render("Task Shortcuts"))
	b.WriteString("\n")
	b.WriteString("  e=edit  g=view  x=done\n")
	b.WriteString("  s=start b=block r=reopen\n")
	b.WriteString("\n")

	b.WriteString(SubtitleStyle.Render("Multi-Select"))
	b.WriteString("\n")
	b.WriteString("  Space   Toggle selection\n")
	b.WriteString("  Tab     Bulk actions\n")
	b.WriteString("\n")

	b.WriteString(SubtitleStyle.Render("History"))
	b.WriteString("\n")
	b.WriteString("  Alt+↑   Previous search\n")
	b.WriteString("  Alt+↓   Next search\n")
	b.WriteString("\n")

	b.WriteString(SubtitleStyle.Render("Pinning"))
	b.WriteString("\n")
	b.WriteString("  Ctrl+F  Pin/unpin command\n")
	b.WriteString("  *       Pinned indicator\n")
	b.WriteString("\n")

	b.WriteString(SubtitleStyle.Render("Saved Searches"))
	b.WriteString("\n")
	b.WriteString("  Ctrl+B  Save current search\n")
	b.WriteString("  :saved  Browse saved searches\n")

	return b.String()
}

// renderInlineTaskPreview shows a preview of the task being created.
func (m *CommandPaletteModel) renderInlineTaskPreview() string {
	var b strings.Builder

	b.WriteString(TaskJumperPreviewHeaderStyle.Render("New Task"))
	b.WriteString("\n\n")

	if m.inlineTaskParsed == nil {
		b.WriteString(MutedStyle.Render("Type a task title after +"))
		return b.String()
	}

	if m.inlineTaskParsed.title != "" {
		b.WriteString(BoldStyle.Render(m.inlineTaskParsed.title))
	} else {
		b.WriteString(MutedStyle.Render("(no title)"))
	}
	b.WriteString("\n\n")

	priorityStyle := GetPriorityStyle(m.inlineTaskParsed.priority)
	fmt.Fprintf(&b, "%s %s\n",
		MutedStyle.Render("Priority:"),
		priorityStyle.Render(fmt.Sprintf("%d", m.inlineTaskParsed.priority)))

	if len(m.inlineTaskParsed.tags) > 0 {
		tags := make([]string, len(m.inlineTaskParsed.tags))
		for i, tag := range m.inlineTaskParsed.tags {
			tags[i] = TagInlineStyle.Render("#" + tag)
		}
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Tags:"),
			strings.Join(tags, " "))
	}

	if m.inlineTaskParsed.dueDate != nil {
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Due:"),
			DueDateStyle.Render(m.inlineTaskParsed.dueDate.Format(util.DateFormat)))
	}

	b.WriteString("\n")
	b.WriteString(SubtitleStyle.Render("Syntax"))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("!1-5 priority  #tag  @today/@tomorrow/@YYYY-MM-DD"))
	b.WriteString("\n\n")

	if m.inlineTaskParsed.title != "" {
		b.WriteString(HelpStyle.Render("Press Enter to create task"))
	}

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
