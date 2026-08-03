package config

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const V5ConfigSchemaVersion = 5

// V5WorkspaceCatalogCandidate is the lean catalog shape produced before the
// live runtime catalog cuts over to schema v5.
type V5WorkspaceCatalogCandidate struct {
	SchemaVersion        int                  `json:"schema_version"`
	WorkspaceID          string               `json:"workspace_id"`
	Projects             []V5ProjectCandidate `json:"projects"`
	SystemInboxProjectID string               `json:"system_inbox_project_id"`
}

// V5ProjectCandidate contains only the durable project fields retained by the
// simplified product.
type V5ProjectCandidate struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Context     string     `json:"context"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Archived    bool       `json:"archived"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
	Pinned      bool       `json:"pinned"`
	PinnedAt    *time.Time `json:"pinned_at,omitempty"`
	ColorSlot   int        `json:"color_slot"`
}

// IsSystemInboxProject reports whether id is the catalog-owned Inbox. The
// durable ID is the sole protection boundary; matching a display name or slug
// never grants or bypasses protection.
func (catalog V5WorkspaceCatalogCandidate) IsSystemInboxProject(id string) bool {
	return id != "" && catalog.SystemInboxProjectID == id
}

// EnsureProjectColorSlots repairs missing or invalid v5 color slots
// deterministically without recoloring valid assignments. All v5 projects are
// user-visible and therefore eligible for one automatic durable slot.
func (catalog *V5WorkspaceCatalogCandidate) EnsureProjectColorSlots() bool {
	if catalog == nil {
		return false
	}

	var usage [ProjectColorSlotCount + 1]int
	pending := make([]int, 0)
	for index := range catalog.Projects {
		slot := catalog.Projects[index].ColorSlot
		if validProjectColorSlot(slot) {
			usage[slot]++
		} else {
			pending = append(pending, index)
		}
	}
	sort.SliceStable(pending, func(i, j int) bool {
		left, right := catalog.Projects[pending[i]], catalog.Projects[pending[j]]
		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.Before(right.CreatedAt)
		}
		return left.ID < right.ID
	})
	for _, index := range pending {
		slot := leastUsedProjectColorSlot(usage)
		catalog.Projects[index].ColorSlot = slot
		usage[slot]++
	}
	return len(pending) > 0
}

// UpsertProject applies one create/edit/pin/archive/restore snapshot to the lean
// v5 catalog. Existing projects retain their durable color slot; new and
// restored projects receive an automatic balanced slot. A system Inbox update
// may change ordinary metadata or pin state, but cannot rename or archive it.
// Rejected mutations leave the candidate unchanged.
func (catalog *V5WorkspaceCatalogCandidate) UpsertProject(project V5ProjectCandidate) error {
	if catalog == nil {
		return fmt.Errorf("v5 catalog is required")
	}
	if catalog.IsSystemInboxProject(project.ID) && (project.Archived || project.ArchivedAt != nil) {
		return fmt.Errorf("%w: project %q cannot be archived", ErrProtectedSystemUnfiled, project.ID)
	}
	if err := validateV5ProjectCandidate(project, true); err != nil {
		return err
	}

	next := *catalog
	next.Projects = append([]V5ProjectCandidate(nil), catalog.Projects...)
	next.EnsureProjectColorSlots()

	existingIndex := -1
	for index, existing := range next.Projects {
		if existing.ID == project.ID {
			existingIndex = index
			continue
		}
		if existing.Slug == project.Slug {
			return fmt.Errorf(
				"v5 catalog project slug %q is already owned by project %q",
				project.Slug,
				existing.ID,
			)
		}
	}
	if next.IsSystemInboxProject(project.ID) {
		if existingIndex >= 0 {
			existing := next.Projects[existingIndex]
			if existing.Name != project.Name || existing.Slug != project.Slug {
				return fmt.Errorf("%w: project %q cannot be renamed", ErrProtectedSystemUnfiled, project.ID)
			}
		}
	}

	if existingIndex >= 0 {
		project.ColorSlot = next.Projects[existingIndex].ColorSlot
		next.Projects[existingIndex] = project
	} else {
		var usage [ProjectColorSlotCount + 1]int
		for _, existing := range next.Projects {
			if validProjectColorSlot(existing.ColorSlot) {
				usage[existing.ColorSlot]++
			}
		}
		if !validProjectColorSlot(project.ColorSlot) ||
			(usage[project.ColorSlot] > 0 && !allProjectColorSlotsUsed(usage)) {
			project.ColorSlot = leastUsedProjectColorSlot(usage)
		}
		next.Projects = append(next.Projects, project)
	}

	*catalog = next
	return nil
}

// RemoveProject removes one lean v5 project by ID. The protected system Inbox
// can never be removed, and a rejection leaves the candidate unchanged.
func (catalog *V5WorkspaceCatalogCandidate) RemoveProject(projectID string) error {
	if catalog == nil {
		return fmt.Errorf("v5 catalog is required")
	}
	if catalog.IsSystemInboxProject(projectID) {
		return fmt.Errorf("%w: project %q cannot be removed", ErrProtectedSystemUnfiled, projectID)
	}
	kept := make([]V5ProjectCandidate, 0, len(catalog.Projects))
	for _, project := range catalog.Projects {
		if project.ID != projectID {
			kept = append(kept, project)
		}
	}
	catalog.Projects = kept
	return nil
}

// V5PlanningCandidate persists Today membership and Next ordering only.
type V5PlanningCandidate struct {
	Today     []V5PlanningTaskRef `json:"today,omitempty"`
	NextOrder []V5PlanningTaskRef `json:"next_order,omitempty"`
}

type V5PlanningTaskRef = ProjectTaskLocator

// V5MachineCandidate retains only machine-local custom contexts and the
// selected built-in theme.
type V5MachineCandidate struct {
	CustomContexts map[string][]string `json:"custom_contexts,omitempty"`
	Theme          string              `json:"theme"`
}

type V5DomainCandidates struct {
	Catalog  V5WorkspaceCatalogCandidate
	Planning V5PlanningCandidate
	Machine  V5MachineCandidate
}

// V5DomainMigrationAuditEntry records every configuration value that is
// transformed, repaired, or intentionally left in the immutable snapshot.
type V5DomainMigrationAuditEntry struct {
	Domain   string `json:"domain"`
	EntityID string `json:"entity_id,omitempty"`
	Action   string `json:"action"`
	Field    string `json:"field"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Reason   string `json:"reason"`
}

// V5DomainConversionResult is a pure, validated three-domain candidate.
type V5DomainConversionResult struct {
	Domains             V5DomainCandidates
	RemovedProjectIDs   []string
	RemovedProjectSlugs []string
	Audit               []V5DomainMigrationAuditEntry
}

// ConvertLegacyV4Domains converts the five decoded v4 domains into the three
// v5 candidates. It does not mutate input slices or maps.
func ConvertLegacyV4Domains(legacy LegacyV4Domains) (V5DomainConversionResult, error) {
	if err := validateLegacyV4Domains(legacy); err != nil {
		return V5DomainConversionResult{}, err
	}
	if legacy.WorkspaceID == "" {
		return V5DomainConversionResult{}, fmt.Errorf("legacy v4 domain bundle has no workspace ID")
	}

	result := V5DomainConversionResult{
		Domains: V5DomainCandidates{
			Catalog: V5WorkspaceCatalogCandidate{
				SchemaVersion:        V5ConfigSchemaVersion,
				WorkspaceID:          legacy.WorkspaceID,
				SystemInboxProjectID: legacy.Catalog.SystemInboxProjectID,
			},
			Machine: V5MachineCandidate{
				CustomContexts: make(map[string][]string),
				Theme:          mapLegacyV4Theme(legacy.Settings.ActiveTheme),
			},
		},
	}
	audit := func(domain, entityID, action, field, from, to, reason string) {
		result.Audit = append(result.Audit, V5DomainMigrationAuditEntry{
			Domain: domain, EntityID: entityID, Action: action, Field: field,
			From: from, To: to, Reason: reason,
		})
	}

	retainedIDs := make(map[string]bool, len(legacy.Catalog.Projects))
	for _, project := range legacy.Catalog.Projects {
		if project.Sandbox || project.Internal {
			result.RemovedProjectIDs = append(result.RemovedProjectIDs, project.ID)
			result.RemovedProjectSlugs = append(result.RemovedProjectSlugs, project.Slug)
			kind := "Sandbox"
			if project.Internal {
				kind = "Internal"
			}
			audit(
				"workspace", project.ID, "snapshot-only", "project",
				kind, "", kind+" projects are not part of the v5 catalog",
			)
			continue
		}
		retainedIDs[project.ID] = true
		result.Domains.Catalog.Projects = append(result.Domains.Catalog.Projects, V5ProjectCandidate{
			ID:          project.ID,
			Slug:        project.Slug,
			Name:        project.Name,
			Description: project.Description,
			Context:     project.Context,
			CreatedAt:   project.CreatedAt,
			UpdatedAt:   project.UpdatedAt,
			Archived:    project.Archived,
			ArchivedAt:  cloneTimePointer(project.ArchivedAt),
			Pinned:      project.Pinned,
			PinnedAt:    cloneTimePointer(project.PinnedAt),
			ColorSlot:   project.ColorSlot,
		})
		if project.StatusIcon != "" || project.StatusIconSource != "" {
			audit(
				"workspace", project.ID, "snapshot-only", "status_icon",
				project.StatusIcon, "", "v5 projects have no status icon metadata",
			)
		}
	}
	if !retainedIDs[result.Domains.Catalog.SystemInboxProjectID] {
		return V5DomainConversionResult{}, fmt.Errorf(
			"protected system Inbox %q would be removed from the v5 catalog",
			result.Domains.Catalog.SystemInboxProjectID,
		)
	}
	assignV5ProjectColorSlots(result.Domains.Catalog.Projects)

	for _, item := range legacy.Planning.FocusQueue {
		if !retainedIDs[item.ProjectID] {
			audit(
				"planning", item.TaskID, "snapshot-only", "next_order",
				item.ProjectID, "", "planning reference belongs to a project omitted from v5",
			)
		}
	}
	result.Domains.Planning.NextOrder = convertV5PlanningRefs(
		legacy.Planning.FocusQueue,
		retainedIDs,
		func(item LegacyV4FocusQueueItem) V5PlanningTaskRef {
			return V5PlanningTaskRef(item)
		},
		func(item LegacyV4FocusQueueItem) string { return item.ProjectID },
		func(ref V5PlanningTaskRef) {
			audit(
				"planning", ref.TaskID, "repaired", "next_order",
				ref.ProjectID, "", "duplicate planning reference was removed",
			)
		},
	)
	todayInputs := make([]LegacyV4FocusQueueItem, len(legacy.Planning.DailyPlan))
	for index, item := range legacy.Planning.DailyPlan {
		todayInputs[index] = LegacyV4FocusQueueItem{TaskID: item.TaskID, ProjectID: item.ProjectID}
		if !retainedIDs[item.ProjectID] {
			audit(
				"planning", item.TaskID, "snapshot-only", "today",
				item.ProjectID, "", "planning reference belongs to a project omitted from v5",
			)
		}
	}
	result.Domains.Planning.Today = convertV5PlanningRefs(
		todayInputs,
		retainedIDs,
		func(item LegacyV4FocusQueueItem) V5PlanningTaskRef {
			return V5PlanningTaskRef(item)
		},
		func(item LegacyV4FocusQueueItem) string { return item.ProjectID },
		func(ref V5PlanningTaskRef) {
			audit(
				"planning", ref.TaskID, "repaired", "today",
				ref.ProjectID, "", "duplicate planning reference was removed",
			)
		},
	)
	audit(
		"planning", "", "snapshot-only", "retired_collections",
		"", "", "v5 planning persists only Today membership and Next ordering",
	)

	contextNames := make([]string, 0, len(legacy.Machine.CustomContexts))
	for name := range legacy.Machine.CustomContexts {
		contextNames = append(contextNames, name)
	}
	sort.Slice(contextNames, func(i, j int) bool {
		left, right := strings.ToLower(contextNames[i]), strings.ToLower(contextNames[j])
		if left != right {
			return left < right
		}
		return contextNames[i] < contextNames[j]
	})
	seenContextNames := make(map[string]string, len(contextNames))
	for _, name := range contextNames {
		canonicalName := strings.ToLower(strings.TrimSpace(name))
		if prior := seenContextNames[canonicalName]; prior != "" {
			return V5DomainConversionResult{}, fmt.Errorf(
				"legacy custom contexts %q and %q collide case-insensitively",
				prior,
				name,
			)
		}
		seenContextNames[canonicalName] = name

		seenMembers := make(map[string]bool)
		var members []string
		for _, projectID := range legacy.Machine.CustomContexts[name] {
			switch {
			case !retainedIDs[projectID]:
				audit(
					"machine", name, "snapshot-only", "custom_context_member",
					projectID, "", "custom context member is not a retained v5 project",
				)
			case seenMembers[projectID]:
				audit(
					"machine", name, "repaired", "custom_context_member",
					projectID, "", "duplicate custom context membership was removed",
				)
			default:
				seenMembers[projectID] = true
				members = append(members, projectID)
			}
		}
		if len(members) == 0 {
			audit(
				"machine", name, "snapshot-only", "custom_context",
				name, "", "custom contexts with no retained project members are removed",
			)
			continue
		}
		result.Domains.Machine.CustomContexts[name] = members
	}
	audit(
		"machine", "", "converted", "theme",
		legacy.Settings.ActiveTheme, result.Domains.Machine.Theme,
		"v5 maps light to light and dark or custom themes to dark",
	)
	audit(
		"settings", "", "snapshot-only", "domain",
		strconv.Itoa(len(legacy.SettingsRaw))+" bytes", "",
		"v5 has no settings domain after its theme selection moves to machine state",
	)
	audit(
		"automation", "", "snapshot-only", "domain",
		strconv.Itoa(len(legacy.AutomationRaw))+" bytes", "",
		"v5 has no automation domain",
	)
	audit(
		"machine", "", "snapshot-only", "retired_fields",
		"", "", "v5 machine state retains only custom contexts and theme",
	)

	if err := result.Domains.Validate(); err != nil {
		return V5DomainConversionResult{}, err
	}
	return result, nil
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func mapLegacyV4Theme(active string) string {
	if strings.EqualFold(strings.TrimSpace(active), "light") {
		return "light"
	}
	return "dark"
}

func convertV5PlanningRefs[T any](
	items []T,
	retainedProjects map[string]bool,
	toRef func(T) V5PlanningTaskRef,
	projectID func(T) string,
	onDuplicate func(V5PlanningTaskRef),
) []V5PlanningTaskRef {
	seen := make(map[string]bool, len(items))
	result := make([]V5PlanningTaskRef, 0, len(items))
	for _, item := range items {
		if !retainedProjects[projectID(item)] {
			continue
		}
		ref := toRef(item)
		key := ref.ProjectID + "\x00" + ref.TaskID
		if seen[key] {
			if onDuplicate != nil {
				onDuplicate(ref)
			}
			continue
		}
		seen[key] = true
		result = append(result, ref)
	}
	return result
}

func assignV5ProjectColorSlots(projects []V5ProjectCandidate) {
	catalog := V5WorkspaceCatalogCandidate{Projects: projects}
	catalog.EnsureProjectColorSlots()
}

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

func encodeV5DomainEnvelope(kind string, workspaceID string, payload any) ([]byte, error) {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode v5 %s payload: %w", kind, err)
	}
	env := legacyV4DomainEnvelope{
		SchemaVersion: CurrentDomainSchemaVersion,
		WorkspaceID:   workspaceID,
		Kind:          kind,
		Data:          rawPayload,
	}
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode v5 %s envelope: %w", kind, err)
	}
	return data, nil
}
