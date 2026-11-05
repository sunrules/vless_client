// Linux GUI platform implementation (non-headless)
// Build: CGO_ENABLED=1 go build -o vless_client .

//go:build linux && !headless
// +build linux,!headless

package main

import (
	"fmt"
	"os"
)

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
}

// disableSystemProxy removes proxy environment variables for Linux
func disableSystemProxy() {
	os.Unsetenv("ALL_PROXY")
	os.Unsetenv("HTTP_PROXY")
	os.Unsetenv("HTTPS_PROXY")
	os.Unsetenv("http_proxy")
	os.Unsetenv("https_proxy")
	infoLog("Proxy environment variables cleared")
}

// enableWindowsProxy is an alias for enableSystemProxy on Linux
func enableWindowsProxy(socksPort, httpPort int, exceptions string) error {
	enableSystemProxy(socksPort, httpPort)
	return nil
}

// disableWindowsProxy is an alias for disableSystemProxy on Linux
func disableWindowsProxy() error {
	disableSystemProxy()
	return nil
}

// addLoopbackRoute is a dummy implementation for Linux
func addLoopbackRoute() error {
	// On Linux, loopback routes are usually already configured
	return nil
}

// deleteLoopbackRoute is a dummy implementation for Linux
func deleteLoopbackRoute() error {
	return nil
}
