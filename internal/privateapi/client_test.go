package privateapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestDeleteDatabaseView(t *testing.T) {
	t.Parallel()

	var txnReq submitTransactionRequest
	recordRequests := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/getRecordValues":
			recordRequests++
			defer func() { _ = r.Body.Close() }()

			var req getRecordValuesRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode getRecordValues request: %v", err)
			}

			w.Header().Set("Content-Type", "application/json")
			if recordRequests == 1 {
				_, _ = w.Write([]byte(`{
					"results":[
						{
							"id":"db_1",
							"table":"block",
							"value":{
								"collection_id":"col_1",
								"space_id":"space_1",
								"view_ids":["view_keep","view_drop"]
							}
						}
					]
				}`))
				return
			}
			_, _ = w.Write([]byte(`{
				"results":[
					{"id":"view_keep","table":"collection_view","value":{"id":"view_keep","name":"Keep","type":"table","format":{}}},
					{"id":"view_drop","table":"collection_view","value":{"id":"view_drop","name":"Drop","type":"board","format":{"x":1}}}
				]
			}`))
			return

		case "/api/v3/submitTransaction":
			defer func() { _ = r.Body.Close() }()
			if err := json.NewDecoder(r.Body).Decode(&txnReq); err != nil {
				t.Fatalf("decode submitTransaction request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}

		t.Fatalf("unexpected request path: %s", r.URL.Path)
	}))
	defer srv.Close()

	client, err := NewClient(config.PrivateAPIConfig{
		BaseURL:      srv.URL,
		TokenV2:      "token",
		NotionUserID: "user",
		SpaceID:      "space_1",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	deleted, err := client.DeleteDatabaseView(context.Background(), ViewDeleteRequest{
		DatabaseID: "db_1",
		ViewID:     "view_drop",
	})
	if err != nil {
		t.Fatalf("delete database view: %v", err)
	}

	if deleted.ID != "view_drop" {
		t.Fatalf("deleted id mismatch: %q", deleted.ID)
	}
	if deleted.Name != "Drop" {
		t.Fatalf("deleted name mismatch: %q", deleted.Name)
	}
	if deleted.Layout != "board" {
		t.Fatalf("deleted layout mismatch: %q", deleted.Layout)
	}

	if len(txnReq.Operations) != 4 {
		t.Fatalf("expected 4 transaction operations, got %d", len(txnReq.Operations))
	}

	var viewIDs []string
	var aliveSet bool
	for _, op := range txnReq.Operations {
		if op.Table == "block" && len(op.Path) == 1 && op.Path[0] == "view_ids" {
			raw, ok := op.Args.([]any)
			if !ok {
				t.Fatalf("view_ids args type mismatch: %#v", op.Args)
			}
			for _, item := range raw {
				if s, ok := item.(string); ok {
					viewIDs = append(viewIDs, s)
				}
			}
		}
		if op.Table == "collection_view" && op.ID == "view_drop" && len(op.Path) == 1 && op.Path[0] == "alive" {
			if v, ok := op.Args.(bool); ok && !v {
				aliveSet = true
			}
		}
	}
	if len(viewIDs) != 1 || viewIDs[0] != "view_keep" {
		t.Fatalf("unexpected post-delete view_ids: %#v", viewIDs)
	}
	if !aliveSet {
		t.Fatal("expected alive=false operation for deleted view")
	}
}
