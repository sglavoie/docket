package textfit

import "github.com/charmbracelet/x/ansi"

// Hyperlink wraps text in an OSC 8 link to uri. The link is opened and closed
// around the text it is given, so a caller links each already-wrapped row on
// its own and no link is ever left open across a line break, a pad, or a
// clip. The sequences are zero-width to every measurement in this package, and
// a terminal that does not know them prints the text alone.
func Hyperlink(uri, text string) string {
	if uri == "" || text == "" {
		return text
	}
	return ansi.SetHyperlink(uri) + text + ansi.ResetHyperlink()
}
