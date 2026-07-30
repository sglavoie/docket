package models

import (
	"sort"
)

// DedupeTags normalizes, validates, and deduplicates tags while preserving the
// first valid occurrence order.
func DedupeTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}
	seen := make(map[string]bool, len(tags))
	deduped := make([]string, 0, len(tags))
	for _, tag := range tags {
		normalized := NormalizeTag(tag)
		if ValidateTag(normalized) != nil || seen[normalized] {
			continue
		}
		seen[normalized] = true
		deduped = append(deduped, normalized)
	}
	return deduped
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
