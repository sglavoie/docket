package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// That every group answers a theme role is theme's own invariant and is
// asserted there, by TestWorkflowStateGroupThemeSemanticsCoverEveryGroup. What
// belongs here is the grouping the Outliner builds from those same states.
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
