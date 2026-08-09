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

func TestProtectSelfBestEffort(t *testing.T) {
	// Best-effort: succeeds when privileged (CAP_SYS_RESOURCE), fails quietly
	// otherwise. Must never panic.
	_ = ProtectSelf()
}

func TestProtectSelfReadable(t *testing.T) {
	if err := ProtectSelf(); err != nil {
		t.Skipf("unprivileged, cannot self-protect: %v", err)
	}
	adj, err := os.ReadFile("/proc/self/oom_score_adj")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(adj)) != "-1000" {
		t.Fatalf("oom_score_adj = %q, want -1000", adj)
	}
	// Restore so subsequent tests see a clean value (raising is always allowed).
	_ = os.WriteFile("/proc/self/oom_score_adj", []byte("0"), 0o644)
}

func TestProtectNamesEmpty(t *testing.T) {
	if n := ProtectNames(nil); n != 0 {
		t.Fatalf("expected 0 with no names, got %d", n)
	}
}

func TestProtectNamesNoMatch(t *testing.T) {
	if n := ProtectNames([]string{"definitely-not-a-real-process-xyz"}); n != 0 {
		t.Fatalf("expected 0 matches, got %d", n)
	}
}

func TestProtectNamesSelfBestEffort(t *testing.T) {
	// Uses the test binary's own process name; at most one match even when
	// privileged. Must never panic and must not count unprivileged misses.
	name := procName("/proc/self/status")
	if name == "" {
		t.Skip("cannot read own process name")
	}
	n := ProtectNames([]string{name, "definitely-not-a-real-process-xyz"})
	if n > 1 {
		t.Fatalf("expected at most 1 match, got %d", n)
	}
	if n == 0 {
		t.Skipf("unprivileged, cannot self-protect (name %q)", name)
	}
	adj, err := os.ReadFile("/proc/self/oom_score_adj")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(adj)) != "-1000" {
		t.Fatalf("oom_score_adj = %q, want -1000", adj)
	}
}
