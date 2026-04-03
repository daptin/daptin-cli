package cmd

import (
	"testing"
)

func TestDecodePermission(t *testing.T) {
	tests := []struct {
		value    int64
		guest    []string
		owner    []string
		group    []string
	}{
		{
			value: 561441,
			guest: []string{"Peek", "Execute"},
			owner: []string{"Read", "Execute"},
			group: []string{"Read", "Execute"},
		},
		{
			value: 2097151, // all bits set (21 bits)
			guest: []string{"Peek", "Read", "Create", "Update", "Delete", "Execute", "Refer"},
			owner: []string{"Peek", "Read", "Create", "Update", "Delete", "Execute", "Refer"},
			group: []string{"Peek", "Read", "Create", "Update", "Delete", "Execute", "Refer"},
		},
		{
			value: 0,
			guest: nil,
			owner: nil,
			group: nil,
		},
		{
			value: 2, // guest read only
			guest: []string{"Read"},
			owner: nil,
			group: nil,
		},
		{
			value: 16256, // owner full (0x3F80)
			guest: nil,
			owner: []string{"Peek", "Read", "Create", "Update", "Delete", "Execute", "Refer"},
			group: nil,
		},
	}

	for _, tt := range tests {
		p := DecodePermission(tt.value)
		assertOps(t, tt.value, "Guest", p.Guest, tt.guest)
		assertOps(t, tt.value, "Owner", p.Owner, tt.owner)
		assertOps(t, tt.value, "Group", p.Group, tt.group)
	}
}

func assertOps(t *testing.T, value int64, tier string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("value %d %s: expected %v, got %v", value, tier, want, got)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("value %d %s[%d]: expected %s, got %s", value, tier, i, want[i], got[i])
		}
	}
}

func TestEncodePermission(t *testing.T) {
	p := Permission{
		Guest: []string{"Peek", "Execute"},
		Owner: []string{"Read", "Execute"},
		Group: []string{"Read", "Execute"},
	}
	if EncodePermission(p) != 561441 {
		t.Errorf("expected 561441, got %d", EncodePermission(p))
	}
}

func TestEncodePermission_Roundtrip(t *testing.T) {
	values := []int64{0, 2, 561441, 16256, 2097151, 33026}
	for _, v := range values {
		p := DecodePermission(v)
		encoded := EncodePermission(p)
		if encoded != v {
			t.Errorf("roundtrip failed for %d: got %d", v, encoded)
		}
	}
}

func TestFormatPermission(t *testing.T) {
	s := FormatPermission(561441)
	if s == "" {
		t.Error("expected non-empty string")
	}
	// Should contain tier names and ops
	for _, expected := range []string{"Guest", "Owner", "Group", "Peek", "Read", "Execute"} {
		if !containsStr(s, expected) {
			t.Errorf("expected %q in output, got: %s", expected, s)
		}
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestParsePermissionModifier_Add(t *testing.T) {
	result, err := ApplyPermissionModifier(561441, "+GuestRead")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := DecodePermission(result)
	found := false
	for _, op := range p.Guest {
		if op == "Read" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected GuestRead to be set, got %v", p.Guest)
	}
}

func TestParsePermissionModifier_Remove(t *testing.T) {
	result, err := ApplyPermissionModifier(561441, "-GuestPeek")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := DecodePermission(result)
	for _, op := range p.Guest {
		if op == "Peek" {
			t.Error("expected GuestPeek to be removed")
		}
	}
}

func TestParsePermissionModifier_Invalid(t *testing.T) {
	_, err := ApplyPermissionModifier(0, "InvalidMod")
	if err == nil {
		t.Fatal("expected error for invalid modifier")
	}
}
