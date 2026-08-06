package models

import (
	"slices"
	"sort"
	"strings"
)

// ParkedProjectTag is the reserved project tag that parks a project: the
// project keeps its tasks, notes, and attachments but leaves the task
// workspace. It is hard reserved in the project-tag namespace only, so a task
// may still carry an ordinary "parked" tag with no behavioral meaning. Input is
// case-insensitive because NormalizeProjectTags folds the whole set, and the
// canonical form at rest is this lowercase spelling.
const ParkedProjectTag = "parked"

// BehavioralProjectTag describes one project tag that decides something instead
// of merely labeling. It is presentation metadata only: the parking effect
// itself stays one hardcoded predicate behind Runtime.TaskWorkspaceProjects().
type BehavioralProjectTag struct {
	// Name is the canonical lowercase tag spelling.
	Name string
	// RowSummary is the compact description shown beside a focusable picker row.
	RowSummary string
	// Summary is the complete explainer shown in non-focusable help content.
	Summary string
	// BadgeRole names the semantic style role the badge renders with.
	BadgeRole string
}

// BehavioralProjectTags is the single source for every surface that explains a
// behavioral project tag: the picker explainer, the badge, the help line, and
// the status text.
var BehavioralProjectTags = map[string]BehavioralProjectTag{
	ParkedProjectTag: {
		Name:       ParkedProjectTag,
		RowSummary: "Parks this project",
		Summary:    "Parks this project — keeps its tasks, notes and attachments, hides them from Active Projects.",
		BadgeRole:  "warning",
	},
}

// DedupeTags normalizes, validates, and deduplicates tags while preserving the
// first valid occurrence order.
func DedupeTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}
	seen := make(map[string]bool, len(tags))
	deduped := make([]string, 0, len(tags))
	for _, tag := range tags {
		normalized, err := NormalizeTaskTag(tag)
		if err != nil || seen[normalized] {
			continue
		}
		seen[normalized] = true
		deduped = append(deduped, normalized)
	}
	return deduped
}

// NormalizeProjectTags validates a complete project tag set with the ordinary
// tag rules, then folds it to lowercase, dedupes, and sorts. Folding makes the
// namespace case-insensitive, which both predecessors of project tags already
// were, and sorting keeps the persisted catalog bytes deterministic. An empty
// tag set is legal. Any tag invalid as an ordinary tag fails the whole set.
func NormalizeProjectTags(tags []string) ([]string, error) {
	validated, err := NormalizeTaskTags(tags)
	if err != nil {
		return nil, err
	}
	normalized := make([]string, 0, len(validated))
	seen := make(map[string]bool, len(validated))
	for _, tag := range validated {
		folded := strings.ToLower(tag)
		if seen[folded] {
			continue
		}
		seen[folded] = true
		normalized = append(normalized, folded)
	}
	sort.Strings(normalized)
	return normalized, nil
}

// ProjectTagsParked reports whether a project tag set carries the reserved
// parked tag. It is the single predicate behind the only behavioral project
// tag, so callers never compare against the literal spelling. The set must
// already be normalized; NormalizeProjectTags is what makes the input
// case-insensitive.
func ProjectTagsParked(tags []string) bool {
	return slices.Contains(tags, ParkedProjectTag)
}

// HasTag reports whether any live task in the project uses the tag.
func (p *Project) HasTag(tag string) bool {
	normalized := NormalizeTag(tag)
	if normalized == "" {
		return false
	}
	for _, task := range p.Tasks {
		if task.HasTag(normalized) {
			return true
		}
	}
	return false
}

// TagCounts returns task-derived tag usage counts.
func (p *Project) TagCounts() map[string]int {
	counts := make(map[string]int)
	for _, task := range p.Tasks {
		for _, tag := range DedupeTags(task.Tags) {
			counts[tag]++
		}
	}
	return counts
}

// AllTags returns the task-derived project tag vocabulary sorted alphabetically.
func (p *Project) AllTags() []string {
	counts := p.TagCounts()
	tags := make([]string, 0, len(counts))
	for tag := range counts {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}
