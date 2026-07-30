package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

func pinnedTagFromTab(tab string) string {
	return models.NormalizeTag(tab)
}
