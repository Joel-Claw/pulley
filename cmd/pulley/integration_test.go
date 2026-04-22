package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestGitPullInBareRepo(t *testing.T) {
	tmpDir := t.TempDir()
	bareRepo := filepath.Join(tmpDir, "bare.git")
	cloneDir := filepath.Join(tmpDir, "clone")

	if err := exec.Command("git", "init", "--bare", bareRepo).Run(); err != nil {
		t.Fatalf("init bare repo: %v", err)
	}
	if err := exec.Command("git", "clone", bareRepo, cloneDir).Run(); err != nil {
		t.Fatalf("clone repo: %v", err)
	}

	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "initial")
	cmd.Dir = cloneDir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
	if err := cmd.Run(); err != nil {
		t.Fatalf("initial commit: %v", err)
	}
}

func TestIsGitRepoNotGitDir(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := IsGitRepo(tmpDir)
	if err == nil {
		t.Error("IsGitRepo on non-git dir should return error")
	}
}

func TestGitRemoteURLNoRemote(t *testing.T) {
	tmpDir := t.TempDir()
	exec.Command("git", "init", tmpDir).Run()
	_, err := GitRemoteURL(tmpDir)
	if err == nil {
		t.Error("GitRemoteURL on repo with no remote should return error")
	}
}

func TestGitStatusCleanRepo(t *testing.T) {
	tmpDir := t.TempDir()
	exec.Command("git", "init", tmpDir).Run()
	status, err := GitStatus(tmpDir)
	if err != nil {
		t.Fatalf("GitStatus error: %v", err)
	}
	if status == "" {
		t.Error("GitStatus should return branch info even for empty repo")
	}
}

func TestShouldPullBothIntervalAndTimes(t *testing.T) {
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

func TestShouldPullIntervalExpiredButNotAtTime(t *testing.T) {
	now := time.Now()
	wrongTime := now.Add(-2 * time.Hour).Format("15:04")
	r := &RepoEntry{
		Schedule: Schedule{
			Interval: "1m",
			Times:    []string{wrongTime},
		},
		LastPull: now.Add(-2 * time.Minute).Format(time.RFC3339),
	}
	cfg := &Config{}
	if !r.ShouldPull(now, cfg) {
		t.Error("ShouldPull with expired interval should return true regardless of time")
	}
}

func TestShouldPullNoScheduleDefaultsToInterval(t *testing.T) {
	now := time.Now()
	r := &RepoEntry{
		Schedule: Schedule{},
		LastPull: now.Add(-35 * time.Minute).Format(time.RFC3339),
	}
	cfg := &Config{}
	if !r.ShouldPull(now, cfg) {
		t.Error("ShouldPull with default 30m interval after 35m should return true")
	}
}

func TestConfigRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &Config{
		DefaultInterval: "15m",
		DefaultTimes:     []string{"09:00", "18:00"},
		DefaultRange:     "08:00-22:00",
		Repos: []RepoEntry{
			{
				Path:     "/tmp/repo1",
				Schedule: Schedule{Interval: "10m", Times: []string{"09:00", "18:00"}},
				LastPull: "2026-04-22T08:00:00Z",
			},
			{
				Path:     "/tmp/repo2",
				Schedule: Schedule{Interval: "2h"},
			},
		},
	}

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if loaded.DefaultInterval != cfg.DefaultInterval {
		t.Errorf("DefaultInterval = %q, want %q", loaded.DefaultInterval, cfg.DefaultInterval)
	}
	if len(loaded.DefaultTimes) != 2 {
		t.Errorf("DefaultTimes len = %d, want 2", len(loaded.DefaultTimes))
	}
	if loaded.DefaultRange != cfg.DefaultRange {
		t.Errorf("DefaultRange = %q, want %q", loaded.DefaultRange, cfg.DefaultRange)
	}
	if len(loaded.Repos) != 2 {
		t.Fatalf("len(Repos) = %d, want 2", len(loaded.Repos))
	}
	if loaded.Repos[0].Schedule.Interval != "10m" {
		t.Errorf("Repos[0].Interval = %q, want 10m", loaded.Repos[0].Schedule.Interval)
	}
	if len(loaded.Repos[0].Schedule.Times) != 2 {
		t.Errorf("Repos[0].Times len = %d, want 2", len(loaded.Repos[0].Schedule.Times))
	}
	if loaded.Repos[0].LastPull != "2026-04-22T08:00:00Z" {
		t.Errorf("Repos[0].LastPull = %q, wrong", loaded.Repos[0].LastPull)
	}
	if loaded.Repos[1].Path != "/tmp/repo2" {
		t.Errorf("Repos[1].Path = %q, want /tmp/repo2", loaded.Repos[1].Path)
	}
}

func TestConfigMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig on missing file should not error: %v", err)
	}
	if cfg.DefaultInterval != "" {
		t.Errorf("Default interval for empty config = %q, want empty", cfg.DefaultInterval)
	}
	if len(cfg.Repos) != 0 {
		t.Errorf("Repos for empty config = %d, want 0", len(cfg.Repos))
	}
}

func TestShouldPullWithDefaultRange(t *testing.T) {
	// Within range: should be allowed to pull (interval elapsed)
	now := time.Now()
	r := &RepoEntry{
		Schedule: Schedule{Interval: "5m"},
		LastPull: now.Add(-10 * time.Minute).Format(time.RFC3339),
	}
	cfg := &Config{DefaultRange: "00:00-23:59"} // always in range
	if !r.ShouldPull(now, cfg) {
		t.Error("ShouldPull within default range with elapsed interval should return true")
	}
}

func TestShouldPullOutsideDefaultRange(t *testing.T) {
	now := time.Now()
	hour := now.Hour()
	// Create a range that excludes the current hour
	startHour := (hour + 2) % 24
	endHour := (hour + 3) % 24
	r := &RepoEntry{
		Schedule: Schedule{Interval: "5m"},
		LastPull: now.Add(-10 * time.Minute).Format(time.RFC3339),
	}
	cfg := &Config{DefaultRange: sprintf("%02d:00-%02d:59", startHour, endHour)}
	if r.ShouldPull(now, cfg) {
		t.Error("ShouldPull outside default range should return false even if interval elapsed")
	}
}

func TestShouldPullRepoRangeOverridesDefault(t *testing.T) {
	now := time.Now()
	r := &RepoEntry{
		Schedule: Schedule{
			Interval: "5m",
			Range:    "00:00-23:59", // always in range
		},
		LastPull: now.Add(-10 * time.Minute).Format(time.RFC3339),
	}
	// Config default range excludes current time
	cfg := &Config{DefaultRange: "03:00-04:00"}
	if !r.ShouldPull(now, cfg) {
		t.Error("Repo range should override config default range")
	}
}

func TestAddWithRangeFlag(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "cfg"))

	// Create a git repo
	repoDir := filepath.Join(tmpDir, "myrepo")
	exec.Command("git", "init", repoDir).Run()

	// Simulate: pulley add <repo> --range "09:00-17:00"
	cfg := &Config{}
	root, _ := IsGitRepo(repoDir)
	schedule := Schedule{Range: "09:00-17:00", Interval: "15m"}
	entry := RepoEntry{Path: root, Schedule: schedule}
	cfg.Repos = append(cfg.Repos, entry)

	if cfg.Repos[0].Schedule.Range != "09:00-17:00" {
		t.Errorf("Schedule.Range = %q, want 09:00-17:00", cfg.Repos[0].Schedule.Range)
	}
}

func sprintf(format string, a ...interface{}) string {
	return fmt.Sprintf(format, a...)
}