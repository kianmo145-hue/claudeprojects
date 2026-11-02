package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings are the tray's own preferences (the adapters' speed caps live in limits.json).
type Settings struct {
	Lang     string `json:"lang,omitempty"`     // "fa", "en" or "" for the system language
	Welcomed bool   `json:"welcomed,omitempty"` // the first-start message was shown
}

func settingsPath(dir string) string { return filepath.Join(dir, "settings.json") }

// LoadSettings returns the saved settings, or the zero value if there are none.
func LoadSettings(dir string) Settings {
	var s Settings
	if b, err := os.ReadFile(settingsPath(dir)); err == nil {
		json.Unmarshal(b, &s)
	}
	return s
}

// SaveSettings writes the settings.
func SaveSettings(dir string, s Settings) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath(dir), b, 0o644)
}
