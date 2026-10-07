package capturetext

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

func TestParseDate(t *testing.T) {
	now := time.Now()
	today := util.StartOfDay(now)

	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:    "ISO date format",
			input:   "2024-06-15",
			want:    time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "today keyword",
			input:   "today",
			want:    today,
			wantErr: false,
		},
		{
			name:    "TODAY uppercase",
			input:   "TODAY",
			want:    today,
			wantErr: false,
		},
		{
			name:    "tomorrow keyword",
			input:   "tomorrow",
			want:    today.AddDate(0, 0, 1),
			wantErr: false,
		},
		{
			name:    "relative +3d",
			input:   "+3d",
			want:    today.AddDate(0, 0, 3),
			wantErr: false,
		},
		{
			name:    "relative +1w",
			input:   "+1w",
			want:    today.AddDate(0, 0, 7),
			wantErr: false,
		},
		{
			name:    "relative +2w",
			input:   "+2w",
			want:    today.AddDate(0, 0, 14),
			wantErr: false,
		},
		{
			// Month arithmetic clamps rather than overflows, so the expectation has
			// to be AddMonthsClamped: on the 31st, time.AddDate would roll into the
			// month after the one the parser lands in.
			name:    "relative +1m",
			input:   "+1m",
			want:    util.AddMonthsClamped(today, 1),
			wantErr: false,
		},
		{
			name:    "next week keyword",
			input:   "next week",
			want:    today.AddDate(0, 0, 7),
			wantErr: false,
		},
		{
			name:    "next month keyword",
			input:   "next month",
			want:    util.AddMonthsClamped(today, 1),
			wantErr: false,
		},
		{
			name:    "in 3 days",
			input:   "in 3 days",
			want:    today.AddDate(0, 0, 3),
			wantErr: false,
		},
		{
			name:    "in 1 day singular",
			input:   "in 1 day",
			want:    today.AddDate(0, 0, 1),
			wantErr: false,
		},
		{
			name:    "in 1 week",
			input:   "in 1 week",
			want:    today.AddDate(0, 0, 7),
			wantErr: false,
		},
		{
			name:    "in 2 weeks",
			input:   "in 2 weeks",
			want:    today.AddDate(0, 0, 14),
			wantErr: false,
		},
		{
			name:    "in 1 month",
			input:   "in 1 month",
			want:    util.AddMonthsClamped(today, 1),
			wantErr: false,
		},
		{
			name:    "in 3 months plural",
			input:   "in 3 months",
			want:    util.AddMonthsClamped(today, 3),
			wantErr: false,
		},
		{
			name:    "IN 2 WEEKS uppercase",
			input:   "IN 2 WEEKS",
			want:    today.AddDate(0, 0, 14),
			wantErr: false,
		},
		{
			name:    "in many weeks invalid",
			input:   "in many weeks",
			wantErr: true,
		},
		{
			name:    "invalid input",
			input:   "not a date",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDate(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Compare year, month, day only
				gotDate := time.Date(got.Year(), got.Month(), got.Day(), 0, 0, 0, 0, time.UTC)
				wantDate := time.Date(tt.want.Year(), tt.want.Month(), tt.want.Day(), 0, 0, 0, 0, time.UTC)

				if !gotDate.Equal(wantDate) {
					t.Errorf("ParseDate(%q) = %v, want %v", tt.input, gotDate.Format("2006-01-02"), wantDate.Format("2006-01-02"))
				}
			}
		})
	}
}

func TestParseDateWeekdays(t *testing.T) {
	now := time.Now()
	today := util.StartOfDay(now)

	weekdayTests := []struct {
		input   string
		weekday time.Weekday
	}{
		{"mon", time.Monday},
		{"monday", time.Monday},
		{"tue", time.Tuesday},
		{"tuesday", time.Tuesday},
		{"wed", time.Wednesday},
		{"wednesday", time.Wednesday},
		{"thu", time.Thursday},
		{"thursday", time.Thursday},
		{"fri", time.Friday},
		{"friday", time.Friday},
		{"sat", time.Saturday},
		{"saturday", time.Saturday},
		{"sun", time.Sunday},
		{"sunday", time.Sunday},
	}

	for _, tt := range weekdayTests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseDate(tt.input)
			if err != nil {
				t.Errorf("ParseDate(%q) unexpected error: %v", tt.input, err)
				return
			}

			// The result should be the next occurrence of the weekday
			if got.Weekday() != tt.weekday {
				t.Errorf("ParseDate(%q) weekday = %v, want %v", tt.input, got.Weekday(), tt.weekday)
			}

			// It should be in the future (or today if same weekday - but always at least +1 day)
			if !got.After(today) {
				t.Errorf("ParseDate(%q) = %v, should be after today %v", tt.input, got, today)
			}

			// It should be at most 7 days from today
			maxDate := today.AddDate(0, 0, 7)
			if got.After(maxDate) {
				t.Errorf("ParseDate(%q) = %v, should be within 7 days from today", tt.input, got)
			}
		})
	}
}

func TestParseDateNextWeekday(t *testing.T) {
	now := time.Now()
	today := util.StartOfDay(now)

	nextWeekdayTests := []struct {
		input   string
		weekday time.Weekday
	}{
		{"next monday", time.Monday},
		{"next mon", time.Monday},
		{"next tuesday", time.Tuesday},
		{"next tue", time.Tuesday},
		{"next wednesday", time.Wednesday},
		{"next wed", time.Wednesday},
		{"next thursday", time.Thursday},
		{"next thu", time.Thursday},
		{"next friday", time.Friday},
		{"next fri", time.Friday},
		{"next saturday", time.Saturday},
		{"next sat", time.Saturday},
		{"next sunday", time.Sunday},
		{"next sun", time.Sunday},
		{"NEXT Monday", time.Monday}, // case insensitive
		{"Next Friday", time.Friday}, // mixed case
	}

	for _, tt := range nextWeekdayTests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseDate(tt.input)
			if err != nil {
				t.Errorf("ParseDate(%q) unexpected error: %v", tt.input, err)
				return
			}

			// The result should be the correct weekday
			if got.Weekday() != tt.weekday {
				t.Errorf("ParseDate(%q) weekday = %v, want %v", tt.input, got.Weekday(), tt.weekday)
			}

			// "next <weekday>" should always be at least 7 days away
			minDate := today.AddDate(0, 0, 7)
			if got.Before(minDate) {
				t.Errorf("ParseDate(%q) = %v, should be at least 7 days from today %v", tt.input, got, today)
			}

			// It should be at most 14 days from today
			maxDate := today.AddDate(0, 0, 14)
			if got.After(maxDate) {
				t.Errorf("ParseDate(%q) = %v, should be within 14 days from today", tt.input, got)
			}
		})
	}
}
