package demo

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	DemoConfigDir  = ".kubeasy"
	DemoConfigFile = "demo-token"
)

type DemoConfig struct {
	Token string `json:"token"`
}

// GetConfigPath returns ~/.kubeasy/demo-token
func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DemoConfigDir, DemoConfigFile), nil
}

// SaveToken stores the demo token locally
func SaveToken(token string) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	// Create directory if not exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	config := DemoConfig{Token: token}
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}

// LoadToken retrieves the stored demo token
func LoadToken() (string, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	var config DemoConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return "", err
	}

	return config.Token, nil
}

// DeleteToken removes the stored demo token
func DeleteToken() error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}
	return os.Remove(configPath)
}

// HasToken checks if a demo token is stored
func HasToken() bool {
	token, err := LoadToken()
	return err == nil && token != ""
}
