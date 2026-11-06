package grammar

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/theme"
)

// welcomeExamples are the captures the welcome teaches with, and what each
// token becomes. Each line parses under the capture grammar as written, which
// the grammar's own tests hold it to.
var welcomeExamples = []struct{ line, means string }{
	{"Call the bank !1 @fri #errands", "priority 1 · due Friday · tagged errands"},
	{"Draft the offsite agenda $today ##desk", "planned for Today · at your desk"},
	{"Renew the passport @@mon =review", "out of sight until Monday · flagged for review"},
}

// WelcomeCard is the first launch's card: what Docket is for in one line, the
// grammar by example, and the three keys that carry the rest. Examples are
// colored as the composer colors them, so the first capture looks like what
// was taught.
func WelcomeCard(width int) []string {
	exampleWidth := 0
	for _, example := range welcomeExamples {
		exampleWidth = max(exampleWidth, lipgloss.Width(example.line))
	}
	meansWidth := 0
	for _, example := range welcomeExamples {
		meansWidth = max(meansWidth, lipgloss.Width(example.means))
	}
	sideBySide := exampleWidth+4+meansWidth <= width-4
	key := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true)
	lines := []string{
		centerWelcomeLine(theme.TitleStyle.Render("Welcome to Docket"), width),
		centerWelcomeLine(theme.MutedStyle.Render("Press ")+key.Render("a")+theme.MutedStyle.Render(" and type a task. Each word with a sigil becomes a detail:"), width),
		"",
	}
	blockWidth := exampleWidth
	if sideBySide {
		blockWidth = exampleWidth + 4 + meansWidth
	}
	left := strings.Repeat(" ", max((width-blockWidth)/2, 0))
	for _, example := range welcomeExamples {
		highlighted := HighlightCaptureExample(example.line)
		means := theme.MutedStyle.Render(example.means)
		if sideBySide {
			gap := strings.Repeat(" ", exampleWidth-lipgloss.Width(example.line)+4)
			lines = append(lines, left+highlighted+gap+means)
			continue
		}
		lines = append(lines, left+highlighted, left+"  "+means)
	}
	lines = append(lines, "",
		centerWelcomeLine(key.Render("w")+theme.MutedStyle.Render(" moves work along · ")+
			key.Render("/")+theme.MutedStyle.Render(" finds anything · ")+
			key.Render("?")+theme.MutedStyle.Render(" lists every key"), width))
	return lines
}

func centerWelcomeLine(text string, width int) string {
	if pad := (width - lipgloss.Width(text)) / 2; pad > 0 {
		return strings.Repeat(" ", pad) + text
	}
	return text
}
