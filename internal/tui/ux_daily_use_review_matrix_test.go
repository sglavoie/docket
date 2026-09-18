package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// This file is the review matrix the daily-use refinement is judged against. It
// is the third such matrix in the repository and deliberately its own: the
// eighteen-session plan and the eight-session clarity plan each own theirs, and
// a frame list shared between three independently numbered plans would make
// every "replaced by session N" ambiguous.
//
// It differs from the clarity matrix in one structural way, which is the whole
// reason it exists rather than extending that one. The clarity frames all read
// `ux-acceptance`, because that pass was about states that are only
// simultaneously true in one fixture. This plan changes row geometry, project
// identity, capture growth, and startup focus, and three of those are invisible
// in a fixture whose project names all truncate to one width. So a frame names
// its own fixture, and the four fixtures this session was told to reuse —
// `ux-acceptance`, `narrow`, `multi-project`, and `release` — each appear where
// they are the only one that can show the thing.
//
// Every frame here records current behavior. Nothing in this file asserts what
// a later session will do; `ux_daily_use_baseline_test.go` is where the
// expectations later sessions deliberately rewrite are stated, and
// `docs/tui-testing/ux-daily-use-regression-map.md` is where each one is
// assigned an owner.

// uxDailyUseReviewDirEnv names the artifact directory the caller wants the
// frames written to. Optional, on the clarity matrix's terms: with nothing set
// the export still runs into the test's own temporary directory, so the writer
// is exercised on every CI run and a reviewer naming a directory only relocates
// the output.
const uxDailyUseReviewDirEnv = "TT_UX_DAILY_USE_REVIEW_DIR"

// uxDailyUseReviewManifest is the index written beside the frames.
const uxDailyUseReviewManifest = "frames.tsv"

// The color renderings the review covers. Dark and light are palettes the app
// applies; monochrome is what a terminal without color does to either of them,
// so it is derived from each rendered frame rather than configured.
var uxDailyUseReviewVariants = []string{"dark", "light"}

// uxDailyUseFrame is one staged state of the product. `fixture` is part of the
// frame rather than of the matrix because the states this plan changes are not
// all reachable in one catalog. `owners` is every session that replaces some
// part of what the frame shows, which is a list and not a single session because
// two of these frames are the evidence for a pair of sessions that change the
// same screen from different directions.
type uxDailyUseFrame struct {
	name    string
	fixture string
	records string
	owners  []int
	stage   func(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel)
}

// uxDailyUseFrames is the coverage list. It is the six areas session 1 was told
// to cover — unequal project-name lengths, mixed bulk tags, hidden selections,
// capture growth, check-in feedback, and startup focus — plus the two frames
// those six are read against: an untouched launch, and a dense catalog scrolled
// away from its first row, which is the only frame where row geometry is
// measured somewhere other than the top of the window.
var uxDailyUseFrames = []uxDailyUseFrame{
	{
		name:    "startup-focus",
		fixture: FixtureMultiProject,
		records: "an untouched launch: the cursor rests on row 0, a section header, and the footer names navigation only",
		owners:  []int{11, 12},
		stage:   func(*testing.T, *UITestDriver, *outliner.OutlinerModel) {},
	},
	{
		name:    "project-label-widths",
		fixture: FixtureMultiProject,
		records: "four project names of unequal length, each drawn as a bracketed slug ahead of its task title",
		owners:  []int{2, 3},
		stage:   stageUXDailyUseProjectLabelWidths,
	},
	{
		name:    "dense-scrolled",
		fixture: FixtureRelease,
		records: "row geometry at the far end of a large catalog, where the window is not seated at the first row",
		owners:  []int{2, 3, 6},
		stage:   stageUXDailyUseDenseScrolled,
	},
	{
		name:    "hidden-selection",
		fixture: FixtureUXAcceptance,
		records: "a visual selection whose members are all folded out of view, so the summary is the only thing that knows the gesture has targets",
		owners:  []int{4},
		stage:   stageUXDailyUseHiddenSelection,
	},
	{
		name:    "workflow-chord",
		fixture: FixtureUXAcceptance,
		records: "the workflow chord's hints as one flat run, with planning and lifecycle destinations undistinguished",
		owners:  []int{5},
		stage:   stageUXDailyUseWorkflowChord,
	},
	{
		name:    "capture-panel",
		fixture: FixtureNarrow,
		records: "a three-line capture draft in a panel sized by the content it holds",
		owners:  []int{6, 7},
		stage:   stageUXDailyUseCapturePanel,
	},
	{
		name:    "bulk-tag-picker",
		fixture: FixtureUXAcceptance,
		records: "the tags-and-context picker over a multi-task selection, mixed values promoted to an APPLIED section ahead of the unapplied vocabulary",
		owners:  []int{8},
		stage:   stageUXDailyUseBulkTagPicker,
	},
	{
		name:    "check-in-offer",
		fixture: FixtureUXAcceptance,
		records: "the footer note a state change leaves behind, naming the committed task and the route that would schedule it, with nothing mounted over the list",
		owners:  []int{9, 10},
		stage:   stageUXDailyUseCheckInOffer,
	},
}

// uxDailyUseFrameRecord is one manifest row.
type uxDailyUseFrameRecord struct {
	variant string
	width   int
	height  int
	frame   uxDailyUseFrame
	// The measurements a later session changes the arithmetic of, read off the
	// live model and the composed frame rather than off a constant, so a row that
	// stops matching the product is a row that moved.
	headerRows   int
	contentRows  int
	peekDockRows int
	footerRows   int
	clippedLines int
}

// TestUXDailyUseReviewMatrix renders every frame at every reviewed size in every
// reviewed color rendering, checks the three things a frame has to satisfy to be
// worth reviewing at all, and exports it.
//
// The checks are about the frame rather than about its text: that it fits the
// terminal it claims, in display cells rather than runes; that removing color
// removes no content; and that rendering it twice produces the same bytes, since
// a frame that does not is a frame neither a reviewer nor a later diff can trust.
func TestUXDailyUseReviewMatrix(t *testing.T) {
	defer theme.ApplyTheme(theme.ThemeConfig{})
	root := uxDailyUseReviewRoot(t)
	records := make([]uxDailyUseFrameRecord, 0, len(uxDailyUseReviewVariants)*len(uxPresentationMatrixSizes)*len(uxDailyUseFrames))

	for _, variant := range uxDailyUseReviewVariants {
		for _, size := range uxPresentationMatrixSizes {
			for _, frame := range uxDailyUseFrames {
				name := fmt.Sprintf("%s-%dx%d-%s", variant, size[0], size[1], frame.name)
				t.Run(name, func(t *testing.T) {
					// One driver per frame, and a fresh temporary workspace with it:
					// two of these frames commit a mutation, so a shared driver would
					// make every later frame a measurement of the earlier frames' edits.
					driver, view := uxDailyUseDriver(t, frame.fixture, size[0], size[1])
					theme.ApplyTheme(theme.ThemeConfig{Name: variant})

					frame.stage(t, driver, view)

					obs := driver.Observe(ObserveOptions{IncludeANSI: true})
					assertUXScreenFitsItsTerminal(t, obs, size)
					assertUXMonochromeKeepsContent(t, obs)
					mono := uxClarityMonochrome(t, obs.ANSIScreen)

					if again := driver.Observe(ObserveOptions{IncludeANSI: true}); again.ANSIScreen != obs.ANSIScreen {
						t.Fatalf("%s renders differently the second time, so no frame of it is reviewable\n%s",
							frame.name, again.PlainScreen)
					}

					records = append(records, uxDailyUseFrameRecord{
						variant:      variant,
						width:        size[0],
						height:       size[1],
						frame:        frame,
						headerRows:   uxClarityHeaderRows(view, size[0]),
						contentRows:  view.ContentHeight(),
						peekDockRows: len(view.PeekDockLines()),
						footerRows:   lipgloss.Height(driver.App().renderStatusBar()),
						clippedLines: len(obs.ClippedLines),
					})
					uxDailyUseExportFrame(t, root, name, obs.ANSIScreen, obs.PlainScreen, mono)
				})
			}
		}
	}

	if len(records) == 0 {
		t.Fatal("the matrix exported nothing, so it is not reaching the product")
	}
	uxDailyUseWriteManifest(t, root, records)
	t.Logf("UX daily-use review frames: %s (set %s to keep them)", root, uxDailyUseReviewDirEnv)
}

// uxDailyUseDriver launches one named fixture at one size. It is the one place
// the fixture name on a frame is turned into a workspace, so no staging function
// has to know how a driver is built or remember to register the cleanup that
// removes the temporary workspace.
func uxDailyUseDriver(t *testing.T, fixture string, width, height int) (*UITestDriver, *outliner.OutlinerModel) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(fixture, width, height)
	if err != nil {
		t.Fatalf("fixture %s: %v", fixture, err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	view := driver.App().crossOutliner
	if view == nil {
		t.Fatalf("the %s fixture app has no Outliner", fixture)
	}
	return driver, view
}

// uxDailyUseReviewRoot resolves the artifact directory. A caller that named one
// owns it; a caller that named none gets the test's own, which is removed with
// the test and keeps the writer on every run rather than only on a reviewer's.
func uxDailyUseReviewRoot(t *testing.T) string {
	t.Helper()
	if named := strings.TrimSpace(os.Getenv(uxDailyUseReviewDirEnv)); named != "" {
		if err := os.MkdirAll(named, 0o700); err != nil {
			t.Fatalf("create %s=%q: %v", uxDailyUseReviewDirEnv, named, err)
		}
		return named
	}
	return t.TempDir()
}

// uxDailyUseExportFrame writes the three renderings of one frame: the styled
// capture a reviewer looks at, the monochrome projection that proves the content
// survives without color, and the plain text two runs are diffed through.
func uxDailyUseExportFrame(t *testing.T, root, name, styled, plain, mono string) {
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

// uxDailyUseWriteManifest writes the index: tab-separated, one row per frame, so
// the measurements can be read by eye and diffed between two runs of the matrix,
// which is how a session proves it moved only the arithmetic it meant to. The
// fixture is a column here and is not in the clarity manifest, because a reader
// of this directory cannot otherwise tell which catalog a frame was taken from.
func uxDailyUseWriteManifest(t *testing.T, root string, records []uxDailyUseFrameRecord) {
	t.Helper()
	var b strings.Builder
	b.WriteString("frame\tfixture\tvariant\twidth\theight\treplaced_by_sessions\theader_rows\tcontent_rows\tpeek_dock_rows\tfooter_rows\tclipped_lines\trecords\n")
	for _, record := range records {
		fmt.Fprintf(&b, "%s\t%s\t%s\t%d\t%d\t%s\t%d\t%d\t%d\t%d\t%d\t%s\n",
			record.frame.name, record.frame.fixture, record.variant, record.width, record.height,
			uxDailyUseOwnerList(record.frame.owners),
			record.headerRows, record.contentRows, record.peekDockRows, record.footerRows,
			record.clippedLines, record.frame.records)
	}
	path := filepath.Join(root, uxDailyUseReviewManifest)
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// uxDailyUseOwnerList spells a frame's owning sessions as one manifest cell.
func uxDailyUseOwnerList(owners []int) string {
	if len(owners) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(owners))
	for _, owner := range owners {
		parts = append(parts, strconv.Itoa(owner))
	}
	return strings.Join(parts, "+")
}

// The staging functions below are the frames' only definition of what each state
// is. The recorded expectations in ux_daily_use_baseline_test.go reach them
// through uxDailyUseStagedDriver, so the screens exported for review are exactly
// the screens the contracts are written against rather than a second
// approximation of them.

// stageUXDailyUseProjectLabelWidths seats the cursor on a task row in the one
// fixture whose four project names have four different lengths. Every other
// fixture either holds one project or holds names long enough that they all
// truncate to the same width, which is a frame where an alignment change would
// be invisible.
func stageUXDailyUseProjectLabelWidths(t *testing.T, _ *UITestDriver, view *outliner.OutlinerModel) {
	t.Helper()
	seatCursorOnTaskTitled(t, view, uxDailyUseAlphaTitle)
}

// uxDailyUseAlphaTitle is the `multi-project` fixture's first backlog task. It is
// named here rather than spelled at each use so a fixture edit is one failure
// instead of several.
const uxDailyUseAlphaTitle = "Alpha migration kickoff"

// stageUXDailyUseDenseScrolled sends the cursor to the end of the release
// fixture, which is the only catalog in the reviewed set large enough for the
// window to be somewhere other than its first row. Row geometry measured only at
// the top of a short list would not show a change that depends on the scroll
// position.
func stageUXDailyUseDenseScrolled(t *testing.T, driver *UITestDriver, _ *outliner.OutlinerModel) {
	t.Helper()
	uxClarityPress(t, driver, uxClarityKeys("G")...)
}

// stageUXDailyUseHiddenSelection takes a three-task range selection and then
// folds the sections it lives in, so every member is off screen. That is the
// state the selection summary exists for: with the members visible the counts
// restate what the checkboxes already say.
func stageUXDailyUseHiddenSelection(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel) {
	t.Helper()
	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
	uxClarityPress(t, driver, uxClarityKeys("v", "V", "j", "j")...)
	uxClarityPress(t, driver, uxClarityKeys("s")...)
	if counts := view.SelectionCounts(); counts.Hidden == 0 {
		t.Fatalf("folding the sections hid no selected task: %+v", counts)
	}
}

// stageUXDailyUseWorkflowChord holds the workflow chord open over a task row,
// which is where its destinations are listed.
func stageUXDailyUseWorkflowChord(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel) {
	t.Helper()
	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
	uxClarityPress(t, driver, uxClarityKeys("w")...)
}

// stageUXDailyUseCapturePanel opens capture and types a three-line draft. The
// draft is what makes the frame evidence rather than decoration: the panel
// stands three rows taller with it than without it, and a frame of an empty
// panel could not show what those rows cost the outline above.
func stageUXDailyUseCapturePanel(t *testing.T, driver *UITestDriver, _ *outliner.OutlinerModel) {
	t.Helper()
	uxClarityPress(t, driver, uxClarityKeys("a")...)
	uxClarityPress(t, driver, uxDailyUseDraftActions()...)
}

// uxDailyUseDraftActions types the three-line capture draft. The lines are short
// and unpunctuated so that no size in the matrix wraps one of them: a wrapped
// line is two rows of demand, so one frame would stand a row taller than the
// rest for a reason the panel's rule states but the draft does not.
func uxDailyUseDraftActions() []UITestAction {
	actions := make([]UITestAction, 0, 5)
	for i, line := range uxDailyUseDraftLines {
		if i > 0 {
			actions = append(actions, UITestAction{Type: UITestActionKey, Key: "shift+enter"})
		}
		actions = append(actions, UITestAction{Type: UITestActionText, Text: line})
	}
	return actions
}

var uxDailyUseDraftLines = []string{"one", "two", "three"}

// stageUXDailyUseBulkTagPicker opens the tags-and-context picker over a
// three-task selection whose members carry different tags, which is the only way
// to reach a row that is neither applied nor unapplied.
func stageUXDailyUseBulkTagPicker(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel) {
	t.Helper()
	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
	uxClarityPress(t, driver, uxClarityKeys("v", "V", "j", "j")...)
	uxClarityPress(t, driver, uxClarityKeys("#")...)
}

// stageUXDailyUseCheckInOffer moves one task to Doing through the workflow
// chord. Nothing is asked for beyond the transition, which is the point: the
// frame it stages is the committed list with one sentence in the footer.
func stageUXDailyUseCheckInOffer(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel) {
	t.Helper()
	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
	uxClarityPress(t, driver, uxClarityKeys("w", "g")...)
}

// uxDailyUseStagedDriver launches a frame's own fixture at one size and stages
// it, which is what every recorded expectation starts from.
func uxDailyUseStagedDriver(t *testing.T, frame string, width, height int) (*UITestDriver, *outliner.OutlinerModel) {
	t.Helper()
	staged, ok := uxDailyUseFrameNamed(frame)
	if !ok {
		t.Fatalf("no review frame named %q", frame)
	}
	driver, view := uxDailyUseDriver(t, staged.fixture, width, height)
	staged.stage(t, driver, view)
	return driver, view
}

func uxDailyUseFrameNamed(name string) (uxDailyUseFrame, bool) {
	for _, frame := range uxDailyUseFrames {
		if frame.name == name {
			return frame, true
		}
	}
	return uxDailyUseFrame{}, false
}
