package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetSectionLimitClampsNegativeToDisabled(t *testing.T) {
	cfg := NewConfig()
	negative := -1
	cfg.SectionLimitNext = &negative

	if got := cfg.GetSectionLimit("Next"); got != 0 {
		t.Fatalf("expected negative section limit clamped to 0, got %d", got)
	}
}

func TestConfigManagerLoadClampsNegativeSectionLimits(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, err := json.Marshal(map[string]any{
		"version":               CurrentConfigVersion,
		"data_dir":              "/tmp/tt-test",
		"projects":              []any{},
		"section_limit_active":  -1,
		"section_limit_next":    -2,
		"section_limit_inbox":   -3,
		"section_limit_standby": -4,
		"section_limit_waiting": -5,
		"section_limit_someday": -6,
		"section_limit_review":  -7,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	limits := map[string]*int{
		"active":  cfg.SectionLimitActive,
		"next":    cfg.SectionLimitNext,
		"inbox":   cfg.SectionLimitInbox,
		"standby": cfg.SectionLimitStandby,
		"waiting": cfg.SectionLimitWaiting,
		"someday": cfg.SectionLimitSomeday,
		"review":  cfg.SectionLimitReview,
	}
	for name, limit := range limits {
		if limit == nil {
			t.Fatalf("%s section limit should remain set after clamping", name)
		}
		if *limit != 0 {
			t.Errorf("expected %s section limit clamped to 0, got %d", name, *limit)
		}
	}
}
