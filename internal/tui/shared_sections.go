package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// PinnedTagTabs returns ["All"] + project pinned tag labels, or nil if no tags are pinned.
func PinnedTagTabs(project *models.Project) []string {
	if project == nil {
		return nil
	}
	return pinnedTagTabs(project.PinnedTagList())
}

// GlobalPinnedTagTabs returns ["All"] + global pinned tag labels, or nil if no tags are pinned.
func GlobalPinnedTagTabs(settings *config.Settings) []string {
	if settings == nil {
		return nil
	}
	return pinnedTagTabs(settings.GlobalPinnedTagList())
}

// GlobalPinnedTagTabsForProjects returns ["All"] + global pinned tag labels
// that are currently present in the loaded project set. Stale pins stay
// persisted but are hidden until a loaded project exposes the tag again.
func GlobalPinnedTagTabsForProjects(settings *config.Settings, projects []*models.Project) []string {
	if settings == nil {
		return nil
	}
	visibleTags := make(map[string]bool)
	for _, project := range projects {
		if project == nil {
			continue
		}
		for tag := range project.TagCounts() {
			visibleTags[tag] = true
		}
	}
	if len(visibleTags) == 0 {
		return nil
	}
	tags := make([]string, 0, len(settings.GlobalPinnedTags))
	for _, tag := range settings.GlobalPinnedTagList() {
		if visibleTags[tag] {
			tags = append(tags, tag)
		}
	}
	return pinnedTagTabs(tags)
}

// ActivePinnedTagFilter returns the raw pinned tag selected by tabIndex, or "" for "All".
func ActivePinnedTagFilter(project *models.Project, tabIndex int) string {
	return activePinnedTagFilter(PinnedTagTabs(project), tabIndex)
}

func activePinnedTagFilter(tabs []string, tabIndex int) string {
	if tabs == nil || tabIndex <= 0 || tabIndex >= len(tabs) {
		return ""
	}
	return pinnedTagFromTab(tabs[tabIndex])
}

func pinnedTagTabs(tags []string) []string {
	tags = models.DedupeTags(tags)
	if len(tags) == 0 {
		return nil
	}
	tabs := make([]string, 0, len(tags)+1)
	tabs = append(tabs, "All")
	for _, tag := range tags {
		tabs = append(tabs, "#"+tag)
	}
	return tabs
}

func pinnedTagFromTab(tab string) string {
	return models.NormalizeTag(tab)
}

func gtdSections(sections []string) []string {
	filtered := make([]string, 0, len(sections))
	for _, section := range sections {
		if models.IsGTDSection(section) {
			filtered = append(filtered, section)
		}
	}
	return filtered
}
