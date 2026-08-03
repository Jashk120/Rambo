# Rambo

Rambo is a small userspace RAM monitor for Linux. It watches memory usage, sends a notification when RAM crosses a soft threshold, and sends `SIGTERM` to the largest killable process when RAM crosses a hard threshold.

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

## Set RAM Thresholds

Thresholds are configured in GB. The soft threshold is for notification, and the hard threshold is for auto-kill. The soft value must be lower than the hard value.

```sh
rambo threshold set --soft 7.5 --hard 7.9
```

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
  "hard_gb": 7.9
}
```

## Run

Show current RAM usage and the top memory-consuming processes:

```sh
rambo top
```

Start the monitor in the foreground:

```sh
rambo daemon
```

The daemon checks memory every 5 seconds. When the soft threshold is reached, it sends a desktop notification. When the hard threshold is reached, it tries to terminate the largest process that is not protected by Rambo's internal blacklist.

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

## Logs

If desktop notifications are unavailable, or when a process is killed, Rambo writes to:

```text
~/.local/share/rambo/rambo.log
```

View the log:

```sh
tail -f ~/.local/share/rambo/rambo.log
```

## Notes

- Rambo reads memory data from `/proc/meminfo` and process data from `/proc/<pid>/status`, so it is Linux-only.
- The hard threshold action sends `SIGTERM`, not `SIGKILL`.
- Protected process names include `systemd`, `init`, `kwin_wayland`, `plasmashell`, `Xorg`, `sddm`, `pipewire`, `wireplumber`, and `rambo`.
