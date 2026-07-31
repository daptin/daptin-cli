package cmd

import "testing"

func TestParseCreateBodyPreservesJSONAPIRelationships(t *testing.T) {
	payload := `{"data":{"type":"mail_account","attributes":{"username":"probe@example.invalid"},"relationships":{"mail_server_id":{"data":{"type":"mail_server","id":"server-ref"}}}}}`
	body, err := parseCreateBody("mail_account", []string{payload})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data := body["data"].(map[string]interface{})
	if _, nested := data["attributes"].(map[string]interface{})["data"]; nested {
		t.Fatalf("JSON:API payload was nested under attributes: %#v", body)
	}
	relationships := data["relationships"].(map[string]interface{})
	mailServer := relationships["mail_server_id"].(map[string]interface{})
	related := mailServer["data"].(map[string]interface{})
	if related["id"] != "server-ref" {
		t.Fatalf("relationship was not preserved: %#v", body)
	}
}

func TestParseCreateBodyWrapsAttributes(t *testing.T) {
	body, err := parseCreateBody("document", []string{"name=probe"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data := body["data"].(map[string]interface{})
	attrs := data["attributes"].(map[string]interface{})
	if data["type"] != "document" || attrs["name"] != "probe" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestParseCreateBodyRejectsMismatchedType(t *testing.T) {
	_, err := parseCreateBody("mail_account", []string{`{"data":{"type":"document","attributes":{}}}`})
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
}
