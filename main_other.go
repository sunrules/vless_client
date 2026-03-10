// Universal implementation for other platforms (non-Windows, non-Linux)

// +build !windows,!linux

package main

import (
	"fmt"
	"fyne.io/fyne/v2"
)

// CheckSingleInstance is a dummy implementation for non-Windows platforms
func CheckSingleInstance() bool {
	// No single instance check on other platforms
	return true
}

// createHiddenWindow is a dummy implementation for non-Windows platforms
func createHiddenWindow() error {
	// No hidden window needed on other platforms
	return nil
}

// destroyHiddenWindow is a dummy implementation for non-Windows platforms
func destroyHiddenWindow() {
	// No hidden window to destroy on other platforms
}

// setupSystemTray is a dummy implementation for non-Windows platforms
func setupSystemTray(w fyne.Window, a fyne.App) {
	// System tray might not be supported on all platforms
}

// enableWindowsProxy is a dummy implementation for non-Windows platforms
func enableWindowsProxy(socksPort, httpPort int) error {
	return fmt.Errorf("Windows proxy is not supported on this platform")
}

// disableWindowsProxy is a dummy implementation for non-Windows platforms
func disableWindowsProxy() error {
	return fmt.Errorf("Windows proxy is not supported on this platform")
}

// addLoopbackRoute is a dummy implementation for non-Windows platforms
func addLoopbackRoute() error {
	return fmt.Errorf("Loopback route is not supported on this platform")
}

// deleteLoopbackRoute is a dummy implementation for non-Windows platforms
func deleteLoopbackRoute() error {
	return fmt.Errorf("Loopback route is not supported on this platform")
}

// enableSystemProxy is a dummy implementation for non-Windows platforms
func enableSystemProxy(socksPort, httpPort int) {
	// Proxy configuration might be different on other platforms
}

// disableSystemProxy is a dummy implementation for non-Windows platforms
func disableSystemProxy() {
	// Proxy configuration might be different on other platforms
}