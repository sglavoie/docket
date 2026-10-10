package conflict

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/pmezard/go-difflib/difflib"

	"github.com/sglavoie/tt/internal/tui/theme"
)

// unifiedDiff keeps Git's familiar file headers, context, hunk ranges and
// missing-final-newline markers. It needs neither Git nor temporary files.
func unifiedDiff(before, after, from, to string) []string {
	if before == after {
		return []string{"No changes"}
	}
	patch, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A: diffLines(before), B: diffLines(after), FromFile: from, ToFile: to, Context: 3,
	})
	if err != nil {
		return []string{err.Error()}
	}
	// difflib leaves unterminated input lines unterminated in the patch. Supply
	// Git's explicit marker so adjacent removals/additions never run together.
	return strings.Split(strings.TrimSuffix(patch, "\n"), "\n")
}

func diffLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		return lines[:len(lines)-1]
	}
	lines[len(lines)-1] += "\n\\ No newline at end of file\n"
	return lines
}

func paintDiffLine(source, line string) string {
	style := lipgloss.NewStyle()
	switch {
	case strings.HasPrefix(source, "+"):
		style = style.Foreground(theme.ColorSuccess)
	case strings.HasPrefix(source, "-"):
		style = style.Foreground(theme.ColorError)
	case strings.HasPrefix(source, "@@"):
		style = style.Foreground(theme.ColorPrimary)
	default:
		return line
	}
	return style.Render(line)
}
