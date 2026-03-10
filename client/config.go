// Package client provides configuration management for VLESS client
package client

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// ConfigManager interface defines configuration operations
type ConfigManager interface {
	LoadConfig() (*ConfigStorage, error)
	SaveConfig(storage *ConfigStorage) error
	GetDefaultTCPConfig() *Config
	GetDefaultXHTTPConfig() *Config
	ValidateConfig(cfg *Config) error
}

// ConfigStorage holds both configurations
type ConfigStorage struct {
	TCP   *Config `json:"tcp"`
	XHTTP *Config `json:"xhttp"`
}

// Config - full client configuration
type Config struct {
	VLESS     *VLESSConfig     `json:"vless"`
	Transport *TransportConfig `json:"transport,omitempty"`
	SOCKS     struct {
		Enabled bool `json:"enabled"`
		Port    int  `json:"port"`
	} `json:"socks"`
	HTTP struct {
		Enabled bool `json:"enabled"`
		Port    int  `json:"port"`
	} `json:"http"`
}

// VLESSConfig contains VLESS client settings
type VLESSConfig struct {
	Address     string `json:"address"`
	Port        int    `json:"port"`
	UUID        string `json:"uuid"`
	Flow        string `json:"flow,omitempty"`     // xtls-rprx-vision for Reality
	Security    string `json:"security,omitempty"` // reality, tls, none
	PublicKey   string `json:"publicKey,omitempty"`
	ShortID     string `json:"shortId,omitempty"`
	SNI         string `json:"sni,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// TransportConfig contains transport settings
type TransportConfig struct {
	Type     string `json:"type"` // tcp, ws, grpc, http, xhttp
	Path     string `json:"path,omitempty"`
	Host     string `json:"host,omitempty"`
	Mode     string `json:"mode,omitempty"`     // stream, packet or packet-up
	Download string `json:"download,omitempty"` // URL for download (for packet mode)
}

// encryptionKey (should be 16, 24, or 32 bytes long)
var encryptionKey = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f} // Exactly 32 bytes

// Encrypt encrypts data using AES-256-GCM
func Encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// Decrypt decrypts data using AES-256-GCM
func Decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// GetConfigFilesPath returns the path where config.vless and system.ini should be stored
func GetConfigFilesPath() string {
	// On Windows, config files are always in the same directory as the executable
	exePath, err := os.Executable()
	if err != nil {
		return "." // Fallback to current directory
	}
	return filepath.Dir(exePath)
}

// ConfigManagerImpl implements ConfigManager interface
type ConfigManagerImpl struct {
	mu sync.RWMutex
}

// NewConfigManager creates a new ConfigManager
func NewConfigManager() *ConfigManagerImpl {
	return &ConfigManagerImpl{}
}

// LoadConfig loads configuration from encrypted config.vless file
func (cm *ConfigManagerImpl) LoadConfig() (*ConfigStorage, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	configPath := GetConfigFilesPath()
	systemIniPath := filepath.Join(configPath, "config.vless")

	data, err := os.ReadFile(systemIniPath)
	if err != nil || len(data) == 0 {
		// If file not found or empty, create default config
		defaultStorage := &ConfigStorage{
			TCP:   GetDefaultTCPConfig(),
			XHTTP: GetDefaultXHTTPConfig(),
		}
		if err := cm.SaveConfig(defaultStorage); err != nil {
			return nil, err
		}
		return defaultStorage, nil
	}

	// Decrypt data
	decryptedData, err := Decrypt(data)
	if err != nil {
		return nil, err
	}

	var storage ConfigStorage
	if err := json.Unmarshal(decryptedData, &storage); err != nil {
		return nil, err
	}

	// Ensure both configs are present
	if storage.TCP == nil {
		storage.TCP = GetDefaultTCPConfig()
	}
	if storage.XHTTP == nil {
		storage.XHTTP = GetDefaultXHTTPConfig()
	}

	return &storage, nil
}

// SaveConfig saves configuration to encrypted config.vless file
func (cm *ConfigManagerImpl) SaveConfig(storage *ConfigStorage) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if storage == nil {
		return errors.New("storage is nil")
	}
	if storage.TCP == nil {
		storage.TCP = GetDefaultTCPConfig()
	}
	if storage.XHTTP == nil {
		storage.XHTTP = GetDefaultXHTTPConfig()
	}

	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return err
	}

	encryptedData, err := Encrypt(data)
	if err != nil {
		return err
	}

	// Ensure the directory exists before saving
	dir := GetConfigFilesPath()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(dir, "config.vless")
	if err := os.WriteFile(configPath, encryptedData, 0644); err != nil {
		return err
	}

	return nil
}

// GetDefaultTCPConfig returns default TCP transport configuration
func GetDefaultTCPConfig() *Config {
	cfg := &Config{
		VLESS: &VLESSConfig{
			Address:     "ipv4/ipv6",
			Port:        4434,
			UUID:        "User ID",
			Flow:        "xtls-rprx-vision",
			Security:    "reality",
			PublicKey:   "Public Key",
			ShortID:     "62",
			SNI:         "www.google.com",
			Fingerprint: "chrome",
		},
		Transport: &TransportConfig{
			Type: "tcp",
			Mode: "stream",
		},
	}
	cfg.SOCKS.Enabled = true
	cfg.SOCKS.Port = 1080
	cfg.HTTP.Enabled = true
	cfg.HTTP.Port = 1082
	return cfg
}

// GetDefaultXHTTPConfig returns default XHTTP transport configuration
func GetDefaultXHTTPConfig() *Config {
	cfg := &Config{
		VLESS: &VLESSConfig{
			Address:     "ipv4/ipv6",
			Port:        4434,
			UUID:        "User ID",
			Security:    "reality",
			PublicKey:   "Public Key",
			ShortID:     "62",
			SNI:         "www.google.com",
			Fingerprint: "chrome",
		},
		Transport: &TransportConfig{
			Type: "xhttp",
			Path: "/xh",
			Mode: "packet-up",
		},
	}
	cfg.SOCKS.Enabled = true
	cfg.SOCKS.Port = 1080
	cfg.HTTP.Enabled = true
	cfg.HTTP.Port = 1082
	return cfg
}

// ValidateConfig validates configuration syntax and content
func ValidateConfig(cfg *Config) error {
	if cfg.VLESS == nil {
		return errors.New("missing vless section")
	}
	if cfg.VLESS.Address == "" {
		return errors.New("server address not specified")
	}
	if cfg.VLESS.UUID == "" {
		return errors.New("UUID not specified")
	}
	if cfg.VLESS.Port <= 0 || cfg.VLESS.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", cfg.VLESS.Port)
	}
	if cfg.SOCKS.Port <= 0 || cfg.SOCKS.Port > 65535 {
		return fmt.Errorf("invalid SOCKS port: %d", cfg.SOCKS.Port)
	}
	if cfg.HTTP.Port <= 0 || cfg.HTTP.Port > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", cfg.HTTP.Port)
	}
	if cfg.VLESS.Security == "reality" {
		if cfg.VLESS.PublicKey == "" {
			return errors.New("public key is required for Reality security")
		}
		if cfg.VLESS.SNI == "" {
			return errors.New("SNI is required for Reality security")
		}
	}
	if cfg.VLESS.Security == "tls" && cfg.VLESS.SNI == "" {
		return errors.New("SNI is required for TLS security")
	}
	return nil
}