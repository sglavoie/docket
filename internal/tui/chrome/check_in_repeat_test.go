package chrome

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestFormatCheckInRepeatIsTheOneCadenceSpelling pins the compact form Peek,
// the composer preview, and the date picker's now line all read.
func TestFormatCheckInRepeatIsTheOneCadenceSpelling(t *testing.T) {
	for days, want := range map[int]string{
		models.CheckInRepeatOff: "off",
		1:                       "every 1d",
		7:                       "every 7d",
	} {
		if got := FormatCheckInRepeat(days); got != want {
			t.Fatalf("FormatCheckInRepeat(%d) = %q, want %q", days, got, want)
		}
	}
}
