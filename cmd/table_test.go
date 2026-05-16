package cmd

import "testing"

func TestTableDefaultsSetPermission(t *testing.T) {
	defaults := &tableDefaults{Schema: map[string]interface{}{"DefaultPermission": float64(1)}}
	if !defaults.setPermission(1618275) {
		t.Fatal("expected permission change")
	}
	got, ok := defaults.defaultPermission()
	if !ok || got != 1618275 {
		t.Fatalf("unexpected permission: %d %v", got, ok)
	}
	if defaults.setPermission(1618275) {
		t.Fatal("expected idempotent permission set")
	}
}

func TestTableDefaultsEnsureGroupAddsObjectBinding(t *testing.T) {
	defaults := &tableDefaults{Schema: map[string]interface{}{}}
	permission := int64(524288)
	if !defaults.ensureGroup("users", &permission) {
		t.Fatal("expected group change")
	}
	groups := defaults.defaultGroups()
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %#v", groups)
	}
	if groups[0]["Name"] != "users" || groups[0]["Permission"] != permission {
		t.Fatalf("unexpected group: %#v", groups[0])
	}
	if defaults.ensureGroup("users", &permission) {
		t.Fatal("expected idempotent group ensure")
	}
}

func TestTableDefaultsEnsureGroupUpgradesStringBinding(t *testing.T) {
	defaults := &tableDefaults{Schema: map[string]interface{}{"DefaultGroups": []interface{}{"users"}}}
	permission := int64(524288)
	if !defaults.ensureGroup("users", &permission) {
		t.Fatal("expected group permission change")
	}
	groups := defaults.defaultGroups()
	if groups[0]["Name"] != "users" || groups[0]["Permission"] != permission {
		t.Fatalf("unexpected group: %#v", groups[0])
	}
}

func TestParseDefaultGroupArg(t *testing.T) {
	name, permission, err := parseDefaultGroupArg("users:1618275")
	if err != nil {
		t.Fatal(err)
	}
	if name != "users" || permission == nil || *permission != 1618275 {
		t.Fatalf("unexpected group arg: %q %#v", name, permission)
	}
}
