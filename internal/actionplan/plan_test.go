package actionplan

import (
	"strings"
	"testing"
)

func TestActionPlanValidationRejectsTerminalControlSequences(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"seven-bit CSI": "clear\x1b[2Jspoof",
		"seven-bit OSC": "title\x1b]0;spoof\x07",
		"eight-bit CSI": "clear\u009b2Jspoof",
		"eight-bit OSC": "title\u009d0;spoof\u009c",
	}
	for name, summary := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			plan := ActionPlan{Version: Version, Summary: summary, Actions: []Action{}}
			err := Validate(plan)
			if err == nil {
				t.Fatal("Validate unexpectedly accepted terminal control sequence")
			}
			if !strings.Contains(err.Error(), "summary contains unsafe terminal control character") {
				t.Fatalf("Validate error = %q, want unsafe summary control error", err)
			}
		})
	}
}
