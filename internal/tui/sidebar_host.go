package tui

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// SidebarHost is the interface that any view must implement to use the shared
// sidebar rendering and key handling. It provides access to the project, config,
// sidebar state, selected task, and navigation/unlink operations.
type SidebarHost interface {
	SidebarProject() *models.Project
	SidebarConfig() *config.Config
	SidebarGetSidebar() *SidebarComponent
	SidebarSelectedTask() *models.Task
	SidebarHeight() int
	NavigateToSidebarTask(task *models.Task)
	UnlinkSelectedSidebarTask(task *models.Task) tea.Cmd
}

// sidebarUnlinkCommand builds the coordinator command that removes the linked
// item at (section, cursor) from sourceTask and adapts the transaction outcome
// back to a LinkRemovedMsg carrying the touched project IDs. Every relation kind
// routes through the link coordinator as one ledger-undoable ChangeSet: a
// blocked-by removal drops sourceTask's own dependency edge, a blocking removal
// drops the dependent task's edge onto sourceTask, and a related removal is
// bidirectional (removing both sides atomically, spanning projects when the peer
// lives elsewhere). A cursor out of range yields a nil command (nothing to
// remove); an error surfaces as ErrorMsg so the shell shows it without popping.
func sidebarUnlinkCommand(link LinkCoordinator, project *models.Project, sourceTask *models.Task, section, cursor int) tea.Cmd {
	var commit tea.Cmd
	switch section {
	case 0: // Blocked by: remove sourceTask's dependency edge
		if cursor < 0 || cursor >= len(sourceTask.DependsOn) {
			return nil
		}
		commit = link.RemoveDependency(sourceTask.ID, sourceTask.DependsOn[cursor])
	case 1: // Blocking: remove the dependent task's edge onto sourceTask
		blockedTasks := project.GetBlockedByTasks(sourceTask)
		if cursor < 0 || cursor >= len(blockedTasks) {
			return nil
		}
		commit = link.RemoveDependency(blockedTasks[cursor].ID, sourceTask.ID)
	case 2: // Related: bidirectional removal (may span projects)
		if cursor < 0 || cursor >= len(sourceTask.RelatedTo) {
			return nil
		}
		commit = link.RemoveRelatedBidirectional(sourceTask.ID, sourceTask.RelatedTo[cursor])
	default:
		return nil
	}
	if commit == nil {
		return nil
	}
	return func() tea.Msg {
		switch r := commit().(type) {
		case TransactionErrorMsg:
			return ErrorMsg{Err: r.Err}
		case TransactionResultMsg:
			return LinkRemovedMsg{SourceTask: sourceTask, Section: section, TouchedProjectIDs: []string{r.Result.ProjectID}}
		case BatchTransactionResultMsg:
			return LinkRemovedMsg{SourceTask: sourceTask, Section: section, TouchedProjectIDs: r.TouchedProjectIDs}
		default:
			return r
		}
	}
}

// sidebarLinkedTask represents a resolved task reference for sidebar display.
type sidebarLinkedTask struct {
	Task      *models.Task    // nil if not found (broken link)
	Project   *models.Project // nil for same-project links
	Ref       string          // Original reference string
	IsMissing bool            // True if the task/project couldn't be found
}

// sidebarLinkedTaskResult holds the result of resolving a sidebar linked task.
type sidebarLinkedTaskResult struct {
	Task      *models.Task
	Project   *models.Project // nil for same-project links
	TaskRef   string          // qualified ref for cross-project links
	IsMissing bool
}

// sidebarHasDependencies returns true if the task has any dependencies.
func sidebarHasDependencies(project *models.Project, task *models.Task) bool {
	return len(task.DependsOn) > 0 || len(project.GetBlockedByTasks(task)) > 0
}

// sidebarHasLinkedTasks returns true if the task has any linked tasks (dependencies or related).
func sidebarHasLinkedTasks(project *models.Project, task *models.Task) bool {
	return len(task.DependsOn) > 0 || len(project.GetBlockedByTasks(task)) > 0 || len(task.RelatedTo) > 0
}

// sidebarGetAvailableSections returns the non-empty section indices for a task.
// Section 0 = Blocked by, 1 = Blocking, 2 = Related
func sidebarGetAvailableSections(project *models.Project, task *models.Task) []int {
	var sections []int
	if len(task.DependsOn) > 0 {
		sections = append(sections, 0)
	}
	if len(project.GetBlockedByTasks(task)) > 0 {
		sections = append(sections, 1)
	}
	if len(task.RelatedTo) > 0 {
		sections = append(sections, 2)
	}
	return sections
}

// sidebarGetLinkedItemCount returns the count of items in the given linked section.
func sidebarGetLinkedItemCount(project *models.Project, sidebar *SidebarComponent, task *models.Task) int {
	switch sidebar.GetDependencySection() {
	case 0:
		return len(task.DependsOn)
	case 1:
		return len(project.GetBlockedByTasks(task))
	case 2:
		return len(task.RelatedTo)
	}
	return 0
}

// sidebarAdjustAfterLinkRemoval re-validates the sidebar cursor after a link
// has been removed. If no linked sections remain the sidebar is unfocused;
// otherwise the current section is switched (if empty) and the cursor is
// clamped to the valid range.
func sidebarAdjustAfterLinkRemoval(project *models.Project, sidebar *SidebarComponent, task *models.Task) {
	sections := sidebarGetAvailableSections(project, task)
	if len(sections) == 0 {
		sidebar.SetFocused(false)
		return
	}
	currentSection := sidebar.GetDependencySection()
	if !slices.Contains(sections, currentSection) {
		sidebar.SetDependencySection(sections[0])
	}
	count := sidebarGetLinkedItemCount(project, sidebar, task)
	if sidebar.GetDependencyCursor() >= count {
		if count > 0 {
			sidebar.SetDependencyCursor(count - 1)
		} else {
			sidebar.SetDependencyCursor(0)
		}
	}
}

// sidebarResolveTaskRefWithProject resolves a task reference and returns the task with its project.
// It delegates to sidebarResolveSidebarTaskRef for the actual resolution logic.
func sidebarResolveTaskRefWithProject(project *models.Project, cfg *config.Config, ref string) sidebarLinkedTaskResult {
	linked := sidebarResolveSidebarTaskRef(project, cfg, ref)
	result := sidebarLinkedTaskResult{
		Task:      linked.Task,
		Project:   linked.Project,
		IsMissing: linked.IsMissing,
	}
	if linked.Project != nil && linked.Task != nil {
		result.TaskRef = ref
	}
	return result
}

// sidebarGetSelectedLinkedTaskWithProject returns the currently selected linked task with its project.
func sidebarGetSelectedLinkedTaskWithProject(project *models.Project, cfg *config.Config, sidebar *SidebarComponent, task *models.Task) sidebarLinkedTaskResult {
	cursor := sidebar.GetDependencyCursor()

	switch sidebar.GetDependencySection() {
	case 0: // Blocked by (DependsOn refs)
		if cursor >= 0 && cursor < len(task.DependsOn) {
			ref := task.DependsOn[cursor]
			return sidebarResolveTaskRefWithProject(project, cfg, ref)
		}
	case 1: // Blocking (tasks that depend on this - same project only)
		blockedBy := project.GetBlockedByTasks(task)
		if cursor >= 0 && cursor < len(blockedBy) {
			return sidebarLinkedTaskResult{Task: blockedBy[cursor], Project: nil}
		}
	case 2: // Related
		if cursor >= 0 && cursor < len(task.RelatedTo) {
			ref := task.RelatedTo[cursor]
			return sidebarResolveTaskRefWithProject(project, cfg, ref)
		}
	}
	return sidebarLinkedTaskResult{IsMissing: true}
}
