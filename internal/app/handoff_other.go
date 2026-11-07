//go:build !windows

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func startDetached(exe string, args []string) error { return exec.Command(exe, args...).Start() }

func waitPreviousExit(limit time.Duration) bool { return true }

// otherInstanceRunning diz se outra cópia do app (1.x ou 2.x) está aberta:
// procura em /proc um processo com o binário do app (o AppImage roda o
// binário de dentro dele, com o mesmo nome).
func otherInstanceRunning() bool {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	self := os.Getpid()
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		exe, err := os.Readlink(filepath.Join("/proc", e.Name(), "exe"))
		if err != nil {
			continue
		}
		n := strings.ToLower(filepath.Base(strings.TrimSuffix(exe, " (deleted)")))
		if strings.HasPrefix(n, "gmc-monitor") || strings.HasPrefix(n, "radlog") {
			return true
		}
	}
	return false
}
