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
		{"max valid", 3, false, false},
		{"mid valid", 3, false, false},
		{"zero not allowed", 0, false, true},
		{"zero allowed", 0, true, false},
		{"negative", -1, false, true},
		{"above max", 4, false, true},
		{"far above max", 100, false, true},
		{"zero allowed but negative still invalid", -1, true, true},
		{"zero allowed but above max still invalid", 4, true, true},
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

func TestEffectivePrioritySortsUnsetAndOffScaleAfterLow(t *testing.T) {
	tests := []struct {
		name  string
		value int
		want  int
	}{
		{"unset", PriorityUnset, 4},
		{"critical", PriorityMin, PriorityMin},
		{"medium", PriorityMedium, PriorityMedium},
		{"low", PriorityLow, PriorityLow},
		{"negative", -3, 4},
		{"above max", PriorityMax + 1, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectivePriority(tt.value); got != tt.want {
				t.Errorf("EffectivePriority(%d) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestPriorityNameUsesCanonicalVocabulary(t *testing.T) {
	want := map[int]string{
		PriorityUnset:  "Unset",
		PriorityHigh:   "High",
		PriorityMedium: "Medium",
		PriorityLow:    "Low",
		99:             "",
	}
	for priority, name := range want {
		if got := PriorityName(priority); got != name {
			t.Errorf("PriorityName(%d) = %q, want %q", priority, got, name)
		}
	}
}
