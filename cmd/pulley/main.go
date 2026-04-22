package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

var version = "0.2.0" // set via -ldflags at build time

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
	case "config":
		cmdConfig(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("pulley %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`pulley - automatic git pull daemon

Usage:
  pulley add [path] [--interval <dur>] [--at <times>] [--range <range>] [--branches <branches>]
                                                Register a repo
  pulley remove <path>                          Unregister a repo
  pulley list                                   List registered repos
  pulley pull [path]                            Pull repos now
  pulley daemon                                 Run as daemon (foreground)
  pulley config                                 Show current config
  pulley config set <key> <value>               Set a config default
  pulley config set --at "09:00,18:00"          Set default pull times
  pulley config set --range "09:00-17:00"       Set default active range
  pulley version                                Show version

Schedule flags (for add):
  --interval 15m          Pull every 15 minutes (default: 30m or config default)
  --at "09:00,18:00"      Pull at specific times (HH:MM, comma-separated)
  --range "09:00-17:00"   Only pull within this time window
  --branches "main,dev"  Pull specific branches (comma-separated, or "all" for every local branch)

Config keys:
  defaultInterval   Default pull interval for repos without one (e.g. 15m, 2h)
  defaultTimes      Default pull times for repos without any
  defaultRange      Default time range - repos only pull within this window
  defaultBranches   Default branches to pull (e.g. ["main","dev"] or ["all"])`)
}

func cmdAdd(args []string) {
	var path string
	var interval string
	var timesRaw string
	var timeRange string
	var branchesRaw string

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
		case "--range":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "--range requires a value (e.g. \"09:00-17:00\")")
				os.Exit(1)
			}
			timeRange = args[i]
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
	if timeRange != "" {
		schedule.Range = timeRange
	}
	if branchesRaw != "" {
		schedule.Branches = splitBranches(branchesRaw)
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
	effectiveInterval := cfg.EffectiveInterval(schedule)
	effectiveTimes := cfg.EffectiveTimes(schedule)
	effectiveRange := cfg.EffectiveRange(schedule)

	fmt.Printf("Added: %s\n", root)
	if remote != "" {
		fmt.Printf("  Remote: %s\n", remote)
	}
	fmt.Printf("  Interval: %s", effectiveInterval)
	if interval == "" && cfg.DefaultInterval != "" {
		fmt.Printf(" (from config default)")
	}
	fmt.Println()
	if len(effectiveTimes) > 0 {
		fmt.Printf("  At: %v", effectiveTimes)
		if len(schedule.Times) == 0 && len(cfg.DefaultTimes) > 0 {
			fmt.Printf(" (from config default)")
		}
		fmt.Println()
	}
	if effectiveRange != "" {
		fmt.Printf("  Range: %s", effectiveRange)
		if schedule.Range == "" && cfg.DefaultRange != "" {
			fmt.Printf(" (from config default)")
		}
		fmt.Println()
	}
	effectiveBranches := cfg.EffectiveBranches(schedule)
	if len(effectiveBranches) > 0 {
		fmt.Printf("  Branches: %v", effectiveBranches)
		if len(schedule.Branches) == 0 && len(cfg.DefaultBranches) > 0 {
			fmt.Printf(" (from config default)")
		}
		fmt.Println()
	} else {
		fmt.Println("  Branches: current only")
	}
}

func cmdRemove(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: pulley remove <path>")
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

	// Show defaults if set
	if cfg.DefaultInterval != "" || len(cfg.DefaultTimes) > 0 || cfg.DefaultRange != "" || len(cfg.DefaultBranches) > 0 {
		fmt.Println("Defaults:")
		if cfg.DefaultInterval != "" {
			fmt.Printf("  Interval: %s\n", cfg.DefaultInterval)
		}
		if len(cfg.DefaultTimes) > 0 {
			fmt.Printf("  Times: %v\n", cfg.DefaultTimes)
		}
		if cfg.DefaultRange != "" {
			fmt.Printf("  Range: %s\n", cfg.DefaultRange)
		}
		if len(cfg.DefaultBranches) > 0 {
			fmt.Printf("  Branches: %v\n", cfg.DefaultBranches)
		}
		fmt.Println()
	}

	if len(cfg.Repos) == 0 {
		fmt.Println("No repos registered. Use 'pulley add' to add one.")
		return
	}

	for i, r := range cfg.Repos {
		effectiveInterval := cfg.EffectiveInterval(r.Schedule)
		effectiveTimes := cfg.EffectiveTimes(r.Schedule)
		effectiveRange := cfg.EffectiveRange(r.Schedule)

		fmt.Printf("%d. %s\n", i+1, r.Path)
		remote, _ := GitRemoteURL(r.Path)
		if remote != "" {
			fmt.Printf("   Remote: %s\n", remote)
		}
		fmt.Printf("   Interval: %s", effectiveInterval)
		if r.Schedule.Interval == "" && cfg.DefaultInterval != "" {
			fmt.Printf(" (default)")
		}
		fmt.Println()
		if len(effectiveTimes) > 0 {
			fmt.Printf("   At: %v", effectiveTimes)
			if len(r.Schedule.Times) == 0 && len(cfg.DefaultTimes) > 0 {
				fmt.Printf(" (default)")
			}
			fmt.Println()
		}
		if effectiveRange != "" {
			fmt.Printf("   Range: %s", effectiveRange)
			if r.Schedule.Range == "" && cfg.DefaultRange != "" {
				fmt.Printf(" (default)")
			}
			fmt.Println()
		}
		effectiveBranches := cfg.EffectiveBranches(r.Schedule)
		if len(effectiveBranches) > 0 {
			fmt.Printf("   Branches: %v", effectiveBranches)
			if len(r.Schedule.Branches) == 0 && len(cfg.DefaultBranches) > 0 {
				fmt.Printf(" (default)")
			}
			fmt.Println()
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
		branches := cfg.EffectiveBranches(r.Schedule)
		if len(branches) == 1 && branches[0] == "all" {
			var err error
			branches, err = GitListBranches(r.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  warning: could not list branches: %v\n", err)
				branches = nil
			}
		}
		fmt.Printf("Pulling %s...\n", r.Path)
		if err := GitPull(r.Path, branches); err != nil {
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

	log.Println("pulley daemon started")
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
		if !r.ShouldPull(now, cfg) {
			continue
		}

		branches := cfg.EffectiveBranches(r.Schedule)
		if len(branches) == 1 && branches[0] == "all" {
			var err error
			branches, err = GitListBranches(r.Path)
			if err != nil {
				log.Printf("  warning: could not list branches: %v", err)
				branches = nil
			}
		}
		log.Printf("pulling %s", r.Path)
		if err := GitPull(r.Path, branches); err != nil {
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

func cmdConfig(args []string) {
	if len(args) == 0 {
		// Show current config
		cmdConfigShow()
		return
	}

	switch args[0] {
	case "set":
		cmdConfigSet(args[1:])
	case "show":
		cmdConfigShow()
	default:
		fmt.Fprintf(os.Stderr, "unknown config subcommand: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "Usage: pulley config [set|show]")
		os.Exit(1)
	}
}

func cmdConfigShow() {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Config:")
	if cfg.DefaultInterval != "" {
		fmt.Printf("  defaultInterval: %s\n", cfg.DefaultInterval)
	} else {
		fmt.Println("  defaultInterval: (not set, uses 30m)")
	}
	if len(cfg.DefaultTimes) > 0 {
		fmt.Printf("  defaultTimes: %v\n", cfg.DefaultTimes)
	} else {
		fmt.Println("  defaultTimes: (not set)")
	}
	if cfg.DefaultRange != "" {
		fmt.Printf("  defaultRange: %s\n", cfg.DefaultRange)
	} else {
		fmt.Println("  defaultRange: (not set)")
	}
	if len(cfg.DefaultBranches) > 0 {
		fmt.Printf("  defaultBranches: %v\n", cfg.DefaultBranches)
	} else {
		fmt.Println("  defaultBranches: (not set, pulls current branch only)")
	}
	fmt.Printf("  repos: %d registered\n", len(cfg.Repos))
}

func cmdConfigSet(args []string) {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	// Parse flags and positional args
	var interval string
	var timesRaw string
	var timeRange string
	var branchesRaw string
	var key string
	var value string

	i := 0
	for i < len(args) {
		switch args[i] {
		case "--interval":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "--interval requires a value")
				os.Exit(1)
			}
			interval = args[i]
		case "--at":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "--at requires a value")
				os.Exit(1)
			}
			timesRaw = args[i]
		case "--range":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "--range requires a value")
				os.Exit(1)
			}
			timeRange = args[i]
		case "--branches":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "--branches requires a value (e.g. \"main,dev\" or \"all\")")
				os.Exit(1)
			}
			branchesRaw = args[i]
		default:
			if key == "" {
				key = args[i]
			} else if value == "" {
				value = args[i]
			}
		}
		i++
	}

	changed := false

	// Handle flags
	if interval != "" {
		cfg.DefaultInterval = interval
		fmt.Printf("Set defaultInterval: %s\n", interval)
		changed = true
	}
	if timesRaw != "" {
		cfg.DefaultTimes = splitTimes(timesRaw)
		fmt.Printf("Set defaultTimes: %v\n", cfg.DefaultTimes)
		changed = true
	}
	if timeRange != "" {
		cfg.DefaultRange = timeRange
		fmt.Printf("Set defaultRange: %s\n", timeRange)
		changed = true
	}
	if branchesRaw != "" {
		cfg.DefaultBranches = splitBranches(branchesRaw)
		fmt.Printf("Set defaultBranches: %v\n", cfg.DefaultBranches)
		changed = true
	}

	// Handle key=value pairs
	if key != "" {
		switch key {
		case "defaultInterval":
			if value == "" {
				fmt.Fprintln(os.Stderr, "defaultInterval requires a value (e.g. 15m, 2h)")
				os.Exit(1)
			}
			if _, err := time.ParseDuration(value); err != nil {
				fmt.Fprintf(os.Stderr, "invalid duration: %s (e.g. 15m, 2h, 1h30m)\n", value)
				os.Exit(1)
			}
			cfg.DefaultInterval = value
			fmt.Printf("Set defaultInterval: %s\n", value)
			changed = true
		case "defaultRange":
			if value == "" {
				fmt.Fprintln(os.Stderr, "defaultRange requires a value (e.g. \"09:00-17:00\")")
				os.Exit(1)
			}
			cfg.DefaultRange = value
			fmt.Printf("Set defaultRange: %s\n", value)
			changed = true
		default:
			fmt.Fprintf(os.Stderr, "unknown config key: %s\n", key)
			fmt.Fprintln(os.Stderr, "Valid keys: defaultInterval, defaultRange")
			fmt.Fprintln(os.Stderr, "Use flags for defaultTimes: --at \"09:00,18:00\"")
			fmt.Fprintln(os.Stderr, "Use flags for defaultBranches: --branches \"main,dev\"")
			os.Exit(1)
		}
	}

	if !changed {
		fmt.Fprintln(os.Stderr, "nothing to set. Use --interval, --at, --range, --branches, or a key=value pair.")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  pulley config set --interval 15m")
		fmt.Fprintln(os.Stderr, "  pulley config set --at \"09:00,18:00\"")
		fmt.Fprintln(os.Stderr, "  pulley config set --range \"09:00-17:00\"")
		fmt.Fprintln(os.Stderr, "  pulley config set --branches \"main,dev\"")
		fmt.Fprintln(os.Stderr, "  pulley config set defaultInterval 2h")
		os.Exit(1)
	}

	if err := SaveConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error saving config: %v\n", err)
		os.Exit(1)
	}
}

// splitTimes splits a comma-separated time string like "09:00,18:00" into a slice.
func splitTimes(raw string) []string {
	var result []string
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

// splitBranches splits a comma-separated branch string like "main,dev" into a slice.
// The special value "all" is kept as-is.
func splitBranches(raw string) []string {
	var result []string
	for _, b := range strings.Split(raw, ",") {
		b = strings.TrimSpace(b)
		if b != "" {
			result = append(result, b)
		}
	}
	return result
}