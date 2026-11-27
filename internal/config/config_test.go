package config

import "testing"

func TestConfiguration(t *testing.T) {
	for _, key := range []string{"PULSE_ADDR", "POI_MODE", "DATABASE_URL", "OPENAI_API_KEY", "OPENAI_MODEL", "OPENAI_BASE_URL"} {
		t.Setenv(key, "")
	}
	if _, err := Load(); err == nil {
		t.Fatal("missing database accepted")
	}
	t.Setenv("POI_MODE", "demo")
	c, err := Load()
	if err != nil || c.Address != "127.0.0.1:8080" || c.AgentEnabled() {
		t.Fatalf("got %+v %v", c, err)
	}
	t.Setenv("OPENAI_API_KEY", "test-secret")
	if _, err := Load(); err == nil {
		t.Fatal("missing model accepted")
	}
	t.Setenv("OPENAI_MODEL", "test-model")
	t.Setenv("OPENAI_BASE_URL", "file:///tmp/model")
	if _, err := Load(); err == nil {
		t.Fatal("invalid endpoint accepted")
	}
	t.Setenv("OPENAI_BASE_URL", "https://example.com/v1")
	c, err = Load()
	if err != nil || !c.AgentEnabled() {
		t.Fatal(err)
	}
	t.Setenv("POI_MODE", "unknown")
	if _, err := Load(); err == nil {
		t.Fatal("unknown mode accepted")
	}
}
