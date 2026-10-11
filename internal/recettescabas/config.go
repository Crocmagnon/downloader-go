// Package recettescabas sends a weekly Recettes & Cabas recap to Telegram
// and applies the selection changes requested from the Telegram bot.
package recettescabas

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// Flag marks items whose ingredients contain one of Keywords with Emoji.
type Flag struct {
	Emoji    string   `json:"emoji"`
	Keywords []string `json:"keywords"`
}

type Config struct {
	// ExcludeOptions hides options whose name or label contains one of these keywords.
	ExcludeOptions []string `json:"excludeOptions"`
	Flags          []Flag   `json:"flags"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}

	return cfg, nil
}
