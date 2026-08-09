package memory

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/jashk120/rambo/internal/config"
)

func TestApplyLimitsWritesBytes(t *testing.T) {
	dir := t.TempDir()
	w := &Watcher{
		cfg:    config.Default(),
		softKB: 14678295,
		maxKB:  15730545,
	}
	w.applyLimits(dir)

	got, err := os.ReadFile(filepath.Join(dir, "memory.high"))
	if err != nil {
		t.Fatal(err)
	}
	want := strconv.FormatUint(w.softKB*1024, 10)
	if string(got) != want {
		t.Fatalf("memory.high = %s, want %s (bytes)", got, want)
	}

	got, err = os.ReadFile(filepath.Join(dir, "memory.max"))
	if err != nil {
		t.Fatal(err)
	}
	want = strconv.FormatUint(w.maxKB*1024, 10)
	if string(got) != want {
		t.Fatalf("memory.max = %s, want %s (bytes)", got, want)
	}
}

func TestApplyLimitsSafetyFloor(t *testing.T) {
	dir := t.TempDir()
	w := &Watcher{
		cfg:    config.Default(),
		softKB: 1000, // would be ~1MB as bytes — must be refused
		maxKB:  1500, // ~1.5MB as bytes — must be refused
	}
	w.applyLimits(dir)

	// The safety floor must prevent any write.
	for _, name := range []string{"memory.high", "memory.max"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Fatalf("%s should not have been written", name)
		}
	}
}

func TestParseEvents(t *testing.T) {
	data := "low 0\nhigh 3\nmax 1\noom 0\noom_kill 2\n"
	ev := parseEvents(data)
	if ev["high"] != 3 || ev["max"] != 1 || ev["oom_kill"] != 2 {
		t.Fatalf("parseEvents = %v", ev)
	}
}
