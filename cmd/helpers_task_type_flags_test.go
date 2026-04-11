package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseTaskTypeFlags(t *testing.T) {
	tests := []struct {
		name     string
		epic     bool
		story    bool
		bug      bool
		idea     bool
		typeFlag string
		want     models.TaskType
		wantErr  bool
	}{
		{
			name: "default is task",
			want: models.TaskTypeTask,
		},
		{
			name: "epic flag",
			epic: true,
			want: models.TaskTypeEpic,
		},
		{
			name:  "story flag",
			story: true,
			want:  models.TaskTypeStory,
		},
		{
			name: "bug flag",
			bug:  true,
			want: models.TaskTypeBug,
		},
		{
			name: "idea flag",
			idea: true,
			want: models.TaskTypeIdea,
		},
		{
			name:     "type flag epic",
			typeFlag: "epic",
			want:     models.TaskTypeEpic,
		},
		{
			name:     "type flag story",
			typeFlag: "story",
			want:     models.TaskTypeStory,
		},
		{
			name:     "type flag task",
			typeFlag: "task",
			want:     models.TaskTypeTask,
		},
		{
			name:     "type flag bug",
			typeFlag: "bug",
			want:     models.TaskTypeBug,
		},
		{
			name:     "type flag idea",
			typeFlag: "idea",
			want:     models.TaskTypeIdea,
		},
		{
			name:     "invalid type flag",
			typeFlag: "invalid",
			wantErr:  true,
		},
		{
			name:     "conflicting boolean flag and type flag",
			epic:     true,
			typeFlag: "bug",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTaskTypeFlags(tt.epic, tt.story, tt.bug, tt.idea, tt.typeFlag)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTaskTypeFlags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseTaskTypeFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}
