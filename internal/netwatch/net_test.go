package netwatch

import (
	"syscall"
	"testing"
	"time"

	"github.com/jashk120/rambo/internal/config"
)

// TestDump verifies the netlink socket opens and a link dump returns real
// interfaces. Requires a working NETLINK_ROUTE socket (normal Linux).
func TestDump(t *testing.T) {
	w := NewWatcher(config.Default())
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_ROUTE)
	if err != nil {
		t.Skipf("no netlink route socket: %v", err)
	}
	defer syscall.Close(fd)
	if err := syscall.SetNonblock(fd, true); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Bind(fd, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		t.Fatal(err)
	}
	w.dump(fd)

	deadline := time.Now().Add(2 * time.Second)
	buf := make([]byte, 16384)
	seen := false
	for time.Now().Before(deadline) {
		n, _, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		w.parse(buf[:n], nil)
		w.mu.Lock()
		if len(w.prev) > 0 {
			seen = true
		}
		nifaces := len(w.prev)
		w.mu.Unlock()
		if seen {
			t.Logf("parsed %d interfaces from netlink dump", nifaces)
			break
		}
	}
	if !seen {
		t.Fatal("no link statistics parsed from netlink dump")
	}
}
