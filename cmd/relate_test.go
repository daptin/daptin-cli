package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daptin/daptin-cli/config"
)

func TestRelateWithAttributes(t *testing.T) {
	step := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step++
		w.Header().Set("Content-Type", "application/json")

		switch step {
		case 1:
			if r.Method != http.MethodPatch || r.URL.Path != "/api/world/doc-ref/relationships/usergroup_id" {
				t.Fatalf("unexpected relation request: %s %s", r.Method, r.URL.Path)
			}
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode relation body: %v", err)
			}
			data := body["data"].([]interface{})[0].(map[string]interface{})
			if data["type"] != "usergroup" || data["id"] != "users-ref" {
				t.Fatalf("unexpected relation data: %#v", data)
			}
			w.WriteHeader(http.StatusNoContent)
		case 2:
			if r.Method != http.MethodGet || r.URL.Path != "/api/world/doc-ref/usergroup_id" {
				t.Fatalf("unexpected related request: %s %s", r.Method, r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"relation-ref","type":"usergroup","attributes":{"reference_id":"relation-ref","world_id":"doc-ref","relation_reference_id":"users-ref"}}]}`))
		case 3:
			if r.Method != http.MethodPatch || r.URL.Path != "/api/world_world_id_has_usergroup_usergroup_id/relation-ref" {
				t.Fatalf("unexpected relation update request: %s %s", r.Method, r.URL.Path)
			}
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode relation update body: %v", err)
			}
			data := body["data"].(map[string]interface{})
			attrs := data["attributes"].(map[string]interface{})
			if attrs["permission"] != float64(1032192) {
				t.Fatalf("expected numeric permission, got %#v", attrs["permission"])
			}
			_, _ = w.Write([]byte(`{"data":{"id":"relation-ref","type":"world_world_id_has_usergroup_usergroup_id","attributes":{"reference_id":"relation-ref","permission":1032192}}}`))
		default:
			t.Fatalf("unexpected extra request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	app := NewApp(&config.Config{}, "test")
	err := app.Run([]string{"daptin", "--quiet", "--endpoint", server.URL, "relate", "world", "doc-ref", "usergroup_id", "users-ref", "permission=1032192"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if step != 3 {
		t.Fatalf("expected 3 requests, got %d", step)
	}
}

func TestUpdateRelationCommand(t *testing.T) {
	step := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step++
		w.Header().Set("Content-Type", "application/json")

		switch step {
		case 1:
			if r.Method != http.MethodGet || r.URL.Path != "/api/world/doc-ref/usergroup_id" {
				t.Fatalf("unexpected related request: %s %s", r.Method, r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"relation-ref","type":"usergroup","attributes":{"reference_id":"relation-ref","world_id":"doc-ref","relation_reference_id":"users-ref"}}]}`))
		case 2:
			if r.Method != http.MethodPatch || r.URL.Path != "/api/world_world_id_has_usergroup_usergroup_id/relation-ref" {
				t.Fatalf("unexpected relation update request: %s %s", r.Method, r.URL.Path)
			}
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode relation update body: %v", err)
			}
			data := body["data"].(map[string]interface{})
			attrs := data["attributes"].(map[string]interface{})
			if attrs["permission"] != float64(1032192) {
				t.Fatalf("expected numeric permission, got %#v", attrs["permission"])
			}
			_, _ = w.Write([]byte(`{"data":{"id":"relation-ref","type":"world_world_id_has_usergroup_usergroup_id","attributes":{"reference_id":"relation-ref","permission":1032192}}}`))
		default:
			t.Fatalf("unexpected extra request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	app := NewApp(&config.Config{}, "test")
	err := app.Run([]string{"daptin", "--quiet", "--endpoint", server.URL, "update-relation", "world", "doc-ref", "usergroup_id", "users-ref", "permission=1032192"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if step != 2 {
		t.Fatalf("expected 2 requests, got %d", step)
	}
}

func TestParseRelationAttributesInfersScalarTypes(t *testing.T) {
	attrs, err := parseRelationAttributes([]string{"permission=1032192", "enabled=true", "ratio=1.5", "one=1", "zero=0", "name=users"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attrs["permission"] != int64(1032192) {
		t.Fatalf("expected int permission, got %#v", attrs["permission"])
	}
	if attrs["enabled"] != true {
		t.Fatalf("expected bool enabled, got %#v", attrs["enabled"])
	}
	if attrs["ratio"] != float64(1.5) {
		t.Fatalf("expected float ratio, got %#v", attrs["ratio"])
	}
	if attrs["one"] != int64(1) {
		t.Fatalf("expected int one, got %#v", attrs["one"])
	}
	if attrs["zero"] != int64(0) {
		t.Fatalf("expected int zero, got %#v", attrs["zero"])
	}
	if attrs["name"] != "users" {
		t.Fatalf("expected string name, got %#v", attrs["name"])
	}
}
