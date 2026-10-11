package recettescabas

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	cfg, err := LoadConfig("testdata/config.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ExcludeOptions) == 0 || len(cfg.Flags) != 2 || cfg.Flags[0].Emoji != "🍄" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"exclude_options": ["vin"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Error("expected an error for a misspelled field")
	}
}
