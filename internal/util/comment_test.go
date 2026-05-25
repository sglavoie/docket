package util

import (
	"strings"
	"testing"
)

func TestParseEditedCommentContent_StandardFormat(t *testing.T) {
	in := "# Comment 1 on #5: My Task\n\n<!-- Edit the comment text below. -->\n\nThis is my comment text.\n"
	got := ParseEditedCommentContent(in)
	want := "This is my comment text."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseEditedCommentContent_PreservesUserHeaders(t *testing.T) {
	// User-added markdown headers (# heading) inside the body must be preserved.
	in := "# Comment 1 on #5: My Task\n\n<!-- Edit the comment text below. -->\n\n# My Heading\n\nbody"
	got := ParseEditedCommentContent(in)
	want := "# My Heading\n\nbody"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseEditedCommentContent_MultilineBody(t *testing.T) {
	in := "# Header\n\n<!-- hint -->\n\nline1\nline2\n\nline4"
	got := ParseEditedCommentContent(in)
	want := "line1\nline2\n\nline4"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseEditedCommentContent_EmptyBodyAfterMarker(t *testing.T) {
	in := "# Header\n\n<!-- hint -->\n\n"
	got := ParseEditedCommentContent(in)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestParseEditedCommentContent_NoHTMLMarker(t *testing.T) {
	// Without the <!-- marker, foundComment stays false → returns "".
	// Important: this is observable behavior callers depend on
	// (they show "cannot be empty" errors rather than silently saving).
	in := "# Header\n\nbody text without marker"
	got := ParseEditedCommentContent(in)
	if got != "" {
		t.Errorf("expected empty (no <!-- marker), got %q", got)
	}
}

func TestParseEditedCommentContent_EmptyInput(t *testing.T) {
	if got := ParseEditedCommentContent(""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestParseEditedCommentContent_TrimsLeadingAndTrailingBlanks(t *testing.T) {
	in := "# Header\n\n<!-- hint -->\n\n\n\nthe text\n\n\n"
	got := ParseEditedCommentContent(in)
	want := "the text"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseEditedCommentContent_MultipleHashHeadersBeforeMarker(t *testing.T) {
	// Several "# " title lines before the marker (e.g. accidental duplication)
	// must all be skipped without being treated as content.
	in := "# Title A\n# Title B\n\n<!-- hint -->\n\ncontent"
	got := ParseEditedCommentContent(in)
	want := "content"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseEditedCommentContent_HashHeaderAfterMarkerKept(t *testing.T) {
	// A "# foo" line inside the body must be preserved (regression: ensure the
	// "skip title" branch doesn't fire after foundComment becomes true).
	in := "# Title\n\n<!-- hint -->\n\n# Section\nbody"
	got := ParseEditedCommentContent(in)
	want := "# Section\nbody"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseEditedCommentContent_WhitespaceOnlyBody(t *testing.T) {
	// A body that is only whitespace (spaces, tabs) must collapse to ""
	// so that callers' `newText == ""` empty-checks reject it as invalid
	// rather than silently saving a blank comment.
	in := "# Header\n\n<!-- hint -->\n\n   \n\t \n  "
	got := ParseEditedCommentContent(in)
	if got != "" {
		t.Errorf("expected empty for whitespace-only body, got %q", got)
	}
}

// TestParseEditedCommentContent_CRLF verifies CRLF line endings (Windows
// editors, some clipboard sources) are normalized — no stray \r in the body.
func TestParseEditedCommentContent_CRLF(t *testing.T) {
	in := "# Comment 1 on #5: Hello\r\n\r\n<!-- Edit the comment text below. -->\r\n\r\nActual text here\r\nsecond line\r\n"
	got := ParseEditedCommentContent(in)
	want := "Actual text here\nsecond line"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestParseEditedCommentContent_CROnly verifies bare CR line endings (legacy
// classic-Mac style; some unusual editors) are also normalized.
func TestParseEditedCommentContent_CROnly(t *testing.T) {
	in := "# Header\r\r<!-- hint -->\r\rbody\r"
	got := ParseEditedCommentContent(in)
	want := "body"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseEditedCommentContent_LongBody(t *testing.T) {
	body := strings.Repeat("line\n", 50)
	in := "# Header\n\n<!-- hint -->\n\n" + body
	got := ParseEditedCommentContent(in)
	want := strings.TrimRight(body, "\n")
	if got != want {
		t.Errorf("long body mismatch: got len=%d, want len=%d", len(got), len(want))
	}
}
