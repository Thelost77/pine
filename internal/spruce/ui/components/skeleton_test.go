package components

import (
	"testing"

	"github.com/Thelost77/pine/internal/spruce/ui"
)

func TestBuildSkeletonRows(t *testing.T) {
	styles := ui.DefaultStyles()
	rows := BuildSkeletonRows(styles)

	if len(rows) != 5 {
		t.Fatalf("expected 5 skeleton rows, got %d", len(rows))
	}

	for i, r := range rows {
		item, ok := r.(SkeletonItem)
		if !ok {
			t.Fatalf("row %d is not SkeletonItem: %T", i, r)
		}
		if item.Title() == "" {
			t.Errorf("row %d title is empty", i)
		}
		if item.Description() == "" {
			t.Errorf("row %d description is empty", i)
		}
		if item.FilterValue() != "" {
			t.Errorf("expected empty filter value for skeleton row %d, got %q", i, item.FilterValue())
		}
	}
}
