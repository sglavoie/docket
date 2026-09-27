package tui

import "testing"

func TestSentenceCaseFeedbackLeavesSlugsAndKeysAlone(t *testing.T) {
	cases := map[string]string{
		"save refused: title is required": "Save refused: title is required",
		"Already capitalized":             "Already capitalized",
		"#tag is reserved":                "#tag is reserved",
		"tasks.md could not be read":      "tasks.md could not be read",
		"3 tasks refused":                 "3 tasks refused",
		"a/b is not a project":            "a/b is not a project",
		"":                                "",
	}
	for in, want := range cases {
		if got := sentenceCaseFeedback(in); got != want {
			t.Errorf("sentenceCaseFeedback(%q) = %q, want %q", in, got, want)
		}
	}
}
