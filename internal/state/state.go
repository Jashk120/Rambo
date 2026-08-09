package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const maxRecords = 500

type Record struct {
	Time     string             `json:"time"`
	Source   string             `json:"source"`
	Severity string             `json:"severity"`
	Action   string             `json:"action"`
	Message  string             `json:"message"`
	Process  string             `json:"process,omitempty"`
	PID      int                `json:"pid,omitempty"`
	Values   map[string]float64 `json:"values,omitempty"`
}

var mu sync.Mutex

func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "rambo", "state.json")
}

func Append(r Record) error {
	mu.Lock()
	defer mu.Unlock()

	recs, err := load()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	recs = append(recs, r)
	if len(recs) > maxRecords {
		recs = recs[len(recs)-maxRecords:]
	}

	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func load() ([]Record, error) {
	data, err := os.ReadFile(Path())
	if err != nil {
		return nil, err
	}
	var recs []Record
	if len(data) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(data, &recs); err != nil {
		return nil, err
	}
	return recs, nil
}

func History(source string, n int) ([]Record, error) {
	mu.Lock()
	defer mu.Unlock()

	recs, err := load()
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(recs))
	for _, r := range recs {
		if source == "" || r.Source == source {
			out = append(out, r)
		}
	}
	if n > 0 && len(out) > n {
		out = out[len(out)-n:]
	}
	return out, nil
}
