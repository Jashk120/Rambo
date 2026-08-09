package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jashk120/rambo/internal/config"
	"github.com/jashk120/rambo/internal/oom"
	"github.com/spf13/cobra"
)

// oomProtectBlacklist is the internal never-kill list. The kernel OOM killer
// is shielded from these too, not just rambo's own SIGTERMs.
var oomProtectBlacklist = []string{
	"systemd",
	"init",
	"kwin_wayland",
	"plasmashell",
	"Xorg",
	"sddm",
	"pipewire",
	"wireplumber",
	"rambo", // itself and the daemon
}

var oomProtectCmd = &cobra.Command{
	Use:   "oom-protect",
	Short: "Set oom_score_adj=-1000 on protected processes (root/pkexec helper)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runOOMProtect()
	},
}

func runOOMProtect() error {
	targets := make(map[string]bool, len(oomProtectBlacklist))
	for _, n := range oomProtectBlacklist {
		targets[n] = true
	}
	cfg := loadHelperConfig()
	for _, n := range cfg.Protect {
		if n != "" {
			targets[n] = true
		}
	}

	names := make([]string, 0, len(targets))
	for n := range targets {
		names = append(names, n)
	}

	if err := oom.ProtectSelf(); err != nil {
		fmt.Printf("[rambo] oom-protect: cannot protect self: %v\n", err)
	} else {
		fmt.Println("[rambo] oom-protect: self oom_score_adj=-1000")
	}
	n := oom.ProtectNames(names)
	fmt.Printf("[rambo] oom-protect: set oom_score_adj=-1000 on %d protected process(es)\n", n)
	return nil
}

// loadHelperConfig reads the config of the user that invoked this helper. When
// running as root under pkexec, PKEXEC_UID names the real user, so their config
// is resolved from their home rather than /root's. Falls back to the current
// user's config, then defaults.
func loadHelperConfig() config.Config {
	if os.Geteuid() == 0 {
		if uid := os.Getenv("PKEXEC_UID"); uid != "" {
			if home := homeForUID(uid); home != "" {
				if cfg, err := config.LoadFrom(filepath.Join(home, ".config", "rambo", "config.toml")); err == nil {
					return cfg
				}
			}
		}
	}
	if cfg, err := config.Load(); err == nil {
		return cfg
	}
	return config.Default()
}

// homeForUID returns the home directory for a numeric UID from /etc/passwd,
// avoiding cgo. Empty when not found.
func homeForUID(uid string) string {
	b, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Split(line, ":")
		if len(f) >= 6 && f[2] == uid {
			return f[5]
		}
	}
	return ""
}

func init() {
	rootCmd.AddCommand(oomProtectCmd)
}
