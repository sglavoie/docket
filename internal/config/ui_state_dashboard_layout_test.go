package config

import "testing"

func TestGetDashboardBottomLayout_DefaultHorizontal(t *testing.T) {
	cfg := NewConfig()

	if got := cfg.GetDashboardBottomLayout(); got != DashboardLayoutHorizontal {
		t.Errorf("expected DashboardLayoutHorizontal (0) by default, got %d", got)
	}
}

func TestGetDashboardBottomLayout_ExplicitValues(t *testing.T) {
	cases := []struct {
		stored int
		want   DashboardBottomLayout
	}{
		{0, DashboardLayoutHorizontal},
		{1, DashboardLayoutVertical},
		{2, DashboardLayoutListOnly},
	}
	for _, tc := range cases {
		cfg := NewConfig()
		v := tc.stored
		cfg.DashboardBottomLayout = &v
		if got := cfg.GetDashboardBottomLayout(); got != tc.want {
			t.Errorf("stored=%d: expected %d, got %d", tc.stored, tc.want, got)
		}
	}
}

func TestGetDashboardBottomLayout_ClampInvalid(t *testing.T) {
	cases := []int{-1, 3, 99}
	for _, stored := range cases {
		cfg := NewConfig()
		v := stored
		cfg.DashboardBottomLayout = &v
		if got := cfg.GetDashboardBottomLayout(); got != DashboardLayoutHorizontal {
			t.Errorf("stored=%d: expected clamp to DashboardLayoutHorizontal, got %d", stored, got)
		}
	}
}

func TestSetDashboardBottomLayout(t *testing.T) {
	cfg := NewConfig()
	cfg.SetDashboardBottomLayout(DashboardLayoutVertical)

	if cfg.DashboardBottomLayout == nil {
		t.Fatal("DashboardBottomLayout should not be nil after Set")
	}
	if *cfg.DashboardBottomLayout != int(DashboardLayoutVertical) {
		t.Errorf("expected stored value 1, got %d", *cfg.DashboardBottomLayout)
	}
	if got := cfg.GetDashboardBottomLayout(); got != DashboardLayoutVertical {
		t.Errorf("expected DashboardLayoutVertical, got %d", got)
	}
}

func TestGetDashboardHorizontalSplitPct_Default(t *testing.T) {
	cfg := NewConfig()
	if got := cfg.GetDashboardHorizontalSplitPct(); got != DefaultDashboardHorizontalSplitPct {
		t.Errorf("expected default %d, got %d", DefaultDashboardHorizontalSplitPct, got)
	}
}

func TestGetDashboardHorizontalSplitPct_ClampStored(t *testing.T) {
	cases := []struct {
		stored int
		want   int
	}{
		{0, MinDashboardSplitPct},
		{5, MinDashboardSplitPct},
		{10, 10},
		{50, 50},
		{90, 90},
		{95, MaxDashboardSplitPct},
		{1000, MaxDashboardSplitPct},
		{-5, MinDashboardSplitPct},
	}
	for _, tc := range cases {
		cfg := NewConfig()
		v := tc.stored
		cfg.DashboardHorizontalSplitPct = &v
		if got := cfg.GetDashboardHorizontalSplitPct(); got != tc.want {
			t.Errorf("stored=%d: expected %d, got %d", tc.stored, tc.want, got)
		}
	}
}

func TestSetDashboardHorizontalSplitPct_Clamps(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{5, MinDashboardSplitPct},
		{50, 50},
		{99, MaxDashboardSplitPct},
	}
	for _, tc := range cases {
		cfg := NewConfig()
		cfg.SetDashboardHorizontalSplitPct(tc.in)
		if cfg.DashboardHorizontalSplitPct == nil {
			t.Fatalf("in=%d: pointer should not be nil after Set", tc.in)
		}
		if *cfg.DashboardHorizontalSplitPct != tc.want {
			t.Errorf("in=%d: expected stored %d, got %d", tc.in, tc.want, *cfg.DashboardHorizontalSplitPct)
		}
	}
}

func TestGetDashboardVerticalSplitPct_Default(t *testing.T) {
	cfg := NewConfig()
	if got := cfg.GetDashboardVerticalSplitPct(); got != DefaultDashboardVerticalSplitPct {
		t.Errorf("expected default %d, got %d", DefaultDashboardVerticalSplitPct, got)
	}
}

func TestGetDashboardVerticalSplitPct_ClampStored(t *testing.T) {
	cases := []struct {
		stored int
		want   int
	}{
		{0, MinDashboardSplitPct},
		{10, 10},
		{60, 60},
		{90, 90},
		{200, MaxDashboardSplitPct},
		{-1, MinDashboardSplitPct},
	}
	for _, tc := range cases {
		cfg := NewConfig()
		v := tc.stored
		cfg.DashboardVerticalSplitPct = &v
		if got := cfg.GetDashboardVerticalSplitPct(); got != tc.want {
			t.Errorf("stored=%d: expected %d, got %d", tc.stored, tc.want, got)
		}
	}
}

func TestSetDashboardVerticalSplitPct_Clamps(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{1, MinDashboardSplitPct},
		{40, 40},
		{500, MaxDashboardSplitPct},
	}
	for _, tc := range cases {
		cfg := NewConfig()
		cfg.SetDashboardVerticalSplitPct(tc.in)
		if cfg.DashboardVerticalSplitPct == nil {
			t.Fatalf("in=%d: pointer should not be nil after Set", tc.in)
		}
		if *cfg.DashboardVerticalSplitPct != tc.want {
			t.Errorf("in=%d: expected stored %d, got %d", tc.in, tc.want, *cfg.DashboardVerticalSplitPct)
		}
	}
}
