package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestPromotedToolOperationsPreserveActiveScopeSemantics(t *testing.T) {
	workA := &models.Project{ID: "work-a", Name: "Work A", Slug: "work-a", Context: "work"}
	workB := &models.Project{ID: "work-b", Name: "Work B", Slug: "work-b", Context: "work"}
	home := &models.Project{ID: "home", Name: "Home", Slug: "home", Context: "home"}

	tests := []struct {
		name             string
		configure        func(*OutlinerModel)
		wantProjectIDs   []string
		wantProjectNotes *models.Project
	}{
		{
			name:           "all",
			configure:      func(*OutlinerModel) {},
			wantProjectIDs: []string{home.ID, workA.ID, workB.ID},
		},
		{
			name: "context",
			configure: func(out *OutlinerModel) {
				out.activeContext = RegularContextScope("work")
				out.refreshContextScopeProjects()
			},
			wantProjectIDs: []string{workA.ID, workB.ID},
		},
		{
			name: "project focus",
			configure: func(out *OutlinerModel) {
				out.scopedProjectID = workB.ID
			},
			wantProjectIDs:   []string{workB.ID},
			wantProjectNotes: workB,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newUITestApp()
			setTestAppProjects(app, workA, workB, home)
			app.crossOutliner.loadedProjects = []*models.Project{workA, workB, home}
			test.configure(app.crossOutliner)
			app.operationRegistry = NewOperationRegistry()
			app.operationExecutor = NewOperationExecutor(app.operationRegistry)
			if err := app.registerCoreInputOperations(); err != nil {
				t.Fatalf("register core operations: %v", err)
			}

			context := app.operationCommandContext()
			if !slices.Equal(context.ScopeProjectIDs, test.wantProjectIDs) {
				t.Fatalf("operation scope = %v, want %v", context.ScopeProjectIDs, test.wantProjectIDs)
			}

			notes := executePromotedTool(t, app, operationCoreNotes, context)
			if test.wantProjectNotes == nil {
				if _, ok := notes.(ShowGlobalNotesMsg); !ok {
					t.Fatalf("Notes message = %T, want ShowGlobalNotesMsg", notes)
				}
			} else {
				msg, ok := notes.(ShowNotesMsg)
				if !ok || msg.Project != test.wantProjectNotes {
					t.Fatalf("Notes message = %#v, want project %s", notes, test.wantProjectNotes.ID)
				}
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
			name:    "global notes",
			message: ShowGlobalNotesMsg{},
			check: func(t *testing.T, params RouteParams) {
				got, ok := params.(ManageParams)
				if !ok || got.Module != ManageNotes || got.Scope.Kind != ScopeApp {
					t.Fatalf("global Notes route params = %#v", params)
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
	id OperationID,
	context CommandContext,
) any {
	t.Helper()
	result := app.operationExecutor.Execute(OperationExecutionRequest{
		ID:      id,
		Context: context,
		Source:  OperationSourceDirectKey,
	})
	if result.Err != nil {
		t.Fatalf("execute %s: %v", id, result.Err)
	}
	if result.Message == nil {
		t.Fatalf("execute %s returned no message", id)
	}
	return result.Message
}
