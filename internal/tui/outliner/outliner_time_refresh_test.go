package outliner

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/util"
)

func TestTimeDrivenRefreshIsOwedOnlyAcrossCalendarDays(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	built := time.Date(2026, time.November, 1, 0, 1, 0, 0, location)
	outliner := &OutlinerModel{timeDrivenDay: util.StartOfDay(built)}

	if outliner.TimeDrivenRefreshOwedAt(built.Add(22 * time.Hour)) {
		t.Fatal("same local day was treated as a day-boundary refresh")
	}
	if !outliner.TimeDrivenRefreshOwedAt(built.AddDate(0, 0, 1)) {
		t.Fatal("next local day did not leave a refresh owed")
	}
	if (&OutlinerModel{}).TimeDrivenRefreshOwedAt(built) {
		t.Fatal("an unbuilt projection invented a refresh debt")
	}
}

func TestTimeDrivenInvalidationPreservesCursorAndPeekAcrossPromotion(t *testing.T) {
	promoted := primaryRowsTask("promoted", models.StateBacklog)
	stable := primaryRowsTask("stable", models.StateBacklog)
	project := primaryRowsProject("alpha", promoted, stable)
	outliner, _ := primaryRowsOutliner(project)
	outliner.SelectOutlinerMode(chrome.ModeAll)
	identity := taskref.For(stable, project)
	if !outliner.MoveCursorToIdentity(identity) {
		t.Fatal("cursor did not land on the stable task")
	}
	outliner.PeekMode = true

	// Moving the earlier task into Review reshapes the rows around the cursor.
	// The periodic invalidation must keep the selected identity rather than the
	// old numeric index, and Peek remains open on that same owner.
	promoted.Review = true
	outliner.InvalidateTimeDrivenRows()
	if !outliner.Population.ForceRebuild {
		t.Fatal("time-driven invalidation did not bypass row memoization")
	}
	outliner.CrossProjectRebuildRows()

	if got := outliner.TaskIdentityForRow(outliner.Rows[outliner.Cursor]); got != identity {
		t.Fatalf("cursor after time-driven promotion = %+v, want %+v", got, identity)
	}
	if !outliner.PeekMode {
		t.Fatal("time-driven promotion closed Peek")
	}
	if outliner.timeDrivenCursorPending || outliner.Population.ForceRebuild {
		t.Fatalf("rebuild left refresh state pending: cursor=%t force=%t",
			outliner.timeDrivenCursorPending, outliner.Population.ForceRebuild)
	}
	if countTaskRows(outliner, promoted.ID) != 1 || primarySectionForTask(outliner, promoted.ID) != "Review" {
		t.Fatal("promoted task was not deduplicated in Review")
	}
}
