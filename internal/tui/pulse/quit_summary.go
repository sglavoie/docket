package pulse

import (
	"fmt"
	"strings"
)

// QuitSummary is the one line an interactive session leaves on the terminal
// after the alternate screen closes: what the day finished and what Today
// still holds, in the pulse's own spelling. A day with nothing done, nothing
// planned, and nothing overdue leaves no line at all.
func QuitSummary(pulse WorkspacePulse) string {
	parts := make([]string, 0, 3)
	if pulse.Done > 0 {
		parts = append(parts, fmt.Sprintf("%d done today", pulse.Done))
	}
	switch {
	case pulse.Today > 0:
		parts = append(parts, fmt.Sprintf("%d still in Today", pulse.Today))
	case pulse.Done > 0:
		parts = append(parts, "Today is clear")
	}
	if pulse.Overdue > 0 {
		parts = append(parts, fmt.Sprintf("%d overdue", pulse.Overdue))
	}
	if len(parts) == 0 {
		return ""
	}
	return "docket: " + strings.Join(parts, " · ")
}
