package capturetext

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// The unknown-project refusal carries the value it names, so a caller offering
// to create the project reads a field rather than parsing the message.
func TestUnknownProjectRefusalIsTyped(t *testing.T) {
	inbox, err := models.NewProject("p1", "inbox", "")
	if err != nil {
		t.Fatal(err)
	}
	projects := []*models.Project{inbox}

	cases := []struct {
		text      string
		unknown   string
		creatable bool
	}{
		{"Plant beans %garden", "garden", true},
		{"Plant beans %Garden", "Garden", false},
		{"Plant beans %garden_shed", "garden_shed", false},
		{"Plant beans $ready", "", false},
	}
	for _, tc := range cases {
		_, problems := Requests(tc.text, inbox, projects, Defaults{})
		if len(problems) != 1 {
			t.Fatalf("%q: problems = %+v, want one", tc.text, problems)
		}
		if problems[0].UnknownProject != tc.unknown {
			t.Fatalf("%q: UnknownProject = %q, want %q", tc.text, problems[0].UnknownProject, tc.unknown)
		}
		slug, creatable := problems[0].CreatableProject()
		if creatable != tc.creatable || (creatable && slug != tc.unknown) {
			t.Fatalf("%q: CreatableProject = %q, %v, want %v", tc.text, slug, creatable, tc.creatable)
		}
	}
}
