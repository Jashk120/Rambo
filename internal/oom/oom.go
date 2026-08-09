package oom

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/jashk120/rambo/internal/score"
)

// Manager steers the kernel OOM killer toward rambo's list of preferred
// victims by raising their oom_score_adj to 1000, which makes them the OOM
// killer's first choice.
//
// Raising oom_score_adj is permitted for a same-uid process without extra
// capabilities. Lowering it (to protect a process) requires CAP_SYS_RESOURCE,
// so protecting processes from the kernel OOM killer is NOT possible from an
// unprivileged daemon — the protect list still fully protects from rambo's own
// SIGTERM kills.
type Manager struct {
	mu         sync.Mutex
	marked     map[int]int // pid -> original oom_score_adj
	expendable map[string]bool
}

func New(expendable []string) *Manager {
	m := make(map[string]bool, len(expendable))
	for _, s := range expendable {
		if s != "" {
			m[s] = true
		}
	}
	return &Manager{marked: map[int]int{}, expendable: m}
}

// MarkExpendable raises oom_score_adj to 1000 on every running process whose
// name is in the expendable list, so the kernel OOM killer prefers them.
// Returns the number of processes newly marked.
func (m *Manager) MarkExpendable() int {
	if len(m.expendable) == 0 {
		return 0
	}
	entries, err := filepath.Glob("/proc/[0-9]*/status")
	if err != nil {
		return 0
	}
	count := 0
	for _, path := range entries {
		pid, err := strconv.Atoi(filepath.Base(filepath.Dir(path)))
		if err != nil {
			continue
		}
		if !m.expendable[procName(path)] {
			continue
		}
		if m.raise(pid) {
			count++
		}
	}
	return count
}

// MarkVictim raises oom_score_adj to 1000 on a specific process that rambo's
// kill policy has already chosen to terminate. The victim is never protected
// or blacklisted by construction, so this only makes the kernel pick the same
// process rambo would have picked.
func (m *Manager) MarkVictim(c *score.Candidate) {
	if c == nil {
		return
	}
	m.raise(c.PID)
}

// Reset restores the originally recorded oom_score_adj values. This is
// best-effort: lowering back below 1000 needs CAP_SYS_RESOURCE, so it usually
// fails for exactly the same reason unprivileged protection does.
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for pid, orig := range m.marked {
		_ = os.WriteFile(fmt.Sprintf("/proc/%d/oom_score_adj", pid), []byte(strconv.Itoa(orig)), 0o644)
	}
	m.marked = map[int]int{}
}

func (m *Manager) raise(pid int) bool {
	path := fmt.Sprintf("/proc/%d/oom_score_adj", pid)
	cur, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	val := strings.TrimSpace(string(cur))
	if val == "1000" {
		return false // already preferred
	}
	if err := os.WriteFile(path, []byte("1000"), 0o644); err != nil {
		return false
	}
	orig, _ := strconv.Atoi(val)
	m.mu.Lock()
	m.marked[pid] = orig
	m.mu.Unlock()
	return true
}

func procName(statusPath string) string {
	b, err := os.ReadFile(statusPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "Name:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1]
			}
		}
	}
	return ""
}
