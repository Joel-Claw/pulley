package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IsGitRepo checks if the given path is inside a git repository
// and returns the root of the repo.
func IsGitRepo(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	// Check if .git exists (repo root) or use git rev-parse
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = abs
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %s", abs)
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", fmt.Errorf("could not determine repo root: %s", abs)
	}
	return root, nil
}

// GitPull performs a git pull in the given repo path.
// If branches are specified, it pulls each branch that exists locally.
// If no branches are specified, it pulls the current branch.
func GitPull(repoPath string, branches []string) error {
	if len(branches) == 0 {
		cmd := exec.Command("git", "pull", "--ff-only")
		cmd.Dir = repoPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Save current branch
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoPath
	origOut, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("get current branch: %w", err)
	}
	origBranch := strings.TrimSpace(string(origOut))

	// Track which branches had errors
	var pullErrs []string
	pulled := 0

	for _, branch := range branches {
		// Check if branch exists locally
		cmd := exec.Command("git", "rev-parse", "--verify", branch)
		cmd.Dir = repoPath
		if err := cmd.Run(); err != nil {
			// Branch doesn't exist locally, skip
			continue
		}

		// Checkout the branch
		cmd = exec.Command("git", "checkout", branch)
		cmd.Dir = repoPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			pullErrs = append(pullErrs, fmt.Sprintf("%s: checkout failed: %v", branch, err))
			continue
		}

		// Pull
		cmd = exec.Command("git", "pull", "--ff-only")
		cmd.Dir = repoPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			pullErrs = append(pullErrs, fmt.Sprintf("%s: pull failed: %v", branch, err))
			continue
		}
		pulled++
	}

	// Restore original branch
	if origBranch != "" && origBranch != "(no branch)" {
		cmd = exec.Command("git", "checkout", origBranch)
		cmd.Dir = repoPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			// Log but don't fail — pull may have succeeded
			fmt.Fprintf(os.Stderr, "warning: failed to restore branch %s: %v\n", origBranch, err)
		}
	}

	if len(pullErrs) > 0 {
		return fmt.Errorf("pull errors: %s", strings.Join(pullErrs, "; "))
	}
	return nil
}

// GitListBranches returns the list of local branch names in the repo.
func GitListBranches(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// GitStatus returns a brief status string for the repo.
func GitStatus(repoPath string) (string, error) {
	cmd := exec.Command("git", "status", "--short", "--branch")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// GitRemoteURL returns the fetch URL of the origin remote.
func GitRemoteURL(repoPath string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("no origin remote: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}