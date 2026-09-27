package status

import (
	"strings"
	"testing"
	"time"
)

func TestViewStatusDurationScalesWithTheText(t *testing.T) {
	if got := ViewStatusDuration(""); got != StatusUpdateTimeout {
		t.Fatalf("empty text lasts %v, want the %v floor", got, StatusUpdateTimeout)
	}
	short := ViewStatusDuration("Saved")
	long := ViewStatusDuration("Cannot rename: note has a parse error; press Enter to repair it")
	if short >= long {
		t.Fatalf("a short notice lasts %v, not less than a long one's %v", short, long)
	}
	if got := ViewStatusDuration(strings.Repeat("x", 1000)); got != 6*time.Second {
		t.Fatalf("a very long notice lasts %v, want the 6s ceiling", got)
	}
}
