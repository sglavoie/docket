package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// renderPreview renders the preview pane for the selected item.
func (m *CommandPaletteModel) renderPreview() string {
	// In action mode, show action help
	if m.actionMode {
		return m.renderActionPreview()
	}

	// In inline task creation mode, show task preview
	if m.inlineTaskMode {
		return m.renderInlineTaskPreview()
	}

	// Check for help mode (query starts with ?)
	query := strings.TrimSpace(m.searchInput.Value())
	if strings.HasPrefix(query, "?") {
		return m.renderHelpReference()
	}

	if len(m.filteredItems) == 0 || m.cursor >= len(m.filteredItems) {
		return MutedStyle.Render("No item selected.")
	}

	filtered := m.filteredItems[m.cursor]
	item := filtered.Item

	var preview string
	switch item.Category {
	case ItemCategoryCommands:
		preview = m.renderCommandPreview(item)
	case ItemCategoryTasks:
		preview = m.renderTaskPreview(item)
	case ItemCategoryProjects:
		preview = m.renderProjectPreview(item)
	case ItemCategoryTags:
		preview = m.renderTagPreview(item)
	case ItemCategorySections:
		preview = m.renderSectionPreview(item)
	case ItemCategorySavedSearch:
		preview = m.renderSavedSearchPreview(item)
	case ItemCategorySavedView:
		preview = m.renderSavedViewPreview(item)
	case ItemCategoryNotes:
		preview = m.renderNotePreview(item)
	case ItemCategoryViews:
		preview = m.renderViewPreview(item)
	default:
		return MutedStyle.Render("Unknown item type.")
	}

	// Append match info if there's an active search with a non-name match
	if filtered.MatchedField != MatchFieldNone && filtered.MatchedField != MatchFieldName && filtered.MatchedTerm != "" {
		preview += "\n" + m.renderMatchInfo(filtered)
	}

	return preview
}

// renderMatchInfo shows why an item matched the search query.
func (m *CommandPaletteModel) renderMatchInfo(filtered filteredPaletteItem) string {
	var b strings.Builder

	b.WriteString(MutedStyle.Render("─────────────────────"))
	b.WriteString("\n")

	var fieldName string
	switch filtered.MatchedField {
	case MatchFieldDescription:
		fieldName = "description"
	case MatchFieldAlias:
		fieldName = "alias"
	case MatchFieldTag:
		fieldName = "tag"
	case MatchFieldSection:
		fieldName = "section"
	default:
		return ""
	}

	b.WriteString(MutedStyle.Render("Matched in "))
	b.WriteString(MatchHighlightStyle.Render(fieldName))
	b.WriteString(MutedStyle.Render(": "))
	b.WriteString(BoldStyle.Render("\"" + filtered.MatchedTerm + "\""))

	return b.String()
}

// renderHelpReference shows the command palette syntax reference.
func (m *CommandPaletteModel) renderHelpReference() string {
	var b strings.Builder

	b.WriteString(TaskJumperPreviewHeaderStyle.Render("Command Palette Help"))
	b.WriteString("\n\n")

	// Category prefixes
	b.WriteString(SubtitleStyle.Render("Category Prefixes"))
	b.WriteString("\n")
	b.WriteString("  :cmd    Commands only\n")
	b.WriteString("  :task   Tasks only\n")
	b.WriteString("  :prj    Projects only\n")
	b.WriteString("  :tag    Tags only\n")
	b.WriteString("  :sec    Sections only\n")
	b.WriteString("  :saved  Saved searches\n")
	b.WriteString("  :sv     Saved views\n")
	b.WriteString("  :note   Notes only\n")
	b.WriteString("  :view   Views only\n")
	b.WriteString("  :recent Recent commands\n")
	b.WriteString("\n")

	// Task filters
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

	// Search syntax
	b.WriteString(SubtitleStyle.Render("Search Syntax"))
	b.WriteString("\n")
	b.WriteString("  term1 term2    AND (both match)\n")
	b.WriteString("  term1|term2    OR (either match)\n")
	b.WriteString("  -term          Exclude term\n")
	b.WriteString("  \"exact\"        Exact match\n")
	b.WriteString("  #123           Jump to task ID\n")
	b.WriteString("\n")

	// Inline task creation
	b.WriteString(SubtitleStyle.Render("Create Task (+)"))
	b.WriteString("\n")
	b.WriteString("  +Title         Create task\n")
	b.WriteString("  +Title !2      With priority\n")
	b.WriteString("  +Title #bug    With tag\n")
	b.WriteString("  +Title @today  With due date\n")
	b.WriteString("\n")

	// Keyboard shortcuts
	b.WriteString(SubtitleStyle.Render("Task Shortcuts"))
	b.WriteString("\n")
	b.WriteString("  e=edit  g=view  x=done\n")
	b.WriteString("  s=start b=block r=reopen\n")
	b.WriteString("\n")

	// Multi-select
	b.WriteString(SubtitleStyle.Render("Multi-Select"))
	b.WriteString("\n")
	b.WriteString("  Space   Toggle selection\n")
	b.WriteString("  Tab     Bulk actions\n")
	b.WriteString("\n")

	// History navigation
	b.WriteString(SubtitleStyle.Render("History"))
	b.WriteString("\n")
	b.WriteString("  Alt+↑   Previous search\n")
	b.WriteString("  Alt+↓   Next search\n")
	b.WriteString("\n")

	// Pinning
	b.WriteString(SubtitleStyle.Render("Pinning"))
	b.WriteString("\n")
	b.WriteString("  Ctrl+F  Pin/unpin command\n")
	b.WriteString("  *       Pinned indicator\n")
	b.WriteString("\n")

	// Saved searches
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

	// Title
	if m.inlineTaskParsed.title != "" {
		b.WriteString(BoldStyle.Render(m.inlineTaskParsed.title))
	} else {
		b.WriteString(MutedStyle.Render("(no title)"))
	}
	b.WriteString("\n\n")

	// Priority
	priorityStyle := GetPriorityStyle(m.inlineTaskParsed.priority)
	fmt.Fprintf(&b, "%s %s\n",
		MutedStyle.Render("Priority:"),
		priorityStyle.Render(fmt.Sprintf("%d", m.inlineTaskParsed.priority)))

	// Tags
	if len(m.inlineTaskParsed.tags) > 0 {
		tags := make([]string, len(m.inlineTaskParsed.tags))
		for i, tag := range m.inlineTaskParsed.tags {
			tags[i] = TagInlineStyle.Render("#" + tag)
		}
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Tags:"),
			strings.Join(tags, " "))
	}

	// Due date
	if m.inlineTaskParsed.dueDate != nil {
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Due:"),
			DueDateStyle.Render(m.inlineTaskParsed.dueDate.Format(util.DateFormat)))
	}

	// Syntax help
	b.WriteString("\n")
	b.WriteString(SubtitleStyle.Render("Syntax"))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("!1-5 priority  #tag  @today/@tomorrow/@YYYY-MM-DD"))
	b.WriteString("\n\n")

	// Action hint
	if m.inlineTaskParsed.title != "" {
		b.WriteString(HelpStyle.Render("Press Enter to create task"))
	}

	return b.String()
}

// renderCommandPreview shows details for a command.
func (m *CommandPaletteModel) renderCommandPreview(item PaletteItem) string {
	if item.Command == nil {
		return MutedStyle.Render("No command details.")
	}
	cmd := item.Command
	var b strings.Builder

	// Header
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(cmd.Name))
	b.WriteString("\n\n")

	// Category
	b.WriteString(MutedStyle.Render("Category: "))
	b.WriteString(string(cmd.Category))
	b.WriteString("\n")

	// Shortcut
	if cmd.Shortcut != "" {
		b.WriteString(MutedStyle.Render("Shortcut: "))
		b.WriteString(HelpKeyStyle.Render(cmd.Shortcut))
		b.WriteString("\n")
	}

	// Description
	if cmd.Description != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Description"))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Width(m.previewWidth - 2).Render(cmd.Description))
		b.WriteString("\n")
	}

	// Aliases
	if len(cmd.Aliases) > 0 {
		b.WriteString("\n")
		aliasText := "Also searchable as: " + strings.Join(cmd.Aliases, ", ")
		b.WriteString(lipgloss.NewStyle().Width(m.previewWidth - 2).Render(MutedStyle.Render(aliasText)))
		b.WriteString("\n")
	}

	return b.String()
}

// renderTaskPreview shows details for a task.
func (m *CommandPaletteModel) renderTaskPreview(item PaletteItem) string {
	if item.Task == nil {
		return MutedStyle.Render("No task details.")
	}
	task := item.Task
	var b strings.Builder

	// Project header if available
	if item.Project != nil {
		b.WriteString(MutedStyle.Render("Project: "))
		b.WriteString(BoldStyle.Render(item.Project.Name))
		b.WriteString("\n\n")
	}

	// Header with type and title
	typeStyle := GetTypeStyle(string(task.Type))
	header := fmt.Sprintf("%s #%d: %s",
		typeStyle.Render(task.Type.HeadingPrefix()),
		task.ShortID,
		task.Title)
	styledHeader := TaskJumperPreviewHeaderStyle.Render(header)
	b.WriteString(lipgloss.NewStyle().Width(m.previewWidth - 2).Render(styledHeader))
	b.WriteString("\n\n")

	// Status
	statusStyle := GetStatusStyle(string(task.Status))
	fmt.Fprintf(&b, "%s %s %s\n",
		MutedStyle.Render("Status:"),
		statusStyle.Render(task.Status.Icon()),
		statusStyle.Render(string(task.Status)))

	// Priority
	priorityStyle := GetPriorityStyle(task.Priority)
	fmt.Fprintf(&b, "%s %s\n",
		MutedStyle.Render("Priority:"),
		priorityStyle.Render(fmt.Sprintf("%d", task.Priority)))

	// Tags
	if len(task.Tags) > 0 {
		tags := make([]string, len(task.Tags))
		for i, tag := range task.Tags {
			tags[i] = TagInlineStyle.Render(tag)
		}
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Tags:"),
			strings.Join(tags, ", "))
	}

	// Section
	if task.Section != "" {
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Section:"),
			task.Section)
	}

	// Due date
	if task.DueDate != nil {
		dueStyle := DueDateStyle
		now := time.Now()
		today := util.StartOfDay(now)
		dueDay := util.StartOfDay(*task.DueDate)
		if dueDay.Before(today) {
			dueStyle = OverdueStyle
		} else if dueDay.Equal(today) {
			dueStyle = DueTodayStyle
		}
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Due:"),
			dueStyle.Render(task.DueDate.Format(util.DateFormat)))
	}

	// Parent reference
	if task.Parent != nil {
		parentRef := fmt.Sprintf("#%d: %s", task.Parent.ShortID, task.Parent.Title)
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Parent:"),
			MutedStyle.Render(parentRef))
	}

	// Subtasks count
	if len(task.Children) > 0 {
		completed := 0
		for _, child := range task.Children {
			if child.Status.IsComplete() {
				completed++
			}
		}
		fmt.Fprintf(&b, "%s %d/%d done\n",
			MutedStyle.Render("Subtasks:"),
			completed, len(task.Children))
	}

	// Dependencies - show blocking and blocked-by relationships
	if item.Project != nil {
		// Tasks this task depends on (blockers)
		blockers := item.Project.GetBlockingTasks(task)
		if len(blockers) > 0 {
			fmt.Fprintf(&b, "%s ", MutedStyle.Render("Blocked by:"))
			maxShow := 3
			if len(blockers) < maxShow {
				maxShow = len(blockers)
			}
			var refs []string
			for i := 0; i < maxShow; i++ {
				blocker := blockers[i]
				statusStyle := GetStatusStyle(string(blocker.Status))
				ref := fmt.Sprintf("%s #%d", statusStyle.Render(blocker.Status.Icon()), blocker.ShortID)
				refs = append(refs, ref)
			}
			b.WriteString(strings.Join(refs, ", "))
			if len(blockers) > maxShow {
				b.WriteString(MutedStyle.Render(fmt.Sprintf(" +%d more", len(blockers)-maxShow)))
			}
			b.WriteString("\n")
		}

		// Tasks that depend on this task (blocked by this)
		blocked := item.Project.GetBlockedByTasks(task)
		if len(blocked) > 0 {
			fmt.Fprintf(&b, "%s ", MutedStyle.Render("Blocking:"))
			maxShow := 3
			if len(blocked) < maxShow {
				maxShow = len(blocked)
			}
			var refs []string
			for i := 0; i < maxShow; i++ {
				dep := blocked[i]
				statusStyle := GetStatusStyle(string(dep.Status))
				ref := fmt.Sprintf("%s #%d", statusStyle.Render(dep.Status.Icon()), dep.ShortID)
				refs = append(refs, ref)
			}
			b.WriteString(strings.Join(refs, ", "))
			if len(blocked) > maxShow {
				b.WriteString(MutedStyle.Render(fmt.Sprintf(" +%d more", len(blocked)-maxShow)))
			}
			b.WriteString("\n")
		}
	}

	// Description
	if task.Description != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Description"))
		b.WriteString("\n")
		b.WriteString(renderMarkdown(task.Description, m.previewWidth-2))
		b.WriteString("\n")
	}

	// Quick actions hint (when not actively searching)
	if m.isDirectActionEnabled() {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Quick Actions"))
		b.WriteString("\n")
		var actions []string
		actions = append(actions, HelpKeyStyle.Render("e")+" edit")
		actions = append(actions, HelpKeyStyle.Render("g")+" view")
		if task.Status != models.StatusDone && task.Status != models.StatusCanceled {
			actions = append(actions, HelpKeyStyle.Render("x")+" done")
		}
		if task.Status == models.StatusTodo {
			actions = append(actions, HelpKeyStyle.Render("s")+" start")
		}
		if task.Status == models.StatusDone || task.Status == models.StatusCanceled {
			actions = append(actions, HelpKeyStyle.Render("r")+" reopen")
		}
		b.WriteString(MutedStyle.Render(strings.Join(actions, "  ")))
		b.WriteString("\n")
	}

	return b.String()
}

// renderProjectPreview shows details for a project.
func (m *CommandPaletteModel) renderProjectPreview(item PaletteItem) string {
	if item.Project == nil {
		return MutedStyle.Render("No project details.")
	}
	project := item.Project
	var b strings.Builder

	// Header
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(project.Name))
	b.WriteString("\n\n")

	// Slug
	b.WriteString(MutedStyle.Render("Slug: "))
	b.WriteString(project.Slug)
	b.WriteString("\n")

	// Context
	if project.Context != "" {
		b.WriteString(MutedStyle.Render("Context: "))
		b.WriteString(project.Context)
		b.WriteString("\n")
	}

	// Description
	if project.Description != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Description"))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Width(m.previewWidth - 2).Render(project.Description))
		b.WriteString("\n")
	}

	// Task counts and health (if loaded)
	if len(project.Tasks) > 0 {
		now := time.Now()
		today := util.StartOfDay(now)
		weekFromNow := today.AddDate(0, 0, 7)

		active, completed := 0, 0
		overdue, dueToday, dueThisWeek := 0, 0, 0
		highPriority, blocked := 0, 0

		for _, task := range project.Tasks {
			if task.Status.IsComplete() {
				completed++
				continue
			}
			active++

			// Count high priority (1-2)
			if task.Priority <= 2 {
				highPriority++
			}

			// Count blocked tasks
			if task.Status == models.StatusBlocked {
				blocked++
			}

			// Count by due date
			if task.DueDate != nil {
				dueDay := util.StartOfDay(*task.DueDate)
				if dueDay.Before(today) {
					overdue++
				} else if dueDay.Equal(today) {
					dueToday++
				} else if dueDay.Before(weekFromNow) {
					dueThisWeek++
				}
			}
		}

		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Task Health"))
		b.WriteString("\n")

		// Show overdue in red if any
		if overdue > 0 {
			fmt.Fprintf(&b, "  %s %d overdue\n",
				OverdueStyle.Render("!"),
				overdue)
		}

		// Show due today in yellow if any
		if dueToday > 0 {
			fmt.Fprintf(&b, "  %s %d due today\n",
				DueTodayStyle.Render("!"),
				dueToday)
		}

		// Show due this week
		if dueThisWeek > 0 {
			fmt.Fprintf(&b, "  %s %d due this week\n",
				WarningIconStyle.Render("~"),
				dueThisWeek)
		}

		// Show high priority count
		if highPriority > 0 {
			fmt.Fprintf(&b, "  %s %d high priority\n",
				WarningIconStyle.Render("!"),
				highPriority)
		}

		// Show blocked count
		if blocked > 0 {
			fmt.Fprintf(&b, "  %s %d blocked\n",
				ErrorIconStyle.Render("◈"),
				blocked)
		}

		// Show if no urgent items
		if overdue == 0 && dueToday == 0 && blocked == 0 {
			fmt.Fprintf(&b, "  %s No urgent items\n",
				CheckmarkStyle.Render("✓"))
		}

		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("Tasks: "))
		fmt.Fprintf(&b, "%d active, %d completed", active, completed)
		b.WriteString("\n")
	}

	return b.String()
}

// renderTagPreview shows tasks with this tag.
func (m *CommandPaletteModel) renderTagPreview(item PaletteItem) string {
	if item.Tag == "" {
		return MutedStyle.Render("No tag details.")
	}
	var b strings.Builder

	// Header
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(CategoryTagStyle.Render("#" + item.Tag)))
	b.WriteString("\n\n")

	// Find tasks with this tag
	var tasksWithTag []PaletteItem
	for _, fi := range m.filteredItems {
		if fi.Item.Category == ItemCategoryTasks && fi.Item.Task != nil {
			for _, tag := range fi.Item.Task.Tags {
				if tag == item.Tag {
					tasksWithTag = append(tasksWithTag, fi.Item)
					break
				}
			}
		}
	}

	if len(tasksWithTag) == 0 {
		b.WriteString(MutedStyle.Render("No active tasks with this tag."))
		return b.String()
	}

	b.WriteString(SubtitleStyle.Render(fmt.Sprintf("Tasks (%d)", len(tasksWithTag))))
	b.WriteString("\n")

	// Show up to 5 tasks
	maxShow := 5
	if len(tasksWithTag) < maxShow {
		maxShow = len(tasksWithTag)
	}
	wrapStyle := lipgloss.NewStyle().Width(m.previewWidth - 2)
	for i := 0; i < maxShow; i++ {
		task := tasksWithTag[i].Task
		line := fmt.Sprintf("  %s #%d: %s",
			GetTypeBadge(string(task.Type), task.Type.Icon()),
			task.ShortID,
			task.Title)
		b.WriteString(wrapStyle.Render(line))
		b.WriteString("\n")
	}
	if len(tasksWithTag) > maxShow {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more", len(tasksWithTag)-maxShow)))
		b.WriteString("\n")
	}

	return b.String()
}

// renderSectionPreview shows tasks in this section.
func (m *CommandPaletteModel) renderSectionPreview(item PaletteItem) string {
	if item.Section == "" {
		return MutedStyle.Render("No section details.")
	}
	var b strings.Builder

	// Header
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(CategorySectionStyle.Render("§ " + item.Section)))
	b.WriteString("\n\n")

	// Find tasks in this section
	var tasksInSection []PaletteItem
	for _, fi := range m.filteredItems {
		if fi.Item.Category == ItemCategoryTasks && fi.Item.Task != nil {
			if fi.Item.Task.Section == item.Section {
				tasksInSection = append(tasksInSection, fi.Item)
			}
		}
	}

	if len(tasksInSection) == 0 {
		b.WriteString(MutedStyle.Render("No active tasks in this section."))
		return b.String()
	}

	b.WriteString(SubtitleStyle.Render(fmt.Sprintf("Tasks (%d)", len(tasksInSection))))
	b.WriteString("\n")

	// Show up to 5 tasks
	maxShow := 5
	if len(tasksInSection) < maxShow {
		maxShow = len(tasksInSection)
	}
	wrapStyle := lipgloss.NewStyle().Width(m.previewWidth - 2)
	for i := 0; i < maxShow; i++ {
		task := tasksInSection[i].Task
		line := fmt.Sprintf("  %s #%d: %s",
			GetTypeBadge(string(task.Type), task.Type.Icon()),
			task.ShortID,
			task.Title)
		b.WriteString(wrapStyle.Render(line))
		b.WriteString("\n")
	}
	if len(tasksInSection) > maxShow {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more", len(tasksInSection)-maxShow)))
		b.WriteString("\n")
	}

	return b.String()
}

// renderSavedSearchPreview shows details for a saved search.
func (m *CommandPaletteModel) renderSavedSearchPreview(item PaletteItem) string {
	if item.SavedSearch == "" {
		return MutedStyle.Render("No saved search details.")
	}
	var b strings.Builder

	// Header
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(CategorySavedSearchStyle.Render("★ " + item.Name)))
	b.WriteString("\n\n")

	// Query
	b.WriteString(SubtitleStyle.Render("Query"))
	b.WriteString("\n")
	b.WriteString(BoldStyle.Render(item.SavedSearch))
	b.WriteString("\n\n")

	// Parse and explain the query components
	b.WriteString(SubtitleStyle.Render("Components"))
	b.WriteString("\n")

	query := item.SavedSearch
	categoryFilter, searchQuery := m.parseQueryPrefix(query)

	if categoryFilter != "" {
		fmt.Fprintf(&b, "  Category: %s\n", getCategoryDisplayName(categoryFilter))
	}

	// Parse field filters
	remaining, filters := parseFieldFilters(searchQuery)
	for _, f := range filters {
		switch f.field {
		case "priority":
			if f.op == "range" {
				fmt.Fprintf(&b, "  Priority: %s\n", f.value)
			} else {
				fmt.Fprintf(&b, "  Priority: %s\n", f.value)
			}
		case "status":
			if f.op == "in" {
				fmt.Fprintf(&b, "  Status: %s\n", strings.Join(f.alt, " or "))
			} else {
				fmt.Fprintf(&b, "  Status: %s\n", f.value)
			}
		case "tag":
			if f.op == "in" {
				fmt.Fprintf(&b, "  Tag: %s\n", strings.Join(f.alt, " or "))
			} else {
				fmt.Fprintf(&b, "  Tag: %s\n", f.value)
			}
		case "section":
			if f.op == "in" {
				fmt.Fprintf(&b, "  Section: %s\n", strings.Join(f.alt, " or "))
			} else {
				fmt.Fprintf(&b, "  Section: %s\n", f.value)
			}
		case "date":
			fmt.Fprintf(&b, "  Due: %s\n", f.value)
		}
	}

	if remaining != "" {
		fmt.Fprintf(&b, "  Search: %s\n", remaining)
	}

	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("Press Enter to run · Tab for actions · d to delete"))

	return b.String()
}

// renderSavedViewPreview shows details for a saved view.
func (m *CommandPaletteModel) renderSavedViewPreview(item PaletteItem) string {
	if item.SavedView == nil {
		return MutedStyle.Render("No saved view details.")
	}
	sv := item.SavedView
	var b strings.Builder

	// Header
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(sv.Name))
	b.WriteString("\n\n")

	// Filter description
	desc := describeSavedViewFilters(
		sv.FilterType,
		sv.FilterStatus,
		sv.FilterPriority,
		sv.FilterTags,
		sv.FilterSections,
		sv.FilterDueDate,
		sv.FilterActionable,
		sv.FilterAttrs,
		sv.FilterScoreMin,
		sv.SortField,
		sv.ViewMode,
	)

	b.WriteString(SubtitleStyle.Render("Filters"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Width(m.previewWidth - 2).Render(desc))
	b.WriteString("\n\n")

	b.WriteString(HelpStyle.Render("Press Enter to apply · Tab for actions"))

	return b.String()
}

// renderNotePreview shows details for a note.
func (m *CommandPaletteModel) renderNotePreview(item PaletteItem) string {
	if item.Note == nil {
		return MutedStyle.Render("No note details.")
	}
	note := item.Note
	var b strings.Builder

	// Header
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(note.Title))
	b.WriteString("\n\n")

	// Project
	if item.Project != nil {
		b.WriteString(MutedStyle.Render("Project: "))
		b.WriteString(BoldStyle.Render(item.Project.Name))
		b.WriteString("\n")
	}

	// Tags
	if len(note.Tags) > 0 {
		tags := make([]string, len(note.Tags))
		for i, tag := range note.Tags {
			tags[i] = TagInlineStyle.Render("#" + tag)
		}
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Tags:"),
			strings.Join(tags, " "))
	}

	// Pinned
	if note.Pinned {
		b.WriteString(MutedStyle.Render("Pinned: "))
		b.WriteString("yes\n")
	}

	// Linked tasks
	if len(note.LinkedTasks) > 0 {
		fmt.Fprintf(&b, "%s %d\n",
			MutedStyle.Render("Linked tasks:"),
			len(note.LinkedTasks))
	}

	// Body excerpt
	if note.Body != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Content"))
		b.WriteString("\n")
		b.WriteString(renderMarkdown(strings.TrimSpace(note.Body), m.previewWidth-2))
		b.WriteString("\n")
	}

	return b.String()
}

// renderViewPreview shows details for a navigable view.
func (m *CommandPaletteModel) renderViewPreview(item PaletteItem) string {
	var b strings.Builder

	b.WriteString(TaskJumperPreviewHeaderStyle.Render(item.Name))
	b.WriteString("\n\n")

	if item.Description != "" {
		b.WriteString(SubtitleStyle.Render("Description"))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Width(m.previewWidth - 2).Render(item.Description))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("Press Enter to navigate"))

	return b.String()
}

// renderActionPreview shows help for the current action.
func (m *CommandPaletteModel) renderActionPreview() string {
	var b strings.Builder

	// Show item context
	b.WriteString(SubtitleStyle.Render("Acting on:"))
	b.WriteString("\n")
	b.WriteString(BoldStyle.Render(m.actionItem.Name))
	b.WriteString("\n")
	if m.actionItem.Description != "" {
		b.WriteString(MutedStyle.Render(m.actionItem.Description))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Show available actions with hints
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
