package main

import (
	"testing"
	"time"
)

func TestScheduleParseInterval(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		hasErr   bool
	}{
		{"15m", 15 * time.Minute, false},
		{"2h", 2 * time.Hour, false},
		{"1h30m", 90 * time.Minute, false},
		{"", 30 * time.Minute, false}, // default
		{"abc", 0, true},
	}
	for _, tt := range tests {
		s := Schedule{Interval: tt.input}
		got, err := s.ParseInterval()
		if tt.hasErr && err == nil {
			t.Errorf("ParseInterval(%q): expected error, got none", tt.input)
		}
		if !tt.hasErr && err != nil {
			t.Errorf("ParseInterval(%q): unexpected error: %v", tt.input, err)
		}
		if !tt.hasErr && got != tt.expected {
			t.Errorf("ParseInterval(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestShouldPullNoLastPull(t *testing.T) {
	r := &RepoEntry{Schedule: Schedule{Interval: "10m"}}
	cfg := &Config{}
	now := time.Now()
	if !r.ShouldPull(now, cfg) {
		t.Error("ShouldPull with no lastPull should return true")
	}
}

func TestShouldPullWithinInterval(t *testing.T) {
	r := &RepoEntry{
		Schedule: Schedule{Interval: "10m"},
		LastPull: time.Now().Format(time.RFC3339),
	}
	cfg := &Config{}
	now := time.Now()
	if r.ShouldPull(now, cfg) {
		t.Error("ShouldPull within interval should return false")
	}
}

func TestShouldPullAfterInterval(t *testing.T) {
	r := &RepoEntry{
		Schedule: Schedule{Interval: "10m"},
		LastPull: time.Now().Add(-15 * time.Minute).Format(time.RFC3339),
	}
	cfg := &Config{}
	now := time.Now()
	if !r.ShouldPull(now, cfg) {
		t.Error("ShouldPull after interval should return true")
	}
}

func TestShouldPullAtTime(t *testing.T) {
	now := time.Now()
	r := &RepoEntry{
		Schedule: Schedule{
			Interval: "24h",
			Times:    []string{now.Format("15:04")},
		},
		LastPull: now.Add(-1 * time.Hour).Format(time.RFC3339),
	}
	cfg := &Config{}
	if !r.ShouldPull(now, cfg) {
		t.Error("ShouldPull at scheduled time should return true even within interval")
	}
}

func TestShouldPullNotAtTime(t *testing.T) {
	r := &RepoEntry{
		Schedule: Schedule{
			Interval: "24h",
			Times:    []string{"03:00"},
		},
		LastPull: time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
	}
	cfg := &Config{}
	now := time.Now()
	if r.ShouldPull(now, cfg) {
		t.Error("ShouldPull at non-scheduled time within interval should return false")
	}
}

func TestConfigDefaultInterval(t *testing.T) {
	r := &RepoEntry{
		Schedule: Schedule{},
		LastPull: time.Now().Add(-25 * time.Minute).Format(time.RFC3339),
	}
	cfg := &Config{DefaultInterval: "1h"}
	now := time.Now()
	if r.ShouldPull(now, cfg) {
		t.Error("ShouldPull with 1h default interval after 25m should return false")
	}
}

func TestConfigDefaultTimes(t *testing.T) {
	now := time.Now()
	r := &RepoEntry{
		Schedule: Schedule{Interval: "24h"},
		LastPull: now.Add(-2 * time.Hour).Format(time.RFC3339),
	}
	cfg := &Config{DefaultTimes: []string{now.Format("15:04")}}
	if !r.ShouldPull(now, cfg) {
		t.Error("ShouldPull with default times matching current time should return true")
	}
}

func TestEffectiveInterval(t *testing.T) {
	cfg := &Config{DefaultInterval: "1h"}

	s := Schedule{Interval: "10m"}
	if got := cfg.EffectiveInterval(s); got != "10m" {
		t.Errorf("EffectiveInterval with repo override = %q, want 10m", got)
	}

	s = Schedule{}
	if got := cfg.EffectiveInterval(s); got != "1h" {
		t.Errorf("EffectiveInterval with config default = %q, want 1h", got)
	}

	cfg2 := &Config{}
	if got := cfg2.EffectiveInterval(Schedule{}); got != "30m" {
		t.Errorf("EffectiveInterval with no defaults = %q, want 30m", got)
	}
}

func TestEffectiveTimes(t *testing.T) {
	cfg := &Config{DefaultTimes: []string{"09:00", "18:00"}}

	s := Schedule{Times: []string{"12:00"}}
	if got := cfg.EffectiveTimes(s); len(got) != 1 || got[0] != "12:00" {
		t.Errorf("EffectiveTimes with repo override = %v, want [12:00]", got)
	}

	s = Schedule{}
	if got := cfg.EffectiveTimes(s); len(got) != 2 {
		t.Errorf("EffectiveTimes with config default = %v, want [09:00 18:00]", got)
	}
}

func TestEffectiveRange(t *testing.T) {
	cfg := &Config{DefaultRange: "09:00-17:00"}

	s := Schedule{Range: "10:00-14:00"}
	if got := cfg.EffectiveRange(s); got != "10:00-14:00" {
		t.Errorf("EffectiveRange with repo override = %q, want 10:00-14:00", got)
	}

	s = Schedule{}
	if got := cfg.EffectiveRange(s); got != "09:00-17:00" {
		t.Errorf("EffectiveRange with config default = %q, want 09:00-17:00", got)
	}

	cfg2 := &Config{}
	if got := cfg2.EffectiveRange(Schedule{}); got != "" {
		t.Errorf("EffectiveRange with no defaults = %q, want empty", got)
	}
}

func TestSplitTimes(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"09:00,18:00", []string{"09:00", "18:00"}},
		{"09:00, 18:00", []string{"09:00", "18:00"}},
		{"09:00", []string{"09:00"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := splitTimes(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("splitTimes(%q) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i, v := range got {
			if v != tt.expected[i] {
				t.Errorf("splitTimes(%q)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
			}
		}
	}
}

func TestIsWithinRange(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	if !isWithinRange(t1, "09:00-17:00") {
		t.Error("10:00 should be within 09:00-17:00")
	}

	t2 := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	if isWithinRange(t2, "09:00-17:00") {
		t.Error("08:00 should NOT be within 09:00-17:00")
	}

	t3 := time.Date(2026, 1, 1, 17, 0, 0, 0, time.UTC)
	if !isWithinRange(t3, "09:00-17:00") {
		t.Error("17:00 should be within 09:00-17:00 (inclusive)")
	}

	if !isWithinRange(t2, "invalid") {
		t.Error("Invalid range should not restrict")
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasErr   bool
	}{
		{"09:00", 9*60 + 0, false},
		{"17:30", 17*60 + 30, false},
		{"25:00", 0, true},  // invalid hour
		{"09:70", 0, true},  // invalid minute
		{"abc", 0, true},
	}
	for _, tt := range tests {
		got, err := parseTime(tt.input)
		if tt.hasErr && err == nil {
			t.Errorf("parseTime(%q): expected error, got none", tt.input)
		}
		if !tt.hasErr && err != nil {
			t.Errorf("parseTime(%q): unexpected error: %v", tt.input, err)
		}
		if !tt.hasErr && got != tt.expected {
			t.Errorf("parseTime(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}
func TestEffectiveBranches(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		schedule  Schedule
		want      []string
	}{
		{
			name:     "repo branches override default",
			cfg:      Config{DefaultBranches: []string{"main"}},
			schedule: Schedule{Branches: []string{"dev", "staging"}},
			want:    []string{"dev", "staging"},
		},
		{
			name:     "falls back to default branches",
			cfg:      Config{DefaultBranches: []string{"main", "dev"}},
			schedule: Schedule{},
			want:    []string{"main", "dev"},
		},
		{
			name:     "no branches set returns nil",
			cfg:      Config{},
			schedule: Schedule{},
			want:    nil,
		},
		{
			name:     "all keyword passes through",
			cfg:      Config{},
			schedule: Schedule{Branches: []string{"all"}},
			want:    []string{"all"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.EffectiveBranches(tt.schedule)
			if len(got) != len(tt.want) {
				t.Errorf("EffectiveBranches() = %v, want %v", got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("EffectiveBranches()[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestSplitBranches(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"main,dev", []string{"main", "dev"}},
		{"all", []string{"all"}},
		{" main , dev ", []string{"main", "dev"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := splitBranches(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitBranches(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
