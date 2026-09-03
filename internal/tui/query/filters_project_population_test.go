package query

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
		mode ProjectPopulationMode
	}{
		{"exclude", ProjectPopulationExclude},
		{"include", ProjectPopulationInclude},
		{"only", ProjectPopulationOnly},
	}

	for _, parkedMode := range modes {
		for _, archivedMode := range modes {
			filter := NewProjectFilterState()
			filter.ParkedMode = parkedMode.mode
			filter.ArchivedMode = archivedMode.mode

			for _, candidate := range []*models.Project{plain, parked, archived, both} {
				isParked := models.ProjectTagsParked(candidate.Tags)
				want := projectPopulationAdmits(parkedMode.mode, isParked) &&
					projectPopulationAdmits(archivedMode.mode, candidate.Archived)
				if got := ProjectFilterAdmits(candidate, filter); got != want {
					t.Errorf("parked=%s archived=%s admits %s = %v, want %v",
						parkedMode.name, archivedMode.name, candidate.Name, got, want)
				}
			}
		}
	}
}

// The F# picker's parked row is a checkbox, so it cannot express the difference
// between including and isolating. Confirming it must therefore preserve an
// isolation the F keys set rather than widening the population behind the user.
func TestProjectTagFilterValuePreservesIsolatedParkedPopulation(t *testing.T) {
	cases := []struct {
		name    string
		initial ProjectPopulationMode
		names   []string
		want    ProjectPopulationMode
	}{
		{"checked row keeps only", ProjectPopulationOnly, []string{models.ParkedProjectTag}, ProjectPopulationOnly},
		{"checked row widens exclude", ProjectPopulationExclude, []string{models.ParkedProjectTag}, ProjectPopulationInclude},
		{"checked row keeps include", ProjectPopulationInclude, []string{models.ParkedProjectTag}, ProjectPopulationInclude},
		{"unchecked row clears only", ProjectPopulationOnly, []string{"client"}, ProjectPopulationExclude},
		{"unchecked row clears include", ProjectPopulationInclude, nil, ProjectPopulationExclude},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			filter := NewProjectFilterState()
			filter.ParkedMode = testCase.initial
			ApplyProjectTagFilterValue(&filter, testCase.names)
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
	filter := NewProjectFilterState()
	filter.Tags = map[string]bool{"client": true}
	filter.Untagged = true
	filter.ParkedMode = ProjectPopulationOnly

	selection := ProjectFilterSelection(&filter)
	for _, want := range []string{"tag:client", "tag:" + models.ParkedProjectTag, "tag:" + UntaggedProjectFilterKey} {
		if !slices.Contains(selection, want) {
			t.Errorf("selection %v omits %q", selection, want)
		}
	}
}

func TestProjectFilterPartsNamePopulationModes(t *testing.T) {
	filter := NewProjectFilterState()
	filter.ParkedMode = ProjectPopulationInclude
	filter.ArchivedMode = ProjectPopulationOnly

	got := ProjectFilterParts(&filter)
	want := []string{"+parked", "+archived:only"}
	if !slices.Equal(got, want) {
		t.Errorf("ProjectFilterParts() = %v, want %v", got, want)
	}
	if count := ProjectFilterFacetCount(&filter); count != 2 {
		t.Errorf("facet count = %d, want one facet per active axis", count)
	}
}
