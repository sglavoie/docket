package storage

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

func FormatRelativeTime(t time.Time) string {
	diff := time.Since(t)
	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}
	if diff < util.Day {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	days := util.DaysBetween(t, time.Now())
	if days == 1 {
		return "yesterday"
	}
	return fmt.Sprintf("%d days ago", days)
}
