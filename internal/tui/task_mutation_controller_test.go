package tui

import (
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/listsurface"
)

func taskMutationTestRefs() []TaskProjectRef {
	project := &models.Project{ID: "proj", Slug: "sonar", Name: "Sonar"}
	task := &models.Task{ID: "task", ShortID: 4, Title: "Rebuild the index"}
	project.Tasks = []*models.Task{task}
	return []TaskProjectRef{{Task: task, Project: project}}
}

func TestTaskMutationControllerDeleteKeyIsModalWhileArmed(t *testing.T) {
	t.Parallel()

	refs := taskMutationTestRefs()

	// A confirmed prompt keeps its roots: the dialog closes, but the transaction
	// has not read them yet and does so through takeDelete. Only a cancellation
	// drops them, and an unanswered key changes nothing.
	cases := []struct {
		name          string
		key           string
		want          chrome.ConfirmAnswer
		stillArmed    bool
		rootsRetained bool
	}{
		{"confirm", "y", chrome.ConfirmAnswerConfirmed, false, true},
		{"cancel", "n", chrome.ConfirmAnswerCanceled, false, false},
		{"escape", "esc", chrome.ConfirmAnswerCanceled, false, false},
		{"unrelated key is swallowed", "j", chrome.ConfirmAnswerSwallowed, true, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			var controller taskMutationController
			impact := calculateTaskDeleteImpact(refs, listsurface.SelectionCounts{Total: 1, Visible: 1})
			controller.armDelete(refs, refs, impact)
			if !controller.deleteConfirm.IsActive() {
				t.Fatal("armDelete leaves the prompt armed")
			}
			if got := controller.resolveDeleteKey(testCase.key); got != testCase.want {
				t.Fatalf("resolveDeleteKey(%q) = %d, want %d", testCase.key, got, testCase.want)
			}
			if got := controller.deleteConfirm.IsActive(); got != testCase.stillArmed {
				t.Fatalf("armed after %q = %v, want %v", testCase.key, got, testCase.stillArmed)
			}
			if got := len(controller.pendingDelete.roots) != 0; got != testCase.rootsRetained {
				t.Fatalf("roots retained after %q = %v, want %v", testCase.key, got, testCase.rootsRetained)
			}
		})
	}
}

func TestTaskMutationControllerUnarmedDeleteConsumesNothing(t *testing.T) {
	t.Parallel()

	var controller taskMutationController
	if got := controller.resolveDeleteKey("y"); got != chrome.ConfirmAnswerUnarmed {
		t.Fatalf("resolveDeleteKey on an unarmed prompt = %d, want %d", got, chrome.ConfirmAnswerUnarmed)
	}
	if got := controller.resolveOutlineKey("y"); got != chrome.ConfirmAnswerUnarmed {
		t.Fatalf("resolveOutlineKey on an unarmed prompt = %d, want %d", got, chrome.ConfirmAnswerUnarmed)
	}
}

func TestTaskMutationControllerTakeDeleteDisarms(t *testing.T) {
	t.Parallel()

	refs := taskMutationTestRefs()
	var controller taskMutationController
	controller.armDelete(refs, refs, taskDeleteImpact{selectedRoots: 1, totalTasks: 1})

	pending := controller.takeDelete()
	if len(pending.roots) != 1 || len(pending.selected) != 1 {
		t.Fatalf("takeDelete() = %+v, want the armed roots and selection", pending)
	}
	if controller.deleteConfirm.IsActive() || len(controller.pendingDelete.roots) != 0 {
		t.Fatal("takeDelete disarms, so a dispatched transaction cannot be prompted for twice")
	}
	if second := controller.takeDelete(); len(second.roots) != 0 {
		t.Fatalf("a second takeDelete() = %+v, want empty", second)
	}
}

// TestTaskMutationControllerDeclinedOutlineStillNamesItsBuffer pins why the
// outline cancellation does not clear inside resolveOutlineKey: the status it
// owes the user names the path the pending value carries.
func TestTaskMutationControllerDeclinedOutlineStillNamesItsBuffer(t *testing.T) {
	t.Parallel()

	var controller taskMutationController
	project := &models.Project{ID: "proj", Name: "Sonar"}
	deleted := &models.Task{ID: "task", ShortID: 4, Title: "Rebuild the index"}
	controller.armOutlineApply(outlineApplyPending{
		project:   project,
		revision:  "rev-1",
		path:      "/tmp/outline-1234.md",
		deletions: []*models.Task{deleted},
	})
	if got, want := controller.outlineConfirm.Prompt(), "Delete task #4 from Sonar?"; got != want {
		t.Fatalf("prompt = %q, want %q", got, want)
	}

	if got := controller.resolveOutlineKey("esc"); got != chrome.ConfirmAnswerCanceled {
		t.Fatalf("resolveOutlineKey(esc) = %d, want %d", got, chrome.ConfirmAnswerCanceled)
	}
	pending := controller.takeOutlineApply()
	if pending.path != "/tmp/outline-1234.md" {
		t.Fatalf("declined apply path = %q, want the buffer it was parsed from", pending.path)
	}
	if controller.outlineConfirm.IsActive() || controller.pendingOutline.project != nil {
		t.Fatal("takeOutlineApply disarms the prompt and drops the buffer")
	}
}

func TestTaskMutationControllerConfirmOverlayAnswersForTheArmedPrompt(t *testing.T) {
	t.Parallel()

	var controller taskMutationController
	if _, armed := controller.confirmOverlay(120); armed {
		t.Fatal("nothing armed, so the cluster renders no overlay")
	}

	controller.armOutlineApply(outlineApplyPending{
		project:   &models.Project{ID: "proj", Name: "Sonar"},
		deletions: []*models.Task{{ID: "task", ShortID: 4, Title: "Rebuild the index"}},
	})
	overlay, armed := controller.confirmOverlay(120)
	if !armed || overlay == "" {
		t.Fatal("an armed outline confirmation renders its overlay")
	}
}

// TestTaskMutationClusterHasOneHome pins the cluster to its single named
// AppModel field. Both pendings and both dialogs are the state a destructive
// task edit waits in; a second home on the god struct is how a prompt and the
// transaction it gates would come to read different values.
func TestTaskMutationClusterHasOneHome(t *testing.T) {
	t.Parallel()

	clustered := map[string]bool{
		"pendingTaskDelete":    true,
		"taskDeleteConfirm":    true,
		"pendingOutlineApply":  true,
		"outlineDeleteConfirm": true,
	}
	assertAppModelClusterHasOneHome(t, "taskMutation", clustered)
}
