package killer

import (
	"fmt"
	"os"
	"syscall"

	"github.com/jashk120/rambo/internal/monitor"
)

// processes that should never be killed
var blacklist = map[string]bool{
	"systemd":      true,
	"init":         true,
	"kwin_wayland": true,
	"plasmashell":  true,
	"Xorg":         true,
	"sddm":         true,
	"pipewire":     true,
	"wireplumber":  true,
	"rambo":        true,
}

var whitelist = map[string]bool{} // user-defined, loaded from config

// LoadWhitelist replaces the in-memory whitelist with the given process names.
// It is called from the daemon with the Whitelist field from config at kill time.
func LoadWhitelist(names []string) {
	whitelist = make(map[string]bool, len(names))
	for _, name := range names {
		if name != "" {
			whitelist[name] = true
		}
	}
}

func KillTopConsumer() (string, int, error) {
	procs, err := monitor.GetTopProcesses(20)
	if err != nil {
		return "", 0, err
	}

	for _, p := range procs {
		if blacklist[p.Name] || whitelist[p.Name] {
			continue
		}
		if p.PID == os.Getpid() {
			continue
		}

		proc, err := os.FindProcess(p.PID)
		if err != nil {
			continue
		}

		err = proc.Signal(syscall.SIGTERM)
		if err != nil {
			continue
		}

		fmt.Printf("[rambo] killed %s (PID %d, was using %d MB)\n", p.Name, p.PID, p.RSS/1024)
		return p.Name, p.PID, nil
	}

	return "", 0, fmt.Errorf("no killable process found")
}
