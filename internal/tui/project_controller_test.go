package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/projecthub"
)

func projectControllerTestProject() *models.Project {
	return &models.Project{ID: "proj", Slug: "sonar", Name: "Sonar"}
}

// TestProjectControllerConfirmationsAreModalWhileArmed pins the reading every
// project prompt gets: a confirmed prompt keeps its pending edit, because the
// commit has not read it yet and does so through the take; a cancellation drops
// it; and an unanswered key is swallowed rather than leaked to the view beneath.
func TestProjectControllerConfirmationsAreModalWhileArmed(t *testing.T) {
	t.Parallel()

	arms := []struct {
		axis    string
		arm     func(*projectController)
		resolve func(*projectController, string) confirmAnswer
		armed   func(*projectController) bool
		pending func(*projectController) bool
	}{
		{
			axis:    "archive",
			arm:     func(c *projectController) { c.armArchive(projectControllerTestProject(), true) },
			resolve: (*projectController).resolveArchiveKey,
			armed:   func(c *projectController) bool { return c.archiveConfirm.IsActive() },
			pending: func(c *projectController) bool { return c.pendingArchive != nil },
		},
		{
			axis: "delete",
			arm: func(c *projectController) {
				c.armDelete(projectControllerTestProject(), "Delete Sonar and everything it owns?", nil)
			},
			resolve: (*projectController).resolveDeleteKey,
			armed:   func(c *projectController) bool { return c.deleteConfirm.IsActive() },
			pending: func(c *projectController) bool { return c.pendingDelete != nil },
		},
		{
			axis: "park",
			arm: func(c *projectController) {
				c.armPark(projectParkPending{project: projectControllerTestProject()}, "Park Sonar?")
			},
			resolve: (*projectController).resolveParkKey,
			armed:   func(c *projectController) bool { return c.parkConfirm.IsActive() },
			pending: func(c *projectController) bool { return c.pendingPark.project != nil },
		},
	}
	keys := []struct {
		name       string
		key        string
		want       confirmAnswer
		stillArmed bool
		retained   bool
	}{
		{"confirm", "y", confirmAnswerConfirmed, false, true},
		{"cancel", "n", confirmAnswerCanceled, false, false},
		{"escape", "esc", confirmAnswerCanceled, false, false},
		{"unrelated key is swallowed", "j", confirmAnswerSwallowed, true, true},
	}

	for _, axis := range arms {
		for _, testCase := range keys {
			t.Run(axis.axis+"/"+testCase.name, func(t *testing.T) {
				t.Parallel()
				var controller projectController
				axis.arm(&controller)
				if !axis.armed(&controller) {
					t.Fatalf("%s arm leaves the prompt armed", axis.axis)
				}
				if got := axis.resolve(&controller, testCase.key); got != testCase.want {
					t.Fatalf("%s resolve(%q) = %d, want %d", axis.axis, testCase.key, got, testCase.want)
				}
				if got := axis.armed(&controller); got != testCase.stillArmed {
					t.Fatalf("%s armed after %q = %v, want %v", axis.axis, testCase.key, got, testCase.stillArmed)
				}
				if got := axis.pending(&controller); got != testCase.retained {
					t.Fatalf("%s pending after %q = %v, want %v", axis.axis, testCase.key, got, testCase.retained)
				}
			})
		}
	}
}

func TestProjectControllerUnarmedPromptsConsumeNothing(t *testing.T) {
	t.Parallel()

	var controller projectController
	if got := controller.resolveArchiveKey("y"); got != confirmAnswerUnarmed {
		t.Fatalf("resolveArchiveKey on an unarmed prompt = %d, want %d", got, confirmAnswerUnarmed)
	}
	if got := controller.resolveDeleteKey("y"); got != confirmAnswerUnarmed {
		t.Fatalf("resolveDeleteKey on an unarmed prompt = %d, want %d", got, confirmAnswerUnarmed)
	}
	if got := controller.resolveParkKey("y"); got != confirmAnswerUnarmed {
		t.Fatalf("resolveParkKey on an unarmed prompt = %d, want %d", got, confirmAnswerUnarmed)
	}
	if _, armed := controller.confirmOverlay(120); armed {
		t.Fatal("nothing armed, so the cluster renders no overlay")
	}
}

// TestProjectControllerArchiveKeepsItsDirection pins that the lifecycle axis
// carries which way it was requested all the way to the commit: taking an
// unarchive back as an archive would make the two acts one.
func TestProjectControllerArchiveKeepsItsDirection(t *testing.T) {
	t.Parallel()

	for _, archive := range []bool{true, false} {
		var controller projectController
		project := projectControllerTestProject()
		controller.armArchive(project, archive)

		taken, direction := controller.takeArchive()
		if taken != project || direction != archive {
			t.Fatalf("takeArchive() = (%v, %v), want the armed project and %v", taken, direction, archive)
		}
		if controller.archiveConfirm.IsActive() || controller.pendingArchive != nil || controller.archiving {
			t.Fatal("takeArchive disarms, so a dispatched lifecycle change cannot be prompted for twice")
		}
		if second, _ := controller.takeArchive(); second != nil {
			t.Fatalf("a second takeArchive() = %v, want nil", second)
		}
	}
}

// TestProjectControllerParkReleasesWhicheverRouteArmedIt pins the shared prompt:
// three routes raise it and one take releases it, so a declined editor save
// cannot leave a stale tag set behind for the next confirmation to commit.
func TestProjectControllerParkReleasesWhicheverRouteArmedIt(t *testing.T) {
	t.Parallel()

	project := projectControllerTestProject()
	cases := []struct {
		name    string
		pending projectParkPending
		check   func(*testing.T, projectParkPending)
	}{
		{
			name:    "tag picker",
			pending: projectParkPending{project: project, tags: []string{models.ParkedProjectTag}},
			check: func(t *testing.T, taken projectParkPending) {
				if taken.project != project || len(taken.tags) != 1 {
					t.Fatalf("taken = %+v, want the project and its replacement tag set", taken)
				}
			},
		},
		{
			name:    "project editor",
			pending: projectParkPending{editorSave: &projecthub.ProjectEditorSave{Name: "Sonar", Project: project}},
			check: func(t *testing.T, taken projectParkPending) {
				if taken.editorSave == nil || taken.editorSave.Name != "Sonar" {
					t.Fatalf("taken = %+v, want the whole editor save", taken)
				}
			},
		},
		{
			name:    "bulk tags",
			pending: projectParkPending{bulk: &projectBulkTagEdit{tag: models.ParkedProjectTag}},
			check: func(t *testing.T, taken projectParkPending) {
				if taken.bulk == nil || taken.bulk.tag != models.ParkedProjectTag {
					t.Fatalf("taken = %+v, want the resolved bulk edit", taken)
				}
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			var controller projectController
			controller.armPark(testCase.pending, "Park Sonar? Its 1 task leaves Active Projects.")
			if got := controller.parkConfirm.Prompt(); got != "Park Sonar? Its 1 task leaves Active Projects." {
				t.Fatalf("prompt = %q, want the one park sentence every route shows", got)
			}

			taken := controller.takePark()
			testCase.check(t, taken)
			if controller.parkConfirm.IsActive() {
				t.Fatal("takePark disarms the shared prompt")
			}
			if second := controller.takePark(); second.project != nil || second.bulk != nil || second.editorSave != nil {
				t.Fatalf("a second takePark() = %+v, want empty", second)
			}
		})
	}
}

// TestProjectControllerConfirmOverlayKeepsItsOrder pins archive before delete
// before park, which is the order the shell has always checked them in.
func TestProjectControllerConfirmOverlayKeepsItsOrder(t *testing.T) {
	t.Parallel()

	var controller projectController
	controller.armArchive(projectControllerTestProject(), true)
	controller.armDelete(projectControllerTestProject(), "Delete Sonar and everything it owns?", &chrome.ConfirmDetail{UndoHint: true})
	controller.armPark(projectParkPending{project: projectControllerTestProject()}, "Park Sonar?")

	overlay, armed := controller.confirmOverlay(120)
	if !armed || overlay == "" {
		t.Fatal("an armed confirmation renders its overlay")
	}
	if got := controller.archiveConfirm.RenderOverlay(120); overlay != got {
		t.Fatal("archive leads the overlay order")
	}

	controller.clearArchive()
	if got := controller.deleteConfirm.RenderOverlay(120); mustOverlay(t, &controller) != got {
		t.Fatal("delete follows archive")
	}

	controller.clearDelete()
	if got := controller.parkConfirm.RenderOverlay(120); mustOverlay(t, &controller) != got {
		t.Fatal("park follows delete")
	}
}

func mustOverlay(t *testing.T, controller *projectController) string {
	t.Helper()
	overlay, armed := controller.confirmOverlay(120)
	if !armed {
		t.Fatal("a confirmation is armed, so the cluster renders an overlay")
	}
	return overlay
}

// TestProjectClusterKeepsItsThreeAxesApart pins the axis separation CLAUDE.md
// requires against the one place a controller could quietly merge them: the
// cluster's own field list. A task lifecycle state has no home here, and the
// archive lifecycle and the tag set stay separate pendings behind separate
// dialogs, so neither project axis can be reached through the other.
func TestProjectClusterKeepsItsThreeAxesApart(t *testing.T) {
	t.Parallel()

	fields := projectControllerFields(t)
	for _, forbidden := range []string{
		"state", "State", "lifecycle", "Lifecycle", "archived", "Archived",
	} {
		if _, named := fields[forbidden]; named {
			t.Errorf("projectController field %s expresses a lifecycle axis the cluster must not own", forbidden)
		}
	}
	for _, required := range []string{
		"pendingArchive", "archiveConfirm", "pendingPark", "parkConfirm",
	} {
		if _, named := fields[required]; !named {
			t.Errorf("projectController lost %s, so the archive and tag axes share a home", required)
		}
	}
}

func projectControllerFields(t *testing.T) map[string]bool {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller for package directory")
	}
	controllerFile := filepath.Join(filepath.Dir(thisFile), "project_controller.go")
	parsed, err := parser.ParseFile(token.NewFileSet(), controllerFile, nil, 0)
	if err != nil {
		t.Fatalf("parse project_controller.go: %v", err)
	}

	fields := map[string]bool{}
	ast.Inspect(parsed, func(node ast.Node) bool {
		spec, isType := node.(*ast.TypeSpec)
		if !isType || spec.Name.Name != "projectController" {
			return true
		}
		controller, isStruct := spec.Type.(*ast.StructType)
		if !isStruct {
			return true
		}
		for _, field := range controller.Fields.List {
			for _, name := range field.Names {
				fields[name.Name] = true
			}
		}
		return false
	})
	if len(fields) == 0 {
		t.Fatal("projectController declares no fields")
	}
	return fields
}

// TestProjectClusterHasOneHome pins the cluster to its single named AppModel
// field. Every pending here is an edit a prompt is standing in front of; a
// second home on the god struct is how a prompt and the commit it gates would
// come to read different values.
func TestProjectClusterHasOneHome(t *testing.T) {
	t.Parallel()

	clustered := map[string]bool{
		"pendingProjectArchive":        true,
		"pendingProjectArchiveArchive": true,
		"projectArchiveConfirm":        true,
		"pendingProjectDelete":         true,
		"projectDeleteConfirm":         true,
		"pendingProjectTags":           true,
		"pendingProjectTagsSet":        true,
		"pendingProjectBulkTags":       true,
		"pendingProjectEditorSave":     true,
		"projectParkConfirm":           true,
		"projectTagActionSubject":      true,
	}
	assertAppModelClusterHasOneHome(t, "project", clustered)
}

// assertAppModelClusterHasOneHome walks the AppModel type spec and reports both
// halves of a controller extraction: no retired field came back, and the
// controller itself is named exactly once.
func assertAppModelClusterHasOneHome(t *testing.T, controller string, clustered map[string]bool) {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller for package directory")
	}
	appFile := filepath.Join(filepath.Dir(thisFile), "app.go")
	parsed, err := parser.ParseFile(token.NewFileSet(), appFile, nil, 0)
	if err != nil {
		t.Fatalf("parse app.go: %v", err)
	}

	controllerFields := 0
	ast.Inspect(parsed, func(node ast.Node) bool {
		spec, isType := node.(*ast.TypeSpec)
		if !isType || spec.Name.Name != "AppModel" {
			return true
		}
		model, isStruct := spec.Type.(*ast.StructType)
		if !isStruct {
			return true
		}
		for _, field := range model.Fields.List {
			for _, name := range field.Names {
				if clustered[name.Name] {
					t.Errorf("AppModel field %s belongs to the %s controller", name.Name, controller)
				}
				if name.Name == controller {
					controllerFields++
				}
			}
		}
		return false
	})
	if controllerFields != 1 {
		t.Errorf("AppModel %s fields = %d, want exactly 1", controller, controllerFields)
	}
}
