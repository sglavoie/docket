package config

import (
	"slices"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// PinGlobalTag adds a normalized tag to the global pinned tag list.
func (c *Config) PinGlobalTag(tag string) (bool, error) {
	normalized := models.NormalizeTag(tag)
	if err := models.ValidateTag(normalized); err != nil {
		return false, err
	}

	c.normalizeGlobalPinnedTags()
	if slices.Contains(c.GlobalPinnedTags, normalized) {
		return false, nil
	}

	c.GlobalPinnedTags = append(c.GlobalPinnedTags, normalized)
	return true, nil
}

// UnpinGlobalTag removes a normalized tag from the global pinned tag list.
func (c *Config) UnpinGlobalTag(tag string) bool {
	normalized := models.NormalizeTag(tag)
	if normalized == "" {
		return false
	}

	c.normalizeGlobalPinnedTags()
	var removed bool
	c.GlobalPinnedTags, removed = util.SliceRemove(c.GlobalPinnedTags, normalized)
	return removed
}

// ToggleGlobalPinnedTag flips the global pinned state for a normalized tag and
// returns the resulting pinned state.
func (c *Config) ToggleGlobalPinnedTag(tag string) (bool, error) {
	normalized := models.NormalizeTag(tag)
	if err := models.ValidateTag(normalized); err != nil {
		return false, err
	}
	if c.IsGlobalTagPinned(normalized) {
		c.UnpinGlobalTag(normalized)
		return false, nil
	}
	if _, err := c.PinGlobalTag(normalized); err != nil {
		return false, err
	}
	return true, nil
}

// IsGlobalTagPinned reports whether a normalized tag is globally pinned.
func (c *Config) IsGlobalTagPinned(tag string) bool {
	normalized := models.NormalizeTag(tag)
	if normalized == "" {
		return false
	}
	return slices.Contains(models.DedupeTags(c.GlobalPinnedTags), normalized)
}

// GlobalPinnedTagList returns normalized, deduplicated global pinned tags in
// stored order.
func (c *Config) GlobalPinnedTagList() []string {
	return models.DedupeTags(c.GlobalPinnedTags)
}

func (c *Config) normalizeGlobalPinnedTags() {
	c.GlobalPinnedTags = models.DedupeTags(c.GlobalPinnedTags)
}
