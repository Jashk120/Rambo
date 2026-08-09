package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type Memory struct {
	SoftPct float64 `toml:"soft_pct"`
	HardPct float64 `toml:"hard_pct"`
	MaxPct  float64 `toml:"max_pct"`
	Action  string  `toml:"action"`
}

type MemoryPressure struct {
	SomePct float64 `toml:"some_pct"`
	FullPct float64 `toml:"full_pct"`
	Window  string  `toml:"window"`
	Action  string  `toml:"action"`
}

type Temperature struct {
	Warning  float64  `toml:"warning"`
	Critical float64  `toml:"critical"`
	Action   string   `toml:"action"`
	Sensors  []string `toml:"sensors"`
}

type Network struct {
	AlertMBps float64 `toml:"alert_mbps"`
	Action    string  `toml:"action"`
}

type CPU struct {
	AlertPct float64 `toml:"alert_pct"`
	Action   string  `toml:"action"`
}

type Disk struct {
	SpaceAlertPct float64 `toml:"space_alert_pct"`
	IOAlert       bool    `toml:"io_alert"`
	Action        string  `toml:"action"`
}

type KillWeights struct {
	RSS     float64 `toml:"rss"`
	CPU     float64 `toml:"cpu"`
	Runtime float64 `toml:"runtime"`
}

type Kill struct {
	Policy    string      `toml:"policy"`
	Cooldown  string      `toml:"cooldown"`
	MaxPerMin int         `toml:"max_per_min"`
	Weights   KillWeights `toml:"weights"`
	OOMPrefer bool        `toml:"oom_prefer"`
}

type Battery struct {
	LowPct float64 `toml:"low_pct"`
	Action string  `toml:"action"`
}

type Config struct {
	Protect        []string       `toml:"protect"`
	Expendable     []string       `toml:"expendable"`
	Memory         Memory         `toml:"memory"`
	MemoryPressure MemoryPressure `toml:"memory_pressure"`
	Temperature    Temperature    `toml:"temperature"`
	Network        Network        `toml:"network"`
	CPU            CPU            `toml:"cpu"`
	Disk           Disk           `toml:"disk"`
	Kill           Kill           `toml:"kill"`
	Battery        Battery        `toml:"battery"`
}

func Default() Config {
	return Config{
		Memory: Memory{SoftPct: 90, HardPct: 96, MaxPct: 99, Action: "kill"},
		MemoryPressure: MemoryPressure{
			SomePct: 60, FullPct: 20, Window: "10s", Action: "escalate",
		},
		Temperature: Temperature{Warning: 85, Critical: 90, Action: "kill", Sensors: nil},
		Network:     Network{AlertMBps: 900, Action: "notify"},
		CPU:         CPU{AlertPct: 90, Action: "notify"},
		Disk:        Disk{SpaceAlertPct: 90, IOAlert: true, Action: "notify"},
		Kill: Kill{
			Policy: "score", Cooldown: "30s", MaxPerMin: 3,
			Weights:   KillWeights{RSS: 0.6, CPU: 0.3, Runtime: 0.1},
			OOMPrefer: true,
		},
		Battery: Battery{LowPct: 20, Action: "suspend"},
	}
}

func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "rambo", "config.toml")
}

func oldJSONPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "rambo", "config.json")
}

// Load reads the TOML config, migrating the old config.json on first run and
// writing defaults if nothing exists yet.
func Load() (Config, error) {
	cfg := Default()
	path := Path()
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return cfg, err
		}
		if migrated, merr := migrateFromJSON(); merr != nil || !migrated {
			if d2, e2 := os.ReadFile(path); e2 == nil {
				_ = toml.Unmarshal(d2, &cfg)
				return cfg, nil
			}
			_ = Save(path, cfg)
			return cfg, nil
		}
		if d2, e2 := os.ReadFile(path); e2 == nil {
			_ = toml.Unmarshal(d2, &cfg)
		}
		return cfg, nil
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

// TotalRAMKB returns total system RAM in KiB from the kernel.
func TotalRAMKB() uint64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			v, _ := strconv.ParseUint(fields[1], 10, 64)
			return v
		}
	}
	return 0
}

type legacyJSON struct {
	SoftGB       float64  `json:"soft_gb"`
	HardGB       float64  `json:"hard_gb"`
	NetAlertMbps float64  `json:"net_alert_mbps,omitempty"`
	CPUAlertPct  float64  `json:"cpu_alert_pct,omitempty"`
	DiskAlertPct float64  `json:"disk_alert_pct,omitempty"`
	Whitelist    []string `json:"whitelist,omitempty"`
}

// migrateFromJSON converts an old config.json to config.toml if present.
func migrateFromJSON() (bool, error) {
	old := oldJSONPath()
	data, err := os.ReadFile(old)
	if err != nil {
		return false, nil // no legacy config
	}
	var lj legacyJSON
	if err := json.Unmarshal(data, &lj); err != nil {
		return false, err
	}

	total := TotalRAMKB()
	totalGB := float64(total) / 1048576

	cfg := Default()
	if lj.SoftGB > 0 {
		cfg.Memory.SoftPct = lj.SoftGB / totalGB * 100
	}
	if lj.HardGB > 0 {
		cfg.Memory.HardPct = lj.HardGB / totalGB * 100
	}
	if lj.NetAlertMbps > 0 {
		cfg.Network.AlertMBps = lj.NetAlertMbps
	}
	if lj.CPUAlertPct > 0 {
		cfg.CPU.AlertPct = lj.CPUAlertPct
	}
	if lj.DiskAlertPct > 0 {
		cfg.Disk.SpaceAlertPct = lj.DiskAlertPct
	}
	if len(lj.Whitelist) > 0 {
		cfg.Protect = lj.Whitelist
	}

	if err := Save(Path(), cfg); err != nil {
		return false, err
	}
	fmt.Printf("migrated %s -> %s\n", old, Path())
	return true, nil
}
