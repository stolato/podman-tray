package tray

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// settings are the user's choices from the menu, kept across restarts.
type settings struct {
	// QuietSuccess skips notifications for actions that worked; errors and alerts
	// (a container crashed or became unhealthy) are always shown.
	QuietSuccess bool `json:"quietSuccess"`
}

func settingsPath() string {
	dir, err := os.UserConfigDir() // ~/Library/Application Support
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "com.podman-tray", "settings.json")
}

// loadSettings returns the saved settings, or the defaults if there are none.
func loadSettings(path string) settings {
	var s settings
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

func saveSettings(path string, s settings) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
