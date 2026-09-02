package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// populationFilterClauseEdit is one clause constructor per population facet.
// Each toggle reads the axis the derived cache currently holds and spells the
// mode the key selects, so Fp, FP, and the F# picker's parked row cannot leave
// the population claiming different things. Like the row facets, it mutates
// nothing: applyFilterClauseEdit in filter_operations.go is the one writer.
func (m *OutlinerModel) populationFilterClauseEdit(
	invocation OperationInvocation,
) (filterClauseEdit, error) {
	switch invocation.Descriptor.ID {
	case operationFilterProjectTags:
		names, ok := invocation.Value.([]string)
		if !ok {
			return filterClauseEdit{}, fmt.Errorf("invalid project tag filter value")
		}
		next := m.projectFilter.clone()
		applyProjectTagFilterValue(&next, names)
		clauses := append(projectTagClauses(&next),
			populationClauses(models.QueryFieldParked, next.ParkedMode)...)
		return filterClauseEdit{clauses: clauses, status: projectTagFilterStatus(&next)}, nil
	case operationFilterShowParked:
		return m.populationToggleEdit(models.QueryFieldParked, m.projectFilter.ParkedMode,
			projectPopulationInclude, "Parked"), nil
	case operationFilterOnlyParked:
		return m.populationToggleEdit(models.QueryFieldParked, m.projectFilter.ParkedMode,
			projectPopulationOnly, "Parked"), nil
	case operationFilterIncludeArchived:
		return m.populationToggleEdit(models.QueryFieldArchived, m.projectFilter.ArchivedMode,
			projectPopulationInclude, "Archived"), nil
	case operationFilterOnlyArchived:
		return m.populationToggleEdit(models.QueryFieldArchived, m.projectFilter.ArchivedMode,
			projectPopulationOnly, "Archived"), nil
	case operationFilterProjectClear:
		if !hasActiveProjectFilters(&m.projectFilter) {
			return filterClauseEdit{declined: true, status: "No project filters are active"}, nil
		}
		return filterClauseEdit{status: "Project filters cleared"}, nil
	default:
		return filterClauseEdit{}, fmt.Errorf(
			"unknown project filter operation %s", invocation.Descriptor.ID)
	}
}

func (m *OutlinerModel) populationToggleEdit(
	field models.QueryField,
	current, target projectPopulationMode,
	name string,
) filterClauseEdit {
	mode := toggleProjectPopulation(current, target)
	return filterClauseEdit{
		clauses: populationClauses(field, mode),
		status:  projectPopulationStatus(mode, name),
	}
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
