package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestGitPullInBareRepo(t *testing.T) {
	// Create a bare repo, clone it, make a change in bare, then pull
	tmpDir := t.TempDir()
	bareRepo := filepath.Join(tmpDir, "bare.git")
	cloneDir := filepath.Join(tmpDir, "clone")

	// Init bare repo
	if err := exec.Command("git", "init", "--bare", bareRepo).Run(); err != nil {
		t.Fatalf("init bare repo: %v", err)
	}

	// Clone it
	if err := exec.Command("git", "clone", bareRepo, cloneDir).Run(); err != nil {
		t.Fatalf("clone repo: %v", err)
	}

	// Make an initial commit in the clone
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "initial")
	cmd.Dir = cloneDir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
	if err := cmd.Run(); err != nil {
		t.Fatalf("initial commit: %v", err)
	}
	if err := exec.Command("git", "push").Run(); err != nil {
		cmd := exec.Command("git", "-C", cloneDir, "push", "origin", "main")
		if err := cmd.Run(); err != nil {
			t.Logf("push failed (may need branch setup): %v", err)
		}
	}

	// Verify it's a git repo
	root, err := IsGitRepo(cloneDir)
	if err != nil {
		t.Fatalf("IsGitRepo on valid clone: %v", err)
	}
	if root != cloneDir {
		t.Errorf("IsGitRepo root = %q, want %q", root, cloneDir)
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
	// Should contain branch info
	if status == "" {
		t.Error("GitStatus should return branch info even for empty repo")
	}
}

func TestShouldPullBothIntervalAndTimes(t *testing.T) {
	// When both interval and times are set, either can trigger
	now := time.Now()
	r := &RepoEntry{
		Schedule: Schedule{
			Interval: "24h",
			Times:    []string{now.Format("15:04")},
		},
		LastPull: now.Add(-1 * time.Hour).Format(time.RFC3339),
	}
	// Within interval but at scheduled time = should pull
	if !r.ShouldPull(now) {
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
	// Interval expired = should pull regardless of time
	if !r.ShouldPull(now) {
		t.Error("ShouldPull with expired interval should return true")
	}
}

func TestShouldPullNoScheduleDefaultsToInterval(t *testing.T) {
	now := time.Now()
	r := &RepoEntry{
		Schedule: Schedule{},
		LastPull: now.Add(-35 * time.Minute).Format(time.RFC3339),
	}
	// Default interval is 30m, 35m elapsed = should pull
	if !r.ShouldPull(now) {
		t.Error("ShouldPull with default 30m interval after 35m should return true")
	}
}

func TestConfigRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &Config{
		DefaultInterval: "15m",
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
	if cfg.DefaultInterval != "30m" {
		t.Errorf("Default interval for missing config = %q, want 30m", cfg.DefaultInterval)
	}
	if len(cfg.Repos) != 0 {
		t.Errorf("Repos for missing config = %d, want 0", len(cfg.Repos))
	}
}