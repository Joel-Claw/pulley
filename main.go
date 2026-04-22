package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "add":
		cmdAdd(os.Args[2:])
	case "remove", "rm":
		cmdRemove(os.Args[2:])
	case "list", "ls":
		cmdList()
	case "pull":
		cmdPull(os.Args[2:])
	case "daemon":
		cmdDaemon()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`autopull - automatic git pull daemon

Usage:
  autopull add [path] [--interval <dur>] [--at <times>]   Register a repo
  autopull remove <path>                                    Unregister a repo
  autopull list                                             List registered repos
  autopull pull [path]                                      Pull repos now
  autopull daemon                                           Run as daemon (foreground)

Schedule flags:
  --interval 15m     Pull every 15 minutes (default: 30m)
  --at "09:00,18:00" Pull at specific times (HH:MM, comma-separated)`)
}

func cmdAdd(args []string) {
	var path string
	var interval string
	var timesRaw string

	i := 0
	for i < len(args) {
		switch args[i] {
		case "--interval":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "--interval requires a value (e.g. 15m, 2h)")
				os.Exit(1)
			}
			interval = args[i]
		case "--at":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "--at requires a value (e.g. \"09:00,18:00\")")
				os.Exit(1)
			}
			timesRaw = args[i]
		default:
			if path == "" {
				path = args[i]
			}
		}
		i++
	}

	if path == "" {
		path = "."
	}

	// Verify it's a git repo
	root, err := IsGitRepo(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	// Check for duplicate
	for _, r := range cfg.Repos {
		if r.Path == root {
			fmt.Fprintf(os.Stderr, "repo already registered: %s\n", root)
			os.Exit(1)
		}
	}

	schedule := Schedule{Interval: interval}
	if timesRaw != "" {
		schedule.Times = splitTimes(timesRaw)
	}

	entry := RepoEntry{
		Path:     root,
		Schedule: schedule,
	}
	cfg.Repos = append(cfg.Repos, entry)

	if err := SaveConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error saving config: %v\n", err)
		os.Exit(1)
	}

	remote, _ := GitRemoteURL(root)
	fmt.Printf("Added: %s\n", root)
	if remote != "" {
		fmt.Printf("  Remote: %s\n", remote)
	}
	if interval != "" {
		fmt.Printf("  Interval: %s\n", interval)
	}
	if len(schedule.Times) > 0 {
		fmt.Printf("  At: %v\n", schedule.Times)
	}
}

func cmdRemove(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: autopull remove <path>")
		os.Exit(1)
	}

	path := args[0]
	root, err := IsGitRepo(path)
	if err != nil {
		// Might already be removed or path gone, try exact match
		root = path
	}

	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	found := false
	var filtered []RepoEntry
	for _, r := range cfg.Repos {
		if r.Path == root {
			found = true
			continue
		}
		filtered = append(filtered, r)
	}
	cfg.Repos = filtered

	if !found {
		fmt.Fprintf(os.Stderr, "repo not registered: %s\n", root)
		os.Exit(1)
	}

	if err := SaveConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error saving config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Removed: %s\n", root)
}

func cmdList() {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	if len(cfg.Repos) == 0 {
		fmt.Println("No repos registered. Use 'autopull add' to add one.")
		return
	}

	for i, r := range cfg.Repos {
		interval := r.Schedule.Interval
		if interval == "" {
			interval = "30m (default)"
		}
		fmt.Printf("%d. %s\n", i+1, r.Path)
		remote, _ := GitRemoteURL(r.Path)
		if remote != "" {
			fmt.Printf("   Remote: %s\n", remote)
		}
		fmt.Printf("   Interval: %s\n", interval)
		if len(r.Schedule.Times) > 0 {
			fmt.Printf("   At: %v\n", r.Schedule.Times)
		}
		if r.LastPull != "" {
			fmt.Printf("   Last pull: %s\n", r.LastPull)
		}
	}
}

func cmdPull(args []string) {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	if len(cfg.Repos) == 0 {
		fmt.Println("No repos registered.")
		return
	}

	var target string
	if len(args) > 0 {
		target = args[0]
	}

	for i := range cfg.Repos {
		r := &cfg.Repos[i]
		if target != "" && r.Path != target {
			continue
		}
		fmt.Printf("Pulling %s...\n", r.Path)
		if err := GitPull(r.Path); err != nil {
			fmt.Fprintf(os.Stderr, "  failed: %v\n", err)
			continue
		}
		r.LastPull = time.Now().Format(time.RFC3339)
		fmt.Println("  done")
	}

	if err := SaveConfig(cfg); err != nil {
		log.Printf("warning: could not save config: %v", err)
	}
}

func cmdDaemon() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	log.Println("autopull daemon started")
	log.Printf("watching %d repo(s)", len(cfg.Repos))

	// Check every minute
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Do an initial check
	pullIfNeeded(cfg)

	for range ticker.C {
		// Reload config each cycle to pick up changes
		cfg, err = LoadConfig()
		if err != nil {
			log.Printf("error reloading config: %v", err)
			continue
		}
		pullIfNeeded(cfg)
	}
}

func pullIfNeeded(cfg *Config) {
	now := time.Now()
	changed := false

	for i := range cfg.Repos {
		r := &cfg.Repos[i]
		if !r.ShouldPull(now) {
			continue
		}

		log.Printf("pulling %s", r.Path)
		if err := GitPull(r.Path); err != nil {
			log.Printf("  failed: %v", err)
			continue
		}
		r.LastPull = now.Format(time.RFC3339)
		changed = true
	}

	if changed {
		if err := SaveConfig(cfg); err != nil {
			log.Printf("warning: could not save config: %v", err)
		}
	}
}

// splitTimes splits a comma-separated time string like "09:00,18:00" into a slice.
func splitTimes(raw string) []string {
	var result []string
	for _, t := range splitComma(raw) {
		t = strings.TrimSpace(t)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

func splitComma(s string) []string {
	return strings.Split(s, ",")
}