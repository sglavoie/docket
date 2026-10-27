package tui

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// The `$EDITOR` handoff (Ctrl+E) runs through tea.ExecProcess, and the terminal
// must not report clicks into the editor. Docket writes no code for that: the
// renderer resets every mouse mode when it stops for an exec and re-enables the
// last view's mode when it starts again. This test pins that dependency
// behavior, so an upgrade that drops it fails here rather than in an editor.

type mouseExecProbeCommand struct{}

func (mouseExecProbeCommand) Run() error          { return nil }
func (mouseExecProbeCommand) SetStdin(io.Reader)  {}
func (mouseExecProbeCommand) SetStdout(io.Writer) {}
func (mouseExecProbeCommand) SetStderr(io.Writer) {}

type mouseExecDoneMsg struct{}

type mouseExecProbeModel struct{ execed bool }

func (m *mouseExecProbeModel) Init() tea.Cmd { return nil }

func (m *mouseExecProbeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.WindowSizeMsg:
		if !m.execed {
			m.execed = true
			return m, tea.Exec(mouseExecProbeCommand{}, func(error) tea.Msg { return mouseExecDoneMsg{} })
		}
	case mouseExecDoneMsg:
		return m, tea.Quit
	}
	return m, nil
}

func (m *mouseExecProbeModel) View() tea.View {
	view := tea.NewView("probe")
	view.MouseMode = tea.MouseModeCellMotion
	return view
}

func TestExecReleasesMouseReportingAndRestoresIt(t *testing.T) {
	var out bytes.Buffer
	program := tea.NewProgram(&mouseExecProbeModel{},
		tea.WithInput(nil), tea.WithOutput(&out), tea.WithWindowSize(40, 5))
	done := make(chan error, 1)
	go func() { _, err := program.Run(); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(10 * time.Second):
		program.Kill()
		t.Fatal("the probe program never quit")
	}

	written := out.String()
	enable := ansi.SetModeMouseButtonEvent
	reset := ansi.ResetModeMouseButtonEvent
	first := strings.Index(written, enable)
	if first < 0 {
		t.Fatalf("the renderer never enabled mouse reporting: %q", written)
	}
	released := strings.Index(written[first:], reset)
	if released < 0 {
		t.Fatalf("mouse reporting was never released for the exec: %q", written)
	}
	if !strings.Contains(written[first+released:], enable) {
		t.Fatalf("mouse reporting was not restored after the exec: %q", written)
	}
}
