package models

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxResumeNoteLength bounds the resume note in Unicode characters rather than
// bytes, so a note written in any script gets the same allowance. It is short on
// purpose: the note answers "where was I", which is one line of orientation, and
// anything longer belongs in the description.
const MaxResumeNoteLength = 240

// ValidateResumeNote is the one gate every surface that writes a resume note
// passes through, so the model, the storage writer, and any later editor agree
// on what a stored note may be rather than each bounding it their own way.
//
// The empty note is valid and means the task carries none: absence is the only
// spelling of "no resume note", which is what lets the canonical document omit
// the block entirely instead of writing an empty one.
//
// A note is stored exactly as it validates. Trimming belongs to whatever
// collects the text from a human, and an untrimmed value reaching here is a
// caller that skipped that step rather than a value to silently repair: two
// notes differing only in surrounding whitespace would otherwise be two
// different documents that mean the same thing.
func ValidateResumeNote(note string) error {
	// Line breaks are refused before the trim check so a multi-line note is
	// reported as the shape error it is rather than as stray whitespace.
	if strings.ContainsAny(note, "\n\r") {
		return fmt.Errorf("resume note must be a single line")
	}
	if note != strings.TrimSpace(note) {
		return fmt.Errorf("resume note must not have leading or trailing whitespace")
	}
	if n := utf8.RuneCountInString(note); n > MaxResumeNoteLength {
		return fmt.Errorf("resume note too long (%d chars, max %d)", n, MaxResumeNoteLength)
	}
	return nil
}
