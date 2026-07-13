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
			markParkPrompted(taskID)
			_ = wasParkPrompted(taskID)
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
	if wasParkPrompted("task-1") {
		t.Fatal("prompted tasks should be cleared after reset")
	}
}
