package config

import "testing"

func TestGetResumeCardLimit(t *testing.T) {
	cases := []struct {
		name string
		val  *int
		want int
	}{
		{name: "nil returns default", val: nil, want: DefaultResumeCardLimit},
		{name: "zero returns default", val: resumeLimitIntPtr(0), want: DefaultResumeCardLimit},
		{name: "valid passes through", val: resumeLimitIntPtr(8), want: 8},
		{name: "above upper bound clamps", val: resumeLimitIntPtr(999), want: ResumeCardLimitUpperBound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.ResumeCardLimit = tc.val
			if got := cfg.GetResumeCardLimit(); got != tc.want {
				t.Fatalf("GetResumeCardLimit() = %d, want %d", got, tc.want)
			}
		})
	}
}

func resumeLimitIntPtr(v int) *int {
	return &v
}
