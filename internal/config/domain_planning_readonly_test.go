package config

import (
	"os"
	"reflect"
	"testing"
)

func TestPlanningReadOnlyNeverRecoversCorruptPrimary(t *testing.T) {
	store := newIsolatedStore(t)
	root := store.DataDir()
	planning := &PlanningState{Today: []ProjectTaskLocator{{ProjectID: "project", TaskID: "task"}}}
	if err := store.SavePlanning(planning); err != nil {
		t.Fatal(err)
	}
	got, err := NewDomainFileStore(root).LoadPlanningReadOnly()
	if err != nil || !reflect.DeepEqual(got, planning) {
		t.Fatalf("read = %+v, %v; want %+v", got, err, planning)
	}
	// Save again to retain a valid backup. Read-only preflight must leave a
	// corrupt primary alone even when the ordinary loader could repair it.
	if err := store.SavePlanning(planning); err != nil {
		t.Fatal(err)
	}
	path := PlanningStatePath(root)
	const corrupt = "{broken"
	if err := os.WriteFile(path, []byte(corrupt), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDomainFileStore(root).LoadPlanningReadOnly(); err == nil {
		t.Fatal("read-only load accepted a corrupt primary")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != corrupt {
		t.Fatalf("read-only load rewrote primary: %q, %v", data, err)
	}
	// Prove the backup really was usable, rather than passing because neither
	// loader could have recovered the workspace.
	if _, err := NewDomainFileStore(root).LoadPlanning(); err != nil {
		t.Fatalf("ordinary load could not recover backup: %v", err)
	}
}
