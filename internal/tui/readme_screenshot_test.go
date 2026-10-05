package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestReadmeScreenshot exports an actual application frame over fictional data.
// The optional output directory follows the existing visual-review convention;
// ordinary test runs write only into their own temporary directory.
func TestReadmeScreenshot(t *testing.T) {
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
	driver, err := w.launch(100, 32)
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.SendKey("b"); err != nil {
		t.Fatal(err)
	}
	driver.drain()
	obs := driver.Observe(ObserveOptions{IncludeANSI: true})
	if len(obs.LayoutIssues) != 0 || !strings.Contains(obs.PlainScreen, "Start with the narrow-screen navigation.") {
		t.Fatalf("screenshot does not show the task and Peek cleanly:\n%s\n%+v", obs.PlainScreen, obs.LayoutIssues)
	}
	dir := os.Getenv("TT_README_SCREENSHOT_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "outliner.ansi"), []byte(obs.ANSIScreen), 0o644); err != nil {
		t.Fatal(err)
	}
}
