package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

const (
	taskTagSemanticCategory ListCategoryID = "task-tag:semantic"

	semanticTaskTagExplainerLabel     = "semantic"
	semanticTaskTagExplainerSeparator = " › "
	semanticTaskTagExplainerSummary   = "— the highest one a task carries colors its title."
)

// semanticTaskTagProvider reports whether a picker draws the task tag
// vocabulary. The project namespace has its own two tiers and reserves parked,
// so a semantic name means nothing there.
func semanticTaskTagProvider(provider ValueProviderID) bool {
	return provider == ValueProviderTags || provider == ValueProviderFilterTags
}

// taskTagListSource pins the semantic vocabulary above the tags in use.
// seedUnused offers every semantic tag even when nothing carries it, which the
// editor wants for discovery and the filter must not do, because a filter row
// nothing matches can only empty the projection.
func taskTagListSource(tags []TagInfo, seedUnused bool) ListSource {
	items := make([]ListItem, 0, len(tags)+len(models.SemanticTaskTags))
	promoted := make(map[string]bool, len(tags))
	for _, entry := range models.SemanticTaskTags {
		canonicalInUse := false
		for _, tag := range tags {
			if !strings.EqualFold(tag.Name, entry.Name) {
				continue
			}
			// Task tags keep their stored spelling, so a task tagged Bug must
			// round-trip as Bug when the row is toggled off.
			promoted[tag.Name] = true
			canonicalInUse = canonicalInUse || tag.Name == entry.Name
			items = append(items, semanticTaskTagItem(tag.Name, entry.Summary, tag.TaskCount))
		}
		if seedUnused && !canonicalInUse {
			items = append(items, semanticTaskTagItem(entry.Name, entry.Summary, 0))
		}
	}

	descriptive := make([]TagInfo, 0, len(tags))
	for _, tag := range tags {
		if !promoted[tag.Name] {
			descriptive = append(descriptive, tag)
		}
	}
	sort.Slice(descriptive, func(i, j int) bool {
		return descriptive[i].Name < descriptive[j].Name
	})
	for _, tag := range descriptive {
		items = append(items, ListItem{
			ID:            ListItemID("tag:" + tag.Name),
			CategoryID:    tagDescriptiveCategory,
			PrimaryText:   tag.Name,
			SecondaryText: fmt.Sprintf("%d tasks", tag.TaskCount),
			Aliases:       []string{tag.Name, "#" + tag.Name},
			Preview: ListPreview{
				Title:  tag.Name,
				Fields: []ListPreviewField{{Label: "Tasks", Value: strconv.Itoa(tag.TaskCount)}},
			},
			Data: tag.Name,
		})
	}

	return ListSource{
		Items: items,
		Categories: []ListCategory{
			{ID: taskTagSemanticCategory, Label: "Semantic"},
			{ID: tagDescriptiveCategory, Label: "Tags"},
		},
	}
}

// semanticTaskTagItem trades the usage count for the meaning, because a tag the
// picker offers before anything carries it has no count worth showing and the
// meaning is the reason it is offered at all.
func semanticTaskTagItem(name string, summary string, taskCount int) ListItem {
	fields := []ListPreviewField{{Label: "Meaning", Value: summary}}
	if taskCount > 0 {
		fields = append(fields, ListPreviewField{Label: "Tasks", Value: strconv.Itoa(taskCount)})
	}
	return ListItem{
		ID:            ListItemID("tag:" + name),
		CategoryID:    taskTagSemanticCategory,
		PrimaryText:   name,
		SecondaryText: summary,
		Aliases:       []string{name, "#" + name},
		Preview:       ListPreview{Title: name, Fields: fields},
		Data:          name,
	}
}

// semanticTaskTagExplainer states the precedence the section order applies
// silently. Five restated summaries would swamp the overlay, so it is one line,
// and every name renders in the title color it produces so the line
// demonstrates what it describes. The color lookup is the row one, which is
// what keeps a future registry entry from rendering uncolored.
func semanticTaskTagExplainer(provider ValueProviderID, width int) string {
	if !semanticTaskTagProvider(provider) {
		return ""
	}
	names := make([]string, 0, len(models.SemanticTaskTags))
	headWidth := lipgloss.Width(semanticTaskTagExplainerLabel) + 2
	for index, entry := range models.SemanticTaskTags {
		if index > 0 {
			headWidth += lipgloss.Width(semanticTaskTagExplainerSeparator)
		}
		headWidth += lipgloss.Width(entry.Name)
		names = append(names, semanticTaskTitleStyle(entry.Role, 0, false, false).Render(entry.Name))
	}
	if width < headWidth {
		return ""
	}
	head := MutedStyle.Render(semanticTaskTagExplainerLabel) + "  " +
		strings.Join(names, MutedStyle.Render(semanticTaskTagExplainerSeparator))
	summary := truncateToVisualWidth(semanticTaskTagExplainerSummary, width-headWidth-1)
	if summary == "" {
		return head
	}
	return head + " " + MutedStyle.Render(summary)
}

// semanticTaskTagPrecedence ranks one semantic row by the registry order the
// explainer states. A name the registry does not know sorts last instead of
// reshuffling the section it was put in.
func semanticTaskTagPrecedence(name string) models.SemanticTaskTagRole {
	if entry, ok := models.SemanticTaskTagByName(name); ok {
		return entry.Role
	}
	return models.SemanticTaskTagRole(len(models.SemanticTaskTags) + 1)
}
