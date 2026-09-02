package config

import (
	"fmt"
	"strings"
)

// Validate verifies the complete three-domain candidate as one bundle.
func (domains V5DomainCandidates) Validate() error {
	if err := domains.Catalog.Validate(); err != nil {
		return err
	}
	if err := domains.Planning.Validate(domains.Catalog); err != nil {
		return err
	}
	if err := domains.Machine.Validate(domains.Catalog); err != nil {
		return err
	}
	return nil
}

func (catalog V5WorkspaceCatalogCandidate) Validate() error {
	if catalog.SchemaVersion != V5ConfigSchemaVersion {
		return fmt.Errorf("v5 catalog schema is %d, want %d", catalog.SchemaVersion, V5ConfigSchemaVersion)
	}
	if err := validateWorkspaceID(catalog.WorkspaceID); err != nil {
		return fmt.Errorf("v5 catalog: %w", err)
	}
	ids := make(map[string]bool, len(catalog.Projects))
	slugs := make(map[string]bool, len(catalog.Projects))
	for index, project := range catalog.Projects {
		if err := validateV5ProjectCandidate(project, false); err != nil {
			return fmt.Errorf("v5 catalog project %d: %w", index, err)
		}
		if ids[project.ID] {
			return fmt.Errorf("v5 catalog has duplicate project ID %q", project.ID)
		}
		if slugs[project.Slug] {
			return fmt.Errorf("v5 catalog has duplicate project slug %q", project.Slug)
		}
		ids[project.ID] = true
		slugs[project.Slug] = true
	}
	if catalog.SystemInboxProjectID == "" || !ids[catalog.SystemInboxProjectID] {
		return fmt.Errorf("v5 catalog protected system Inbox is missing or unknown")
	}
	for _, project := range catalog.Projects {
		if project.ID == catalog.SystemInboxProjectID && project.Archived {
			return fmt.Errorf(
				"%w: project %q cannot be archived",
				ErrProtectedSystemUnfiled,
				project.ID,
			)
		}
	}
	return nil
}

func validateV5ProjectCandidate(project V5ProjectCandidate, allowColorRepair bool) error {
	if strings.TrimSpace(project.ID) == "" {
		return fmt.Errorf("project has an empty ID")
	}
	if err := validateLegacyV4ProjectSlug(project.Slug, false); err != nil {
		return fmt.Errorf("project %q: %w", project.ID, err)
	}
	if strings.TrimSpace(project.Name) == "" || strings.TrimSpace(project.Context) == "" {
		return fmt.Errorf("project %q requires a name and regular context", project.ID)
	}
	if project.CreatedAt.IsZero() || project.UpdatedAt.IsZero() {
		return fmt.Errorf("project %q requires created and updated timestamps", project.ID)
	}
	if !allowColorRepair && !validProjectColorSlot(project.ColorSlot) {
		return fmt.Errorf("project %q has invalid color slot %d", project.ID, project.ColorSlot)
	}
	if project.Archived != (project.ArchivedAt != nil) {
		return fmt.Errorf("project %q has inconsistent archive state", project.ID)
	}
	if project.Pinned != (project.PinnedAt != nil) {
		return fmt.Errorf("project %q has inconsistent pin state", project.ID)
	}
	return nil
}

func (planning V5PlanningCandidate) Validate(catalog V5WorkspaceCatalogCandidate) error {
	projectIDs := make(map[string]bool, len(catalog.Projects))
	for _, project := range catalog.Projects {
		projectIDs[project.ID] = true
	}
	collections := []struct {
		label string
		refs  []V5PlanningTaskRef
	}{
		{label: "Today", refs: planning.Today},
		{label: "Next order", refs: planning.NextOrder},
	}
	for _, collection := range collections {
		label, refs := collection.label, collection.refs
		seen := make(map[string]bool, len(refs))
		for index, ref := range refs {
			if strings.TrimSpace(ref.TaskID) == "" || !projectIDs[ref.ProjectID] {
				return fmt.Errorf("v5 planning %s item %d has invalid identity", label, index)
			}
			key := ref.ProjectID + "\x00" + ref.TaskID
			if seen[key] {
				return fmt.Errorf("v5 planning %s duplicates task %q in project %q", label, ref.TaskID, ref.ProjectID)
			}
			seen[key] = true
		}
	}
	return nil
}

func (machine V5MachineCandidate) Validate(catalog V5WorkspaceCatalogCandidate) error {
	if machine.Theme != "dark" && machine.Theme != "light" {
		return fmt.Errorf("v5 machine theme %q is invalid", machine.Theme)
	}
	projectIDs := make(map[string]bool, len(catalog.Projects))
	for _, project := range catalog.Projects {
		projectIDs[project.ID] = true
	}
	names := make(map[string]string, len(machine.CustomContexts))
	for name, members := range machine.CustomContexts {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" || trimmed != name {
			return fmt.Errorf("v5 custom context name %q is invalid", name)
		}
		key := strings.ToLower(name)
		if prior := names[key]; prior != "" {
			return fmt.Errorf("v5 custom contexts %q and %q collide case-insensitively", prior, name)
		}
		names[key] = name
		if len(members) == 0 {
			return fmt.Errorf("v5 custom context %q has no members", name)
		}
		seen := make(map[string]bool, len(members))
		for _, projectID := range members {
			if !projectIDs[projectID] {
				return fmt.Errorf("v5 custom context %q references unknown project %q", name, projectID)
			}
			if seen[projectID] {
				return fmt.Errorf("v5 custom context %q repeats project %q", name, projectID)
			}
			seen[projectID] = true
		}
	}
	return nil
}
