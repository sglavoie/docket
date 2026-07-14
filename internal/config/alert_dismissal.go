package config

import "time"

type AlertDismissal struct {
	Key         string    `json:"key"`
	Category    string    `json:"category"`
	DismissedAt time.Time `json:"dismissed_at"`
}

const alertDismissalMaxAge = 30 * 24 * time.Hour
