package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/textgrammar"
)

func TestParseQuickCapturePinSetsPinnedResult(t *testing.T) {
	result, err := parseQuickCapture("Ship the release !1 *pin", quickCaptureTestProject(), nil, false)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !result.pinned {
		t.Error("expected pinned to be set")
	}
	if result.title != "Ship the release" {
		t.Errorf("title = %q, want %q", result.title, "Ship the release")
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
		_, err := parseQuickCaptureSurface("Ship the release *unpin", quickCaptureTestProject(), nil, true, tt.surface)
		if err == nil {
			t.Fatalf("surface %v: expected an error", tt.surface)
		}
		if !strings.Contains(err.Error(), tt.want) {
			t.Errorf("surface %v: error %q should contain %q", tt.surface, err.Error(), tt.want)
		}
	}
}

func TestParseQuickCaptureKeepsUnrecognizedPinValueInTitle(t *testing.T) {
	result, err := parseQuickCapture("Refactor *args handling", quickCaptureTestProject(), nil, false)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.title != "Refactor *args handling" {
		t.Errorf("title = %q, want %q", result.title, "Refactor *args handling")
	}
	if result.pinned {
		t.Error("expected pinned to stay unset")
	}
}
