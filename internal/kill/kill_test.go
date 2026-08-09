package kill

import (
	"testing"
	"time"

	"github.com/jashk120/rambo/internal/config"
)

func TestCooldown(t *testing.T) {
	cfg := config.Default()
	cfg.Kill.Cooldown = "30s"
	cfg.Kill.MaxPerMin = 3
	c := NewCoordinator(cfg, 16<<30)

	if !c.CanKill() {
		t.Fatal("should be able to kill immediately")
	}
	c.RecordKill()
	if c.CooldownRemaining() <= 0 {
		t.Fatal("expected positive cooldown after a kill")
	}
	if c.CanKill() {
		t.Fatal("must not kill within cooldown window")
	}

	// simulate cooldown expiry
	c.lastKill = time.Now().Add(-31 * time.Second)
	if !c.CanKill() {
		t.Fatal("should be able to kill after cooldown")
	}
}

func TestRateLimitPerMinute(t *testing.T) {
	cfg := config.Default()
	cfg.Kill.Cooldown = "1s"
	cfg.Kill.MaxPerMin = 2
	c := NewCoordinator(cfg, 16<<30)

	for i := 0; i < 2; i++ {
		if !c.CanKill() {
			t.Fatalf("kill %d should be allowed", i+1)
		}
		c.RecordKill()
		c.lastKill = time.Now().Add(-2 * time.Second) // bypass cooldown, keep rate window
	}
	if c.CanKill() {
		t.Fatal("third kill within a minute must be blocked by rate limit")
	}
}

func TestExcluded(t *testing.T) {
	cfg := config.Default()
	cfg.Protect = []string{"nvim"}
	c := NewCoordinator(cfg, 16<<30)
	ex := c.excluded()
	if !ex["nvim"] || !ex["systemd"] || !ex["rambo"] {
		t.Fatalf("excluded set missing protected/blacklist entries: %v", ex)
	}
	if ex["firefox"] {
		t.Fatal("firefox should not be excluded")
	}
}
