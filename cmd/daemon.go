package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jashk120/rambo/internal/battery"
	"github.com/jashk120/rambo/internal/config"
	"github.com/jashk120/rambo/internal/cpu"
	"github.com/jashk120/rambo/internal/disk"
	"github.com/jashk120/rambo/internal/kill"
	"github.com/jashk120/rambo/internal/memory"
	"github.com/jashk120/rambo/internal/monitor"
	"github.com/jashk120/rambo/internal/netwatch"
	"github.com/jashk120/rambo/internal/notify"
	"github.com/jashk120/rambo/internal/oom"
	"github.com/jashk120/rambo/internal/policy"
	"github.com/jashk120/rambo/internal/pressure"
	"github.com/jashk120/rambo/internal/state"
	"github.com/jashk120/rambo/internal/temp"
	"github.com/jashk120/rambo/internal/watcher"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the rambo event-driven monitor daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDaemon()
	},
}

func runDaemon() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	emit := make(chan watcher.Event, 128)
	var watchers []watcher.Watcher

	mw, err := memory.NewWatcher(cfg)
	if err != nil {
		fmt.Printf("[rambo] memory watcher disabled: %v\n", err)
	} else {
		watchers = append(watchers, mw)
	}

	tw := temp.NewWatcher(cfg)
	watchers = append(watchers, tw)
	pw := pressure.NewWatcher(cfg)
	watchers = append(watchers, pw)
	nw := netwatch.NewWatcher(cfg)
	watchers = append(watchers, nw)
	cw := cpu.NewWatcher(cfg)
	watchers = append(watchers, cw)
	dw := disk.NewWatcher(cfg)
	watchers = append(watchers, dw)
	bw := battery.NewWatcher(cfg, "/sys/class/power_supply")
	if bw.Enabled() {
		watchers = append(watchers, bw)
	}

	for _, w := range watchers {
		w := w
		go func() { _ = w.Run(ctx, emit) }()
	}

	totalKB := config.TotalRAMKB()
	coord := kill.NewCoordinator(cfg, totalKB*1024)

	oomMgr := oom.New(cfg.Expendable)
	if cfg.Kill.OOMPrefer {
		n := oomMgr.MarkExpendable()
		if n > 0 {
			fmt.Printf("[rambo] marked %d expendable processes as OOM-preferred\n", n)
		}
		oomTicker := time.NewTicker(time.Minute)
		defer oomTicker.Stop()
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-oomTicker.C:
					oomMgr.MarkExpendable()
				}
			}
		}()
	}

	if cfg.Kill.OOMProtect {
		if err := oom.ProtectSelf(); err != nil {
			fmt.Printf("[rambo] oom-protect: cannot self-protect (unprivileged): %v\n", err)
		} else {
			fmt.Println("[rambo] oom-protect: self oom_score_adj=-1000")
		}
		runOOMProtectHelper()
		protectTicker := time.NewTicker(5 * time.Minute)
		defer protectTicker.Stop()
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-protectTicker.C:
					runOOMProtectHelper()
				}
			}
		}()
	}

	fmt.Printf("[rambo] daemon started — soft %.0f%% (%.1fG) | hard %.0f%% (%.1fG) | max %.0f%% (%.1fG) | temp kill %.0fC\n",
		cfg.Memory.SoftPct, gbpct(totalKB, cfg.Memory.SoftPct),
		cfg.Memory.HardPct, gbpct(totalKB, cfg.Memory.HardPct),
		cfg.Memory.MaxPct, gbpct(totalKB, cfg.Memory.MaxPct),
		cfg.Temperature.Critical)
	if mw != nil && len(mw.LimitDirs()) > 0 {
		fmt.Printf("[rambo] kernel limits applied on: %s\n", strings.Join(mw.LimitDirs(), ", "))
	} else {
		fmt.Println("[rambo] warning: no writable session cgroup — kernel enforcement off, monitoring from meminfo")
	}
	if !bw.Enabled() {
		fmt.Println("[rambo] no battery found — battery protection off")
	}

	statusC := time.NewTicker(1 * time.Second)
	defer statusC.Stop()
	prevNet, _ := monitor.SnapshotNet()
	var prevCPU struct {
		Overall monitor.CPURaw
		PerCore []monitor.CPURaw
	}
	if o, c, err := monitor.ReadCPURaw(); err == nil {
		prevCPU.Overall = o
		prevCPU.PerCore = c
	}
	cpuPct := 0.0

	for {
		select {
		case ev := <-emit:
			handleEvent(cfg, coord, oomMgr, ev)
		case <-statusC.C:
			mem, _ := monitor.GetMemInfo()
			curO, curC, err := monitor.ReadCPURaw()
			if err == nil {
				cur := struct {
					Overall monitor.CPURaw
					PerCore []monitor.CPURaw
				}{curO, curC}
				cpuPct = monitor.CalcCPUStats(prevCPU, cur).Overall
				prevCPU = cur
			}
			netStats, newNet, _ := monitor.GetIfaceStats(prevNet, 1.0)
			prevNet = newNet
			netLine := "-"
			if len(netStats) > 0 {
				top := netStats[0]
				netLine = fmt.Sprintf("%s ↓%.1f ↑%.1fMB/s", top.Name, top.RxBytesPerSec/1e6, top.TxBytesPerSec/1e6)
			}
			tempLine := "-"
			if tmax, where := tw.MaxTemp(); tmax > 0 {
				tempLine = fmt.Sprintf("%s %.0fC", where, tmax)
			}
			psiLine := "-"
			if p, err := pressure.Read("memory"); err == nil {
				psiLine = fmt.Sprintf("some %.1f%% full %.1f%%", p.Some10, p.Full10)
			}
			fmt.Printf("[rambo] RAM %.1f/%.1fG (%.0f%%) | CPU %.1f%% | temp %s | net %s | psi %s\n",
				float64(mem.Used)/1048576, float64(mem.Total)/1048576,
				percent(mem.Used, mem.Total), cpuPct, tempLine, netLine, psiLine)
		case <-ctx.Done():
			if mw != nil {
				mw.Restore()
			}
			oomMgr.Reset()
			fmt.Println("[rambo] daemon stopped — cgroup limits restored")
			return nil
		}
	}
}

// runOOMProtectHelper invokes the privileged `rambo oom-protect` helper once,
// in the background, so the kernel OOM killer is kept away from protected
// processes. The helper only adjusts oom_score_adj and never kills. Any failure
// (no pkexec, no polkit rule, prompt declined) is logged and ignored — the
// daemon keeps running unprivileged and never blocks on it.
func runOOMProtectHelper() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Printf("[rambo] oom-protect: cannot locate rambo binary: %v\n", err)
		return
	}
	if _, err := exec.LookPath("pkexec"); err != nil {
		fmt.Println("[rambo] oom-protect: pkexec not found — kernel OOM protection off")
		return
	}
	cmd := exec.Command("pkexec", exe, "oom-protect")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Start(); err != nil {
		fmt.Printf("[rambo] oom-protect: pkexec failed to start: %v\n", err)
		return
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			fmt.Printf("[rambo] oom-protect: helper failed: %v\n%s", err, strings.TrimSpace(buf.String()))
		} else if buf.Len() > 0 {
			fmt.Printf("[rambo] %s\n", strings.TrimSpace(buf.String()))
		}
	}()
}

func handleEvent(cfg config.Config, coord *kill.Coordinator, oomMgr *oom.Manager, ev watcher.Event) {
	act := policy.Resolve(cfg, ev)
	rec := state.Record{
		Time:     ev.Time.Format("2006-01-02 15:04:05"),
		Source:   ev.Source,
		Severity: ev.Severity.String(),
		Action:   act.String(),
		Message:  ev.Message,
		Values:   ev.Values,
	}

	switch act {
	case policy.ActionNotify:
		notify.Send(title(ev), ev.Message)
	case policy.ActionKill:
		if !coord.CanKill() {
			msg := fmt.Sprintf("kill throttled (cooldown %s): %s", coord.CooldownRemaining().Round(time.Second), ev.Message)
			notify.Send("RAMBO: kill throttled", msg)
			rec.Action = "throttled"
			rec.Message = msg
			break
		}
		victim, err := coord.PickVictim()
		if err != nil {
			notify.Send(title(ev), "no killable process: "+err.Error())
			rec.Action = "kill_failed"
			rec.Message = "kill failed: " + err.Error()
			break
		}
		if err := coord.Kill(victim); err != nil {
			notify.Send(title(ev), fmt.Sprintf("kill of %s (PID %d) failed: %v", victim.Name, victim.PID, err))
			rec.Action = "kill_failed"
			rec.Process = victim.Name
			rec.PID = victim.PID
			rec.Message = "kill failed: " + err.Error()
			break
		}
		coord.RecordKill()
		if cfg.Kill.OOMPrefer {
			oomMgr.MarkVictim(victim)
		}
		msg := fmt.Sprintf("Killed %s (PID %d, %.0f%% CPU, %d MB RSS) — %s",
			victim.Name, victim.PID, victim.CPU, victim.RSS/1048576, ev.Message)
		notify.Send(title(ev), msg)
		logKill(victim.Name, victim.PID)
		rec.Process = victim.Name
		rec.PID = victim.PID
		rec.Message = ev.Message
	case policy.ActionSuspend:
		n := coord.SuspendHeavy(2)
		notify.Send("RAMBO: low battery", fmt.Sprintf("Suspended %d heavy processes", n))
		rec.Message = fmt.Sprintf("suspended %d processes", n)
	case policy.ActionResume:
		n := coord.ResumeSuspended()
		if n > 0 {
			notify.Send("RAMBO: charging", fmt.Sprintf("Resumed %d suspended processes", n))
		}
		rec.Message = fmt.Sprintf("resumed %d processes", n)
	case policy.ActionNone:
		return
	}
	_ = state.Append(rec)
}

func title(ev watcher.Event) string {
	return fmt.Sprintf("RAMBO: %s (%s)", strings.ToUpper(ev.Source), ev.Severity)
}

func gb(kb uint64) float64 {
	return float64(kb) / 1048576
}

func gbpct(kb uint64, pct float64) float64 {
	return float64(kb) * pct / 100 / 1048576
}

func percent(part, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

func logKill(name string, pid int) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".local", "share", "rambo", "rambo.log")
	os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] killed %s (PID %d)\n", time.Now().Format("2006-01-02 15:04:05"), name, pid)
}
