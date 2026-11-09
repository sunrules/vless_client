// Linux GUI platform implementation (non-headless)
// Build: CGO_ENABLED=1 go build -o vless_client .
//go:build linux && !headless
// +build linux,!headless

package main

import (
	"fmt"
	"fyne.io/fyne/v2"
	"os"
	"os/exec"
	"strings"
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

// setupSystemTray installs a native Fyne system tray on Linux GUI builds.
// Closing the window hides it (the app keeps running in memory); the tray lets
// the user restore the window ("Show VLESS Client") or exit ("Exit").
//
// SetSystemTrayMenu is not on the fyne.App interface - it lives on the private
// systrayDriver marker interface - so we type-assert against it. If the driver
// doesn't support a tray (some desktop environments), we log a warning instead
// of crashing. Left-click restores the window, right-click shows the menu.
func setupSystemTray(w interface{}, a interface{}) {
	tray, ok := a.(interface {
		SetSystemTrayMenu(*fyne.Menu)
	})
	if !ok {
		infoLog("Warning: app does not support native system tray menu")
		return
	}

	iconResource := loadTrayIcon()

	// Type-assert the app for its Quit() method (it's typed as interface{}).
	appQuit, okQuit := a.(interface{ Quit() })
	if !okQuit {
		infoLog("Warning: app does not support Quit()")
		return
	}

	mainMenu := fyne.NewMenu("VLESS Client",
		fyne.NewMenuItemWithIcon("Show VLESS Client", iconResource, func() {
			if win, ok2 := w.(interface {
				Show()
				RequestFocus()
			}); ok2 {
				fyne.Do(func() {
					win.Show()
					win.RequestFocus()
				})
			}
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Exit", func() {
			fyne.Do(appQuit.Quit)
		}),
	)

	tray.SetSystemTrayMenu(mainMenu)
}

// loadTrayIcon loads the application icon for the tray, or returns nil so Fyne
// falls back to its default icon.
func loadTrayIcon() fyne.Resource {
	iconData, err := os.ReadFile("vless.ico")
	if err != nil {
		infoLog("Warning: failed to load tray icon: %v", err)
		return nil
	}
	return fyne.NewStaticResource("vless.ico", iconData)
}

// gsettingsAvailable checks if gsettings command is available
func gsettingsAvailable() bool {
	_, err := exec.LookPath("gsettings")
	return err == nil
}

// setGsettings runs gsettings set with the given schema, key, and value
func setGsettings(schema, key, value string) error {
	out, err := exec.Command("gsettings", "set", schema, key, value).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gsettings set %s.%s: %s (%v)", schema, key, string(out), err)
	}
	return nil
}

// enableSystemProxy configures the GNOME system proxy via gsettings.
// This makes the proxy available to all GNOME applications (browser, etc.).
// The exceptions parameter contains comma-separated domains/IPs to bypass.
func enableSystemProxy(socksPort, httpPort int, exceptions string) {
	if !gsettingsAvailable() {
		infoLog("gsettings not available, falling back to environment variables")
		fallbackEnvProxy(socksPort, httpPort, exceptions)
		return
	}

	// Set proxy mode to manual
	if err := setGsettings("org.gnome.system.proxy", "mode", "'manual'"); err != nil {
		warnLog("Failed to set proxy mode: %v", err)
		fallbackEnvProxy(socksPort, httpPort, exceptions)
		return
	}

	// Set HTTP proxy (sub-schema: org.gnome.system.proxy.http)
	if httpPort > 0 {
		if err := setGsettings("org.gnome.system.proxy.http", "host", "'127.0.0.1'"); err != nil {
			warnLog("Failed to set HTTP proxy host: %v", err)
		}
		if err := setGsettings("org.gnome.system.proxy.http", "port", fmt.Sprintf("%d", httpPort)); err != nil {
			warnLog("Failed to set HTTP proxy port: %v", err)
		}
		if err := setGsettings("org.gnome.system.proxy.http", "enabled", "true"); err != nil {
			warnLog("Failed to enable HTTP proxy: %v", err)
		}
	}

	// Set HTTPS proxy (sub-schema: org.gnome.system.proxy.https)
	if httpPort > 0 {
		if err := setGsettings("org.gnome.system.proxy.https", "host", "'127.0.0.1'"); err != nil {
			warnLog("Failed to set HTTPS proxy host: %v", err)
		}
		if err := setGsettings("org.gnome.system.proxy.https", "port", fmt.Sprintf("%d", httpPort)); err != nil {
			warnLog("Failed to set HTTPS proxy port: %v", err)
		}
	}

	// Set SOCKS proxy (sub-schema: org.gnome.system.proxy.socks)
	if socksPort > 0 {
		if err := setGsettings("org.gnome.system.proxy.socks", "host", "'127.0.0.1'"); err != nil {
			warnLog("Failed to set SOCKS proxy host: %v", err)
		}
		if err := setGsettings("org.gnome.system.proxy.socks", "port", fmt.Sprintf("%d", socksPort)); err != nil {
			warnLog("Failed to set SOCKS proxy port: %v", err)
		}
	}

	// Build ignore-hosts list: default local networks + user exceptions
	ignoreHosts := []string{"localhost", "127.*", "192.168.*", "10.*", "172.16.*", "172.17.*"}
	for _, item := range strings.Split(exceptions, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			ignoreHosts = append(ignoreHosts, item)
		}
	}
	ignoreHostsArg := buildGVariantStringArray(ignoreHosts)
	if err := setGsettings("org.gnome.system.proxy", "ignore-hosts", ignoreHostsArg); err != nil {
		warnLog("Failed to set ignore-hosts: %v", err)
	}

	// Also set environment variables for terminal applications
	fallbackEnvProxy(socksPort, httpPort, exceptions)

	infoLog("System proxy enabled via gsettings: HTTP=127.0.0.1:%d SOCKS=127.0.0.1:%d", httpPort, socksPort)
}

// buildGVariantStringArray formats a Go string slice as a GVariant string array literal
func buildGVariantStringArray(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = "'" + item + "'"
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// disableSystemProxy removes the GNOME system proxy configuration
func disableSystemProxy() {
	if gsettingsAvailable() {
		if err := setGsettings("org.gnome.system.proxy", "mode", "'none'"); err != nil {
			warnLog("Failed to disable system proxy: %v", err)
		}
	}

	// Clear environment variables
	os.Unsetenv("ALL_PROXY")
	os.Unsetenv("all_proxy")
	os.Unsetenv("HTTP_PROXY")
	os.Unsetenv("HTTPS_PROXY")
	os.Unsetenv("http_proxy")
	os.Unsetenv("https_proxy")
	os.Unsetenv("NO_PROXY")
	os.Unsetenv("no_proxy")

	infoLog("System proxy disabled")
}

// fallbackEnvProxy sets proxy environment variables for the current process
func fallbackEnvProxy(socksPort, httpPort int, exceptions string) {
	if socksPort > 0 {
		proxy := fmt.Sprintf("socks5://127.0.0.1:%d", socksPort)
		os.Setenv("ALL_PROXY", proxy)
		os.Setenv("all_proxy", proxy)
	}
	if httpPort > 0 {
		proxy := fmt.Sprintf("http://127.0.0.1:%d", httpPort)
		os.Setenv("HTTP_PROXY", proxy)
		os.Setenv("HTTPS_PROXY", proxy)
		os.Setenv("http_proxy", proxy)
		os.Setenv("https_proxy", proxy)
	}
	noProxy := "localhost,127.*,192.168.*,10.*"
	if exceptions != "" {
		noProxy += "," + exceptions
	}
	os.Setenv("NO_PROXY", noProxy)
	os.Setenv("no_proxy", noProxy)
}

// enableWindowsProxy is the Linux implementation of system proxy
func enableWindowsProxy(socksPort, httpPort int, exceptions string) error {
	enableSystemProxy(socksPort, httpPort, exceptions)
	return nil
}

// disableWindowsProxy is the Linux implementation of system proxy disable
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
