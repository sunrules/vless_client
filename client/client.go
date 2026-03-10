// Package client handles the VLESS client implementation
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xtls/xray-core/core"
	_ "github.com/xtls/xray-core/main/distro/all" // Import all config formats (JSON, YAML, TOML)
)

// Client wraps xray-core
type Client struct {
	core    *core.Instance
	config  *Config
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// LoadConfigFromVLESS loads configuration from encrypted config.vless file
func LoadConfigFromVLESS(configPath string) (*ConfigStorage, error) {
	if configPath == "" {
		// Get default platform-specific path
		configPath = GetConfigFilesPath()
	}

	// Strip file:// prefix if present (for Android URI)
	if len(configPath) > 7 && configPath[:7] == "file://" {
		configPath = configPath[7:]
		// On Windows, file:///D:/... becomes /D:/..., so we need to trim leading slash
		if len(configPath) > 0 && configPath[0] == '/' && len(configPath) > 2 && configPath[2] == ':' {
			configPath = configPath[1:]
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil || len(data) == 0 {
		// If file not found or empty, create default config
		defaultStorage := &ConfigStorage{
			TCP:   GetDefaultTCPConfig(),
			XHTTP: GetDefaultXHTTPConfig(),
		}
		if err := SaveConfigToVLESS(defaultStorage, configPath); err != nil {
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

// SaveConfigToVLESS saves configuration to encrypted config.vless file
func SaveConfigToVLESS(storage *ConfigStorage, configPath string) error {
	if configPath == "" {
		// Get default platform-specific path
		configPath = GetConfigFilesPath()
	}

	// Strip file:// prefix if present (for Android URI)
	if len(configPath) > 7 && configPath[:7] == "file://" {
		configPath = configPath[7:]
		// On Windows, file:///D:/... becomes /D:/..., so we need to trim leading slash
		if len(configPath) > 0 && configPath[0] == '/' && len(configPath) > 2 && configPath[2] == ':' {
			configPath = configPath[1:]
		}
	}

	// Validate storage
	if storage == nil {
		return fmt.Errorf("storage is nil")
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
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(configPath, encryptedData, 0644); err != nil {
		return err
	}

	return nil
}

// BuildXrayConfigJSON generates JSON config for xray-core
func BuildXrayConfigJSON(cfg *Config) ([]byte, error) {
	// Build config in xray-core format
	xrayCfg := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "warning",
		},
		"inbounds":  []interface{}{},
		"outbounds": []interface{}{},
	}

	// Inbound: SOCKS5 proxy - on Android we need to listen on all interfaces
	if cfg.SOCKS.Enabled {
		xrayCfg["inbounds"] = append(xrayCfg["inbounds"].([]interface{}), map[string]interface{}{
			"tag":      "socks-in",
			"port":     cfg.SOCKS.Port,
			"listen":   "0.0.0.0",
			"protocol": "socks",
			"settings": map[string]interface{}{
				"auth": "noauth",
				"udp":  true,
			},
		})
	}

	// Inbound: HTTP proxy - on Android we need to listen on all interfaces
	if cfg.HTTP.Enabled {
		xrayCfg["inbounds"] = append(xrayCfg["inbounds"].([]interface{}), map[string]interface{}{
			"tag":      "http-in",
			"port":     cfg.HTTP.Port,
			"listen":   "0.0.0.0",
			"protocol": "http",
		})
	}

	// Outbound: VLESS
	vless := cfg.VLESS
	outbound := map[string]interface{}{
		"tag":      "proxy",
		"protocol": "vless",
		"settings": map[string]interface{}{
			"vnext": []interface{}{
				map[string]interface{}{
					"address": vless.Address,
					"port":    vless.Port,
					"users": []interface{}{
						func() map[string]interface{} {
							user := map[string]interface{}{
								"id":         vless.UUID,
								"encryption": "none",
							}
							// Add flow only if not using xhttp transport
							if cfg.Transport == nil || cfg.Transport.Type != "xhttp" {
								if vless.Flow != "" {
									user["flow"] = vless.Flow
								}
							}
							return user
						}(),
					},
				},
			},
		},
		"streamSettings": map[string]interface{}{
			"network": "tcp",
		},
	}

	// Transport settings - only supported protocols
	if cfg.Transport != nil {
		switch cfg.Transport.Type {
		case "tcp":
			outbound["streamSettings"] = map[string]interface{}{
				"network": "tcp",
			}
		case "ws":
			outbound["streamSettings"] = map[string]interface{}{
				"network": "ws",
				"wsSettings": map[string]interface{}{
					"path": cfg.Transport.Path,
					"headers": map[string]interface{}{
						"Host": cfg.Transport.Host,
					},
				},
			}
		case "grpc":
			outbound["streamSettings"] = map[string]interface{}{
				"network": "grpc",
				"grpcSettings": map[string]interface{}{
					"serviceName": cfg.Transport.Path,
				},
			}
		case "http":
			outbound["streamSettings"] = map[string]interface{}{
				"network": "http",
				"httpSettings": map[string]interface{}{
					"path": cfg.Transport.Path,
					"host": cfg.Transport.Host,
				},
			}
		case "xhttp":
			xhttpSettings := map[string]interface{}{
				"path": cfg.Transport.Path,
				"host": cfg.Transport.Host,
			}
			if cfg.Transport.Mode != "" {
				xhttpSettings["mode"] = cfg.Transport.Mode
			}
			if cfg.Transport.Download != "" {
				xhttpSettings["download"] = cfg.Transport.Download
			}
			outbound["streamSettings"] = map[string]interface{}{
				"network":       "xhttp",
				"xhttpSettings": xhttpSettings,
			}
		default:
			outbound["streamSettings"] = map[string]interface{}{
				"network": "tcp",
			}
		}
	}

	// Security settings (TLS/Reality)
	switch vless.Security {
	case "reality":
		streamSettings, ok := outbound["streamSettings"].(map[string]interface{})
		if !ok {
			streamSettings = map[string]interface{}{}
		}
		streamSettings["security"] = "reality"
		realitySettings := map[string]interface{}{
			"publicKey":  vless.PublicKey,
			"shortId":    vless.ShortID,
			"serverName": vless.SNI,
		}
		if vless.Fingerprint != "" {
			realitySettings["fingerprint"] = vless.Fingerprint
		}
		streamSettings["realitySettings"] = realitySettings
		outbound["streamSettings"] = streamSettings
	case "tls":
		streamSettings, ok := outbound["streamSettings"].(map[string]interface{})
		if !ok {
			streamSettings = map[string]interface{}{}
		}
		streamSettings["security"] = "tls"
		tlsSettings := map[string]interface{}{
			"serverName": vless.SNI,
		}
		if vless.Fingerprint != "" {
			tlsSettings["fingerprint"] = vless.Fingerprint
		}
		streamSettings["tlsSettings"] = tlsSettings
		outbound["streamSettings"] = streamSettings
	}

	xrayCfg["outbounds"] = append(xrayCfg["outbounds"].([]interface{}), outbound)

	// Add direct outbound
	xrayCfg["outbounds"] = append(xrayCfg["outbounds"].([]interface{}), map[string]interface{}{
		"tag":      "direct",
		"protocol": "freedom",
	})

	// Add block outbound
	xrayCfg["outbounds"] = append(xrayCfg["outbounds"].([]interface{}), map[string]interface{}{
		"tag":      "block",
		"protocol": "blackhole",
	})

	// Routing: all traffic through VLESS (no geosite/geoip files needed)
	xrayCfg["routing"] = map[string]interface{}{
		"domainStrategy": "IPIfNonMatch",
		"rules": []interface{}{
			map[string]interface{}{
				"type":        "field",
				"outboundTag": "proxy",
				"domain":      []interface{}{"regexp:.*"}, // All domains (regex)
			},
		},
	}

	result, err := json.MarshalIndent(xrayCfg, "", "  ")
	if err != nil {
		return nil, err
	}

	return result, nil
}

// NewClient creates a new VLESS client
func NewClient(cfg *Config) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		config:  cfg,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

// Start starts the client
func (c *Client) Start() error {
	if c.running {
		return fmt.Errorf("client already running")
	}

	// Generate JSON config
	jsonCfg, err := BuildXrayConfigJSON(c.config)
	if err != nil {
		return fmt.Errorf("error creating config: %w", err)
	}

	// Create config directly without writing to temp file
	coreCfg, err := core.LoadConfig("json", bytes.NewReader(jsonCfg))
	if err != nil {
		return fmt.Errorf("config parse error: %w", err)
	}

	// Create xray instance
	instance, err := core.New(coreCfg)
	if err != nil {
		return fmt.Errorf("error creating xray instance: %w", err)
	}

	c.core = instance

	// Start
	if err := c.core.Start(); err != nil {
		return fmt.Errorf("error starting: %w", err)
	}

	c.running = true
	return nil
}

// Stop stops the client
func (c *Client) Stop() error {
	if !c.running || c.core == nil {
		return nil
	}

	c.core.Close()
	c.running = false
	c.core = nil
	c.cancel() // Cancel the context

	return nil
}

// IsRunning returns running status
func (c *Client) IsRunning() bool {
	return c.running
}

// GetStatus returns string status
func (c *Client) GetStatus() string {
	if c.running {
		return "connected"
	}
	return "disconnected"
}
