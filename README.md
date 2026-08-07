# Rambo

Rambo is a small userspace system monitor for Linux. It watches memory usage, sends a desktop notification when RAM crosses a soft threshold, and sends `SIGTERM` to the largest killable process when RAM crosses a hard threshold. It also monitors network throughput, CPU usage, and disk space, with configurable alerts for each.

## Build

```sh
go build -o rambo .
```

Optional user install:

```sh
mkdir -p ~/.local/bin
cp ./rambo ~/.local/bin/rambo
```

Make sure `~/.local/bin` is in your `PATH` if you want to run `rambo` from anywhere.

## Commands

| Command | Description |
| --- | --- |
| `rambo top` | One-shot view of system stats and top RAM consumers |
| `rambo stats` | Live dashboard of system stats (Ctrl+C to exit) |
| `rambo daemon` | Run the background monitor in the foreground |
| `rambo threshold set` | Configure thresholds and alert levels |
| `rambo threshold status` | Show current thresholds |
| `rambo whitelist add` | Protect a process from auto-kill |
| `rambo whitelist remove` | Unprotect a process |
| `rambo whitelist list` | List whitelisted processes |
| `rambo clean` | Delete the rambo log file |

## Configure

Thresholds are configured in GB. The soft threshold is for notification, and the hard threshold is for auto-kill. The soft value must be lower than the hard value.

```sh
rambo threshold set --soft 7.5 --hard 7.9
```

Optional alert levels:

```sh
rambo threshold set --soft 7.5 --hard 7.9 --net-alert 900 --cpu-alert 90 --disk-alert 90
```

- `--net-alert` — notify if any network interface exceeds this combined rx+tx throughput (MB/s, 0 = off)
- `--cpu-alert` — notify if overall CPU usage exceeds this percent (0 = off)
- `--disk-alert` — notify if any mount exceeds this used percent (0 = off)

Check the current thresholds:

```sh
rambo threshold status
```

The config is written to:

```text
~/.config/rambo/config.json
```

Example config:

```json
{
  "soft_gb": 7.5,
  "hard_gb": 7.9,
  "net_alert_mbps": 900,
  "cpu_alert_pct": 90,
  "disk_alert_pct": 90,
  "whitelist": ["firefox"]
}
```

## Whitelist

By default Rambo will never kill processes on its internal blacklist (`systemd`, `init`, `kwin_wayland`, `plasmashell`, `Xorg`, `sddm`, `pipewire`, `wireplumber`, and `rambo`). You can protect additional processes by name:

```sh
rambo whitelist add --name firefox
rambo whitelist list
rambo whitelist remove --name firefox
```

## Run

Show current RAM usage and the top memory-consuming processes:

```sh
rambo top
```

Open a live, full-screen dashboard of RAM, CPU, network, and disk stats:

```sh
rambo stats
```

Start the monitor in the foreground:

```sh
rambo daemon
```

The daemon checks every 5 seconds. On each tick it:

- Notifies when RAM crosses the soft threshold
- Terminates the largest unprotected process when RAM crosses the hard threshold
- Notifies on high network throughput, CPU usage, or disk usage when configured

## Run With systemd

A user service template is included at `systemd/rambo.service`.

Install it:

```sh
mkdir -p ~/.config/systemd/user
cp systemd/rambo.service ~/.config/systemd/user/rambo.service
systemctl --user daemon-reload
systemctl --user enable --now rambo.service
```

Check service status:

```sh
systemctl --user status rambo.service
```

Stop the service:

```sh
systemctl --user stop rambo.service
```

The daemon also prints a periodic status line to stdout, which you can watch with `journalctl --user -u rambo -f`.

## Logs

When a process is killed, or when desktop notifications are unavailable, Rambo writes to:

```text
~/.local/share/rambo/rambo.log
```

View the log:

```sh
tail -f ~/.local/share/rambo/rambo.log
```

Delete the log:

```sh
rambo clean
```

Notifications are attempted via `notify-send`, then `zenity`, then `kdialog`, before falling back to the log file.

## Notes

- Rambo reads memory data from `/proc/meminfo`, process data from `/proc/<pid>/status`, and other stats from `/proc`, so it is Linux-only.
- The hard threshold action sends `SIGTERM`, not `SIGKILL`.
- Disk I/O is tracked for display only and never triggers a kill.
