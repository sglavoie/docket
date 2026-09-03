package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
)

func TestPromotedToolNotesScope(t *testing.T) {
	workA := &models.Project{ID: "work-a", Name: "Work A", Slug: "work-a"}
	workB := &models.Project{ID: "work-b", Name: "Work B", Slug: "work-b"}
	home := &models.Project{ID: "home", Name: "Home", Slug: "home"}
	archived := &models.Project{
		ID: "old", Name: "Old", Slug: "old", Archived: true,
	}

	tests := []struct {
		name           string
		configure      func(*AppModel)
		wantProjectIDs []string
		wantScope      NotesScope
	}{
		{
			name:           "all",
			configure:      func(*AppModel) {},
			wantProjectIDs: []string{home.ID, workA.ID, workB.ID},
			wantScope:      AllNotesScope(),
		},
		{
			name: "project focus",
			configure: func(app *AppModel) {
				app.crossOutliner.scopedProjectID = workB.ID
			},
			wantProjectIDs: []string{workB.ID},
			wantScope:      AllNotesScope(),
		},
		{
			name: "task row cursor",
			configure: func(app *AppModel) {
				app.crossOutliner.rows = []outlinerRow{
					{task: &models.Task{ID: "t1"}, ProjectID: workA.ID},
				}
				app.crossOutliner.cursor = 0
			},
			wantProjectIDs: []string{home.ID, workA.ID, workB.ID},
			wantScope:      AllNotesScope(),
		},
		{
			name: "project header cursor",
			configure: func(app *AppModel) {
				app.crossOutliner.rows = []outlinerRow{
					{IsProjectHeader: true, ProjectID: workA.ID},
					{IsProjectHeader: true, ProjectID: workB.ID},
				}
				app.crossOutliner.cursor = 1
			},
			wantProjectIDs: []string{home.ID, workA.ID, workB.ID},
			wantScope:      ProjectNotesScope(workB.ID),
		},
		{
			name: "archived project header cursor",
			configure: func(app *AppModel) {
				setTestAppProjects(app, workA, workB, home, archived)
				app.crossOutliner.loadedProjects = append(
					app.crossOutliner.loadedProjects, archived)
				app.crossOutliner.rows = []outlinerRow{
					{IsProjectHeader: true, ProjectID: archived.ID},
				}
				app.crossOutliner.cursor = 0
			},
			wantProjectIDs: []string{home.ID, archived.ID, workA.ID, workB.ID},
			wantScope:      AllNotesScope(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newUITestApp()
			setTestAppProjects(app, workA, workB, home)
			app.crossOutliner.loadedProjects = []*models.Project{workA, workB, home}
			test.configure(app)
			app.operationRegistry = operation.NewOperationRegistry()
			app.operationExecutor = NewOperationExecutor(app.operationRegistry)
			if err := app.registerCoreInputOperations(); err != nil {
				t.Fatalf("register core operations: %v", err)
			}

			context := app.operationCommandContext()
			if !slices.Equal(context.ScopeProjectIDs, test.wantProjectIDs) {
				t.Fatalf("operation scope = %v, want %v", context.ScopeProjectIDs, test.wantProjectIDs)
			}

			notes := executePromotedTool(t, app, operation.OperationCoreNotes, context)
			msg, ok := notes.(ShowNotesManagerMsg)
			if !ok || msg.Scope != test.wantScope || msg.Focus != (NoteRowIdentity{}) {
				t.Fatalf("Notes message = %#v, want unfocused %#v", notes, test.wantScope)
			}
		})
	}
}

func TestPromotedToolMessagesOpenTypedSecondaryChildren(t *testing.T) {
	tests := []struct {
		name    string
		message any
		check   func(*testing.T, RouteParams)
	}{
		{
			name:    "all notes",
			message: ShowNotesManagerMsg{Scope: AllNotesScope()},
			check: func(t *testing.T, params RouteParams) {
				got, ok := params.(ManageParams)
				if !ok || got != (ManageParams{Module: ManageNotes, NotesScope: AllNotesScope()}) {
					t.Fatalf("All Notes route params = %#v", params)
				}
			},
		},
		{
			name:    "global notes",
			message: ShowNotesManagerMsg{Scope: GlobalNotesScope()},
			check: func(t *testing.T, params RouteParams) {
				got, ok := params.(ManageParams)
				if !ok || got != (ManageParams{Module: ManageNotes, NotesScope: GlobalNotesScope()}) {
					t.Fatalf("Global Notes route params = %#v", params)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newTestAppModel()
			_, _ = app.Update(test.message)
			route := app.navigation.Primary.Current()
			if route.ID() != SecondaryTool || app.navigation.Primary.Depth() != 2 {
				t.Fatalf("route = %s depth %d, want typed SecondaryTool child", route.ID(), app.navigation.Primary.Depth())
			}
			test.check(t, route.Params())
		})
	}
}

func executePromotedTool(
	t *testing.T,
	app *AppModel,
	id operation.OperationID,
	context operation.CommandContext,
) any {
	t.Helper()
	result := app.operationExecutor.Execute(OperationExecutionRequest{
		ID:      id,
		Context: context,
	})
	if result.Err != nil {
		t.Fatalf("execute %s: %v", id, result.Err)
	}
	if result.Message == nil {
		t.Fatalf("execute %s returned no message", id)
	}
	return result.Message
}
