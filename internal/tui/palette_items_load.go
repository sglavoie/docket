package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/storage"
)

// loadAllItems aggregates commands, tasks, projects, tags, and sections into PaletteItems.
// This follows the lazy loading pattern from GlobalSearchModel, loading all searchable
// items when the palette opens rather than maintaining a persistent index.
// Commands are filtered by the current view context (only showing relevant commands).
//
//nolint:gocyclo // Palette aggregation intentionally keeps every item category in one ordered pipeline.
func (m *CommandPaletteModel) loadAllItems() []PaletteItem {
	var items []PaletteItem

	// Load commands from the typed capability context when the invoking route
	// supplies one; renderer-scoped commands use PresentationID.
	commands := m.registry.GetForView(m.currentView)
	if m.commandContext != nil {
		commands = m.registry.GetForContext(*m.commandContext)
	}
	for _, cmd := range commands {
		cmdCopy := cmd
		items = append(items, PaletteItem{
			Name:        cmdCopy.Name,
			Description: cmdCopy.Description,
			Shortcut:    activeGlobalPaletteShortcut(cmdCopy),
			Category:    ItemCategoryCommands,
			Command:     &cmdCopy,
		})
	}

	// Track tags and sections with task counts across all projects
	tagCounts := make(map[string]int)
	sectionCounts := make(map[string]int)

	// Load project discovery rows from active projects plus archived projects.
	// Operational task/tag/section/note rows below stay active-only.
	for _, project := range projectDiscoveryProjects(m.runtime) {
		items = append(items, PaletteItem{
			Name:        project.Name,
			Description: projectDiscoveryDescription(project),
			Category:    ItemCategoryProjects,
			Project:     project,
		})
	}

	// Load tasks, tags, sections, and notes from active projects.
	// When currentProject is set, only load tasks from that project
	projects := m.runtime.ActiveProjects()
	// Include sandbox if it's the current project (GetActiveProjects excludes it)
	if m.currentProject != nil && m.currentProject.Sandbox {
		projects = append(projects, m.currentProject)
	}
	for _, project := range projects {
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
			if task.IsTerminalState() {
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

			if bucket := taskSectionBucket(task); bucket != "" {
				sectionCounts[bucket]++
			}
		}
	}

	tags := make([]string, 0, len(tagCounts))
	for tag := range tagCounts {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	for _, tag := range tags {
		count := tagCounts[tag]
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

	sections := make([]string, 0, len(sectionCounts))
	for section := range sectionCounts {
		sections = append(sections, section)
	}
	sort.Strings(sections)
	for _, section := range sections {
		count := sectionCounts[section]
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

	savedFilters := m.runtime.Planning().GetSavedFilters()
	for i := range savedFilters {
		sf := savedFilters[i]
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

	for _, v := range staticPaletteViewItems() {
		items = append(items, PaletteItem{
			Name:        v.name,
			Description: v.description,
			Category:    ItemCategoryViews,
			ViewTarget:  v.viewType,
			AliasTarget: v.aliasTarget,
		})
	}

	// Every Outliner catalog mode is a selectable palette action, derived from
	// OutlinerModeCatalog rather than a hand-maintained parallel command list.
	items = append(items, outlinerModePaletteItems()...)

	return items
}

// paletteViewItemSpec describes a directly-openable surface exposed as a
// Command Palette "Views" entry.
type paletteViewItemSpec struct {
	name        string
	description string
	viewType    PresentationID
	aliasTarget PaletteAliasTarget
}

// staticPaletteViewItems is the fixed catalog of direct-tool palette entries.
// It is a package-level function (not an inline literal) so guard tests can scan
// the palette registry for retired surface names without a live runtime.
func staticPaletteViewItems() []paletteViewItemSpec {
	return []paletteViewItemSpec{
		{name: "Daily Review", description: "Review and plan your day", viewType: ViewDailyReview},
		{name: "Weekly Review", description: "Strategic project health review", viewType: ViewWeeklyReview},
		{name: "Recent Activity", description: "Browse recent and completed tasks", viewType: ViewRecentActivity},
		{name: "Metrics Dashboard", description: "Completion, cycle-time, and throughput analytics", viewType: ViewMetricsDashboard},
		{name: "Roadmap", description: "Timelines, forecasts, and critical paths", viewType: ViewRoadmap},
		{name: "Blockers", description: "View blocked and blocking tasks", viewType: ViewBlockers},
		{name: "Global Blockers", description: "Blockers across all projects", viewType: ViewGlobalBlockers},
		{name: "Global Inbox", description: "Process unassigned tasks across projects", viewType: ViewGlobalInboxProcessor},
		{name: "Global Tag Manager", description: "Manage tags across all projects", viewType: ViewGlobalTagManager},
		{name: "Notes", description: "Project notes and documents", viewType: ViewNotes},
		{name: "Global Notes", description: "Notes not tied to any project", viewType: ViewGlobalNotes},
		{name: "Workflow Rules", description: "Manage workflow automation rules", viewType: ViewWorkflowRules},
		{name: "Settings", description: "Application preferences and configuration", viewType: ViewSettings},
		// These names remain searchable aliases for consolidated surfaces. They
		// do not restore the retired view implementations or their old keys.
		{name: "Global Search", description: "Alias for Command Palette search", viewType: ViewGlobalSearch},
		{name: "Notes Search", description: "Alias for Manage Notes search", viewType: ViewNotesSearch},
	}
}

// activeGlobalPaletteShortcut exposes only bindings from the exact global map.
// Contextual operations remain searchable and executable through Enter, but
// stale descriptor shortcuts cannot become ghost-key documentation.
func activeGlobalPaletteShortcut(command Command) string {
	if command.Reachability != CommandReachGlobalBinding {
		return ""
	}
	switch command.Shortcut {
	case ":", "?", "esc", "ctrl+c", "ctrl+esc", "ctrl+n", "ctrl+x", "|", "#":
		return command.Shortcut
	default:
		return ""
	}
}
