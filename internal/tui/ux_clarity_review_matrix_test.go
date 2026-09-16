package tui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// This file is the review matrix the UX clarity refinement is judged against:
// one staged state per thing the plan changes, rendered at every reviewed size
// in every reviewed color rendering, exported as frames a human can read.
//
// It is deliberately separate from TestUXRefinementPresentationMatrix. That pass
// answers questions about the relationship between two renderings of the
// finished eighteen-session work and owns no artifacts; this one records what
// the product looks like *before* the clarity sessions and hands the frames to
// a reviewer. Both read the same `ux-acceptance` fixture, because a second
// fixture would make the two passes incomparable.

// uxClarityReviewDirEnv names the artifact directory the caller wants the frames
// written to. It is optional: with nothing set the export still runs, into the
// test's own temporary directory, so the writer is exercised on every CI run and
// a caller asking for the frames only relocates them.
const uxClarityReviewDirEnv = "TT_UX_CLARITY_REVIEW_DIR"

// uxClarityReviewManifest is the index written beside the frames. It is what
// makes the export reviewable rather than a directory of screens: one row per
// frame, naming the state, the session that will deliberately replace what the
// frame shows, and the layout measurements taken while it was rendered.
const uxClarityReviewManifest = "frames.tsv"

// The color renderings the review covers. Dark and light are palettes the app
// applies; monochrome is not a third palette but what a terminal without color
// does to either of them, so it is derived from each rendered frame rather than
// configured. Deriving it is the point: it proves the two palettes carry the
// same content, which a separately configured third palette could not.
var uxClarityReviewVariants = []string{"dark", "light"}

// uxClarityFrame is one staged state of the product. `records` is what the frame
// is evidence of and `owner` the session that replaces it, both written into the
// manifest so the exported directory explains itself.
type uxClarityFrame struct {
	name    string
	records string
	owner   int
	stage   func(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel)
}

// uxClarityFrames is the coverage list. The first entry is the resting view
// every other frame is read against; the rest are the combinations the existing
// `ux-acceptance` coverage had no case for — feedback and guidance owed at once,
// a modal standing on another modal's instructions, every context axis true
// together, and capture raised over Peek at the app's own floor.
var uxClarityFrames = []uxClarityFrame{
	{
		name:    "launch",
		records: "the resting Outliner: one header row, no feedback, the surface's own footer ranks",
		owner:   5,
		stage:   func(*testing.T, *UITestDriver, *outliner.OutlinerModel) {},
	},
	{
		name:    "feedback-over-guidance",
		records: "a routine success standing where the capture controls were",
		owner:   4,
		stage:   stageUXClarityFeedbackOverGuidance,
	},
	{
		name:    "picker-over-outline",
		records: "a value picker owning the one instruction set over a quiet host Outliner",
		owner:   3,
		stage:   stageUXClarityPickerOverOutline,
	},
	{
		name:    "nested-discard-prompt",
		records: "a discard confirmation over a capture draft over the outline",
		owner:   3,
		stage:   stageUXClarityNestedDiscardPrompt,
	},
	{
		name:    "combined-context",
		records: "selection, search, filters and sort competing for the one header row",
		owner:   5,
		stage:   stageUXClarityCombinedContext,
	},
	{
		name:    "capture-over-peek",
		records: "capture raised over an open Peek dock of fixed height",
		owner:   7,
		stage:   stageUXClarityCaptureOverPeek,
	},
}

// uxClarityFrameRecord is one manifest row.
type uxClarityFrameRecord struct {
	variant string
	width   int
	height  int
	frame   uxClarityFrame
	// The measurements a later session changes the arithmetic of. They are read
	// off the live model and the composed frame rather than off a constant, so a
	// row that stops matching the product is a row that moved.
	headerRows   int
	contentRows  int
	peekDockRows int
	footerRows   int
	clippedLines int
}

// TestUXClarityReviewMatrix renders every frame at every reviewed size in every
// reviewed color rendering, checks the three things a frame has to satisfy to be
// worth reviewing at all, and exports it.
//
// The checks are deliberately about the frame rather than about its text: that
// it fits the terminal it claims (in display cells, which a rune count cannot
// answer), that removing color removes no content, and that rendering it twice
// produces the same bytes — a frame that does not is a frame no reviewer and no
// later diff can trust.
func TestUXClarityReviewMatrix(t *testing.T) {
	defer theme.ApplyTheme(theme.ThemeConfig{})
	root := uxClarityReviewRoot(t)
	records := make([]uxClarityFrameRecord, 0, len(uxClarityReviewVariants)*len(uxPresentationMatrixSizes)*len(uxClarityFrames))

	for _, variant := range uxClarityReviewVariants {
		for _, size := range uxPresentationMatrixSizes {
			for _, frame := range uxClarityFrames {
				name := fmt.Sprintf("%s-%dx%d-%s", variant, size[0], size[1], frame.name)
				t.Run(name, func(t *testing.T) {
					// One driver per frame: the states below are reached by real key
					// presses and several of them cannot be left without spending the
					// Esc cascade a later frame would then be measuring.
					driver, workspace, err := NewFixtureDriver(FixtureUXAcceptance, size[0], size[1])
					if err != nil {
						t.Fatalf("fixture: %v", err)
					}
					t.Cleanup(func() { _ = workspace.Cleanup() })
					theme.ApplyTheme(theme.ThemeConfig{Name: variant})
					view := driver.App().crossOutliner
					if view == nil {
						t.Fatal("the fixture app has no Outliner")
					}

					frame.stage(t, driver, view)

					obs := driver.Observe(ObserveOptions{IncludeANSI: true})
					assertUXScreenFitsItsTerminal(t, obs, size)
					assertUXMonochromeKeepsContent(t, obs)
					mono := uxClarityMonochrome(t, obs.ANSIScreen)

					if again := driver.Observe(ObserveOptions{IncludeANSI: true}); again.ANSIScreen != obs.ANSIScreen {
						t.Fatalf("%s renders differently the second time, so no frame of it is reviewable\n%s",
							frame.name, again.PlainScreen)
					}

					record := uxClarityFrameRecord{
						variant:      variant,
						width:        size[0],
						height:       size[1],
						frame:        frame,
						headerRows:   lipgloss.Height(view.BuildOutlinerTitleLine(size[0])),
						contentRows:  view.ContentHeight(),
						peekDockRows: len(view.PeekDockLines()),
						footerRows:   lipgloss.Height(driver.App().renderStatusBar()),
						clippedLines: len(obs.ClippedLines),
					}
					records = append(records, record)
					uxClarityExportFrame(t, root, name, obs.ANSIScreen, obs.PlainScreen, mono)
				})
			}
		}
	}

	if len(records) == 0 {
		t.Fatal("the matrix exported nothing, so it is not reaching the product")
	}
	uxClarityWriteManifest(t, root, records)
	t.Logf("UX clarity review frames: %s (set %s to keep them)", root, uxClarityReviewDirEnv)
}

// uxClarityReviewRoot resolves the artifact directory. A caller that named one
// owns it; a caller that named none gets the test's own, which is removed with
// the test and keeps the writer on every run rather than only on a reviewer's.
func uxClarityReviewRoot(t *testing.T) string {
	t.Helper()
	if named := strings.TrimSpace(os.Getenv(uxClarityReviewDirEnv)); named != "" {
		if err := os.MkdirAll(named, 0o700); err != nil {
			t.Fatalf("create %s=%q: %v", uxClarityReviewDirEnv, named, err)
		}
		return named
	}
	return t.TempDir()
}

// uxClarityMonochrome projects a styled frame through the ASCII color profile,
// which is what a terminal with no color does to it.
func uxClarityMonochrome(t *testing.T, styled string) string {
	t.Helper()
	var buffer bytes.Buffer
	writer := colorprofile.Writer{Forward: &buffer, Profile: colorprofile.ASCII}
	if _, err := writer.WriteString(styled); err != nil {
		t.Fatalf("project the frame through the ASCII profile: %v", err)
	}
	return buffer.String()
}

// uxClarityExportFrame writes the three renderings of one frame. The plain text
// is written beside the two ANSI captures because a reviewer diffing two runs
// wants the content without the escapes, and a reviewer looking at the frame
// wants the escapes.
func uxClarityExportFrame(t *testing.T, root, name, styled, plain, mono string) {
	t.Helper()
	files := map[string]string{
		name + ".ansi":      styled,
		name + "-mono.ansi": mono,
		name + ".txt":       plain,
	}
	for file, content := range files {
		path := filepath.Join(root, file)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

// uxClarityWriteManifest writes the index. Tab-separated and one row per frame,
// so the measurements can be read by eye and diffed between two runs of the
// matrix — which is how a session proves it moved only the arithmetic it meant to.
func uxClarityWriteManifest(t *testing.T, root string, records []uxClarityFrameRecord) {
	t.Helper()
	var b strings.Builder
	b.WriteString("frame\tvariant\twidth\theight\treplaced_by_session\theader_rows\tcontent_rows\tpeek_dock_rows\tfooter_rows\tclipped_lines\trecords\n")
	for _, record := range records {
		fmt.Fprintf(&b, "%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s\n",
			record.frame.name, record.variant, record.width, record.height, record.frame.owner,
			record.headerRows, record.contentRows, record.peekDockRows, record.footerRows,
			record.clippedLines, record.frame.records)
	}
	path := filepath.Join(root, uxClarityReviewManifest)
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// The staging functions below are the frames' only definition of what each state
// is. The combined-state assertions in ux_clarity_baseline_test.go call the same
// functions, so the screens exported for review are exactly the screens the
// contracts are written against rather than a second approximation of them.

// uxClarityPress replays one production action and fails on the first refusal.
func uxClarityPress(t *testing.T, driver *UITestDriver, actions ...UITestAction) {
	t.Helper()
	for index, action := range actions {
		if _, err := driver.ApplyAction(action, ObserveOptions{}); err != nil {
			t.Fatalf("action %d (%s %q%q): %v", index, action.Type, action.Key, action.Command, err)
		}
	}
}

func uxClarityKeys(keys ...string) []UITestAction {
	actions := make([]UITestAction, 0, len(keys))
	for _, key := range keys {
		actions = append(actions, UITestAction{Type: UITestActionKey, Key: key})
	}
	return actions
}

// stageUXClarityFeedbackOverGuidance raises a routine success and then opens the
// capture panel, which is the one surface whose controls are contextual
// guidance. The copy comes first because it is the message that has to survive
// into the next presentation to put the two in the footer at once.
func stageUXClarityFeedbackOverGuidance(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel) {
	t.Helper()
	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
	uxClarityPress(t, driver, uxClarityKeys("y", "a")...)
}

// stageUXClarityPickerOverOutline opens the tags-and-context picker over a task
// row. It is the ordinary shell-mounted value picker: an overlay with its own
// instructions, composited over a host that keeps rendering its own.
func stageUXClarityPickerOverOutline(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel) {
	t.Helper()
	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
	uxClarityPress(t, driver, uxClarityKeys("#")...)
}

// stageUXClarityNestedDiscardPrompt puts a confirmation over a draft-bearing
// panel over the outline: three presentations, two of which are asking for a
// key. It is the nesting the plan's "one active instruction set" has to answer.
func stageUXClarityNestedDiscardPrompt(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel) {
	t.Helper()
	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
	uxClarityPress(t, driver,
		UITestAction{Type: UITestActionKey, Key: "a"},
		UITestAction{Type: UITestActionText, Text: uxClarityDraftTitle},
		UITestAction{Type: UITestActionKey, Key: "esc"},
	)
}

// uxClarityDraftTitle is the capture draft the discard prompt is raised over. It
// is short on purpose: the frame is about the prompt, and a title long enough to
// wrap would move the panel's own rows between two sizes.
const uxClarityDraftTitle = "Draft"

// stageUXClarityCombinedContext turns on every context axis the header can
// carry at once — a narrowing search, an explicit sort, a facet filter, and a
// visual selection — over a projection that still draws a row. The order is the
// one a user can actually reach: the search installs the projection, the sort
// and the filter narrow it, and the selection is taken last because it is the
// only axis a rebuild drops.
func stageUXClarityCombinedContext(t *testing.T, driver *UITestDriver, _ *outliner.OutlinerModel) {
	t.Helper()
	uxClarityPress(t, driver,
		UITestAction{Type: UITestActionKey, Key: "/"},
		UITestAction{Type: UITestActionText, Text: uxClaritySearchQuery},
		UITestAction{Type: UITestActionKey, Key: "enter"},
		UITestAction{Type: UITestActionCommand, Command: "Sort by Priority"},
	)
	uxClarityPress(t, driver, uxClarityKeys("f", "=", "v")...)
}

// uxClaritySearchQuery matches every fixture title through a word none of them
// can lose, so the narrowing is real at every width and the result is stable.
const uxClaritySearchQuery = "the"

// stageUXClarityCaptureOverPeek opens the dock and then raises capture over it,
// which is the combination the app's own floor has the least room for.
func stageUXClarityCaptureOverPeek(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel) {
	t.Helper()
	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
	uxClarityPress(t, driver, uxClarityKeys("b")...)
	if !view.PeekDockVisible() {
		t.Fatal("b did not open the Peek dock")
	}
	uxClarityPress(t, driver, uxClarityKeys("a")...)
}

// uxClarityStagedDriver launches the fixture at one size and stages one named
// frame on it, which is what every combined-state assertion starts from.
func uxClarityStagedDriver(t *testing.T, frame string, width, height int) (*UITestDriver, *outliner.OutlinerModel) {
	t.Helper()
	staged, ok := uxClarityFrameNamed(frame)
	if !ok {
		t.Fatalf("no review frame named %q", frame)
	}
	driver, workspace, err := NewFixtureDriver(FixtureUXAcceptance, width, height)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	view := driver.App().crossOutliner
	if view == nil {
		t.Fatal("the fixture app has no Outliner")
	}
	staged.stage(t, driver, view)
	return driver, view
}

func uxClarityFrameNamed(name string) (uxClarityFrame, bool) {
	for _, frame := range uxClarityFrames {
		if frame.name == name {
			return frame, true
		}
	}
	return uxClarityFrame{}, false
}

// uxClarityFooter is the shell-composed bottom bar with its styling removed,
// which is where feedback and guidance either coexist or replace one another.
func uxClarityFooter(driver *UITestDriver) string {
	return ansi.Strip(driver.App().renderStatusBar())
}
