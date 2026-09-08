package filterops

import (
	"fmt"
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/tui/valuepicker"
)

func ProjectFilterOperationDescriptors() []operation.OperationDescriptor {
	descriptor := filterOperationDescriptor
	return []operation.OperationDescriptor{
		descriptor(operation.OperationFilterProjectTags, "Filter Projects by Tag", operation.ValueProviderFilterProjectTags, "project tag filter"),
		descriptor(operation.OperationFilterShowParked, "Include Parked Projects", "", "parked projects"),
		descriptor(operation.OperationFilterOnlyParked, "Only Parked Projects", "", "isolate parked projects"),
		descriptor(operation.OperationFilterIncludeArchived, "Include Archived Projects", "", "archived projects"),
		descriptor(operation.OperationFilterOnlyArchived, "Only Archived Projects", "", "isolate archived projects"),
		descriptor(operation.OperationFilterProjectClear, "Clear Project Filters", "", "reset project filters"),
	}
}

// executeProjectFilterOperation applies one population facet. A row facet can
// rebuild in place, but a project this facet admits or drops was never in the
// snapshot at all, so applyFilterClauseEdit reloads instead — an obligation it
// reads off the population fields themselves.
func (c *Controller) ExecuteProjectFilterOperation(shell Shell, invocation operation.OperationInvocation) operation.OperationHandlerResult {
	out := shell.Outliner()
	if out == nil {
		return operation.OperationHandlerResult{Err: fmt.Errorf("the Outliner is unavailable")}
	}
	edit, err := out.PopulationFilterClauseEdit(invocation)
	if err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	return c.ApplyFilterClauseEdit(shell, invocation.Descriptor.ID, edit)
}

// projectFilterReloadCmd reloads the population and reports the new facets. The
// The status text travels inside the command because a handler that returns
// both a message and a status delivers only the message.
func (c *Controller) ProjectFilterReloadCmd(shell Shell, text string) tea.Cmd {
	out := shell.Outliner()
	if out == nil {
		return nil
	}
	// The population is about to change underneath the cursor, so restoring the
	// previous row would fight the facet the user just applied.
	out.Population.RequestReloadFromTop()
	return tea.Batch(
		out.ReloadCrossProjects(),
		func() tea.Msg { return status.StatusMsg(text) },
		status.DefaultStatusTimeout(),
	)
}

// newFilterProjectTagsPicker is the F# multi-select picker over the whole
// project tag vocabulary. It sources the catalog rather than the loaded projects
// so a facet can still name a tag whose only projects the current facets exclude
// — otherwise a filter could never be widened back out from inside the picker.
func (c *Controller) NewFilterProjectTagsPicker(shell Shell,
	request operation.OperationExecutionRequest,
) (*valuepicker.OperationValuePickerModel, error) {
	counts, untagged := c.ProjectTagVocabularyCounts(shell)
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]listsurface.ListItem, 0, len(names)+1)
	for _, name := range names {
		// The facet rows name the same vocabulary the editor at # does, so they
		// take the same two styles: a chip for a descriptive tag, the badge for a
		// behavioral one. Otherwise parked would render plain on one project-tag
		// surface and in warning on the other.
		style := theme.TagChipStyle(name)
		item := listsurface.ListItem{
			ID:            listsurface.ListItemID("tag:" + name),
			CategoryID:    valuepicker.TagDescriptiveCategory,
			PrimaryText:   name,
			SecondaryText: ProjectCountPhrase(counts[name]),
			Aliases:       []string{name, "#" + name},
			Data:          name,
		}
		if info, behavioral := models.BehavioralProjectTags[name]; behavioral {
			item.CategoryID = valuepicker.ProjectTagBehavioralCategory
			item.SecondaryText = info.Summary
			style = projecthub.BehavioralProjectTagBadgeStyle(info.BadgeRole)
		}
		item.PrimaryStyle = &style
		items = append(items, item)
	}
	// The empty tag set is legal, so without this row a tag facet could never
	// name the projects the vocabulary does not cover.
	items = append(items, listsurface.ListItem{
		ID:            listsurface.ListItemID("tag:" + query.UntaggedProjectFilterKey),
		CategoryID:    valuepicker.TagDescriptiveCategory,
		PrimaryText:   "Untagged",
		SecondaryText: ProjectCountPhrase(untagged),
		Aliases:       []string{"untagged", "no tags"},
		Data:          query.UntaggedProjectFilterKey,
	})

	source := listsurface.ListSource{
		Items: items,
		Categories: []listsurface.ListCategory{
			{ID: valuepicker.ProjectTagBehavioralCategory, Label: "Behavioral"},
			{ID: valuepicker.TagDescriptiveCategory, Label: "Tags"},
		},
	}
	selected := []listsurface.ListItemID(nil)
	if shell.Outliner() != nil {
		rows := query.ProjectFilterSelection(&shell.Outliner().ProjectFilter)
		selected = make([]listsurface.ListItemID, 0, len(rows))
		for _, row := range rows {
			selected = append(selected, listsurface.ListItemID(row))
		}
	}
	return valuepicker.NewOperationValuePickerModel(
		request,
		operation.ValueProviderFilterProjectTags,
		"Filter Projects by Tag",
		"choose which projects load",
		source,
		true,
		source.RetainKnownIDs(selected),
	)
}

// projectTagVocabularyCounts returns how many projects carry each tag, plus the
// count carrying none. The parked and archived populations are both included so
// the numbers describe the vocabulary rather than the current facets, which the
// picker exists to change.
func (c *Controller) ProjectTagVocabularyCounts(shell Shell) (map[string]int, int) {
	counts := make(map[string]int)
	untagged := 0
	for name := range models.BehavioralProjectTags {
		counts[name] = 0
	}
	projects := append(shell.AvailableProjects(), shell.ArchivedProjects()...)
	for _, project := range projects {
		if project == nil {
			continue
		}
		if len(project.Tags) == 0 {
			untagged++
			continue
		}
		for _, tag := range project.Tags {
			counts[tag]++
		}
	}
	return counts, untagged
}

// ProjectCountPhrase pluralizes a project count for the one-line summaries the
// facets and the bulk project actions both write.
func ProjectCountPhrase(count int) string {
	if count == 1 {
		return "1 project"
	}
	return fmt.Sprintf("%d projects", count)
}
