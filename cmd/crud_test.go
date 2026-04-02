package cmd

import "testing"

func TestParseAttributes_KeyVal(t *testing.T) {
	args := []string{"name=alice", "email=a@b.com", "age=30"}

	result, err := parseAttributes(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["name"] != "alice" {
		t.Errorf("expected alice, got %v", result["name"])
	}
	if result["email"] != "a@b.com" {
		t.Errorf("expected a@b.com, got %v", result["email"])
	}
	if result["age"] != "30" {
		t.Errorf("expected 30, got %v", result["age"])
	}
}

func TestParseAttributes_JSON(t *testing.T) {
	args := []string{`{"name":"alice","count":5}`}

	result, err := parseAttributes(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["name"] != "alice" {
		t.Errorf("expected alice, got %v", result["name"])
	}
	// JSON numbers are float64
	if result["count"] != float64(5) {
		t.Errorf("expected 5, got %v", result["count"])
	}
}

func TestParseAttributes_Empty(t *testing.T) {
	result, err := parseAttributes([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestParseAttributes_InvalidKeyVal(t *testing.T) {
	_, err := parseAttributes([]string{"no-equals-sign"})
	if err == nil {
		t.Fatal("expected error for missing =")
	}
}

func TestParseAttributes_InvalidJSON(t *testing.T) {
	_, err := parseAttributes([]string{"{bad json"})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseAttributes_ValueWithEquals(t *testing.T) {
	args := []string{"query=name=alice"}

	result, err := parseAttributes(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["query"] != "name=alice" {
		t.Errorf("expected name=alice, got %v", result["query"])
	}
}
