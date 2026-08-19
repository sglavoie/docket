package tui

import (
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// untaggedProjectFilterKey is the pseudo-entry the project tag picker offers for
// projects carrying no tags at all. The empty tag set is legal, so without it a
// tag filter could never name the projects the vocabulary does not cover.
const untaggedProjectFilterKey = "\x00untagged"

// projectPopulationMode is how one population axis answers a lifecycle question.
// Exclude is the base workspace, Include widens it, and Only isolates it, so a
// population the base scope hides can be audited without being mixed back in.
type projectPopulationMode int

const (
	projectPopulationExclude projectPopulationMode = iota
	projectPopulationInclude
	projectPopulationOnly
)

// ProjectFilterState is the Outliner's population facet: which projects the task
// projection reads at all. It is not a row predicate. A project this state drops
// never enters the loaded snapshot, so no mode, section, or planning projection
// downstream can reach its tasks.
type ProjectFilterState struct {
	// Tags restricts the population to projects carrying one of these tags. The
	// facets compose with OR, unlike the AND semantics of the task tag filter,
	// because a project belongs to its whole tag set at once. Every tag here is
	// descriptive; the reserved parked tag decides a population instead and
	// lives in ParkedMode.
	Tags map[string]bool

	// Untagged admits projects whose tag set is empty.
	Untagged bool

	// ParkedMode is the parked population axis. Fp, FP, and the parked row inside
	// F# all write this one field, so the routes cannot disagree.
	ParkedMode projectPopulationMode

	// ArchivedMode is the archived population axis. It composes with ParkedMode
	// by AND, like every other facet.
	ArchivedMode projectPopulationMode
}

func newProjectFilterState() ProjectFilterState {
	return ProjectFilterState{Tags: make(map[string]bool)}
}

// toggleProjectPopulation gives each key one state to own: pressing it selects
// that mode, pressing it again while it already holds clears the axis.
func toggleProjectPopulation(current, target projectPopulationMode) projectPopulationMode {
	if current == target {
		return projectPopulationExclude
	}
	return target
}

// projectPopulationAdmits applies one axis to one candidate.
func projectPopulationAdmits(mode projectPopulationMode, carries bool) bool {
	switch mode {
	case projectPopulationOnly:
		return carries
	case projectPopulationInclude:
		return true
	default:
		return !carries
	}
}

// clone detaches the tag set so an async load reads the facets as they were when
// it was requested, not as a later edit leaves them.
func (f ProjectFilterState) clone() ProjectFilterState {
	f.Tags = maps.Clone(f.Tags)
	return f
}

// showsParked reports whether the parked population is loaded at all, under
// either Include or Only.
func (f *ProjectFilterState) showsParked() bool {
	return f.ParkedMode != projectPopulationExclude
}

// restrictionTags returns the descriptive tags the filter narrows by, sorted.
func (f *ProjectFilterState) restrictionTags() []string {
	tags := make([]string, 0, len(f.Tags))
	for tag := range f.Tags {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

func hasActiveProjectFilters(f *ProjectFilterState) bool {
	return len(f.Tags) > 0 ||
		f.Untagged ||
		f.ParkedMode != projectPopulationExclude ||
		f.ArchivedMode != projectPopulationExclude
}

// projectFilterFacetCount reports independently clearable population facets.
// Descriptive tags and Untagged share one picker and therefore count once; the
// parked and archived axes are one facet each however many keys reach them.
func projectFilterFacetCount(f *ProjectFilterState) int {
	if f == nil {
		return 0
	}
	count := 0
	if len(f.restrictionTags()) > 0 || f.Untagged {
		count++
	}
	if f.ParkedMode != projectPopulationExclude {
		count++
	}
	if f.ArchivedMode != projectPopulationExclude {
		count++
	}
	return count
}

func clearProjectFilters(f *ProjectFilterState) {
	f.Tags = make(map[string]bool)
	f.Untagged = false
	f.ParkedMode = projectPopulationExclude
	f.ArchivedMode = projectPopulationExclude
}

// projectFilterAdmits applies the facets to one candidate project. Both
// lifecycle axes are enforced here, so the candidate populations only decide
// presentation order and how much is loaded, never what is admitted.
func projectFilterAdmits(project *models.Project, f ProjectFilterState) bool {
	if project == nil {
		return false
	}
	if !projectPopulationAdmits(f.ParkedMode, models.ProjectTagsParked(project.Tags)) {
		return false
	}
	if !projectPopulationAdmits(f.ArchivedMode, project.Archived) {
		return false
	}
	restriction := f.restrictionTags()
	if len(restriction) == 0 && !f.Untagged {
		return true
	}
	if f.Untagged && len(project.Tags) == 0 {
		return true
	}
	for _, tag := range restriction {
		if slices.Contains(project.Tags, tag) {
			return true
		}
	}
	return false
}

// projectFilterParts renders the active facets as scope chrome chips. An
// included population reads as an addition to the base scope name; an isolated
// one reads as a replacement, because the base name no longer describes it.
func projectFilterParts(f *ProjectFilterState) []string {
	var parts []string
	for _, tag := range f.restrictionTags() {
		parts = append(parts, "#"+tag)
	}
	if f.Untagged {
		parts = append(parts, "untagged")
	}
	parts = append(parts, projectPopulationPart(f.ParkedMode, "parked")...)
	parts = append(parts, projectPopulationPart(f.ArchivedMode, "archived")...)
	return parts
}

func projectPopulationPart(mode projectPopulationMode, name string) []string {
	switch mode {
	case projectPopulationOnly:
		return []string{name + ":only"}
	case projectPopulationInclude:
		return []string{"+" + name}
	default:
		return nil
	}
}

// projectFilterSelection returns the picker row IDs the current facets select,
// including the parked behavioral row and the untagged pseudo-entry.
func projectFilterSelection(f *ProjectFilterState) []ListItemID {
	selected := make([]ListItemID, 0, len(f.Tags)+2)
	for tag := range f.Tags {
		selected = append(selected, ListItemID("tag:"+tag))
	}
	if f.showsParked() {
		selected = append(selected, ListItemID("tag:"+models.ParkedProjectTag))
	}
	if f.Untagged {
		selected = append(selected, ListItemID("tag:"+untaggedProjectFilterKey))
	}
	return selected
}

// applyProjectTagFilterValue replaces the tag facets from one picker result. The
// parked row and the untagged pseudo-entry travel in the same list of names so
// the picker needs no second value channel. A checked parked row that is already
// isolating keeps Only, because the picker offers no way to say Include instead
// and silently widening the population would answer a question nobody asked.
func applyProjectTagFilterValue(f *ProjectFilterState, names []string) {
	tags := make(map[string]bool, len(names))
	untagged := false
	parked := false
	for _, name := range names {
		switch name {
		case untaggedProjectFilterKey:
			untagged = true
		case models.ParkedProjectTag:
			parked = true
		default:
			tags[name] = true
		}
	}
	f.Tags = tags
	f.Untagged = untagged
	switch {
	case !parked:
		f.ParkedMode = projectPopulationExclude
	case f.ParkedMode == projectPopulationExclude:
		f.ParkedMode = projectPopulationInclude
	}
}

// projectTagFilterStatus names the resulting population rather than the count of
// rows chosen, because a tag facet changes which projects load at all.
func projectTagFilterStatus(f *ProjectFilterState) string {
	parts := projectFilterParts(f)
	if len(parts) == 0 {
		return "Project filter cleared"
	}
	return "Projects: " + strings.Join(parts, " ")
}

// projectFilterFingerprint contributes the facets to the row memoization guard,
// so a population change can never be short-circuited by an unchanged task count.
func projectFilterFingerprint(f *ProjectFilterState) string {
	return strings.Join(projectFilterParts(f), "\x1f")
}
