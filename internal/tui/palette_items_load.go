package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/storage"
)

// loadAllItems aggregates commands, tasks, projects, tags, and sections into PaletteItems.
// This follows the lazy loading pattern from GlobalSearchModel, loading all searchable
// items when the palette opens rather than maintaining a persistent index.
// Commands are filtered by the current view context (only showing relevant commands).
func (m *CommandPaletteModel) loadAllItems() []PaletteItem {
	var items []PaletteItem

	// Load commands from registry (filtered by current view)
	for _, cmd := range m.registry.GetForView(m.currentView) {
		cmdCopy := cmd
		items = append(items, PaletteItem{
			Name:        cmdCopy.Name,
			Description: cmdCopy.Description,
			Shortcut:    cmdCopy.Shortcut,
			Category:    ItemCategoryCommands,
			Command:     &cmdCopy,
		})
	}

	// Track tags and sections with task counts across all projects
	tagCounts := make(map[string]int)
	sectionCounts := make(map[string]int)

	// Load tasks, projects, tags, and sections from active projects
	// When currentProject is set, only load tasks from that project
	projects := m.config.GetActiveProjects()
	// Include sandbox if it's the current project (GetActiveProjects excludes it)
	if m.currentProject != nil && m.currentProject.Sandbox {
		projects = append(projects, m.currentProject)
	}
	for _, project := range projects {
		items = append(items, PaletteItem{
			Name:        project.Name,
			Description: fmt.Sprintf("Project: %s", project.Slug),
			Category:    ItemCategoryProjects,
			Project:     project,
		})

		if m.currentProject != nil && project.ID != m.currentProject.ID {
			continue
		}

		if err := storage.LoadProject(project); err != nil {
			continue
		}

		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			if !m.includeCompleted && !task.Status.IsActive() {
				continue
			}

			description := fmt.Sprintf("#%d in %s", task.ShortID, project.Slug)
			if m.currentProject != nil {
				description = fmt.Sprintf("#%d", task.ShortID)
			}
			items = append(items, PaletteItem{
				Name:        task.Title,
				Description: description,
				Category:    ItemCategoryTasks,
				Task:        task,
				Project:     project,
			})

			for _, tag := range task.Tags {
				tagCounts[tag]++
			}

			if task.Section != "" {
				sectionCounts[task.Section]++
			}
		}
	}

	for tag, count := range tagCounts {
		var desc string
		if count == 1 {
			desc = "Tag (1 task)"
		} else {
			desc = fmt.Sprintf("Tag (%d tasks)", count)
		}
		items = append(items, PaletteItem{
			Name:        tag,
			Description: desc,
			Category:    ItemCategoryTags,
			Tag:         tag,
		})
	}

	for section, count := range sectionCounts {
		var desc string
		if count == 1 {
			desc = "Section (1 task)"
		} else {
			desc = fmt.Sprintf("Section (%d tasks)", count)
		}
		items = append(items, PaletteItem{
			Name:        section,
			Description: desc,
			Category:    ItemCategorySections,
			Section:     section,
		})
	}

	for name, query := range m.config.GetSavedSearches() {
		items = append(items, PaletteItem{
			Name:        name,
			Description: query,
			Category:    ItemCategorySavedSearch,
			SavedSearch: query,
		})
	}

	for _, project := range projects {
		notes, err := storage.ListNotes(project)
		if err != nil {
			continue
		}
		for _, note := range notes {
			desc := project.Name
			if len(note.Tags) > 0 {
				desc += " · " + strings.Join(note.Tags, ", ")
			}
			items = append(items, PaletteItem{
				Name:        note.Title,
				Description: desc,
				Category:    ItemCategoryNotes,
				Note:        note,
				Project:     project,
			})
		}
	}

	for i := range m.config.GetSavedFilters() {
		sf := m.config.GetSavedFilters()[i]
		desc := describeSavedFilterSummary(&sf)
		modeBadge := "[S] "
		if sf.Mode == "criteria" {
			modeBadge = "[C] "
		}
		items = append(items, PaletteItem{
			Name:        modeBadge + sf.Name,
			Description: desc,
			Category:    ItemCategorySavedFilter,
			SavedFilter: &sf,
		})
	}

	viewItems := []struct {
		name        string
		description string
		viewType    ViewType
	}{
		{"Daily Review", "Review and plan your day", ViewDailyReview},
		{"Weekly Review", "Strategic project health review", ViewWeeklyReview},
		{"Backlog", "Cross-project task suggestions and backlog", ViewFocusView},
		{"Due Date Overview", "Tasks organized by due date", ViewDueDateOverview},
		{"Priority View", "Tasks sorted by priority level", ViewPriorityView},
		{"Blockers", "View blocked and blocking tasks", ViewBlockers},
		{"All Tasks", "View tasks across all projects", ViewAllTasks},
		{"Global Blockers", "Blockers across all projects", ViewGlobalBlockers},
		{"Global Inbox", "Process unassigned tasks across projects", ViewGlobalInboxProcessor},
		{"Global Tag Manager", "Manage tags across all projects", ViewGlobalTagManager},
		{"Notes", "Project notes and documents", ViewNotes},
		{"Global Notes", "Notes not tied to any project", ViewGlobalNotes},
		{"Workflow Rules", "Manage workflow automation rules", ViewWorkflowRules},
		{"Settings", "Application preferences and configuration", ViewSettings},
		{"Undo History", "View and restore previous changes", ViewUndoHistory},
	}
	for _, v := range viewItems {
		items = append(items, PaletteItem{
			Name:        v.name,
			Description: v.description,
			Category:    ItemCategoryViews,
			ViewTarget:  v.viewType,
		})
	}

	return items
}
