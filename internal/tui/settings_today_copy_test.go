package tui

import (
	"testing"
)

func TestSettingsSearchRetiresTodayPresentationControl(t *testing.T) {
	if results := searchSettingsFields("today mode"); len(results) != 0 {
		t.Fatalf("search for retired Today mode returned %#v", results)
	}
}

func TestSettingsSearchRetiresStartViewControl(t *testing.T) {
	// The configurable startup destination is gone: the Outliner always launches
	// globally in Today, so no Start View / Startup control should remain.
	for _, query := range []string{"start view", "startup destination"} {
		if results := searchSettingsFields(query); len(results) != 0 {
			t.Fatalf("search %q returned retired start-view fields: %#v", query, results)
		}
	}
}
