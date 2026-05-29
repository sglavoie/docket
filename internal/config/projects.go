package config

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// GetProject returns a project by ID or slug.
// An empty idOrSlug never matches; nil project entries are skipped defensively
// so a malformed in-memory slice can't panic before Load() has scrubbed it.
func (c *Config) GetProject(idOrSlug string) *models.Project {
	if idOrSlug == "" {
		return nil
	}
	for _, p := range c.Projects {
		if p == nil {
			continue
		}
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
	if id == "" {
		return false
	}
	found := false
	var removedSlug string
	for i, p := range c.Projects {
		if p == nil {
			continue
		}
		if p.ID == id {
			removedSlug = p.Slug
			c.Projects = append(c.Projects[:i], c.Projects[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return false
	}

	if c.CurrentProject == id || (removedSlug != "" && c.CurrentProject == removedSlug) {
		c.CurrentProject = ""
	}
	if c.ActionInboxProjectID == id || (removedSlug != "" && c.ActionInboxProjectID == removedSlug) {
		c.ActionInboxProjectID = ""
	}

	c.removeProjectRefs(id, removedSlug)

	return true
}

func (c *Config) removeProjectRefs(id, slug string) {
	deleteProjectKey(c.ProjectScoringProfile, id, slug)
	deleteProjectKey(c.ProjectViewStates, id, slug)
	deleteProjectKey(c.WorkflowEvalStates, id, slug)

	c.FocusQueue, _ = filterProjectRefSlice(c.FocusQueue, func(item FocusQueueItem) string {
		return item.ProjectID
	}, func(pid string) bool {
		return pid != id && (slug == "" || pid != slug)
	})
	c.DailyPlan, _ = filterProjectRefSlice(c.DailyPlan, func(item DailyPlanItem) string {
		return item.ProjectID
	}, func(pid string) bool {
		return pid != id && (slug == "" || pid != slug)
	})
	c.RecentTaskVisits, _ = filterProjectRefSlice(c.RecentTaskVisits, func(visit RecentTaskVisit) string {
		return visit.ProjectID
	}, func(pid string) bool {
		return pid != id && (slug == "" || pid != slug)
	})

	c.CycleProjects, _ = filterProjectIDs(c.CycleProjects, func(pid string) bool {
		return pid != id && (slug == "" || pid != slug)
	})
	filterProjectIDMap(c.ContextCycleProjects, func(pid string) bool {
		return pid != id && (slug == "" || pid != slug)
	})
	filterProjectIDMap(c.CustomContexts, func(pid string) bool {
		return pid != id && (slug == "" || pid != slug)
	})
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
		if p == nil {
			continue
		}
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
		if p == nil {
			continue
		}
		if p.Sandbox {
			return false
		}
	}
	now := time.Now()
	// Append a short uuid suffix so back-to-back create/delete cycles within
	// the same wall-clock second don't collide on the timestamp-only ID.
	project := &models.Project{
		ID:            "sandbox-" + now.Format("20060102150405") + "-" + uuid.New().String()[:8],
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
			if p == nil {
				continue
			}
			if p.ID == c.ActionInboxProjectID {
				return p, false
			}
		}
	}

	for _, p := range c.Projects {
		if p == nil {
			continue
		}
		if p.Internal && p.Slug == "action-inbox" {
			c.ActionInboxProjectID = p.ID
			return p, false
		}
	}

	now := time.Now()
	// Append a short uuid suffix so back-to-back create/delete cycles within
	// the same wall-clock second don't collide on the timestamp-only ID.
	project := &models.Project{
		ID:            "action-inbox-" + now.Format("20060102150405") + "-" + uuid.New().String()[:8],
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
		if p == nil {
			continue
		}
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
		if p == nil {
			continue
		}
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
		if p == nil {
			continue
		}
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
		if p == nil {
			continue
		}
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

// PruneOrphanedProjectRefs drops persisted references to projects that no
// longer exist in c.Projects and normalizes legacy slug references to IDs.
// Without this pass, stale entries from hand-edited configs or out-of-band
// project removals quietly accumulate and keep round-tripping through every
// Save. Returns true if any reference was removed or normalized.
func (c *Config) PruneOrphanedProjectRefs() bool {
	knownRefs := make(map[string]string, len(c.Projects)*2)
	for _, p := range c.Projects {
		if p == nil || p.ID == "" {
			continue
		}
		knownRefs[p.ID] = p.ID
		if p.Slug != "" {
			knownRefs[p.Slug] = p.ID
		}
	}

	changed := false
	changed = normalizeProjectKeyedMap(c.ProjectScoringProfile, knownRefs) || changed
	changed = normalizeProjectKeyedMap(c.ProjectViewStates, knownRefs) || changed
	changed = normalizeProjectKeyedMap(c.WorkflowEvalStates, knownRefs) || changed

	var filtered bool
	c.FocusQueue, filtered = normalizeProjectRefSlice(c.FocusQueue, func(item FocusQueueItem) string {
		return item.ProjectID
	}, func(item FocusQueueItem, pid string) FocusQueueItem {
		item.ProjectID = pid
		return item
	}, knownRefs)
	changed = filtered || changed
	c.DailyPlan, filtered = normalizeProjectRefSlice(c.DailyPlan, func(item DailyPlanItem) string {
		return item.ProjectID
	}, func(item DailyPlanItem, pid string) DailyPlanItem {
		item.ProjectID = pid
		return item
	}, knownRefs)
	changed = filtered || changed
	c.RecentTaskVisits, filtered = normalizeProjectRefSlice(c.RecentTaskVisits, func(visit RecentTaskVisit) string {
		return visit.ProjectID
	}, func(visit RecentTaskVisit, pid string) RecentTaskVisit {
		visit.ProjectID = pid
		return visit
	}, knownRefs)
	changed = filtered || changed

	c.CycleProjects, filtered = normalizeProjectIDs(c.CycleProjects, knownRefs)
	changed = filtered || changed
	changed = normalizeProjectIDMap(c.ContextCycleProjects, knownRefs) || changed
	changed = normalizeProjectIDMap(c.CustomContexts, knownRefs) || changed

	if c.ActionInboxProjectID != "" {
		if canonical, ok := knownRefs[c.ActionInboxProjectID]; ok {
			if c.ActionInboxProjectID != canonical {
				c.ActionInboxProjectID = canonical
				changed = true
			}
		} else {
			c.ActionInboxProjectID = ""
			changed = true
		}
	}

	return changed
}

func deleteProjectKey[T any](values map[string]T, id, slug string) {
	delete(values, id)
	if slug != "" {
		delete(values, slug)
	}
}

func normalizeProjectKeyedMap[T any](values map[string]T, knownRefs map[string]string) bool {
	changed := false
	for pid, value := range values {
		canonical, ok := knownRefs[pid]
		if !ok {
			delete(values, pid)
			changed = true
			continue
		}
		if canonical != pid {
			if _, exists := values[canonical]; !exists {
				values[canonical] = value
			}
			delete(values, pid)
			changed = true
		}
	}
	return changed
}

func normalizeProjectRefSlice[T any](values []T, projectID func(T) string, setProjectID func(T, string) T, knownRefs map[string]string) ([]T, bool) {
	if len(values) == 0 {
		return values, false
	}
	filtered := values[:0]
	changed := false
	for _, value := range values {
		pid := projectID(value)
		canonical, ok := knownRefs[pid]
		if !ok {
			changed = true
			continue
		}
		if canonical != pid {
			value = setProjectID(value, canonical)
			changed = true
		}
		filtered = append(filtered, value)
	}
	return filtered, changed
}

func normalizeProjectIDs(ids []string, knownRefs map[string]string) ([]string, bool) {
	return normalizeProjectRefSlice(ids, func(id string) string {
		return id
	}, func(_ string, id string) string {
		return id
	}, knownRefs)
}

func normalizeProjectIDMap(projectIDs map[string][]string, knownRefs map[string]string) bool {
	changed := false
	for key, ids := range projectIDs {
		var filtered bool
		projectIDs[key], filtered = normalizeProjectIDs(ids, knownRefs)
		changed = filtered || changed
	}
	return changed
}

func filterProjectRefSlice[T any](values []T, projectID func(T) string, keep func(string) bool) ([]T, bool) {
	if len(values) == 0 {
		return values, false
	}
	filtered := values[:0]
	changed := false
	for _, value := range values {
		if keep(projectID(value)) {
			filtered = append(filtered, value)
		} else {
			changed = true
		}
	}
	return filtered, changed
}

func filterProjectIDs(ids []string, keep func(string) bool) ([]string, bool) {
	return filterProjectRefSlice(ids, func(id string) string {
		return id
	}, keep)
}

func filterProjectIDMap(projectIDs map[string][]string, keep func(string) bool) bool {
	changed := false
	for key, ids := range projectIDs {
		var filtered bool
		projectIDs[key], filtered = filterProjectIDs(ids, keep)
		changed = filtered || changed
	}
	return changed
}

// GetAllContexts returns a sorted list of unique context values from all projects.
func (c *Config) GetAllContexts() []string {
	contextSet := make(map[string]bool)
	for _, p := range c.Projects {
		if p == nil {
			continue
		}
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
