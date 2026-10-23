// Package recettescabas bridges Recettes & Cabas with Home Assistant: it pushes each read of
// the site to a Home Assistant webhook and applies the selection Home Assistant asks for.
package recettescabas

import (
	"encoding/json/v2"
	"fmt"
	"os"
)

// Flag marks items whose ingredients contain one of Keywords with Emoji.
type Flag struct {
	Emoji    string   `json:"emoji"`
	Keywords []string `json:"keywords"`
}

type Config struct {
	// ExcludeOptions hides options whose name, label or ingredients contain one of these keywords:
	// some wines are only called a wine there ("Vin agréé par la marque…").
	ExcludeOptions []string `json:"excludeOptions"`
	Flags          []Flag   `json:"flags"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg, json.RejectUnknownMembers(true)); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}

	return cfg, nil
}
