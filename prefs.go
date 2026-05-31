package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Prefs struct {
	Lang    string `json:"lang"`
	Scraper string `json:"scraper"`
}

func prefsPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "moose", "prefs.json")
}

// LoadPrefs returns saved preferences, or a zero-value Prefs if none exist.
func LoadPrefs() Prefs {
	data, err := os.ReadFile(prefsPath())
	if err != nil {
		return Prefs{}
	}
	var p Prefs
	if err := json.Unmarshal(data, &p); err != nil {
		return Prefs{}
	}
	return p
}

// SavePrefs writes preferences to ~/.config/moose/prefs.json.
func SavePrefs(p Prefs) error {
	path := prefsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
