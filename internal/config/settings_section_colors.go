package config

import (
	"encoding/json"
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// legacySectionColorGroups is confined to settings decoding. It maps the
// pre-v4 presentation buckets onto canonical workflow-state groups.
func legacySectionColorGroups(value string) ([]models.WorkflowStateGroup, bool) {
	switch value {
	case "", "Active":
		return []models.WorkflowStateGroup{models.StateGroupBacklog, models.StateGroupInProgress}, true
	case "Inbox":
		return []models.WorkflowStateGroup{models.StateGroupInbox}, true
	case "Next":
		return []models.WorkflowStateGroup{models.StateGroupNext}, true
	case "Waiting":
		return []models.WorkflowStateGroup{models.StateGroupWaiting}, true
	case "Standby":
		return []models.WorkflowStateGroup{models.StateGroupPaused}, true
	case "Review":
		return []models.WorkflowStateGroup{models.StateGroupReview}, true
	case "Someday":
		return []models.WorkflowStateGroup{models.StateGroupSomeday}, true
	case "Completed", "Done":
		return []models.WorkflowStateGroup{models.StateGroupDone, models.StateGroupCanceled}, true
	default:
		return nil, false
	}
}

// decodeLegacySectionColors accepts the v3 registry and seeds canonical
// state_group_colors. Explicit canonical entries win mixed payloads.
func (s *Settings) decodeLegacySectionColors(data []byte) error {
	var wire struct {
		Colors map[string]string `json:"section_colors"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	s.legacySectionColorsDecoded = jsonObjectHasKey(data, "section_colors")
	for value, color := range wire.Colors {
		groups, ok := legacySectionColorGroups(value)
		if !ok {
			return fmt.Errorf("settings: cannot migrate unknown legacy color key %q", value)
		}
		if color == "" {
			color = "gray"
		}
		if !models.IsValidThemeColor(color) {
			return fmt.Errorf("settings: invalid legacy color %q for %q", color, value)
		}
		if s.StateGroupColors == nil {
			s.StateGroupColors = make(map[models.WorkflowStateGroup]string)
		}
		for _, group := range groups {
			if _, exists := s.StateGroupColors[group]; !exists {
				s.StateGroupColors[group] = color
			}
		}
	}
	return nil
}

// MigrateLegacySectionColors reports and clears the one-time decode marker.
// The decode step already populated the canonical registry.
func (s *Settings) MigrateLegacySectionColors() (bool, error) {
	if s == nil {
		return false, nil
	}
	changed := s.legacySectionColorsDecoded
	s.legacySectionColorsDecoded = false
	return changed, nil
}
