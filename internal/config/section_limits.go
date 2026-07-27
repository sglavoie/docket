package config

import (
	"encoding/json"

	"github.com/sglavoie/tt/internal/models"
)

const (
	DefaultStateGroupLimitToday      = 0
	DefaultStateGroupLimitInProgress = 5
	DefaultStateGroupLimitInbox      = 5
	DefaultStateGroupLimitNext       = 5
	DefaultStateGroupLimitBacklog    = 5
	DefaultStateGroupLimitReview     = 5
	DefaultStateGroupLimitWaiting    = 3
	DefaultStateGroupLimitPaused     = 5
	DefaultStateGroupLimitSomeday    = 10
)

func stateGroupLimitValue(limit int) int {
	if limit < 0 {
		return 0
	}
	return limit
}

func defaultStateGroupLimit(group models.WorkflowStateGroup) int {
	switch group {
	case models.StateGroupToday:
		return DefaultStateGroupLimitToday
	case models.StateGroupInProgress:
		return DefaultStateGroupLimitInProgress
	case models.StateGroupInbox:
		return DefaultStateGroupLimitInbox
	case models.StateGroupNext:
		return DefaultStateGroupLimitNext
	case models.StateGroupBacklog:
		return DefaultStateGroupLimitBacklog
	case models.StateGroupReview:
		return DefaultStateGroupLimitReview
	case models.StateGroupWaiting:
		return DefaultStateGroupLimitWaiting
	case models.StateGroupPaused:
		return DefaultStateGroupLimitPaused
	case models.StateGroupSomeday:
		return DefaultStateGroupLimitSomeday
	default:
		return 0
	}
}

// GetStateGroupLimit returns the canonical WIP limit for group. Zero disables
// the warning. Negative persisted values are clamped to disabled for
// compatibility with the former section-limit behavior.
func (s *Settings) GetStateGroupLimit(group models.WorkflowStateGroup) int {
	if s != nil && s.StateGroupLimits != nil {
		if limit, ok := s.StateGroupLimits[group]; ok {
			return stateGroupLimitValue(limit)
		}
	}
	return defaultStateGroupLimit(group)
}

// SetStateGroupLimit stores a canonical group limit. Invalid groups are
// ignored; negative values are normalized to disabled.
func (s *Settings) SetStateGroupLimit(group models.WorkflowStateGroup, limit int) {
	if s == nil || !group.IsValid() {
		return
	}
	if s.StateGroupLimits == nil {
		s.StateGroupLimits = make(map[models.WorkflowStateGroup]int)
	}
	s.StateGroupLimits[group] = stateGroupLimitValue(limit)
}

// decodeLegacySectionLimits migrates retired section_limit_* JSON values into
// the canonical map. Explicit canonical keys always win when both generations
// are present.
func (s *Settings) decodeLegacySectionLimits(data []byte) error {
	s.legacySectionLimitsDecoded = false
	type legacySectionLimits struct {
		Active  *int `json:"section_limit_active"`
		Next    *int `json:"section_limit_next"`
		Inbox   *int `json:"section_limit_inbox"`
		Standby *int `json:"section_limit_standby"`
		Waiting *int `json:"section_limit_waiting"`
		Someday *int `json:"section_limit_someday"`
		Review  *int `json:"section_limit_review"`
	}
	var legacy legacySectionLimits
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}

	legacyKeys := []string{
		"section_limit_active",
		"section_limit_next",
		"section_limit_inbox",
		"section_limit_standby",
		"section_limit_waiting",
		"section_limit_someday",
		"section_limit_review",
	}
	for _, key := range legacyKeys {
		if jsonObjectHasKey(data, key) {
			s.legacySectionLimitsDecoded = true
			break
		}
	}

	setIfAbsent := func(group models.WorkflowStateGroup, value *int) {
		if value == nil {
			return
		}
		if s.StateGroupLimits == nil {
			s.StateGroupLimits = make(map[models.WorkflowStateGroup]int)
		}
		if _, exists := s.StateGroupLimits[group]; !exists {
			s.StateGroupLimits[group] = stateGroupLimitValue(*value)
		}
	}
	setIfAbsent(models.StateGroupBacklog, legacy.Active)
	setIfAbsent(models.StateGroupInProgress, legacy.Active)
	setIfAbsent(models.StateGroupNext, legacy.Next)
	setIfAbsent(models.StateGroupInbox, legacy.Inbox)
	setIfAbsent(models.StateGroupPaused, legacy.Standby)
	setIfAbsent(models.StateGroupWaiting, legacy.Waiting)
	setIfAbsent(models.StateGroupSomeday, legacy.Someday)
	setIfAbsent(models.StateGroupReview, legacy.Review)
	return nil
}
