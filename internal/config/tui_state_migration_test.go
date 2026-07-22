package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTUIStateMigrationFixtures(t *testing.T) {
	tests := []struct {
		name    string
		before  string
		after   string
		migrate func([]byte) ([]byte, bool, error)
	}{
		{
			name: "machine", before: "machine-before.json", after: "machine-after.json",
			migrate: func(data []byte) ([]byte, bool, error) {
				var state MachineState
				if err := json.Unmarshal(data, &state); err != nil {
					return nil, false, err
				}
				changed, err := state.MigrateTUIState()
				out, marshalErr := json.Marshal(state)
				if err == nil {
					err = marshalErr
				}
				return out, changed, err
			},
		},
		{
			name: "planning", before: "planning-before.json", after: "planning-after.json",
			migrate: func(data []byte) ([]byte, bool, error) {
				var state PlanningState
				if err := json.Unmarshal(data, &state); err != nil {
					return nil, false, err
				}
				changed, err := state.MigrateTUIState()
				out, marshalErr := json.Marshal(state)
				if err == nil {
					err = marshalErr
				}
				return out, changed, err
			},
		},
		{
			name: "settings", before: "settings-before.json", after: "settings-after.json",
			migrate: func(data []byte) ([]byte, bool, error) {
				var state Settings
				if err := json.Unmarshal(data, &state); err != nil {
					return nil, false, err
				}
				changed, err := state.MigrateTUIState()
				out, marshalErr := json.Marshal(state)
				if err == nil {
					err = marshalErr
				}
				return out, changed, err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := readTUIStateFixture(t, tc.before)
			want := readTUIStateFixture(t, tc.after)
			got, changed, err := tc.migrate(before)
			if err != nil {
				t.Fatalf("migrate fixture: %v", err)
			}
			if !changed {
				t.Fatal("first migration reported no change")
			}
			assertJSONEqual(t, got, want)

			again, changed, err := tc.migrate(got)
			if err != nil {
				t.Fatalf("repeat migration: %v", err)
			}
			if changed {
				t.Fatal("repeat migration was not idempotent")
			}
			assertJSONEqual(t, again, want)
		})
	}
}

func TestMachineTUIStateMigrationMapsEveryLegacyStartValue(t *testing.T) {
	tests := []struct {
		legacy string
		view   string
		route  StartRouteState
	}{
		{"dashboard", StartViewHome, StartRouteState{}},
		{"focus-board", StartViewPlanner, StartRouteState{PlannerTab: StartPlannerToday}},
		{"backlog", StartViewPlanner, StartRouteState{PlannerTab: StartPlannerBacklog}},
		{"cross-project-outliner", StartViewTasks, StartRouteState{TaskFocus: StartTaskActive}},
		{"all-tasks", StartViewTasks, StartRouteState{TaskFocus: StartTaskAll}},
		{"priority", StartViewTasks, StartRouteState{TaskFocus: StartTaskHighPriority, TaskSort: StartTaskSortPriority}},
		{"due-dates", StartViewTasks, StartRouteState{TaskFocus: StartTaskAll, TaskSort: StartTaskSortDueDate}},
		{"daily-review", StartViewReview, StartRouteState{ReviewModule: StartReviewDaily}},
		{"not-a-view", StartViewPlanner, StartRouteState{PlannerTab: StartPlannerToday}},
	}
	for _, tc := range tests {
		t.Run(tc.legacy, func(t *testing.T) {
			machine := &MachineState{StartView: tc.legacy}
			changed, err := machine.MigrateTUIState()
			if err != nil || !changed {
				t.Fatalf("MigrateTUIState changed=%v err=%v", changed, err)
			}
			if machine.StartView != tc.view || !reflect.DeepEqual(machine.StartRoute, tc.route) {
				t.Fatalf("migration = %q %+v, want %q %+v", machine.StartView, machine.StartRoute, tc.view, tc.route)
			}
		})
	}
}

func TestMachineUnmarshalMergesLegacySidebarStatePerProject(t *testing.T) {
	payload := []byte(`{
		"project_sidebar_enabled":{"current":false},
		"project_view_states":{
			"current":{"sidebar_enabled":true},
			"legacy":{"sidebar_enabled":true}
		}
	}`)
	var machine MachineState
	if err := json.Unmarshal(payload, &machine); err != nil {
		t.Fatal(err)
	}
	if machine.ProjectSidebarEnabled["current"] {
		t.Fatal("legacy value overwrote explicit current sidebar value")
	}
	if !machine.ProjectSidebarEnabled["legacy"] {
		t.Fatal("legacy-only project sidebar value was not imported")
	}
}

func TestLoadTimeTUIStateMigrationIsAtomicPerDomainAndIdempotent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	manager := NewManager(ConfigPathForDataDir(dataDir))
	domains, err := manager.LoadOrCreateDomains()
	if err != nil {
		t.Fatalf("LoadOrCreateDomains: %v", err)
	}
	workspaceID := domains.Catalog.WorkspaceID

	catalogPath := WorkspaceCatalogPath(dataDir)
	automationPath := AutomationPath(dataDir)
	catalogBefore, _ := os.ReadFile(catalogPath)
	automationBefore, _ := os.ReadFile(automationPath)

	planningPath := writeRawDomainPayload(t, dataDir, workspaceID, domainPlanning, map[string]any{
		"tui_state_version": 0,
		"streak_count":      7,
		"saved_filters": []map[string]any{{
			"id": "f", "name": "Bugs", "mode": "snapshot", "filter_type": "bug", "sort_field": 2,
		}},
	})
	settingsPath := writeRawDomainPayload(t, dataDir, workspaceID, domainSettings, map[string]any{
		"tui_state_version":     0,
		"default_context":       "work",
		"popup_timeout_seconds": 8,
	})
	machinePath := writeRawDomainPayload(t, dataDir, workspaceID, domainMachine, map[string]any{
		"tui_state_version": 0,
		"editor":            "vim",
		"start_view":        "due-dates",
		"command_history":   []string{"discard"},
	})
	before := map[string][]byte{}
	for _, path := range []string{planningPath, settingsPath, machinePath} {
		before[path], _ = os.ReadFile(path)
	}

	runtime, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	if runtime.Planning().StreakCount != 7 || runtime.Settings().DefaultContext != "work" || runtime.Machine().Editor != "vim" {
		t.Fatalf("durable values changed: planning=%d settings=%q machine=%q", runtime.Planning().StreakCount, runtime.Settings().DefaultContext, runtime.Machine().Editor)
	}
	if runtime.Machine().GetStartView() != StartViewTasks ||
		runtime.Machine().GetStartRoute().TaskFocus != StartTaskAll ||
		runtime.Machine().GetStartRoute().TaskSort != StartTaskSortDueDate {
		t.Fatalf("startup route not migrated: %q %+v", runtime.Machine().GetStartView(), runtime.Machine().GetStartRoute())
	}

	after := map[string][]byte{}
	for _, path := range []string{planningPath, settingsPath, machinePath} {
		after[path], _ = os.ReadFile(path)
		backup, readErr := os.ReadFile(path + ".bak")
		if readErr != nil {
			t.Fatalf("read migration backup %s: %v", path, readErr)
		}
		if !bytes.Equal(backup, before[path]) {
			t.Fatalf("migration backup for %s does not contain the complete before-domain", path)
		}
	}
	if got, _ := os.ReadFile(catalogPath); !bytes.Equal(got, catalogBefore) {
		t.Fatal("catalog changed during TUI-state migration")
	}
	if got, _ := os.ReadFile(automationPath); !bytes.Equal(got, automationBefore) {
		t.Fatal("automation changed during TUI-state migration")
	}

	if _, err := manager.LoadRuntimeState(); err != nil {
		t.Fatalf("idempotent reload: %v", err)
	}
	for path, want := range after {
		got, _ := os.ReadFile(path)
		if !bytes.Equal(got, want) {
			t.Fatalf("idempotent reload rewrote %s", path)
		}
	}
}

func readTUIStateFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "tui-state", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func assertJSONEqual(t *testing.T, got, want []byte) {
	t.Helper()
	var gotValue, wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("decode got JSON: %v", err)
	}
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("decode want JSON: %v", err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("JSON mismatch\ngot:  %s\nwant: %s", got, want)
	}
}
