package tui

import "github.com/sglavoie/tt/internal/models"

func (m *AppModel) activeOutliner() *OutlinerModel {
	if m == nil || m.navigation.Primary.Current().ID() != TaskWorkspace {
		return nil
	}
	return m.crossOutliner
}

func (m *AppModel) outlinerTarget() (*models.Task, *models.Project) {
	if out := m.activeOutliner(); out != nil {
		return out.getTaskRef()
	}
	return nil, nil
}
