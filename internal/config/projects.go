package config

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// GetProject returns a project by ID or slug.
func (c *Config) GetProject(idOrSlug string) *models.Project {
	for _, p := range c.Projects {
		if p.ID == idOrSlug || p.Slug == idOrSlug {
			return p
		}
	}
	return nil
}

// GetCurrentProject returns the currently selected project.
func (c *Config) GetCurrentProject() *models.Project {
	if c.CurrentProject == "" {
		return nil
	}
	return c.GetProject(c.CurrentProject)
}

// AddProject adds a project to the config.
func (c *Config) AddProject(project *models.Project) {
	c.Projects = append(c.Projects, project)
}

// RemoveProject removes a project by ID and cleans up all config references
// (scoring profiles, view states, focus queue, recent visits, cycle lists, etc.).
func (c *Config) RemoveProject(id string) bool {
	found := false
	for i, p := range c.Projects {
		if p.ID == id {
			c.Projects = append(c.Projects[:i], c.Projects[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return false
	}

	if c.CurrentProject == id {
		c.CurrentProject = ""
	}

	// Clean up scoring profile assignment
	if c.ProjectScoringProfile != nil {
		delete(c.ProjectScoringProfile, id)
	}

	// Clean up per-project view state
	if c.ProjectViewStates != nil {
		delete(c.ProjectViewStates, id)
	}

	// Clean up workflow evaluation state
	if c.WorkflowEvalStates != nil {
		delete(c.WorkflowEvalStates, id)
	}

	// Clean up focus queue entries referencing this project
	if len(c.FocusQueue) > 0 {
		filtered := c.FocusQueue[:0]
		for _, item := range c.FocusQueue {
			if item.ProjectID != id {
				filtered = append(filtered, item)
			}
		}
		c.FocusQueue = filtered
	}

	// Clean up daily plan entries referencing this project
	if len(c.DailyPlan) > 0 {
		filtered := c.DailyPlan[:0]
		for _, item := range c.DailyPlan {
			if item.ProjectID != id {
				filtered = append(filtered, item)
			}
		}
		c.DailyPlan = filtered
	}

	// Clean up recent task visits referencing this project
	if len(c.RecentTaskVisits) > 0 {
		filtered := c.RecentTaskVisits[:0]
		for _, visit := range c.RecentTaskVisits {
			if visit.ProjectID != id {
				filtered = append(filtered, visit)
			}
		}
		c.RecentTaskVisits = filtered
	}

	// Clean up cycle projects (global)
	if len(c.CycleProjects) > 0 {
		filtered := c.CycleProjects[:0]
		for _, pid := range c.CycleProjects {
			if pid != id {
				filtered = append(filtered, pid)
			}
		}
		c.CycleProjects = filtered
	}

	// Clean up per-context cycle projects
	for ctx, pids := range c.ContextCycleProjects {
		filtered := pids[:0]
		for _, pid := range pids {
			if pid != id {
				filtered = append(filtered, pid)
			}
		}
		c.ContextCycleProjects[ctx] = filtered
	}

	// Clean up custom contexts referencing this project
	for ctx, pids := range c.CustomContexts {
		filtered := pids[:0]
		for _, pid := range pids {
			if pid != id {
				filtered = append(filtered, pid)
			}
		}
		c.CustomContexts[ctx] = filtered
	}

	// Clean up action inbox reference if it points to this project
	if c.ActionInboxProjectID == id {
		c.ActionInboxProjectID = ""
	}

	return true
}

// sortProjectsByName sorts projects alphabetically by name (case-insensitive).
func sortProjectsByName(projects []*models.Project) {
	sort.Slice(projects, func(i, j int) bool {
		return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
	})
}

// sortProjectsWithPinnedFirst sorts projects with pinned ones first, then non-pinned.
// Each group is sorted alphabetically by name (case-insensitive).
func sortProjectsWithPinnedFirst(projects []*models.Project) {
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].Pinned != projects[j].Pinned {
			return projects[i].Pinned
		}
		return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
	})
}

// GetSandboxProject returns the sandbox project, or nil if none exists.
func (c *Config) GetSandboxProject() *models.Project {
	for _, p := range c.Projects {
		if p.Sandbox {
			return p
		}
	}
	return nil
}

// EnsureSandboxProject checks if a sandbox project exists and creates one if not.
// Returns true if a new sandbox project was created.
func (c *Config) EnsureSandboxProject() bool {
	for _, p := range c.Projects {
		if p.Sandbox {
			return false
		}
	}
	now := time.Now()
	project := &models.Project{
		ID:            "sandbox-" + now.Format("20060102150405"),
		Slug:          "_sandbox",
		Name:          "Sandbox",
		Context:       "personal",
		Path:          filepath.Join(c.GetInternalDir(), "sandbox"),
		CreatedAt:     now,
		UpdatedAt:     now,
		Sandbox:       true,
		Tasks:         []*models.Task{},
		NextShortID:   1,
		Sections:      []string{models.SectionNext},
		SectionColors: make(map[string]string),
	}
	c.Projects = append(c.Projects, project)
	return true
}

// EnsureActionInboxProject checks if the action inbox project exists and creates one if not.
// Returns the project and true if a new project was created.
func (c *Config) EnsureActionInboxProject() (*models.Project, bool) {
	if c.ActionInboxProjectID != "" {
		for _, p := range c.Projects {
			if p.ID == c.ActionInboxProjectID {
				return p, false
			}
		}
	}

	for _, p := range c.Projects {
		if p.Internal && p.Slug == "action-inbox" {
			c.ActionInboxProjectID = p.ID
			return p, false
		}
	}

	now := time.Now()
	project := &models.Project{
		ID:            "action-inbox-" + now.Format("20060102150405"),
		Slug:          "action-inbox",
		Name:          "Action Inbox",
		Context:       "personal",
		Path:          filepath.Join(c.GetInternalDir(), "action-inbox"),
		CreatedAt:     now,
		UpdatedAt:     now,
		Internal:      true,
		Tasks:         []*models.Task{},
		NextShortID:   1,
		Sections:      []string{models.SectionNext},
		SectionColors: make(map[string]string),
	}
	c.Projects = append(c.Projects, project)
	c.ActionInboxProjectID = project.ID
	return project, true
}

// GetActionInboxProject returns the action inbox project, or nil if none exists.
func (c *Config) GetActionInboxProject() *models.Project {
	if c.ActionInboxProjectID == "" {
		return nil
	}
	for _, p := range c.Projects {
		if p.ID == c.ActionInboxProjectID {
			return p
		}
	}
	return nil
}

// GetActiveProjects returns non-archived, non-sandbox, non-internal projects sorted with pinned first, then alphabetically.
func (c *Config) GetActiveProjects() []*models.Project {
	var active []*models.Project
	for _, p := range c.Projects {
		if !p.Archived && !p.Sandbox && !p.Internal {
			active = append(active, p)
		}
	}
	sortProjectsWithPinnedFirst(active)
	return active
}

// GetAllLoadableProjects returns all non-archived projects including internal ones (for focus board data loading).
func (c *Config) GetAllLoadableProjects() []*models.Project {
	var projects []*models.Project
	for _, p := range c.Projects {
		if !p.Archived {
			projects = append(projects, p)
		}
	}
	sortProjectsWithPinnedFirst(projects)
	return projects
}

// GetArchivedProjects returns archived projects sorted alphabetically.
func (c *Config) GetArchivedProjects() []*models.Project {
	var archived []*models.Project
	for _, p := range c.Projects {
		if p.Archived && !p.Sandbox {
			archived = append(archived, p)
		}
	}
	sortProjectsByName(archived)
	return archived
}

// SetCurrentProject sets the current project by ID.
func (c *Config) SetCurrentProject(id string) bool {
	project := c.GetProject(id)
	if project == nil {
		return false
	}
	c.CurrentProject = project.ID
	return true
}

// GetAllContexts returns a sorted list of unique context values from all projects.
func (c *Config) GetAllContexts() []string {
	contextSet := make(map[string]bool)
	for _, p := range c.Projects {
		if p.Context != "" {
			contextSet[p.Context] = true
		}
	}
	contexts := make([]string, 0, len(contextSet))
	for ctx := range contextSet {
		contexts = append(contexts, ctx)
	}
	sort.Strings(contexts)
	return contexts
}
