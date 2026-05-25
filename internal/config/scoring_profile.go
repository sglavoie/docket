package config

import (
	"fmt"

	"github.com/sglavoie/tt/internal/scoring"
)

// GetScoringProfile returns a scoring profile by ID, or nil if not found.
func (c *Config) GetScoringProfile(id string) *scoring.ScoringProfile {
	for i := range c.ScoringProfiles {
		if c.ScoringProfiles[i].ID == id {
			return &c.ScoringProfiles[i]
		}
	}
	return nil
}

// GetProjectScoringProfile returns the active scoring profile for a project, or nil.
func (c *Config) GetProjectScoringProfile(projectID string) *scoring.ScoringProfile {
	if c.ProjectScoringProfile == nil {
		return nil
	}
	profileID, ok := c.ProjectScoringProfile[projectID]
	if !ok {
		return nil
	}
	return c.GetScoringProfile(profileID)
}

// SetProjectScoringProfile assigns a scoring profile to a project.
func (c *Config) SetProjectScoringProfile(projectID, profileID string) {
	if c.ProjectScoringProfile == nil {
		c.ProjectScoringProfile = make(map[string]string)
	}
	c.ProjectScoringProfile[projectID] = profileID
}

// UnsetProjectScoringProfile removes the scoring profile assignment for a project.
func (c *Config) UnsetProjectScoringProfile(projectID string) {
	if c.ProjectScoringProfile != nil {
		delete(c.ProjectScoringProfile, projectID)
	}
}

// AddScoringProfile adds a scoring profile to the config.
// Returns an error if a profile with the same ID already exists.
func (c *Config) AddScoringProfile(p scoring.ScoringProfile) error {
	if c.GetScoringProfile(p.ID) != nil {
		return fmt.Errorf("scoring profile %q already exists", p.ID)
	}
	c.ScoringProfiles = append(c.ScoringProfiles, p)
	return nil
}

// RemoveScoringProfile removes a scoring profile by ID and clears any
// project assignments that referenced it. Without this cleanup, projects
// would silently keep a dangling ProjectScoringProfile entry: GetProjectScoringProfile
// would return nil while the orphaned reference resurfaces if a profile with
// the same ID is later re-added.
// Returns false if the profile was not found.
func (c *Config) RemoveScoringProfile(id string) bool {
	for i := range c.ScoringProfiles {
		if c.ScoringProfiles[i].ID == id {
			c.ScoringProfiles = append(c.ScoringProfiles[:i], c.ScoringProfiles[i+1:]...)
			if c.ProjectScoringProfile != nil {
				for projectID, profileID := range c.ProjectScoringProfile {
					if profileID == id {
						delete(c.ProjectScoringProfile, projectID)
					}
				}
			}
			return true
		}
	}
	return false
}
