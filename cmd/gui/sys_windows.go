//go:build windows

package main

import (
	"os/exec"
	"syscall"
	"unsafe"
)

var (
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutex = kernel32.NewProc("CreateMutexW")
	procCloseHandle = kernel32.NewProc("CloseHandle")
)

// hideWindow empêche l'apparition furtive d'une fenêtre console cmd.exe lors de l'exécution de sous-commandes
func hideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= 0x08000000 // CREATE_NO_WINDOW
}

// acquireSingleInstanceLock s'assure qu'une seule instance de CrazyPing s'exécute simultanément
func acquireSingleInstanceLock(name string) (uintptr, bool) {
	namePtr, err := syscall.UTF16PtrFromString("Local\\" + name)
	if err != nil {
		return 0, true
	}
	ret, _, errCall := procCreateMutex.Call(0, 1, uintptr(unsafe.Pointer(namePtr)))
	const ERROR_ALREADY_EXISTS = 183
	if errCall != nil && errCall.(syscall.Errno) == ERROR_ALREADY_EXISTS {
		return ret, false
	}
	return ret, true
}

func releaseSingleInstanceLock(handle uintptr) {
	if handle != 0 {
		procCloseHandle.Call(handle)
	}
}

// closeNativeWindow ferme proprement et sans délai la fenêtre Edge/Chrome de l'interface
func closeNativeWindow() {
	browserMu.Lock()
	if activeBrowserCmd != nil && activeBrowserCmd.Process != nil {
		_ = activeBrowserCmd.Process.Kill()
		activeBrowserCmd = nil
	}
	browserMu.Unlock()

	// Filet de sécurité : forcer la fermeture de toute fenêtre résiduelle portant le titre CrazyPing
	cmd := exec.Command("taskkill", "/F", "/FI", "WINDOWTITLE eq CrazyPing*")
	hideWindow(cmd)
	_ = cmd.Run()
}
