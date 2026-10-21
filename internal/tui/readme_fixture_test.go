package tui

import (
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// readmeWidth and readmeHeight are the terminal every README image is drawn at.
const (
	readmeWidth  = 100
	readmeHeight = 32
)

// launchReadmeFixture opens the fictional two-project workspace every README
// image is drawn from, at the 100x32 size those images share.
func launchReadmeFixture(t *testing.T) *UITestDriver {
	t.Helper()
	w, err := newUITestWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Cleanup() })
	_, err = w.addProject("Website", func(p *models.Project) {
		task := newFixtureTask(p, "Ship the new landing page")
		task.ApplyWorkflowState(models.StateDoing)
		task.Priority = models.PriorityHigh
		task.Description = "Polish the introduction, check the mobile layout, and publish the first version."
		task.ResumeNote = "Start with the narrow-screen navigation."
		task.Tags = []string{"design", "feature"}
		task.Context = "desk"
		newFixtureTask(p, "Collect feedback from the first readers").ApplyWorkflowState(models.StateInbox)
		review := newFixtureTask(p, "Review the accessibility checklist")
		review.ApplyWorkflowState(models.StateBacklog)
		review.Review = true
		later := newFixtureTask(p, "Write a short getting-started guide")
		later.ApplyWorkflowState(models.StateBacklog)
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.addProject("Home", func(p *models.Project) {
		newFixtureTask(p, "Plan the weekend walk").ApplyWorkflowState(models.StateInbox)
		task := newFixtureTask(p, "Get an estimate for the bookshelf")
		task.ApplyWorkflowState(models.StateWaiting)
	})
	if err != nil {
		t.Fatal(err)
	}
	driver, err := w.launch(readmeWidth, readmeHeight)
	if err != nil {
		t.Fatal(err)
	}
	return driver
}

// readmeScreenshotDir is where a README image's source frames are exported:
// TT_README_SCREENSHOT_DIR when regenerating them, else the test's own
// temporary directory so ordinary runs leave the committed images alone.
func readmeScreenshotDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("TT_README_SCREENSHOT_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}
