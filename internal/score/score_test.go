package score

import (
	"testing"

	"github.com/jashk120/rambo/internal/config"
)

func TestCompute(t *testing.T) {
	w := config.KillWeights{RSS: 0.6, CPU: 0.3, Runtime: 0.1}
	const totalRAM = 16 * 1024 * 1024 * 1024 // 16GiB

	bigOld := Candidate{Name: "chrome", RSS: 8 << 30, CPU: 95, ElapsedSec: 4 * 3600}
	leaky := Candidate{Name: "renderer", RSS: 4 << 30, CPU: 5, ElapsedSec: 3600}

	if score := Compute(bigOld, w, totalRAM, false); score <= 0 {
		t.Fatalf("expected positive score, got %f", score)
	}
	if Compute(bigOld, w, totalRAM, false) <= Compute(leaky, w, totalRAM, false) {
		t.Fatalf("CPU-heavy process should outscore idle one at equal weights")
	}

	// rss-only weights must rank bigOld above a CPU-heavy smaller one
	rssW := config.KillWeights{RSS: 1, CPU: 0, Runtime: 0}
	if Compute(leaky, rssW, totalRAM, false) > Compute(bigOld, rssW, totalRAM, false) {
		t.Fatalf("rss policy mis-ranked")
	}

	// interactive penalty
	inter := bigOld
	inter.Interactive = true
	if Compute(inter, w, totalRAM, false) <= Compute(bigOld, w, totalRAM, false) {
		t.Fatalf("interactive penalty missing")
	}

	// expendable bonus
	if Compute(bigOld, w, totalRAM, true) <= Compute(bigOld, w, totalRAM, false) {
		t.Fatalf("expendable bonus missing")
	}

	// younger process outranks old identical one
	young := bigOld
	young.ElapsedSec = 60
	if Compute(young, w, totalRAM, false) <= Compute(bigOld, w, totalRAM, false) {
		t.Fatalf("runtime factor mis-ranked")
	}
}

func TestParseStat(t *testing.T) {
	// pid(comm) state ppid pgrp sess tty ... utime stime ... starttime
	s := parseStat("1234 (my app) S 1 2 3 34816 0 0 0 0 0 0 100 200 0 0 0 0 0 0 1000")
	if s == nil {
		t.Fatal("parseStat returned nil")
	}
	if s.pid != 1234 || s.name != "my app" {
		t.Fatalf("got pid=%d name=%q", s.pid, s.name)
	}
	if s.tty != 34816 {
		t.Fatalf("tty=%d want 34816", s.tty)
	}
	if s.utime != 100 || s.stime != 200 {
		t.Fatalf("utime=%d stime=%d", s.utime, s.stime)
	}
	if s.start != 1000 {
		t.Fatalf("start=%d want 1000", s.start)
	}
}
