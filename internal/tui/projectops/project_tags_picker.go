package projectops

import (
	"fmt"
	"sort"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/tui/valuepicker"
)

// NewProjectTagsPicker offers the project's own tag set. Behavioral tags are
// pinned above the descriptive vocabulary because they are the only ones that
// change what the project does, and the explainer beneath the list says what
// that change is before the user commits to it.
func (c *Controller) NewProjectTagsPicker(
	shell Shell,
	request operation.OperationExecutionRequest,
) (*valuepicker.OperationValuePickerModel, error) {
	project := shell.ProjectByID(operation.CurrentEntityIdentity(request.Context).ProjectID)
	if project == nil {
		return nil, fmt.Errorf("project tags need a project target")
	}
	selected := make([]listsurface.ListItemID, 0, len(project.Tags))
	for _, tag := range project.Tags {
		selected = append(selected, listsurface.ListItemID("tag:"+tag))
	}
	return valuepicker.NewOperationValuePickerModel(
		request,
		operation.ValueProviderProjectTags,
		"Project Tags",
		project.Name,
		c.TagListSource(shell, project),
		true,
		selected,
	)
}

// TagListSource merges the workspace-wide project tag vocabulary with the
// project's own set, so a tag only this project carries stays selectable and a
// behavioral tag is always offered even when nothing uses it yet.
func (c *Controller) TagListSource(shell Shell, project *models.Project) listsurface.ListSource {
	descriptive := make(map[string]bool)
	if shell != nil {
		for _, tag := range shell.AllProjectTags() {
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
		style := projecthub.BehavioralProjectTagBadgeStyle(info.BadgeRole)
		items = append(items, listsurface.ListItem{
			ID:            listsurface.ListItemID("tag:" + name),
			CategoryID:    valuepicker.ProjectTagBehavioralCategory,
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
			CategoryID:   valuepicker.TagDescriptiveCategory,
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
			{ID: valuepicker.ProjectTagBehavioralCategory, Label: "Behavioral"},
			{ID: valuepicker.TagDescriptiveCategory, Label: "Tags"},
		},
	}
}
