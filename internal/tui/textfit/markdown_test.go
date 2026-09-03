package textfit

import (
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// assertWidthBounded fails if any rendered line exceeds width (visual, ANSI-aware).
func assertWidthBounded(t *testing.T, rendered string, width int) {
	t.Helper()
	for _, line := range strings.Split(rendered, "\n") {
		stripped := ansi.Strip(line)
		if !utf8.ValidString(stripped) {
			t.Fatalf("RenderMarkdown produced invalid UTF-8 line: %q", stripped)
		}
		if w := lipgloss.Width(stripped); w > width {
			t.Fatalf("line width = %d, want <= %d for line %q\n\nfull output:\n%s",
				w, width, stripped, ansi.Strip(rendered))
		}
	}
}

func TestRenderMarkdownWideTableStaysWithinWidth(t *testing.T) {
	table := strings.Join([]string{
		"| Column One Heading | Column Two Heading | Column Three Heading |",
		"| --- | --- | --- |",
		"| a really long cell value here | another long value in this cell | yet more content that overflows |",
		"| short | short | short |",
	}, "\n")

	const width = 30
	rendered := RenderMarkdown(table, width)
	assertWidthBounded(t, rendered, width)

	// Important cell text must survive (wrapped vertically, not truncated away).
	stripped := ansi.Strip(rendered)
	for _, word := range []string{"really", "another", "content"} {
		if !strings.Contains(stripped, word) {
			t.Fatalf("expected wide-table cell word %q preserved, got:\n%s", word, stripped)
		}
	}
}

func TestRenderMarkdownMalformedPipeTableStaysWithinWidth(t *testing.T) {
	// Mirrors the screenshot content shape: prose around a table-like block with
	// broken/ragged pipe rows, long unbreakable identifiers, quotes, and backticks.
	md := strings.Join([]string{
		"Some prose before the table explaining the `RenderMarkdown` contract in detail.",
		"",
		"| Field | Value | Notes |",
		"| --- | --- |", // ragged: missing a delimiter cell
		"| id | ExtremelyLongUnbreakableIdentifierValueThatCannotWrapNicely | see below |",
		"| quote | \"a quoted phrase with, commas and 'nested' quotes\" | ok |",
		"| code | `github.com/sglavoie/tt/internal/tui/textfit.RenderMarkdown` | backticked path |",
		"| broken row without trailing pipe and extra | | pipes | | scattered", // malformed
		"|||", // degenerate empty row
		"",
		"Trailing prose after the malformed table referencing `boundToWidth` again.",
	}, "\n")

	const width = 32
	rendered := RenderMarkdown(md, width)
	assertWidthBounded(t, rendered, width)

	// Key text from prose, quotes, backticks, and identifiers must survive wrapping
	// (may be split across lines, but must not be truncated away entirely).
	stripped := strings.ReplaceAll(ansi.Strip(rendered), "\n", " ")
	for _, word := range []string{"RenderMarkdown", "quoted", "backticked", "boundToWidth"} {
		if !strings.Contains(stripped, word) {
			t.Fatalf("expected malformed-table word %q preserved, got:\n%s", word, ansi.Strip(rendered))
		}
	}
}

func TestRenderMarkdownLongPlainTextStaysWithinWidth(t *testing.T) {
	text := strings.Repeat("supercalifragilisticexpialidocious ", 8)

	const width = 20
	rendered := RenderMarkdown(text, width)
	assertWidthBounded(t, rendered, width)

	if strings.TrimSpace(ansi.Strip(rendered)) == "" {
		t.Fatal("expected non-empty rendered output for long plain text")
	}
}

func TestRenderMarkdownPreservesTextNotTruncated(t *testing.T) {
	const width = 24
	rendered := RenderMarkdown("The quick brown fox jumps over the lazy dog repeatedly.", width)
	assertWidthBounded(t, rendered, width)

	stripped := ansi.Strip(rendered)
	for _, word := range []string{"quick", "brown", "lazy", "repeatedly"} {
		if !strings.Contains(stripped, word) {
			t.Fatalf("expected word %q preserved in output, got:\n%s", word, stripped)
		}
	}
}

func TestBoundToWidthLeavesShortLinesUntouched(t *testing.T) {
	in := "short line\nanother short one"
	got := boundToWidth(in, 40)
	if got != in {
		t.Fatalf("boundToWidth mutated already-fitting text:\ngot:  %q\nwant: %q", got, in)
	}
}

func TestBoundToWidthHardWrapsLongSeparatorFragments(t *testing.T) {
	in := "| ------------------------------------------------------------------- | ------------------------------------------------------------------------------ |"
	const width = 32

	got := boundToWidth(in, width)
	assertWidthBounded(t, got, width)
	if !strings.Contains(ansi.Strip(got), "---") {
		t.Fatalf("expected separator content preserved, got:\n%s", ansi.Strip(got))
	}
}

func TestBoundToWidthPreservesANSIStyling(t *testing.T) {
	styled := lipgloss.NewStyle().Bold(true).Render("this styled line is definitely too wide to fit")
	got := boundToWidth(styled, 12)
	assertWidthBounded(t, got, 12)
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected ANSI escape codes preserved after wrapping, got: %q", got)
	}
}
