package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config holds all autopull configuration.
type Config struct {
	DefaultInterval string     `json:"defaultInterval,omitempty"`
	Repos           []RepoEntry `json:"repos"`
}

// RepoEntry represents a registered git repository.
type RepoEntry struct {
	Path     string   `json:"path"`
	Schedule Schedule `json:"schedule"`
	LastPull string   `json:"lastPull,omitempty"`
}

// Schedule defines when a repo should be pulled.
type Schedule struct {
	Interval string   `json:"interval,omitempty"`
	Times    []string `json:"times,omitempty"`
}

// ParseInterval parses a duration string like "15m", "2h", "30m" into a time.Duration.
func (s Schedule) ParseInterval() (time.Duration, error) {
	raw := s.Interval
	if raw == "" {
		raw = "30m"
	}
	return time.ParseDuration(raw)
}

// ShouldPull returns true if the repo should be pulled based on its schedule
// and the time since last pull.
func (r *RepoEntry) ShouldPull(now time.Time) bool {
	interval, err := r.Schedule.ParseInterval()
	if err != nil {
		interval = 30 * time.Minute
	}

	// Check interval-based schedule
	if r.LastPull != "" {
		last, err := time.Parse(time.RFC3339, r.LastPull)
		if err == nil && now.Sub(last) < interval {
			// Not enough time has passed; but check times-based schedule too
			return r.shouldPullAtTime(now)
		}
	}

	// No last pull or interval elapsed
	if r.LastPull == "" {
		// If times-based schedule, check if we're at a scheduled time
		if len(r.Schedule.Times) > 0 {
			return r.shouldPullAtTime(now)
		}
		return true
	}

	// Interval elapsed
	return true
}

// shouldPullAtTime checks if current time matches any scheduled time (within 1 minute window).
func (r *RepoEntry) shouldPullAtTime(now time.Time) bool {
	if len(r.Schedule.Times) == 0 {
		return false
	}
	nowStr := now.Format("15:04")
	for _, t := range r.Schedule.Times {
		if t == nowStr {
			return true
		}
	}
	return false
}

// ConfigPath returns the path to the config file.
func ConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "autopull", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	return filepath.Join(home, ".config", "autopull", "config.json")
}

// LoadConfig reads the config from disk, creating defaults if missing.
func LoadConfig() (*Config, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{DefaultInterval: "30m"}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// SaveConfig writes the config to disk.
func SaveConfig(cfg *Config) error {
	path := ConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}