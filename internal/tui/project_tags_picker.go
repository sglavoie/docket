package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

const (
	projectTagBehavioralCategory listsurface.ListCategoryID = "project-tag:behavioral"
	tagDescriptiveCategory       listsurface.ListCategoryID = "tag:other"
)

// newProjectTagsPicker offers the project's own tag set. Behavioral tags are
// pinned above the descriptive vocabulary because they are the only ones that
// change what the project does, and the explainer beneath the list says what
// that change is before the user commits to it.
func (m *AppModel) newProjectTagsPicker(
	request operation.OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	project := m.projectByID(operation.CurrentEntityIdentity(request.Context).ProjectID)
	if project == nil {
		return nil, fmt.Errorf("project tags need a project target")
	}
	selected := make([]listsurface.ListItemID, 0, len(project.Tags))
	for _, tag := range project.Tags {
		selected = append(selected, listsurface.ListItemID("tag:"+tag))
	}
	return newOperationValuePickerModel(
		request,
		operation.ValueProviderProjectTags,
		"Project Tags",
		project.Name,
		m.projectTagListSource(project),
		true,
		selected,
	)
}

// projectTagListSource merges the workspace-wide project tag vocabulary with the
// project's own set, so a tag only this project carries stays selectable and a
// behavioral tag is always offered even when nothing uses it yet.
func (m *AppModel) projectTagListSource(project *models.Project) listsurface.ListSource {
	descriptive := make(map[string]bool)
	if m.runtime != nil {
		for _, tag := range m.runtime.AllProjectTags() {
			descriptive[tag] = true
		}
	}
	for _, tag := range project.Tags {
		descriptive[tag] = true
	}

	items := make([]listsurface.ListItem, 0, len(descriptive)+len(models.BehavioralProjectTags))
	behavioral := make([]string, 0, len(models.BehavioralProjectTags))
	for name := range models.BehavioralProjectTags {
		behavioral = append(behavioral, name)
		delete(descriptive, name)
	}
	sort.Strings(behavioral)
	for _, name := range behavioral {
		info := models.BehavioralProjectTags[name]
		// The row takes the same expression the explainer below the list renders
		// the name with, so the row and the sentence explaining it cannot disagree
		// about what parked looks like.
		style := behavioralProjectTagBadgeStyle(info.BadgeRole)
		items = append(items, listsurface.ListItem{
			ID:            listsurface.ListItemID("tag:" + name),
			CategoryID:    projectTagBehavioralCategory,
			PrimaryText:   name,
			SecondaryText: info.RowSummary,
			Aliases:       []string{name, "#" + name},
			PrimaryStyle:  &style,
			Preview: listsurface.ListPreview{
				Title:  name,
				Fields: []listsurface.ListPreviewField{{Label: "Effect", Value: info.Summary}},
			},
			Data: name,
		})
	}

	names := make([]string, 0, len(descriptive))
	for tag := range descriptive {
		names = append(names, tag)
	}
	sort.Strings(names)
	for _, tag := range names {
		// One ramp serves both namespaces, so #infra on a project reads as the
		// same color as #infra on a task even though the two tag sets are
		// otherwise unrelated.
		style := theme.TagChipStyle(tag)
		items = append(items, listsurface.ListItem{
			ID:           listsurface.ListItemID("tag:" + tag),
			CategoryID:   tagDescriptiveCategory,
			PrimaryText:  tag,
			Aliases:      []string{tag, "#" + tag},
			PrimaryStyle: &style,
			Preview:      listsurface.ListPreview{Title: tag},
			Data:         tag,
		})
	}

	return listsurface.ListSource{
		Items: items,
		Categories: []listsurface.ListCategory{
			{ID: projectTagBehavioralCategory, Label: "Behavioral"},
			{ID: tagDescriptiveCategory, Label: "Tags"},
		},
	}
}

// behavioralProjectTagExplainer restates every behavioral tag beneath the list.
// The picker cannot show the effect by example the way a state change can, so
// the sentence has to be on screen the whole time the choice is open. Each
// complete entry wraps inline rather than sending a detached tail elsewhere.
func behavioralProjectTagExplainer(provider operation.ValueProviderID, width int) string {
	if provider != operation.ValueProviderProjectTags {
		return ""
	}
	names := make([]string, 0, len(models.BehavioralProjectTags))
	for name := range models.BehavioralProjectTags {
		names = append(names, name)
	}
	sort.Strings(names)
	lines := make([]string, 0, len(names))
	for _, name := range names {
		info := models.BehavioralProjectTags[name]
		entry := behavioralProjectTagBadgeStyle(info.BadgeRole).Render(info.Name) +
			" " + theme.MutedStyle.Render(info.Summary)
		lines = append(lines, strings.Join(textfit.WrapStyled(entry, width), "\n"))
	}
	return strings.Join(lines, "\n")
}

// behavioralProjectTagBadgeStyle turns the metadata badge role into a palette
// style. An unrecognized role renders muted so a tag is never hidden by a
// missing style.
func behavioralProjectTagBadgeStyle(role string) lipgloss.Style {
	switch role {
	case "warning":
		return lipgloss.NewStyle().Foreground(theme.ColorWarning).Bold(true)
	default:
		return theme.MutedStyle
	}
}
