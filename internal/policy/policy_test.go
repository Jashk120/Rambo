package policy

import (
	"testing"

	"github.com/jashk120/rambo/internal/config"
	"github.com/jashk120/rambo/internal/watcher"
)

func TestResolve(t *testing.T) {
	cfg := config.Default()

	ev := watcher.NewEvent(watcher.Critical, "memory", "x", nil)
	if Resolve(cfg, ev) != ActionKill {
		t.Fatal("memory critical should kill")
	}
	ev = watcher.NewEvent(watcher.Warning, "memory", "x", nil)
	if Resolve(cfg, ev) != ActionNotify {
		t.Fatal("memory warning should notify")
	}
	ev = watcher.NewEvent(watcher.Emergency, "memory", "x", nil)
	if Resolve(cfg, ev) != ActionKill {
		t.Fatal("memory emergency should kill")
	}

	ev = watcher.NewEvent(watcher.Critical, "temperature", "x", nil)
	if Resolve(cfg, ev) != ActionKill {
		t.Fatal("temperature critical should kill by default")
	}
	cfg.Temperature.Action = "notify"
	if Resolve(cfg, ev) != ActionNotify {
		t.Fatal("temperature critical should notify when action=notify")
	}

	ev = watcher.NewEvent(watcher.Critical, "memory_pressure", "x", nil)
	if Resolve(cfg, ev) != ActionKill {
		t.Fatal("pressure critical with action=escalate should kill")
	}
	ev = watcher.NewEvent(watcher.Warning, "memory_pressure", "x", nil)
	if Resolve(cfg, ev) != ActionNotify {
		t.Fatal("pressure warning should notify")
	}

	ev = watcher.NewEvent(watcher.Critical, "battery", "x", nil)
	if Resolve(cfg, ev) != ActionSuspend {
		t.Fatal("battery critical should suspend")
	}
	ev = watcher.NewEvent(watcher.Info, "battery_resume", "x", nil)
	if Resolve(cfg, ev) != ActionResume {
		t.Fatal("battery_resume should resume")
	}
}

func TestSeverityString(t *testing.T) {
	if watcher.Emergency.String() != "emergency" {
		t.Fatal("severity string wrong")
	}
}
