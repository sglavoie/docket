package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/theme"
)

// Export the actual product frames for the opt-in Ghostty pixel review. The
// ordinary suite also checks their geometry; the native test renders these
// bytes through the same Metal surface as tt.app, rather than an SVG emulator.
func TestPolishVisualReviewFrames(t *testing.T) {
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
			for _, state := range []string{"peek", "peek-rich", "peek-expanded", "menu-peek", "help", "capture-error", "expanded-schedule", "expanded-relations"} {
				name := fmt.Sprintf("%s-%dx%d-%s", palette, size[0], size[1], state)
				t.Run(name, func(t *testing.T) {
					d, view := uxDailyUseDriver(t, FixtureUXAcceptance, size[0], size[1])
					theme.ApplyTheme(theme.ThemeConfig{Name: palette})
					if state == "peek-rich" || state == "peek-expanded" {
						uxClarityPress(t, d, uxClarityKeys("s")...)
					}
					for i, row := range view.Rows {
						if row.IsTaskRow() && row.Task != nil && row.Task.DueDate != nil {
							view.Cursor = i
							break
						}
					}
					keys := []string{"b"}
					switch state {
					case "peek-expanded":
						keys = append(keys, "B")
					case "menu-peek":
						keys = append(keys, "space")
					case "help":
						keys = []string{"?"}
					case "capture-error", "expanded-schedule", "expanded-relations":
						keys = []string{"a"}
					}
					for _, key := range keys {
						uxClarityPress(t, d, uxClarityKeys(key)...)
					}
					if state == "capture-error" {
						if err := d.runAction(UITestAction{Type: UITestActionText, Text: "Keep this long draft while correcting its invalid deadline @not-a-date"}); err != nil {
							t.Fatal(err)
						}
						uxClarityPress(t, d, uxClarityKeys("enter")...)
					}
					if strings.HasPrefix(state, "expanded-") {
						if err := d.runAction(UITestAction{Type: UITestActionText, Text: "Prepare release @@tomorrow <1 ~2 %ux-platform-delivery"}); err != nil {
							t.Fatal(err)
						}
						if d.App().taskComposer.IsAutocompleteActive() {
							uxClarityPress(t, d, uxClarityKeys("esc")...)
						}
						uxClarityPress(t, d, uxClarityKeys("ctrl+f", "shift+right")...)
						if d.App().taskComposer == nil || d.App().taskComposer.Expanded == nil {
							t.Fatal("capture did not expand")
						}
						if state == "expanded-relations" {
							uxClarityPress(t, d, uxClarityKeys("shift+right", "shift+right")...)
						}
					}
					obs := d.Observe(ObserveOptions{IncludeANSI: true})
					assertUXScreenFitsItsTerminal(t, obs, size)
					assertUXMonochromeKeepsContent(t, obs)
					if got := lipgloss.Height(obs.ANSIScreen); got != size[1] {
						t.Fatalf("frame height = %d, want %d", got, size[1])
					}
					assertPolishFrameReading(t, obs.PlainScreen, state, size[0])
					for extension, content := range map[string]string{"ansi": obs.ANSIScreen, "txt": obs.PlainScreen} {
						if err := os.WriteFile(filepath.Join(root, name+"."+extension), []byte(content), 0644); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		}
	}
}

func assertPolishFrameReading(t *testing.T, screen, state string, width int) {
	t.Helper()
	if state == "help" {
		lines := strings.Split(screen, "\n")
		if strings.TrimSpace(lines[len(lines)-1]) == "" {
			t.Fatal("blank row below Help footer")
		}
		if width >= 60 {
			for _, action := range []string{"Capture", "Edit Task", "Toggle Today Plan", "Toggle Next Plan", "Start Task", "Complete Task", "Undo", "Search", "Peek"} {
				if !strings.Contains(screen, action) {
					t.Fatalf("first Help screen lost %q:\n%s", action, screen)
				}
			}
		}
	}
	if state == "capture-error" && !strings.Contains(screen, "@+3d)") {
		t.Fatal("capture repair examples were clipped")
	}
}
