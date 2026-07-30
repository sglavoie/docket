package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	legacyV4DomainEnvelopeSchema = 1
	legacyV4ConfigSchema         = 4
	legacyV4TUIStateVersion      = 4
)

// LegacyV4DomainArtifacts is the byte-exact five-domain input to the v5
// configuration migration. Keeping envelope decoding here, rather than routing
// through the live domain store, lets the runtime Settings and Automation
// models disappear without losing the ability to upgrade a v4 workspace.
type LegacyV4DomainArtifacts struct {
	Workspace  []byte
	Planning   []byte
	Settings   []byte
	Automation []byte
	Machine    []byte
}

// LegacyV4Domains is the migration-only representation of the five v4
// configuration domains. Only values that affect a v5 candidate are decoded
// into fields; the complete settings and automation payloads remain available
// as raw JSON for snapshot-only audit accounting.
type LegacyV4Domains struct {
	WorkspaceID   string
	Catalog       LegacyV4WorkspaceCatalog
	Planning      LegacyV4PlanningState
	Settings      LegacyV4Settings
	Automation    LegacyV4AutomationConfig
	Machine       LegacyV4MachineState
	SettingsRaw   json.RawMessage
	AutomationRaw json.RawMessage
}

type LegacyV4WorkspaceCatalog struct {
	SchemaVersion        int                      `json:"schema_version"`
	WorkspaceID          string                   `json:"workspace_id,omitempty"`
	Projects             []LegacyV4CatalogProject `json:"projects"`
	SystemInboxProjectID string                   `json:"system_inbox_project_id,omitempty"`
}

// LegacyV4CatalogProject deliberately owns every v4 project metadata field,
// including fields that later runtime models remove.
type LegacyV4CatalogProject struct {
	ID               string     `json:"id"`
	Slug             string     `json:"slug"`
	Name             string     `json:"name"`
	Description      string     `json:"description,omitempty"`
	StatusIcon       string     `json:"status_icon,omitempty"`
	StatusIconSource string     `json:"status_icon_source,omitempty"`
	Context          string     `json:"context"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Archived         bool       `json:"archived"`
	ArchivedAt       *time.Time `json:"archived_at,omitempty"`
	Pinned           bool       `json:"pinned"`
	PinnedAt         *time.Time `json:"pinned_at,omitempty"`
	Sandbox          bool       `json:"sandbox,omitempty"`
	Internal         bool       `json:"internal,omitempty"`
	ColorSlot        int        `json:"color_slot,omitempty"`
}

type LegacyV4PlanningState struct {
	TUIStateVersion int                      `json:"tui_state_version"`
	FocusQueue      []LegacyV4FocusQueueItem `json:"focus_queue,omitempty"`
	DailyPlan       []LegacyV4DailyPlanItem  `json:"daily_plan,omitempty"`
}

type LegacyV4FocusQueueItem struct {
	TaskID    string `json:"task_id"`
	ProjectID string `json:"project_id"`
}

type LegacyV4DailyPlanItem struct {
	TaskID      string `json:"task_id"`
	ProjectID   string `json:"project_id"`
	PlannedDate string `json:"planned_date"`
}

// LegacyV4Settings retains only the value that moves into v5 machine state.
// The raw payload on LegacyV4Domains accounts for every other setting as
// snapshot-only data.
type LegacyV4Settings struct {
	TUIStateVersion int    `json:"tui_state_version"`
	ActiveTheme     string `json:"active_theme,omitempty"`
}

// LegacyV4AutomationConfig is intentionally fieldless. Automation is valid JSON
// migration input, but none of it has a v5 target.
type LegacyV4AutomationConfig struct{}

type LegacyV4MachineState struct {
	TUIStateVersion int                 `json:"tui_state_version"`
	CustomContexts  map[string][]string `json:"custom_contexts,omitempty"`
}

type legacyV4DomainEnvelope struct {
	SchemaVersion int             `json:"schema_version"`
	WorkspaceID   string          `json:"workspace_id"`
	Kind          string          `json:"kind"`
	Data          json.RawMessage `json:"data"`
}

// DecodeLegacyV4Domains validates and decodes a complete five-domain v4 bundle
// without touching the filesystem or constructing live runtime configuration.
func DecodeLegacyV4Domains(artifacts LegacyV4DomainArtifacts) (LegacyV4Domains, error) {
	specs := []struct {
		kind string
		data []byte
		out  any
	}{
		{kind: "workspace", data: artifacts.Workspace},
		{kind: "planning", data: artifacts.Planning},
		{kind: "settings", data: artifacts.Settings},
		{kind: "automation", data: artifacts.Automation},
		{kind: "machine", data: artifacts.Machine},
	}

	var result LegacyV4Domains
	specs[0].out = &result.Catalog
	specs[1].out = &result.Planning
	specs[2].out = &result.Settings
	specs[3].out = &result.Automation
	specs[4].out = &result.Machine

	for _, spec := range specs {
		env, err := decodeLegacyV4DomainEnvelope(spec.kind, spec.data)
		if err != nil {
			return LegacyV4Domains{}, err
		}
		if result.WorkspaceID == "" {
			result.WorkspaceID = env.WorkspaceID
		} else if env.WorkspaceID != result.WorkspaceID {
			return LegacyV4Domains{}, fmt.Errorf(
				"legacy v4 %s domain workspace ID %q does not match %q",
				spec.kind,
				env.WorkspaceID,
				result.WorkspaceID,
			)
		}
		if err := json.Unmarshal(env.Data, spec.out); err != nil {
			return LegacyV4Domains{}, fmt.Errorf("decode legacy v4 %s payload: %w", spec.kind, err)
		}
		switch spec.kind {
		case "settings":
			result.SettingsRaw = append(json.RawMessage(nil), env.Data...)
		case "automation":
			result.AutomationRaw = append(json.RawMessage(nil), env.Data...)
		}
	}

	if result.Catalog.WorkspaceID != "" && result.Catalog.WorkspaceID != result.WorkspaceID {
		return LegacyV4Domains{}, fmt.Errorf(
			"legacy v4 catalog payload workspace ID %q does not match envelope ID %q",
			result.Catalog.WorkspaceID,
			result.WorkspaceID,
		)
	}
	if err := validateLegacyV4Domains(result); err != nil {
		return LegacyV4Domains{}, err
	}
	return result, nil
}

func decodeLegacyV4DomainEnvelope(kind string, data []byte) (legacyV4DomainEnvelope, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return legacyV4DomainEnvelope{}, fmt.Errorf("legacy v4 %s domain is empty", kind)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var env legacyV4DomainEnvelope
	if err := decoder.Decode(&env); err != nil {
		return legacyV4DomainEnvelope{}, fmt.Errorf("decode legacy v4 %s envelope: %w", kind, err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return legacyV4DomainEnvelope{}, fmt.Errorf("decode legacy v4 %s envelope: %w", kind, err)
	}
	if env.SchemaVersion != legacyV4DomainEnvelopeSchema {
		return legacyV4DomainEnvelope{}, fmt.Errorf(
			"legacy v4 %s envelope schema is %d, want %d",
			kind,
			env.SchemaVersion,
			legacyV4DomainEnvelopeSchema,
		)
	}
	if env.Kind != kind {
		return legacyV4DomainEnvelope{}, fmt.Errorf(
			"legacy v4 %s artifact contains %q domain",
			kind,
			env.Kind,
		)
	}
	if err := validateWorkspaceID(env.WorkspaceID); err != nil {
		return legacyV4DomainEnvelope{}, fmt.Errorf("legacy v4 %s envelope: %w", kind, err)
	}
	if err := validateLegacyV4JSONObject(kind, env.Data); err != nil {
		return legacyV4DomainEnvelope{}, err
	}
	return env, nil
}

func validateLegacyV4JSONObject(kind string, raw json.RawMessage) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return fmt.Errorf("legacy v4 %s payload must be a JSON object: %w", kind, err)
	}
	if object == nil {
		return fmt.Errorf("legacy v4 %s payload must be a JSON object", kind)
	}
	return nil
}

//nolint:gocyclo // Validation intentionally reports every legacy-domain invariant in one pass.
func validateLegacyV4Domains(domains LegacyV4Domains) error {
	if domains.Catalog.SchemaVersion != legacyV4ConfigSchema {
		return fmt.Errorf(
			"legacy workspace payload schema is %d, want %d",
			domains.Catalog.SchemaVersion,
			legacyV4ConfigSchema,
		)
	}
	if domains.Catalog.SystemInboxProjectID == "" {
		return fmt.Errorf("legacy v4 catalog has no protected system Inbox ID")
	}

	projectIDs := make(map[string]bool, len(domains.Catalog.Projects))
	projectSlugs := make(map[string]bool, len(domains.Catalog.Projects))
	for index, project := range domains.Catalog.Projects {
		if strings.TrimSpace(project.ID) == "" {
			return fmt.Errorf("legacy v4 catalog project %d has an empty ID", index)
		}
		if projectIDs[project.ID] {
			return fmt.Errorf("legacy v4 catalog has duplicate project ID %q", project.ID)
		}
		if err := validateLegacyV4ProjectSlug(project.Slug, project.Sandbox); err != nil {
			return fmt.Errorf("legacy v4 project %q: %w", project.ID, err)
		}
		if projectSlugs[project.Slug] {
			return fmt.Errorf("legacy v4 catalog has duplicate project slug %q", project.Slug)
		}
		if strings.TrimSpace(project.Name) == "" {
			return fmt.Errorf("legacy v4 project %q has an empty name", project.ID)
		}
		if strings.TrimSpace(project.Context) == "" {
			return fmt.Errorf("legacy v4 project %q has an empty regular context", project.ID)
		}
		if project.ColorSlot < 0 || project.ColorSlot > ProjectColorSlotCount {
			return fmt.Errorf("legacy v4 project %q has invalid color slot %d", project.ID, project.ColorSlot)
		}
		if project.Archived && project.ArchivedAt == nil {
			return fmt.Errorf("legacy v4 project %q is archived without an archive timestamp", project.ID)
		}
		if project.Pinned && project.PinnedAt == nil {
			return fmt.Errorf("legacy v4 project %q is pinned without a pin timestamp", project.ID)
		}
		projectIDs[project.ID] = true
		projectSlugs[project.Slug] = true
	}
	if !projectIDs[domains.Catalog.SystemInboxProjectID] {
		return fmt.Errorf(
			"legacy v4 protected system Inbox references unknown project %q",
			domains.Catalog.SystemInboxProjectID,
		)
	}

	if domains.Planning.TUIStateVersion > legacyV4TUIStateVersion {
		return fmt.Errorf("legacy v4 planning TUI state version %d is unsupported", domains.Planning.TUIStateVersion)
	}
	for index, item := range domains.Planning.FocusQueue {
		if strings.TrimSpace(item.TaskID) == "" || strings.TrimSpace(item.ProjectID) == "" {
			return fmt.Errorf("legacy v4 Next order item %d has incomplete identity", index)
		}
	}
	for index, item := range domains.Planning.DailyPlan {
		if strings.TrimSpace(item.TaskID) == "" || strings.TrimSpace(item.ProjectID) == "" {
			return fmt.Errorf("legacy v4 Today item %d has incomplete identity", index)
		}
		if item.PlannedDate != "" {
			if _, err := time.Parse("2006-01-02", item.PlannedDate); err != nil {
				return fmt.Errorf("legacy v4 Today item %d has invalid planned date %q", index, item.PlannedDate)
			}
		}
	}
	if domains.Settings.TUIStateVersion > legacyV4TUIStateVersion {
		return fmt.Errorf("legacy v4 settings TUI state version %d is unsupported", domains.Settings.TUIStateVersion)
	}
	if domains.Machine.TUIStateVersion > legacyV4TUIStateVersion {
		return fmt.Errorf("legacy v4 machine TUI state version %d is unsupported", domains.Machine.TUIStateVersion)
	}
	for name, members := range domains.Machine.CustomContexts {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(name) != name {
			return fmt.Errorf("legacy v4 machine has invalid custom context name %q", name)
		}
		for index, projectID := range members {
			if strings.TrimSpace(projectID) == "" {
				return fmt.Errorf("legacy v4 custom context %q member %d has an empty project ID", name, index)
			}
		}
	}
	return nil
}

func validateLegacyV4ProjectSlug(slug string, sandbox bool) error {
	if slug == "_sandbox" {
		if sandbox {
			return nil
		}
		return fmt.Errorf("slug %q is reserved for Sandbox", slug)
	}
	if len(slug) == 0 || len(slug) > 100 {
		return fmt.Errorf("invalid slug length %d", len(slug))
	}
	switch slug {
	case ".tt", "projects", "new", "list", "delete", "switch", "archive":
		return fmt.Errorf("slug %q is reserved", slug)
	}
	for index, r := range slug {
		valid := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-'
		if !valid || index == 0 && r == '-' {
			return fmt.Errorf("slug %q is not a canonical safe project slug", slug)
		}
	}
	if strings.HasSuffix(slug, "-") || strings.Contains(slug, "--") {
		return fmt.Errorf("slug %q is not a canonical safe project slug", slug)
	}
	return nil
}
