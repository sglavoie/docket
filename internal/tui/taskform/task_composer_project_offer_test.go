package taskform

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/coordinator"
)

// newProjectOfferComposer opens a global composer over a workspace on disk
// holding inbox and an archived project named shelf, with a stub mutator that
// records the capture it is handed.
func newProjectOfferComposer(t *testing.T) (*TaskComposerModel, *stubMutator) {
	t.Helper()
	inbox := &models.Project{ID: "inbox-id", Name: "inbox", Slug: "inbox"}
	shelf := &models.Project{ID: "shelf-id", Name: "shelf", Slug: "shelf", Archived: true}
	base := newTestConfig()
	catalog := config.NewWorkspaceCatalog()
	for _, project := range []*models.Project{inbox, shelf} {
		if err := catalog.UpsertProject(project); err != nil {
			t.Fatalf("UpsertProject: %v", err)
		}
	}
	base.SetCatalogInMemory(catalog)
	runtime := testRuntimeAt(t.TempDir(), base)

	stub := &stubMutator{result: app.TaskResult{ProjectID: "created", TaskID: "task", TransactionID: "txn"}}
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:               ComposerGlobal,
		Project:            inbox,
		AllProjects:        []*models.Project{inbox},
		AllowProjectSwitch: true,
	}, runtime)
	composer.SetCaptureCoordinator(coordinator.NewFeatureCoordinators(stub).Capture)
	composer.SetSize(80, 24)
	return composer, stub
}

func pressComposerKey(composer *TaskComposerModel, key tea.KeyPressMsg) tea.Cmd {
	_, cmd := composer.Update(key)
	return cmd
}

// A refusal naming only an unknown canonical slug asks to create it; Yes
// commits the capture with the project, and the draft is never rewritten.
func TestComposerOffersToCreateAnUnknownProject(t *testing.T) {
	composer, stub := newProjectOfferComposer(t)
	composer.Input.SetValue("Plant beans %garden\nWater beans %garden\nBuy pots %shed")

	if cmd := pressComposerKey(composer, syntheticSpecialKey(tea.KeyEnter)); cmd != nil {
		t.Fatal("a refused submit dispatched a capture before the confirmation")
	}
	dialog := composer.GetConfirmDialog()
	if dialog == nil || !dialog.IsActive() || dialog.Prompt() != "Create projects garden, shed and capture?" {
		t.Fatalf("dialog = %+v, want the creation offer", dialog)
	}
	if len(composer.Errors) != 3 {
		t.Fatalf("the refusal was not kept under the offer: %+v", composer.Errors)
	}

	cmd := pressComposerKey(composer, syntheticKey('y'))
	if cmd == nil {
		t.Fatal("Yes dispatched nothing")
	}
	msg, ok := cmd().(coordinator.TaskCreatedMsg)
	if !ok {
		t.Fatalf("Yes returned %T, want TaskCreatedMsg", cmd())
	}
	if !msg.ProjectsCreated {
		t.Fatal("the created message does not ask the shell to refresh the catalog")
	}
	if len(stub.creation.Projects) != 2 || stub.creation.Projects[0].Slug != "garden" || stub.creation.Projects[1].Slug != "shed" {
		t.Fatalf("created projects = %+v, want garden and shed", stub.creation.Projects)
	}
	if len(stub.captured) != 3 || stub.captured[0].ProjectID != stub.creation.Projects[0].ID ||
		stub.captured[2].ProjectID != stub.creation.Projects[1].ID {
		t.Fatalf("captured requests do not route to the created projects: %+v", stub.captured)
	}
	if got := composer.Input.Value(); got != "Plant beans %garden\nWater beans %garden\nBuy pots %shed" {
		t.Fatalf("the draft was rewritten: %q", got)
	}
}

// No and Esc put the question down and keep the draft and its refusal.
func TestComposerProjectOfferNoKeepsTheRefusal(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{syntheticKey('n'), syntheticSpecialKey(tea.KeyEscape)} {
		composer, stub := newProjectOfferComposer(t)
		composer.Input.SetValue("Plant beans %garden")
		pressComposerKey(composer, syntheticSpecialKey(tea.KeyEnter))
		if cmd := pressComposerKey(composer, key); cmd != nil {
			t.Fatalf("%s dispatched a command", key.String())
		}
		if composer.GetConfirmDialog().IsActive() {
			t.Fatalf("%s left the question open", key.String())
		}
		if len(composer.Errors) != 1 || composer.Errors[0].UnknownProject != "garden" {
			t.Fatalf("%s lost the refusal: %+v", key.String(), composer.Errors)
		}
		if composer.Input.Value() != "Plant beans %garden" || stub.calls != 0 {
			t.Fatalf("%s changed the draft or captured: %q, %d calls", key.String(), composer.Input.Value(), stub.calls)
		}
	}
}

// Ctrl+N and Ctrl+E take the same path; Ctrl+N keeps the composer open and
// then routes to the project it just created.
func TestComposerProjectOfferFollowsCtrlNAndCtrlE(t *testing.T) {
	composer, _ := newProjectOfferComposer(t)
	composer.Input.SetValue("Plant beans %garden")
	pressComposerKey(composer, syntheticCtrlKey('e'))
	cmd := pressComposerKey(composer, syntheticKey('y'))
	if msg, ok := cmd().(coordinator.TaskCreatedMsg); !ok || !msg.OpenContent {
		t.Fatalf("Ctrl+E then Yes = %+v, want a created message opening the editor", msg)
	}

	composer, _ = newProjectOfferComposer(t)
	composer.Input.SetValue("Plant beans %garden")
	pressComposerKey(composer, syntheticCtrlKey('n'))
	cmd = pressComposerKey(composer, syntheticKey('y'))
	msg, ok := cmd().(coordinator.TaskCreatedMsg)
	if !ok || !msg.KeepComposerOpen {
		t.Fatalf("Ctrl+N then Yes = %+v, want a created message keeping the composer", msg)
	}
	composer.Update(msg)
	composer.Input.SetValue("Water beans %garden")
	if _, ok := composer.ParseCompactLines(); !ok {
		t.Fatalf("the open composer does not route to the project it created: %+v", composer.Errors)
	}
}

// Any refusal the offer cannot answer whole refuses exactly as before: a mixed
// draft, a non-canonical %Work, and a slug an archived project already holds.
func TestComposerProjectOfferOnlyWhenEveryProblemQualifies(t *testing.T) {
	for _, draft := range []string{
		"Plant beans %garden\nWater beans $ready",
		"Plant beans %garden $ready",
		"Plant beans %Garden",
		"Dust books %shelf",
	} {
		composer, stub := newProjectOfferComposer(t)
		composer.Input.SetValue(draft)
		if cmd := pressComposerKey(composer, syntheticSpecialKey(tea.KeyEnter)); cmd != nil {
			t.Fatalf("%q: a refused submit dispatched a command", draft)
		}
		if composer.GetConfirmDialog().IsActive() {
			t.Fatalf("%q: offered %q", draft, composer.GetConfirmDialog().Prompt())
		}
		if len(composer.Errors) == 0 || stub.calls != 0 {
			t.Fatalf("%q: errors %+v, calls %d", draft, composer.Errors, stub.calls)
		}
	}
}
