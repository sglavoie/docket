package capturetext

import (
	"errors"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
)

// A repeated singleton is refused rather than resolved by keeping one value, so
// neither `@mon` nor a mistyped `@tomorow` can vanish from a draft that already
// carried a date. The refusal points at the later token, and the preview shows
// the same message instead of a schedule that silently dropped half the input.
func TestCaptureRefusesRepeatedSingleton(t *testing.T) {
	inbox, err := models.NewProject("p1", "inbox", "")
	if err != nil {
		t.Fatal(err)
	}
	projects := []*models.Project{inbox}

	cases := []struct {
		text   string
		column int
		want   string
	}{
		{"Pay rent @fri @mon", 14, "duplicate due: @mon conflicts with earlier @fri"},
		{"Pay rent @fri @tomorow", 14, "duplicate due: @tomorow conflicts with earlier @fri"},
		{"Pay rent !1 !2", 12, "duplicate priority: !2 conflicts with earlier !1"},
	}
	for _, tc := range cases {
		_, err := ParseQuickCapture(tc.text, inbox, projects, false)
		var tokenErr *CaptureTokenError
		if !errors.As(err, &tokenErr) {
			t.Fatalf("%q: err = %v, want a CaptureTokenError", tc.text, err)
		}
		if tokenErr.Column != tc.column || !strings.Contains(tokenErr.Message, tc.want) {
			t.Fatalf("%q: refusal = %d %q, want %d containing %q", tc.text, tokenErr.Column, tokenErr.Message, tc.column, tc.want)
		}
		_, preview := ParseQuickCaptureForPreview(tc.text, inbox, projects, false)
		if !strings.Contains(preview, tc.want) {
			t.Fatalf("%q: preview error = %q, want it to contain %q", tc.text, preview, tc.want)
		}
	}
}
