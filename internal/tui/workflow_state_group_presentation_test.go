package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestWorkflowStateGroupThemeSemanticsCoverEveryGroup(t *testing.T) {
	groups := append(models.AllModeWorkflowStateGroups(), models.StateGroupDone, models.StateGroupCanceled)
	for _, group := range groups {
		label := group.Label()
		if got := theme.GetWorkflowStateGroupStyle(group).Render(label); got == "" {
			t.Errorf("group %q rendered an empty header", group)
		}
	}
}

func TestCanonicalAllGroupingUsesExactStateGroups(t *testing.T) {
	backlog, _ := models.NewGenericTask("backlog", 1, "Backlog")
	doing, _ := models.NewGenericTask("doing", 2, "Doing")
	inbox, _ := models.NewGenericTask("inbox", 3, "Inbox")
	doing.ApplyWorkflowState(models.StateDoing)
	inbox.ApplyWorkflowState(models.StateInbox)

	forests := buildStateGroupTaskForests([]*models.Task{backlog, doing, inbox})
	if got := forests[models.StateGroupBacklog]; got == nil || got.count != 1 {
		t.Fatalf("Backlog state group = %#v, want one task", got)
	}
	if got := forests[models.StateGroupInbox]; got == nil || got.count != 1 {
		t.Fatalf("Inbox state group = %#v, want one task", got)
	}
	if got := forests[models.StateGroupDoing]; got == nil || got.count != 1 {
		t.Fatalf("Doing state group = %#v, want one task", got)
	}
}
