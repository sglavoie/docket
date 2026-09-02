package config

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

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

	retainedIDs, err := convertV5CatalogProjects(legacy, &result, audit)
	if err != nil {
		return V5DomainConversionResult{}, err
	}
	convertV5Planning(legacy, &result, retainedIDs, audit)
	if err := convertV5Machine(legacy, &result, retainedIDs, audit); err != nil {
		return V5DomainConversionResult{}, err
	}

	if err := result.Domains.Validate(); err != nil {
		return V5DomainConversionResult{}, err
	}
	return result, nil
}

// v5AuditFunc records one audit entry during conversion.
type v5AuditFunc func(domain, entityID, action, field, from, to, reason string)

// convertV5CatalogProjects fills the v5 catalog and reports which legacy
// project IDs survive into it.
func convertV5CatalogProjects(
	legacy LegacyV4Domains,
	result *V5DomainConversionResult,
	audit v5AuditFunc,
) (map[string]bool, error) {
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
		return nil, fmt.Errorf(
			"protected system Inbox %q would be removed from the v5 catalog",
			result.Domains.Catalog.SystemInboxProjectID,
		)
	}
	assignV5ProjectColorSlots(result.Domains.Catalog.Projects)
	return retainedIDs, nil
}

// convertV5Planning rebuilds Today membership and Next ordering over the
// retained projects only.
func convertV5Planning(
	legacy LegacyV4Domains,
	result *V5DomainConversionResult,
	retainedIDs map[string]bool,
	audit v5AuditFunc,
) {
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
}

// convertV5Machine narrows machine-local state to custom contexts over
// retained projects plus the mapped theme.
func convertV5Machine(
	legacy LegacyV4Domains,
	result *V5DomainConversionResult,
	retainedIDs map[string]bool,
	audit v5AuditFunc,
) error {
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
			return fmt.Errorf(
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
	return nil
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
