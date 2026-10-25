package storage

import (
	"errors"
	"testing"

	"github.com/sglavoie/docket/internal/util"
)

// TestSafeChildNameRejectsTraversal confirms the storage wrapper forwards the
// util sentinel errors so callers can classify rejections uniformly.
func TestSafeChildNameRejectsTraversal(t *testing.T) {
	tests := []struct {
		input   string
		wantErr error
	}{
		{"notes.md", nil},
		{"", util.ErrPathEmpty},
		{"..", util.ErrPathTraversal},
		{"../etc/passwd", util.ErrPathSeparator},
		{"a/b", util.ErrPathSeparator},
		{"a\\b", util.ErrPathSeparator},
		{"/abs", util.ErrPathAbsolute},
	}
	for _, tc := range tests {
		err := SafeChildName(tc.input)
		if tc.wantErr == nil {
			if err != nil {
				t.Fatalf("SafeChildName(%q) = %v, want nil", tc.input, err)
			}
			continue
		}
		if !errors.Is(err, tc.wantErr) {
			t.Fatalf("SafeChildName(%q) = %v, want %v", tc.input, err, tc.wantErr)
		}
	}
}
