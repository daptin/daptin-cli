package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompleteAssetUploadUsesDirectStreamCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/asset/product/product-ref/photo/upload" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("operation") != "complete" {
			t.Fatalf("unexpected operation: %q", r.URL.Query().Get("operation"))
		}
		if _, present := r.URL.Query()["upload_id"]; present {
			t.Fatalf("direct stream completion must not include upload_id")
		}
		if r.URL.Query().Get("filename") != "image one.png" {
			t.Fatalf("unexpected filename: %q", r.URL.Query().Get("filename"))
		}
		if r.Header.Get("X-File-Type") != "image/png" || r.Header.Get("X-File-Size") != "1234" {
			t.Fatalf("unexpected file headers: %#v", r.Header)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["fileName"] != "image one.png" || body["type"] != "image/png" || body["size"] != float64(1234) {
			t.Fatalf("unexpected completion body: %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"completed","fileName":"image one.png","size":1234}`))
	}))
	defer server.Close()

	result, err := New(server.URL, "", false).CompleteAssetUpload("product", "product-ref", "photo", "image one.png", "image/png", 1234)
	if err != nil {
		t.Fatalf("complete upload: %v", err)
	}
	if result["status"] != "completed" {
		t.Fatalf("unexpected result: %#v", result)
	}
}
