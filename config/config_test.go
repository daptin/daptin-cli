package config

import "testing"

func TestActiveHost_ReturnsMatchingHost(t *testing.T) {
	cfg := Config{
		CurrentContext: "prod",
		Hosts: []HostEndpoint{
			{Name: "dev", Endpoint: "http://localhost:6336"},
			{Name: "prod", Endpoint: "https://api.example.com", Token: "tok123"},
		},
	}

	host, err := cfg.ActiveHost()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host.Name != "prod" {
		t.Errorf("expected prod, got %s", host.Name)
	}
	if host.Token != "tok123" {
		t.Errorf("expected tok123, got %s", host.Token)
	}
}

func TestActiveHost_ErrorWhenEmpty(t *testing.T) {
	cfg := Config{}
	_, err := cfg.ActiveHost()
	if err == nil {
		t.Fatal("expected error for empty context")
	}
}

func TestActiveHost_ErrorWhenNotFound(t *testing.T) {
	cfg := Config{
		CurrentContext: "missing",
		Hosts:          []HostEndpoint{{Name: "dev", Endpoint: "http://localhost"}},
	}
	_, err := cfg.ActiveHost()
	if err == nil {
		t.Fatal("expected error for missing context")
	}
}

func TestSetContext_SetsWhenExists(t *testing.T) {
	cfg := Config{
		Hosts: []HostEndpoint{
			{Name: "dev", Endpoint: "http://localhost"},
			{Name: "prod", Endpoint: "https://api.example.com"},
		},
	}

	err := cfg.SetContext("prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CurrentContext != "prod" {
		t.Errorf("expected prod, got %s", cfg.CurrentContext)
	}
}

func TestSetContext_ErrorWhenNotFound(t *testing.T) {
	cfg := Config{
		Hosts: []HostEndpoint{{Name: "dev", Endpoint: "http://localhost"}},
	}

	err := cfg.SetContext("missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if cfg.CurrentContext != "" {
		t.Errorf("context should not have changed, got %s", cfg.CurrentContext)
	}
}

func TestUpsertHost_AddsNew(t *testing.T) {
	cfg := Config{}
	cfg.UpsertHost(HostEndpoint{Name: "dev", Endpoint: "http://localhost"})

	if len(cfg.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(cfg.Hosts))
	}
	if cfg.Hosts[0].Name != "dev" {
		t.Errorf("expected dev, got %s", cfg.Hosts[0].Name)
	}
}

func TestUpsertHost_UpdatesByName(t *testing.T) {
	cfg := Config{
		Hosts: []HostEndpoint{
			{Name: "dev", Endpoint: "http://old", Token: "old"},
		},
	}
	cfg.UpsertHost(HostEndpoint{Name: "dev", Endpoint: "http://new", Token: "new"})

	if len(cfg.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(cfg.Hosts))
	}
	if cfg.Hosts[0].Endpoint != "http://new" {
		t.Errorf("expected http://new, got %s", cfg.Hosts[0].Endpoint)
	}
	if cfg.Hosts[0].Token != "new" {
		t.Errorf("expected new token, got %s", cfg.Hosts[0].Token)
	}
}

func TestUpsertHost_UpdatesByEndpoint(t *testing.T) {
	cfg := Config{
		Hosts: []HostEndpoint{
			{Name: "dev", Endpoint: "http://localhost", Token: "old"},
		},
	}
	cfg.UpsertHost(HostEndpoint{Name: "renamed", Endpoint: "http://localhost", Token: "new"})

	if len(cfg.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(cfg.Hosts))
	}
	if cfg.Hosts[0].Name != "renamed" {
		t.Errorf("expected renamed, got %s", cfg.Hosts[0].Name)
	}
}

func TestMarshalUnmarshal_Roundtrip(t *testing.T) {
	cfg := Config{
		CurrentContext: "prod",
		Hosts: []HostEndpoint{
			{Name: "prod", Endpoint: "https://api.example.com", Token: "tok"},
		},
	}

	data, err := cfg.Marshal()
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	parsed, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed.CurrentContext != "prod" {
		t.Errorf("expected prod, got %s", parsed.CurrentContext)
	}
	if len(parsed.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(parsed.Hosts))
	}
	if parsed.Hosts[0].Token != "tok" {
		t.Errorf("expected tok, got %s", parsed.Hosts[0].Token)
	}
}

func TestUnmarshal_EmptyBytes(t *testing.T) {
	cfg, err := Unmarshal([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CurrentContext != "" {
		t.Errorf("expected empty context, got %s", cfg.CurrentContext)
	}
}
