package models

import (
	"slices"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// TagDeletionResult describes what changed when deleting a tag everywhere in a
// project.
type TagDeletionResult struct {
	Name            string
	CatalogChanged  bool
	PinnedChanged   bool
	TasksUnassigned int
}

// Changed reports whether the deletion changed any project or task metadata.
func (r TagDeletionResult) Changed() bool {
	return r.CatalogChanged || r.PinnedChanged || r.TasksUnassigned > 0
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
		normalized := NormalizeTag(tag)
		if ValidateTag(normalized) != nil || seen[normalized] {
			continue
		}
		seen[normalized] = true
		deduped = append(deduped, normalized)
	}
	return deduped
}

// HasCatalogTag reports whether the project's explicit tag catalog contains
// the given tag. Task-used tags are intentionally ignored.
func (p *Project) HasCatalogTag(tag string) bool {
	normalized := NormalizeTag(tag)
	if normalized == "" {
		return false
	}
	return slices.Contains(DedupeTags(p.Tags), normalized)
}

// HasTag reports whether the tag is known to the project through either the
// explicit project catalog or task usage.
func (p *Project) HasTag(tag string) bool {
	normalized := NormalizeTag(tag)
	if normalized == "" {
		return false
	}
	if p.HasCatalogTag(normalized) {
		return true
	}
	for _, task := range p.Tasks {
		if task.HasTag(normalized) {
			return true
		}
	}
	return false
}

// AddTag adds a tag to the explicit project catalog. If the tag is already
// known through the catalog or task usage, it is treated as a duplicate.
func (p *Project) AddTag(tag string) (bool, error) {
	normalized := NormalizeTag(tag)
	if err := ValidateTag(normalized); err != nil {
		return false, err
	}

	catalogChanged := p.normalizeCatalogTags()
	if p.HasTag(normalized) {
		if catalogChanged {
			p.UpdatedAt = time.Now()
		}
		return false, nil
	}

	p.Tags = append(p.Tags, normalized)
	p.UpdatedAt = time.Now()
	return true, nil
}

// RemoveTag removes a tag from the explicit project catalog. Task tags are not
// modified; if tasks still use the tag, it remains part of AllTags().
func (p *Project) RemoveTag(tag string) bool {
	normalized := NormalizeTag(tag)
	if normalized == "" {
		return false
	}

	changed := p.normalizeCatalogTags()
	var removed bool
	p.Tags, removed = util.SliceRemove(p.Tags, normalized)

	if removed && !p.hasTaskTag(normalized) {
		var unpinned bool
		p.PinnedTags, unpinned = util.SliceRemove(DedupeTags(p.PinnedTags), normalized)
		changed = changed || unpinned
	}

	if removed || changed {
		p.UpdatedAt = time.Now()
	}
	return removed
}

// DeleteTag removes a tag from the project catalog, pinned tag metadata, and
// every task in the project.
func (p *Project) DeleteTag(tag string) (TagDeletionResult, error) {
	normalized := NormalizeTag(tag)
	result := TagDeletionResult{Name: normalized}
	if err := ValidateTag(normalized); err != nil {
		return result, err
	}

	var changed bool
	var catalogChanged bool
	p.Tags, catalogChanged = removeNormalizedTagMatches(p.Tags, normalized)
	if catalogChanged {
		result.CatalogChanged = true
		changed = true
	}

	var pinnedChanged bool
	p.PinnedTags, pinnedChanged = removeNormalizedTagMatches(p.PinnedTags, normalized)
	if pinnedChanged {
		result.PinnedChanged = true
		changed = true
	}

	now := time.Now()
	for _, task := range p.Tasks {
		var taskChanged bool
		task.Tags, taskChanged = removeNormalizedTagMatches(task.Tags, normalized)
		if !taskChanged {
			continue
		}
		task.UpdatedAt = now
		result.TasksUnassigned++
		changed = true
	}

	if changed {
		p.UpdatedAt = now
	}
	return result, nil
}

// RenameTag renames a tag in the project catalog and all task usages. If a
// task already has the destination tag, the source tag is removed to avoid a
// duplicate.
func (p *Project) RenameTag(oldName, newName string) (bool, error) {
	oldName = NormalizeTag(oldName)
	if err := ValidateTag(oldName); err != nil {
		return false, err
	}
	newName = NormalizeTag(newName)
	if err := ValidateTag(newName); err != nil {
		return false, err
	}

	changed := p.normalizeCatalogTags()
	if p.normalizePinnedTags() {
		changed = true
	}
	if oldName == newName {
		if changed {
			p.UpdatedAt = time.Now()
		}
		return false, nil
	}

	if p.hasCatalogTag(oldName) {
		p.Tags, _ = util.SliceRemove(p.Tags, oldName)
		if !p.hasCatalogTag(newName) && !p.hasTaskTag(newName) {
			p.Tags = append(p.Tags, newName)
		}
		changed = true
	}

	renamedPins := renameTagInSlice(p.PinnedTags, oldName, newName)
	if !slices.Equal(p.PinnedTags, renamedPins) {
		p.PinnedTags = renamedPins
		changed = true
	}

	now := time.Now()
	for _, task := range p.Tasks {
		if !task.HasTag(oldName) {
			continue
		}
		renamed := renameTagInSlice(task.Tags, oldName, newName)
		if !slices.Equal(task.Tags, renamed) {
			task.Tags = renamed
			task.UpdatedAt = now
			changed = true
		}
	}

	if changed {
		p.UpdatedAt = now
	}
	return changed, nil
}

// PinTag adds a normalized tag to the project's pinned tag list.
func (p *Project) PinTag(tag string) (bool, error) {
	normalized := NormalizeTag(tag)
	if err := ValidateTag(normalized); err != nil {
		return false, err
	}

	pinnedChanged := p.normalizePinnedTags()
	if slices.Contains(p.PinnedTags, normalized) {
		if pinnedChanged {
			p.UpdatedAt = time.Now()
		}
		return false, nil
	}

	p.PinnedTags = append(p.PinnedTags, normalized)
	p.UpdatedAt = time.Now()
	return true, nil
}

// UnpinTag removes a normalized tag from the project's pinned tag list.
func (p *Project) UnpinTag(tag string) bool {
	normalized := NormalizeTag(tag)
	if normalized == "" {
		return false
	}

	changed := p.normalizePinnedTags()
	var removed bool
	p.PinnedTags, removed = util.SliceRemove(p.PinnedTags, normalized)
	if removed || changed {
		p.UpdatedAt = time.Now()
	}
	return removed
}

// TogglePinnedTag flips the pinned state for a normalized tag and returns the
// resulting pinned state.
func (p *Project) TogglePinnedTag(tag string) (bool, error) {
	normalized := NormalizeTag(tag)
	if err := ValidateTag(normalized); err != nil {
		return false, err
	}
	if p.IsTagPinned(normalized) {
		p.UnpinTag(normalized)
		return false, nil
	}
	if _, err := p.PinTag(normalized); err != nil {
		return false, err
	}
	return true, nil
}

// IsTagPinned reports whether a normalized tag is pinned for this project.
func (p *Project) IsTagPinned(tag string) bool {
	normalized := NormalizeTag(tag)
	if normalized == "" {
		return false
	}
	return slices.Contains(DedupeTags(p.PinnedTags), normalized)
}

// PinnedTagList returns the normalized, deduplicated pinned tags in stored order.
func (p *Project) PinnedTagList() []string {
	return DedupeTags(p.PinnedTags)
}

// TagCounts returns all known project tags mapped to task usage count. Tags
// present only in the project catalog are included with a zero count.
func (p *Project) TagCounts() map[string]int {
	counts := make(map[string]int)
	for _, tag := range DedupeTags(p.Tags) {
		counts[tag] = 0
	}
	for _, task := range p.Tasks {
		for _, tag := range DedupeTags(task.Tags) {
			counts[tag]++
		}
	}
	return counts
}

// AllTags returns all known project tags, including zero-use catalog tags and
// tags discovered from tasks, sorted alphabetically.
func (p *Project) AllTags() []string {
	counts := p.TagCounts()
	tags := make([]string, 0, len(counts))
	for tag := range counts {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

func (p *Project) normalizeCatalogTags() bool {
	deduped := DedupeTags(p.Tags)
	if slices.Equal(p.Tags, deduped) {
		return false
	}
	p.Tags = deduped
	return true
}

func (p *Project) normalizePinnedTags() bool {
	deduped := DedupeTags(p.PinnedTags)
	if slices.Equal(p.PinnedTags, deduped) {
		return false
	}
	p.PinnedTags = deduped
	return true
}

func (p *Project) hasCatalogTag(normalized string) bool {
	return slices.Contains(p.Tags, normalized)
}

func (p *Project) hasTaskTag(normalized string) bool {
	for _, task := range p.Tasks {
		if task.HasTag(normalized) {
			return true
		}
	}
	return false
}

func removeNormalizedTagMatches(tags []string, normalized string) ([]string, bool) {
	if len(tags) == 0 {
		return tags, false
	}
	filtered := make([]string, 0, len(tags))
	var removed bool
	for _, tag := range tags {
		if NormalizeTag(tag) == normalized {
			removed = true
			continue
		}
		filtered = append(filtered, tag)
	}
	if !removed {
		return tags, false
	}
	return filtered, true
}

func renameTagInSlice(tags []string, oldName, newName string) []string {
	renamed := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		normalized := NormalizeTag(tag)
		if normalized == oldName {
			normalized = newName
		}
		if ValidateTag(normalized) != nil || seen[normalized] {
			continue
		}
		seen[normalized] = true
		renamed = append(renamed, normalized)
	}
	return renamed
}
