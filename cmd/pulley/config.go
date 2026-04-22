package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config holds all pulley configuration.
type Config struct {
	DefaultInterval  string     `json:"defaultInterval,omitempty"`
	DefaultTimes     []string   `json:"defaultTimes,omitempty"`
	DefaultRange     string     `json:"defaultRange,omitempty"` // e.g. "09:00-17:00"
	DefaultBranches  []string   `json:"defaultBranches,omitempty"` // e.g. ["main", "dev"] or ["all"]
	Repos            []RepoEntry `json:"repos"`
}

// EffectiveInterval returns the interval for a repo, falling back to the config default then "30m".
func (c *Config) EffectiveInterval(s Schedule) string {
	if s.Interval != "" {
		return s.Interval
	}
	if c.DefaultInterval != "" {
		return c.DefaultInterval
	}
	return "30m"
}

// EffectiveTimes returns the times for a repo, falling back to the config default.
func (c *Config) EffectiveTimes(s Schedule) []string {
	if len(s.Times) > 0 {
		return s.Times
	}
	if len(c.DefaultTimes) > 0 {
		return c.DefaultTimes
	}
	return nil
}

// EffectiveBranches returns the branches for a repo, falling back to the config default.
func (c *Config) EffectiveBranches(s Schedule) []string {
	if len(s.Branches) > 0 {
		return s.Branches
	}
	if len(c.DefaultBranches) > 0 {
		return c.DefaultBranches
	}
	return nil // nil = current branch only
}

// EffectiveRange returns the time range for a repo, falling back to the config default.
func (c *Config) EffectiveRange(s Schedule) string {
	if s.Range != "" {
		return s.Range
	}
	return c.DefaultRange
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
	Range    string   `json:"range,omitempty"`    // e.g. "09:00-17:00"
	Branches []string `json:"branches,omitempty"` // e.g. ["main", "dev"] or ["all"]
}

// ParseInterval parses a duration string like "15m", "2h", "30m" into a time.Duration.
func (s Schedule) ParseInterval() (time.Duration, error) {
	raw := s.Interval
	if raw == "" {
		raw = "30m"
	}
	return time.ParseDuration(raw)
}

// ShouldPull returns true if the repo should be pulled based on its schedule,
// the time since last pull, and any time range constraints.
func (r *RepoEntry) ShouldPull(now time.Time, cfg *Config) bool {
	intervalStr := cfg.EffectiveInterval(r.Schedule)
	times := cfg.EffectiveTimes(r.Schedule)
	activeRange := cfg.EffectiveRange(r.Schedule)

	// If a time range is set, only pull within that range
	if activeRange != "" {
		if !isWithinRange(now, activeRange) {
			return false
		}
	}

	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		interval = 30 * time.Minute
	}

	// Check interval-based schedule
	if r.LastPull != "" {
		last, err := time.Parse(time.RFC3339, r.LastPull)
		if err == nil && now.Sub(last) < interval {
			// Not enough time has passed; but check times-based schedule too
			return matchesTimes(now, times)
		}
	}

	// No last pull or interval elapsed
	if r.LastPull == "" {
		// If times-based schedule, check if we're at a scheduled time
		if len(times) > 0 {
			return matchesTimes(now, times)
		}
		return true
	}

	// Interval elapsed
	return true
}

// matchesTimes checks if current time matches any scheduled time (within 1 minute window).
func matchesTimes(now time.Time, times []string) bool {
	if len(times) == 0 {
		return false
	}
	nowStr := now.Format("15:04")
	for _, t := range times {
		if t == nowStr {
			return true
		}
	}
	return false
}

// isWithinRange checks if the current time is within a time range like "09:00-17:00".
// Supports overnight ranges like "18:00-06:00" (crosses midnight).
func isWithinRange(now time.Time, rng string) bool {
	parts := splitRange(rng)
	if len(parts) != 2 {
		return true // invalid range = no restriction
	}
	start, errStart := parseTime(parts[0])
	end, errEnd := parseTime(parts[1])
	if errStart != nil || errEnd != nil {
		return true // invalid = no restriction
	}
	nowMins := now.Hour()*60 + now.Minute()
	if start <= end {
		// Normal range: 09:00-17:00
		return nowMins >= start && nowMins <= end
	}
	// Overnight range: 18:00-06:00 (start > end means crosses midnight)
	return nowMins >= start || nowMins <= end
}

// parseTime parses "HH:MM" into minutes since midnight.
func parseTime(s string) (int, error) {
	s = strings.TrimSpace(s)
	var h, m int
	n, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil || n != 2 {
		return 0, fmt.Errorf("invalid time: %s", s)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid time: %s", s)
	}
	return h*60 + m, nil
}

// splitRange splits a range string like "09:00-17:00" into ["09:00", "17:00"].
func splitRange(rng string) []string {
	for i, c := range rng {
		if c == '-' {
			return []string{rng[:i], rng[i+1:]}
		}
	}
	return []string{rng}
}


// ConfigPath returns the path to the config file.
func ConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "pulley", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	return filepath.Join(home, ".config", "pulley", "config.json")
}

// LoadConfig reads the config from disk, creating defaults if missing.
func LoadConfig() (*Config, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
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