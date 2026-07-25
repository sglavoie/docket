package tui

import (
	"os"
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

func TestStartupMachineReferenceReconciliationPreservesStaleCustomContextMembers(t *testing.T) {
	project := newSession4Project(t, "Startup Repair")
	fixture := newSession4LifecycleFixture(t, true, project)

	machine := cloneMachineState(fixture.app.runtime.Machine())
	machine.Editor = "vim"
	machine.CurrentProject = "missing-project"
	machine.SetCustomContext("Work", []string{project.ID, "missing-project"})
	machine.SetRecentTaskVisits([]config.RecentTaskVisit{{
		ProjectID: "missing-project",
		TaskID:    "missing-task",
	}})
	if err := fixture.app.runtime.CommitMachine(machine); err != nil {
		t.Fatalf("seed stale machine references: %v", err)
	}

	restarted, err := NewAppModel(fixture.app.runtime, fixture.manager)
	if err != nil {
		t.Fatalf("restart app: %v", err)
	}
	got := restarted.runtime.Machine()
	if got.CurrentProject != "" {
		t.Fatalf("CurrentProject = %q, want stale project cleared", got.CurrentProject)
	}
	wantRefs := []string{project.ID, "missing-project"}
	if refs := got.GetCustomContext("Work"); !slices.Equal(refs, wantRefs) {
		t.Fatalf("custom context refs = %v, want stale membership preserved as %v", refs, wantRefs)
	}
	if len(got.RecentTaskVisits) != 0 {
		t.Fatalf("recent visits retained deleted project: %+v", got.RecentTaskVisits)
	}
	if got.Editor != "vim" {
		t.Fatalf("startup repair clobbered editor = %q", got.Editor)
	}

	persisted, err := config.LoadRuntimeState(fixture.dataDir)
	if err != nil {
		t.Fatalf("reload persisted runtime: %v", err)
	}
	if persisted.Machine().CurrentProject != "" ||
		!slices.Equal(persisted.Machine().GetCustomContext("Work"), wantRefs) {
		t.Fatalf("startup repair was not persisted: %+v", persisted.Machine())
	}
}

func TestStartupMachineReferenceReconciliationKeepsArchivedContextMembership(t *testing.T) {
	project := newSession4Project(t, "Archived Startup Repair")
	project.Archive()
	fixture := newSession4LifecycleFixture(t, true, project)

	machine := cloneMachineState(fixture.app.runtime.Machine())
	machine.CurrentProject = project.Slug
	machine.SetCustomContext("Archive", []string{project.ID})
	if err := fixture.app.runtime.CommitMachine(machine); err != nil {
		t.Fatalf("seed archived machine references: %v", err)
	}

	restarted, err := NewAppModel(fixture.app.runtime, fixture.manager)
	if err != nil {
		t.Fatalf("restart app: %v", err)
	}
	got := restarted.runtime.Machine()
	if got.CurrentProject != "" {
		t.Fatalf("archived CurrentProject = %q, want cleared", got.CurrentProject)
	}
	if refs := got.GetCustomContext("Archive"); !slices.Equal(refs, []string{project.ID}) {
		t.Fatalf("archived custom-context membership = %v, want retained", refs)
	}
}

func TestStartupMachineReferenceReconciliationQueuesBlockedCommit(t *testing.T) {
	project := newSession4Project(t, "Blocked Startup Repair")
	fixture := newSession4LifecycleFixture(t, true, project)

	machine := cloneMachineState(fixture.app.runtime.Machine())
	machine.CurrentProject = "missing-project"
	if err := fixture.app.runtime.CommitMachine(machine); err != nil {
		t.Fatalf("seed stale current project: %v", err)
	}
	machineLock := blockTrashRestoreMachineCommit(t, fixture.app)

	restarted, err := NewAppModel(fixture.app.runtime, fixture.manager)
	if err != nil {
		t.Fatalf("restart app with blocked machine commit: %v", err)
	}
	if restarted.pendingMachineDomainRepair.empty() {
		t.Fatal("blocked startup repair was not retained")
	}
	if restarted.machineDomainRepairRetryScheduled {
		t.Fatal("startup repair was marked scheduled before Init")
	}
	_ = restarted.Init()
	if !restarted.machineDomainRepairRetryScheduled {
		t.Fatal("Init did not schedule retained startup repair")
	}

	if err := os.RemoveAll(machineLock); err != nil {
		t.Fatalf("repair machine lock: %v", err)
	}
	if cmd := restarted.retryMachineDomainRepair(); cmd != nil {
		t.Fatal("successful startup retry returned another schedule")
	}
	if restarted.runtime.Machine().CurrentProject != "" {
		t.Fatalf("startup retry retained stale current project %q", restarted.runtime.Machine().CurrentProject)
	}
	if !restarted.pendingMachineDomainRepair.empty() || restarted.machineDomainRepairRetryScheduled {
		t.Fatal("successful startup retry retained pending state")
	}
}

func TestStartupRecentVisitReconciliationQueuesBlockedTaskRepair(t *testing.T) {
	project := newSession4Project(t, "Blocked Startup Task Repair")
	fixture := newSession4LifecycleFixture(t, true, project)

	machine := cloneMachineState(fixture.app.runtime.Machine())
	machine.SetRecentTaskVisits([]config.RecentTaskVisit{{
		ProjectID: project.ID,
		TaskID:    "missing-task",
	}})
	if err := fixture.app.runtime.CommitMachine(machine); err != nil {
		t.Fatalf("seed stale task visit: %v", err)
	}
	machineLock := blockTrashRestoreMachineCommit(t, fixture.app)

	restarted, err := NewAppModel(fixture.app.runtime, fixture.manager)
	if err != nil {
		t.Fatalf("restart app with blocked machine commit: %v", err)
	}
	if len(restarted.pendingMachineDomainRepair.removeTaskLocators) != 1 {
		t.Fatalf(
			"pending task repairs = %v, want one stale task locator",
			restarted.pendingMachineDomainRepair.removeTaskLocators,
		)
	}
	_ = restarted.Init()
	if !restarted.machineDomainRepairRetryScheduled {
		t.Fatal("Init did not schedule retained task repair")
	}

	if err := os.RemoveAll(machineLock); err != nil {
		t.Fatalf("repair machine lock: %v", err)
	}
	if cmd := restarted.retryMachineDomainRepair(); cmd != nil {
		t.Fatal("successful task retry returned another schedule")
	}
	if visits := restarted.runtime.Machine().RecentTaskVisits; len(visits) != 0 {
		t.Fatalf("startup task retry retained stale visits: %+v", visits)
	}
	if !restarted.pendingMachineDomainRepair.empty() || restarted.machineDomainRepairRetryScheduled {
		t.Fatal("successful task retry retained pending state")
	}
}

func TestMachineDomainRepairRevalidatesRestoredReferences(t *testing.T) {
	project := newSession4Project(t, "Restored Before Retry")
	fixture := newSession4LifecycleFixture(t, true, project)
	app := fixture.app
	taskID := project.Tasks[0].ID

	machine := cloneMachineState(app.runtime.Machine())
	machine.CurrentProject = project.ID
	machine.SetCustomContext("Restored", []string{project.ID})
	machine.SetRecentTaskVisits([]config.RecentTaskVisit{{
		ProjectID: project.ID,
		TaskID:    taskID,
	}})
	if err := app.runtime.CommitMachine(machine); err != nil {
		t.Fatalf("seed restored references: %v", err)
	}

	// These patches represent cleanup queued before the project/task was
	// restored. The current catalog and active task storage are authoritative.
	app.pendingMachineDomainRepair.merge(machineRepairForRemovedProject(project.ID, project.Slug))
	app.pendingMachineDomainRepair.merge(machineRepairForArchivedProject(
		project.ID,
		project.Slug,
		[]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: taskID}},
	))

	if cmd := app.retryMachineDomainRepair(); cmd != nil {
		t.Fatal("revalidated repair returned another schedule")
	}
	got := app.runtime.Machine()
	if got.CurrentProject != project.ID {
		t.Fatalf("restored current project was removed: %q", got.CurrentProject)
	}
	if refs := got.GetCustomContext("Restored"); !slices.Equal(refs, []string{project.ID}) {
		t.Fatalf("restored custom-context refs were removed: %v", refs)
	}
	if len(got.RecentTaskVisits) != 1 || got.RecentTaskVisits[0].TaskID != taskID {
		t.Fatalf("restored task visit was removed: %+v", got.RecentTaskVisits)
	}
	if !app.pendingMachineDomainRepair.empty() {
		t.Fatal("revalidated no-op repair remained pending")
	}
}
