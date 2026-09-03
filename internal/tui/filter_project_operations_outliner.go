package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/query"
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
		next := m.projectFilter.Clone()
		query.ApplyProjectTagFilterValue(&next, names)
		clauses := append(query.ProjectTagClauses(&next),
			query.PopulationClauses(models.QueryFieldParked, next.ParkedMode)...)
		return filterClauseEdit{clauses: clauses, status: query.ProjectTagFilterStatus(&next)}, nil
	case operationFilterShowParked:
		return m.populationToggleEdit(models.QueryFieldParked, m.projectFilter.ParkedMode,
			query.ProjectPopulationInclude, "Parked"), nil
	case operationFilterOnlyParked:
		return m.populationToggleEdit(models.QueryFieldParked, m.projectFilter.ParkedMode,
			query.ProjectPopulationOnly, "Parked"), nil
	case operationFilterIncludeArchived:
		return m.populationToggleEdit(models.QueryFieldArchived, m.projectFilter.ArchivedMode,
			query.ProjectPopulationInclude, "Archived"), nil
	case operationFilterOnlyArchived:
		return m.populationToggleEdit(models.QueryFieldArchived, m.projectFilter.ArchivedMode,
			query.ProjectPopulationOnly, "Archived"), nil
	case operationFilterProjectClear:
		if !query.HasActiveProjectFilters(&m.projectFilter) {
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
	current, target query.ProjectPopulationMode,
	name string,
) filterClauseEdit {
	mode := query.ToggleProjectPopulation(current, target)
	return filterClauseEdit{
		clauses: query.PopulationClauses(field, mode),
		status:  projectPopulationStatus(mode, name),
	}
}

// projectPopulationStatus names the population one axis now loads rather than
// the key that was pressed, because two keys reach the same three modes.
func projectPopulationStatus(mode query.ProjectPopulationMode, name string) string {
	switch mode {
	case query.ProjectPopulationOnly:
		return "Showing only " + strings.ToLower(name) + " projects"
	case query.ProjectPopulationInclude:
		return "Showing " + strings.ToLower(name) + " projects"
	default:
		return name + " projects hidden"
	}
}
