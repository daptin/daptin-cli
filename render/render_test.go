package render

import "testing"

func TestIncludeColumns_KeepsOnlyNamed(t *testing.T) {
	row := map[string]interface{}{
		"name": "alice", "email": "a@b.com", "age": 30, "secret": "xyz",
	}

	result := IncludeColumns(row, []string{"name", "email"})

	if len(result) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(result))
	}
	if result["name"] != "alice" {
		t.Errorf("expected alice, got %v", result["name"])
	}
	if result["email"] != "a@b.com" {
		t.Errorf("expected a@b.com, got %v", result["email"])
	}
}

func TestIncludeColumns_DoesNotMutateOriginal(t *testing.T) {
	row := map[string]interface{}{"name": "alice", "email": "a@b.com"}
	_ = IncludeColumns(row, []string{"name"})

	if len(row) != 2 {
		t.Errorf("original was mutated: expected 2 keys, got %d", len(row))
	}
}

func TestIncludeColumns_MissingColumns(t *testing.T) {
	row := map[string]interface{}{"name": "alice"}
	result := IncludeColumns(row, []string{"name", "nonexistent"})

	if len(result) != 1 {
		t.Errorf("expected 1 key, got %d", len(result))
	}
}

func TestExcludeColumns_RemovesNamed(t *testing.T) {
	row := map[string]interface{}{
		"name": "alice", "email": "a@b.com", "secret": "xyz",
	}

	result := ExcludeColumns(row, []string{"secret"})

	if len(result) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(result))
	}
	if _, ok := result["secret"]; ok {
		t.Error("secret should have been excluded")
	}
}

func TestExcludeColumns_DoesNotMutateOriginal(t *testing.T) {
	row := map[string]interface{}{"name": "alice", "secret": "xyz"}
	_ = ExcludeColumns(row, []string{"secret"})

	if len(row) != 2 {
		t.Errorf("original was mutated: expected 2 keys, got %d", len(row))
	}
}

func TestFilterColumns_Array(t *testing.T) {
	data := []map[string]interface{}{
		{"name": "a", "email": "a@b", "extra": "x"},
		{"name": "b", "email": "b@c", "extra": "y"},
	}

	result := FilterColumns(data, []string{"name", "email"})

	if len(result) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result))
	}
	for i, row := range result {
		if len(row) != 2 {
			t.Errorf("row %d: expected 2 keys, got %d", i, len(row))
		}
	}
}

func TestFilterColumns_EmptyColumns(t *testing.T) {
	data := []map[string]interface{}{
		{"name": "a", "email": "a@b"},
	}

	result := FilterColumns(data, []string{})

	if len(result[0]) != 0 {
		t.Errorf("expected 0 keys with empty columns, got %d", len(result[0]))
	}
}
