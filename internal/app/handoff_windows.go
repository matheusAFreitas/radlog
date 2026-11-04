//go:build windows

package app

import (
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// a trava de uma cópia só da Wails (single_instance.go): "wails-app-" + id + "sim"
const instanceMutex = "wails-app-" + instanceID + "sim"

// startDetached abre o .exe sem ficar preso a esta cópia.
func startDetached(exe string, args []string) error {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP}
	return cmd.Start()
}

// waitPreviousExit espera a cópia antiga soltar a trava (até o limite).
func waitPreviousExit(limit time.Duration) bool {
	name, _ := windows.UTF16PtrFromString(instanceMutex)
	deadline := time.Now().Add(limit)
	for {
		h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, name)
		if err != nil {
			return true // não existe mais: a antiga fechou
		}
		windows.CloseHandle(h)
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(200 * time.Millisecond)
	}
}
