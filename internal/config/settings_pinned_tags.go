package config

import (
	"slices"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// This file mirrors pinned_tags.go onto the Settings domain, which owns
// GlobalPinnedTags, so the TUI tag handlers and pinned-tag bars can read and
// mutate the global pin list through the owning domain rather than the
// config.Config facade.

// PinGlobalTag adds a normalized tag to the global pinned tag list.
func (s *Settings) PinGlobalTag(tag string) (bool, error) {
	normalized := models.NormalizeTag(tag)
	if err := models.ValidateTag(normalized); err != nil {
		return false, err
	}

	s.normalizeGlobalPinnedTags()
	if slices.Contains(s.GlobalPinnedTags, normalized) {
		return false, nil
	}

	s.GlobalPinnedTags = append(s.GlobalPinnedTags, normalized)
	return true, nil
}

// UnpinGlobalTag removes a normalized tag from the global pinned tag list.
func (s *Settings) UnpinGlobalTag(tag string) bool {
	normalized := models.NormalizeTag(tag)
	if normalized == "" {
		return false
	}

	s.normalizeGlobalPinnedTags()
	var removed bool
	s.GlobalPinnedTags, removed = util.SliceRemove(s.GlobalPinnedTags, normalized)
	return removed
}

// ToggleGlobalPinnedTag flips the global pinned state for a normalized tag and
// returns the resulting pinned state.
func (s *Settings) ToggleGlobalPinnedTag(tag string) (bool, error) {
	normalized := models.NormalizeTag(tag)
	if err := models.ValidateTag(normalized); err != nil {
		return false, err
	}
	if s.IsGlobalTagPinned(normalized) {
		s.UnpinGlobalTag(normalized)
		return false, nil
	}
	if _, err := s.PinGlobalTag(normalized); err != nil {
		return false, err
	}
	return true, nil
}

// IsGlobalTagPinned reports whether a normalized tag is globally pinned.
func (s *Settings) IsGlobalTagPinned(tag string) bool {
	normalized := models.NormalizeTag(tag)
	if normalized == "" {
		return false
	}
	return slices.Contains(models.DedupeTags(s.GlobalPinnedTags), normalized)
}

// GlobalPinnedTagList returns normalized, deduplicated global pinned tags in
// stored order.
func (s *Settings) GlobalPinnedTagList() []string {
	return models.DedupeTags(s.GlobalPinnedTags)
}

func (s *Settings) normalizeGlobalPinnedTags() {
	s.GlobalPinnedTags = models.DedupeTags(s.GlobalPinnedTags)
}
