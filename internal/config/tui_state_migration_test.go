package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/models"
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

func TestMachineTUIStateMigrationCanonicalizesLegacyStartup(t *testing.T) {
	legacy := []byte(`{
		"editor": "vim",
		"start_view": "due-dates",
		"start_route": {"task_focus": "all", "task_sort": "due-date"},
		"home_projects_height_pct": 42,
		"sidebar_enabled": true
	}`)
	var machine MachineState
	if err := json.Unmarshal(legacy, &machine); err != nil {
		t.Fatalf("unmarshal legacy machine: %v", err)
	}
	changed, err := machine.MigrateTUIState()
	if err != nil || !changed {
		t.Fatalf("MigrateTUIState changed=%v err=%v", changed, err)
	}
	if machine.TUIStateVersion != CurrentTUIStateVersion {
		t.Fatalf("version = %d, want %d", machine.TUIStateVersion, CurrentTUIStateVersion)
	}
	if machine.Editor != "vim" {
		t.Fatalf("durable editor lost: %q", machine.Editor)
	}
	if machine.StartupRoute.Mode != OutlinerModeDue {
		t.Fatalf("startup route = %+v, want Due", machine.StartupRoute)
	}
	out, err := json.Marshal(machine)
	if err != nil {
		t.Fatalf("marshal migrated machine: %v", err)
	}
	for _, field := range []string{"start_view", "start_route", "home_projects_height_pct", "sidebar_enabled"} {
		if bytes.Contains(out, []byte(field)) {
			t.Fatalf("migrated machine still emits retired field %q: %s", field, out)
		}
	}
}

func TestNormalizeOutlinerRouteStateLegacyModes(t *testing.T) {
	active := mustTestStateSet(t, models.StateReady, models.StateInProgress)
	terminal := mustTestStateSet(t, models.StateDone, models.StateCanceled)
	tests := []struct {
		legacy  string
		mode    string
		states  models.WorkflowStateSet
		snoozed bool
		pinned  bool
	}{
		{legacy: "", mode: OutlinerModeAll},
		{legacy: "unknown", mode: OutlinerModeAll},
		{legacy: "active", mode: OutlinerModeAll, states: active},
		{legacy: "waiting", mode: OutlinerModeAll, states: mustTestStateSet(t, models.StateWaiting)},
		{legacy: "standby", mode: OutlinerModeAll, states: mustTestStateSet(t, models.StatePaused)},
		{legacy: "someday", mode: OutlinerModeAll, states: mustTestStateSet(t, models.StateSomeday)},
		{legacy: "review", mode: OutlinerModeAll, states: mustTestStateSet(t, models.StateReview)},
		{legacy: "snoozed", mode: OutlinerModeAll, snoozed: true},
		{legacy: "completed", mode: OutlinerModeAll, states: terminal},
		{legacy: "due soon", mode: OutlinerModeDue},
		{legacy: "high_priority", mode: OutlinerModePriority},
		{legacy: "pinned", mode: OutlinerModeAll, pinned: true},
		{legacy: "recently-visited", mode: OutlinerModeActivity},
	}
	for _, tc := range tests {
		t.Run(tc.legacy, func(t *testing.T) {
			got := NormalizeOutlinerRouteState(OutlinerRouteState{Mode: tc.legacy})
			if got.Mode != tc.mode || !got.FilterStates.Equal(tc.states) ||
				got.FilterSnoozed != tc.snoozed || got.FilterPinned != tc.pinned {
				t.Fatalf("normalize %q = %+v states=%v, want mode=%q states=%v snoozed=%v pinned=%v",
					tc.legacy, got, got.FilterStates.Values(), tc.mode, tc.states.Values(), tc.snoozed, tc.pinned)
			}
		})
	}
}

func TestMachineRouteMigrationCanonicalFieldsWin(t *testing.T) {
	payload := []byte(`{
		"tui_state_version": 3,
		"startup_route": {"mode": "unknown", "filter_pinned": true},
		"start_view": "priority",
		"current_route": {"mode": "activity"},
		"cross_outliner_filter_sections": ["Waiting"],
		"project_routes": {"project-1": {"mode": "due"}},
		"project_view_states": {
			"project-1": {"outliner_active_perspective": 2},
			"project-2": {"outliner_active_perspective": 6}
		}
	}`)
	var machine MachineState
	if err := json.Unmarshal(payload, &machine); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if machine.StartupRoute.Mode != OutlinerModeAll || !machine.StartupRoute.FilterPinned {
		t.Fatalf("canonical startup route lost precedence: %+v", machine.StartupRoute)
	}
	if machine.CurrentRoute.Mode != OutlinerModeActivity || !machine.CurrentRoute.FilterStates.IsEmpty() {
		t.Fatalf("canonical current route lost precedence: %+v", machine.CurrentRoute)
	}
	if len(machine.ProjectRoutes) != 1 || machine.ProjectRoutes["project-1"].Mode != OutlinerModeDue {
		t.Fatalf("canonical project routes lost precedence: %+v", machine.ProjectRoutes)
	}
	changed, err := machine.MigrateTUIState()
	if err != nil || !changed {
		t.Fatalf("MigrateTUIState changed=%v err=%v", changed, err)
	}
	data, err := json.Marshal(machine)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, legacy := range []string{"start_view", "start_route", "project_view_states", "cross_outliner_filter_sections"} {
		if bytes.Contains(data, []byte(legacy)) {
			t.Fatalf("canonical JSON retained %q: %s", legacy, data)
		}
	}
}

func TestMachineRouteMigrationDecodesCurrentPerspective(t *testing.T) {
	payload := []byte(`{
		"tui_state_version": 3,
		"outliner_active_perspective": 2
	}`)
	var machine MachineState
	if err := json.Unmarshal(payload, &machine); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := mustTestStateSet(t, models.StateReady, models.StateInProgress)
	if machine.CurrentRoute.Mode != OutlinerModeAll || !machine.CurrentRoute.FilterStates.Equal(want) {
		t.Fatalf("current route = %+v states=%v, want All + %v",
			machine.CurrentRoute, machine.CurrentRoute.FilterStates.Values(), want.Values())
	}
}

func mustTestStateSet(t *testing.T, states ...models.WorkflowState) models.WorkflowStateSet {
	t.Helper()
	set, err := models.NewWorkflowStateSet(states...)
	if err != nil {
		t.Fatalf("state set: %v", err)
	}
	return set
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
		"saved_views": []map[string]any{{
			"name": "Bugs", "filter_type": "bug", "sort_field": 2,
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
	if runtime.Machine().TUIStateVersion != CurrentTUIStateVersion {
		t.Fatalf("machine TUI state version = %d, want %d", runtime.Machine().TUIStateVersion, CurrentTUIStateVersion)
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
