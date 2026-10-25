package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// This small fictional workspace complements the release stress fixture. Most
// tasks have only a title; similar project names and a few urgent rows expose
// hierarchy and reading effort that repeated numbered titles cannot exercise.
func buildEverydayFixture(w *uitestWorkspace) error {
	projects := []struct {
		name   string
		titles []string
	}{
		{"Atlas Website", []string{"Fix the password reset link", "Check the pricing page on a small screen", "Ask Maya for the revised logo", "Remove the old launch banner", "Write the accessibility review notes"}},
		{"Atlas Workshop", []string{"Confirm the room booking", "Send the agenda", "Print name cards", "Review the café’s lunch options", "Pack the spare adapters"}},
		{"Home", []string{"Book a bicycle service", "Return the library books", "Compare the two repair estimates"}},
	}
	for i, data := range projects {
		_, err := w.addProject(data.name, func(p *models.Project) {
			for j, title := range data.titles {
				task := newFixtureTask(p, title)
				task.ApplyWorkflowState(models.StateBacklog)
				if j == 2 {
					task.ApplyWorkflowState(models.StateInbox)
				}
				if i == 0 && j == 0 {
					task.ApplyWorkflowState(models.StateDoing)
					task.Priority = 1
					due := time.Now().AddDate(0, 0, -1)
					task.DueDate = &due
					task.ResumeNote = "The reset email works; check the expired-link screen next."
				}
				if i == 1 && j == 0 {
					task.ApplyWorkflowState(models.StateWaiting)
				}
			}
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func TestEverydayPolishReviewFrames(t *testing.T) {
	defer theme.ApplyTheme(theme.ThemeConfig{})
	root := os.Getenv("DOCKET_TERMINAL_REVIEW_DIR")
	if root == "" {
		root = t.TempDir()
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	for _, palette := range []string{"dark", "light"} {
		for _, size := range uxPresentationMatrixSizes {
			t.Run(fmt.Sprintf("%s-%dx%d", palette, size[0], size[1]), func(t *testing.T) {
				d, view := uxDailyUseDriver(t, FixtureEveryday, size[0], size[1])
				theme.ApplyTheme(theme.ThemeConfig{Name: palette})
				step := 0
				frame := func(label string) {
					t.Helper()
					obs := d.Observe(ObserveOptions{IncludeANSI: true})
					name := fmt.Sprintf("%s-%dx%d-transition-everyday-%dx%d-%02d-%s", palette, size[0], size[1], size[0], size[1], step, label)
					writePolishReviewFrame(t, root, name, obs, size)
					step++
				}
				press := func(keys ...string) { t.Helper(); uxClarityPress(t, d, uxClarityKeys(keys...)...) }
				title := "Send the revised workshop agenda"
				selectCaptured := func() {
					t.Helper()
					press("g", "g")
					for range len(view.Rows) {
						if view.Cursor >= 0 && view.Cursor < len(view.Rows) && view.Rows[view.Cursor].Task != nil && view.Rows[view.Cursor].Task.Title == title {
							return
						}
						press("j")
					}
					t.Fatalf("captured task not in view: %s", d.Observe(ObserveOptions{}).PlainScreen)
				}
				frame("overview")
				press("a")
				uxClarityPress(t, d, UITestAction{Type: UITestActionText, Text: title + " %atlas-workshop"})
				frame("capture")
				press("enter")
				d.Wait(5 * time.Second)
				press("g", "i")
				selectCaptured()
				frame("triage")
				press("w", "t")
				d.Wait(5 * time.Second)
				press("g", "t")
				selectCaptured()
				frame("planned")
				press("w", "g")
				d.Wait(5 * time.Second)
				task, _ := workspaceTaskTitled(t, d.App(), title)
				if task.WorkflowState != models.StateDoing {
					t.Fatal("start did not commit")
				}
				frame("started")
				press("w", "d")
				d.Wait(5 * time.Second)
				task, _ = workspaceTaskTitled(t, d.App(), title)
				if task.WorkflowState != models.StateDone {
					t.Fatal("completion did not commit")
				}
				frame("completed")
				press("u")
				d.Wait(5 * time.Second)
				task, _ = workspaceTaskTitled(t, d.App(), title)
				if task.WorkflowState != models.StateDoing || !strings.Contains(d.Observe(ObserveOptions{}).PlainScreen, "Send the revised") {
					t.Fatal("undo did not restore the task and its visibility")
				}
				frame("undone")
				press("v", "f10", "down")
				if !strings.Contains(d.Observe(ObserveOptions{}).PlainScreen, "Leave multi-select") {
					t.Fatal("unavailable Edit did not explain how to enable it")
				}
				if !strings.Contains(d.Observe(ObserveOptions{}).PlainScreen, "╰") {
					t.Fatal("feedback or wrapped menu hints clipped the menu's bottom border")
				}
				frame("unavailable-edit")
			})
		}
	}
}
