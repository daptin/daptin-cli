package cmd

import (
	"encoding/base64"
	"testing"

	daptinClient "github.com/daptin/daptin-go-client"
)

func TestProcessResponses_Token(t *testing.T) {
	responses := []daptinClient.DaptinActionResponse{
		{
			ResponseType: "client.store.set",
			Attributes:   map[string]interface{}{"key": "token", "value": "jwt-abc"},
		},
	}

	effects := ProcessResponses(responses)

	if len(effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effects))
	}
	if effects[0].Type != "token" {
		t.Errorf("expected token effect, got %s", effects[0].Type)
	}
	if effects[0].Token != "jwt-abc" {
		t.Errorf("expected jwt-abc, got %s", effects[0].Token)
	}
}

func TestProcessResponses_Notify(t *testing.T) {
	responses := []daptinClient.DaptinActionResponse{
		{
			ResponseType: "client.notify",
			Attributes:   map[string]interface{}{"message": "Account created"},
		},
	}

	effects := ProcessResponses(responses)

	if len(effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effects))
	}
	if effects[0].Type != "notify" {
		t.Errorf("expected notify, got %s", effects[0].Type)
	}
	if effects[0].Message != "Account created" {
		t.Errorf("expected Account created, got %s", effects[0].Message)
	}
}

func TestProcessResponses_Redirect(t *testing.T) {
	responses := []daptinClient.DaptinActionResponse{
		{
			ResponseType: "client.redirect",
			Attributes:   map[string]interface{}{"location": "/auth/signin"},
		},
	}

	effects := ProcessResponses(responses)

	if len(effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effects))
	}
	if effects[0].Type != "redirect" {
		t.Errorf("expected redirect, got %s", effects[0].Type)
	}
	if effects[0].Message != "/auth/signin" {
		t.Errorf("expected /auth/signin, got %s", effects[0].Message)
	}
}

func TestProcessResponses_FileDownload(t *testing.T) {
	responses := []daptinClient.DaptinActionResponse{
		{
			ResponseType: "client.file.download",
			Attributes:   map[string]interface{}{"filename": "schema.json", "content": "{}"},
		},
	}

	effects := ProcessResponses(responses)

	if len(effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effects))
	}
	if effects[0].Type != "file_download" {
		t.Errorf("expected file_download, got %s", effects[0].Type)
	}
	if effects[0].Data["filename"] != "schema.json" {
		t.Errorf("expected schema.json, got %v", effects[0].Data["filename"])
	}
}

func TestProcessResponses_CookieIgnored(t *testing.T) {
	responses := []daptinClient.DaptinActionResponse{
		{
			ResponseType: "client.cookie.set",
			Attributes:   map[string]interface{}{"key": "session", "value": "abc"},
		},
	}

	effects := ProcessResponses(responses)

	if len(effects) != 0 {
		t.Errorf("expected 0 effects for cookie, got %d", len(effects))
	}
}

func TestProcessResponses_UnknownRendered(t *testing.T) {
	responses := []daptinClient.DaptinActionResponse{
		{
			ResponseType: "some.new.type",
			Attributes:   map[string]interface{}{"data": "something"},
		},
	}

	effects := ProcessResponses(responses)

	if len(effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effects))
	}
	if effects[0].Type != "render_object" {
		t.Errorf("expected render_object, got %s", effects[0].Type)
	}
}

func TestProcessResponses_EmptyAttributesSkipped(t *testing.T) {
	responses := []daptinClient.DaptinActionResponse{
		{
			ResponseType: "unknown",
			Attributes:   map[string]interface{}{},
		},
	}

	effects := ProcessResponses(responses)

	if len(effects) != 0 {
		t.Errorf("expected 0 effects for empty attributes, got %d", len(effects))
	}
}

func TestProcessResponses_MultipleResponses(t *testing.T) {
	// Simulates a signup response: notify + redirect
	responses := []daptinClient.DaptinActionResponse{
		{ResponseType: "client.notify", Attributes: map[string]interface{}{"message": "Success"}},
		{ResponseType: "client.redirect", Attributes: map[string]interface{}{"location": "/signin"}},
	}

	effects := ProcessResponses(responses)

	if len(effects) != 2 {
		t.Fatalf("expected 2 effects, got %d", len(effects))
	}
	if effects[0].Type != "notify" {
		t.Errorf("expected notify, got %s", effects[0].Type)
	}
	if effects[1].Type != "redirect" {
		t.Errorf("expected redirect, got %s", effects[1].Type)
	}
}

func TestProcessResponses_TokenWithEmptyValue(t *testing.T) {
	responses := []daptinClient.DaptinActionResponse{
		{
			ResponseType: "client.store.set",
			Attributes:   map[string]interface{}{"key": "token", "value": ""},
		},
	}

	effects := ProcessResponses(responses)

	if len(effects) != 0 {
		t.Errorf("expected 0 effects for empty token, got %d", len(effects))
	}
}

func TestProcessResponses_StoreSetNonToken(t *testing.T) {
	responses := []daptinClient.DaptinActionResponse{
		{
			ResponseType: "client.store.set",
			Attributes:   map[string]interface{}{"key": "theme", "value": "dark"},
		},
	}

	effects := ProcessResponses(responses)

	if len(effects) != 0 {
		t.Errorf("expected 0 effects for non-token store.set, got %d", len(effects))
	}
}

// --- MissingFields tests ---

func TestMissingFields_AllProvided(t *testing.T) {
	inFields := []map[string]interface{}{
		{"ColumnName": "email", "ColumnType": "email", "Name": "Email"},
		{"ColumnName": "password", "ColumnType": "password", "Name": "Password"},
	}
	existing := map[string]interface{}{"email": "a@b.com", "password": "secret"}

	prompts := MissingFields(inFields, existing)

	if len(prompts) != 0 {
		t.Errorf("expected 0 prompts, got %d", len(prompts))
	}
}

func TestMissingFields_SomeMissing(t *testing.T) {
	inFields := []map[string]interface{}{
		{"ColumnName": "email", "ColumnType": "email", "Name": "Email"},
		{"ColumnName": "password", "ColumnType": "password", "Name": "Password"},
		{"ColumnName": "otp", "ColumnType": "label", "Name": "OTP Code", "IsNullable": true},
	}
	existing := map[string]interface{}{"email": "a@b.com"}

	prompts := MissingFields(inFields, existing)

	if len(prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(prompts))
	}
	if prompts[0].ColumnName != "password" {
		t.Errorf("expected password, got %s", prompts[0].ColumnName)
	}
	if prompts[0].ColumnType != "password" {
		t.Errorf("expected password type, got %s", prompts[0].ColumnType)
	}
	if prompts[1].ColumnName != "otp" {
		t.Errorf("expected otp, got %s", prompts[1].ColumnName)
	}
	if !prompts[1].IsNullable {
		t.Error("expected otp to be nullable")
	}
	if prompts[1].Label != "OTP Code" {
		t.Errorf("expected OTP Code label, got %s", prompts[1].Label)
	}
}

func TestMissingFields_EmptyColumnNameSkipped(t *testing.T) {
	inFields := []map[string]interface{}{
		{"ColumnName": "", "ColumnType": "label"},
		{"ColumnName": "email", "ColumnType": "email"},
	}

	prompts := MissingFields(inFields, map[string]interface{}{})

	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if prompts[0].ColumnName != "email" {
		t.Errorf("expected email, got %s", prompts[0].ColumnName)
	}
}

func TestMissingFields_FallbackLabel(t *testing.T) {
	inFields := []map[string]interface{}{
		{"ColumnName": "count", "ColumnType": "measurement"},
	}

	prompts := MissingFields(inFields, map[string]interface{}{})

	if prompts[0].Label != "count" {
		t.Errorf("expected count as fallback label, got %s", prompts[0].Label)
	}
}

// --- ParseActionSchema tests ---

func TestParseActionSchema_Valid(t *testing.T) {
	schema := `{"InFields":[{"ColumnName":"email","ColumnType":"email"},{"ColumnName":"password","ColumnType":"password"}],"OutFields":[{"Type":"jwt.token","Method":"EXECUTE"}]}`

	fields, err := ParseActionSchema(schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fields))
	}
	if fields[0]["ColumnName"] != "email" {
		t.Errorf("expected email, got %v", fields[0]["ColumnName"])
	}
}

func TestParseActionSchema_NoInFields(t *testing.T) {
	schema := `{"OutFields":[{"Type":"jwt.token"}]}`

	fields, err := ParseActionSchema(schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fields != nil {
		t.Errorf("expected nil for missing InFields, got %v", fields)
	}
}

func TestParseActionSchema_InvalidJSON(t *testing.T) {
	_, err := ParseActionSchema("not json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- FindWorldRefId tests ---

func TestFindWorldRefId_Found(t *testing.T) {
	worlds := []map[string]interface{}{
		{"table_name": "user_account", "reference_id": "uuid-user"},
		{"table_name": "world", "reference_id": "uuid-world"},
	}

	refId := FindWorldRefId(worlds, "world")
	if refId != "uuid-world" {
		t.Errorf("expected uuid-world, got %s", refId)
	}
}

func TestFindWorldRefId_NotFound(t *testing.T) {
	worlds := []map[string]interface{}{
		{"table_name": "user_account", "reference_id": "uuid-user"},
	}

	refId := FindWorldRefId(worlds, "missing")
	if refId != "" {
		t.Errorf("expected empty string, got %s", refId)
	}
}

// --- FindActionSchema tests ---

func TestFindActionRefId_Found(t *testing.T) {
	actions := []map[string]interface{}{
		{"action_name": "signin", "world_id": "uuid-user", "reference_id": "ref-signin"},
		{"action_name": "signup", "world_id": "uuid-user", "reference_id": "ref-signup"},
	}

	refId := FindActionRefId(actions, "uuid-user", "signin")
	if refId != "ref-signin" {
		t.Errorf("expected ref-signin, got %s", refId)
	}
}

func TestFindActionRefId_NotFound(t *testing.T) {
	actions := []map[string]interface{}{
		{"action_name": "signin", "world_id": "uuid-user", "reference_id": "ref-signin"},
	}

	refId := FindActionRefId(actions, "uuid-user", "nonexistent")
	if refId != "" {
		t.Errorf("expected empty, got %s", refId)
	}
}

func TestFindActionRefId_WrongWorld(t *testing.T) {
	actions := []map[string]interface{}{
		{"action_name": "signin", "world_id": "uuid-other", "reference_id": "ref-signin"},
	}

	refId := FindActionRefId(actions, "uuid-user", "signin")
	if refId != "" {
		t.Errorf("expected empty for wrong world, got %s", refId)
	}
}

func TestDecodeActionSchemaResponse_Valid(t *testing.T) {
	// Base64 of: {"InFields":[{"ColumnName":"email","ColumnType":"email"}]}
	schema := `{"InFields":[{"ColumnName":"email","ColumnType":"email"}]}`
	encoded := base64.StdEncoding.EncodeToString([]byte(schema))

	responses := []daptinClient.DaptinActionResponse{
		{
			ResponseType: "client.file.download",
			Attributes:   map[string]interface{}{"content": encoded},
		},
	}

	fields, err := DecodeActionSchemaResponse(responses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if fields[0]["ColumnName"] != "email" {
		t.Errorf("expected email, got %v", fields[0]["ColumnName"])
	}
}

func TestDecodeActionSchemaResponse_NoDownload(t *testing.T) {
	responses := []daptinClient.DaptinActionResponse{
		{ResponseType: "client.notify", Attributes: map[string]interface{}{"message": "ok"}},
	}

	_, err := DecodeActionSchemaResponse(responses)
	if err == nil {
		t.Fatal("expected error for missing download response")
	}
}
