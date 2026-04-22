package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// renderCommandPreview shows details for a command.
func (m *CommandPaletteModel) renderCommandPreview(item PaletteItem) string {
	if item.Command == nil {
		return MutedStyle.Render("No command details.")
	}
	cmd := item.Command
	var b strings.Builder

	b.WriteString(TaskJumperPreviewHeaderStyle.Render(cmd.Name))
	b.WriteString("\n\n")

	b.WriteString(MutedStyle.Render("Category: "))
	b.WriteString(string(cmd.Category))
	b.WriteString("\n")

	if cmd.Shortcut != "" {
		b.WriteString(MutedStyle.Render("Shortcut: "))
		b.WriteString(HelpKeyStyle.Render(cmd.Shortcut))
		b.WriteString("\n")
	}

	if cmd.Description != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Description"))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Width(max(m.previewWidth-2, 1)).Render(cmd.Description))
		b.WriteString("\n")
	}

	if len(cmd.Aliases) > 0 {
		b.WriteString("\n")
		aliasText := "Also searchable as: " + strings.Join(cmd.Aliases, ", ")
		b.WriteString(lipgloss.NewStyle().Width(max(m.previewWidth-2, 1)).Render(MutedStyle.Render(aliasText)))
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

	if item.Project != nil {
		b.WriteString(MutedStyle.Render("Project: "))
		b.WriteString(BoldStyle.Render(item.Project.Name))
		b.WriteString("\n\n")
	}

	typeStyle := GetTypeStyle(string(task.Type))
	header := fmt.Sprintf("%s #%d: %s",
		typeStyle.Render(task.Type.HeadingPrefix()),
		task.ShortID,
		GetTitlePriorityStyle(task.Priority).Render(task.Title))
	styledHeader := TaskJumperPreviewHeaderStyle.Render(header)
	b.WriteString(lipgloss.NewStyle().Width(max(m.previewWidth-2, 1)).Render(styledHeader))
	b.WriteString("\n\n")

	statusStyle := GetStatusStyle(string(task.Status))
	fmt.Fprintf(&b, "%s %s %s\n",
		MutedStyle.Render("Status:"),
		statusStyle.Render(task.Status.Icon()),
		statusStyle.Render(string(task.Status)))

	priorityStyle := GetPriorityStyle(task.Priority)
	fmt.Fprintf(&b, "%s %s\n",
		MutedStyle.Render("Priority:"),
		priorityStyle.Render(fmt.Sprintf("%d", task.Priority)))

	if len(task.Tags) > 0 {
		tags := make([]string, len(task.Tags))
		for i, tag := range task.Tags {
			tags[i] = TagInlineStyle.Render(tag)
		}
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Tags:"),
			strings.Join(tags, ", "))
	}

	if task.Section != "" {
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Section:"),
			task.Section)
	}

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
		dateStr := task.DueDate.Format(util.DateFormat)
		dateStr = AppendTimeIfHasTime(task, dateStr, "15:04")
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Due:"),
			dueStyle.Render(dateStr))
	}

	if task.Parent != nil {
		parentRef := fmt.Sprintf("#%d: %s", task.Parent.ShortID, task.Parent.Title)
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Parent:"),
			MutedStyle.Render(parentRef))
	}

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

	if item.Project != nil {
		m.writeTaskDependencyPreview(&b, item.Project, task)
	}

	if task.Description != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Description"))
		b.WriteString("\n")
		b.WriteString(renderMarkdown(task.Description, m.previewWidth-2))
		b.WriteString("\n")
	}

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

func (m *CommandPaletteModel) writeTaskDependencyPreview(b *strings.Builder, project *models.Project, task *models.Task) {
	blockers := project.GetBlockingTasks(task)
	if len(blockers) > 0 {
		fmt.Fprintf(b, "%s ", MutedStyle.Render("Blocked by:"))
		maxShow := min(len(blockers), 3)
		var refs []string
		for i := range maxShow {
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

	blocked := project.GetBlockedByTasks(task)
	if len(blocked) > 0 {
		fmt.Fprintf(b, "%s ", MutedStyle.Render("Blocking:"))
		maxShow := min(len(blocked), 3)
		var refs []string
		for i := range maxShow {
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

// renderProjectPreview shows details for a project.
func (m *CommandPaletteModel) renderProjectPreview(item PaletteItem) string {
	if item.Project == nil {
		return MutedStyle.Render("No project details.")
	}
	project := item.Project
	var b strings.Builder

	b.WriteString(TaskJumperPreviewHeaderStyle.Render(project.Name))
	b.WriteString("\n\n")

	b.WriteString(MutedStyle.Render("Slug: "))
	b.WriteString(project.Slug)
	b.WriteString("\n")

	if project.Context != "" {
		b.WriteString(MutedStyle.Render("Context: "))
		b.WriteString(project.Context)
		b.WriteString("\n")
	}

	if project.Description != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Description"))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Width(max(m.previewWidth-2, 1)).Render(project.Description))
		b.WriteString("\n")
	}

	if len(project.Tasks) > 0 {
		m.writeProjectHealthPreview(&b, project)
	}

	return b.String()
}

func (m *CommandPaletteModel) writeProjectHealthPreview(b *strings.Builder, project *models.Project) {
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

		if task.Priority <= 2 {
			highPriority++
		}
		if task.Status == models.StatusBlocked {
			blocked++
		}
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

	if overdue > 0 {
		fmt.Fprintf(b, "  %s %d overdue\n", OverdueStyle.Render("!"), overdue)
	}
	if dueToday > 0 {
		fmt.Fprintf(b, "  %s %d due today\n", DueTodayStyle.Render("!"), dueToday)
	}
	if dueThisWeek > 0 {
		fmt.Fprintf(b, "  %s %d due this week\n", WarningIconStyle.Render("~"), dueThisWeek)
	}
	if highPriority > 0 {
		fmt.Fprintf(b, "  %s %d high priority\n", WarningIconStyle.Render("!"), highPriority)
	}
	if blocked > 0 {
		fmt.Fprintf(b, "  %s %d blocked\n", ErrorIconStyle.Render("◈"), blocked)
	}
	if overdue == 0 && dueToday == 0 && blocked == 0 {
		fmt.Fprintf(b, "  %s No urgent items\n", CheckmarkStyle.Render("✓"))
	}

	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("Tasks: "))
	fmt.Fprintf(b, "%d active, %d completed", active, completed)
	b.WriteString("\n")
}
