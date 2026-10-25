package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/tui/grammar"
)

// Assert terminal coordinates, not just panel order: a footer that changes
// height must not move the editor when suggestions open or feedback expires.
func TestCaptureEditorStaysAtTerminalBottom(t *testing.T) {
	for _, size := range [][2]int{{60, 20}, {80, 24}, {100, 35}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			app := typedTestApp(t)
			app.handleWindowSize(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			app.handleViewMessages(app.captureComposerMessage(app.currentProject, app.taskWorkspaceProjects()))
			composer := app.taskComposer
			composer.TagCache = []string{"release", "review", "ready"}

			check := func(lastLine, detail string) {
				t.Helper()
				app.syncShellFrameMetrics()
				app.syncHostedPanelHost()
				content, issues := withRenderAudit(func() string { return app.View().Content })
				if len(issues) != 0 {
					t.Fatalf("layout issues: %+v", issues)
				}
				plain := ansi.Strip(content)
				lines := strings.Split(plain, "\n")
				if len(lines) != size[1] || strings.TrimSpace(lines[len(lines)-1]) != lastLine {
					t.Fatalf("want editor %q on terminal row %d:\n%s", lastLine, size[1], plain)
				}
				if detail != "" && !strings.Contains(strings.Join(lines[:len(lines)-1], "\n"), detail) {
					t.Fatalf("missing detail %q above editor:\n%s", detail, plain)
				}
			}
			setDraft := func(value string) {
				composer.Input.Reset()
				composer.Update(tea.PasteMsg{Content: value})
			}

			check("Type a task title…", "Enter create")
			setDraft("Anchor task")
			check("Anchor task", "Task     Anchor task")
			for _, r := range " #re" {
				app.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
				check(strings.TrimSpace(composer.Input.Value()), "Project: Test Project")
			}
			if !composer.IsAutocompleteActive() {
				t.Fatal("typing a tag did not open completions")
			}
			app.Update(tea.KeyPressMsg{Code: tea.KeyTab})
			check("Anchor task #release", "Tags     #release")
			setDraft("Anchor task !2 #release ")
			check("Anchor task !2 #release", "Tags     #release")
			setDraft("Anchor task !2 #release #re")
			if !composer.IsAutocompleteActive() {
				t.Fatal("tag completion did not open")
			}
			check("Anchor task !2 #release #re", "#review")
			app.feedback.message = "Task saved"
			check("Anchor task !2 #release #re", "Task saved")
			app.feedback.message = ""
			check("Anchor task !2 #release #re", "Tab/Enter accept")
			composer.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
			check("Anchor task !2 #release #re", "Enter create")

			setDraft("First task\nLast task !2 #release ")
			check("Last task !2 #release", "First task")
			composer.HelpExpanded = true
			check("Last task !2 #release", "close syntax")
			composer.HelpExpanded = false
			check("Last task !2 #release", "2 tasks")

			setDraft(strings.Repeat("wrapped words ", 20) + "WRAP-END")
			// A wrapped line may have other words on its last visual row.
			app.syncShellFrameMetrics()
			app.syncHostedPanelHost()
			wrapped := strings.Split(ansi.Strip(app.View().Content), "\n")
			if len(wrapped) != size[1] || !strings.HasSuffix(strings.TrimSpace(wrapped[len(wrapped)-1]), "WRAP-END") {
				t.Fatalf("wrapped draft is not anchored to the bottom:\n%s", strings.Join(wrapped, "\n"))
			}

			setDraft(strings.Repeat("Earlier task\n", 30) + "Final task $")
			check("Final task $", "$inbox")
			app.feedback.message = "Task saved"
			check("Final task $", "Task saved")
			app.feedback.message = ""
			for _, resized := range [][2]int{{60, 20}, {100, 35}} {
				size = resized
				app.handleWindowSize(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				check("Final task $", "$inbox")
			}
			composer.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
			composer.Errors = []grammar.LineError{{Line: 31, Message: "Choose a destination"}}
			check("Final task $", "Choose a destination")
		})
	}
}
