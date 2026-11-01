//go:build windows

package main

import (
	"strings"
	"syscall"
	"unsafe"
)

// findRunningGame parcourt la table des processus via l'API Win32 CreateToolhelp32Snapshot
func findRunningGame(watchList []GameProcessDef) (*GameProcessDef, string) {
	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, ""
	}
	defer syscall.CloseHandle(snapshot)

	var entry syscall.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := syscall.Process32First(snapshot, &entry); err != nil {
		return nil, ""
	}

	for {
		exe := strings.ToLower(syscall.UTF16ToString(entry.ExeFile[:]))
		for i := range watchList {
			for _, target := range watchList[i].Executables {
				if exe == target {
					return &watchList[i], exe
				}
			}
		}
		if err := syscall.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}

	return nil, ""
}
