package tui

import (
	"errors"
	"testing"
)

func TestWrapCursor(t *testing.T) {
	tests := []struct {
		name    string
		current int
		listLen int
		delta   int
		want    int
	}{
		{
			name:    "wrap down from last to first",
			current: 4,
			listLen: 5,
			delta:   1,
			want:    0,
		},
		{
			name:    "wrap up from first to last",
			current: 0,
			listLen: 5,
			delta:   -1,
			want:    4,
		},
		{
			name:    "normal move down",
			current: 2,
			listLen: 5,
			delta:   1,
			want:    3,
		},
		{
			name:    "normal move up",
			current: 2,
			listLen: 5,
			delta:   -1,
			want:    1,
		},
		{
			name:    "empty list returns zero",
			current: 0,
			listLen: 0,
			delta:   1,
			want:    0,
		},
		{
			name:    "empty list up returns zero",
			current: 0,
			listLen: 0,
			delta:   -1,
			want:    0,
		},
		{
			name:    "single element wrap down",
			current: 0,
			listLen: 1,
			delta:   1,
			want:    0,
		},
		{
			name:    "single element wrap up",
			current: 0,
			listLen: 1,
			delta:   -1,
			want:    0,
		},
		{
			name:    "move to first from second",
			current: 1,
			listLen: 5,
			delta:   -1,
			want:    0,
		},
		{
			name:    "move to last from second-to-last",
			current: 3,
			listLen: 5,
			delta:   1,
			want:    4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WrapCursor(tt.current, tt.listLen, tt.delta)
			if got != tt.want {
				t.Errorf("WrapCursor(%d, %d, %d) = %d, want %d", tt.current, tt.listLen, tt.delta, got, tt.want)
			}
		})
	}
}

func TestBaseView_Dimensions(t *testing.T) {
	v := &BaseView{}
	v.SetSize(100, 50)

	if got := v.Width(); got != 100 {
		t.Errorf("Width() = %d, want 100", got)
	}
	if got := v.Height(); got != 50 {
		t.Errorf("Height() = %d, want 50", got)
	}
	if got := v.ContentWidth(); got != 96 {
		t.Errorf("ContentWidth() = %d, want 96", got)
	}
	if got := v.ContentHeight(); got != 48 {
		t.Errorf("ContentHeight() = %d, want 48", got)
	}
}

func TestBaseView_ContentDimensionsMinimum(t *testing.T) {
	v := &BaseView{}
	v.SetSize(10, 5)

	if got := v.ContentWidth(); got != 6 {
		t.Errorf("ContentWidth() = %d, want 6", got)
	}
	if got := v.ContentHeight(); got != 3 {
		t.Errorf("ContentHeight() = %d, want 3", got)
	}
}

func TestBaseView_ContentDimensionsZeroFloor(t *testing.T) {
	v := &BaseView{}
	v.SetSize(2, 1)

	if got := v.ContentWidth(); got != 0 {
		t.Errorf("ContentWidth() = %d, want 0", got)
	}
	if got := v.ContentHeight(); got != 0 {
		t.Errorf("ContentHeight() = %d, want 0", got)
	}
}

func TestBaseView_Error(t *testing.T) {
	v := &BaseView{}

	if v.HasError() {
		t.Error("HasError() = true, want false for new view")
	}
	if v.Error() != nil {
		t.Error("Error() != nil, want nil for new view")
	}

	testErr := errTestError{}
	v.SetError(testErr)

	if !v.HasError() {
		t.Error("HasError() = false, want true after SetError")
	}
	if !errors.Is(v.Error(), testErr) {
		t.Error("Error() did not return set error")
	}

	v.ClearError()
	if v.HasError() {
		t.Error("HasError() = true, want false after ClearError")
	}
}

type errTestError struct{}

func (errTestError) Error() string { return "test error" }

func TestBaseSearchableView_SearchWorkflow(t *testing.T) {
	v := &BaseSearchableView{}
	v.InitSearchInput("Search...", 100, 40)

	if v.HasActiveSearch() {
		t.Error("HasActiveSearch() = true, want false initially")
	}
	if v.SearchQuery() != "" {
		t.Errorf("SearchQuery() = %q, want empty string initially", v.SearchQuery())
	}

	_ = v.ActivateSearch()
	if !v.HasActiveSearch() {
		t.Error("HasActiveSearch() = false after ActivateSearch")
	}

	// SetSearchQuery sets the query field directly (for programmatic use)
	v.SetSearchQuery("test query")
	if v.SearchQuery() != "test query" {
		t.Errorf("SearchQuery() = %q, want %q", v.SearchQuery(), "test query")
	}

	// ConfirmSearch reads from the input field, so set that too
	v.searchInput.SetValue("confirmed query")
	v.ConfirmSearch()
	if v.HasActiveSearch() {
		t.Error("HasActiveSearch() = true after ConfirmSearch")
	}
	if v.SearchQuery() != "confirmed query" {
		t.Errorf("SearchQuery() = %q, want %q after confirm", v.SearchQuery(), "confirmed query")
	}

	v.DeactivateSearch()
	if v.SearchQuery() != "" {
		t.Errorf("SearchQuery() = %q, want empty after DeactivateSearch", v.SearchQuery())
	}
}

func TestBaseSearchableView_ClearSearch(t *testing.T) {
	v := &BaseSearchableView{}
	v.InitSearchInput("Search...", 100, 40)

	_ = v.ActivateSearch()
	v.SetSearchQuery("some query")

	v.ClearSearch()
	if v.HasActiveSearch() {
		t.Error("HasActiveSearch() = true after ClearSearch")
	}
	if v.SearchQuery() != "" {
		t.Errorf("SearchQuery() = %q, want empty after ClearSearch", v.SearchQuery())
	}
}
