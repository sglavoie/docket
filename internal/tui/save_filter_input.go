package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func describeSavedFilterSummary(sf *config.SavedFilter) string {
	if sf.Mode == "criteria" {
		return describeCriteriaFilter(sf)
	}
	return describeSnapshotFilters(
		sf.FilterType, sf.FilterStates, sf.HasStateFilter(), sf.FilterPriority,
		sf.FilterTags, sf.FilterDueDate,
		sf.FilterActionable, sf.FilterHideDimmed, sf.FilterGitHubIssue,
		sf.FilterParked,
		sf.FilterAttrs, sf.FilterScoreMin,
		0, 0,
	)
}

func describeCriteriaFilter(sf *config.SavedFilter) string {
	if len(sf.Criteria) == 0 {
		return "No criteria"
	}
	var parts []string
	for _, criterion := range sf.Criteria {
		if criterion.Field == "state" {
			values := criterion.States.Values()
			states := make([]string, len(values))
			for index, state := range values {
				states[index] = string(state)
			}
			parts = append(parts, criterion.Field+" "+criterion.Operation+" "+strings.Join(states, ","))
			continue
		}
		parts = append(parts, criterion.Field+" "+criterion.Operation+" "+criterion.Value)
	}
	separator := " AND "
	if sf.EffectiveLogic() == "or" {
		separator = " OR "
	}
	return strings.Join(parts, separator)
}

func describeSnapshotFilters(
	filterType string,
	filterStates models.WorkflowStateSet,
	hasStateFilter bool,
	filterPriority int,
	filterTags []string,
	filterDueDate int,
	filterActionable bool,
	filterHideDimmed bool,
	filterGitHubIssue bool,
	filterParked bool,
	filterAttrs map[string]string,
	filterScoreMin float64,
	sortField int,
	viewMode int,
) string {
	var parts []string

	switch viewMode {
	case 1:
		parts = append(parts, "Next")
	case 2:
		parts = append(parts, "Inbox")
	case 3:
		parts = append(parts, "Waiting")
	case 4:
		parts = append(parts, "Someday")
	case 5:
		parts = append(parts, "Completed")
	case 6:
		parts = append(parts, "Snoozed")
	case 7:
		parts = append(parts, "All")
	}

	if filterPriority == 1 {
		parts = append(parts, "!Hi")
	}
	if filterType != "" {
		parts = append(parts, "type:"+filterType)
	}
	if hasStateFilter {
		values := filterStates.Values()
		states := make([]string, len(values))
		for index, state := range values {
			states[index] = string(state)
		}
		value := "none"
		if len(states) > 0 {
			value = strings.Join(states, ",")
		}
		parts = append(parts, "state:"+value)
	}
	for _, tag := range filterTags {
		parts = append(parts, "#"+tag)
	}
	switch filterDueDate {
	case 1:
		parts = append(parts, "due:overdue")
	case 2:
		parts = append(parts, "due:today")
	case 3:
		parts = append(parts, "due:week")
	case 4:
		parts = append(parts, "due:none")
	}
	if filterActionable {
		parts = append(parts, "ready")
	}
	if filterHideDimmed {
		parts = append(parts, "no-dim")
	}
	if filterGitHubIssue {
		parts = append(parts, "issue")
	}
	if filterParked {
		parts = append(parts, "parked")
	}
	if len(filterAttrs) > 0 {
		keys := make([]string, 0, len(filterAttrs))
		for key := range filterAttrs {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			parts = append(parts, key+"="+filterAttrs[key])
		}
	}
	if filterScoreMin > 0 {
		parts = append(parts, fmt.Sprintf("score>=%.0f", filterScoreMin))
	}

	sortNames := map[int]string{1: "priority", 2: "due date", 3: "created", 4: "status", 5: "score"}
	if name, ok := sortNames[sortField]; ok {
		parts = append(parts, "sort:"+name)
	}
	if len(parts) == 0 {
		return "Active (default)"
	}
	return strings.Join(parts, " | ")
}
