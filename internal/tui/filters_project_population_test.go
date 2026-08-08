package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestProjectFilterAdmitsAcrossPopulationModes(t *testing.T) {
	plain := &models.Project{ID: "plain", Name: "Plain"}
	parked := &models.Project{ID: "parked", Name: "Parked", Tags: []string{models.ParkedProjectTag}}
	archived := &models.Project{ID: "archived", Name: "Archived", Archived: true}
	both := &models.Project{ID: "both", Name: "Both", Archived: true, Tags: []string{models.ParkedProjectTag}}

	modes := []struct {
		name string
		mode projectPopulationMode
	}{
		{"exclude", projectPopulationExclude},
		{"include", projectPopulationInclude},
		{"only", projectPopulationOnly},
	}

	for _, parkedMode := range modes {
		for _, archivedMode := range modes {
			filter := newProjectFilterState()
			filter.ParkedMode = parkedMode.mode
			filter.ArchivedMode = archivedMode.mode

			for _, candidate := range []*models.Project{plain, parked, archived, both} {
				isParked := models.ProjectTagsParked(candidate.Tags)
				want := projectPopulationAdmits(parkedMode.mode, isParked) &&
					projectPopulationAdmits(archivedMode.mode, candidate.Archived)
				if got := projectFilterAdmits(candidate, filter); got != want {
					t.Errorf("parked=%s archived=%s admits %s = %v, want %v",
						parkedMode.name, archivedMode.name, candidate.Name, got, want)
				}
			}
		}
	}
}

// The Ft picker's parked row is a checkbox, so it cannot express the difference
// between including and isolating. Confirming it must therefore preserve an
// isolation the F keys set rather than widening the population behind the user.
func TestProjectTagFilterValuePreservesIsolatedParkedPopulation(t *testing.T) {
	cases := []struct {
		name    string
		initial projectPopulationMode
		names   []string
		want    projectPopulationMode
	}{
		{"checked row keeps only", projectPopulationOnly, []string{models.ParkedProjectTag}, projectPopulationOnly},
		{"checked row widens exclude", projectPopulationExclude, []string{models.ParkedProjectTag}, projectPopulationInclude},
		{"checked row keeps include", projectPopulationInclude, []string{models.ParkedProjectTag}, projectPopulationInclude},
		{"unchecked row clears only", projectPopulationOnly, []string{"client"}, projectPopulationExclude},
		{"unchecked row clears include", projectPopulationInclude, nil, projectPopulationExclude},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			filter := newProjectFilterState()
			filter.ParkedMode = testCase.initial
			applyProjectTagFilterValue(&filter, testCase.names)
			if filter.ParkedMode != testCase.want {
				t.Errorf("ParkedMode = %v, want %v", filter.ParkedMode, testCase.want)
			}
			if filter.Tags[models.ParkedProjectTag] {
				t.Error("the reserved parked tag leaked into the descriptive restriction")
			}
		})
	}
}

func TestProjectFilterSelectionRoundTripsThroughThePicker(t *testing.T) {
	filter := newProjectFilterState()
	filter.Tags = map[string]bool{"client": true}
	filter.Untagged = true
	filter.ParkedMode = projectPopulationOnly

	selection := projectFilterSelection(&filter)
	for _, want := range []ListItemID{"tag:client", "tag:" + models.ParkedProjectTag, "tag:" + untaggedProjectFilterKey} {
		if !slices.Contains(selection, want) {
			t.Errorf("selection %v omits %q", selection, want)
		}
	}
}

func TestProjectFilterPartsNamePopulationModes(t *testing.T) {
	filter := newProjectFilterState()
	filter.ParkedMode = projectPopulationInclude
	filter.ArchivedMode = projectPopulationOnly

	got := projectFilterParts(&filter)
	want := []string{"+parked", "archived:only"}
	if !slices.Equal(got, want) {
		t.Errorf("projectFilterParts() = %v, want %v", got, want)
	}
	if count := projectFilterFacetCount(&filter); count != 2 {
		t.Errorf("facet count = %d, want one facet per active axis", count)
	}
}
