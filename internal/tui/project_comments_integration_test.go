package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/projecthub"
)

// This file is the integration half of project comments: not whether one
// mutation is correct, but whether the panel and the stored collection survive
// everything else the product does around them.
//
// Two questions are asked here and nowhere else. A draft is unsaved text that
// lives only in the panel, so every refresh the shell can run under it — an
// external write, a rename, an archival, a deletion — is a chance to lose it,
// and losing it is silent. And a project comment shares one tasks.md with the
// whole task list, so every use case that rewrites that document is a chance to
// drop the block: an inline task edit, an outline plan, a cross-project move,
// and a project rename each rewrite it through a different call.

// openProjectCommentsDraft mounts the panel on the fixture project's header and
// leaves an unsaved add-mode draft in it, which is the state every refresh case
// below perturbs.
func openProjectCommentsDraft(t *testing.T, driver *UITestDriver, draft string) {
	t.Helper()
	seatCursorOnProjectHeader(t, driver.App().crossOutliner)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: ";"},
		{Type: UITestActionKey, Key: "a"},
		{Type: UITestActionText, Text: draft},
	})
	assertDraftIntact(t, driver, draft)
}

// assertDraftIntact is the claim every refresh owes: the panel is still the one
// the user was typing into, holding exactly the text they typed.
func assertDraftIntact(t *testing.T, driver *UITestDriver, draft string) {
	t.Helper()
	panel := mountedProjectCommentsPanel(t, driver.App())
	if !panel.AddMode {
		t.Fatalf("the panel is not holding a draft:\n%s", driver.Render())
	}
	if got := panel.TextInput.Value(); got != draft {
		t.Fatalf("draft = %q, want the untouched %q", got, draft)
	}
}

// externalProjectCommentWriter opens a second application service over the same
// workspace directory, which is what another surface — or another machine — is
// from this shell's point of view: a writer whose commits this process learns
// about only by reloading.
func externalProjectCommentWriter(t *testing.T, workspace *uitestWorkspace) *app.Service {
	t.Helper()
	manager := config.NewManager(config.ConfigPathForDataDir(workspace.DataDir))
	if _, err := manager.LoadDomains(); err != nil {
		t.Fatalf("LoadDomains(%q): %v", workspace.DataDir, err)
	}
	runtime, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState(%q): %v", workspace.DataDir, err)
	}
	return newTUIAppService(workspace.DataDir, manager.RuntimeDependencies(), workspaceLedgerFromConfig(runtime))
}

// refreshUnderThePanel runs the one fan-out every committed mutation ends in,
// which is also the only path that can dismiss an open panel. It reports whether
// the shell asked to leave, so a case can tell "still readable" from "the owner
// is gone" without reading the panel's internals.
func refreshUnderThePanel(t *testing.T, driver *UITestDriver) bool {
	t.Helper()
	return driver.App().refreshTaskViews(true, false) != nil
}

// TestProjectCommentsPanelKeepsItsDraftAcrossAnExternalRefresh is the case the
// adapter's identity resolution exists for. Another writer's comment reaches the
// list, and the text the user has not saved is not what pays for it.
func TestProjectCommentsPanelKeepsItsDraftAcrossAnExternalRefresh(t *testing.T) {
	driver, workspace, _ := addCommentFixtureApp(t)
	const draft = "Half-typed, and not ready to save"
	openProjectCommentsDraft(t, driver, draft)

	owner := driver.App().projectByID(projectCommentFixtureSlug)
	if owner == nil {
		t.Fatal("the fixture project is not in the shell cache")
	}
	writer := externalProjectCommentWriter(t, workspace)
	if _, err := writer.AddProjectComment(owner.ID, "Written by another surface"); err != nil {
		t.Fatalf("the external writer could not comment: %v", err)
	}

	driver.App().reloadCrossProjectOutlinerProject(owner.ID)
	if refreshUnderThePanel(t, driver) {
		t.Fatal("an external comment dismissed the panel")
	}

	assertDraftIntact(t, driver, draft)
	if got := panelCommentTexts(mountedProjectCommentsPanel(t, driver.App())); !reflect.DeepEqual(
		got, []string{"Written by another surface"}) {
		t.Fatalf("the refreshed panel lists %v, want the external comment", got)
	}

	// And the draft is still a draft: saving it now appends beside the comment
	// that arrived rather than replacing it.
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})
	stored := projectCommentsOnDisk(t, workspace)
	if len(stored) != 2 || stored[0].Text != "Written by another surface" || stored[1].Text != draft {
		t.Fatalf("stored comments = %+v, want the external one then the draft", stored)
	}
}

// TestProjectCommentsPanelKeepsItsDraftWhenTheOwnerChanges walks the three
// remaining ways the workspace moves under an open draft. A rename and an
// archival both leave the panel readable, so both must leave the draft alone; a
// deletion is the one that ends the panel, and it must still write nothing.
func TestProjectCommentsPanelKeepsItsDraftWhenTheOwnerChanges(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		draft    string
		perturb  func(t *testing.T, shell *AppModel, owner *models.Project)
		leaving  bool
		expects  string
		refusing bool
	}{
		{
			name:    "renamed under the panel",
			draft:   "Typed before the rename",
			perturb: func(_ *testing.T, _ *AppModel, owner *models.Project) { owner.Name = "Renamed Under The Draft" },
			expects: "Renamed Under The Draft",
		},
		{
			name:  "archived under the panel",
			draft: "Typed before the archival",
			// The archive is committed rather than flipped in memory, because the
			// refusal below is the application layer's and it rules on the catalog
			// it loads, not on the flag this shell happens to be holding.
			perturb: func(t *testing.T, shell *AppModel, owner *models.Project) {
				handled, cmd := shell.handleProjectMessages(projecthub.ProjectArchivedMsg{Project: owner})
				if !handled {
					t.Fatal("the shell did not handle the archive")
				}
				runSession2AppCommand(t, shell, cmd)
				if reloaded := shell.projectByID(owner.ID); reloaded == nil || !reloaded.Archived {
					t.Fatalf("the archive did not stick: %+v", reloaded)
				}
			},
			expects:  checklistFixtureProjectName,
			refusing: true,
		},
		{
			name:  "deleted under the panel",
			draft: "Typed before the deletion",
			perturb: func(_ *testing.T, shell *AppModel, owner *models.Project) {
				for id, project := range shell.domain.projects {
					if project != nil && project.ID == owner.ID {
						delete(shell.domain.projects, id)
					}
				}
			},
			leaving: true,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			driver, workspace, _ := addCommentFixtureApp(t)
			openProjectCommentsDraft(t, driver, testCase.draft)

			owner := driver.App().projectByID(projectCommentFixtureSlug)
			if owner == nil {
				t.Fatal("the fixture project is not in the shell cache")
			}
			testCase.perturb(t, driver.App(), owner)

			if leaving := refreshUnderThePanel(t, driver); leaving != testCase.leaving {
				t.Fatalf("the refresh asked to leave = %v, want %v", leaving, testCase.leaving)
			}
			if testCase.leaving {
				// A dismissed panel is the shell's decision, not a write: the draft
				// was never a comment and must not have become one on the way out.
				if got := projectCommentsOnDisk(t, workspace); len(got) != 0 {
					t.Fatalf("a dismissed draft still stored %+v", got)
				}
				return
			}

			assertDraftIntact(t, driver, testCase.draft)
			if !strings.Contains(driver.Render(), testCase.expects) {
				t.Fatalf("the panel does not name %q:\n%s", testCase.expects, driver.Render())
			}
			if !testCase.refusing {
				return
			}

			// An archived owner stays readable and refuses the write, and the
			// refusal is what must not cost the draft either.
			runChecklistActions(t, driver, []UITestAction{
				{Type: UITestActionKey, Key: "ctrl+s"},
				{Type: UITestActionWait},
			})
			if got := projectCommentsOnDisk(t, workspace); len(got) != 0 {
				t.Fatalf("an archived owner still stored %+v", got)
			}
			assertDraftIntact(t, driver, testCase.draft)
		})
	}
}

// projectCommentTextsOnDisk reads one named project's stored comments, so a run
// that renames a project or moves a task out of one can read either document.
func projectCommentTextsOnDisk(t *testing.T, workspace *uitestWorkspace, slug string) []string {
	t.Helper()
	path := filepath.Join(workspace.DataDir, "projects", slug, "tasks.md")
	content, err := os.ReadFile(path) //nolint:gosec // The path is this test's own workspace.
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	document, err := storage.ParseProjectDocument(string(content))
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	texts := make([]string, 0, len(document.Comments))
	for _, comment := range document.Comments {
		if comment != nil {
			texts = append(texts, comment.Text)
		}
	}
	return texts
}

// selectProjectPickerRow walks the open value picker onto the row the caller
// names, which is how the `%` move chooses a destination without depending on
// the fixture's project order.
func selectProjectPickerRow(t *testing.T, driver *UITestDriver, label string) {
	t.Helper()
	for range 32 {
		for _, line := range strings.Split(driver.Observe(ObserveOptions{}).PlainScreen, "\n") {
			if strings.Contains(line, "\u203a") && strings.Contains(line, label) {
				return
			}
		}
		runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "down"}})
	}
	t.Fatalf("no picker row named %q:\n%s", label, driver.Render())
}

// seatCursorOnProjectHeaderFor parks the cursor on one project's header in the
// cross-project view, which is where the project half of `e` answers.
func seatCursorOnProjectHeaderFor(t *testing.T, driver *UITestDriver, projectID string) {
	t.Helper()
	view := driver.App().crossOutliner
	for index, row := range view.Rows {
		if row.IsProjectHeader && row.ProjectID == projectID {
			view.Cursor = index
			return
		}
	}
	t.Fatalf("no project header for %q:\n%s", projectID, driver.Render())
}

// TestProjectCommentsSurviveTheOrdinaryTaskWorkBesideThem is the document half.
// Every act below rewrites the whole of the tasks.md the comments live in, each
// through a different call, and none of those calls is about comments. One of
// them dropping the block would stay invisible until the next launch.
func TestProjectCommentsSurviveTheOrdinaryTaskWorkBesideThem(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureMultiProject, 100, 35)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	shell := driver.App()

	const (
		sourceSlug  = "client-alpha"
		targetSlug  = "client-beta"
		sourceTask  = "Alpha migration kickoff"
		rescheduled = sourceTask + " rescheduled"
	)
	source := shell.projectByID(sourceSlug)
	if source == nil {
		t.Fatal("the fixture's source project is not in the shell cache")
	}
	want := []string{"Kickoff moved to Monday", "Room four is booked"}
	for _, text := range want {
		seatCursorOnProjectHeaderFor(t, driver, source.ID)
		runChecklistActions(t, driver, []UITestAction{
			{Type: UITestActionKey, Key: "C"},
			{Type: UITestActionText, Text: text},
			{Type: UITestActionKey, Key: "ctrl+s"},
			{Type: UITestActionWait},
		})
	}
	if got := projectCommentTextsOnDisk(t, workspace, sourceSlug); !reflect.DeepEqual(got, want) {
		t.Fatalf("seeded comments = %v, want %v", got, want)
	}

	assertCommentsIntact := func(t *testing.T, slug, after string) {
		t.Helper()
		if got := projectCommentTextsOnDisk(t, workspace, slug); !reflect.DeepEqual(got, want) {
			t.Fatalf("after %s the stored comments are %v, want %v", after, got, want)
		}
	}

	// An inline task edit rewrites the document through the single task-line
	// path, which is the narrowest write the product has.
	seatCursorOnTaskTitled(t, shell.crossOutliner, sourceTask)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "e"},
		{Type: UITestActionText, Text: " !1 "},
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	})
	assertCommentsIntact(t, sourceSlug, "an inline task edit")

	// An outline plan rewrites the whole task list at once, which is the widest.
	// The buffer travels the production path from the dump to the apply; only the
	// editor process in the middle is missing, because the harness has none.
	project, revision, buffer, scopeTaskIDs, err := shell.outlineDumpSource(source.ID, "")
	if err != nil {
		t.Fatalf("dump the outline: %v", err)
	}
	edited := strings.Replace(buffer, sourceTask, rescheduled, 1)
	if edited == buffer {
		t.Fatalf("the outline buffer does not name %q:\n%s", sourceTask, buffer)
	}
	bufferPath := filepath.Join(t.TempDir(), "outline.md")
	if err = os.WriteFile(bufferPath, []byte(edited), 0o600); err != nil {
		t.Fatalf("stage the outline buffer: %v", err)
	}
	runSession2AppCommand(t, shell, shell.handleOutlineEdited(OutlineEditedMsg{
		Project: project, Revision: revision, ScopeTaskIDs: scopeTaskIDs,
		Path: bufferPath, Buffer: edited,
	}))
	assertCommentsIntact(t, sourceSlug, "an outline edit")

	// A cross-project move commits both documents, and the comments belong to
	// neither task: the source keeps them and the destination gains none.
	seatCursorOnTaskTitled(t, shell.crossOutliner, rescheduled)
	runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "%"}})
	selectProjectPickerRow(t, driver, "Client Beta")
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	})
	if got := projectCommentTextsOnDisk(t, workspace, targetSlug); len(got) != 0 {
		t.Fatalf("the move carried project comments into the destination: %v", got)
	}
	assertCommentsIntact(t, sourceSlug, "a cross-project move")

	// And a rename rewrites the document at a new path entirely, which is the one
	// act that moves the block rather than rewriting it in place.
	//
	// The move left the source holding no tasks, so All prints no group header for
	// it any more and the rename is reached the way the product reaches an empty
	// project at all: scope onto it, and edit the one focused header row it leads
	// with.
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "P"},
		{Type: UITestActionKey, Key: "/"},
		{Type: UITestActionText, Text: "alpha"},
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	})
	seatCursorOnProjectHeaderFor(t, driver, source.ID)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "e"},
		{Type: UITestActionText, Text: " Renamed"},
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})
	assertCommentsIntact(t, "client-alpha-renamed", "a project rename")
}
