package listsurface

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestListSurfacePreviewGeometryFitsTheDefaultSplitToContent(t *testing.T) {
	cases := []struct {
		name        string
		percent     int
		content     int
		wantList    int
		wantPreview int
	}{
		{name: "short rows hand the rest to the preview", content: 40, wantList: 40, wantPreview: 77},
		{name: "never below the list minimum", content: 10, wantList: 24, wantPreview: 93},
		{name: "never past the default share", content: 100, wantList: 66, wantPreview: 51},
		{name: "a moved split wins", percent: 60, content: 40, wantList: 72, wantPreview: 45},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			list, preview := ListSurfacePreviewGeometry(ListSurfaceRenderConfig{
				Width:            120,
				ListWidthPercent: tc.percent,
				ListContentWidth: tc.content,
			})
			if list != tc.wantList || preview != tc.wantPreview {
				t.Fatalf("geometry = %d/%d, want %d/%d", list, preview, tc.wantList, tc.wantPreview)
			}
		})
	}
}

func TestListSurfaceNaturalRowWidthMeasuresTheWidestUntruncatedRow(t *testing.T) {
	widest := ListItem{ID: "a", CategoryID: "c", PrimaryText: "Alpha", SecondaryText: "1 open · 2 tasks"}
	model, err := NewListSurfaceModel([]ListItem{
		widest,
		{ID: "b", CategoryID: "c", PrimaryText: "Beta"},
	}, []ListCategory{{ID: "c", Label: "Projects"}}, ListSurfaceConfig{SelectionMode: ListSelectionMulti})
	if err != nil {
		t.Fatal(err)
	}
	row := ListSurfaceRow{Kind: ListRowItem, Match: ListMatch{Item: widest}}
	drawn := strings.TrimRight(ansi.Strip(RenderListSurfaceRow(row, ListSelectionMulti, 200)), " ")
	if got, want := model.NaturalRowWidth(), lipgloss.Width(drawn); got != want {
		t.Fatalf("NaturalRowWidth = %d, want the drawn row's %d (%q)", got, want, drawn)
	}
}

func TestListSurfaceRowPaintsTheSecondaryAccentOnlyWhereNothingElseOwnsTheLine(t *testing.T) {
	style := theme.ProjectPressureOverdueStyle
	item := ListItem{
		ID:                   "p",
		PrimaryText:          "Atlas",
		SecondaryText:        "7 open · 2 overdue · 9 tasks",
		SecondaryAccent:      "2 overdue",
		SecondaryAccentStyle: &style,
	}
	painted := style.Render("2 overdue")
	for _, state := range []struct {
		name  string
		row   ListSurfaceRow
		width int
		want  bool
	}{
		{name: "ordinary", row: ListSurfaceRow{}, width: 60, want: true},
		{name: "checked", row: ListSurfaceRow{Selected: true}, width: 60, want: false},
		{name: "cursor", row: ListSurfaceRow{Cursor: true}, width: 60, want: false},
		{name: "run cut by truncation", row: ListSurfaceRow{}, width: 24, want: false},
	} {
		t.Run(state.name, func(t *testing.T) {
			state.row.Match = ListMatch{Item: item}
			got := RenderListSurfaceRow(state.row, ListSelectionMulti, state.width)
			if has := strings.Contains(got, painted); has != state.want {
				t.Fatalf("accent painted = %v, want %v:\n%q", has, state.want, got)
			}
			plain := item
			plain.SecondaryAccent, plain.SecondaryAccentStyle = "", nil
			state.row.Match = ListMatch{Item: plain}
			if want := RenderListSurfaceRow(state.row, ListSelectionMulti, state.width); ansi.Strip(got) != ansi.Strip(want) {
				t.Fatalf("accent moved text: %q, want %q", ansi.Strip(got), ansi.Strip(want))
			}
		})
	}
}
