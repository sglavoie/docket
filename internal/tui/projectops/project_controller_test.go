package projectops

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
		arm     func(*Controller)
		resolve func(*Controller, string) chrome.ConfirmAnswer
		armed   func(*Controller) bool
		pending func(*Controller) bool
	}{
		{
			axis:    "archive",
			arm:     func(c *Controller) { c.ArmArchive(projectControllerTestProject(), true) },
			resolve: (*Controller).ResolveArchiveKey,
			armed:   func(c *Controller) bool { return c.archiveConfirm.IsActive() },
			pending: func(c *Controller) bool { return c.pendingArchive != nil },
		},
		{
			axis: "delete",
			arm: func(c *Controller) {
				c.ArmDelete(projectControllerTestProject(), "Delete Sonar and everything it owns?", nil)
			},
			resolve: (*Controller).ResolveDeleteKey,
			armed:   func(c *Controller) bool { return c.deleteConfirm.IsActive() },
			pending: func(c *Controller) bool { return c.pendingDelete != nil },
		},
		{
			axis: "park",
			arm: func(c *Controller) {
				c.ArmPark(ParkPending{Project: projectControllerTestProject()}, "Park Sonar?")
			},
			resolve: (*Controller).ResolveParkKey,
			armed:   func(c *Controller) bool { return c.parkConfirm.IsActive() },
			pending: func(c *Controller) bool { return c.pendingPark.Project != nil },
		},
	}
	keys := []struct {
		name       string
		key        string
		want       chrome.ConfirmAnswer
		stillArmed bool
		retained   bool
	}{
		{"confirm", "y", chrome.ConfirmAnswerConfirmed, false, true},
		{"cancel", "n", chrome.ConfirmAnswerCanceled, false, false},
		{"escape", "esc", chrome.ConfirmAnswerCanceled, false, false},
		{"unrelated key is swallowed", "j", chrome.ConfirmAnswerSwallowed, true, true},
	}

	for _, axis := range arms {
		for _, testCase := range keys {
			t.Run(axis.axis+"/"+testCase.name, func(t *testing.T) {
				t.Parallel()
				var controller Controller
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

	var controller Controller
	if got := controller.ResolveArchiveKey("y"); got != chrome.ConfirmAnswerUnarmed {
		t.Fatalf("ResolveArchiveKey on an unarmed prompt = %d, want %d", got, chrome.ConfirmAnswerUnarmed)
	}
	if got := controller.ResolveDeleteKey("y"); got != chrome.ConfirmAnswerUnarmed {
		t.Fatalf("ResolveDeleteKey on an unarmed prompt = %d, want %d", got, chrome.ConfirmAnswerUnarmed)
	}
	if got := controller.ResolveParkKey("y"); got != chrome.ConfirmAnswerUnarmed {
		t.Fatalf("ResolveParkKey on an unarmed prompt = %d, want %d", got, chrome.ConfirmAnswerUnarmed)
	}
	if _, armed := controller.ConfirmOverlay(120); armed {
		t.Fatal("nothing armed, so the cluster renders no overlay")
	}
}

// TestProjectControllerArchiveKeepsItsDirection pins that the lifecycle axis
// carries which way it was requested all the way to the commit: taking an
// unarchive back as an archive would make the two acts one.
func TestProjectControllerArchiveKeepsItsDirection(t *testing.T) {
	t.Parallel()

	for _, archive := range []bool{true, false} {
		var controller Controller
		project := projectControllerTestProject()
		controller.ArmArchive(project, archive)

		taken, direction := controller.TakeArchive()
		if taken != project || direction != archive {
			t.Fatalf("TakeArchive() = (%v, %v), want the armed project and %v", taken, direction, archive)
		}
		if controller.archiveConfirm.IsActive() || controller.pendingArchive != nil || controller.archiving {
			t.Fatal("TakeArchive disarms, so a dispatched lifecycle change cannot be prompted for twice")
		}
		if second, _ := controller.TakeArchive(); second != nil {
			t.Fatalf("a second TakeArchive() = %v, want nil", second)
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
		pending ParkPending
		check   func(*testing.T, ParkPending)
	}{
		{
			name:    "tag picker",
			pending: ParkPending{Project: project, Tags: []string{models.ParkedProjectTag}},
			check: func(t *testing.T, taken ParkPending) {
				if taken.Project != project || len(taken.Tags) != 1 {
					t.Fatalf("taken = %+v, want the project and its replacement tag set", taken)
				}
			},
		},
		{
			name:    "project editor",
			pending: ParkPending{EditorSave: &projecthub.ProjectEditorSave{Name: "Sonar", Project: project}},
			check: func(t *testing.T, taken ParkPending) {
				if taken.EditorSave == nil || taken.EditorSave.Name != "Sonar" {
					t.Fatalf("taken = %+v, want the whole editor save", taken)
				}
			},
		},
		{
			name:    "bulk tags",
			pending: ParkPending{Bulk: &BulkTagEdit{Tag: models.ParkedProjectTag}},
			check: func(t *testing.T, taken ParkPending) {
				if taken.Bulk == nil || taken.Bulk.Tag != models.ParkedProjectTag {
					t.Fatalf("taken = %+v, want the resolved bulk edit", taken)
				}
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			var controller Controller
			controller.ArmPark(testCase.pending, "Park Sonar? Its 1 task leaves Active Projects.")
			if got := controller.parkConfirm.Prompt(); got != "Park Sonar? Its 1 task leaves Active Projects." {
				t.Fatalf("prompt = %q, want the one park sentence every route shows", got)
			}

			taken := controller.TakePark()
			testCase.check(t, taken)
			if controller.parkConfirm.IsActive() {
				t.Fatal("TakePark disarms the shared prompt")
			}
			if second := controller.TakePark(); second.Project != nil || second.Bulk != nil || second.EditorSave != nil {
				t.Fatalf("a second TakePark() = %+v, want empty", second)
			}
		})
	}
}

// TestProjectControllerConfirmOverlayKeepsItsOrder pins archive before delete
// before park, which is the order the shell has always checked them in.
func TestProjectControllerConfirmOverlayKeepsItsOrder(t *testing.T) {
	t.Parallel()

	var controller Controller
	controller.ArmArchive(projectControllerTestProject(), true)
	controller.ArmDelete(projectControllerTestProject(), "Delete Sonar and everything it owns?", &chrome.ConfirmDetail{UndoHint: true})
	controller.ArmPark(ParkPending{Project: projectControllerTestProject()}, "Park Sonar?")

	overlay, armed := controller.ConfirmOverlay(120)
	if !armed || overlay == "" {
		t.Fatal("an armed confirmation renders its overlay")
	}
	if got := controller.archiveConfirm.RenderOverlay(120); overlay != got {
		t.Fatal("archive leads the overlay order")
	}

	controller.ClearArchive()
	if got := controller.deleteConfirm.RenderOverlay(120); mustOverlay(t, &controller) != got {
		t.Fatal("delete follows archive")
	}

	controller.ClearDelete()
	if got := controller.parkConfirm.RenderOverlay(120); mustOverlay(t, &controller) != got {
		t.Fatal("park follows delete")
	}
}

func mustOverlay(t *testing.T, controller *Controller) string {
	t.Helper()
	overlay, armed := controller.ConfirmOverlay(120)
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
			t.Errorf("Controller field %s expresses a lifecycle axis the cluster must not own", forbidden)
		}
	}
	for _, required := range []string{
		"pendingArchive", "archiveConfirm", "pendingPark", "parkConfirm",
	} {
		if _, named := fields[required]; !named {
			t.Errorf("Controller lost %s, so the archive and tag axes share a home", required)
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
		if !isType || spec.Name.Name != "Controller" {
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
		t.Fatal("Controller declares no fields")
	}
	return fields
}
