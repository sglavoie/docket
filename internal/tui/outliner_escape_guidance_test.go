package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/outliner"
)

// Escape has one authority on the Outliner: the cascade EscSteps declares. These
// assertions are about the two things that makes true — an Esc entry in any
// profile names the act the next press performs rather than a literal of its
// own, and the acts it names survive the app's own minimum terminal.

// escGuidanceCase is one standing layer: how to reach it, the cascade act it
// owes, and how to tell it is still up.
type escGuidanceCase struct {
	name       string
	build      func(t *testing.T) *outliner.OutlinerModel
	wantAction string
	standing   func(*outliner.OutlinerModel) bool
}

func escGuidanceCases() []escGuidanceCase {
	return []escGuidanceCase{
		{
			name: "open Peek dock",
			build: func(t *testing.T) *outliner.OutlinerModel {
				out := escGuidanceOutliner(t)
				seatCursorOnFirstTaskRow(t, out)
				out.PeekMode = true
				return out
			},
			wantAction: "close Peek",
			standing:   func(out *outliner.OutlinerModel) bool { return out.PeekDockVisible() },
		},
		{
			name: "live range selection",
			build: func(t *testing.T) *outliner.OutlinerModel {
				out := escGuidanceOutliner(t)
				seatCursorOnFirstTaskRow(t, out)
				out.StartCursorRangeSelection()
				return out
			},
			wantAction: "freeze range",
			standing:   func(out *outliner.OutlinerModel) bool { return out.IsRangeSelectionActive() },
		},
		{
			name: "frozen selection",
			build: func(t *testing.T) *outliner.OutlinerModel {
				out := escGuidanceOutliner(t)
				seatCursorOnFirstTaskRow(t, out)
				out.ToggleCursorSelection()
				return out
			},
			wantAction: "clear selection",
			standing:   func(out *outliner.OutlinerModel) bool { return out.MultiSelectMode },
		},
		{
			name: "search projection",
			build: func(t *testing.T) *outliner.OutlinerModel {
				out := escGuidanceOutliner(t)
				out.SetSearchQuery("critical")
				return out
			},
			wantAction: "clear search",
			standing:   func(out *outliner.OutlinerModel) bool { return out.HasSearchProjection() },
		},
		{
			name: "search suspending a lens",
			build: func(t *testing.T) *outliner.OutlinerModel {
				out := lensLifecycleOutliner()
				applyLensOrFatal(t, out, waitingLens(lensTestID(1), "Waiting", "needle"))
				out.ActivateSearch()
				typeIntoOutlinerSearch(t, out, "beta")
				out.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
				return out
			},
			wantAction: "restore lens",
			standing:   func(out *outliner.OutlinerModel) bool { return out.HasSuspendedLens() },
		},
		{
			name: "active lens",
			build: func(t *testing.T) *outliner.OutlinerModel {
				out := lensLifecycleOutliner()
				applyLensOrFatal(t, out, waitingLens(lensTestID(2), "Waiting", "needle"))
				return out
			},
			wantAction: "leave lens",
			standing:   func(out *outliner.OutlinerModel) bool { return out.HasActiveLens() },
		},
		{
			name: "task Focus",
			build: func(t *testing.T) *outliner.OutlinerModel {
				out := escGuidanceOutliner(t)
				out.Focus.Stack = []outliner.FocusEntry{{TaskID: "critical"}}
				out.Population.Invalidate()
				out.RebuildRows()
				return out
			},
			wantAction: "leave Focus",
			standing:   func(out *outliner.OutlinerModel) bool { return len(out.Focus.Stack) > 0 },
		},
		{
			name: "section zoom",
			build: func(t *testing.T) *outliner.OutlinerModel {
				out := escGuidanceOutliner(t)
				zoomIntoSection(t, out, models.StateGroupBacklog.Label())
				return out
			},
			wantAction: "unzoom",
			standing:   func(out *outliner.OutlinerModel) bool { return out.Focus.Zoom.Active() },
		},
		{
			name: "project scope",
			build: func(t *testing.T) *outliner.OutlinerModel {
				out := lensLifecycleOutliner()
				out.PushProjectScope("alpha")
				out.Population.Invalidate()
				out.RebuildRows()
				return out
			},
			wantAction: "all projects",
			standing:   func(out *outliner.OutlinerModel) bool { return out.IsProjectScoped() },
		},
	}
}

// TestOutlinerEscGuidanceNamesTheCascadeAction holds every profile that spells
// an Esc entry to the cascade's own answer, and then presses the key to prove
// the sentence was not merely plausible: what the footer said is what the
// standing layer loses.
func TestOutlinerEscGuidanceNamesTheCascadeAction(t *testing.T) {
	for _, test := range escGuidanceCases() {
		t.Run(test.name, func(t *testing.T) {
			out := test.build(t)
			if !test.standing(out) {
				t.Fatalf("the fixture did not reach the %s layer", test.name)
			}

			action := out.EscAction()
			if action != test.wantAction {
				t.Fatalf("EscAction() = %q, want %q", action, test.wantAction)
			}
			if got, want := out.EscHint(), "Esc "+test.wantAction; got != want {
				t.Fatalf("EscHint() = %q, want %q", got, want)
			}
			if got := listsurface.NextEscAction(out.EscSteps()); got != action {
				t.Fatalf("EscAction() = %q, but the cascade's next act is %q", action, got)
			}

			spec, ok := out.ContextualGuidance()
			if !ok {
				t.Fatal("a standing layer produced no guidance")
			}
			entries := escGuidanceEntries(spec)
			if len(entries) == 0 {
				t.Fatalf("no profile entry names Esc: %q", guidanceEntryText(spec))
			}
			for _, entry := range entries {
				if !strings.HasSuffix(entry.text, action) {
					t.Errorf("Esc entry %q does not name the cascade act %q", entry.text, action)
				}
				if !entry.required {
					t.Errorf("Esc entry %q is optional; the way out of a state is required", entry.text)
				}
			}

			if !listsurface.RunEscCascade(out.EscSteps()) {
				t.Fatal("the cascade consumed nothing")
			}
			if test.standing(out) {
				t.Fatalf("the press the footer promised left the %s layer standing", test.name)
			}
		})
	}
}

// TestOutlinerRequiredGuidanceFitsMinimumTerminal renders every profile at the
// app's own floor. Required entries are reserved before any optional one, so a
// profile that loses a word there has promised a key the narrowest supported
// terminal cannot read.
func TestOutlinerRequiredGuidanceFitsMinimumTerminal(t *testing.T) {
	type namedProfile struct {
		name string
		spec chrome.GuidanceSpec
	}
	profiles := make([]namedProfile, 0, len(escGuidanceCases())+5)
	record := func(name string, spec chrome.GuidanceSpec) {
		profiles = append(profiles, namedProfile{name: name, spec: spec})
	}

	for _, test := range escGuidanceCases() {
		out := test.build(t)
		spec, ok := out.ContextualGuidance()
		if !ok {
			t.Fatalf("%s produced no guidance", test.name)
		}
		record(test.name, spec)
	}

	// The modal claims above the cascade own the key outright, so each names its
	// own act and each pays for it out of the same floor.
	browse := escGuidanceOutliner(t)
	seatCursorOnFirstTaskRow(t, browse)
	if spec, ok := browse.ContextualGuidance(); ok {
		record("task row", spec)
	}
	searching := escGuidanceOutliner(t)
	searching.SetSearchQuery("critical")
	searching.SetSearchActive(true)
	if spec, ok := searching.ContextualGuidance(); ok {
		record("search input", spec)
	}
	menu := escGuidanceOutliner(t)
	menu.MenuBar.Open = true
	if spec, ok := menu.ContextualGuidance(); ok {
		record("menu bar", spec)
	}
	readout := escGuidanceOutliner(t)
	readout.QueryReadout.Expanded = true
	if spec, ok := readout.ContextualGuidance(); ok {
		record("query readout", spec)
	}
	filtered := escGuidanceOutliner(t)
	filtered.SetSearchQuery("no such task")
	filtered.Population.Invalidate()
	filtered.RebuildRows()
	if spec, ok := filtered.ContextualGuidance(); ok {
		record("empty search", spec)
	}

	for _, profile := range profiles {
		// Styling is applied after packing, so the plain projection is what the
		// packer measured and the only thing a truncation would show up in.
		rendered := ansi.Strip(chrome.RenderGuidanceText(profile.spec, chrome.MinTerminalWidth))
		lines := strings.Split(rendered, "\n")
		budget := max(profile.spec.Rows, 1)
		if len(lines) > budget {
			t.Errorf("%s guidance took %d rows at width %d, want at most %d",
				profile.name, len(lines), chrome.MinTerminalWidth, budget)
		}
		for _, hint := range profile.spec.Hints {
			if !hint.Required {
				continue
			}
			entry := chrome.GuidanceHintText(hint)
			if !strings.Contains(rendered, entry) {
				t.Errorf("%s guidance truncated the required entry %q at width %d: %q",
					profile.name, entry, chrome.MinTerminalWidth, rendered)
			}
		}
	}
}

type escGuidanceEntry struct {
	text     string
	required bool
}

// escGuidanceEntries collects the profile entries whose key spells Esc. A
// combined entry — Peek's own key beside the cascade's — is one of them, which
// is why the assertion is on the tail rather than on the whole text.
func escGuidanceEntries(spec chrome.GuidanceSpec) []escGuidanceEntry {
	entries := make([]escGuidanceEntry, 0, 2)
	for _, hint := range spec.Hints {
		if !strings.Contains(hint.Key, "Esc") {
			continue
		}
		entries = append(entries, escGuidanceEntry{
			text:     chrome.GuidanceHintText(hint),
			required: hint.Required,
		})
	}
	return entries
}

func escGuidanceOutliner(t *testing.T) *outliner.OutlinerModel {
	t.Helper()
	return newFilterChordTestApp(t).crossOutliner
}

func seatCursorOnFirstTaskRow(t *testing.T, out *outliner.OutlinerModel) {
	t.Helper()
	for index, row := range out.Rows {
		if row.IsTaskRow() {
			out.Cursor = index
			return
		}
	}
	t.Fatal("the fixture exposed no task row")
}
