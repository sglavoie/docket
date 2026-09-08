package models

import (
	"strings"
	"testing"
)

// TestValidateResumeNoteAcceptsTheShapesTheNoteIsFor keeps the gate from being
// stricter than the contract: the empty string is the only spelling of "no
// note", and everything else is one ordinary line of prose in any script.
func TestValidateResumeNoteAcceptsTheShapesTheNoteIsFor(t *testing.T) {
	for name, note := range map[string]string{
		"absent":            "",
		"plain":             "stopped at the failing integration test",
		"punctuation":       "next: re-run `just ci` — then ask Dana",
		"non-latin":         "続きから: ステージングのデプロイ待ち",
		"at the bound":      strings.Repeat("é", MaxResumeNoteLength),
		"interior spaces":   "a note   with   interior spacing",
		"leading delimiter": "<!-- RESUME --> is just prose here",
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateResumeNote(note); err != nil {
				t.Fatalf("ValidateResumeNote(%q) = %v, want nil", note, err)
			}
		})
	}
}

// TestValidateResumeNoteRefusesUnrepresentableValues names each refusal, because
// the storage writer, the merge, and every later editing surface all refuse
// through this one gate and must refuse the same set.
func TestValidateResumeNoteRefusesUnrepresentableValues(t *testing.T) {
	for name, note := range map[string]string{
		"newline":             "one\ntwo",
		"carriage return":     "one\rtwo",
		"leading whitespace":  " padded",
		"trailing whitespace": "padded\t",
		"one over the bound":  strings.Repeat("é", MaxResumeNoteLength+1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateResumeNote(note); err == nil {
				t.Fatalf("ValidateResumeNote(%q) = nil, want a refusal", note)
			}
		})
	}
}

// TestResumeNoteBoundIsCountedInCharacters pins the unit. A byte bound would
// give a note written in one script a fraction of the allowance another gets.
func TestResumeNoteBoundIsCountedInCharacters(t *testing.T) {
	multibyte := strings.Repeat("続", MaxResumeNoteLength)
	if len(multibyte) <= MaxResumeNoteLength {
		t.Fatalf("fixture is not multibyte: %d bytes", len(multibyte))
	}
	if err := ValidateResumeNote(multibyte); err != nil {
		t.Fatalf("a %d-character note was refused on its byte length: %v", MaxResumeNoteLength, err)
	}
}
