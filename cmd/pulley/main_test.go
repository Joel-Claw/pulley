package main

import (
	"testing"
	"time"
)

func TestScheduleParseInterval(t *testing.T) {
	tests := []struct {
		interval string
		want     time.Duration
		wantErr  bool
	}{
		{"15m", 15 * time.Minute, false},
		{"2h", 2 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"", 30 * time.Minute, false}, // default
		{"abc", 0, true},
	}

	for _, tt := range tests {
		s := Schedule{Interval: tt.interval}
		got, err := s.ParseInterval()
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseInterval(%q) expected error, got none", tt.interval)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseInterval(%q) unexpected error: %v", tt.interval, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseInterval(%q) = %v, want %v", tt.interval, got, tt.want)
		}
	}
}

func TestShouldPullNoLastPull(t *testing.T) {
	r := &RepoEntry{
		Schedule: Schedule{Interval: "30m"},
	}
	// No last pull = should always pull
	if !r.ShouldPull(time.Now()) {
		t.Error("ShouldPull with no LastPull should return true")
	}
}

func TestShouldPullWithinInterval(t *testing.T) {
	now := time.Now()
	r := &RepoEntry{
		Schedule: Schedule{Interval: "30m"},
		LastPull: now.Add(-10 * time.Minute).Format(time.RFC3339),
	}
	if r.ShouldPull(now) {
		t.Error("ShouldPull within interval should return false")
	}
}

func TestShouldPullAfterInterval(t *testing.T) {
	now := time.Now()
	r := &RepoEntry{
		Schedule: Schedule{Interval: "30m"},
		LastPull: now.Add(-35 * time.Minute).Format(time.RFC3339),
	}
	if !r.ShouldPull(now) {
		t.Error("ShouldPull after interval should return true")
	}
}

func TestShouldPullAtTime(t *testing.T) {
	now := time.Now()
	r := &RepoEntry{
		Schedule: Schedule{
			Interval: "24h", // long interval so time-based check wins
			Times:    []string{now.Format("15:04")},
		},
		LastPull: now.Add(-1 * time.Hour).Format(time.RFC3339),
	}
	if !r.ShouldPull(now) {
		t.Error("ShouldPull at scheduled time should return true")
	}
}

func TestShouldPullNotAtTime(t *testing.T) {
	now := time.Now()
	wrongTime := now.Add(-2 * time.Hour).Format("15:04")
	r := &RepoEntry{
		Schedule: Schedule{
			Interval: "24h",
			Times:    []string{wrongTime},
		},
		LastPull: now.Add(-1 * time.Hour).Format(time.RFC3339),
	}
	if r.ShouldPull(now) {
		t.Error("ShouldPull not at scheduled time with recent last pull should return false")
	}
}

func TestConfigPath(t *testing.T) {
	path := ConfigPath()
	if path == "" {
		t.Error("ConfigPath should not be empty")
	}
}

func TestLoadSaveConfig(t *testing.T) {
	// Use a temp dir
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &Config{
		DefaultInterval: "15m",
		Repos: []RepoEntry{
			{
				Path:     "/tmp/test-repo",
				Schedule: Schedule{Interval: "10m"},
			},
		},
	}

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig error: %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if loaded.DefaultInterval != cfg.DefaultInterval {
		t.Errorf("DefaultInterval = %q, want %q", loaded.DefaultInterval, cfg.DefaultInterval)
	}
	if len(loaded.Repos) != 1 {
		t.Fatalf("len(Repos) = %d, want 1", len(loaded.Repos))
	}
	if loaded.Repos[0].Path != cfg.Repos[0].Path {
		t.Errorf("Repos[0].Path = %q, want %q", loaded.Repos[0].Path, cfg.Repos[0].Path)
	}
}

func TestIsGitRepo(t *testing.T) {
	// Current dir should be a git repo
	root, err := IsGitRepo(".")
	if err != nil {
		t.Fatalf("IsGitRepo('.') error: %v", err)
	}
	if root == "" {
		t.Error("IsGitRepo should return non-empty root")
	}

	// /tmp should not be a git repo
	_, err = IsGitRepo("/tmp")
	if err == nil {
		t.Error("IsGitRepo('/tmp') should return error")
	}
}

func TestSplitTimes(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"09:00,18:00", []string{"09:00", "18:00"}},
		{"09:00", []string{"09:00"}},
		{"09:00, 18:00, 22:00", []string{"09:00", "18:00", "22:00"}},
	}

	for _, tt := range tests {
		got := splitTimes(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitTimes(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitTimes(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}