package privateapi

import (
	"testing"

	"github.com/lox/notion-cli/internal/config"
)

func TestMissingAuthFields(t *testing.T) {
	t.Parallel()

	missing := missingAuthFields(config.PrivateAPIConfig{})
	if len(missing) != 2 {
		t.Fatalf("expected 2 missing fields, got %v", missing)
	}
	if missing[0] != "token_v2" || missing[1] != "notion_user_id" {
		t.Fatalf("unexpected missing fields order/content: %v", missing)
	}
}

func TestApplyViewOptionsEncodesValues(t *testing.T) {
	t.Parallel()

	show := true
	wrap := false
	open := "center-peek"
	preview := "page-cover"
	size := "large"
	layout := "compact"

	format := map[string]any{}
	applyViewOptions(format, ViewOptions{
		ShowPageIcon: &show,
		WrapContent:  &wrap,
		OpenPagesIn:  &open,
		CardPreview:  &preview,
		CardSize:     &size,
		CardLayout:   &layout,
	}, "table")

	if got := format["show_page_icon"]; got != true {
		t.Fatalf("show_page_icon mismatch: %v", got)
	}
	if got := format["wrap_all_content"]; got != false {
		t.Fatalf("wrap_all_content mismatch: %v", got)
	}
	if got := format["table_wrap"]; got != false {
		t.Fatalf("table_wrap mismatch: %v", got)
	}
	if got := format["open_page_in"]; got != "center_peek" {
		t.Fatalf("open_page_in mismatch: %v", got)
	}
	if got := format["card_preview"]; got != "page_cover" {
		t.Fatalf("card_preview mismatch: %v", got)
	}
	if got := format["card_size"]; got != "large" {
		t.Fatalf("card_size mismatch: %v", got)
	}
	if got := format["card_layout_mode"]; got != "default" {
		t.Fatalf("card_layout_mode mismatch: %v", got)
	}
}

func TestEncodeCardPreview(t *testing.T) {
	t.Parallel()
	if got := encodeCardPreview("files-media"); got != "files_and_media" {
		t.Fatalf("expected files_and_media, got %s", got)
	}
}

func TestFreshFormatForLayoutIncludesCollectionPointer(t *testing.T) {
	t.Parallel()

	format := freshFormatForLayout("chart", "collection-id", "space-id", map[string]any{
		"title": map[string]any{
			"type": "title",
		},
	})
	ptr := toMap(format["collection_pointer"])
	if len(ptr) == 0 {
		t.Fatal("expected collection_pointer to be present")
	}
	if got := ptr["id"]; got != "collection-id" {
		t.Fatalf("collection_pointer.id mismatch: %v", got)
	}
	if got := ptr["table"]; got != "collection" {
		t.Fatalf("collection_pointer.table mismatch: %v", got)
	}
	if got := ptr["spaceId"]; got != "space-id" {
		t.Fatalf("collection_pointer.spaceId mismatch: %v", got)
	}
	if cfg := toMap(format["chart_config"]); len(cfg) == 0 {
		t.Fatal("expected chart_config for chart layout")
	}
}

func TestEnsureCollectionPointerKeepsExistingPointer(t *testing.T) {
	t.Parallel()

	format := map[string]any{
		"collection_pointer": map[string]any{
			"id":      "existing-collection",
			"table":   "collection",
			"spaceId": "existing-space",
		},
	}

	ensureCollectionPointer(format, "new-collection", "new-space")
	ptr := toMap(format["collection_pointer"])
	if got := ptr["id"]; got != "existing-collection" {
		t.Fatalf("expected existing collection_pointer.id to be preserved, got %v", got)
	}
	if got := ptr["spaceId"]; got != "existing-space" {
		t.Fatalf("expected existing collection_pointer.spaceId to be preserved, got %v", got)
	}
}

func TestChooseDefaultChartPropertyPrefersSelectLikeTypes(t *testing.T) {
	t.Parallel()

	propertyID, propertyType := chooseDefaultChartProperty(map[string]any{
		"title": map[string]any{"type": "title"},
		"stat":  map[string]any{"type": "status"},
		"num":   map[string]any{"type": "number"},
	})
	if propertyID != "stat" || propertyType != "status" {
		t.Fatalf("unexpected default chart property: %s (%s)", propertyID, propertyType)
	}
}

func TestDefaultChartGroupByNumber(t *testing.T) {
	t.Parallel()

	groupBy := defaultChartGroupBy("amount", "number")
	if got := groupBy["type"]; got != "number" {
		t.Fatalf("unexpected type: %v", got)
	}
	if got := groupBy["property"]; got != "amount" {
		t.Fatalf("unexpected property: %v", got)
	}
	if got := toMap(groupBy["groupBy"])["type"]; got != "unique" {
		t.Fatalf("unexpected number groupBy: %v", groupBy["groupBy"])
	}
}
