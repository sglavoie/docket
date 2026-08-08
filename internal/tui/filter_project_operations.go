package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// The F chord owns the population facets. They are separate operations from the
// f chord's row facets because they decide which projects load at all: applying
// one needs a reload, not a rebuild.
const (
	operationFilterProjectTags     OperationID = "tui.filter.project-tags"
	operationFilterShowParked      OperationID = "tui.filter.show-parked"
	operationFilterOnlyParked      OperationID = "tui.filter.only-parked"
	operationFilterIncludeArchived OperationID = "tui.filter.include-archived"
	operationFilterOnlyArchived    OperationID = "tui.filter.only-archived"
	operationFilterProjectClear    OperationID = "tui.filter.project-clear"
)

func projectFilterOperationDescriptors() []OperationDescriptor {
	descriptor := filterOperationDescriptor
	return []OperationDescriptor{
		descriptor(operationFilterProjectTags, "Filter Projects by Tag", ValueProviderFilterProjectTags, "project tag filter"),
		descriptor(operationFilterShowParked, "Include Parked Projects", "", "parked projects"),
		descriptor(operationFilterOnlyParked, "Only Parked Projects", "", "isolate parked projects"),
		descriptor(operationFilterIncludeArchived, "Include Archived Projects", "", "archived projects"),
		descriptor(operationFilterOnlyArchived, "Only Archived Projects", "", "isolate archived projects"),
		descriptor(operationFilterProjectClear, "Clear Project Filters", "", "reset project filters"),
	}
}

// executeProjectFilterOperation applies one population facet and reloads. A row
// facet can rebuild in place, but a project this facet admits or drops was never
// in the snapshot at all, so the change only becomes visible after a load.
func (m *AppModel) executeProjectFilterOperation(invocation OperationInvocation) OperationHandlerResult {
	out := m.crossOutliner
	if out == nil {
		return OperationHandlerResult{Err: fmt.Errorf("the Outliner is unavailable")}
	}
	if out.hasSearchProjection() {
		return OperationHandlerResult{Status: "Exit search before changing filters"}
	}
	filter := &out.projectFilter

	var status string
	switch invocation.Descriptor.ID {
	case operationFilterProjectTags:
		names, ok := invocation.Value.([]string)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid project tag filter value")}
		}
		applyProjectTagFilterValue(filter, names)
		status = projectTagFilterStatus(filter)
	case operationFilterShowParked:
		// Fp, FP, and the Ft picker's parked row all write this one field, so the
		// routes cannot leave the population claiming different things.
		filter.ParkedMode = toggleProjectPopulation(filter.ParkedMode, projectPopulationInclude)
		status = projectPopulationStatus(filter.ParkedMode, "Parked")
	case operationFilterOnlyParked:
		filter.ParkedMode = toggleProjectPopulation(filter.ParkedMode, projectPopulationOnly)
		status = projectPopulationStatus(filter.ParkedMode, "Parked")
	case operationFilterIncludeArchived:
		filter.ArchivedMode = toggleProjectPopulation(filter.ArchivedMode, projectPopulationInclude)
		status = projectPopulationStatus(filter.ArchivedMode, "Archived")
	case operationFilterOnlyArchived:
		filter.ArchivedMode = toggleProjectPopulation(filter.ArchivedMode, projectPopulationOnly)
		status = projectPopulationStatus(filter.ArchivedMode, "Archived")
	case operationFilterProjectClear:
		if !hasActiveProjectFilters(filter) {
			return OperationHandlerResult{Status: "No project filters are active"}
		}
		clearProjectFilters(filter)
		status = "Project filters cleared"
	default:
		return OperationHandlerResult{Err: fmt.Errorf("unknown project filter operation %s", invocation.Descriptor.ID)}
	}

	return OperationHandlerResult{Message: OperationCommandResultMsg{Cmd: m.projectFilterReloadCmd(status)}}
}

// projectPopulationStatus names the population one axis now loads rather than
// the key that was pressed, because two keys reach the same three modes.
func projectPopulationStatus(mode projectPopulationMode, name string) string {
	switch mode {
	case projectPopulationOnly:
		return "Showing only " + strings.ToLower(name) + " projects"
	case projectPopulationInclude:
		return "Showing " + strings.ToLower(name) + " projects"
	default:
		return name + " projects hidden"
	}
}

// projectFilterReloadCmd reloads the population and reports the new facets. The
// status travels inside the command because a handler that returns both a
// message and a status delivers only the message.
func (m *AppModel) projectFilterReloadCmd(status string) tea.Cmd {
	out := m.crossOutliner
	if out == nil {
		return nil
	}
	// The population is about to change underneath the cursor, so restoring the
	// previous row would fight the facet the user just applied.
	out.crossReloadFromTop = true
	return tea.Batch(
		out.reloadCrossProjects(),
		func() tea.Msg { return StatusMsg(status) },
		defaultStatusTimeout(),
	)
}

// newFilterProjectTagsPicker is the Ft multi-select picker over the whole
// project tag vocabulary. It sources the catalog rather than the loaded projects
// so a facet can still name a tag whose only projects the current facets exclude
// — otherwise a filter could never be widened back out from inside the picker.
func (m *AppModel) newFilterProjectTagsPicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	counts, untagged := m.projectTagVocabularyCounts()
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]ListItem, 0, len(names)+1)
	for _, name := range names {
		item := ListItem{
			ID:            ListItemID("tag:" + name),
			CategoryID:    tagDescriptiveCategory,
			PrimaryText:   name,
			SecondaryText: projectCountPhrase(counts[name]),
			Aliases:       []string{name, "#" + name},
			Data:          name,
		}
		if info, behavioral := models.BehavioralProjectTags[name]; behavioral {
			item.CategoryID = projectTagBehavioralCategory
			item.SecondaryText = info.Summary
		}
		items = append(items, item)
	}
	// The empty tag set is legal, so without this row a tag facet could never
	// name the projects the vocabulary does not cover.
	items = append(items, ListItem{
		ID:            ListItemID("tag:" + untaggedProjectFilterKey),
		CategoryID:    tagDescriptiveCategory,
		PrimaryText:   "Untagged",
		SecondaryText: projectCountPhrase(untagged),
		Aliases:       []string{"untagged", "no tags"},
		Data:          untaggedProjectFilterKey,
	})

	source := ListSource{
		Items: items,
		Categories: []ListCategory{
			{ID: projectTagBehavioralCategory, Label: "Behavioral"},
			{ID: tagDescriptiveCategory, Label: "Tags"},
		},
	}
	selected := []ListItemID(nil)
	if m.crossOutliner != nil {
		selected = projectFilterSelection(&m.crossOutliner.projectFilter)
	}
	return newOperationValuePickerModel(
		request,
		ValueProviderFilterProjectTags,
		"Filter Projects by Tag",
		"choose which projects load",
		source,
		true,
		selected,
	)
}

// projectTagVocabularyCounts returns how many projects carry each tag, plus the
// count carrying none. The parked and archived populations are both included so
// the numbers describe the vocabulary rather than the current facets, which the
// picker exists to change.
func (m *AppModel) projectTagVocabularyCounts() (map[string]int, int) {
	counts := make(map[string]int)
	untagged := 0
	if m.runtime == nil {
		return counts, untagged
	}
	for name := range models.BehavioralProjectTags {
		counts[name] = 0
	}
	projects := append(m.availableProjects(), m.runtime.ArchivedProjects()...)
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
