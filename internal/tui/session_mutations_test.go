package tui

import (
	"fmt"
	"sync"
	"testing"
)

func TestSessionMutationRegistryConcurrentSafeAndReset(t *testing.T) {
	resetSessionMutations()
	t.Cleanup(resetSessionMutations)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			taskID := fmt.Sprintf("task-%d", i)
			markSessionTaskMutated("project-1", []string{taskID})
			identity := TaskIdentity{ProjectID: "project-1", TaskID: taskID}
			markParkPrompted(identity)
			_ = wasParkPrompted(identity)
		}(i)
	}
	wg.Wait()

	if _, ok := lastSessionMutation(); !ok {
		t.Fatal("lastSessionMutation should report the last recorded mutation")
	}

	resetSessionMutations()
	if _, ok := lastSessionMutation(); ok {
		t.Fatal("lastSessionMutation should be empty after reset")
	}
	if wasParkPrompted(TaskIdentity{ProjectID: "project-1", TaskID: "task-1"}) {
		t.Fatal("prompted tasks should be cleared after reset")
	}
}

func TestParkPromptRegistryKeepsDuplicateTaskIDsIndependent(t *testing.T) {
	resetSessionMutations()
	t.Cleanup(resetSessionMutations)
	alpha := TaskIdentity{ProjectID: "alpha", TaskID: "shared-id"}
	beta := TaskIdentity{ProjectID: "beta", TaskID: "shared-id"}
	markParkPrompted(alpha)
	if !wasParkPrompted(alpha) {
		t.Fatal("alpha prompt was not recorded")
	}
	if wasParkPrompted(beta) {
		t.Fatal("alpha prompt leaked to duplicate beta task")
	}
}
