package kill

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/jashk120/rambo/internal/config"
	"github.com/jashk120/rambo/internal/score"
)

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

type Coordinator struct {
	cfg         config.Config
	protect     map[string]bool
	expendable  map[string]bool
	totalRAM    uint64
	cooldown    time.Duration
	lastKill    time.Time
	windowStart time.Time
	windowKills int
	suspended   []int
}

func NewCoordinator(cfg config.Config, totalRAM uint64) *Coordinator {
	cd, err := time.ParseDuration(cfg.Kill.Cooldown)
	if err != nil || cd <= 0 {
		cd = 30 * time.Second
	}
	return &Coordinator{
		cfg:        cfg,
		protect:    toSet(cfg.Protect),
		expendable: toSet(cfg.Expendable),
		totalRAM:   totalRAM,
		cooldown:   cd,
	}
}

func toSet(list []string) map[string]bool {
	m := make(map[string]bool, len(list))
	for _, s := range list {
		if s != "" {
			m[s] = true
		}
	}
	return m
}

func (c *Coordinator) excluded() map[string]bool {
	m := make(map[string]bool, len(blacklist)+len(c.protect))
	for k := range blacklist {
		m[k] = true
	}
	for k := range c.protect {
		m[k] = true
	}
	return m
}

func (c *Coordinator) CooldownRemaining() time.Duration {
	if c.lastKill.IsZero() {
		return 0
	}
	d := c.cooldown - time.Since(c.lastKill)
	if d < 0 {
		d = 0
	}
	return d
}

func (c *Coordinator) CanKill() bool {
	now := time.Now()
	if c.windowStart.IsZero() || now.Sub(c.windowStart) >= time.Minute {
		c.windowStart = now
		c.windowKills = 0
	}
	if c.cfg.Kill.MaxPerMin > 0 && c.windowKills >= c.cfg.Kill.MaxPerMin {
		return false
	}
	return c.CooldownRemaining() == 0
}

func (c *Coordinator) RecordKill() {
	c.windowKills++
	c.lastKill = time.Now()
}

func (c *Coordinator) weights() config.KillWeights {
	if c.cfg.Kill.Policy == "rss" {
		return config.KillWeights{RSS: 1, CPU: 0, Runtime: 0}
	}
	return c.cfg.Kill.Weights
}

func (c *Coordinator) PickVictim() (*score.Candidate, error) {
	cands, err := score.Collect(c.excluded())
	if err != nil {
		return nil, err
	}
	if len(cands) == 0 {
		return nil, fmt.Errorf("no killable process found")
	}
	ranked := score.Rank(cands, c.weights(), c.totalRAM, c.expendable)
	v := ranked[0]
	return &v, nil
}

func (c *Coordinator) Kill(v *score.Candidate) error {
	proc, err := os.FindProcess(v.PID)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}

func (c *Coordinator) SuspendHeavy(n int) int {
	cands, err := score.Collect(c.excluded())
	if err != nil {
		return 0
	}
	ranked := score.Rank(cands, c.weights(), c.totalRAM, c.expendable)
	count := 0
	for _, v := range ranked {
		if count >= n {
			break
		}
		proc, err := os.FindProcess(v.PID)
		if err != nil {
			continue
		}
		if proc.Signal(syscall.SIGSTOP) != nil {
			continue
		}
		c.suspended = append(c.suspended, v.PID)
		count++
	}
	return count
}

func (c *Coordinator) ResumeSuspended() int {
	n := 0
	for _, pid := range c.suspended {
		proc, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		if proc.Signal(syscall.SIGCONT) == nil {
			n++
		}
	}
	c.suspended = nil
	return n
}
