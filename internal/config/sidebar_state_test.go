package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMachineProjectSidebarEnabledDefaultsAndRoundTrips(t *testing.T) {
	machine := NewMachineState()
	if !machine.GetProjectSidebarEnabled("project") {
		t.Fatal("unset project sidebar should default to enabled")
	}

	machine.SetProjectSidebarEnabled("project", false)
	if machine.GetProjectSidebarEnabled("project") {
		t.Fatal("stored project sidebar state = enabled, want disabled")
	}

	data, err := json.Marshal(machine)
	if err != nil {
		t.Fatalf("marshal machine: %v", err)
	}
	if !strings.Contains(string(data), `"project_sidebar_enabled":{"project":false}`) {
		t.Fatalf("machine JSON missing sidebar preference: %s", data)
	}

	var reloaded MachineState
	if err := json.Unmarshal(data, &reloaded); err != nil {
		t.Fatalf("unmarshal machine: %v", err)
	}
	if reloaded.GetProjectSidebarEnabled("project") {
		t.Fatal("reloaded project sidebar state = enabled, want disabled")
	}
	if !reloaded.GetProjectSidebarEnabled("unconfigured") {
		t.Fatal("unconfigured reloaded project sidebar should default to enabled")
	}
}

func TestMachineProjectSidebarEnabledImportsLegacyViewState(t *testing.T) {
	payload := []byte(`{
		"tui_state_version": 2,
		"project_view_states": {
			"disabled": {"sidebar_enabled": false},
			"enabled": {"sidebar_enabled": true},
			"missing": {"filter_type": "task"}
		}
	}`)

	var machine MachineState
	if err := json.Unmarshal(payload, &machine); err != nil {
		t.Fatalf("unmarshal legacy machine: %v", err)
	}
	if machine.GetProjectSidebarEnabled("disabled") {
		t.Fatal("legacy disabled sidebar was not imported")
	}
	if !machine.GetProjectSidebarEnabled("enabled") {
		t.Fatal("legacy enabled sidebar was not imported")
	}
	if _, ok := machine.ProjectSidebarEnabled["missing"]; ok {
		t.Fatal("legacy view state without sidebar_enabled should not create a preference")
	}

	currentPayload := []byte(`{
		"tui_state_version": 2,
		"project_sidebar_enabled": {"project": true},
		"project_view_states": {"project": {"sidebar_enabled": false}}
	}`)
	if err := json.Unmarshal(currentPayload, &machine); err != nil {
		t.Fatalf("unmarshal current machine: %v", err)
	}
	if !machine.GetProjectSidebarEnabled("project") {
		t.Fatal("current sidebar preference should take precedence over legacy view state")
	}
}

func TestProjectSidebarPreferenceFollowsProjectReferenceLifecycle(t *testing.T) {
	domains := NewDomains()
	domains.Machine.SetProjectSidebarEnabled("project-slug", false)
	runtime := NewRuntimeState("", domains)

	refs := runtime.ExtractProjectReferences("project-id", "project-slug")
	if refs.SidebarEnabled == nil || *refs.SidebarEnabled {
		t.Fatalf("extracted sidebar preference = %v, want false", refs.SidebarEnabled)
	}
	if !domains.Machine.RemoveProjectReferences("project-id", "project-slug") {
		t.Fatal("removing sidebar preference should report a change")
	}
	if _, ok := domains.Machine.ProjectSidebarEnabled["project-slug"]; ok {
		t.Fatal("project sidebar preference survived reference removal")
	}
	if !domains.Machine.ApplyProjectReferences("restored-id", refs) {
		t.Fatal("restoring sidebar preference should report a machine-state change")
	}
	if domains.Machine.GetProjectSidebarEnabled("restored-id") {
		t.Fatal("restored project sidebar state = enabled, want disabled")
	}
}
