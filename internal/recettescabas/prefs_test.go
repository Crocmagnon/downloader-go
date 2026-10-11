package recettescabas

import (
	"path/filepath"
	"testing"
)

func TestPrefsDefaultToTitles(t *testing.T) {
	p, err := LoadPrefs(filepath.Join(t.TempDir(), "prefs.json"))
	if err != nil || p.Compact {
		t.Errorf("prefs = %+v, err = %v, want titles by default", p, err)
	}
}

func TestPrefsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	if err := SavePrefs(path, Prefs{Compact: true}); err != nil {
		t.Fatal(err)
	}
	if p, err := LoadPrefs(path); err != nil || !p.Compact {
		t.Errorf("prefs = %+v, err = %v", p, err)
	}
}
