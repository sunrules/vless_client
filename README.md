# VLESS Client with Reality and XHTTP Support

This is a GUI client for working with the VLESS protocol and Reality technology. The client is developed in Go using the Fyne library for the graphical interface.

## Features

- Support for VLESS protocol with Reality technology (xtls-rprx-vision)
- Support for Transports: TCP, WebSocket, gRPC, HTTP/2 (h2), XHTTP
- SOCKS5 and HTTP proxy servers for local traffic
- Encryption settings: None, TLS, Reality
- Built-in debug mode for debugging
- **Configuration encryption**: Configuration is stored in encrypted form (AES-256-GCM) in `config.vless` file
- **Program state storage**: Uses `system.ini` to save the last state
- **Configuration file management**: Load and save configurations from/to files
- **System tray**: Support for working from the system tray (minimize window instead of closing)
- **Windows system proxy**: Automatic enabling/disabling of the system proxy
- **UAC (Loopback Route)**: (User Account Control) Management of static route for loopback interface with high priority. Improves access to local resources and bypasses VPN/proxy for local traffic.
- **Configuration check**: "Check Configuration" button to validate settings
- **Logging**: Support for logging mode with saving to `debug.log` and `vless_client.log` files
- Automatic saving of configuration file
- "Set Default" button for quick restoration of default settings

## Installation and Running

### Requirements

- Go 1.24.0 or higher
- Windows (for other platforms adaptation is required)
- For Reality to work, valid public and short identifiers are required

### Running from Source Code

```bash
# Clone the repository
git clone https://github.com/sunrules/vless_client.git
cd vless_client

# Install dependencies
go mod tidy

# Build and run (Windows)
$env:CGO_ENABLED = "1"; $env:GOOS = "windows"; $env:GOARCH = "amd64"; go build -ldflags '-s -w' -o vless_client.exe -ldflags -H=windowsgui

# Run with debugging
.\vless_client.exe -debug

# Run with logging
.\vless_client.exe -log
```

## Configuration

### Main Parameters

On first launch, the program loads the configuration from the encrypted `config.vless` file. If the file is missing or contains errors, default values are used. The configuration stores two versions of settings: for TCP and XHTTP.

### Configuration Storage

The configuration is encrypted using AES-256-GCM and saved in the `config.vless` file. The program automatically creates this file on first run with default values.

### ConfigStorage Structure

```json
{
  "tcp": {
    "vless": {
      "address": "ipv4/ipv6",
      "port": 4434,
      "uuid": "User ID",
      "flow": "xtls-rprx-vision",
      "security": "reality",
      "publicKey": "Public Key",
      "shortId": "62",
      "sni": "www.google.com",
      "fingerprint": "chrome"
    },
    "transport": {
      "type": "tcp",
      "path": "",
      "host": "",
      "mode": "stream",
      "download": ""
    },
    "socks": {
      "enabled": true,
      "port": 1080
    },
    "http": {
      "enabled": true,
      "port": 1082
    }
  },
  "xhttp": {
    "vless": {
      "address": "ipv4/ipv6",
      "port": 4434,
      "uuid": "User ID",
      "security": "reality",
      "publicKey": "Public Key",
      "shortId": "62",
      "sni": "www.google.com",
      "fingerprint": "chrome"
    },
    "transport": {
      "type": "xhttp",
      "path": "/xh",
      "host": "",
      "mode": "packet-up",
      "download": ""
    },
    "socks": {
      "enabled": true,
      "port": 1080
    },
    "http": {
      "enabled": true,
      "port": 1082
    }
  }
}
```

### VLESS Parameters

| Field         | Description                                                                 |
|----------------|-----------------------------------------------------------------------------|
| address        | Server IP address or domain                                                 |
| port           | Server port                                                                |
| uuid           | Unique user identifier                                                     |
| flow           | Flow type (for Reality use `xtls-rprx-vision`)                             |
| security       | Encryption type: `none`, `tls`, or `reality`                                |
| publicKey      | Reality public key                                                          |
| shortId        | Reality short identifier (optional but recommended)                        |
| sni            | SNI (Server Name Indication) for TLS or Reality                            |
| fingerprint    | TLS fingerprint (chrome, firefox, safari, edge)                            |

### Transport Settings

#### TCP

```json
"transport": {
  "type": "tcp"
}
```

#### WebSocket

```json
"transport": {
  "type": "ws",
  "path": "/path",
  "host": "example.com"
}
```

#### gRPC

```json
"transport": {
  "type": "grpc",
  "path": "service_name"
}
```

#### HTTP/2 (h2)

```json
"transport": {
  "type": "http",
  "path": "/path",
  "host": "example.com"
}
```

#### XHTTP

```json
"transport": {
  "type": "xhttp",
  "path": "/splithttp",        // Path for XHTTP
  "host": "example.com",        // Host header
  "mode": "packet",             // Mode: "stream" or "packet" or "packet-up"
  "download": "https://example.com/download"  // Download URL (only for packet mode)
}
```

### Local Proxy Settings

```json
"socks": {
  "enabled": true,
  "port": 1080
},
"http": {
  "enabled": true,
  "port": 1082
}
```

## Working with XHTTP

XHTTP is a transport protocol that improves bypass capabilities by splitting traffic into separate requests.

### Client Settings

```json
"transport": {
  "type": "xhttp",
  "path": "/xhttp",
  "host": "example.com",
  "mode": "stream",
  "download": ""
}
```

### Server Settings

For XHTTP to work, you need a server with support for this protocol. Example server configuration:

```json
{
  "inbounds": [
    {
      "port": 443,
      "protocol": "vless",
      "settings": {
        "clients": [
          {
            "id": "User ID",
            "flow": "xtls-rprx-vision"
          }
        ]
      },
      "streamSettings": {
        "network": "xhttp",
        "xhttpSettings": {
          "path": "/xhttp",
          "host": "example.com"
        },
        "security": "reality",
        "realitySettings": {
          "show": false,
          "dest": "example.com:443",
          "xver": 0,
          "serverNames": [
            "example.com"
          ],
          "privateKey": "Your private key",
          "minClientVer": "",
          "maxClientVer": "",
          "maxTimeDiff": 0,
          "shortIds": [
            "62"
          ]
        }
      }
    }
  ],
  "outbounds": [
    {
      "protocol": "freedom",
      "tag": "direct"
    }
  ]
}
```

## Using the GUI

1. Run `vless_client.exe`
2. **Configuration selection**: Use the dropdown menu to select between TCP and XHTTP configurations
3. **Configuration management**:
   - "Load config": Load configuration from file
   - "Save config": Save current configuration to file
   - "Edit config": Open settings window for editing
4. **Main functions**:
   - "Connect": Start proxy servers
   - "System Proxy": Enable/disable system proxy (Windows)
   - "UAC":(User Account Control) Manage static route for loopback interface
   - "Exit": Close the application
5. **Validation**: "Check Configuration" button checks settings correctness
6. **Restore defaults**: "Set Default" button returns to standard values
7. **Debug**: Use `-debug` argument for debug mode and `-log` for logging
8. **System tray**: When closing the window, the application minimizes to system tray

## Logging

### Logging Modes

- **Debug mode**: Detailed information for debugging (file `debug.log`)
- **Log mode**: Main events (file `vless_client.log`)

```bash
# Run with debugging
.\vless_client.exe -debug

# Run with logging
.\vless_client.exe -log

# Run with both modes
.\vless_client.exe -debug -log
```

### Log Structure

- `[DEBUG]`: Detailed information for developers
- `[INFO]`: Main client operation events
- `[ERROR]`: Errors and critical issues

## UAC (Loopback Route) Mode

### What is UAC Mode?

The UAC (User Account Control) mode manages a static network route for the loopback interface (127.0.0.1) with high priority. It adds a static route:
```
route add 127.0.0.1 mask 255.255.255.255 127.0.0.1 metric 1
```

This route ensures that traffic to the local host always goes through the loopback interface with the highest priority, bypassing any other network interfaces (such as VPN or proxy).

### When to Use UAC Mode

UAC mode can be helpful in several scenarios:

#### 1. Bypass VPN for Local Traffic
If you are connected to a VPN, all network traffic (including to 127.0.0.1) may be routed through the VPN interface. This can cause issues with the local proxy servers (SOCKS5 on 1080 and HTTP on 1082) that the VLESS Client creates.

#### 2. Improve Local Traffic Performance
The high-priority static route guarantees that local traffic is always sent directly through the loopback interface, resulting in the fastest possible response times.

#### 3. Fix Local Resource Access
If you're experiencing problems accessing local services (like `localhost:3000` or `127.0.0.1:8080`) when the client is running, especially when connected to a VPN, enabling UAC mode can resolve these issues.

#### 4. Troubleshoot Proxy Connection Problems
When using system proxy settings, sometimes the local proxy servers might not be reachable. Enabling UAC mode ensures that the proxy servers are always accessible.

### How to Use UAC Mode

1. Click the "UAC" button in the main interface
2. If the button text changes to "UAC (off)", the mode is enabled
3. To disable, click the button again - it will change back to "UAC"
4. The mode requires elevated privileges, so you may see a UAC prompt when clicking the button

## Problems and Solutions

### Launch Errors

- **core: Unable to load config injson**: Connection error to server or invalid configuration
- **infra/conf/serial: failed to parse json config**: JSON config format error
- **unknown transport protocol: xhttp**: Server does not support XHTTP

### Solutions

1. Check server connection
2. Check Reality settings correctness
3. Ensure server supports selected transport
4. For XHTTP, use only compatible Xray Core versions
5. Check configuration file path correctness
6. If local proxy servers are not reachable, try enabling UAC mode

## License

MIT License. See LICENSE file for more information.

## Authors

Developed using:
- Xray Core: https://github.com/xtls/xray-core
- Fyne: https://github.com/fyne-io/fyne