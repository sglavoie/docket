package tui

import (
	"sync"
	"time"
)

type sessionTaskMutation struct {
	ProjectID   string
	TaskID      string
	LastMutated time.Time
}

var sessionMutationRegistry = struct {
	sync.Mutex
	last     sessionTaskMutation
	hasLast  bool
	prompted map[string]bool
}{
	prompted: make(map[string]bool),
}

func markSessionTaskMutated(projectID string, taskIDs []string) {
	if projectID == "" || len(taskIDs) == 0 {
		return
	}
	now := time.Now()

	sessionMutationRegistry.Lock()
	defer sessionMutationRegistry.Unlock()

	for _, taskID := range taskIDs {
		if taskID == "" {
			continue
		}
		sessionMutationRegistry.last = sessionTaskMutation{
			ProjectID:   projectID,
			TaskID:      taskID,
			LastMutated: now,
		}
		sessionMutationRegistry.hasLast = true
	}
}

func lastSessionMutation() (sessionTaskMutation, bool) {
	sessionMutationRegistry.Lock()
	defer sessionMutationRegistry.Unlock()
	if !sessionMutationRegistry.hasLast {
		return sessionTaskMutation{}, false
	}
	return sessionMutationRegistry.last, true
}

func markParkPrompted(taskID string) {
	if taskID == "" {
		return
	}
	sessionMutationRegistry.Lock()
	defer sessionMutationRegistry.Unlock()
	sessionMutationRegistry.prompted[taskID] = true
}

func wasParkPrompted(taskID string) bool {
	if taskID == "" {
		return false
	}
	sessionMutationRegistry.Lock()
	defer sessionMutationRegistry.Unlock()
	return sessionMutationRegistry.prompted[taskID]
}

func resetSessionMutations() {
	sessionMutationRegistry.Lock()
	defer sessionMutationRegistry.Unlock()
	sessionMutationRegistry.last = sessionTaskMutation{}
	sessionMutationRegistry.hasLast = false
	sessionMutationRegistry.prompted = make(map[string]bool)
}
