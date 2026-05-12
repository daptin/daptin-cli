package client

import (
	"errors"
	"testing"

	daptinClient "github.com/daptin/daptin-go-client"
)

func TestParseSingleResponse_ValidData(t *testing.T) {
	body := []byte(`{"data":{"id":"abc","type":"world","attributes":{"table_name":"users"}}}`)
	data, err := ParseSingleResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["id"] != "abc" {
		t.Errorf("expected id abc, got %v", data["id"])
	}
}

func TestParseSingleResponse_NullData(t *testing.T) {
	body := []byte(`{"data":null}`)
	_, err := ParseSingleResponse(body)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestParseSingleResponse_InvalidJSON(t *testing.T) {
	body := []byte(`not json`)
	_, err := ParseSingleResponse(body)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseListResponse_ValidData(t *testing.T) {
	body := []byte(`{"data":[{"id":"a","type":"t"},{"id":"b","type":"t"}]}`)
	items, err := ParseListResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0]["id"] != "a" {
		t.Errorf("expected a, got %v", items[0]["id"])
	}
}

func TestParseListResponse_EmptyArray(t *testing.T) {
	body := []byte(`{"data":[]}`)
	items, err := ParseListResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestParseListResponse_NoDataKey(t *testing.T) {
	body := []byte(`{"errors":[{"status":"404"}]}`)
	items, err := ParseListResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items != nil {
		t.Errorf("expected nil, got %v", items)
	}
}

func TestParseActionResponses(t *testing.T) {
	body := []byte(`[{"ResponseType":"client.notify","Attributes":{"message":"hello"}},{"ResponseType":"client.store.set","Attributes":{"key":"token","value":"jwt123"}}]`)
	responses, err := ParseActionResponses(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}
	if responses[0].ResponseType != "client.notify" {
		t.Errorf("expected client.notify, got %s", responses[0].ResponseType)
	}
	if responses[1].Attributes["value"] != "jwt123" {
		t.Errorf("expected jwt123, got %v", responses[1].Attributes["value"])
	}
}

func TestCheckStatusCode(t *testing.T) {
	tests := []struct {
		code   int
		expect error
	}{
		{200, nil},
		{201, nil},
		{204, nil},
		{401, ErrUnauthorized},
		{403, ErrForbidden},
		{404, ErrNotFound},
		{500, nil}, // returns generic error, not a sentinel
	}

	for _, tt := range tests {
		err := CheckStatusCode(tt.code, "body")
		if tt.expect == nil && tt.code < 400 {
			if err != nil {
				t.Errorf("code %d: expected nil, got %v", tt.code, err)
			}
		} else if tt.expect != nil {
			if !errors.Is(err, tt.expect) {
				t.Errorf("code %d: expected %v, got %v", tt.code, tt.expect, err)
			}
		} else if tt.code >= 400 {
			if err == nil {
				t.Errorf("code %d: expected error, got nil", tt.code)
			}
		}
	}
}

func TestBuildFindOneURL_NoParams(t *testing.T) {
	u := BuildFindOneURL("http://localhost:6336", "world", "abc-123", nil)
	expected := "http://localhost:6336/api/world/abc-123"
	if u != expected {
		t.Errorf("expected %s, got %s", expected, u)
	}
}

func TestBuildFindOneURL_WithParams(t *testing.T) {
	params := map[string]interface{}{
		"fields": "name,email",
	}
	u := BuildFindOneURL("http://localhost:6336", "user", "xyz", params)
	if u != "http://localhost:6336/api/user/xyz?fields=name%2Cemail" {
		t.Errorf("unexpected URL: %s", u)
	}
}

func TestBuildFindAllURL_WithParams(t *testing.T) {
	params := map[string]interface{}{
		"page[size]":   50,
		"page[number]": 2,
		"query":        `{"name":"admin"}`,
	}
	u := BuildFindAllURL("http://localhost:6336", "usergroup", params)
	expected := `http://localhost:6336/api/usergroup?page%5Bnumber%5D=2&page%5Bsize%5D=50&query=%7B%22name%22%3A%22admin%22%7D`
	if u != expected {
		t.Errorf("expected %s, got %s", expected, u)
	}
}

func TestMapArray(t *testing.T) {
	objects := []daptinClient.JsonApiObject{
		{"id": "1", "attributes": map[string]interface{}{"name": "a"}},
		{"id": "2", "attributes": map[string]interface{}{"name": "b"}},
		{"id": "3", "other": "no attributes"},
	}

	result := MapArray(objects, "attributes")
	if len(result) != 2 {
		t.Fatalf("expected 2 results (skipping item without attributes), got %d", len(result))
	}
	if result[0]["name"] != "a" {
		t.Errorf("expected a, got %v", result[0]["name"])
	}
	if result[1]["name"] != "b" {
		t.Errorf("expected b, got %v", result[1]["name"])
	}
}
