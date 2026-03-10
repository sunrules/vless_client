// Windows-specific implementation for VLESS Client
// $env:CGO_ENABLED = "1"; $env:GOOS = "windows"; $env:GOARCH = "amd64"; go build -ldflags '-s -w' -o vless_client.exe -ldflags -H=windowsgui

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

// Window message for restoring from tray
const wmRestoreFromTray = 0x400 + 1 // WM_USER + 1

// HWND_BROADCAST is a special window handle for broadcasting messages
const HWND_BROADCAST = 0xffff

// FindWindow retrieves a window handle by class name and window name
var FindWindow = syscall.NewLazyDLL("user32.dll").NewProc("FindWindowW")

// SendMessage sends a message to a window or windows
var SendMessage = syscall.NewLazyDLL("user32.dll").NewProc("SendMessageW")

// RegisterClassEx registers a window class
var RegisterClassEx = syscall.NewLazyDLL("user32.dll").NewProc("RegisterClassExW")

// CreateWindowEx creates a window
var CreateWindowEx = syscall.NewLazyDLL("user32.dll").NewProc("CreateWindowExW")

// DestroyWindow destroys a window
var DestroyWindow = syscall.NewLazyDLL("user32.dll").NewProc("DestroyWindow")

// UnregisterClass unregisters a window class
var UnregisterClass = syscall.NewLazyDLL("user32.dll").NewProc("UnregisterClassW")

// GetModuleHandle retrieves a module handle for the specified module
var GetModuleHandle = syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW")

// DefWindowProc is the default window procedure
var DefWindowProc = syscall.NewLazyDLL("user32.dll").NewProc("DefWindowProcW")

// CreateMutex creates or opens a named or unnamed mutex object
var CreateMutex = syscall.NewLazyDLL("kernel32.dll").NewProc("CreateMutexW")

// CloseHandle closes an open object handle
var CloseHandle = syscall.NewLazyDLL("kernel32.dll").NewProc("CloseHandle")

// ERROR_ALREADY_EXISTS is returned if the mutex already exists
const ERROR_ALREADY_EXISTS = 183

// WNDPROC is the window procedure callback type
type WNDPROC func(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr

// WNDCLASSEX is the extended window class structure
type WNDCLASSEX struct {
	Size          uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     syscall.Handle
	HIcon         syscall.Handle
	HCursor       syscall.Handle
	HbrBackground syscall.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       syscall.Handle
}

// Window class for our hidden window
var windowClass = WNDCLASSEX{
	Size:          uint32(unsafe.Sizeof(WNDCLASSEX{})),
	Style:         0,
	LpfnWndProc:   syscall.NewCallback(windowProc),
	CbClsExtra:    0,
	CbWndExtra:    0,
	HInstance:     0, // Will be set later
	HIcon:         0,
	HCursor:       0,
	HbrBackground: 0,
	LpszMenuName:  nil,
	LpszClassName: syscall.StringToUTF16Ptr("VLESSClientHiddenWindowClass"),
	HIconSm:       0,
}

// Hidden window handle
var hiddenWindowHandle syscall.Handle

// Global window variable for access in windowProc
var w fyne.Window

// Window procedure for hidden window
func windowProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmRestoreFromTray:
		// Restore the main window from tray when message received
		if w != nil {
			w.Show()
			w.RequestFocus()
		}
	}

	// Call default window procedure
	ret, _, _ := DefWindowProc.Call(
		uintptr(hwnd),
		uintptr(msg),
		wParam,
		lParam,
	)
	return ret
}

// Create hidden window for message handling
func createHiddenWindow() error {
	// Get module handle
	hInstance, _, err := GetModuleHandle.Call(0)
	if err != syscall.Errno(0) {
		return fmt.Errorf("failed to get module handle: %v", err)
	}
	windowClass.HInstance = syscall.Handle(hInstance)

	// Register window class
	_, _, err = RegisterClassEx.Call(
		uintptr(unsafe.Pointer(&windowClass)),
	)
	if err != syscall.Errno(0) && err != syscall.Errno(1410) { // ERROR_CLASS_ALREADY_EXISTS = 1410
		return fmt.Errorf("failed to register window class: %v", err)
	}

	// Create hidden window
	hwnd, _, err := CreateWindowEx.Call(
		0, // dwExStyle
		uintptr(unsafe.Pointer(windowClass.LpszClassName)),                              // lpClassName
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("VLESS Client Hidden Window"))), // lpWindowName
		0,          // dwStyle (WS_OVERLAPPEDWINDOW)
		0, 0, 0, 0, // x, y, nWidth, nHeight
		0,         // hWndParent
		0,         // hMenu
		hInstance, // hInstance
		0,         // lpParam
	)
	if err != syscall.Errno(0) {
		UnregisterClass.Call(
			uintptr(unsafe.Pointer(windowClass.LpszClassName)),
			hInstance,
		)
		return fmt.Errorf("failed to create hidden window: %v", err)
	}

	hiddenWindowHandle = syscall.Handle(hwnd)
	return nil
}

// Destroy hidden window
func destroyHiddenWindow() {
	if hiddenWindowHandle != 0 {
		DestroyWindow.Call(uintptr(hiddenWindowHandle))
		UnregisterClass.Call(
			uintptr(unsafe.Pointer(windowClass.LpszClassName)),
			uintptr(windowClass.HInstance),
		)
		hiddenWindowHandle = 0
	}
}

// Broadcast a restore message to other instances
func broadcastRestoreMessage() error {
	// Find the hidden window of another instance
	hwnd, _, err := FindWindow.Call(
		uintptr(unsafe.Pointer(windowClass.LpszClassName)),
		0,
	)
	if err != syscall.Errno(0) || hwnd == 0 {
		return fmt.Errorf("no other instance window found: %v", err)
	}

	// Send restore message
	_, _, err = SendMessage.Call(
		hwnd,
		uintptr(wmRestoreFromTray),
		0,
		0,
	)
	if err != syscall.Errno(0) {
		return fmt.Errorf("failed to send restore message: %v", err)
	}

	return nil
}

// CheckSingleInstance checks if another instance is running and handles it
func CheckSingleInstance() bool {
	_, _, err := CreateMutex.Call(
		0, // lpMutexAttributes
		1, // bInitialOwner - Take ownership of the mutex
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(singleInstanceMutexName))),
	)
	if err != syscall.Errno(0) {
		if err == syscall.Errno(ERROR_ALREADY_EXISTS) {
			// Another instance is running, send restore message
			if err := broadcastRestoreMessage(); err == nil {
				os.Exit(0)
			}
		}
		fmt.Printf("Error checking for single instance: %v\n", err)
		os.Exit(1)
	}
	// Do NOT close the mutex handle - we need to keep it open to maintain the lock
	// The handle will be automatically closed when the process exits
	return true
}

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

// enableWindowsProxy enables system proxy in Windows registry
func enableWindowsProxy(socksPort, httpPort int) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer key.Close()

	if err := key.SetDWordValue("ProxyEnable", 1); err != nil {
		return err
	}

	httpProxy := `127.0.0.1:` + fmt.Sprintf("%d", httpPort)
	socksProxy := `127.0.0.1:` + fmt.Sprintf("%d", socksPort)
	proxyAddr := "http=" + httpProxy + ";https=" + httpProxy + ";socks=" + socksProxy

	if err := key.SetStringValue("ProxyServer", proxyAddr); err != nil {
		return err
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

	if err := enableWindowsProxy(socksPort, httpPort); err != nil {
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

func setupSystemTray(w fyne.Window, a fyne.App) {
	go func() {
		systray.Run(onTrayReady(w, a), onTrayExit)
	}()
}

func onTrayReady(w fyne.Window, a fyne.App) func() {
	return func() {
		iconData, err := os.ReadFile("vless.ico")
		if err != nil {
			infoLog("Warning: failed to load tray icon: %v", err)
		} else {
			systray.SetIcon(iconData)
		}

		systray.SetTitle("VLESS Client")
		systray.SetTooltip("VLESS Client")

		mOpen := systray.AddMenuItem("Open", "Show main window")
		mExit := systray.AddMenuItem("Exit", "Exit application")

		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					w.Show()
					w.RequestFocus()
				}
			}
		}()

		go func() {
			<-mExit.ClickedCh
			systray.Quit()
			a.Quit()
		}()
	}
}

func onTrayExit() {
	closeLogFiles()
	os.Exit(0)
}
