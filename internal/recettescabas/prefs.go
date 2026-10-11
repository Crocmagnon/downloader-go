package recettescabas

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// Prefs are display settings changed from the bot (/affichage).
type Prefs struct {
	// Compact shows numbers instead of titles in the buttons.
	Compact bool `json:"compact"`
}

// LoadPrefs reads path; a missing file gives the defaults.
func LoadPrefs(path string) (Prefs, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Prefs{}, nil
	}
	if err != nil {
		return Prefs{}, fmt.Errorf("reading prefs: %w", err)
	}

	var p Prefs
	if err := json.Unmarshal(data, &p); err != nil {
		return Prefs{}, fmt.Errorf("parsing prefs %s: %w", path, err)
	}

	return p, nil
}

func SavePrefs(path string, p Prefs) error {
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshaling prefs: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing prefs: %w", err)
	}

	return nil
}
