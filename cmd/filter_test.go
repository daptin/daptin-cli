package cmd

import (
	"encoding/json"
	"testing"
)

func TestParseFilter_SimpleContains(t *testing.T) {
	result, err := ParseFilter("name contains foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(result))
	}
	if result[0].Column != "name" {
		t.Errorf("expected name, got %s", result[0].Column)
	}
	if result[0].Operator != "contains" {
		t.Errorf("expected contains, got %s", result[0].Operator)
	}
	if result[0].Value != "foo" {
		t.Errorf("expected foo, got %s", result[0].Value)
	}
}

func TestParseFilter_Equals(t *testing.T) {
	result, err := ParseFilter("status is active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0].Operator != "is" {
		t.Errorf("expected is, got %s", result[0].Operator)
	}
}

func TestParseFilter_TwoWordOperator(t *testing.T) {
	tests := []struct {
		input    string
		operator string
		value    string
	}{
		{"name begins with foo", "begins with", "foo"},
		{"name ends with bar", "ends with", "bar"},
		{"count more than 5", "more than", "5"},
		{"count less than 10", "less than", "10"},
		{"active is true", "is true", ""},
		{"deleted is false", "is false", ""},
		{"notes is empty", "is empty", ""},
		{"status is not pending", "is not", "pending"},
	}

	for _, tt := range tests {
		result, err := ParseFilter(tt.input)
		if err != nil {
			t.Fatalf("input %q: unexpected error: %v", tt.input, err)
		}
		if len(result) != 1 {
			t.Fatalf("input %q: expected 1 filter, got %d", tt.input, len(result))
		}
		if result[0].Operator != tt.operator {
			t.Errorf("input %q: expected operator %q, got %q", tt.input, tt.operator, result[0].Operator)
		}
		if result[0].Value != tt.value {
			t.Errorf("input %q: expected value %q, got %q", tt.input, tt.value, result[0].Value)
		}
	}
}

func TestParseFilter_MultipleFilters(t *testing.T) {
	result, err := ParseFilter("name contains foo;status is active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 filters, got %d", len(result))
	}
	if result[0].Column != "name" {
		t.Errorf("expected name, got %s", result[0].Column)
	}
	if result[1].Column != "status" {
		t.Errorf("expected status, got %s", result[1].Column)
	}
}

func TestParseFilter_PassthroughJSON(t *testing.T) {
	input := `[{"column":"name","operator":"contains","value":"foo"}]`
	result, err := ParseFilter(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(result))
	}
	if result[0].Column != "name" {
		t.Errorf("expected name, got %s", result[0].Column)
	}
}

func TestParseFilter_Empty(t *testing.T) {
	result, err := ParseFilter("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 filters, got %d", len(result))
	}
}

func TestParseFilter_InvalidExpression(t *testing.T) {
	_, err := ParseFilter("just-one-word")
	if err == nil {
		t.Fatal("expected error for invalid expression")
	}
}

func TestFilterToJSON(t *testing.T) {
	filters := []FilterClause{
		{Column: "name", Operator: "contains", Value: "foo"},
		{Column: "active", Operator: "is true", Value: ""},
	}

	jsonStr := FilterToJSON(filters)

	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(parsed))
	}
	if parsed[0]["column"] != "name" {
		t.Errorf("expected name, got %v", parsed[0]["column"])
	}
	if parsed[1]["operator"] != "is true" {
		t.Errorf("expected is true, got %v", parsed[1]["operator"])
	}
}
