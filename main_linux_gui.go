// Linux GUI platform implementation (non-headless)
// Build: CGO_ENABLED=1 go build -o vless_client .

//go:build linux && !headless
// +build linux,!headless

package main

import (
	"fmt"

	"fyne.io/fyne/v2"
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
func setupSystemTray(w fyne.Window, a fyne.App) {
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
	return nil
}

// enableSystemProxy is a dummy implementation for Linux
func enableSystemProxy(socksPort, httpPort int) {
	// Proxy configuration might be different on Linux
}

// disableSystemProxy is a dummy implementation for Linux
func disableSystemProxy() {
	// Proxy configuration might be different on Linux
}
