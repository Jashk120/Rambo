package netwatch

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/jashk120/rambo/internal/config"
	"github.com/jashk120/rambo/internal/watcher"
)

const (
	rtmNewLink = 16
	rtmDelLink = 17
	rtmGetLink = 18

	nlmFRequest = 0x01
	nlmFDump    = 0x300

	rtmgrpLink = 1

	iflaIfName  = 3
	iflaStats64 = 23

	ifFUp      = 0x1
	ifFRunning = 0x40
)

type iface struct {
	name string
	up   bool
	rx   uint64
	tx   uint64
}

type Watcher struct {
	cfg     config.Config
	mu      sync.Mutex
	prev    map[int]*iface
	rates   map[string][2]float64
	topRate float64
	topName string
	alerted bool
	seq     uint32
}

func NewWatcher(cfg config.Config) *Watcher {
	return &Watcher{cfg: cfg, prev: map[int]*iface{}, rates: map[string][2]float64{}}
}

func (w *Watcher) Name() string { return "network" }

func (w *Watcher) Snapshot() map[string]float64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return map[string]float64{"net_mbps": w.topRate / 1e6}
}

func (w *Watcher) Run(ctx context.Context, emit chan<- watcher.Event) error {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_ROUTE)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	if err := syscall.SetNonblock(fd, true); err != nil {
		return err
	}
	sa := &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK, Groups: rtmgrpLink}
	if err := syscall.Bind(fd, sa); err != nil {
		return err
	}

	w.dump(fd)
	dumpTicker := time.NewTicker(5 * time.Second)
	defer dumpTicker.Stop()

	buf := make([]byte, 16384)
	for {
		for {
			n, _, err := syscall.Recvfrom(fd, buf, 0)
			if err != nil {
				if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
					break
				}
				if err == syscall.EINTR {
					continue
				}
				return err
			}
			w.parse(buf[:n], emit)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-dumpTicker.C:
			w.dump(fd)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (w *Watcher) dump(fd int) {
	w.seq++
	payload := make([]byte, 16) // ifinfomsg, zeroed = all interfaces
	msg := nlmsg(w.seq, rtmGetLink, nlmFRequest|nlmFDump, payload)
	syscall.Sendto(fd, msg, 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK})
}

func (w *Watcher) parse(b []byte, emit chan<- watcher.Event) {
	msgs, err := syscall.ParseNetlinkMessage(b)
	if err != nil {
		return
	}
	cur := map[int]*iface{}
	for _, m := range msgs {
		switch m.Header.Type {
		case rtmNewLink, rtmDelLink:
			i, ok := parseLink(m)
			if !ok || i.name == "" || i.name == "lo" {
				continue
			}
			idx := int(binary.LittleEndian.Uint32(m.Data[4:8]))
			prev, hadPrev := w.prev[idx]
			cur[idx] = i

			if m.Header.Type == rtmDelLink {
				if hadPrev && prev.up {
					watcher.Emit(emit, watcher.NewEvent(watcher.Warning, "network",
						fmt.Sprintf("link %s went down", prev.name), nil))
				}
				continue
			}
			if hadPrev && prev.up && !i.up {
				watcher.Emit(emit, watcher.NewEvent(watcher.Warning, "network",
					fmt.Sprintf("link %s went down", i.name), nil))
			}
		}
	}
	if len(cur) == 0 {
		return
	}

	w.mu.Lock()
	w.rates = map[string][2]float64{}
	best := 0.0
	bestName := ""
	const elapsed = 5.0 // dump interval seconds
	for idx, i := range cur {
		if p, ok := w.prev[idx]; ok && i.rx >= p.rx && i.tx >= p.tx {
			rxR := float64(i.rx-p.rx) / elapsed
			txR := float64(i.tx-p.tx) / elapsed
			w.rates[i.name] = [2]float64{rxR, txR}
			if t := rxR + txR; t > best {
				best = t
				bestName = i.name
			}
		} else {
			w.rates[i.name] = [2]float64{0, 0}
		}
	}
	w.topRate = best
	w.topName = bestName
	w.mu.Unlock()

	for idx := range w.prev {
		if _, ok := cur[idx]; !ok {
			delete(w.prev, idx)
		}
	}
	for idx, i := range cur {
		w.prev[idx] = i
	}

	w.checkAlert(emit)
}

func (w *Watcher) checkAlert(emit chan<- watcher.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()
	thresh := w.cfg.Network.AlertMBps * 1e6
	if thresh <= 0 {
		return
	}
	if w.topRate > thresh {
		if !w.alerted {
			w.alerted = true
			watcher.Emit(emit, watcher.NewEvent(watcher.Warning, "network",
				fmt.Sprintf("%s at %.1f MB/s (rx+tx)", w.topName, w.topRate/1e6),
				map[string]float64{"net_mbps": w.topRate / 1e6}))
		}
	} else if w.topRate < thresh*0.8 {
		w.alerted = false
	}
}

func parseLink(m syscall.NetlinkMessage) (*iface, bool) {
	if len(m.Data) < 16 {
		return nil, false
	}
	flags := binary.LittleEndian.Uint32(m.Data[8:12])
	i := &iface{up: flags&ifFUp != 0 && flags&ifFRunning != 0}
	for _, a := range parseAttrs(m.Data[16:]) {
		switch a.Attr.Type {
		case iflaIfName:
			i.name = string(a.Value)
		case iflaStats64:
			if len(a.Value) >= 32 {
				i.rx = binary.LittleEndian.Uint64(a.Value[16:24])
				i.tx = binary.LittleEndian.Uint64(a.Value[24:32])
			}
		}
	}
	return i, true
}

func parseAttrs(b []byte) []syscall.NetlinkRouteAttr {
	var attrs []syscall.NetlinkRouteAttr
	for len(b) >= 4 {
		l := int(binary.LittleEndian.Uint16(b[0:2]))
		if l < 4 || l > len(b) {
			break
		}
		t := int(binary.LittleEndian.Uint16(b[2:4]))
		attrs = append(attrs, syscall.NetlinkRouteAttr{
			Attr:  syscall.RtAttr{Len: uint16(l), Type: uint16(t)},
			Value: b[4:l],
		})
		b = b[align(l):]
	}
	return attrs
}

func nlmsg(seq uint32, typ, flags uint16, data []byte) []byte {
	// no payload for this use; header + data
	l := 16 + len(data)
	b := make([]byte, l)
	binary.LittleEndian.PutUint32(b[0:4], uint32(l))
	binary.LittleEndian.PutUint16(b[4:6], typ)
	binary.LittleEndian.PutUint16(b[6:8], flags)
	binary.LittleEndian.PutUint32(b[8:12], seq)
	binary.LittleEndian.PutUint32(b[12:16], uint32(os.Getpid()))
	copy(b[16:], data)
	return b
}

func align(n int) int {
	return (n + 3) &^ 3
}
