package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SaveEpisode writes one episode artifact as pretty-printed JSON under
// runDir, creating the directory if needed. Returns the written path.
// Episode JSON files are the canonical eval record (ADR-0016).
func SaveEpisode(runDir string, ep *Episode) (string, error) {
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", runDir, err)
	}
	path := filepath.Join(runDir, ep.ID+".json")
	data, err := json.MarshalIndent(ep, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, data, 0o644)
}

// LoadEpisode reads one episode artifact back from disk.
func LoadEpisode(path string) (*Episode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ep Episode
	if err := json.Unmarshal(data, &ep); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &ep, nil
}
