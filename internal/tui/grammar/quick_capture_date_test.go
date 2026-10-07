package grammar

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/capturetext"
	"github.com/sglavoie/tt/internal/util"
)

func TestParseDateIntegration(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"today", false},
		{"tomorrow", false},
		{"+3d", false},
		{"+1w", false},
		{"+2m", false},
		{"monday", false},
		{"fri", false},
		{"2025-12-31", false},
		{"12/31/2025", false},
		{"31.12.2025", false},
		{"next week", false},
		{"next month", false},
		{"next monday", false},
		{"next friday", false},
		{"in 2 weeks", false},
		{"in 3 days", false},
		{"in 1 month", false},
		{"notadate", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := capturetext.ParseDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("capturetext.ParseDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestParseDateTomorrow(t *testing.T) {
	result, err := capturetext.ParseDate("tomorrow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	now := time.Now()
	expected := util.StartOfDay(now).AddDate(0, 0, 1)

	if !result.Equal(expected) {
		t.Errorf("capturetext.ParseDate(tomorrow) = %v, want %v", result, expected)
	}
}
