package query

import (
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// UntaggedProjectFilterKey is the pseudo-entry the project tag picker offers for
// projects carrying no tags at all. The empty tag set is legal, so without it a
// tag filter could never name the projects the vocabulary does not cover.
const UntaggedProjectFilterKey = "\x00untagged"

// ProjectPopulationMode is how one population axis answers a lifecycle question.
// Exclude is the base workspace, Include widens it, and Only isolates it, so a
// population the base scope hides can be audited without being mixed back in.
type ProjectPopulationMode int

const (
	ProjectPopulationExclude ProjectPopulationMode = iota
	ProjectPopulationInclude
	ProjectPopulationOnly
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
	ParkedMode ProjectPopulationMode

	// ArchivedMode is the archived population axis. It composes with ParkedMode
	// by AND, like every other facet.
	ArchivedMode ProjectPopulationMode
}

func NewProjectFilterState() ProjectFilterState {
	return ProjectFilterState{Tags: make(map[string]bool)}
}

// ToggleProjectPopulation gives each key one state to own: pressing it selects
// that mode, pressing it again while it already holds clears the axis.
func ToggleProjectPopulation(current, target ProjectPopulationMode) ProjectPopulationMode {
	if current == target {
		return ProjectPopulationExclude
	}
	return target
}

// projectPopulationAdmits applies one axis to one candidate.
func projectPopulationAdmits(mode ProjectPopulationMode, carries bool) bool {
	switch mode {
	case ProjectPopulationOnly:
		return carries
	case ProjectPopulationInclude:
		return true
	default:
		return !carries
	}
}

// clone detaches the tag set so an async load reads the facets as they were when
// it was requested, not as a later edit leaves them.
func (f ProjectFilterState) Clone() ProjectFilterState {
	f.Tags = maps.Clone(f.Tags)
	return f
}

// showsParked reports whether the parked population is loaded at all, under
// either Include or Only.
func (f *ProjectFilterState) ShowsParked() bool {
	return f.ParkedMode != ProjectPopulationExclude
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

func HasActiveProjectFilters(f *ProjectFilterState) bool {
	return len(f.Tags) > 0 ||
		f.Untagged ||
		f.ParkedMode != ProjectPopulationExclude ||
		f.ArchivedMode != ProjectPopulationExclude
}

// ProjectFilterFacetCount reports independently clearable population facets.
// Descriptive tags and Untagged share one picker and therefore count once; the
// parked and archived axes are one facet each however many keys reach them.
func ProjectFilterFacetCount(f *ProjectFilterState) int {
	if f == nil {
		return 0
	}
	count := 0
	if len(f.restrictionTags()) > 0 || f.Untagged {
		count++
	}
	if f.ParkedMode != ProjectPopulationExclude {
		count++
	}
	if f.ArchivedMode != ProjectPopulationExclude {
		count++
	}
	return count
}

// ProjectLifecycleAdmits applies the two lifecycle axes alone. It is the half of
// the population rule that answers whether the workspace still offers a project
// to this view at all, as opposed to whether a descriptive criterion selected
// it: parked and archived are populations, while a tag restriction is criteria a
// view may narrow by and widen out of again. A caller restoring a retained
// membership wants exactly this half, so the axes live in one function and
// ProjectFilterAdmits is that answer plus the tag restriction.
func ProjectLifecycleAdmits(project *models.Project, f ProjectFilterState) bool {
	if project == nil {
		return false
	}
	return projectPopulationAdmits(f.ParkedMode, models.ProjectTagsParked(project.Tags)) &&
		projectPopulationAdmits(f.ArchivedMode, project.Archived)
}

// ProjectFilterAdmits applies the facets to one candidate project. Both
// lifecycle axes are enforced through ProjectLifecycleAdmits above, so the
// candidate populations only decide presentation order and how much is loaded,
// never what is admitted.
func ProjectFilterAdmits(project *models.Project, f ProjectFilterState) bool {
	if !ProjectLifecycleAdmits(project, f) {
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

// ProjectFilterParts renders the active facets as scope chrome chips. The chips
// are the population clauses' own canonical sigil text: an included population
// reads as an addition to the base scope name, an isolated one as a replacement,
// because the base name no longer describes it.
func ProjectFilterParts(f *ProjectFilterState) []string {
	return FilterClauseParts(projectFilterClauses(f))
}

// ProjectFilterSelection returns the picker row IDs the current facets select,
// including the parked behavioral row and the untagged pseudo-entry. The rows
// are named as plain strings and converted by the one picker that seeds itself
// from them, because a list row's identity belongs to the surface that draws it
// rather than to the facets it reports.
func ProjectFilterSelection(f *ProjectFilterState) []string {
	selected := make([]string, 0, len(f.Tags)+2)
	for tag := range f.Tags {
		selected = append(selected, "tag:"+tag)
	}
	if f.ShowsParked() {
		selected = append(selected, "tag:"+models.ParkedProjectTag)
	}
	if f.Untagged {
		selected = append(selected, "tag:"+UntaggedProjectFilterKey)
	}
	return selected
}

// ApplyProjectTagFilterValue replaces the tag facets from one picker result. The
// parked row and the untagged pseudo-entry travel in the same list of names so
// the picker needs no second value channel. A checked parked row that is already
// isolating keeps Only, because the picker offers no way to say Include instead
// and silently widening the population would answer a question nobody asked.
func ApplyProjectTagFilterValue(f *ProjectFilterState, names []string) {
	tags := make(map[string]bool, len(names))
	untagged := false
	parked := false
	for _, name := range names {
		switch name {
		case UntaggedProjectFilterKey:
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
		f.ParkedMode = ProjectPopulationExclude
	case f.ParkedMode == ProjectPopulationExclude:
		f.ParkedMode = ProjectPopulationInclude
	}
}

// ProjectTagFilterStatus names the resulting population rather than the count of
// rows chosen, because a tag facet changes which projects load at all.
func ProjectTagFilterStatus(f *ProjectFilterState) string {
	parts := ProjectFilterParts(f)
	if len(parts) == 0 {
		return "Project filter cleared"
	}
	return "Projects: " + strings.Join(parts, " ")
}

// ProjectFilterFingerprint contributes the facets to the row memoization guard,
// so a population change can never be short-circuited by an unchanged task count.
func ProjectFilterFingerprint(f *ProjectFilterState) string {
	return strings.Join(ProjectFilterParts(f), "\x1f")
}
