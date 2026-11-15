package chrome

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestRenderBlockClock(t *testing.T) {
	for _, tc := range []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"0:00", 0, "███   ███ ███\n█ █ █ █ █ █ █\n█ █   █ █ █ █\n█ █ █ █ █ █ █\n███   ███ ███"},
		{"1:07", 67 * time.Minute, " █    ███ ███\n██  █ █ █   █\n █    █ █   █\n █  █ █ █   █\n███   ███   █"},
		{"12:45", 765 * time.Minute, " █  ███   █ █ ███\n██    █ █ █ █ █  \n █  ███   ███ ███\n █  █   █   █   █\n███ ███     █ ███"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ansi.Strip(RenderBlockClock(tc.duration, false))
			if got != tc.want {
				t.Fatalf("clock:\n%s\nwant:\n%s", got, tc.want)
			}
			lines := strings.Split(got, "\n")
			for _, line := range lines {
				if lipgloss.Width(line) != lipgloss.Width(lines[0]) {
					t.Fatalf("unequal widths: %q", lines)
				}
			}
		})
	}
	if RenderBlockClock(-time.Minute, false) != RenderBlockClock(0, false) {
		t.Fatal("negative clock did not clamp")
	}
}

func nowScreenFixture() NowScreenSpec {
	return NowScreenSpec{
		Title: "Fix login redirect", Meta: "Website", Elapsed: "1h in Doing", ElapsedDuration: 67 * time.Minute,
		ResumeNote: "Reproduce with an expired token", Progress: "1/3",
		Steps: []NowScreenStep{{Text: "Reproduce the loop", Done: true}, {Text: "Add a regression test"}, {Text: "Ship the fix"}},
	}
}

func assertNowFrame(t *testing.T, view string, width, height int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) != height {
		t.Fatalf("height %d, want %d", len(lines), height)
	}
	for _, line := range lines {
		if lipgloss.Width(line) != width {
			t.Fatalf("width %d, want %d: %q", lipgloss.Width(line), width, line)
		}
	}
}

func TestNowScreenShowsBlockClock(t *testing.T) {
	spec := nowScreenFixture()
	view := RenderNowScreen(spec, 100, 30)
	assertNowFrame(t, view, 100, 30)
	plain := ansi.Strip(view)
	// Frame padding follows each glyph row, so compare the painted prefix.
	for _, line := range strings.Split(ansi.Strip(RenderBlockClock(67*time.Minute, false)), "\n") {
		if !strings.Contains(plain, "▎ "+line) {
			t.Fatalf("missing clock row %q:\n%s", line, plain)
		}
	}
	if !strings.Contains(plain, "in Doing") || strings.Contains(plain, spec.Elapsed) {
		t.Fatal("large clock did not replace text age")
	}
}

func TestNowScreenShowsFocusRemaining(t *testing.T) {
	spec := nowScreenFixture()
	spec.Focus = &NowScreenFocus{Remaining: 18*time.Minute + time.Second, Length: 25 * time.Minute}
	for _, width := range []int{60, 100} {
		view := RenderNowScreen(spec, width, 30)
		assertNowFrame(t, view, width, 30)
		plain := ansi.Strip(view)
		if !strings.Contains(plain, "focus left") || strings.Contains(plain, "in Doing") {
			t.Fatalf("wrong caption:\n%s", plain)
		}
		for _, line := range strings.Split(ansi.Strip(RenderBlockClock(19*time.Minute, true)), "\n") {
			if !strings.Contains(plain, "▎ "+line) {
				t.Fatalf("missing rounded remaining row %q", line)
			}
		}
		barFound := false
		for _, line := range strings.Split(plain, "\n") {
			if strings.Contains(line, "━") {
				bar := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "▎"))
				if lipgloss.Width(bar) != min(nowScreenMaxWidth, width-8) {
					t.Fatalf("bar width: %q", bar)
				}
				barFound = true
			}
		}
		if !barFound {
			t.Fatal("no focus bar")
		}
	}
	if RenderBlockClock(-time.Second, true) != RenderBlockClock(0, true) {
		t.Fatal("expired focus did not clamp")
	}
}

func TestNowScreenFallsBackToTextClockWhenShort(t *testing.T) {
	for _, focus := range []*NowScreenFocus{nil, {Remaining: 18 * time.Minute, Length: 25 * time.Minute}} {
		spec := nowScreenFixture()
		spec.Focus = focus
		view := RenderNowScreen(spec, 60, 12)
		assertNowFrame(t, view, 60, 12)
		plain := ansi.Strip(view)
		for _, want := range []string{spec.Title, "Left off: " + spec.ResumeNote, "→ Add a regression test"} {
			if !strings.Contains(plain, want) {
				t.Fatalf("missing %q:\n%s", want, plain)
			}
		}
		want := spec.Elapsed
		if focus != nil {
			want = "◷ focus 18m left"
		}
		if !strings.Contains(plain, want) || strings.Contains(plain, "███") {
			t.Fatalf("not a text clock:\n%s", plain)
		}
	}
	t.Run("wrapped content before completed steps", func(t *testing.T) {
		spec := nowScreenFixture()
		spec.Title = "Fix login redirect and verify the expired token recovery flow"
		spec.ResumeNote = "Reproduce with an expired token before changing the redirect handler"
		spec.Steps = append([]NowScreenStep{{Text: "Inspect logs", Done: true}, {Text: "Trace the handler", Done: true}}, spec.Steps...)
		spec.Focus = &NowScreenFocus{Remaining: 18 * time.Minute, Length: 25 * time.Minute}
		view := RenderNowScreen(spec, 60, 9)
		assertNowFrame(t, view, 60, 9)
		plain := ansi.Strip(view)
		for _, want := range []string{"Fix login redirect and verify the expired token", "recovery flow", "Left off: Reproduce with an expired token before", "changing the redirect handler", "→ Add a regression test"} {
			if !strings.Contains(plain, want) {
				t.Fatalf("essential content missing %q:\n%s", want, plain)
			}
		}
		if strings.Contains(plain, "Inspect logs") || strings.Contains(plain, "███") {
			t.Fatalf("optional content crowded short frame:\n%s", plain)
		}
	})
}
