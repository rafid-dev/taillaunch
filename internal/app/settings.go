package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Settings are the small set of GUI preferences that should survive a
// launcher restart. Tailscale identity and Chromium profile data remain in
// the directories selected by Persist/Portable and are not stored here.
type Settings struct {
	Persist    bool       `json:"persist"`
	Browser    string     `json:"browser"`
	ControlURL string     `json:"control_url"`
	Verbose    bool       `json:"verbose"`
	MemoryMode MemoryMode `json:"memory_mode"`
}

func DefaultSettings() Settings {
	return Settings{MemoryMode: MemoryAuto}
}

func (s Settings) normalized() Settings {
	s.Browser = strings.TrimSpace(s.Browser)
	s.ControlURL = strings.TrimSpace(s.ControlURL)
	if s.MemoryMode != MemoryAuto && s.MemoryMode != MemoryNormal && s.MemoryMode != MemoryLow {
		s.MemoryMode = MemoryAuto
	}
	return s
}

// SettingsPath returns the per-user settings file path.
func SettingsPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "TailLaunch", "settings.json"), nil
}

// LoadSettings returns defaults when the settings file does not exist.
func LoadSettings() (Settings, error) {
	path, err := SettingsPath()
	if err != nil {
		return DefaultSettings(), err
	}
	return LoadSettingsFrom(path)
}

func LoadSettingsFrom(path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultSettings(), nil
	}
	if err != nil {
		return DefaultSettings(), fmt.Errorf("read settings: %w", err)
	}
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return DefaultSettings(), fmt.Errorf("parse settings: %w", err)
	}
	return settings.normalized(), nil
}

func SaveSettings(settings Settings) error {
	path, err := SettingsPath()
	if err != nil {
		return err
	}
	return SaveSettingsTo(path, settings)
}

func SaveSettingsTo(path string, settings Settings) error {
	settings = settings.normalized()
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("prepare settings directory: %w", err)
	}
	_ = os.Chmod(filepath.Dir(path), 0o700)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	return nil
}
