package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// renderSidebarDepSectionItems renders the scrolled list portion of a dependency section.
func renderSidebarDepSectionItems(sidebar *SidebarComponent, items []sidebarLinkedTask, sectionIndex, innerWidth int) []string {
	const maxVisiblePerSection = 5
	var lines []string

	sectionScroll := 0
	if sidebar.IsFocused() && sidebar.GetDependencySection() == sectionIndex {
		cursor := sidebar.GetDependencyCursor()
		sectionScroll = sidebar.GetDepsScrollOffset()
		if cursor >= sectionScroll+maxVisiblePerSection {
			sectionScroll = cursor - maxVisiblePerSection + 1
		}
		if cursor < sectionScroll {
			sectionScroll = cursor
		}
		if sectionScroll != sidebar.GetDepsScrollOffset() {
			sidebar.SetDepsScrollOffset(sectionScroll)
		}
	}

	if sectionScroll > 0 {
		lines = append(lines, "  "+ScrollUpIndicator(sectionScroll))
	}

	visibleEnd := min(sectionScroll+maxVisiblePerSection, len(items))
	for i := sectionScroll; i < visibleEnd; i++ {
		isCursor := sidebar.IsFocused() && sidebar.GetDependencySection() == sectionIndex && i == sidebar.GetDependencyCursor()
		lines = append(lines, formatSidebarLinkedTask(items[i], innerWidth, isCursor, sidebar.IsFocused()))
	}

	if visibleEnd < len(items) {
		lines = append(lines, "  "+ScrollDownIndicator(len(items)-visibleEnd))
	}

	return lines
}

// sidebarDepSectionHeader returns a styled section header, bold+underline when focused.
func sidebarDepSectionHeader(sidebar *SidebarComponent, label string, sectionIndex int) string {
	if sidebar.IsFocused() && sidebar.GetDependencySection() == sectionIndex {
		return BoldStyle.Underline(true).Render(label)
	}
	return MutedStyle.Render(label)
}

// renderSidebarDependenciesCard renders the dependencies card with navigation support.
func renderSidebarDependenciesCard(host SidebarHost, task *models.Task, width, innerWidth int) string {
	project := host.SidebarProject()
	cfg := host.SidebarConfig()
	sidebar := host.SidebarGetSidebar()
	var lines []string

	if len(task.DependsOn) > 0 {
		lines = append(lines, sidebarDepSectionHeader(sidebar, "Blocked by:", 0))

		var blockingTasks []sidebarLinkedTask
		for _, depRef := range task.DependsOn {
			blockingTasks = append(blockingTasks, sidebarResolveSidebarTaskRef(project, cfg, depRef))
		}
		lines = append(lines, renderSidebarDepSectionItems(sidebar, blockingTasks, 0, innerWidth)...)
	}

	blockedBy := project.GetBlockedByTasks(task)
	if len(blockedBy) > 0 {
		if len(task.DependsOn) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, sidebarDepSectionHeader(sidebar, "Blocking:", 1))

		var blockingItems []sidebarLinkedTask
		for _, blocked := range blockedBy {
			blockingItems = append(blockingItems, sidebarLinkedTask{Task: blocked, Ref: blocked.ID})
		}
		lines = append(lines, renderSidebarDepSectionItems(sidebar, blockingItems, 1, innerWidth)...)
	}

	focused := sidebar.IsFocused() && sidebarHasDependencies(project, task)
	return renderSidebarCard("Deps", strings.Join(lines, "\n"), width, focused, "")
}

// renderSidebarRelatedCard renders the related tasks card with navigation support.
func renderSidebarRelatedCard(host SidebarHost, task *models.Task, width, innerWidth int) string {
	project := host.SidebarProject()
	cfg := host.SidebarConfig()
	sidebar := host.SidebarGetSidebar()

	var relatedTasks []sidebarLinkedTask
	for _, relRef := range task.RelatedTo {
		relatedTasks = append(relatedTasks, sidebarResolveSidebarTaskRef(project, cfg, relRef))
	}
	if len(relatedTasks) == 0 {
		return ""
	}

	lines := []string{sidebarDepSectionHeader(sidebar, "Related:", 2)}
	lines = append(lines, renderSidebarDepSectionItems(sidebar, relatedTasks, 2, innerWidth)...)

	focused := sidebar.IsFocused() && sidebar.GetDependencySection() == 2
	return renderSidebarCard("Related", strings.Join(lines, "\n"), width, focused, "")
}

// sidebarResolveSidebarTaskRef resolves a task reference (local or cross-project) for sidebar display.
func sidebarResolveSidebarTaskRef(project *models.Project, cfg *config.Config, ref string) sidebarLinkedTask {
	taskRef := models.ParseTaskRef(ref)

	if !taskRef.IsCrossProject() {
		task := project.GetTaskByID(taskRef.TaskID)
		if task != nil {
			return sidebarLinkedTask{Task: task, Ref: ref}
		}
		return sidebarLinkedTask{Ref: ref, IsMissing: true}
	}

	if cfg == nil {
		return sidebarLinkedTask{Ref: ref, IsMissing: true}
	}

	targetProject := cfg.GetProject(taskRef.ProjectID)
	if targetProject == nil {
		return sidebarLinkedTask{Ref: ref, IsMissing: true}
	}

	if len(targetProject.Tasks) == 0 {
		if err := storage.LoadProject(targetProject); err != nil {
			logging.Warn("failed to load project %s: %v", targetProject.Name, err)
		}
	}

	task := targetProject.GetTaskByID(taskRef.TaskID)
	if task == nil {
		return sidebarLinkedTask{Project: targetProject, Ref: ref, IsMissing: true}
	}

	return sidebarLinkedTask{Task: task, Project: targetProject, Ref: ref}
}

// formatSidebarLinkedTask formats a linked task line for sidebar display.
func formatSidebarLinkedTask(linked sidebarLinkedTask, innerWidth int, cursor, isFocused bool) string {
	cursorStr := "  "
	if cursor {
		cursorStr = "> "
	}

	if linked.IsMissing {
		ref := models.ParseTaskRef(linked.Ref)
		label := "(missing)"
		if ref.IsCrossProject() {
			if linked.Project != nil {
				label = fmt.Sprintf("[%s] (missing)", linked.Project.Name)
			} else {
				label = fmt.Sprintf("[%s] (missing)", ref.ProjectID)
			}
		}
		line := cursorStr + ValidationErrorStyle.Render(label)
		if cursor && isFocused {
			line = BoldStyle.Render(line)
		}
		return line
	}

	depStatus := GetStatusStyle(string(linked.Task.Status))
	prefix := cursorStr
	if linked.Project != nil {
		projectName := truncateToVisualWidth(linked.Project.Name, max(innerWidth/3, 8))
		prefix += MutedStyle.Render(fmt.Sprintf("[%s] ", projectName))
	}
	prefix += fmt.Sprintf("#%-3d %s ", linked.Task.ShortID, depStatus.Render(linked.Task.Status.Icon()))

	titleWidth := max(innerWidth-lipgloss.Width(prefix), 1)
	if linked.Project != nil && titleWidth < 6 {
		projectName := truncateToVisualWidth(linked.Project.Name, max(innerWidth/4, 4))
		prefix = cursorStr + MutedStyle.Render(fmt.Sprintf("[%s] ", projectName)) + fmt.Sprintf("#%-3d %s ", linked.Task.ShortID, depStatus.Render(linked.Task.Status.Icon()))
		titleWidth = max(innerWidth-lipgloss.Width(prefix), 1)
	}

	line := prefix + truncateToVisualWidth(linked.Task.Title, titleWidth)
	if cursor && isFocused {
		line = BoldStyle.Render(line)
	}
	return line
}
