package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// Ordered frames exercise the real shell between states, including timers and
// transactions. They can be rendered by the existing Ghostty review runner;
// filenames preserve each sequence's order and identify its actual grid size.
func TestPolishTransitionReviewFrames(t *testing.T) {
	defer theme.ApplyTheme(theme.ThemeConfig{})
	root := os.Getenv("TT_TERMINAL_REVIEW_DIR")
	if root == "" {
		root = t.TempDir()
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	for _, palette := range []string{"dark", "light"} {
		for _, size := range uxPresentationMatrixSizes {
			for _, sequence := range []string{"feedback", "last-task", "peek-resize", "capture-scroll"} {
				t.Run(fmt.Sprintf("%s-%dx%d-%s", palette, size[0], size[1], sequence), func(t *testing.T) {
					fixture := FixtureRelease
					if sequence == "last-task" {
						fixture = FixtureFresh
					}
					d, view := uxDailyUseDriver(t, fixture, size[0], size[1])
					theme.ApplyTheme(theme.ThemeConfig{Name: palette})
					step := 0
					frame := func(label string, grid [2]int) UITestObservation {
						t.Helper()
						obs := d.Observe(ObserveOptions{IncludeANSI: true})
						assertUXScreenFitsItsTerminal(t, obs, grid)
						assertUXMonochromeKeepsContent(t, obs)
						if lipgloss.Height(obs.ANSIScreen) != grid[1] {
							t.Fatal("frame height changed")
						}
						name := fmt.Sprintf("%s-%dx%d-transition-%s-%dx%d-%02d-%s", palette, grid[0], grid[1], sequence, size[0], size[1], step, label)
						for ext, content := range map[string]string{"ansi": obs.ANSIScreen, "txt": obs.PlainScreen} {
							if err := os.WriteFile(filepath.Join(root, name+"."+ext), []byte(content), 0644); err != nil {
								t.Fatal(err)
							}
						}
						step++
						return obs
					}
					press := func(keys ...string) { t.Helper(); uxClarityPress(t, d, uxClarityKeys(keys...)...) }
					wait := func() { t.Helper(); uxClarityPress(t, d, UITestAction{Type: UITestActionWait, Duration: "7s"}) }
					if sequence == "last-task" {
						press("a")
						uxClarityPress(t, d, UITestAction{Type: UITestActionText, Text: "Finish this last task"})
						press("enter", "g", "i")
						wait()
						frame("before", size)
						press("w", "d")
						obs := frame("completed", size)
						if !strings.Contains(obs.PlainScreen, "Inbox is empty") {
							t.Fatalf("last task did not leave Inbox:\n%s", obs.PlainScreen)
						}
						wait()
						frame("settled", size)
						return
					}
					press("g", "i")
					for range 32 {
						press("j")
					}
					wait()
					if view.Cursor < 0 || view.Cursor >= len(view.Rows) || view.Rows[view.Cursor].Task == nil {
						t.Fatal("fixture has no selected task")
					}
					identity := view.Rows[view.Cursor].Task.ID
					switch sequence {
					case "peek-resize":
						var paragraphs []string
						for i := range 20 {
							paragraphs = append(paragraphs, fmt.Sprintf("Passage %02d: Keep this sentence visible through a window resize. 確認 équipe.", i))
						}
						view.Rows[view.Cursor].Task.Description = strings.Join(paragraphs, "\n\n")
						press("b", "ctrl+d", "ctrl+d", "ctrl+d")
						before := frame("reading", size)
						via := [2]int{60, 20}
						if size == via {
							via = [2]int{100, 35}
						}
						d.Resize(via[0], via[1])
						frame("resized", via)
						d.Resize(size[0], size[1])
						after := frame("returned", size)
						if before.PlainScreen != after.PlainScreen {
							t.Fatal("resize round trip changed the reading frame")
						}
					case "feedback":
						before := frame("before", size)
						d.dispatch(status.StatusMsg("Saved the selected task"))
						d.drain()
						shown := frame("message", size)
						if !strings.Contains(shown.PlainScreen, "Saved the selected task") {
							t.Fatal("feedback was not visible")
						}
						wait()
						after := frame("expired", size)
						if before.PlainScreen != after.PlainScreen {
							t.Fatal("feedback expiry did not restore the resting frame")
						}
					default:
						press("b")
						before := frame("before", size)
						cursor, scroll := view.Cursor, view.ViewScrollOffset
						if scroll == 0 {
							t.Fatal("capture fixture was not scrolled")
						}
						press("a")
						frame("opened", size)
						uxClarityPress(t, d, UITestAction{Type: UITestActionText, Text: "Follow up on the release"})
						obs := frame("typing", size)
						if !strings.Contains(obs.PlainScreen, "Follow up on the release") {
							t.Fatal("active draft was hidden")
						}
						press("esc", "Y")
						after := frame("closed", size)
						if view.Cursor != cursor || view.ViewScrollOffset != scroll {
							t.Fatal("capture changed outline position")
						}
						if before.PlainScreen != after.PlainScreen {
							t.Fatal("capture close did not restore the reading frame")
						}
					}
					if view.Rows[view.Cursor].Task == nil || view.Rows[view.Cursor].Task.ID != identity {
						t.Fatal("transition changed selected task")
					}
				})
			}
		}
	}
}
