package config

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

//nolint:gocyclo // This release matrix asserts every preserved domain field in one transaction.
func TestSplitDomainV3ToV4MigrationIsAtomicLosslessAndIdempotent(t *testing.T) {
	manager, dataDir := seededManager(t)
	domains, err := manager.LoadDomains()
	if err != nil {
		t.Fatalf("load seeded domains: %v", err)
	}
	workspaceID := domains.Catalog.WorkspaceID

	legacyCatalog := *domains.Catalog
	legacyCatalog.SchemaVersion = previousConfigVersion
	writeRawDomainPayload(t, dataDir, workspaceID, domainCatalog, &legacyCatalog)
	writeRawDomainPayload(t, dataDir, workspaceID, domainPlanning, map[string]any{
		"tui_state_version": CurrentTUIStateVersion,
		"streak_count":      9,
		"saved_filters": []map[string]any{{
			"id":              "legacy-filter",
			"name":            "Legacy filter",
			"mode":            "snapshot",
			"filter_status":   "todo",
			"filter_sections": []string{string(models.StateNext)},
			"filter_tags":     []string{"backend", "urgent"},
		}},
	})
	writeRawDomainPayload(t, dataDir, workspaceID, domainSettings, map[string]any{
		"tui_state_version":    CurrentTUIStateVersion,
		"default_context":      "work",
		"section_limit_active": 8,
		"section_limit_next":   3,
		"section_colors": map[string]string{
			"Active":    "red",
			"Completed": "green",
			"Waiting":   "yellow",
			// Custom workflow sections were presentation-only. Their colors
			// have no canonical v4 destination and are discarded.
			"New":      "yellow",
			"Features": "purple",
		},
		"state_group_colors": map[string]string{
			string(models.StateGroupBacklog): "blue",
		},
	})
	writeRawDomainPayload(t, dataDir, workspaceID, domainAutomation, map[string]any{
		"default_rules_initialized": true,
		"workflow_rules": []map[string]any{{
			"id":            "legacy-rule",
			"name":          "Legacy rule",
			"enabled":       false,
			"priority":      4,
			"trigger_count": 17,
			"source":        "imported",
			"trigger_group": map[string]any{
				"logic": "or",
				"triggers": []map[string]any{
					{"field": "status", "operation": "equals", "value": "todo"},
					{"field": "section", "operation": "equals", "value": "Completed"},
				},
			},
			"actions": []map[string]any{{
				"action_type": "set_status",
				"value":       "blocked",
				"condition":   map[string]any{"field": "section", "operation": "equals", "value": "Next"},
			}},
			"scope": map[string]any{
				"projects": []string{"project-b", "project-a"},
				"sections": []string{"Active", "Inbox"},
			},
		}},
		"task_templates": []map[string]any{{
			"id":          "legacy-template",
			"name":        "Legacy template",
			"enabled":     true,
			"source":      "imported",
			"type":        "task",
			"section":     "Next",
			"tags":        []string{"second", "first"},
			"checklist":   []string{"two", "one"},
			"use_count":   12,
			"description": "preserved",
		}},
	})

	paths := syncedDomainPathMap(t, manager.domainStore())
	before := readDomainBytes(t, paths)

	migrated, err := manager.LoadDomains()
	if err != nil {
		t.Fatalf("LoadDomains migration: %v", err)
	}
	if migrated.Catalog.SchemaVersion != CurrentConfigVersion {
		t.Fatalf("catalog schema version = %d, want %d", migrated.Catalog.SchemaVersion, CurrentConfigVersion)
	}
	if migrated.Planning.StreakCount != 9 || len(migrated.Planning.SavedFilters) != 1 {
		t.Fatalf("planning metadata changed: %+v", migrated.Planning)
	}
	filter := migrated.Planning.SavedFilters[0]
	if !filter.HasStateFilter() || !filter.FilterStates.Equal(mustStateSet(t, models.StateNext)) ||
		strings.Join(filter.FilterTags, ",") != "backend,urgent" {
		t.Fatalf("migrated filter = %+v", filter)
	}
	if got := migrated.Settings.StateGroupLimits[models.StateGroupBacklog]; got != 8 {
		t.Fatalf("backlog limit = %d, want 8", got)
	}
	if got := migrated.Settings.StateGroupLimits[models.StateGroupInProgress]; got != 8 {
		t.Fatalf("in-progress limit = %d, want 8", got)
	}
	if got := migrated.Settings.StateGroupColors[models.StateGroupBacklog]; got != "blue" {
		t.Fatalf("explicit canonical backlog color = %q, want blue", got)
	}
	if got := migrated.Settings.StateGroupColors[models.StateGroupInProgress]; got != "red" {
		t.Fatalf("migrated in-progress color = %q, want red", got)
	}
	for _, group := range []models.WorkflowStateGroup{models.StateGroupDone, models.StateGroupCanceled} {
		if got := migrated.Settings.StateGroupColors[group]; got != "green" {
			t.Fatalf("%s color = %q, want green", group, got)
		}
	}
	if migrated.Settings.DefaultContext != "work" {
		t.Fatalf("default context = %q, want work", migrated.Settings.DefaultContext)
	}

	if len(migrated.Automation.WorkflowRules) != 1 {
		t.Fatalf("workflow rules = %d, want 1", len(migrated.Automation.WorkflowRules))
	}
	rule := migrated.Automation.WorkflowRules[0]
	if rule.ID != "legacy-rule" || rule.Enabled || rule.Priority != 4 ||
		rule.TriggerCount != 17 || rule.Source != "imported" {
		t.Fatalf("workflow metadata changed: %+v", rule)
	}
	if got := rule.TriggerGroup.Triggers[0]; got != (workflow.RuleTrigger{
		Field: workflow.FieldState, Operation: workflow.OpIn, Value: "inbox,ready,next",
	}) {
		t.Fatalf("status trigger = %+v", got)
	}
	if got := rule.Actions[0]; got.ActionType != workflow.ActionSetState ||
		got.Value != string(models.StateWaiting) || got.Condition == nil ||
		*got.Condition != (workflow.RuleTrigger{
			Field: workflow.FieldState, Operation: workflow.OpEquals, Value: "next",
		}) {
		t.Fatalf("migrated action = %+v", got)
	}
	if !equalStates(rule.Scope.States, []models.WorkflowState{
		models.StateInbox, models.StateReady, models.StateInProgress,
	}) || strings.Join(rule.Scope.Projects, ",") != "project-b,project-a" {
		t.Fatalf("migrated scope = %+v", rule.Scope)
	}

	if len(migrated.Automation.TaskTemplates) != 1 {
		t.Fatalf("task templates = %d, want 1", len(migrated.Automation.TaskTemplates))
	}
	template := migrated.Automation.TaskTemplates[0]
	if template.ID != "legacy-template" || template.State != models.StateNext ||
		template.UseCount != 12 || strings.Join(template.Tags, ",") != "second,first" ||
		strings.Join(template.Checklist, ",") != "two,one" ||
		template.Description != "preserved" {
		t.Fatalf("migrated template = %+v", template)
	}

	after := readDomainBytes(t, paths)
	for kind, path := range paths {
		if bytes.Equal(after[kind], before[kind]) {
			t.Fatalf("%s was not rewritten", kind.name())
		}
		backup, err := os.ReadFile(path + ".bak")
		if err != nil {
			t.Fatalf("read %s migration backup: %v", kind.name(), err)
		}
		if !bytes.Equal(backup, before[kind]) {
			t.Fatalf("%s backup does not contain byte-exact v3 payload", kind.name())
		}
	}
	assertCanonicalV4Payloads(t, after)

	if _, err := manager.LoadDomains(); err != nil {
		t.Fatalf("idempotent reload: %v", err)
	}
	stable := readDomainBytes(t, paths)
	for kind := range paths {
		if !bytes.Equal(stable[kind], after[kind]) {
			t.Fatalf("idempotent reload rewrote %s", kind.name())
		}
	}
}

func TestSplitDomainV3ToV4MigrationValidationFailureWritesNothing(t *testing.T) {
	manager, dataDir := seededManager(t)
	domains, err := manager.LoadDomains()
	if err != nil {
		t.Fatalf("load seeded domains: %v", err)
	}
	workspaceID := domains.Catalog.WorkspaceID

	legacyCatalog := *domains.Catalog
	legacyCatalog.SchemaVersion = previousConfigVersion
	writeRawDomainPayload(t, dataDir, workspaceID, domainCatalog, &legacyCatalog)
	writeRawDomainPayload(t, dataDir, workspaceID, domainPlanning, map[string]any{
		"tui_state_version": CurrentTUIStateVersion,
	})
	writeRawDomainPayload(t, dataDir, workspaceID, domainSettings, map[string]any{
		"tui_state_version": CurrentTUIStateVersion,
		"section_colors":    map[string]string{"Active": "red"},
	})
	writeRawDomainPayload(t, dataDir, workspaceID, domainAutomation, map[string]any{
		"workflow_rules": []map[string]any{{
			"id":      "invalid",
			"name":    "Invalid legacy rule",
			"enabled": true,
			"trigger": map[string]any{"field": "status", "operation": "equals", "value": "mystery"},
			"actions": []map[string]any{{"action_type": "set_status", "value": "done"}},
		}},
	})

	paths := syncedDomainPathMap(t, manager.domainStore())
	before := readDomainBytes(t, paths)
	if _, err := manager.LoadDomains(); err == nil || !strings.Contains(err.Error(), "unknown legacy task status") {
		t.Fatalf("LoadDomains error = %v, want unknown legacy task status", err)
	}
	after := readDomainBytes(t, paths)
	for kind := range paths {
		if !bytes.Equal(after[kind], before[kind]) {
			t.Fatalf("failed migration changed %s", kind.name())
		}
	}
}

func TestCurrentSchemaRejectsRetiredLifecycleInputs(t *testing.T) {
	tests := []struct {
		name    string
		kind    domainKind
		payload map[string]any
		want    string
	}{
		{
			name: "template section",
			kind: domainAutomation,
			payload: map[string]any{"task_templates": []map[string]any{{
				"id": "legacy-template", "name": "Legacy", "enabled": true,
				"type": "task", "section": "Next",
			}}},
			want: "explicit canonical state",
		},
		{
			name: "saved filter status",
			kind: domainPlanning,
			payload: map[string]any{"tui_state_version": CurrentTUIStateVersion, "saved_filters": []map[string]any{{
				"id": "legacy-filter", "name": "Legacy", "mode": "snapshot", "filter_status": "todo",
			}}},
			want: "retired status or section",
		},
		{
			name: "workflow trigger",
			kind: domainAutomation,
			payload: map[string]any{"workflow_rules": []map[string]any{{
				"id": "legacy-rule", "name": "Legacy", "enabled": true,
				"trigger": map[string]any{"field": "status", "operation": "equals", "value": "todo"},
				"actions": []map[string]any{{"action_type": "add_tag", "value": "legacy"}},
			}}},
			want: "invalid field: status",
		},
		{
			name: "workflow section scope",
			kind: domainAutomation,
			payload: map[string]any{"workflow_rules": []map[string]any{{
				"id": "legacy-scope", "name": "Legacy scope", "enabled": true,
				"triggers": []map[string]any{{"field": "state", "operation": "equals", "value": "ready"}},
				"actions":  []map[string]any{{"action_type": "add_tag", "value": "legacy"}},
				"scope":    map[string]any{"sections": []string{"Next"}},
			}}},
			want: "sections input was removed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, dataDir := seededManager(t)
			domains, err := manager.LoadDomains()
			if err != nil {
				t.Fatalf("load seeded domains: %v", err)
			}
			writeRawDomainPayload(t, dataDir, domains.Catalog.WorkspaceID, tt.kind, tt.payload)

			if _, err := manager.LoadDomains(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadDomains error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestV3DirectDomainLoadDefersPersistenceToBundleMigration(t *testing.T) {
	manager, dataDir := seededManager(t)
	domains, err := manager.LoadDomains()
	if err != nil {
		t.Fatalf("load seeded domains: %v", err)
	}
	workspaceID := domains.Catalog.WorkspaceID

	legacyCatalog := *domains.Catalog
	legacyCatalog.SchemaVersion = previousConfigVersion
	writeRawDomainPayload(t, dataDir, workspaceID, domainCatalog, &legacyCatalog)
	path := writeRawDomainPayload(t, dataDir, workspaceID, domainAutomation, map[string]any{
		"task_templates": []map[string]any{{
			"id":      "legacy-template",
			"name":    "Legacy template",
			"enabled": true,
			"type":    "task",
			"section": "Next",
		}},
	})
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	loaded, revision, err := manager.domainStore().LoadAutomationChecked()
	if err != nil {
		t.Fatalf("LoadAutomationChecked: %v", err)
	}
	if len(loaded.TaskTemplates) != 1 || loaded.TaskTemplates[0].State != models.StateNext {
		t.Fatalf("direct load did not canonicalize in memory: %+v", loaded.TaskTemplates)
	}
	afterDirectLoad, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterDirectLoad, before) || revision != revisionOf(before) {
		t.Fatal("direct v3 domain load persisted outside the bundle transaction")
	}

	if _, migrationErr := manager.LoadDomains(); migrationErr != nil {
		t.Fatalf("bundle migration: %v", migrationErr)
	}
	afterBundleLoad, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(afterBundleLoad, before) {
		t.Fatal("bundle migration did not persist deferred automation cleanup")
	}
}

func syncedDomainPathMap(t *testing.T, store *DomainFileStore) map[domainKind]string {
	t.Helper()
	paths := make(map[domainKind]string, len(syncedDomainKinds))
	for _, kind := range syncedDomainKinds {
		path, err := store.pathFor(kind)
		if err != nil {
			t.Fatalf("resolve %s path: %v", kind.name(), err)
		}
		paths[kind] = path
	}
	return paths
}

func readDomainBytes(t *testing.T, paths map[domainKind]string) map[domainKind][]byte {
	t.Helper()
	result := make(map[domainKind][]byte, len(paths))
	for kind, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", kind.name(), err)
		}
		result[kind] = data
	}
	return result
}

func mustStateSet(t *testing.T, states ...models.WorkflowState) models.WorkflowStateSet {
	t.Helper()
	set, err := models.NewWorkflowStateSet(states...)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func equalStates(got, want []models.WorkflowState) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func assertCanonicalV4Payloads(t *testing.T, artifacts map[domainKind][]byte) {
	t.Helper()
	payloads := make(map[domainKind][]byte, len(artifacts))
	for kind, data := range artifacts {
		var envelope domainEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			t.Fatalf("decode %s envelope: %v", kind.name(), err)
		}
		payloads[kind] = envelope.Data
	}

	var catalog WorkspaceCatalog
	if err := json.Unmarshal(payloads[domainCatalog], &catalog); err != nil {
		t.Fatalf("decode catalog payload: %v", err)
	}
	if catalog.SchemaVersion != CurrentConfigVersion {
		t.Fatalf("persisted catalog version = %d, want %d", catalog.SchemaVersion, CurrentConfigVersion)
	}
	for _, token := range []string{`"filter_status"`, `"filter_sections"`} {
		if bytes.Contains(payloads[domainPlanning], []byte(token)) {
			t.Fatalf("canonical planning retained %s: %s", token, payloads[domainPlanning])
		}
	}
	for _, token := range []string{`"section_limit_`, `"section_colors"`} {
		if bytes.Contains(payloads[domainSettings], []byte(token)) {
			t.Fatalf("canonical settings retained %s: %s", token, payloads[domainSettings])
		}
	}
	for _, token := range []string{
		`"field":"status"`, `"field":"section"`, `"action_type":"set_status"`,
		`"action_type":"set_section"`, `"sections"`, `"section"`,
	} {
		if bytes.Contains(payloads[domainAutomation], []byte(token)) {
			t.Fatalf("canonical automation retained %s: %s", token, payloads[domainAutomation])
		}
	}
}
