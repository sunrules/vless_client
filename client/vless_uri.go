package client

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// normalizeAddress ensures IPv6 addresses are wrapped in brackets, as required
// by xray-core config format and URL conventions. IPv4 addresses and hostnames
// are returned unchanged.
func normalizeAddress(host string) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		ip := net.ParseIP(host)
		if ip != nil && ip.To4() == nil {
			return "[" + host + "]"
		}
	}
	return host
}

// ParseVLESSURI parses a vless:// URI into Config
func ParseVLESSURI(uri string) (*Config, error) {
	uri = strings.TrimSpace(uri)
	if !strings.HasPrefix(uri, "vless://") {
		return nil, fmt.Errorf("not a vless URI")
	}
	uri = strings.TrimSpace(uri)
	// Remove possible line breaks
	uri = strings.ReplaceAll(uri, "\n", "")
	uri = strings.ReplaceAll(uri, "\r", "")
	u, err := url.Parse(uri)
	if err != nil {
		u, err = url.ParseRequestURI(uri)
		if err != nil {
			return nil, fmt.Errorf("parse: %w", err)
		}
	}
	uuid := u.User.Username()
	if uuid == "" {
		return nil, fmt.Errorf("missing UUID")
	}
	host := normalizeAddress(u.Hostname())
	portStr := u.Port()
	port := 443
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, err
		}
		port = p
	}
	q := u.Query()

	cfg := &Config{
		VLESS: &VLESSConfig{
			Address:     host,
			Port:        port,
			UUID:        uuid,
			Security:    q.Get("security"),
			SNI:         q.Get("sni"),
			Fingerprint: q.Get("fp"),
			PublicKey:   q.Get("pbk"),
			ShortID:     q.Get("sid"),
			Flow:        q.Get("flow"),
		},
		SOCKS: struct {
			Enabled bool `json:"enabled"`
			Port    int  `json:"port"`
		}{Enabled: true, Port: 1080},
		HTTP: struct {
			Enabled bool `json:"enabled"`
			Port    int  `json:"port"`
		}{Enabled: true, Port: 1082},
	}
	// transport
	t := q.Get("type")
	if t == "" {
		t = "tcp"
	}
	tr := &TransportConfig{Type: t}
	switch t {
	case "xhttp":
		tr.Path = q.Get("path")
		tr.Mode = q.Get("mode")
		tr.Host = q.Get("host")
	case "ws":
		tr.Path = q.Get("path")
		tr.Host = q.Get("host")
	case "grpc":
		tr.Path = q.Get("serviceName")
	case "http":
		tr.Path = q.Get("path")
		tr.Host = q.Get("host")
	}
	cfg.Transport = tr
	// default mode for xhttp
	if t == "xhttp" && tr.Mode == "" {
		tr.Mode = "stream-one"
	}
	return cfg, nil
}