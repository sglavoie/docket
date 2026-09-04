package projecthub

import (
	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
)

// refreshProjectHubEmptyStates keeps the empty copy naming whichever narrowing
// emptied the list. The Ctrl+T filter runs at item-build time, so a list it
// empties is source-empty rather than unmatched, and the stock "create a
// project" copy would be a lie there.
func (m *ProjectHubModel) refreshProjectHubEmptyStates() {
	if m.surface == nil {
		return
	}
	m.surface.SetEmptyStates(m.projectHubEmptyState(), m.projectHubNoMatchState())
}

func (m *ProjectHubModel) projectHubEmptyState() listsurface.ListEmptyState {
	if m.mode == route.ProjectHubModePinned && m.pinnedActiveProjectCount() == 0 {
		return listsurface.ListEmptyState{
			Title:  "No active projects are pinned.",
			Detail: "Press Esc to return to the Outliner; P opens the full Project Hub.",
		}
	}
	switch m.taskFilter {
	case projectHubTaskFilterWithOpenTasks:
		if m.mode == route.ProjectHubModePinned {
			return listsurface.ListEmptyState{Title: "No pinned projects hold open tasks.", Detail: m.projectHubFilterAdvice()}
		}
		return listsurface.ListEmptyState{Title: "No projects hold open tasks.", Detail: m.projectHubFilterAdvice()}
	case projectHubTaskFilterWithoutOpenTasks:
		if m.mode == route.ProjectHubModePinned {
			return listsurface.ListEmptyState{Title: "Every pinned project holds open tasks.", Detail: m.projectHubFilterAdvice()}
		}
		return listsurface.ListEmptyState{Title: "Every project holds open tasks.", Detail: m.projectHubFilterAdvice()}
	default:
		return listsurface.ListEmptyState{
			Title:  "No projects are available.",
			Detail: "Press a to create a project.",
		}
	}
}

// projectHubFilterAdvice names where the next press lands, so a three-state
// cycle never claims one press returns to the whole list.
func (m *ProjectHubModel) projectHubFilterAdvice() string {
	return "Press Ctrl+T to show " + m.projectHubNextFilterPhrase() + "."
}

func (m *ProjectHubModel) projectHubNextFilterPhrase() string {
	next := m.taskFilter.next()
	if m.mode != route.ProjectHubModePinned {
		return next.phrase()
	}
	switch next {
	case projectHubTaskFilterWithOpenTasks:
		return "pinned projects that hold open tasks"
	case projectHubTaskFilterWithoutOpenTasks:
		return "pinned projects without open tasks"
	default:
		return "every pinned project"
	}
}

func (m *ProjectHubModel) projectHubNoMatchState() listsurface.ListEmptyState {
	title := "No projects match this search."
	browsePopulation := "every project"
	if m.mode == route.ProjectHubModePinned {
		title = "No pinned projects match this search."
		browsePopulation = "pinned projects"
	}
	detail := "Press Esc to clear the query and browse " + browsePopulation + "."
	if m.taskFilter != projectHubTaskFilterAll {
		detail = "Press Esc to clear the query, or Ctrl+T to show " + m.projectHubNextFilterPhrase() + "."
	}
	return listsurface.ListEmptyState{
		Title:  title,
		Detail: detail,
	}
}

func (m *ProjectHubModel) selectedProjectHubEntry() (projectHubEntry, bool) {
	if m.surface == nil {
		return projectHubEntry{}, false
	}
	item, ok := m.surface.CurrentItem()
	if !ok {
		return projectHubEntry{}, false
	}
	entry, ok := item.Data.(projectHubEntry)
	return entry, ok
}

// activateProjectHubCurrent is the one Enter path: the search input and the
// browse grammar both reach it, so the key means the same thing whether or not
// the query is focused. The archived refusal stays here because it names the row
// the cursor sits on, which the operation's own applicability can only report as
// a missing action.
func (m *ProjectHubModel) activateProjectHubCurrent() tea.Cmd {
	entry, ok := m.selectedProjectHubEntry()
	if !ok || entry.Project == nil {
		return nil
	}
	if entry.Kind == projectHubEntryArchived {
		m.notice = "Archived projects must be unarchived from Space actions before they can be focused"
		return nil
	}
	return m.dispatchProjectHubKey("enter")
}

func (m *ProjectHubModel) projectHubCommandContext(projectID string) operation.CommandContext {
	ctx := route.PresentationCommandContext(presentation.ViewProjectHub)
	ctx.Capabilities.Add(operation.CapabilityProjectHub)
	if projectID != "" {
		ctx.CurrentEntity = operation.ProjectEntity(projectID)
	}
	return ctx
}
