// Linux-specific implementation for VLESS Client (both GUI and headless versions)
// $env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "amd64"; go build -v -tags "headless" -o vless_client

// +build linux

package main

import (
	"flag"
	"fmt"
	"v2/client"
	_ "github.com/xtls/xray-core/main/distro/all" // Import all config formats (JSON, YAML, TOML)
	"gopkg.in/ini.v1"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

// ProgramState represents the last state of the program
type ProgramState struct {
	ConfigFile  string `ini:"ConfigFile"`
	SystemProxy bool   `ini:"SystemProxy"`
	ConfigPath  string `ini:"ConfigPath"`
}

// LoadProgramState loads the last program state from system.ini
func LoadProgramState() (*ProgramState, error) {
	var err error

	// Get the correct path for system.ini based on platform
	configPath := client.GetConfigFilesPath()
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

	infoLog("LoadProgramState: ConfigFile=%s, SystemProxy=%v, ConfigPath=%s", state.ConfigFile, state.SystemProxy, state.ConfigPath)
	return state, nil
}

// SaveProgramState saves the program state to system.ini
func SaveProgramState(state *ProgramState) error {
	var cfg *ini.File
	var err error

	// Get the correct path for system.ini based on platform
	configPath := client.GetConfigFilesPath()
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

// CheckSingleInstance is a dummy implementation for Linux
func CheckSingleInstance() bool {
	// No single instance check on Linux
	return true
}

// createHiddenWindow is a dummy implementation for Linux
func createHiddenWindow() error {
	// No hidden window needed on Linux
	return nil
}

// destroyHiddenWindow is a dummy implementation for Linux
func destroyHiddenWindow() {
	// No hidden window to destroy on Linux
}

// setupSystemTray is a dummy implementation for Linux
func setupSystemTray(w interface{}, a interface{}) {
	// System tray might not be supported on all Linux desktop environments
}

// enableWindowsProxy is a dummy implementation for Linux
func enableWindowsProxy(socksPort, httpPort int) error {
	return fmt.Errorf("Windows proxy is not supported on this platform")
}

// disableWindowsProxy is a dummy implementation for Linux
func disableWindowsProxy() error {
	return fmt.Errorf("Windows proxy is not supported on this platform")
}

// addLoopbackRoute is a dummy implementation for Linux
func addLoopbackRoute() error {
	// On Linux, loopback routes are usually already configured
	return nil
}

// deleteLoopbackRoute is a dummy implementation for Linux
func deleteLoopbackRoute() error {
	// On Linux, loopback routes are usually already configured
	return nil
}

// enableSystemProxy sets proxy environment variables for Linux
func enableSystemProxy(socksPort, httpPort int) {
	if socksPort > 0 {
		socksProxy := "socks5://127.0.0.1:" + fmt.Sprintf("%d", socksPort)
		os.Setenv("ALL_PROXY", socksProxy)
		os.Setenv("all_proxy", socksProxy)
		infoLog("Set ALL_PROXY: %s", socksProxy)
	}
	if httpPort > 0 {
		httpProxy := "http://127.0.0.1:" + fmt.Sprintf("%d", httpPort)
		os.Setenv("HTTP_PROXY", httpProxy)
		os.Setenv("HTTPS_PROXY", httpProxy)
		os.Setenv("http_proxy", httpProxy)
		os.Setenv("https_proxy", httpProxy)
		infoLog("Set HTTP_PROXY: %s", httpProxy)
	}

	// On Linux, we don't need to add loopback routes as they're already configured
}

// disableSystemProxy removes proxy environment variables for Linux
func disableSystemProxy() {
	os.Unsetenv("ALL_PROXY")
	os.Unsetenv("HTTP_PROXY")
	os.Unsetenv("HTTPS_PROXY")
	os.Unsetenv("http_proxy")
	os.Unsetenv("https_proxy")
	infoLog("Proxy environment variables cleared")

	// On Linux, we don't need to disable system proxy via registry
}

// executeCommand runs a Linux command and returns output and error
func executeCommand(cmd string) (string, error) {
	args := strings.Split(cmd, " ")
	if len(args) == 0 {
		return "", fmt.Errorf("empty command")
	}

	exe := args[0]
	var cmdArgs []string
	if len(args) > 1 {
		cmdArgs = args[1:]
	}

	out, err := exec.Command(exe, cmdArgs...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("command failed: %s", out)
	}

	return string(out), nil
}

// executeCmd executes a Linux command and logs the result
func executeCmd(cmd string) error {
	infoLog("Executing command: %s", cmd)
	out, err := executeCommand(cmd)
	if err != nil {
		return err
	}
	if out != "" {
		infoLog("Command output: %s", out)
	}
	return nil
}

// Headless-specific code
// +build linux,headless

// Global flags
var debugMode bool
var logMode bool

// Log writers
var debugFile *os.File
var logFile *os.File

func debugLog(format string, args ...any) {
	if debugMode {
		msg := fmt.Sprintf("[DEBUG] "+format+"\n", args...)
		if debugFile != nil {
			debugFile.WriteString(msg)
			debugFile.Sync()
		}
	}
}

func infoLog(format string, args ...any) {
	msg := fmt.Sprintf("[INFO] "+format+"\n", args...)
	if logMode {
		if logFile != nil {
			logFile.WriteString(msg)
			logFile.Sync()
		}
	}
}

func errorLog(format string, args ...any) {
	msg := fmt.Sprintf("[ERROR] "+format+"\n", args...)
	if logMode {
		if logFile != nil {
			logFile.WriteString(msg)
			logFile.Sync()
		}
	}
}

// openLogFiles opens log files if needed
func openLogFiles() error {
	if debugMode {
		var err error
		debugFile, err = os.OpenFile("debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		debugLog("Debug log file opened")
	}

	if logMode {
		var err error
		logFile, err = os.OpenFile("vless_client.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		infoLog("Main log file opened")
	}

	return nil
}

// closeLogFiles closes log files
func closeLogFiles() {
	if debugFile != nil {
		debugFile.Close()
		debugFile = nil
	}
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

func main() {
	// Check for single instance (platform-specific)
	CheckSingleInstance()

	// Parse command line arguments
	flag.BoolVar(&debugMode, "debug", false, "Enable debug mode")
	flag.BoolVar(&logMode, "log", false, "Enable detailed logging to file")
	flag.Parse()

	// Open log files if needed
	if err := openLogFiles(); err != nil {
		fmt.Printf("Error opening log files: %v\n", err)
		os.Exit(1)
	}
	defer closeLogFiles()

	if debugMode {
		infoLog("Debug mode enabled")
	}

	// Load program state from system.ini
	state, err := LoadProgramState()
	if err != nil {
		infoLog("Warning: %v", err)
		state = &ProgramState{
			ConfigFile:  "config.json (TCP)",
			SystemProxy: false,
		}
	}
	infoLog("Loaded program state: ConfigFile=%s, SystemProxy=%v", state.ConfigFile, state.SystemProxy)

	// Load encrypted config
	configStorage, err := client.LoadConfigFromVLESS(state.ConfigPath)
	if err != nil {
		infoLog("Warning: %v", err)
		infoLog("Using default settings")
		configStorage = &client.ConfigStorage{
			TCP:   client.GetDefaultTCPConfig(),
			XHTTP: client.GetDefaultXHTTPConfig(),
		}
		if err := client.SaveConfigToVLESS(configStorage, state.ConfigPath); err != nil {
			errorLog("Failed to save default config: %v", err)
		}
	}

	// Get initial config based on saved state
	var cfg *client.Config
	if state.ConfigFile == "config.json (TCP)" {
		cfg = configStorage.TCP
	} else {
		cfg = configStorage.XHTTP
	}

	// Create client
	clientInstance, err := client.NewClient(cfg)
	if err != nil {
		errorLog("Error creating client: %v", err)
		os.Exit(1)
	}

	fmt.Println("VLESS Client (Linux Headless)")
	fmt.Printf("Config: %s\n", state.ConfigFile)
	fmt.Printf("Server: %s:%d\n", cfg.VLESS.Address, cfg.VLESS.Port)
	fmt.Printf("SOCKS Port: %d\n", cfg.SOCKS.Port)
	fmt.Printf("HTTP Port: %d\n", cfg.HTTP.Port)
	fmt.Println("---------------------------------")

	// Start the client
	if err := clientInstance.Start(); err != nil {
		errorLog("Error starting client: %v", err)
		os.Exit(1)
	}
	fmt.Println("Client started successfully!")

	// Handle signals for clean shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("\nPress Ctrl+C to stop the client...")
	<-sigCh

	clientInstance.Stop()
	fmt.Println("\nClient stopped")
}