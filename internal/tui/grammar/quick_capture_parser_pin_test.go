package grammar

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/capturetext"
	"github.com/sglavoie/docket/internal/textgrammar"
)

func TestParseQuickCapturePinSetsPinnedResult(t *testing.T) {
	result, err := capturetext.ParseQuickCapture("Ship the release !1 *pin", quickCaptureTestProject(), nil, false)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !result.Pinned {
		t.Error("expected pinned to be set")
	}
	if result.Title != "Ship the release" {
		t.Errorf("title = %q, want %q", result.Title, "Ship the release")
	}
}

func TestParseQuickCaptureRefusesUnpinPerSurface(t *testing.T) {
	tests := []struct {
		surface textgrammar.Surface
		want    string
	}{
		{textgrammar.SurfaceCapture, "*unpin is not available when capturing"},
		{textgrammar.SurfaceOutline, "omit *pin to unpin here"},
	}
	for _, tt := range tests {
		_, err := capturetext.ParseQuickCaptureSurface("Ship the release *unpin", quickCaptureTestProject(), nil, true, tt.surface)
		if err == nil {
			t.Fatalf("surface %v: expected an error", tt.surface)
		}
		if !strings.Contains(err.Error(), tt.want) {
			t.Errorf("surface %v: error %q should contain %q", tt.surface, err.Error(), tt.want)
		}
	}
}

func TestParseQuickCaptureKeepsUnrecognizedPinValueInTitle(t *testing.T) {
	result, err := capturetext.ParseQuickCapture("Refactor *args handling", quickCaptureTestProject(), nil, false)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.Title != "Refactor *args handling" {
		t.Errorf("title = %q, want %q", result.Title, "Refactor *args handling")
	}
	if result.Pinned {
		t.Error("expected pinned to stay unset")
	}
}
