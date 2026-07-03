package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// LoadTasks reads and validates all task definitions (*.json) in dir,
// sorted by filename so runs are deterministic.
func LoadTasks(dir string) ([]Task, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read tasks dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var tasks []Task
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		var t Task
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		if err := t.Validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		tasks = append(tasks, t)
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no task files in %s", dir)
	}
	return tasks, nil
}
