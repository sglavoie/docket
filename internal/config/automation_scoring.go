package config

import (
	"fmt"

	"github.com/sglavoie/tt/internal/scoring"
)

// This file adds the scoring MUTATORS to the automation domain, mirroring the
// facade methods in scoring_profile.go so the scoring editor/profile-manager
// paths can mutate profiles and per-project assignments through the owning
// AutomationConfig domain instead of the config.Config facade. The reads
// (GetScoringProfile, GetProjectScoringProfile) live in automation_reads.go.

// SetProjectScoringProfile assigns a scoring profile to a project.
func (a *AutomationConfig) SetProjectScoringProfile(projectID, profileID string) {
	if a.ProjectScoringProfile == nil {
		a.ProjectScoringProfile = make(map[string]string)
	}
	a.ProjectScoringProfile[projectID] = profileID
}

// UnsetProjectScoringProfile removes the scoring profile assignment for a project.
func (a *AutomationConfig) UnsetProjectScoringProfile(projectID string) {
	if a.ProjectScoringProfile != nil {
		delete(a.ProjectScoringProfile, projectID)
	}
}

// AddScoringProfile adds a scoring profile to the automation domain. It returns
// an error if a profile with the same ID already exists.
func (a *AutomationConfig) AddScoringProfile(p scoring.ScoringProfile) error {
	if a.GetScoringProfile(p.ID) != nil {
		return fmt.Errorf("scoring profile %q already exists", p.ID)
	}
	a.ScoringProfiles = append(a.ScoringProfiles, p)
	return nil
}

// RemoveScoringProfile removes a scoring profile by ID and clears any project
// assignments that referenced it, so a re-added profile with the same ID never
// resurrects an orphaned assignment. It returns false if the profile was not
// found. Mirrors (*Config).RemoveScoringProfile.
func (a *AutomationConfig) RemoveScoringProfile(id string) bool {
	for i := range a.ScoringProfiles {
		if a.ScoringProfiles[i].ID == id {
			a.ScoringProfiles = append(a.ScoringProfiles[:i], a.ScoringProfiles[i+1:]...)
			if a.ProjectScoringProfile != nil {
				for projectID, profileID := range a.ProjectScoringProfile {
					if profileID == id {
						delete(a.ProjectScoringProfile, projectID)
					}
				}
			}
			return true
		}
	}
	return false
}
