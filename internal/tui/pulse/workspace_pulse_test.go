package pulse

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestWorkspacePulseFormattingFollowsFourStepLadder(t *testing.T) {
	pulse := WorkspacePulse{Inbox: 3, Today: 2, Overdue: 1, Done: 4}
	full := formatWorkspacePulseComponents(pulse, workspacePulseFull)
	wantStyled := theme.MutedStyle.Render("workspace: ") +
		theme.MutedStyle.Render("inbox 3") +
		theme.MutedStyle.Render(" · ") +
		theme.MutedStyle.Render("today 2") +
		theme.MutedStyle.Render(" · ") +
		theme.PreviewWarningStyle.Render("overdue 1") +
		theme.MutedStyle.Render(" · ") +
		theme.MutedStyle.Render("done 4")
	if full != wantStyled {
		t.Fatalf("full pulse ANSI styling = %q, want %q", full, wantStyled)
	}

	for _, tt := range []struct {
		level workspacePulseLevel
		want  string
	}{
		{workspacePulseFull, "workspace: inbox 3 · today 2 · overdue 1 · done 4"},
		{workspacePulseWithoutDone, "workspace: inbox 3 · today 2 · overdue 1"},
		{workspacePulseWithoutInbox, "workspace: today 2 · overdue 1"},
		{workspacePulseTodayOnly, "workspace: today 2"},
		{workspacePulseHidden, ""},
	} {
		if got := ansi.Strip(formatWorkspacePulseComponents(pulse, tt.level)); got != tt.want {
			t.Errorf("level %d pulse = %q, want %q", tt.level, got, tt.want)
		}
	}

	for _, level := range []workspacePulseLevel{workspacePulseFull, workspacePulseWithoutDone, workspacePulseWithoutInbox, workspacePulseTodayOnly, workspacePulseHidden} {
		if got := ansi.Strip(formatWorkspacePulseComponents(WorkspacePulse{}, level)); got != "" {
			t.Errorf("zero pulse at level %d = %q, want no readout and no workspace prefix", level, got)
		}
	}
}

// A component nobody has any of says nothing, but the ladder is still the ladder:
// dropping a zero component must not promote a lower-priority one, and the levels
// keep removing the same components in the same order over what remains.
func TestWorkspacePulseOmitsZeroComponentsWithoutDisturbingTheLadder(t *testing.T) {
	for _, tt := range []struct {
		name  string
		pulse WorkspacePulse
		level workspacePulseLevel
		want  string
	}{
		{"only overdue", WorkspacePulse{Overdue: 2}, workspacePulseFull, "workspace: overdue 2"},
		{"only done", WorkspacePulse{Done: 5}, workspacePulseFull, "workspace: done 5"},
		{"only done dropped first", WorkspacePulse{Done: 5}, workspacePulseWithoutDone, ""},
		{"zero today omitted", WorkspacePulse{Inbox: 1, Overdue: 2}, workspacePulseFull, "workspace: inbox 1 · overdue 2"},
		{"zero today stays omitted alone", WorkspacePulse{Inbox: 1, Overdue: 2}, workspacePulseTodayOnly, ""},
		{"zero inbox omitted", WorkspacePulse{Today: 4, Done: 1}, workspacePulseFull, "workspace: today 4 · done 1"},
	} {
		if got := ansi.Strip(formatWorkspacePulseComponents(tt.pulse, tt.level)); got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestWorkspacePulseFullFormKeepsMutedForegroundThroughDone(t *testing.T) {
	muted := activeForegroundAt(t, theme.MutedStyle.Render("probe"), "probe")
	warning := activeForegroundAt(t, theme.PreviewWarningStyle.Render("probe"), "probe")
	if muted == "" || warning == "" || muted == warning {
		t.Fatalf("style probes: muted = %q, warning = %q, want two distinct non-empty foregrounds", muted, warning)
	}

	full := formatWorkspacePulseComponents(WorkspacePulse{Inbox: 3, Today: 2, Overdue: 1, Done: 4}, workspacePulseFull)
	for _, tt := range []struct {
		segment string
		want    string
	}{
		{"workspace: ", muted},
		{"inbox 3", muted},
		{"today 2", muted},
		{"overdue 1", warning},
		{"done 4", muted},
	} {
		if got := activeForegroundAt(t, full, tt.segment); got != tt.want {
			t.Errorf("foreground in effect at %q = %q, want %q", tt.segment, got, tt.want)
		}
	}

	// Wrapping a pre-styled warning segment in an outer muted style leaves the
	// warning's own reset inside the string, so everything after it renders in the
	// terminal default. This is the composition the per-component styling replaces.
	nested := theme.MutedStyle.Render("workspace: inbox 3 · today 2 · " + theme.PreviewWarningStyle.Render("overdue 1") + " · done 4")
	if got := activeForegroundAt(t, nested, "done 4"); got != "" {
		t.Fatalf("nested-reset composition foreground at %q = %q, want the terminal default", "done 4", got)
	}
}

var sgrSequencePattern = regexp.MustCompile("\x1b\\[[0-9;:]*m")

// activeForegroundAt replays every SGR sequence preceding segment and reports the
// foreground parameters a terminal would still have applied when printing it.
// An empty result means the terminal default foreground.
func activeForegroundAt(t *testing.T, rendered, segment string) string {
	t.Helper()
	plain := ansi.Strip(rendered)
	target := strings.Index(plain, segment)
	if target < 0 {
		t.Fatalf("segment %q missing from %q", segment, plain)
	}

	foreground := ""
	printed, cursor := 0, 0
	for _, location := range sgrSequencePattern.FindAllStringIndex(rendered, -1) {
		text := rendered[cursor:location[0]]
		if printed+len(text) > target {
			return foreground
		}
		printed += len(text)
		foreground = applyForegroundSGR(foreground, rendered[location[0]+2:location[1]-1])
		cursor = location[1]
	}
	return foreground
}

// applyForegroundSGR folds one SGR parameter list into the active foreground,
// tracking only the codes that can change it.
func applyForegroundSGR(current, params string) string {
	if params == "" {
		return ""
	}
	fields := strings.Split(params, ";")
	for i := 0; i < len(fields); i++ {
		code := fields[i]
		switch {
		case code == "" || code == "0" || code == "39":
			current = ""
		case code == "38":
			span := 3
			if i+1 < len(fields) && fields[i+1] == "2" {
				span = 5
			}
			end := min(i+span, len(fields))
			current = strings.Join(fields[i:end], ";")
			i = end - 1
		case len(code) == 2 && (code[0] == '3' || code[0] == '9') && code[1] >= '0' && code[1] <= '7':
			current = code
		}
	}
	return current
}
