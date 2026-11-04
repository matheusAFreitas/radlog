//go:build !windows

package app

import (
	"os/exec"
	"time"
)

func startDetached(exe string, args []string) error { return exec.Command(exe, args...).Start() }

func waitPreviousExit(limit time.Duration) bool { return true }
