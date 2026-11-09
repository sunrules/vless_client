// Windows-specific implementation for VLESS Client
// $env:CGO_ENABLED = "1"; $env:GOOS = "windows"; $env:GOARCH = "amd64"; go build -ldflags '-s -w' -o vless_client.exe -ldflags -H=windowsgui
// go build -o vless_client.exe -ldflags "-s -w -H=windowsgui" 2>&1
// GO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \go build -p $(nproc) -o vless_client.exe -ldflags="-H windowsgui" .

package main

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/systray"
	"golang.org/x/sys/windows/registry"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

// Mutex name for single instance check
const singleInstanceMutexName = "VLESSClientSingleInstanceMutex"

// HWND_BROADCAST is a special window handle for broadcasting messages
const HWND_BROADCAST = 0xffff

// ERROR_ALREADY_EXISTS is returned if the mutex already exists
const ERROR_ALREADY_EXISTS = 183

// CreateMutex creates or opens a named or unnamed mutex object (kernel32.dll).
var CreateMutex = syscall.NewLazyDLL("kernel32.dll").NewProc("CreateMutexW")

// executeCommand runs a Windows command and returns output and error
func executeCommand(cmd string) (string, error) {
	// For route commands, run with elevated privileges
	if strings.HasPrefix(strings.ToLower(cmd), "route add") || strings.HasPrefix(strings.ToLower(cmd), "route delete") {
		args := strings.Split(cmd, " ")
		if len(args) < 2 {
			return "", fmt.Errorf("invalid route command")
		}

		// Run the command with elevated privileges using PowerShell
		psCmd := fmt.Sprintf("Start-Process -FilePath 'route.exe' -ArgumentList '%s' -Verb RunAs -WindowStyle Hidden", strings.Join(args[1:], " "))
		out, err := exec.Command("powershell.exe", "-Command", psCmd).CombinedOutput()
		if err != nil {
			return string(out), fmt.Errorf("command failed: %s", out)
		}
		return string(out), nil
	}

	// For other commands, run normally
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

// CheckSingleInstance keeps the current instance alive and restores another
// instance's window when a second copy is launched. On Windows this is done via
// a named mutex; no hidden window or message broadcast is required.
func CheckSingleInstance() bool {
	_, _, err := CreateMutex.Call(
		0, // lpMutexAttributes
		1, // bInitialOwner - Take ownership of the mutex
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(singleInstanceMutexName))),
	)
	if err != syscall.Errno(0) {
		if err == syscall.Errno(ERROR_ALREADY_EXISTS) {
			// Another instance is already running; nothing to do here.
			os.Exit(0)
		}
		fmt.Printf("Error checking for single instance: %v\n", err)
		os.Exit(1)
	}
	// Do NOT close the mutex handle - we need to keep it open to maintain the lock.
	// The handle is closed automatically when the process exits.
	return true
}

// Internet Explorer/WinINet constants for proxy notification
const (
	INTERNET_OPTION_REFRESH               = 37
	INTERNET_OPTION_SETTINGS_CHANGED      = 39
	INTERNET_OPTION_PER_CONNECTION_OPTION = 75
	WM_SETTINGCHANGE                      = 0x001A

	// INTERNET_PER_CONN_OPTION constants
	INTERNET_PER_CONN_FLAGS        = 1
	INTERNET_PER_CONN_PROXY_SERVER = 2
	INTERNET_PER_CONN_PROXY_BYPASS = 3

	// Proxy flags
	PROXY_TYPE_DIRECT = 0x00000001
	PROXY_TYPE_PROXY  = 0x00000002
)

// WinINet DLL procedure for settings refresh
var InternetSetOption = syscall.NewLazyDLL("wininet.dll").NewProc("InternetSetOptionW")

// PostMessage sends a message to a window without waiting for a response
var PostMessage = syscall.NewLazyDLL("user32.dll").NewProc("PostMessageW")

// SendNotifyMessage sends a message without waiting for a response (non-blocking broadcast)
var SendNotifyMessage = syscall.NewLazyDLL("user32.dll").NewProc("SendNotifyMessageW")

// INTERNET_PER_CONN_OPTION structure
type INTERNET_PER_CONN_OPTION struct {
	dwOption uint32
	Value    [8]byte // Union of DWORD, LPSTR, FILETIME
}

// INTERNET_PER_CONN_OPTION_LIST structure
type INTERNET_PER_CONN_OPTION_LIST struct {
	dwSize        uint32
	pszConnection *uint16
	dwOptionCount uint32
	dwOptionError uint32
	pOptions      uintptr
}

// notifyWindowsProxyChange sends system notification that proxy settings have been changed
// Required since Windows 11 - registry changes alone are no longer sufficient
// This function uses INTERNET_OPTION_PER_CONNECTION_OPTION to properly notify all components
// including UWP apps that use the Windows.Web.Http namespace
func notifyWindowsProxyChange(proxyServer string, proxyEnabled bool) error {
	// Method 1: Use INTERNET_OPTION_PER_CONNECTION_OPTION for proper UWP support
	if proxyServer != "" {
		proxyServerW := syscall.StringToUTF16Ptr(proxyServer)
		proxyBypassW := syscall.StringToUTF16Ptr("localhost;127.*;10.*;172.16.*;172.17.*;172.18.*;172.19.*;172.20.*;172.21.*;172.22.*;172.23.*;172.24.*;172.25.*;172.26.*;172.27.*;172.28.*;172.29.*;172.30.*;172.31.*;192.168.*")

		var flags uint32
		if proxyEnabled {
			flags = PROXY_TYPE_DIRECT | PROXY_TYPE_PROXY
		} else {
			flags = PROXY_TYPE_DIRECT
		}

		options := make([]INTERNET_PER_CONN_OPTION, 3)

		// Set flags
		options[0].dwOption = INTERNET_PER_CONN_FLAGS
		*(*uint32)(unsafe.Pointer(&options[0].Value[0])) = flags

		// Set proxy server
		options[1].dwOption = INTERNET_PER_CONN_PROXY_SERVER
		*(*uintptr)(unsafe.Pointer(&options[1].Value[0])) = uintptr(unsafe.Pointer(proxyServerW))

		// Set proxy bypass
		options[2].dwOption = INTERNET_PER_CONN_PROXY_BYPASS
		*(*uintptr)(unsafe.Pointer(&options[2].Value[0])) = uintptr(unsafe.Pointer(proxyBypassW))

		optionList := INTERNET_PER_CONN_OPTION_LIST{
			dwSize:        uint32(unsafe.Sizeof(INTERNET_PER_CONN_OPTION_LIST{})),
			pszConnection: nil, // nil for LAN
			dwOptionCount: 3,
			dwOptionError: 0,
			pOptions:      uintptr(unsafe.Pointer(&options[0])),
		}

		ret, _, err := InternetSetOption.Call(
			0,
			INTERNET_OPTION_PER_CONNECTION_OPTION,
			uintptr(unsafe.Pointer(&optionList)),
			uintptr(optionList.dwSize),
		)
		if ret == 0 {
			infoLog("INTERNET_OPTION_PER_CONNECTION_OPTION failed: %v", err)
		} else {
			infoLog("INTERNET_OPTION_PER_CONNECTION_OPTION succeeded")
		}
	}

	// Method 2: Also notify WinINet that settings have changed (legacy support)
	ret, _, err := InternetSetOption.Call(0, INTERNET_OPTION_SETTINGS_CHANGED, 0, 0)
	if ret == 0 {
		infoLog("WinINet settings changed notification: %v", err)
	}

	// Method 3: Refresh WinINet settings
	ret, _, err = InternetSetOption.Call(0, INTERNET_OPTION_REFRESH, 0, 0)
	if ret == 0 {
		infoLog("WinINet refresh notification: %v", err)
	}

	// Method 4: Use SendNotifyMessage instead of SendMessage for broadcast to avoid blocking
	// SendNotifyMessage returns immediately without waiting for all windows to process
	_, _, err = SendNotifyMessage.Call(
		uintptr(HWND_BROADCAST),
		uintptr(WM_SETTINGCHANGE),
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("InternetSettings"))),
	)
	if err != nil && err != syscall.Errno(0) {
		infoLog("Warning: SendNotifyMessage failed: %v", err)
	}

	infoLog("System proxy change notification sent")
	return nil
}

// enableWindowsProxy enables system proxy in Windows registry
func enableWindowsProxy(socksPort, httpPort int, exceptions string) error {
	socksProxy := "127.0.0.1:" + fmt.Sprintf("%d", socksPort)
	httpProxy := "127.0.0.1:" + fmt.Sprintf("%d", httpPort)
	return enableWindowsProxyAddr(socksProxy, httpProxy, exceptions)
}

// enableWindowsProxyAddr enables system proxy in Windows registry using provided addresses
func enableWindowsProxyAddr(socksProxy, httpProxy string, exceptions string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer key.Close()

	if err := key.SetDWordValue("ProxyEnable", 1); err != nil {
		return err
	}

	// Use http= prefix for all protocols - Windows sets proxy for all protocols when http= is used
	proxyAddr := "http=" + httpProxy + ";https=" + httpProxy + ";socks=" + socksProxy

	if err := key.SetStringValue("ProxyServer", proxyAddr); err != nil {
		return err
	}

	// Separate tunneling exceptions
	override := exceptions
	if override == "" {
		override = "<local>"
	} else {
		override = strings.ReplaceAll(override, ",", ";") + ";<local>"
	}
	if err := key.SetStringValue("ProxyOverride", override); err != nil {
		infoLog("Warning: failed to set ProxyOverride: %v", err)
	}

	// Windows 11 requires explicit notification about settings change
	// Use the proxy address string for INTERNET_OPTION_PER_CONNECTION_OPTION
	if err := notifyWindowsProxyChange(proxyAddr, true); err != nil {
		infoLog("Warning: failed to send proxy change notification: %v", err)
	}

	infoLog("Windows system proxy enabled: %s", proxyAddr)
	return nil
}

// disableWindowsProxy disables system proxy in Windows registry
func disableWindowsProxy() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer key.Close()

	if err := key.SetDWordValue("ProxyEnable", 0); err != nil {
		return err
	}

	if err := key.SetStringValue("ProxyServer", ""); err != nil {
		return err
	}

	// Windows 11 requires explicit notification about settings change
	if err := notifyWindowsProxyChange("", false); err != nil {
		infoLog("Warning: failed to send proxy change notification: %v", err)
	}

	infoLog("Windows system proxy disabled")
	return nil
}

// AddLoopbackRoute adds a static route for 127.0.0.1/32 through the loopback interface with high priority
func addLoopbackRoute() error {
	// Add static route for 127.0.0.1/32 via loopback interface (1) with metric 1 (high priority)
	cmd := "route add 127.0.0.1 mask 255.255.255.255 127.0.0.1 metric 1"
	err := executeCmd(cmd)
	if err != nil {
		infoLog("Route add failed (may already exist): %v", err)
	} else {
		infoLog("Static loopback route added successfully")
	}
	return nil
}

// DeleteLoopbackRoute deletes the static loopback route
func deleteLoopbackRoute() error {
	cmd := "route delete 127.0.0.1 mask 255.255.255.255 127.0.0.1"
	err := executeCmd(cmd)
	if err != nil {
		infoLog("Route delete failed (may not exist): %v", err)
	} else {
		infoLog("Static loopback route deleted")
	}
	return nil
}

// executeCmd executes a Windows command and logs the result
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

// enableSystemProxy sets proxy environment variables and Windows registry
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

	// Add static loopback route to bypass VPN
	if err := addLoopbackRoute(); err != nil {
		errorLog("Error adding loopback route: %v", err)
	}

	if err := enableWindowsProxy(socksPort, httpPort, ""); err != nil {
		errorLog("Error enabling Windows proxy: %v", err)
	}
}

// disableSystemProxy removes proxy environment variables and disables Windows proxy
func disableSystemProxy() {
	os.Unsetenv("ALL_PROXY")
	os.Unsetenv("HTTP_PROXY")
	os.Unsetenv("HTTPS_PROXY")
	os.Unsetenv("http_proxy")
	os.Unsetenv("https_proxy")
	infoLog("Proxy environment variables cleared")

	// Do NOT delete loopback route here - it should only be deleted when UAC button is pressed
	if err := disableWindowsProxy(); err != nil {
		errorLog("Error disabling Windows proxy: %v", err)
	}
}

// buildTrayMenu builds the native Fyne system tray menu. All menu actions run
// on the main thread via Fyne's runOnMain, so no cross-thread window calls are
// needed. This works reliably on Windows 10 and Windows 11.
func buildTrayMenu(w fyne.Window, a fyne.App) *fyne.Menu {
	// Load tray icon once; reuse it for every item.
	iconData, err := os.ReadFile("vless.ico")
	iconResource := fyne.NewStaticResource("vless.ico", iconData)
	if err != nil {
		infoLog("Warning: failed to load tray icon: %v", err)
	}

	exitItem := fyne.NewMenuItem("Exit", func() {
		a.Quit()
	})
	exitItem.IsQuit = true // tell Fyne this IS the quit item, so it won't add its own "Quit"/"Выйти"

	mainMenu := fyne.NewMenu("VLESS Client",
		fyne.NewMenuItemWithIcon("Show VLESS Client", iconResource, func() {
			w.Show()
			w.RequestFocus()
		}),
		fyne.NewMenuItemSeparator(),
		exitItem,
	)

	// Quit menu is added automatically by Fyne's tray driver.
	return mainMenu
}

// trayApp is a local interface mirroring the Windows-only TrayMenu method of
// fyne.App. The method exists on the concrete *fyneApp but not on the App
// interface, so we type-assert against it here (Windows only).
type trayApp interface {
	fyne.App
	SetSystemTrayMenu(*fyne.Menu)
}

func setupSystemTray(w fyne.Window, a fyne.App) {
	// Register the native Fyne system tray menu. Fyne marshals every menu click
	// onto the main thread through runOnMain, so w.Show()/a.Quit() are safe and
	// the menu stays responsive on Windows 10 and Windows 11.
	tray, ok := a.(trayApp)
	if !ok {
		infoLog("Warning: app does not support native system tray menu")
		return
	}
	tray.SetSystemTrayMenu(buildTrayMenu(w, a))

	// Left-click on the tray icon opens/restores the main window (standard tray
	// behavior), while the right-click shows the menu. fyne.Do() dispatches onto
	// the main thread so the window call is safe from the systray goroutine.
	systray.SetOnTapped(func() {
		fyne.Do(func() {
			w.Show()
			w.RequestFocus()
		})
	})
}
