package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/testutil"
)

// TestReconstructProjectViewRefreshesCachedMetricsDashboardProject guards against
// the cross-project state contamination that occurs when the cached
// m.metricsDashboard was created for a different project: reconstructing the
// metrics view for project B must rebind the model to project B (or recreate it)
// rather than just calling computeMetrics() against the stale project pointer.
func TestReconstructProjectViewRefreshesCachedMetricsDashboardProject(t *testing.T) {
	app := newTestAppModel()
	projectA := testutil.NewTestProjectWithTasks("Project A")
	projectB := testutil.NewTestProjectWithTasks("Project B")
	setTestAppProjects(app, projectA, projectB)

	// Simulate the user having opened the metrics dashboard for project A.
	app.currentProject = projectA
	app.metricsDashboard = NewMetricsDashboardModelForProject(app.configManager, projectA)
	if app.metricsDashboard.project != projectA {
		t.Fatalf("setup precondition: expected metrics dashboard pinned to project A")
	}

	// Reconstruct metrics view via history navigation, targeting project B.
	app.reconstructProjectView(NavEntry{
		ViewType:  ViewMetricsDashboard,
		ProjectID: projectB.ID,
	})

	if app.metricsDashboard.project != projectB {
		t.Fatalf("metrics dashboard still pinned to %q after reconstructing for %q (stale cached model leaks across projects)",
			app.metricsDashboard.project.Name, projectB.Name)
	}
}

// TestReconstructProjectViewRefreshesCachedRoadmapProject mirrors the metrics
// dashboard guard for the roadmap view.
func TestReconstructProjectViewRefreshesCachedRoadmapProject(t *testing.T) {
	app := newTestAppModel()
	projectA := testutil.NewTestProjectWithTasks("Project A")
	projectB := testutil.NewTestProjectWithTasks("Project B")
	setTestAppProjects(app, projectA, projectB)

	app.currentProject = projectA
	app.roadmap = NewRoadmapModelForProject(app.runtime, projectA)
	if app.roadmap.project != projectA {
		t.Fatalf("setup precondition: expected roadmap pinned to project A")
	}

	app.reconstructProjectView(NavEntry{
		ViewType:  ViewRoadmap,
		ProjectID: projectB.ID,
	})

	if app.roadmap.project != projectB {
		t.Fatalf("roadmap still pinned to %q after reconstructing for %q (stale cached model leaks across projects)",
			app.roadmap.project.Name, projectB.Name)
	}
}
