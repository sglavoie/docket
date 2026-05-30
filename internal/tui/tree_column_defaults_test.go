package tui

import "testing"

func TestInitTreeColumnDefaultsIncludesDescription(t *testing.T) {
	idx := projectTreeColumnIndex(ColDescription)
	if idx < 0 {
		t.Fatal("description column not found in project tree column index")
	}

	defaults := initTreeColumnDefaults(map[string]bool{ColDescription: false})
	if defaults[idx] {
		t.Fatalf("expected description default false at index %d", idx)
	}
}

func TestTreeColumnDefaultsToMapIncludesDescription(t *testing.T) {
	idx := projectTreeColumnIndex(ColDescription)
	if idx < 0 {
		t.Fatal("description column not found in project tree column index")
	}

	var defaults [15]bool
	for i := range defaults {
		defaults[i] = true
	}
	defaults[idx] = false

	got := treeColumnDefaultsToMap(defaults)
	if got == nil {
		t.Fatal("expected non-nil map when one column is hidden")
	}
	if v, ok := got[ColDescription]; !ok || v {
		t.Fatalf("expected description=false in map, got present=%v value=%v", ok, v)
	}
}

func TestProjectTreeColumnPickerIncludesDescription(t *testing.T) {
	cfg := NewColumnConfig(nil, nil, DefaultTreeColumns, "")
	m := NewColumnPickerModel(ShowColumnPickerMsg{
		Source:       ViewProjectTree,
		ColumnConfig: cfg,
	})

	foundDescription := false
	for _, item := range m.Items {
		if item.ID == ColDescription {
			foundDescription = true
			break
		}
	}
	if !foundDescription {
		t.Fatal("expected project tree column picker to include Description")
	}
	if len(m.Items) != len(AllColumns) {
		t.Fatalf("expected project tree picker to expose %d columns, got %d", len(AllColumns), len(m.Items))
	}
}

func projectTreeColumnIndex(id string) int {
	idx := 0
	for _, col := range AllColumns {
		if col.OutlinerOnly {
			continue
		}
		if col.ID == id {
			return idx
		}
		idx++
	}
	return -1
}
