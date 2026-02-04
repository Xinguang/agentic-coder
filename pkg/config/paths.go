// Package config provides configuration management for agentic-coder
package config

import (
	"os"
	"path/filepath"
)

const (
	// AppName is the application name
	AppName = "agentic-coder"

	// AppDirName is the directory name for app data
	AppDirName = ".agentic-coder"
)

// GetAppDir returns the application data directory (~/.agentic-coder)
func GetAppDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, AppDirName), nil
}

// GetSessionsDir returns the sessions directory
func GetSessionsDir() (string, error) {
	appDir, err := GetAppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(appDir, "sessions"), nil
}

// GetProjectSessionsDir returns the project-specific sessions directory
func GetProjectSessionsDir(projectPath string) (string, error) {
	sessionsDir, err := GetSessionsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(sessionsDir, sanitizePath(projectPath)), nil
}

// GetConfigPath returns the global config file path
func GetConfigPath() (string, error) {
	appDir, err := GetAppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(appDir, "config.json"), nil
}

// GetProjectConfigPath returns the project-specific config file path
func GetProjectConfigPath(projectPath string) string {
	return filepath.Join(projectPath, AppDirName, "config.json")
}

// sanitizePath converts a file path to a safe directory name
func sanitizePath(path string) string {
	// Use a simple hash-like approach for cleaner names
	result := ""
	for _, r := range path {
		switch r {
		case '/', '\\':
			result += "_"
		case ':':
			result += ""
		case ' ':
			result += "_"
		default:
			result += string(r)
		}
	}

	// Remove leading underscores
	for len(result) > 0 && result[0] == '_' {
		result = result[1:]
	}

	// Truncate if too long
	if len(result) > 100 {
		result = result[:100]
	}

	if result == "" {
		result = "default"
	}

	return result
}
