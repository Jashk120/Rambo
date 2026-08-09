package watcher

import (
	"context"
	"time"
)

type Severity int

const (
	Info Severity = iota
	Warning
	Critical
	Emergency
)

func (s Severity) String() string {
	switch s {
	case Info:
		return "info"
	case Warning:
		return "warning"
	case Critical:
		return "critical"
	case Emergency:
		return "emergency"
	}
	return "unknown"
}

type Event struct {
	Severity Severity
	Source   string
	Message  string
	Values   map[string]float64
	Time     time.Time
}

func NewEvent(sev Severity, source, msg string, values map[string]float64) Event {
	return Event{Severity: sev, Source: source, Message: msg, Values: values, Time: time.Now()}
}

type Watcher interface {
	Name() string
	Run(ctx context.Context, emit chan<- Event) error
	Snapshot() map[string]float64
}

func Emit(emit chan<- Event, ev Event) {
	select {
	case emit <- ev:
	default:
	}
}
