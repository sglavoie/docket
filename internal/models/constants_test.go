package models

import "testing"

func TestValidatePriority(t *testing.T) {
	tests := []struct {
		name      string
		value     int
		allowZero bool
		wantErr   bool
	}{
		{"min valid", 1, false, false},
		{"max valid", 5, false, false},
		{"mid valid", 3, false, false},
		{"zero not allowed", 0, false, true},
		{"zero allowed", 0, true, false},
		{"negative", -1, false, true},
		{"above max", 6, false, true},
		{"far above max", 100, false, true},
		{"zero allowed but negative still invalid", -1, true, true},
		{"zero allowed but above max still invalid", 6, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePriority(tt.value, tt.allowZero)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePriority(%d, %v) error = %v, wantErr %v", tt.value, tt.allowZero, err, tt.wantErr)
			}
		})
	}
}
