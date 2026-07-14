package config

import "testing"

func TestGetSectionLimitClampsNegativeToDisabled(t *testing.T) {
	cfg := NewSettings()
	negative := -1
	cfg.SectionLimitNext = &negative

	if got := cfg.GetSectionLimit("Next"); got != 0 {
		t.Fatalf("expected negative section limit clamped to 0, got %d", got)
	}
}
