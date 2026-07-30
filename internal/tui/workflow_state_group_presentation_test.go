package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestWorkflowStateGroupThemeSemanticsCoverEveryGroup(t *testing.T) {
	for _, group := range models.AllWorkflowStateGroups() {
		if accent := GetWorkflowStateGroupAccent(group); accent == nil {
			t.Errorf("group %q has no theme accent", group)
		}
		label := group.Label()
		if got := GetWorkflowStateGroupStyle(group).Render(label); got == "" {
			t.Errorf("group %q rendered an empty header", group)
		}
	}
}

func TestCanonicalAllGroupingUsesExactStateGroups(t *testing.T) {
	ready, _ := models.NewGenericTask("ready", 1, "Ready")
	started, _ := models.NewGenericTask("started", 2, "Started")
	inbox, _ := models.NewGenericTask("inbox", 3, "Inbox")
	started.ApplyWorkflowState(models.StateInProgress)
	inbox.ApplyWorkflowState(models.StateInbox)

	forests := buildStateGroupTaskForests([]*models.Task{ready, started, inbox})
	if got := forests[models.StateGroupBacklog]; got == nil || got.count != 1 {
		t.Fatalf("Backlog state group = %#v, want one task", got)
	}
	if got := forests[models.StateGroupInbox]; got == nil || got.count != 1 {
		t.Fatalf("Inbox state group = %#v, want one task", got)
	}
	if got := forests[models.StateGroupInProgress]; got == nil || got.count != 1 {
		t.Fatalf("In Progress state group = %#v, want one task", got)
	}
}
