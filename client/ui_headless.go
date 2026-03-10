// Package client provides user interface functionality for VLESS client

// +build headless

package client

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"gopkg.in/ini.v1"
)

// StateManager interface defines UI state operations
type StateManager interface {
	LoadProgramState() (*ProgramState, error)
	SaveProgramState(state *ProgramState) error
	UpdateStatus(status string)
	UpdateProxyStatus(http, socks, system bool)
	ShowConfigWindow()
	HideConfigWindow()
}

// ProgramState represents the last state of the program
type ProgramState struct {
	ConfigFile  string `ini:"ConfigFile"`
	SystemProxy bool   `ini:"SystemProxy"`
	ConfigPath  string `ini:"ConfigPath"`
}

// UIState manages the application UI state
type UIState struct {
	mu          sync.RWMutex
	status      string
	httpProxy   bool
	socksProxy  bool
	systemProxy bool
}

// NewUIState creates a new UIState
func NewUIState() *UIState {
	return &UIState{}
}

// LoadProgramState loads the last program state from system.ini
func (uis *UIState) LoadProgramState() (*ProgramState, error) {
	var err error

	// Get the correct path for system.ini based on platform
	configPath := getConfigFilesPath()
	systemIniPath := filepath.Join(configPath, "system.ini")

	// First, try to load from the platform-specific directory
	cfg, err := ini.Load(systemIniPath)
	if err != nil {
		// If system.ini not found, return default values
		return &ProgramState{
			ConfigFile:  "config.json (TCP)",
			SystemProxy: false,
			ConfigPath:  filepath.Join(configPath, "config.vless"), // Default config path
		}, nil // Return default values if file not found or error
	}

	state := &ProgramState{}

	// Explicitly read from Settings section
	section, err := cfg.GetSection("Settings")
	if err != nil {
		return &ProgramState{
			ConfigFile:  "config.json (TCP)",
			SystemProxy: false,
			ConfigPath:  filepath.Join(configPath, "config.vless"),
		}, nil
	}

	state.ConfigFile = section.Key("ConfigFile").String()
	state.SystemProxy, _ = section.Key("SystemProxy").Bool()
	state.ConfigPath = section.Key("ConfigPath").String()

	// If ConfigPath is not set, use the default platform-specific path
	if state.ConfigPath == "" {
		state.ConfigPath = filepath.Join(configPath, "config.vless")
	}

	// Strip file:// prefix if present (for Android URI)
	if len(state.ConfigPath) > 7 && state.ConfigPath[:7] == "file://" {
		state.ConfigPath = state.ConfigPath[7:]
		// On Windows, file:///D:/... becomes /D:/..., so we need to trim leading slash
		if len(state.ConfigPath) > 0 && state.ConfigPath[0] == '/' && len(state.ConfigPath) > 2 && state.ConfigPath[2] == ':' {
			state.ConfigPath = state.ConfigPath[1:]
		}
	}

	// Validate config file value
	if state.ConfigFile != "config.json (TCP)" && state.ConfigFile != "config_xhttp.json (XHTTP)" {
		state.ConfigFile = "config.json (TCP)"
	}

	// Ensure ConfigPath is always set to the platform-specific location
	if state.ConfigPath == "" || !filepath.IsAbs(state.ConfigPath) {
		state.ConfigPath = filepath.Join(configPath, "config.vless")
	}

	return state, nil
}

// SaveProgramState saves the program state to system.ini
func (uis *UIState) SaveProgramState(state *ProgramState) error {
	var cfg *ini.File
	var err error

	// Get the correct path for system.ini based on platform
	configPath := getConfigFilesPath()
	systemIniPath := filepath.Join(configPath, "system.ini")

	// If ConfigPath is not set, use the default platform-specific path
	if state.ConfigPath == "" || !filepath.IsAbs(state.ConfigPath) {
		state.ConfigPath = filepath.Join(configPath, "config.vless")
	}

	// Load or create system.ini
	cfg, err = ini.Load(systemIniPath)
	if err != nil {
		cfg = ini.Empty()
	}

	// Create or update Settings section
	section, err := cfg.GetSection("Settings")
	if err != nil {
		section, err = cfg.NewSection("Settings")
		if err != nil {
			return err
		}
	}

	section.Key("ConfigFile").SetValue(state.ConfigFile)
	section.Key("SystemProxy").SetValue(fmt.Sprintf("%v", state.SystemProxy))
	section.Key("ConfigPath").SetValue(state.ConfigPath)

	return cfg.SaveTo(systemIniPath)
}

// UpdateStatus updates the application status
func (uis *UIState) UpdateStatus(status string) {
	uis.mu.Lock()
	defer uis.mu.Unlock()
	uis.status = status
}

// UpdateProxyStatus updates proxy status indicators
func (uis *UIState) UpdateProxyStatus(http, socks, system bool) {
	uis.mu.Lock()
	defer uis.mu.Unlock()
	uis.httpProxy = http
	uis.socksProxy = socks
	uis.systemProxy = system
}

// ShowConfigWindow shows the configuration window
func (uis *UIState) ShowConfigWindow() {
	// No-op for headless mode
}

// HideConfigWindow hides the configuration window
func (uis *UIState) HideConfigWindow() {
	// No-op for headless mode
}

// getConfigFilesPath returns the path where config.vless and system.ini should be stored
func getConfigFilesPath() string {
	// On headless systems, config files are always in the same directory as the executable
	exePath, err := os.Executable()
	if err != nil {
		return "." // Fallback to current directory
	}
	return filepath.Dir(exePath)
}
