package config

import (
	"os"
	"testing"
)

func TestRuntimeState_LoadCapturesDomains(t *testing.T) {
	manager, _ := seededManager(t)
	runtimeState, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	if runtimeState.Catalog() == nil || runtimeState.Planning() == nil || runtimeState.Machine() == nil {
		t.Fatal("every lean domain accessor must return a non-nil snapshot")
	}
	if runtimeState.DataDir() == "" {
		t.Fatal("DataDir must be populated from the workspace root")
	}
}

func TestRuntimeState_CommitsRefreshSnapshot(t *testing.T) {
	manager, _ := seededManager(t)
	runtimeState, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	planning := runtimeState.Planning()
	planning.SetToday([]ProjectTaskLocator{{TaskID: "today", ProjectID: "project"}})
	if err := runtimeState.CommitPlanning(planning); err != nil {
		t.Fatalf("CommitPlanning: %v", err)
	}
	machine := runtimeState.Machine()
	machine.Theme = "light"
	if err := runtimeState.CommitMachine(machine); err != nil {
		t.Fatalf("CommitMachine: %v", err)
	}
	if got := runtimeState.Planning().Today; len(got) != 1 || got[0].TaskID != "today" {
		t.Fatalf("planning snapshot did not advance: %+v", got)
	}
	if runtimeState.Machine().Theme != "light" {
		t.Fatalf("machine snapshot did not advance: %+v", runtimeState.Machine())
	}
}

func TestRuntimeState_CommitNilArgsGuarded(t *testing.T) {
	manager, _ := seededManager(t)
	runtimeState, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	if err := runtimeState.CommitCatalog(nil); err == nil {
		t.Error("expected error for nil catalog")
	}
	if err := runtimeState.CommitPlanning(nil); err == nil {
		t.Error("expected error for nil planning")
	}
	if err := runtimeState.CommitMachine(nil); err == nil {
		t.Error("expected error for nil machine")
	}
}

func TestRuntimeState_RefreshAfterCommitObservesWrite(t *testing.T) {
	manager, _ := seededManager(t)
	runtimeState, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	domains, err := manager.LoadDomains()
	if err != nil {
		t.Fatalf("LoadDomains: %v", err)
	}
	domains.Planning.AddToToday("outside", "project")
	if err := saveDomainBundle(manager.domainStore(), domains); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if len(runtimeState.Planning().Today) != 0 {
		t.Fatal("snapshot changed before refresh")
	}
	if err := runtimeState.RefreshAfterCommit(); err != nil {
		t.Fatalf("RefreshAfterCommit: %v", err)
	}
	if got := runtimeState.Planning().Today; len(got) != 1 || got[0].TaskID != "outside" {
		t.Fatalf("refreshed planning = %+v", got)
	}
}

func TestEmptyRuntimeState_AccessorsSafe(t *testing.T) {
	runtimeState := EmptyRuntimeState(t.TempDir())
	if runtimeState.Catalog() == nil || runtimeState.Planning() == nil || runtimeState.Machine() == nil {
		t.Fatal("empty runtime state must default every lean domain")
	}
	if runtimeState.ActiveProjects() != nil {
		t.Fatal("empty catalog should yield no active projects")
	}
}

func TestRuntimeState_CloneDomainsIsIndependent(t *testing.T) {
	manager, _ := seededManager(t)
	runtimeState, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	cloned, err := runtimeState.CloneDomains()
	if err != nil {
		t.Fatalf("CloneDomains: %v", err)
	}
	cloned.Planning.AddToToday("cloned", "project")
	cloned.Machine.Theme = "light"
	if len(runtimeState.Planning().Today) != 0 {
		t.Fatal("planning mutation leaked into runtime snapshot")
	}
	if runtimeState.Machine().Theme != "dark" {
		t.Fatal("machine mutation leaked into runtime snapshot")
	}
}

func TestRuntimeState_CheckedArtifactsKeepStartupRevision(t *testing.T) {
	manager, _ := seededManager(t)
	runtimeState, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	planningPath := PlanningStatePath(runtimeState.DataDir())
	loaded, err := os.ReadFile(planningPath)
	if err != nil {
		t.Fatalf("read loaded planning: %v", err)
	}
	concurrent := []byte(`{"concurrent":true}`)
	if writeErr := os.WriteFile(planningPath, concurrent, FilePermissions); writeErr != nil {
		t.Fatalf("write concurrent planning: %v", writeErr)
	}
	domains, err := runtimeState.CloneDomains()
	if err != nil {
		t.Fatalf("CloneDomains: %v", err)
	}
	artifacts, err := runtimeState.CheckedSyncedArtifacts(domains)
	if err != nil {
		t.Fatalf("CheckedSyncedArtifacts: %v", err)
	}
	for _, artifact := range artifacts {
		if artifact.Path != planningPath {
			continue
		}
		if artifact.ExpectedRevision == nil {
			t.Fatal("planning artifact has no expected revision")
		}
		if got, want := *artifact.ExpectedRevision, revisionOf(loaded); got != want {
			t.Fatalf("expected revision = %q, want startup revision %q", got, want)
		}
		if *artifact.ExpectedRevision == revisionOf(concurrent) {
			t.Fatal("artifact incorrectly adopted concurrent revision")
		}
		return
	}
	t.Fatal("planning artifact not emitted")
}
