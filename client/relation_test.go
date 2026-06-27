package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRelationshipJoinTableName(t *testing.T) {
	got := RelationshipJoinTableName("world", "usergroup_id")
	want := "world_world_id_has_usergroup_usergroup_id"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}

	got = RelationshipJoinTableName("oauth_connect", "usergroup_id")
	want = "oauth_connect_oauth_connect_id_has_usergroup_usergroup_id"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestUpdateRelationAttributesPatchesGeneratedJoinRow(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/world/doc-ref/usergroup_id":
			_, _ = w.Write([]byte(`{"data":[{"id":"relation-ref","type":"usergroup","attributes":{"reference_id":"relation-ref","relation_reference_id":"users-ref"},"relationships":{"world_id":{"data":[]}}}]}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/world_world_id_has_usergroup_usergroup_id/relation-ref":
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode patch body: %v", err)
			}

			data := body["data"].(map[string]interface{})
			if data["type"] != "world_world_id_has_usergroup_usergroup_id" {
				t.Fatalf("unexpected type: %v", data["type"])
			}
			if data["id"] != "relation-ref" {
				t.Fatalf("unexpected id: %v", data["id"])
			}

			attrs := data["attributes"].(map[string]interface{})
			if attrs["permission"] != float64(1032192) {
				t.Fatalf("expected numeric permission, got %#v", attrs["permission"])
			}
			_, _ = w.Write([]byte(`{"data":{"id":"relation-ref","type":"world_world_id_has_usergroup_usergroup_id","attributes":{"reference_id":"relation-ref","permission":1032192}}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := New(server.URL, "", false)
	_, err := c.UpdateRelationAttributes("world", "doc-ref", "usergroup_id", "users-ref", map[string]interface{}{"permission": int64(1032192)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected 2 requests, got %d", requests)
	}
}

func TestUpdateRelationAttributesMissingTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/world/doc-ref/usergroup_id" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"relation-ref","type":"usergroup","attributes":{"reference_id":"relation-ref","world_id":"doc-ref","relation_reference_id":"admins-ref"}}]}`))
	}))
	defer server.Close()

	c := New(server.URL, "", false)
	_, err := c.UpdateRelationAttributes("world", "doc-ref", "usergroup_id", "users-ref", map[string]interface{}{"permission": int64(1032192)})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "relationship row not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}
