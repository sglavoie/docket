package taskref

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// newFixture builds a project and a task with the IDs the caller names, without
// going through the model constructors' validation. The zero-value cases below
// need an empty ID on purpose, which NewProject refuses outright.
func newFixture(t *testing.T, projectID, taskID string) (*models.Task, *models.Project) {
	t.Helper()
	return &models.Task{ID: taskID}, &models.Project{ID: projectID}
}

func TestForBuildsTheIdentityFromTaskAndProject(t *testing.T) {
	task, project := newFixture(t, "project-a", "task-1")
	got := For(task, project)
	want := TaskIdentity{ProjectID: "project-a", TaskID: "task-1"}
	if got != want {
		t.Fatalf("For(task, project) = %#v, want %#v", got, want)
	}
	if !got.Valid() {
		t.Fatalf("identity built from a complete pair is invalid: %#v", got)
	}
}

// For deliberately produces the zero value rather than a partial identity: a
// half-filled key would compare unequal to itself across a refresh and would
// address the wrong task once the project half is filled in elsewhere.
func TestForRefusesIncompleteInputWithTheZeroValue(t *testing.T) {
	completeTask, completeProject := newFixture(t, "project-a", "task-1")
	emptyTask, emptyProject := newFixture(t, "", "")

	cases := []struct {
		name    string
		task    *models.Task
		project *models.Project
	}{
		{"nil task", nil, completeProject},
		{"nil project", completeTask, nil},
		{"both nil", nil, nil},
		{"empty task ID", emptyTask, completeProject},
		{"empty project ID", completeTask, emptyProject},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := For(test.task, test.project)
			if got != (TaskIdentity{}) {
				t.Fatalf("For = %#v, want the zero identity", got)
			}
			if got.Valid() {
				t.Fatal("the zero identity reports itself valid")
			}
		})
	}
}

func TestValidRequiresBothHalvesOfTheKey(t *testing.T) {
	cases := []struct {
		name     string
		identity TaskIdentity
		want     bool
	}{
		{"complete", TaskIdentity{ProjectID: "project-a", TaskID: "task-1"}, true},
		{"zero", TaskIdentity{}, false},
		{"project only", TaskIdentity{ProjectID: "project-a"}, false},
		{"task only", TaskIdentity{TaskID: "task-1"}, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := test.identity.Valid(); got != test.want {
				t.Fatalf("%#v.Valid() = %t, want %t", test.identity, got, test.want)
			}
		})
	}
}

// The locator conversion must keep project ownership on both legs. Dropping it
// would make the application layer resolve a bare task UUID by iterating
// projects, which is exactly what TaskLocator exists to prevent.
func TestLocatorRoundTripKeepsProjectOwnership(t *testing.T) {
	identity := TaskIdentity{ProjectID: "project-a", TaskID: "task-1"}

	locator := identity.Locator()
	if want := (app.TaskLocator{ProjectID: "project-a", TaskID: "task-1"}); locator != want {
		t.Fatalf("Locator() = %#v, want %#v", locator, want)
	}
	if !locator.Valid() {
		t.Fatalf("locator from a valid identity is invalid: %#v", locator)
	}
	if got := FromLocator(locator); got != identity {
		t.Fatalf("FromLocator(Locator()) = %#v, want %#v", got, identity)
	}
}

// The inverse holds in the other direction too, so a locator that came back
// from the application layer addresses the selection it was built from.
func TestFromLocatorRoundTripsInBothDirections(t *testing.T) {
	locators := []app.TaskLocator{
		{ProjectID: "project-a", TaskID: "task-1"},
		{ProjectID: "project-b", TaskID: "task-1"},
		{},
		{ProjectID: "project-a"},
		{TaskID: "task-1"},
	}
	for _, locator := range locators {
		identity := FromLocator(locator)
		if identity.Valid() != locator.Valid() {
			t.Fatalf("FromLocator(%#v).Valid() = %t, want %t", locator, identity.Valid(), locator.Valid())
		}
		if got := identity.Locator(); got != locator {
			t.Fatalf("Locator(FromLocator(%#v)) = %#v", locator, got)
		}
	}
}

// The identity is a map key throughout the TUI, so two tasks sharing a UUID
// across projects must stay distinct. Session 2.4's planning fixtures rely on
// this and the property belongs here rather than in a surface test.
func TestIdentitiesAreDistinctPerProject(t *testing.T) {
	alphaTask, alpha := newFixture(t, "alpha", "shared")
	betaTask, beta := newFixture(t, "beta", "shared")

	alphaID, betaID := For(alphaTask, alpha), For(betaTask, beta)
	if alphaID == betaID {
		t.Fatalf("identities collide across projects: %#v", alphaID)
	}

	selected := map[TaskIdentity]bool{alphaID: true}
	if selected[betaID] {
		t.Fatal("a beta identity hit an alpha entry in the selection map")
	}
	if !selected[For(alphaTask, alpha)] {
		t.Fatal("a recomputed alpha identity missed its own selection entry")
	}
}
