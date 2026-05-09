package cmd

import "testing"

const testOpenAPISpec = `
openapi: 3.0.0
info:
  title: Example
  version: "1.0"
paths:
  /workspaces:
    get:
      operationId: getWorkspaces
      summary: List workspaces
      parameters:
        - name: opt_fields
          in: query
          required: false
          schema:
            type: string
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: object
`

func TestDetectSpecLanguage_OpenAPIV3YAML(t *testing.T) {
	got, err := detectSpecLanguage(testOpenAPISpec, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "openapiv3" {
		t.Fatalf("expected openapiv3, got %q", got)
	}
}

func TestOperationRowsFromDiscovery(t *testing.T) {
	ops := operationRowsFromDiscovery(map[string]interface{}{
		"operations": []interface{}{
			map[string]interface{}{
				"operation_id": "getWorkspaces",
				"method":       "GET",
				"path":         "/workspaces",
			},
		},
	})
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0]["operation_id"] != "getWorkspaces" {
		t.Fatalf("unexpected operation: %#v", ops[0])
	}
}

func TestBuildOperationInputMergesJSONAndKeyValues(t *testing.T) {
	input, err := buildOperationInput(`{"limit":10}`, "", []string{"workspace=abc"})
	if err != nil {
		t.Fatal(err)
	}
	if input["limit"] != float64(10) || input["workspace"] != "abc" {
		t.Fatalf("unexpected input: %#v", input)
	}
}

func TestBuildIntegrationOperationBody(t *testing.T) {
	body := buildIntegrationOperationBody(map[string]interface{}{"limit": "10"}, "tok-ref", "")
	if body["oauth_token_id"] != "tok-ref" {
		t.Fatalf("expected oauth_token_id, got %#v", body)
	}
	input, ok := body["input"].(map[string]interface{})
	if !ok || input["limit"] != "10" {
		t.Fatalf("unexpected input body: %#v", body)
	}
}

func TestNormalizeIntegrationAuthType(t *testing.T) {
	got, err := normalizeIntegrationAuthType("custom")
	if err != nil {
		t.Fatal(err)
	}
	if got != "custom_credentials" {
		t.Fatalf("expected custom_credentials, got %q", got)
	}
}
