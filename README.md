# Rambo

Rambo is an event-driven, kernel-backed system monitor for Linux. It watches RAM (via cgroup v2 limits), temperature (hwmon), pressure (PSI), network (netlink), CPU and disk, then acts on configurable policies: desktop notifications, graceful `SIGTERM` to a score-picked process, or suspending heavy jobs.

Detection (watchers) is separated from policy (a config-driven engine), so adding a sensor or changing behavior never requires touching daemon logic.

## Architecture

```
Watcher ── Event{Severity, Source} ──▶ Policy Engine ──▶ Action
                                      (config.toml)      notify / kill / suspend
```

All watchers are kernel-backed or kernel-accounted:

| Watcher | Kernel source | Triggers |
| --- | --- | --- |
| Memory | cgroup v2 `memory.high`/`memory.max` + `memory.events` | soft → warn, hard → kill, `max`/`oom_kill` → emergency |
| Temperature | hwmon `temp*_input` (CPU/GPU) | ≥ warning → warn, ≥ critical → kill |
| Pressure | `/proc/pressure/*` (PSI) | sustained `some`/`full` → warn / escalate |
| Network | netlink (`NETLINK_ROUTE`) | link down, throughput above alert |
| CPU | `/proc/stat` | busy% sustained above alert |
| Disk | `statfs` + `/proc/diskstats` | space%, pathological I/O |
| Battery | `power_supply` (laptops only) | low charge → suspend heavy jobs |

## Install

### From packages (recommended)

**Arch Linux (AUR)**
```sh
paru -S rambo
# or
yay -S rambo
# or manually
git clone https://aur.archlinux.org/rambo.git
cd rambo && makepkg -si
```

**Debian / Ubuntu (.deb)**
```sh
# Download from Releases: https://github.com/Jashk120/Rambo/releases
sudo apt install ./rambo_0.1.0_linux_amd64.deb
```

**Fedora / RHEL / openSUSE (.rpm)**
```sh
sudo dnf install ./rambo_0.1.0_linux_amd64.rpm
# or on openSUSE
sudo zypper install ./rambo_0.1.0_linux_amd64.rpm
```

**Alpine (.apk)**
```sh
apk add --allow-untrusted rambo_0.1.0_linux_amd64.apk
```

**Homebrew (Linux/macOS)**
```sh
brew tap Jashk120/rambo
brew install rambo
```

**Generic tarball**
```sh
tar xzf rambo_0.1.0_linux_amd64.tar.gz
sudo install -Dm755 rambo /usr/bin/rambo
sudo install -Dm644 polkit/99-rambo.rules /usr/share/polkit-1/rules.d/99-rambo.rules
sudo install -Dm644 systemd/rambo.service /usr/lib/systemd/user/rambo.service
```

### From source

```sh
go build -o rambo .
sudo make install
# installs: binary, polkit rule, systemd user service, man page, completions, license
```

Or run unprivileged without installing:
```sh
./rambo daemon
```

## Commands

| Command | Description |
| --- | --- |
| `rambo daemon` | Run the monitor (event loop) in the foreground |
| `rambo top` | One-shot view of system stats and top RAM consumers |
| `rambo stats` | Live dashboard (Ctrl+C to exit) |
| `rambo threshold set/status` | Set memory thresholds and alert levels |
| `rambo protect add/remove/list` | Protect processes from being killed |
| `rambo oom-protect` | Privileged helper (via pkexec): set `oom_score_adj=-1000` on protected processes |
| `rambo history` | Show past events and kills with reasons |
| `rambo clean` | Delete the log file |

## Configuration

Config lives at `~/.config/rambo/config.toml` (migrated automatically from the old `config.json` on first run). Defaults:

```toml
protect    = ["nvim", "gnome-shell"]   # never killed (internal blacklist applies too)
expendable = ["steam"]                 # killed first (score penalty)

[memory]
soft_pct = 90          # memory.high — kernel throttles here (soft)
hard_pct = 96          # graceful SIGTERM line
max_pct  = 99          # kernel safety-net limit

[memory_pressure]
some_pct = 60          # PSI some → warn when sustained
full_pct = 20          # PSI full  → escalate (kill) when sustained
window   = "10s"
action   = "escalate"

[temperature]
warning = 85
critical = 90          # kill top consumer above this
action  = "kill"
sensors = []           # empty = auto-detect CPU/GPU sensors

[network]
alert_mbps = 900
action     = "notify"

[cpu]
alert_pct = 90
action    = "notify"

[disk]
space_alert_pct = 90
io_alert        = true
action          = "notify"

[kill]
policy      = "score"      # "score" or "rss"
cooldown    = "30s"        # min interval between kills
max_per_min = 3            # rate limit
oom_prefer  = true         # steer the kernel OOM killer toward rambo's picks
oom_protect = true         # shield protected processes from the kernel OOM killer

[kill.weights]
rss     = 0.6              # kill score weights
cpu     = 0.3
runtime = 0.1

[battery]
low_pct = 20
action  = "suspend"        # SIGSTOP heavy jobs at low charge (laptops only)
```

### Thresholds — percent of total RAM, with GB overrides

```sh
rambo threshold set --soft-pct 90 --hard-pct 96 --max-pct 99
rambo threshold set --soft 14 --hard 14.5 --max 15        # GB equivalents
rambo threshold set --temp-kill 90 --net-alert 900 --cpu-alert 90 --disk-alert 90
rambo threshold status
```

Thresholds are relative to total RAM, so the same config works on 16/32/64 GB machines. The soft value must be below the hard value, which must be below the max.

## How RAM enforcement works

On start, rambo finds its session cgroup and applies the limits as kernel enforcement:

- `memory.high` = soft% — the kernel actively throttles the desktop at the soft line (prevents overshoot), and rambo notifies.
- `memory.max` = max% — kernel safety net. If rambo's graceful kill is missed, the kernel protects the session.
- At the hard line, rambo `SIGTERMs` the best kill candidate (see below).

Limits are applied only to `app.slice` and `session.slice` (the desktop slices the user owns). They are restored to `max` when the daemon stops. On non-systemd systems, or if the cgroups aren't writable, rambo degrades to `meminfo`-based monitoring.

> The cgroup files are in bytes. Rambo writes byte values and refuses to write any limit below 512 MiB / 1 GiB as a safety floor.

## Kill policy

The default `score` policy ranks candidates by:

```
score = 0.6·rss_norm + 0.3·cpu_norm + 0.1·runtime_norm
      + 0.3 if interactive (controlling TTY)
      + 0.2 if expendable
```

RSS and CPU are normalized against total RAM / 100%; younger processes outrank long-running ones. Protected processes, the internal blacklist (`systemd`, `kwin_wayland`, `plasmashell`, `Xorg`, `sddm`, `pipewire`, `wireplumber`, `rambo`, …) and rambo itself are never killed.

Cooldowns prevent thrash: `kill.cooldown` (default 30s) and `kill.max_per_min` (default 3) — a kill attempt inside the cooldown window notifies "kill throttled" instead of acting.

Set `policy = "rss"` for the old "biggest process" behavior.

### Steering the kernel OOM killer

The kernel picks OOM victims by `oom_score_adj`. Rambo cannot replace the kernel OOM killer, but with `kill.oom_prefer = true` it steers it:

- **Preferred victims**: each process rambo chooses to kill is marked `oom_score_adj = 1000` (the kernel's #1 choice) before the `SIGTERM`, and every process in the `expendable` list is kept marked — so if the kernel OOM-kills anyway, it takes rambo's pick first. Raising `oom_score_adj` needs no privileges.
- **Protecting processes from the kernel**: with `kill.oom_protect = true` (default), the daemon asks a root helper (`rambo oom-protect`, run through `pkexec`) to lower `oom_score_adj` to `-1000` on the internal blacklist, the `protect` list, and rambo itself — the kernel OOM killer then never chooses them. Lowering below 0 requires `CAP_SYS_RESOURCE`, which is why this runs as root. See [Hardening](#hardening) to install the polkit rule; without it the daemon simply logs a warning and keeps running unprivileged.

Caveat: once a process is marked `1000` it stays marked for the process lifetime (only root, a restart, or the process exiting clears it) — which is why only expendable processes and active kill victims are ever marked. Likewise, a `-1000` protection persists until the process exits or root clears it.

## Hardening

The daemon itself stays unprivileged — only a narrow helper runs as root. With `kill.oom_protect = true` (default), the daemon invokes `pkexec rambo oom-protect` on start and every 5 minutes. That helper sets `oom_score_adj = -1000` (unkillable by the kernel OOM killer) on the internal blacklist, the `protect` list, and rambo itself. It only writes `oom_score_adj`; it never kills, so running it as root carries no kill blast radius.

To let the passwordless helper through, install once (as root):

```sh
sudo make install
```

This installs the binary to `/usr/bin/rambo`, the polkit rule to `/usr/share/polkit-1/rules.d/99-rambo.rules`, and the systemd user service to `/usr/lib/systemd/user/rambo.service`. On Arch, `makepkg -si` (or the AUR) does the same. The rule is machine-wide — every user on the system gets passwordless `rambo oom-protect` with no per-user step.

```sh
systemctl --user enable --now rambo.service
```

The rule matches only the exact `rambo oom-protect` invocation (`program` basename `rambo` + first argument `oom-protect`) and grants nothing else.

Verify protected processes are now -1000 (rambo itself may show up as the daemon):

```sh
cat /proc/$(pgrep -x kwin_wayland)/oom_score_adj   # → -1000
cat /proc/$(pgrep -x rambo)/oom_score_adj          # → -1000
```

Without the rule, the daemon logs `oom-protect: helper failed` and keeps running — kernel protection is simply off. Protection persists for each protected process's lifetime and is re-applied on the 5-minute cadence to catch newly started ones.

## Protect / expendable

```sh
rambo protect add --name nvim
rambo protect remove --name nvim
rambo protect list
```

The old `rambo whitelist` command still works as an alias.

## History

Events and kills are recorded to `~/.local/state/rambo/state.json` (last 500):

```sh
rambo history
rambo history --source temperature --n 50
```

## Run with systemd

A user service template is included at `systemd/rambo.service`. It is installed automatically to `/usr/lib/systemd/user/rambo.service` by `sudo make install` (or the package); enable it with:

```sh
systemctl --user enable --now rambo.service
```

For a manual install without `make install`, copy it to your user unit dir:

```sh
mkdir -p ~/.config/systemd/user
cp systemd/rambo.service ~/.config/systemd/user/rambo.service
systemctl --user daemon-reload
systemctl --user enable --now rambo.service
```

The daemon prints a status line each second (visible with `journalctl --user -u rambo -f`) and restores cgroup limits on a clean stop.

## Logs

`~/.local/share/rambo/rambo.log` records kills, and desktop notifications fall back to it when no notification daemon is available. Delete it with `rambo clean`.

## Notes

- Linux-only; reads kernel data from cgroup v2, hwmon, `/proc`, and netlink.
- The hard action sends `SIGTERM`, not `SIGKILL`. The kernel safety net may `SIGKILL` if the max limit is truly reached.
- Battery protection is dormant on desktops (no `BAT*` in `power_supply`).
