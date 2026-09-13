package tui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// Optional ANSI captures are review artifacts; the layout and visible controls
// remain asserted in CI even when no artifact directory was requested.
func TestCheckInVisualAcceptance(t *testing.T) {
	defer theme.ApplyTheme(theme.ThemeConfig{})
	for _, variant := range []string{"dark", "light"} {
		for _, size := range [][2]int{{110, 35}, {62, 24}, {60, 20}} {
			t.Run(fmt.Sprintf("%s-%dx%d", variant, size[0], size[1]), func(t *testing.T) {
				driver, ws, err := NewFixtureDriver("check-ins", size[0], size[1])
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = ws.Cleanup() }()
				theme.ApplyTheme(theme.ThemeConfig{Name: variant})
				capture := func(name string, required ...string) {
					t.Helper()
					obs := driver.Observe(ObserveOptions{IncludeANSI: true})
					if len(obs.LayoutIssues) > 0 {
						t.Fatalf("%s: %+v\n%s", name, obs.LayoutIssues, obs.PlainScreen)
					}
					for _, text := range required {
						if !strings.Contains(obs.PlainScreen, text) {
							t.Fatalf("%s lacks %q\n%s", name, text, obs.PlainScreen)
						}
					}
					var mono bytes.Buffer
					writer := colorprofile.Writer{Forward: &mono, Profile: colorprofile.ASCII}
					if _, err := writer.WriteString(obs.ANSIScreen); err != nil {
						t.Fatal(err)
					}
					if ansi.Strip(mono.String()) != obs.PlainScreen {
						t.Fatal("monochrome output changed content")
					}
					if root := os.Getenv("TT_CHECK_IN_REVIEW_DIR"); root != "" {
						if err := os.MkdirAll(root, 0700); err != nil {
							t.Fatal(err)
						}
						path := filepath.Join(root, fmt.Sprintf("%s-%dx%d-%s.ansi", variant, size[0], size[1], name))
						if err := os.WriteFile(strings.TrimSuffix(path, ".ansi")+"-mono.ansi", mono.Bytes(), 0600); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path, []byte(obs.ANSIScreen), 0600); err != nil {
							t.Fatal(err)
						}
					}
				}
				capture("rows", "Migrated", "Apr 1 15:30", "today 0")
				out := driver.App().crossOutliner
				var id taskref.TaskIdentity
				for _, row := range out.Rows {
					if row.Task != nil && row.Task.Title == "Migrated vendor wait" {
						id = out.TaskIdentityForRow(row)
						break
					}
				}
				if !out.MoveCursorToIdentity(id) {
					t.Fatal("no migrated task")
				}
				out.ReplaceSelection([]taskref.TaskIdentity{id})
				capture("selected", "Migrated", "Apr 1 15:30")
				if _, err := driver.ApplyAction(UITestAction{Type: UITestActionKey, Key: "esc"}, ObserveOptions{}); err != nil {
					t.Fatal(err)
				}
				if !out.MoveCursorToIdentity(id) {
					t.Fatal("cursor lost after selection")
				}
				if _, err := driver.ApplyAction(UITestAction{Type: UITestActionCommand, Command: "Set Check-In"}, ObserveOptions{}); err != nil {
					t.Fatal(err)
				}
				capture("editor", "Next check-in:", "Repeat:", "2020-04-01")
				if strings.Count(driver.Render(), "Esc cancel") != 1 {
					t.Fatal("editor repeats its Escape guidance")
				}
				if _, err := driver.ApplyAction(UITestAction{Type: UITestActionKey, Key: "esc"}, ObserveOptions{}); err != nil {
					t.Fatal(err)
				}
				if _, err := driver.ApplyAction(UITestAction{Type: UITestActionCommand, Command: "Set up check-ins"}, ObserveOptions{}); err != nil {
					t.Fatal(err)
				}
				capture("setup", "Setup active work", "Next check-in:", "Ctrl+N skip", "Esc exit")
				if strings.Contains(driver.Render(), "Esc cancel") {
					t.Fatal("setup advertises a conflicting Escape action")
				}
			})
		}
	}
}
