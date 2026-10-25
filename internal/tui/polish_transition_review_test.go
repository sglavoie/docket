package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// Ordered frames exercise the real shell between states, including timers and
// transactions. They can be rendered by the existing Ghostty review runner;
// filenames preserve each sequence's order and identify its actual grid size.
func TestPolishTransitionReviewFrames(t *testing.T) {
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
			for _, sequence := range []string{"feedback", "refusal", "search-scope", "last-task", "peek-resize", "capture-scroll"} {
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
						name := fmt.Sprintf("%s-%dx%d-transition-%s-%dx%d-%02d-%s", palette, grid[0], grid[1], sequence, size[0], size[1], step, label)
						writePolishReviewFrame(t, root, name, obs, grid)
						step++
						return obs
					}
					press := func(keys ...string) { t.Helper(); uxClarityPress(t, d, uxClarityKeys(keys...)...) }
					wait := func() { t.Helper(); uxClarityPress(t, d, UITestAction{Type: UITestActionWait, Duration: "7s"}) }
					if sequence == "last-task" {
						press("a")
						uxClarityPress(t, d, UITestAction{Type: UITestActionText, Text: "Finish this last task"})
						press("enter")
						d.Wait(5 * time.Second)
						press("g", "i")
						wait()
						frame("before", size)
						press("w", "d")
						// Settle the disk commit without advancing the feedback clock.
						d.Wait(5 * time.Second)
						obs := frame("completed", size)
						if !strings.Contains(obs.PlainScreen, "Inbox is clear") {
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
					case "search-scope":
						reviewSearchScopeTransition(t, d, func(label string) UITestObservation { return frame(label, size) })
					case "refusal":
						reviewRefusalTransition(t, d, view.Rows[view.Cursor].Task.Title, func(label string) UITestObservation { return frame(label, size) })
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
						press("ctrl+x", "Y")
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

func reviewSearchScopeTransition(t *testing.T, d *UITestDriver, frame func(string) UITestObservation) {
	t.Helper()
	press := func(keys ...string) { t.Helper(); uxClarityPress(t, d, uxClarityKeys(keys...)...) }
	before := frame("before")
	press("/")
	if obs := frame("opened"); !strings.Contains(obs.PlainScreen, "Search · all modes") {
		t.Fatal("search did not explain its scope before typing")
	}
	uxClarityPress(t, d, UITestAction{Type: UITestActionText, Text: "Release"})
	press("enter")
	frame("results")
	press("Q")
	frame("explained")
	press("esc", "esc")
	if after := frame("returned"); before.PlainScreen != after.PlainScreen {
		t.Fatal("search changed the original reading frame")
	}
}

func reviewRefusalTransition(t *testing.T, d *UITestDriver, selectedTaskTitle string, frame func(string) UITestObservation) {
	t.Helper()
	before := frame("before")
	d.app.setWarningStatus("Cannot apply outline: the workspace changed while you were editing. Your draft is kept. Reopen the outline to review the newer tasks.")
	// Direct status setup must perform Update's frame synchronization.
	d.app.syncShellFrameMetrics()
	d.pendingCmd = d.app.statusReadingTimeout()
	d.drain()
	shown := frame("wrapped")
	selectedTitle, _, _ := strings.Cut(selectedTaskTitle, " with ")
	if !strings.Contains(shown.PlainScreen, selectedTitle) {
		t.Fatal("wrapped refusal hid the selected task")
	}
	if !strings.Contains(shown.PlainScreen, "newer tasks.") {
		t.Fatal("wrapped refusal lost the recovery instruction")
	}
	uxClarityPress(t, d, UITestAction{Type: UITestActionWait, Duration: "13s"})
	after := frame("expired")
	if before.PlainScreen != after.PlainScreen {
		t.Fatal("wrapped refusal changed the resting window")
	}
}

func writePolishReviewFrame(t *testing.T, root, name string, obs UITestObservation, grid [2]int) {
	t.Helper()
	assertUXScreenFitsItsTerminal(t, obs, grid)
	assertUXMonochromeKeepsContent(t, obs)
	if lipgloss.Height(obs.ANSIScreen) != grid[1] {
		t.Fatal("frame height changed")
	}
	for ext, content := range map[string]string{"ansi": obs.ANSIScreen, "txt": obs.PlainScreen} {
		if err := os.WriteFile(filepath.Join(root, name+"."+ext), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
