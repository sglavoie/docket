package config

import (
	"slices"
	"testing"
)

func TestSearchHistoryIsMachineAndWorkspaceLocal(t *testing.T) {
	home := t.TempDir()
	dataDir, manager := stageSharedWorkspaceOn(t, home)
	domains, err := manager.LoadDomains()
	if err != nil {
		t.Fatal(err)
	}
	machine := domains.Machine.Clone()
	machine.SearchHistory = []string{"login", "$done auth"}
	if err = manager.CommitMachine(machine); err != nil {
		t.Fatal(err)
	}
	reloaded, err := joinWorkspaceOn(t, dataDir, home).LoadDomains()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(reloaded.Machine.SearchHistory, machine.SearchHistory) {
		t.Fatalf("reloaded history = %v", reloaded.Machine.SearchHistory)
	}
	clone := reloaded.Machine.Clone()
	clone.SearchHistory[0] = "changed"
	if reloaded.Machine.SearchHistory[0] != "login" {
		t.Fatal("clone aliases history")
	}
	otherMachine, err := joinWorkspaceOn(t, dataDir, t.TempDir()).LoadDomains()
	if err != nil {
		t.Fatal(err)
	}
	if len(otherMachine.Machine.SearchHistory) != 0 {
		t.Fatal("history traveled to another machine")
	}
	_, otherManager := stageSharedWorkspaceOn(t, home)
	otherWorkspace, err := otherManager.LoadDomains()
	if err != nil {
		t.Fatal(err)
	}
	if len(otherWorkspace.Machine.SearchHistory) != 0 {
		t.Fatal("history traveled to another workspace")
	}
	machine.SearchHistory = nil
	if err = manager.CommitMachine(machine); err != nil {
		t.Fatal(err)
	}
	cleared, err := manager.LoadDomains()
	if err != nil {
		t.Fatal(err)
	}
	if len(cleared.Machine.SearchHistory) != 0 {
		t.Fatal("clear did not persist")
	}
}

func TestSearchHistoryValidation(t *testing.T) {
	for _, entries := range [][]string{{""}, {" query "}, {"query", "query"}, make([]string, SearchHistoryLimit+1)} {
		machine := NewMachineState()
		machine.SearchHistory = entries
		if err := machine.Validate(); err == nil {
			t.Fatalf("accepted malformed history %v", entries)
		}
	}
}
