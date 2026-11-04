package tui

import (
	"strings"
	"testing"
)

func pressKeys(t *testing.T, driver *UITestDriver, keys ...string) UITestObservation {
	t.Helper()
	var obs UITestObservation
	for _, key := range keys {
		var err error
		if obs, err = driver.ApplyAction(UITestAction{Type: UITestActionKey, Key: key}, ObserveOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	return obs
}

func titleRowANSI(driver *UITestDriver) string {
	obs := driver.Observe(ObserveOptions{IncludeANSI: true})
	return strings.SplitN(obs.ANSIScreen, "\n", 2)[0]
}

func waitFor(t *testing.T, driver *UITestDriver, duration string) {
	t.Helper()
	if _, err := driver.ApplyAction(UITestAction{Type: UITestActionWait, Duration: duration}, ObserveOptions{}); err != nil {
		t.Fatal(err)
	}
}
