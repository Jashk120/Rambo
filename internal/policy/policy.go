package policy

import (
	"github.com/jashk120/rambo/internal/config"
	"github.com/jashk120/rambo/internal/watcher"
)

type Action int

const (
	ActionNone Action = iota
	ActionNotify
	ActionKill
	ActionSuspend
	ActionResume
)

func (a Action) String() string {
	switch a {
	case ActionNotify:
		return "notify"
	case ActionKill:
		return "kill"
	case ActionSuspend:
		return "suspend"
	case ActionResume:
		return "resume"
	}
	return "none"
}

// Resolve maps a detected event to an action using the policy config.
// Detection (watchers) is separated from policy (this).
func Resolve(cfg config.Config, ev watcher.Event) Action {
	switch ev.Source {
	case "memory":
		switch ev.Severity {
		case watcher.Emergency, watcher.Critical:
			return ActionKill
		default:
			return ActionNotify
		}
	case "memory_pressure":
		if ev.Severity == watcher.Critical && cfg.MemoryPressure.Action == "escalate" {
			return ActionKill
		}
		return ActionNotify
	case "temperature":
		if ev.Severity == watcher.Critical && cfg.Temperature.Action == "kill" {
			return ActionKill
		}
		return ActionNotify
	case "battery":
		if cfg.Battery.Action == "suspend" {
			return ActionSuspend
		}
		return ActionNotify
	case "battery_resume":
		return ActionResume
	case "network", "cpu", "disk":
		return ActionNotify
	}
	return ActionNone
}
