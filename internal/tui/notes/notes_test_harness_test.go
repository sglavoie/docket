package notes

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/textfit"
)

// This file holds the fixtures the homed Notes tests share. Each one is a
// deliberate copy of a shell helper rather than an export, on the same terms as
// internal/tui/listsurface: a test helper exported to cross a package boundary
// is production-unused API, which scripts/dead_code.sh rejects. The shell keeps
// its own copies for the migration, guidance, and hub tests that stay root
// integration tests because they drive more than this surface.

// notesTestShell is the half of the shell a mounted Notes manager can see: a
// registry carrying the real descriptors and the real grammar, an executor, and
// the one key path. Only the operation handler is a stand-in, and it is the
// shell's own switch minus the action sheet, which is shell chrome this package
// does not own. Opening the sheet is recorded instead.
type notesTestShell struct {
	registry     *operation.OperationRegistry
	executor     *operation.OperationExecutor
	view         *NotesViewModel
	pendingChord []string

	actionSheetOpened bool
}

// mountNotesManager gives a standalone Notes manager the registry dispatch
// production hands it. The browse grammar resolves through that seam, so an
// unmounted view answers none of its keys.
func mountNotesManager(t *testing.T, view *NotesViewModel) *notesTestShell {
	t.Helper()
	registry := operation.NewOperationRegistry()
	descriptors := operation.CoreInputOperationDescriptors()
	descriptors = append(descriptors, NoteOperationDescriptors()...)
	descriptors = append(descriptors, NotesSurfaceOperationDescriptors()...)
	for _, descriptor := range descriptors {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("register %s: %v", descriptor.ID, err)
		}
	}
	if err := registry.RegisterInputStrategy(NotesInputStrategy()); err != nil {
		t.Fatalf("register the Notes input strategy: %v", err)
	}

	shell := &notesTestShell{registry: registry, view: view}
	shell.executor = operation.NewOperationExecutor(registry)
	views := []presentation.PresentationID{presentation.ViewNotes, presentation.ViewGlobalNotes}
	bind := func(id operation.OperationID) {
		if err := shell.executor.Register(operation.OperationBinding{
			OperationID:              id,
			Views:                    views,
			RequiredCapabilities:     operation.NewCapabilitySet(operation.CapabilityNotesManager),
			Handler:                  shell.execute,
			DelegateTargetResolution: true,
			DelegateValueProvider:    true,
			DelegateConfirmation:     true,
			DelegateTransaction:      true,
		}); err != nil {
			t.Fatalf("bind %s: %v", id, err)
		}
	}
	for _, descriptor := range NotesSurfaceOperationDescriptors() {
		bind(descriptor.ID)
	}
	for _, id := range NotesHostedNoteOperations() {
		bind(id)
	}

	view.SetOperationSystem(registry, shell.dispatch)
	return shell
}

// dispatch mirrors (*AppModel).dispatchOperationKey: chord bookkeeping, then one
// execution, then the executor's message unwrapped back into a command.
func (s *notesTestShell) dispatch(
	strategyID operation.InputStrategyID,
	sequence []string,
	ctx operation.CommandContext,
) (bool, tea.Cmd) {
	if len(sequence) > 1 && operation.NormalizeOperationKey(sequence[len(sequence)-1]) == "esc" {
		s.pendingChord = nil
		return true, nil
	}
	binding, _, resolution := s.registry.InputBindingForSequence(strategyID, sequence, ctx)
	switch resolution {
	case operation.ChordPending:
		s.pendingChord = sequence
		return true, nil
	case operation.ChordNoMatch:
		s.pendingChord = nil
		return true, nil
	case operation.ChordTerminal:
	}
	s.pendingChord = nil

	result := s.executor.Execute(operation.OperationExecutionRequest{
		ID:       binding.OperationID,
		Context:  ctx,
		Value:    binding.PresetValue,
		HasValue: binding.HasPresetValue,
	})
	switch typed := result.AsMessage().(type) {
	case nil:
		return true, nil
	case operation.OperationCommandResultMsg:
		return true, typed.Cmd
	default:
		return true, func() tea.Msg { return typed }
	}
}

//nolint:exhaustive // Only the Notes grammar's operations are bound to this handler.
func (s *notesTestShell) execute(invocation operation.OperationInvocation) operation.OperationHandlerResult {
	command := func(cmd tea.Cmd) operation.OperationHandlerResult {
		return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{Cmd: cmd}}
	}
	switch invocation.Descriptor.ID {
	case OperationNotesCreate:
		return command(s.view.BeginDestinationPicker())
	case OperationNotesSearch:
		return command(s.view.BeginSearch())
	case OperationNotesScope:
		return command(s.view.BeginScopePicker())
	case OperationNotesActions:
		s.actionSheetOpened = true
	case OperationNoteOpen:
		return command(s.view.EditCurrentNote())
	case OperationNoteMove:
		return command(s.view.BeginMovePicker())
	case OperationNoteRename:
		s.view.StartRename()
	case OperationNoteDelete:
		s.view.DeleteCurrentNote()
	}
	return operation.OperationHandlerResult{}
}

// combinedNotesService is the in-memory note collection the manager tests load
// from. It counts queries so a test can prove a rebuild happened, and retains
// every batch so a test can prove what a mutation asked for.
type combinedNotesService struct {
	byLocation  map[app.NoteLocation][]*models.Note
	errors      map[app.NoteLocation]error
	snapshotErr error
	queryCount  int
	requests    []app.NoteBatchRequest
}

func (s *combinedNotesService) NotesAtLocations(locations []app.NoteLocation) ([]app.NoteCollectionResult, error) {
	s.queryCount++
	if s.snapshotErr != nil {
		return nil, s.snapshotErr
	}
	results := make([]app.NoteCollectionResult, len(locations))
	for i, location := range locations {
		results[i] = app.NoteCollectionResult{
			Location: location,
			Notes:    append([]*models.Note(nil), s.byLocation[location]...),
			Err:      s.errors[location],
		}
	}
	return results, nil
}

func (s *combinedNotesService) ApplyNoteBatch(request app.NoteBatchRequest) (app.NoteBatchResult, error) {
	s.requests = append(s.requests, request)
	return app.NoteBatchResult{}, nil
}

// withRenderAudit and normalizeScreen mirror internal/tui's
// uitest_render_audit_helpers_test.go and uitest_screen_test.go, reduced to what
// a single view's render needs. The shell's pair reports UITestIssue values, a
// root type this package sits below, so a copy that reports plain strings is the
// only shape available here; the checks themselves are the same three.
func withRenderAudit(render func() string) (string, []string) {
	var issues []string
	prev := textfit.RequiredTextTruncationSink
	textfit.RequiredTextTruncationSink = func(context, full, shown string) {
		issues = append(issues, fmt.Sprintf("%s: required text truncated: %q -> %q", context, full, shown))
	}
	defer func() { textfit.RequiredTextTruncationSink = prev }()
	return render(), issues
}

func normalizeScreen(rawContent string, width, height int) (string, []string) {
	var issues []string
	if !utf8.ValidString(rawContent) {
		issues = append(issues, "screen contains invalid UTF-8")
	}
	plain := ansi.Strip(rawContent)
	lines := strings.Split(plain, "\n")
	if height > 0 && len(lines) > height {
		issues = append(issues, fmt.Sprintf("rendered %d lines exceeds terminal height %d", len(lines), height))
	}
	if width > 0 {
		for i, line := range lines {
			if w := lipgloss.Width(line); w > width {
				issues = append(issues,
					fmt.Sprintf("line %d visual width %d exceeds terminal width %d", i+1, w, width))
			}
		}
	}
	return plain, issues
}

// guidanceEntryText, assertGuidanceContains and assertRequiredGuidance mirror
// internal/tui/outliner_guidance_test.go and projects_notes_guidance_test.go.
// The shell keeps its own copies for the guidance test that walks every surface.
func guidanceEntryText(spec chrome.GuidanceSpec) string {
	parts := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		parts = append(parts, chrome.GuidanceHintText(hint))
	}
	return strings.Join(parts, " · ")
}

func assertGuidanceContains(t *testing.T, state string, spec chrome.GuidanceSpec, ok bool, want ...string) {
	t.Helper()
	if !ok {
		t.Fatalf("%s guidance is unavailable", state)
	}
	got := guidanceEntryText(spec)
	for _, entry := range want {
		if !strings.Contains(got, entry) {
			t.Errorf("%s guidance = %q, want %q", state, got, entry)
		}
	}
}

func assertRequiredGuidance(t *testing.T, state string, spec chrome.GuidanceSpec, ok bool, want ...string) {
	t.Helper()
	assertGuidanceContains(t, state, spec, ok, want...)
	for _, expected := range want {
		found := false
		for _, hint := range spec.Hints {
			if chrome.GuidanceHintText(hint) == expected && hint.Required {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s guidance does not require %q: %+v", state, expected, spec.Hints)
		}
	}
}

// syntheticKey, syntheticSpecialKey and syntheticCtrlKey mirror
// internal/tui/key_helpers_test.go.
func syntheticKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func syntheticSpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func syntheticCtrlKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl}
}
