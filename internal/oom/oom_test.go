package oom

import (
	"os"
	"strings"
	"testing"
)

func TestMarkExpendableEmpty(t *testing.T) {
	m := New(nil)
	if n := m.MarkExpendable(); n != 0 {
		t.Fatalf("expected 0 with no expendable list, got %d", n)
	}
}

func TestMarkExpendableOnlyMatchesList(t *testing.T) {
	m := New([]string{"definitely-not-a-real-process-xyz"})
	if n := m.MarkExpendable(); n != 0 {
		t.Fatalf("expected 0 matches, got %d", n)
	}
}

func TestRaiseSelf(t *testing.T) {
	// Raising our own oom_score_adj is always allowed.
	pid := os.Getpid()
	m := New(nil)
	if !m.raise(pid) {
		t.Fatal("expected to raise own oom_score_adj")
	}
	adj, err := os.ReadFile("/proc/self/oom_score_adj")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(adj)) != "1000" {
		t.Fatalf("oom_score_adj = %q, want 1000", adj)
	}
	// A second raise must be a no-op (already at 1000).
	m2 := New(nil)
	if m2.raise(pid) {
		t.Fatal("expected second raise to be a no-op")
	}
}

func TestRaiseNonexistentPID(t *testing.T) {
	m := New(nil)
	if m.raise(999999999) {
		t.Fatal("expected raise to fail for nonexistent pid")
	}
}

func TestResetNeverPanics(t *testing.T) {
	m := New(nil)
	m.raise(os.Getpid())
	m.Reset() // best-effort; unprivileged lowering usually fails, must not panic
}
