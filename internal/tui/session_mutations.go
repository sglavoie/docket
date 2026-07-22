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
	prompted map[TaskIdentity]bool
}{
	prompted: make(map[TaskIdentity]bool),
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

func markParkPrompted(identity TaskIdentity) {
	if !identity.Valid() {
		return
	}
	sessionMutationRegistry.Lock()
	defer sessionMutationRegistry.Unlock()
	sessionMutationRegistry.prompted[identity] = true
}

func wasParkPrompted(identity TaskIdentity) bool {
	if !identity.Valid() {
		return false
	}
	sessionMutationRegistry.Lock()
	defer sessionMutationRegistry.Unlock()
	return sessionMutationRegistry.prompted[identity]
}

func resetSessionMutations() {
	sessionMutationRegistry.Lock()
	defer sessionMutationRegistry.Unlock()
	sessionMutationRegistry.last = sessionTaskMutation{}
	sessionMutationRegistry.hasLast = false
	sessionMutationRegistry.prompted = make(map[TaskIdentity]bool)
}
