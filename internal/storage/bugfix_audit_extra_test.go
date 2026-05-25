package storage

import (
	"strings"
	"testing"
)

// TestSplitFrontmatter_HorizontalRuleInsideFrontmatterIsNotClosing guards
// against a content line beginning with "----" being mistaken for the closing
// "---" delimiter. The earlier code used strings.Cut on "\n---", which matches
// the leading three characters of "----" and silently truncated the
// frontmatter and prepended stray characters to the body.
func TestSplitFrontmatter_HorizontalRuleInsideFrontmatterIsNotClosing(t *testing.T) {
	input := "---\nfoo: bar\n----extra\nreal: end\n---\nreal body\n"
	fm, body, err := splitFrontmatter(input)
	if err != nil {
		t.Fatalf("splitFrontmatter: %v", err)
	}
	if !strings.Contains(fm, "real: end") {
		t.Errorf("frontmatter lost trailing keys; got:\n%s", fm)
	}
	if !strings.Contains(fm, "----extra") {
		t.Errorf("frontmatter dropped ---- content line; got:\n%s", fm)
	}
	if strings.TrimSpace(body) != "real body" {
		t.Errorf("body mismatch: got %q, want %q", body, "real body")
	}
}

// TestSplitFrontmatter_BodyStartingWithDashesPreserved verifies that a body
// whose first non-whitespace bytes are "----" (e.g. a Markdown horizontal
// rule) is not corrupted by the closing-delimiter scan. Before the fix, the
// Cut on "\n---" matched the "---" prefix of "----" in the body and the body
// regained a stray leading "-".
func TestSplitFrontmatter_BodyStartingWithDashesPreserved(t *testing.T) {
	input := "---\ntitle: foo\n---\n----\nhorizontal rule line\n"
	_, body, err := splitFrontmatter(input)
	if err != nil {
		t.Fatalf("splitFrontmatter: %v", err)
	}
	if !strings.HasPrefix(body, "----") {
		t.Errorf("body lost leading dashes; got %q", body)
	}
}

// TestSplitFrontmatter_MissingClosingDelimiterErrors verifies the error path
// is preserved by the new lookahead logic.
func TestSplitFrontmatter_MissingClosingDelimiterErrors(t *testing.T) {
	input := "---\ntitle: foo\nno closing here\n"
	if _, _, err := splitFrontmatter(input); err == nil {
		t.Error("expected error for missing closing delimiter, got nil")
	}
}
