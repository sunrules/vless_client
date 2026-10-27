// Linux-specific implementation for VLESS Client (both GUI and headless versions)
// $env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "amd64"; go build -v -tags "headless" -o vless_client

//go:build linux && linux && headless
// +build linux,linux,headless

package main

import (
	"flag"
	"fmt"
	_ "github.com/xtls/xray-core/main/distro/all" // Import all config formats (JSON, YAML, TOML)
	"gopkg.in/ini.v1"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"vless_client/client"
)

// ProgramState represents the last state of the program
type ProgramState struct {
	ConfigFile        string `ini:"ConfigFile"`
	SystemProxy       bool   `ini:"SystemProxy"`
	ConfigPath        string `ini:"ConfigPath"`
	HttpProxy         string `ini:"http_proxy"`
	SocksProxy        string `ini:"socks_proxy"`
	SeparateTunneling string `ini:"separate_tunneling"`
	LogLevel          string `ini:"log_level"`
	LogFile           string `ini:"log_file"`
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
			LogLevel:    "warning",
			LogFile:     "vless_client.log",
		}, nil // Return default values if file not found or error
	}

	state := &ProgramState{}

	// Explicitly read from Settings section
	section, err := cfg.GetSection("Settings")
	if err != nil {
		return &ProgramState{
			ConfigFile:  "config.json (TCP)",
			SystemProxy: false,
		}, nil
	}

	state.ConfigFile = section.Key("ConfigFile").String()
	state.SystemProxy, _ = section.Key("SystemProxy").Bool()
	state.HttpProxy = section.Key("http_proxy").String()
	state.SocksProxy = section.Key("socks_proxy").String()
	state.SeparateTunneling = section.Key("separate_tunneling").String()
	state.LogLevel = section.Key("log_level").String()
	state.LogFile = section.Key("log_file").String()
	if state.LogLevel == "" {
		state.LogLevel = "warning"
	}
	if state.LogFile == "" {
		state.LogFile = "vless_client.log"
	}

	// Validate config file value
	if state.ConfigFile != "config.json (TCP)" && state.ConfigFile != "config_xhttp.json (XHTTP)" {
		state.ConfigFile = "config.json (TCP)"
	}

	infoLog("LoadProgramState: ConfigFile=%s, SystemProxy=%v", state.ConfigFile, state.SystemProxy)
	return state, nil
}

// SaveProgramState saves the program state to system.ini
func SaveProgramState(state *ProgramState) error {
	var cfg *ini.File
	var err error

	// Get the correct path for system.ini based on platform
	configPath := client.GetConfigFilesPath()
	systemIniPath := filepath.Join(configPath, "system.ini")

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
	section.Key("http_proxy").SetValue(state.HttpProxy)
	section.Key("socks_proxy").SetValue(state.SocksProxy)
	section.Key("separate_tunneling").SetValue(state.SeparateTunneling)
	section.Key("log_level").SetValue(state.LogLevel)
	section.Key("log_file").SetValue(state.LogFile)

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
func enableWindowsProxy(socksPort, httpPort int, exceptions string) error {
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

// Global flags
var debugMode bool
var logMode bool

// Log configuration
var logLevel string = "warning"
var logFileName string = "vless_client.log"

// Log writers
var logFile *os.File

func levelAllows(level string) bool {
	switch logLevel {
	case "debug":
		return true
	case "warning":
		return level == "warning" || level == "error" || level == "access"
	default:
		return false
	}
}

func writeLog(prefix string, level string, format string, args ...any) {
	if logFile == nil || !levelAllows(level) {
		return
	}
	msg := fmt.Sprintf("["+prefix+"] "+format+"\n", args...)
	logFile.WriteString(msg)
	logFile.Sync()
}

func debugLog(format string, args ...any) {
	writeLog("DEBUG", "debug", format, args...)
}

func infoLog(format string, args ...any) {
	writeLog("INFO", "debug", format, args...)
}

func warnLog(format string, args ...any) {
	writeLog("WARNING", "warning", format, args...)
}

func errorLog(format string, args ...any) {
	writeLog("ERROR", "error", format, args...)
}

func accessLog(format string, args ...any) {
	writeLog("ACCESS", "access", format, args...)
}

// openLogFiles opens log files if needed
func openLogFiles() error {
	if logLevel == "none" {
		return nil
	}
	if logFileName == "" {
		logFileName = "vless_client.log"
	}
	var err error
	logFile, err = os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	accessLog("Main log file opened (level=%s)", logLevel)
	return nil
}

// closeLogFiles closes log files
func closeLogFiles() {
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

	// Load program state from system.ini
	state, err := LoadProgramState()
	if err != nil {
		state = &ProgramState{
			ConfigFile:  "config.json (TCP)",
			SystemProxy: false,
			LogLevel:    "warning",
			LogFile:     "vless_client.log",
		}
	}
	if debugMode {
		logLevel = "debug"
	} else if logMode {
		logLevel = "warning"
	} else if state.LogLevel != "" {
		logLevel = state.LogLevel
	}
	if state.LogFile != "" {
		logFileName = state.LogFile
	}

	// Open log files if needed
	if err := openLogFiles(); err != nil {
		os.Exit(1)
	}
	defer closeLogFiles()

	// Load encrypted config
	configStorage, err := client.LoadConfigFromVLESS(filepath.Join(client.GetConfigFilesPath(), "config.vless"))
	if err != nil {
		errorLog("Failed to load config: %v", err)
		os.Exit(1)
	}

	// Get initial config based on saved state
	var cfg *client.Config
	if state.ConfigFile == "config.json (TCP)" {
		cfg = configStorage.TCP
	} else {
		cfg = configStorage.XHTTP
	}
	cfg.SeparateTunneling = state.SeparateTunneling

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
