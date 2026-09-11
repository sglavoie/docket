package tui

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	tea "charm.land/bubbletea/v2"
)

// The driver is a reimplementation of the bubbletea event loop, and this file is
// the one request it could not answer: the launch of an external editor.
//
// Everything else a command can ask for is either a message the app handles or a
// group of commands the driver unpacks. An editor is neither. tea.Exec returns a
// message the Program answers itself — it hands the command the terminal, waits
// for it, and sends whatever the callback makes of the result — so a driver that
// routed that message through Update would leave the editor unrun and the
// callback's message forever unsent. Every surface that opens an editor is then
// unreachable from a shell test, and the states only an editor's result can
// produce have to be asserted somewhere that is not the shell.
//
// So the driver performs that step here instead, and deliberately in two halves:
// the editor runs, and the message it produced is handed back rather than
// delivered. The gap between those halves is the one the terminal is really held
// open across, and it is where a test puts what happens to the workspace while
// the user is in their editor.

// externalEditorLaunch is what tea.Exec asks the runtime for: run this command
// with the terminal, then send what the callback makes of the outcome.
type externalEditorLaunch struct {
	command tea.ExecCommand
	finish  tea.ExecCallback
}

// The two field types tea.Exec's message carries. Both are exported; only the
// fields holding them are not.
var (
	execCommandType  = reflect.TypeOf((*tea.ExecCommand)(nil)).Elem()
	execCallbackType = reflect.TypeOf(tea.ExecCallback(nil))
)

// externalEditorRequest recognizes that request in a drained message.
//
// tea.Exec wraps its two arguments in an unexported struct and the Program
// answers it inside the loop this driver stands in for, so there is no exported
// way to name the message or to read it. The shape is matched structurally, the
// way commandGroupMessage matches a tea.Sequence, and only then are the two
// fields read out of an addressable copy of a value this process was already
// handed. A bubbletea whose shape changes stops matching here, and the caller
// fails by name rather than quietly skipping the editor.
func externalEditorRequest(msg tea.Msg) (externalEditorLaunch, bool) {
	value := reflect.ValueOf(msg)
	if value.Kind() != reflect.Struct || value.NumField() != 2 {
		return externalEditorLaunch{}, false
	}
	if value.Field(0).Type() != execCommandType || value.Field(1).Type() != execCallbackType {
		return externalEditorLaunch{}, false
	}
	addressable := reflect.New(value.Type()).Elem()
	addressable.Set(value)
	command, _ := readUnexportedField(addressable.Field(0)).(tea.ExecCommand)
	finish, _ := readUnexportedField(addressable.Field(1)).(tea.ExecCallback)
	if command == nil || finish == nil {
		return externalEditorLaunch{}, false
	}
	return externalEditorLaunch{command: command, finish: finish}, true
}

// readUnexportedField reads one field of an addressable struct whose name makes
// it unreadable from here. It is the standard reflect route and nothing more: the
// value is copied, never written, and the pointer is taken and converted in the
// one expression go vet's unsafeptr analysis recognizes.
func readUnexportedField(field reflect.Value) any {
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface()
}

// runPendingExternalEditor drains the command the last dispatch produced, stops
// at the external editor it asked for, runs that editor, and returns the message
// its callback produced — without delivering it.
//
// Not delivering it is the point. The Program sends that message the moment the
// editor exits, so a caller that wants the world to have changed while the editor
// was up has to act between the two, which is exactly what this split allows.
// Delivery is then the ordinary dispatch every other message takes.
func (d *UITestDriver) runPendingExternalEditor(t *testing.T) tea.Msg {
	t.Helper()
	queue := []tea.Cmd{d.pendingCmd}
	d.pendingCmd = nil
	for steps := 0; len(queue) > 0 && steps < maxDrainSteps; steps++ {
		cmd := queue[0]
		queue = queue[1:]
		if cmd == nil || isAnimationCmd(cmd) {
			continue
		}
		msg := cmd()
		launch, ok := externalEditorRequest(msg)
		if !ok {
			queue = append(queue, d.handleMsg(msg)...)
			continue
		}
		runExternalEditor(t, launch.command)
		// The callback is handed the error the Program would have: nil, once the
		// editor has exited cleanly and the terminal is back.
		return launch.finish(nil)
	}
	t.Fatal("the press asked for no external editor")
	return nil
}

// runExternalEditor stands in for the terminal the Program lends an editor: the
// streams are the test's rather than a tty. An editor that fails is a broken
// fixture here rather than a state under test, so it ends the test where it
// happened instead of arriving as a refusal three assertions later.
func runExternalEditor(t *testing.T, command tea.ExecCommand) {
	t.Helper()
	var output bytes.Buffer
	command.SetStdin(strings.NewReader(""))
	command.SetStdout(&output)
	command.SetStderr(&output)
	if err := command.Run(); err != nil {
		t.Fatalf("the editor the panel launched failed: %v\n%s", err, output.String())
	}
}
