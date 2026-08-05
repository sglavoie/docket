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

// ProjectFilterState is the Outliner's population facet: which projects the task
// projection reads at all. It is not a row predicate. A project this state drops
// never enters the loaded snapshot, so no mode, section, or planning projection
// downstream can reach its tasks.
type ProjectFilterState struct {
	// Tags restricts the population to projects carrying one of these tags. The
	// facets compose with OR, unlike the AND semantics of the task tag filter,
	// because a project belongs to its whole tag set at once.
	//
	// Membership of models.ParkedProjectTag is the single population override:
	// it is what Fp toggles and what selecting parked inside Ft sets, so the two
	// routes cannot disagree. Parking is the one behavioral tag, so that entry
	// widens the population rather than restricting it; every other tag is
	// descriptive and restricts.
	Tags map[string]bool

	// Untagged admits projects whose tag set is empty.
	Untagged bool

	// IncludeArchived adds the archived population beneath the active one.
	IncludeArchived bool
}

func newProjectFilterState() ProjectFilterState {
	return ProjectFilterState{Tags: make(map[string]bool)}
}

// clone detaches the tag set so an async load reads the facets as they were when
// it was requested, not as a later edit leaves them.
func (f ProjectFilterState) clone() ProjectFilterState {
	f.Tags = maps.Clone(f.Tags)
	return f
}

// showsParked reports the shared population override. Fp and the parked row
// inside Ft both read and write it here.
func (f *ProjectFilterState) showsParked() bool {
	return f.Tags[models.ParkedProjectTag]
}

func (f *ProjectFilterState) setShowsParked(show bool) {
	if f.Tags == nil {
		f.Tags = make(map[string]bool)
	}
	if show {
		f.Tags[models.ParkedProjectTag] = true
		return
	}
	delete(f.Tags, models.ParkedProjectTag)
}

// restrictionTags returns the descriptive tags the filter narrows by, sorted.
// The reserved parked tag is excluded because it decides the population instead.
func (f *ProjectFilterState) restrictionTags() []string {
	tags := make([]string, 0, len(f.Tags))
	for tag := range f.Tags {
		if tag != models.ParkedProjectTag {
			tags = append(tags, tag)
		}
	}
	sort.Strings(tags)
	return tags
}

func hasActiveProjectFilters(f *ProjectFilterState) bool {
	return len(f.Tags) > 0 || f.Untagged || f.IncludeArchived
}

func clearProjectFilters(f *ProjectFilterState) {
	f.Tags = make(map[string]bool)
	f.Untagged = false
	f.IncludeArchived = false
}

// projectFilterAdmits applies the facets to one candidate project. Archival is
// decided by which population the candidate came from, so this predicate only
// enforces the parked override and the descriptive restriction.
func projectFilterAdmits(project *models.Project, f ProjectFilterState) bool {
	if project == nil {
		return false
	}
	if models.ProjectTagsParked(project.Tags) && !f.showsParked() {
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

// projectFilterParts renders the active facets as scope chrome chips. Parked and
// archived read as inclusions because they widen a population the base scope
// name already claims to exclude.
func projectFilterParts(f *ProjectFilterState) []string {
	var parts []string
	for _, tag := range f.restrictionTags() {
		parts = append(parts, "#"+tag)
	}
	if f.Untagged {
		parts = append(parts, "untagged")
	}
	if f.showsParked() {
		parts = append(parts, "+parked")
	}
	if f.IncludeArchived {
		parts = append(parts, "+archived")
	}
	return parts
}

// projectFilterSelection returns the picker row IDs the current facets select,
// including the untagged pseudo-entry.
func projectFilterSelection(f *ProjectFilterState) []ListItemID {
	selected := make([]ListItemID, 0, len(f.Tags)+1)
	for tag := range f.Tags {
		selected = append(selected, ListItemID("tag:"+tag))
	}
	if f.Untagged {
		selected = append(selected, ListItemID("tag:"+untaggedProjectFilterKey))
	}
	return selected
}

// applyProjectTagFilterValue replaces the tag facets from one picker result. The
// untagged pseudo-entry travels in the same list of names so the picker needs no
// second value channel.
func applyProjectTagFilterValue(f *ProjectFilterState, names []string) {
	tags := make(map[string]bool, len(names))
	untagged := false
	for _, name := range names {
		if name == untaggedProjectFilterKey {
			untagged = true
			continue
		}
		tags[name] = true
	}
	f.Tags = tags
	f.Untagged = untagged
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
