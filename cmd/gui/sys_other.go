//go:build !windows

package main

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}

func acquireSingleInstanceLock(name string) (uintptr, bool) {
	return 0, true
}

func releaseSingleInstanceLock(handle uintptr) {}

func closeNativeWindow() {
	browserMu.Lock()
	if activeBrowserCmd != nil && activeBrowserCmd.Process != nil {
		_ = activeBrowserCmd.Process.Kill()
		activeBrowserCmd = nil
	}
	browserMu.Unlock()
}
