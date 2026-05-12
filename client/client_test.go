package client

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	daptinClient "github.com/daptin/daptin-go-client"
)

func TestFindAllParsesValidListResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/usergroup" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("page[size]") != "10" {
			t.Fatalf("expected page[size]=10, got %q", r.URL.Query().Get("page[size]"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"1","type":"usergroup","attributes":{"name":"users"}}]}`))
	}))
	defer server.Close()

	c := New(server.URL, "", false)
	rows, err := c.FindAll("usergroup", daptinClient.DaptinQueryParameters{"page[size]": 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["id"] != "1" {
		t.Fatalf("expected row id 1, got %v", rows[0]["id"])
	}
}

func TestFindAllJoinTableNotFoundReturnsHelpfulError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	c := New(server.URL, "", false)
	_, err := c.FindAll("oauth_connect_oauth_connect_id_has_usergroup_usergroup_id", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "generated join table") {
		t.Fatalf("expected join table guidance, got: %v", err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("expected contextual join table error, got ErrNotFound: %v", err)
	}
}

func TestFindAllJoinTableMalformedResponseReturnsHelpfulError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":null}`))
	}))
	defer server.Close()

	c := New(server.URL, "", false)
	_, err := c.FindAll("user_account_user_account_id_has_usergroup_usergroup_id", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not exposed as API entities") {
		t.Fatalf("expected API entity guidance, got: %v", err)
	}
}

func TestFindAllMalformedResponseReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":null}`))
	}))
	defer server.Close()

	c := New(server.URL, "", false)
	_, err := c.FindAll("document", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "expected JSON:API data array") {
		t.Fatalf("expected malformed response error, got: %v", err)
	}
}
