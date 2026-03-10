// VLESS Client

// +build !headless

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"v2/client"
	_ "github.com/xtls/xray-core/main/distro/all"
	"gopkg.in/ini.v1"
	"io"
	"os"
	"os/signal"
	"path/filepath"
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

	// Create hidden window for message handling (platform-specific)
	if err := createHiddenWindow(); err != nil {
		fmt.Printf("Error creating hidden window: %v\n", err)
		os.Exit(1)
	}
	defer destroyHiddenWindow()

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

	// Create Fyne app
	a := app.New()

	// Load icon and set as application icon
	iconData, err := os.ReadFile("vless.ico")
	if err == nil {
		icon := fyne.NewStaticResource("vless.ico", iconData)
		a.SetIcon(icon)
	}

	w := a.NewWindow("VLESS Client")

	// Setup system tray if supported (platform-specific)
	setupSystemTray(w, a)

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

	// Configuration screen UI elements
	addressEntry := widget.NewEntry()
	addressEntry.SetText(cfg.VLESS.Address)
	addressEntry.SetPlaceHolder("Server address (IPv4/IPv6/domain)")

	portEntry := widget.NewEntry()
	portEntry.SetText(fmt.Sprintf("%d", cfg.VLESS.Port))
	portEntry.SetPlaceHolder("Port")

	uuidEntry := widget.NewEntry()
	uuidEntry.SetText(cfg.VLESS.UUID)
	uuidEntry.SetPlaceHolder("UUID")

	flowSelect := widget.NewSelect([]string{"", "xtls-rprx-vision"}, func(s string) {})
	flowSelect.SetSelected(cfg.VLESS.Flow)

	securitySelect := widget.NewSelect([]string{"none", "tls", "reality"}, func(s string) {})
	securitySelect.SetSelected(cfg.VLESS.Security)

	transportSelect := widget.NewSelect([]string{"tcp", "ws", "grpc", "http", "xhttp"}, func(s string) {})
	if cfg.Transport != nil {
		supported := []string{"tcp", "ws", "grpc", "http", "xhttp"}
		found := false
		for _, t := range supported {
			if t == cfg.Transport.Type {
				found = true
				break
			}
		}
		if found {
			transportSelect.SetSelected(cfg.Transport.Type)
		} else {
			transportSelect.SetSelected("tcp")
		}
	} else {
		transportSelect.SetSelected("tcp")
	}

	transportPathEntry := widget.NewEntry()
	if cfg.Transport != nil {
		transportPathEntry.SetText(cfg.Transport.Path)
	}
	transportPathEntry.SetPlaceHolder("Path (for ws/grpc/xhttp)")

	transportHostEntry := widget.NewEntry()
	if cfg.Transport != nil {
		transportHostEntry.SetText(cfg.Transport.Host)
	}
	transportHostEntry.SetPlaceHolder("Host header")

	// Reality settings
	publicKeyEntry := widget.NewEntry()
	publicKeyEntry.SetText(cfg.VLESS.PublicKey)
	publicKeyEntry.SetPlaceHolder("Public Key")

	shortIdEntry := widget.NewEntry()
	shortIdEntry.SetText(cfg.VLESS.ShortID)
	shortIdEntry.SetPlaceHolder("Short ID")

	sniEntry := widget.NewEntry()
	sniEntry.SetText(cfg.VLESS.SNI)
	sniEntry.SetPlaceHolder("SNI (server name)")

	fingerprintSelect := widget.NewSelect([]string{"", "chrome", "firefox", "safari", "edge"}, func(s string) {})
	fingerprintSelect.SetSelected(cfg.VLESS.Fingerprint)

	socksPortEntry := widget.NewEntry()
	socksPortEntry.SetText(fmt.Sprintf("%d", cfg.SOCKS.Port))
	socksPortEntry.SetPlaceHolder("SOCKS port")

	httpPortEntry := widget.NewEntry()
	httpPortEntry.SetText(fmt.Sprintf("%d", cfg.HTTP.Port))
	httpPortEntry.SetPlaceHolder("HTTP port")

	configStatusLabel := widget.NewLabel("")

	// XHTTP mode selection - only visible if XHTTP transport selected
	modeSelect := widget.NewSelect([]string{"", "stream", "packet", "packet-up"}, func(s string) {
		if cfg.Transport == nil {
			cfg.Transport = &client.TransportConfig{}
		}
		cfg.Transport.Mode = s
	})
	if cfg.Transport != nil && cfg.Transport.Mode != "" {
		modeSelect.SetSelected(cfg.Transport.Mode)
	}

	// Config file selector
	var configFileSelect *widget.Select
	configFileSelect = widget.NewSelect([]string{"config.json (TCP)", "config_xhttp.json (XHTTP)"}, func(s string) {
		if clientInstance.IsRunning() {
			configStatusLabel.SetText("Error: Cannot change configuration while client is running")
			configFileSelect.SetSelected(state.ConfigFile)
			return
		}

		var newCfg *client.Config
		if s == "config.json (TCP)" {
			newCfg = configStorage.TCP
		} else {
			newCfg = configStorage.XHTTP
		}

		cfg = newCfg
		addressEntry.SetText(cfg.VLESS.Address)
		portEntry.SetText(fmt.Sprintf("%d", cfg.VLESS.Port))
		uuidEntry.SetText(cfg.VLESS.UUID)
		flowSelect.SetSelected(cfg.VLESS.Flow)
		securitySelect.SetSelected(cfg.VLESS.Security)
		if cfg.Transport != nil {
			transportSelect.SetSelected(cfg.Transport.Type)
			transportPathEntry.SetText(cfg.Transport.Path)
			transportHostEntry.SetText(cfg.Transport.Host)
		}
		publicKeyEntry.SetText(cfg.VLESS.PublicKey)
		shortIdEntry.SetText(cfg.VLESS.ShortID)
		sniEntry.SetText(cfg.VLESS.SNI)
		fingerprintSelect.SetSelected(cfg.VLESS.Fingerprint)
		socksPortEntry.SetText(fmt.Sprintf("%d", cfg.SOCKS.Port))
		httpPortEntry.SetText(fmt.Sprintf("%d", cfg.HTTP.Port))

		clientInstance, err = client.NewClient(cfg)
		if err != nil {
			errorLog("Error creating client: %v", err)
		}

		state.ConfigFile = s
		if err := SaveProgramState(state); err != nil {
			errorLog("Error saving program state: %v", err)
		} else {
			infoLog("Program state saved: ConfigFile=%s", state.ConfigFile)
		}
	})

	// Configuration buttons
	var configContainer *container.Scroll
	var mainContainer *fyne.Container

	// Back button (returns to main screen)
	backBtn := widget.NewButton("Back", func() {
		w.SetContent(mainContainer)
		w.Resize(fyne.NewSize(450, 500))
	})

	// Save button
	saveBtn := widget.NewButton("Save Configuration", func() {
		cfg.VLESS.Address = addressEntry.Text
		fmt.Sscanf(portEntry.Text, "%d", &cfg.VLESS.Port)
		cfg.VLESS.UUID = uuidEntry.Text
		cfg.VLESS.Flow = flowSelect.Selected
		cfg.VLESS.Security = securitySelect.Selected
		cfg.VLESS.PublicKey = publicKeyEntry.Text
		cfg.VLESS.ShortID = shortIdEntry.Text
		cfg.VLESS.SNI = sniEntry.Text
		cfg.VLESS.Fingerprint = fingerprintSelect.Selected

		if cfg.Transport == nil {
			cfg.Transport = &client.TransportConfig{}
		}
		cfg.Transport.Type = transportSelect.Selected
		cfg.Transport.Path = transportPathEntry.Text
		cfg.Transport.Host = transportHostEntry.Text

		fmt.Sscanf(socksPortEntry.Text, "%d", &cfg.SOCKS.Port)
		fmt.Sscanf(httpPortEntry.Text, "%d", &cfg.HTTP.Port)

		if configFileSelect.Selected == "config.json (TCP)" {
			cfg.Transport.Type = "tcp"
			cfg.VLESS.Flow = "xtls-rprx-vision"
			configStorage.TCP = cfg
		} else {
			cfg.Transport.Type = "xhttp"
			cfg.Transport.Path = "/xh"
			if cfg.Transport.Mode == "" {
				cfg.Transport.Mode = "packet-up"
			}
			cfg.VLESS.Security = "reality"
			cfg.VLESS.Flow = ""
			configStorage.XHTTP = cfg
		}

		if err := client.SaveConfigToVLESS(configStorage, state.ConfigPath); err != nil {
			configStatusLabel.SetText("Error: " + err.Error())
			return
		}

		clientInstance, err = client.NewClient(cfg)
		if err != nil {
			configStatusLabel.SetText("Error: " + err.Error())
			return
		}

		configStatusLabel.SetText("Configuration saved successfully!")
	})

	// Set Default button
	setDefaultBtn := widget.NewButton("Set Default", func() {
		var defaultCfg *client.Config
		if configFileSelect.Selected == "config.json (TCP)" {
			defaultCfg = client.GetDefaultTCPConfig()
		} else {
			defaultCfg = client.GetDefaultXHTTPConfig()
		}

		addressEntry.SetText(defaultCfg.VLESS.Address)
		portEntry.SetText(fmt.Sprintf("%d", defaultCfg.VLESS.Port))
		uuidEntry.SetText(defaultCfg.VLESS.UUID)
		flowSelect.SetSelected(defaultCfg.VLESS.Flow)
		securitySelect.SetSelected(defaultCfg.VLESS.Security)
		transportSelect.SetSelected(defaultCfg.Transport.Type)
		transportPathEntry.SetText(defaultCfg.Transport.Path)
		transportHostEntry.SetText(defaultCfg.Transport.Host)
		publicKeyEntry.SetText(defaultCfg.VLESS.PublicKey)
		shortIdEntry.SetText(defaultCfg.VLESS.ShortID)
		sniEntry.SetText(defaultCfg.VLESS.SNI)
		fingerprintSelect.SetSelected(defaultCfg.VLESS.Fingerprint)
		socksPortEntry.SetText(fmt.Sprintf("%d", defaultCfg.SOCKS.Port))
		httpPortEntry.SetText(fmt.Sprintf("%d", defaultCfg.HTTP.Port))
		if defaultCfg.Transport.Mode != "" {
			modeSelect.SetSelected(defaultCfg.Transport.Mode)
		}

		*cfg = *defaultCfg

		configStatusLabel.SetText("Configuration set to default values!")
	})

	// Validate button
	validateBtn := widget.NewButton("Check Configuration", func() {
		tempCfg := &client.Config{
			VLESS: &client.VLESSConfig{
				Address:     addressEntry.Text,
				Port:        cfg.VLESS.Port,
				UUID:        uuidEntry.Text,
				Flow:        flowSelect.Selected,
				Security:    securitySelect.Selected,
				PublicKey:   publicKeyEntry.Text,
				ShortID:     shortIdEntry.Text,
				SNI:         sniEntry.Text,
				Fingerprint: fingerprintSelect.Selected,
			},
			Transport: &client.TransportConfig{
				Type: transportSelect.Selected,
				Path: transportPathEntry.Text,
				Host: transportHostEntry.Text,
			},
		}

		fmt.Sscanf(portEntry.Text, "%d", &tempCfg.VLESS.Port)
		fmt.Sscanf(socksPortEntry.Text, "%d", &tempCfg.SOCKS.Port)
		fmt.Sscanf(httpPortEntry.Text, "%d", &tempCfg.HTTP.Port)
		tempCfg.SOCKS.Enabled = true
		tempCfg.HTTP.Enabled = true

		if err := client.ValidateConfig(tempCfg); err != nil {
			configStatusLabel.SetText("Error: " + err.Error())
			return
		}

		if _, err := client.BuildXrayConfigJSON(tempCfg); err != nil {
			configStatusLabel.SetText("Error: " + err.Error())
			return
		}

		configStatusLabel.SetText("Configuration is valid!")
	})

	// Config form
	configForm := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Address", Widget: addressEntry},
			{Text: "Port", Widget: portEntry},
			{Text: "UUID", Widget: uuidEntry},
			{Text: "Flow", Widget: flowSelect},
			{Text: "Security", Widget: securitySelect},
			{Text: "Transport", Widget: transportSelect},
			{Text: "Path", Widget: transportPathEntry},
			{Text: "Host", Widget: transportHostEntry},
			{Text: "Mode (XHTTP)", Widget: modeSelect},
			{Text: "Public Key", Widget: publicKeyEntry},
			{Text: "Short ID", Widget: shortIdEntry},
			{Text: "SNI", Widget: sniEntry},
			{Text: "Fingerprint", Widget: fingerprintSelect},
			{Text: "SOCKS port", Widget: socksPortEntry},
			{Text: "HTTP port", Widget: httpPortEntry},
		},
	}

	// Configuration screen
	configContainer = container.NewScroll(container.NewVBox(
		widget.NewLabelWithStyle("Configuration", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		configForm,
		widget.NewSeparator(),
		container.NewHBox(backBtn, saveBtn, validateBtn, setDefaultBtn),
		configStatusLabel,
	))

	// UI elements for main screen
	statusLabel := widget.NewLabel("Status: disconnected")

	// Proxy status indicators
	httpProxyIndicator := widget.NewLabelWithStyle("HTTP Proxy: ❌", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	socksProxyIndicator := widget.NewLabelWithStyle("SOCKS Proxy: ❌", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	systemProxyIndicator := widget.NewLabelWithStyle("System Proxy: ❌", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	var connectBtn *widget.Button
	var systemProxyBtn *widget.Button
	var configPathLabel *widget.Label

	// Config path display
	configPathLabel = widget.NewLabel("Config file path: " + state.ConfigPath)

	// Load config button
	loadConfigBtn := widget.NewButton("Load config", func() {
		if clientInstance.IsRunning() {
			statusLabel.SetText("Error: Cannot load config while client is running")
			return
		}

		// Show file open dialog
		fileDlg := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				statusLabel.SetText("Error: " + err.Error())
				return
			}
			if reader == nil {
				// User canceled dialog
				return
			}
			defer reader.Close()

			// Read and decrypt config
			data, err := io.ReadAll(reader)
			if err != nil {
				statusLabel.SetText("Error: " + err.Error())
				return
			}

			decryptedData, err := client.Decrypt(data)
			if err != nil {
				statusLabel.SetText("Error: Invalid config file")
				return
			}

			var storage client.ConfigStorage
			if err := json.Unmarshal(decryptedData, &storage); err != nil {
				statusLabel.SetText("Error: Invalid config format")
				return
			}

			// Validate loaded config
			if storage.TCP == nil {
				storage.TCP = client.GetDefaultTCPConfig()
			}
			if storage.XHTTP == nil {
				storage.XHTTP = client.GetDefaultXHTTPConfig()
			}

			// Save loaded config to selected path
			if err := client.SaveConfigToVLESS(&storage, reader.URI().String()); err != nil {
				statusLabel.SetText("Error: " + err.Error())
				return
			}
			// Save config path to program state
			state.ConfigPath = reader.URI().String()
			if err := SaveProgramState(state); err != nil {
				errorLog("Error saving program state: %v", err)
			} else {
				infoLog("Program state saved: ConfigPath=%s", state.ConfigPath)
			}

			// Update current config storage
			configStorage = &storage

			// Refresh UI based on current selected config
			var newCfg *client.Config
			if configFileSelect.Selected == "config.json (TCP)" {
				newCfg = configStorage.TCP
			} else {
				newCfg = configStorage.XHTTP
			}

			cfg = newCfg
			addressEntry.SetText(cfg.VLESS.Address)
			portEntry.SetText(fmt.Sprintf("%d", cfg.VLESS.Port))
			uuidEntry.SetText(cfg.VLESS.UUID)
			flowSelect.SetSelected(cfg.VLESS.Flow)
			securitySelect.SetSelected(cfg.VLESS.Security)
			if cfg.Transport != nil {
				transportSelect.SetSelected(cfg.Transport.Type)
				transportPathEntry.SetText(cfg.Transport.Path)
				transportHostEntry.SetText(cfg.Transport.Host)
			}
			publicKeyEntry.SetText(cfg.VLESS.PublicKey)
			shortIdEntry.SetText(cfg.VLESS.ShortID)
			sniEntry.SetText(cfg.VLESS.SNI)
			fingerprintSelect.SetSelected(cfg.VLESS.Fingerprint)
			socksPortEntry.SetText(fmt.Sprintf("%d", cfg.SOCKS.Port))
			httpPortEntry.SetText(fmt.Sprintf("%d", cfg.HTTP.Port))

			clientInstance, err = client.NewClient(cfg)
			if err != nil {
				statusLabel.SetText("Error: " + err.Error())
				return
			}

			configPathLabel.SetText("Config file path: " + state.ConfigPath)
			statusLabel.SetText("Config loaded successfully!")
		}, w)
		fileDlg.SetFileName("config.vless")
		fileDlg.Show()
	})

	// Save config button
	saveConfigBtn := widget.NewButton("Save config", func() {
		// Show file save dialog
		fileDlg := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil {
				statusLabel.SetText("Error: " + err.Error())
				return
			}
			if writer == nil {
				// User canceled dialog
				return
			}
			defer writer.Close()

			// Encrypt and save config
			data, err := json.MarshalIndent(configStorage, "", "  ")
			if err != nil {
				statusLabel.SetText("Error: " + err.Error())
				return
			}

			encryptedData, err := client.Encrypt(data)
			if err != nil {
				statusLabel.SetText("Error: " + err.Error())
				return
			}

			if _, err := writer.Write(encryptedData); err != nil {
				statusLabel.SetText("Error: " + err.Error())
				return
			}

			// Save config path to program state
			state.ConfigPath = writer.URI().String()
			if err := SaveProgramState(state); err != nil {
				errorLog("Error saving program state: %v", err)
			} else {
				infoLog("Program state saved: ConfigPath=%s", state.ConfigPath)
			}

			configPathLabel.SetText("Config file path: " + state.ConfigPath)
			statusLabel.SetText("Config saved successfully!")
		}, w)
		fileDlg.SetFileName("config.vless")
		fileDlg.Show()
	})

	// Edit config button
	viewConfigBtn := widget.NewButton("Edit config", func() {
		w.SetContent(configContainer)
		w.Resize(fyne.NewSize(500, 700))
		// Disable configuration buttons if client is running
		if clientInstance.IsRunning() {
			saveBtn.Disable()
			setDefaultBtn.Disable()
			validateBtn.Disable()
		} else {
			saveBtn.Enable()
			setDefaultBtn.Enable()
			validateBtn.Enable()
		}
	})

	// UAC button
	var uacBtn *widget.Button
	var uacEnabled bool = false
	uacBtn = widget.NewButton("UAC", func() {
		if uacEnabled {
			// Disable UAC - delete loopback route
			if err := deleteLoopbackRoute(); err != nil {
				errorLog("Error deleting loopback route: %v", err)
			}
			uacEnabled = false
			uacBtn.SetText("UAC")
		} else {
			// Enable UAC - add loopback route
			if err := addLoopbackRoute(); err != nil {
				errorLog("Error adding loopback route: %v", err)
			}
			uacEnabled = true
			uacBtn.SetText("UAC (off)")
		}
	})

	// Main buttons
	connectBtn = widget.NewButton("Connect", func() {
		if !clientInstance.IsRunning() {
			if err := clientInstance.Start(); err != nil {
				statusLabel.SetText("Error: " + err.Error())
				return
			}

			if state.SystemProxy {
				// Enable system proxy without adding loopback route (default behavior)
				// Use enableWindowsProxy directly instead of enableSystemProxy to avoid adding route
				if err := enableWindowsProxy(cfg.SOCKS.Port, cfg.HTTP.Port); err != nil {
					errorLog("Error enabling Windows proxy: %v", err)
				}
				systemProxyIndicator.SetText("System Proxy: ✅")
				systemProxyBtn.SetText("System Proxy (off)")
			} else {
				disableWindowsProxy()
				systemProxyIndicator.SetText("System Proxy: ❌")
				systemProxyBtn.SetText("System Proxy")
			}

			statusLabel.SetText("Status: connected")
			connectBtn.SetText("Disconnect")
			httpProxyIndicator.SetText(fmt.Sprintf("HTTP Proxy: ✅ 127.0.0.1:%d", cfg.HTTP.Port))
			socksProxyIndicator.SetText(fmt.Sprintf("SOCKS Proxy: ✅ 127.0.0.1:%d", cfg.SOCKS.Port))

			viewConfigBtn.Disable()
			configFileSelect.Disable()
			loadConfigBtn.Disable()
			saveConfigBtn.Disable()
		} else {
			clientInstance.Stop()
			// Disable system proxy without deleting loopback route
			disableWindowsProxy()

			statusLabel.SetText("Status: disconnected")
			connectBtn.SetText("Connect")
			httpProxyIndicator.SetText("HTTP Proxy: ❌")
			socksProxyIndicator.SetText("SOCKS Proxy: ❌")
			systemProxyIndicator.SetText("System Proxy: ❌")

			viewConfigBtn.Enable()
			configFileSelect.Enable()
			loadConfigBtn.Enable()
			saveConfigBtn.Enable()
		}
	})

	// System proxy button
	systemProxyBtn = widget.NewButton("System Proxy", func() {
		if clientInstance.IsRunning() {
			if systemProxyIndicator.Text == "System Proxy: ✅" {
				disableWindowsProxy()
				systemProxyIndicator.SetText("System Proxy: ❌")
				systemProxyBtn.SetText("System Proxy")
				state.SystemProxy = false
			} else {
				enableWindowsProxy(cfg.SOCKS.Port, cfg.HTTP.Port)
				systemProxyIndicator.SetText("System Proxy: ✅")
				systemProxyBtn.SetText("System Proxy (off)")
				state.SystemProxy = true
			}

			if err := SaveProgramState(state); err != nil {
				errorLog("Error saving program state: %v", err)
			} else {
				infoLog("Program state saved: SystemProxy=%v", state.SystemProxy)
			}
		} else {
			statusLabel.SetText("Error: Client not connected")
		}
	})

	// Set the saved config file selection in the UI
	configFileSelect.SetSelected(state.ConfigFile)

	// Apply saved system proxy state if needed
	if state.SystemProxy {
		systemProxyBtn.SetText("System Proxy (off)")
		systemProxyIndicator.SetText("System Proxy: ✅")
	} else {
		systemProxyBtn.SetText("System Proxy")
		systemProxyIndicator.SetText("System Proxy: ❌")
	}

	// Config path display
	configPathLabel = widget.NewLabel("Config file path: " + state.ConfigPath)

	// Exit button
	exitBtn := widget.NewButton("Exit", func() {
		if clientInstance.IsRunning() {
			clientInstance.Stop()
			disableSystemProxy()
		}
		a.Quit()
	})

	// Main screen content
	mainContainer = container.NewVBox(
		widget.NewLabelWithStyle("VLESS Client", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		// Configuration selection and file management
		container.NewHBox(
			container.NewVBox(
				widget.NewLabelWithStyle("Select Configuration", fyne.TextAlignCenter, fyne.TextStyle{Bold: false}),
				configFileSelect,
			),
			container.NewVBox(
				widget.NewLabelWithStyle("Config File", fyne.TextAlignCenter, fyne.TextStyle{Bold: false}),
				container.NewHBox(loadConfigBtn, saveConfigBtn),
			),
		),
		widget.NewSeparator(),
		// Main control buttons
		container.NewHBox(
			connectBtn,
			systemProxyBtn,
			viewConfigBtn,
			uacBtn,
		),
		widget.NewSeparator(),
		// Status section
		widget.NewLabelWithStyle("Status", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		statusLabel,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Proxy Status", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		httpProxyIndicator,
		socksProxyIndicator,
		systemProxyIndicator,
		// Config path display
		configPathLabel,
		widget.NewSeparator(),
		exitBtn,
	)

	w.SetContent(mainContainer)
	w.Resize(fyne.NewSize(450, 500))

	// Handle signals for clean shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		clientInstance.Stop()
		disableSystemProxy()
		os.Exit(0)
	}()

	// Handle window close event - minimize to tray instead of exiting (platform-specific)
	w.SetCloseIntercept(func() {
		w.Hide()
	})

	w.ShowAndRun()
}