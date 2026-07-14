package config

import (
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/template"
)

// GetEnabledTaskTemplates returns only the enabled task templates from the
// automation domain, mirroring Config.GetEnabledTaskTemplates so template
// consumers can read the templates through the owning domain instead of the
// config.Config facade.
func (a *AutomationConfig) GetEnabledTaskTemplates() []template.TaskTemplate {
	var enabled []template.TaskTemplate
	for _, t := range a.TaskTemplates {
		if t.Enabled {
			enabled = append(enabled, t)
		}
	}
	return enabled
}

// GetScoringProfile returns a scoring profile by ID, or nil if not found. It is
// the automation-domain equivalent of Config.GetScoringProfile.
func (a *AutomationConfig) GetScoringProfile(id string) *scoring.ScoringProfile {
	for i := range a.ScoringProfiles {
		if a.ScoringProfiles[i].ID == id {
			return &a.ScoringProfiles[i]
		}
	}
	return nil
}

// GetProjectScoringProfile returns the active scoring profile for a project, or
// nil. It mirrors Config.GetProjectScoringProfile but resolves the assignment
// from the owning automation domain rather than the config.Config facade.
func (a *AutomationConfig) GetProjectScoringProfile(projectID string) *scoring.ScoringProfile {
	if a.ProjectScoringProfile == nil {
		return nil
	}
	profileID, ok := a.ProjectScoringProfile[projectID]
	if !ok {
		return nil
	}
	return a.GetScoringProfile(profileID)
}
