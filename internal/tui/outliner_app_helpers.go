package tui

import "github.com/sglavoie/tt/internal/models"

// visibleOutliner returns the sole primary Outliner instance.
func (m *AppModel) visibleOutliner() *OutlinerModel {
	if m == nil {
		return nil
	}
	return m.crossOutliner
}

func (m *AppModel) activeOutliner() *OutlinerModel {
	return m.visibleOutliner()
}

func (m *AppModel) outlinerTarget() (*models.Task, *models.Project) {
	if out := m.visibleOutliner(); out != nil {
		return out.getTaskRef()
	}
	return nil, nil
}
