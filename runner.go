package main

// runner.go: exec gh, handle signals, restore user

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

type authStatusResponse struct {
	Hosts map[string][]authAccount `json:"hosts"`
}

type authAccount struct {
	Active bool   `json:"active"`
	Login  string `json:"login"`
}

// GetCurrentUser returns the active gh account login for the given hostname.
func GetCurrentUser(hostname string) (string, error) {
	out, err := exec.Command("gh", "auth", "status", "--hostname", hostname, "--json", "hosts").Output()
	if err != nil {
		return "", fmt.Errorf("gh auth status: %w", err)
	}

	var status authStatusResponse
	if err := json.Unmarshal(out, &status); err != nil {
		return "", fmt.Errorf("parse auth status: %w", err)
	}

	for _, a := range status.Hosts[hostname] {
		if a.Active {
			return a.Login, nil
		}
	}
	return "", fmt.Errorf("no active account for host %q", hostname)
}

// SwitchUser switches the active gh account to user on the given hostname.
func SwitchUser(hostname, user string) error {
	cmd := exec.Command("gh", "auth", "switch", "--hostname", hostname, "--user", user)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunGh runs gh with the given args, forwarding stdin/stdout/stderr.
// Returns the exit code from gh.
func RunGh(args []string) int {
	cmd := exec.Command("gh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "[gh-wrapper] failed to run gh: %v\n", err)
		return 1
	}
	return 0
}

// run executes gh with the given args, switching to targetUser first if needed.
// Returns the exit code from gh.
func run(args []string, targetUser, hostname string) int {
	if targetUser == "" {
		return RunGh(args)
	}

	currentUser, err := GetCurrentUser(hostname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gh-wrapper] failed to get current user: %v\n", err)
		return 1
	}

	if currentUser == targetUser {
		return RunGh(args)
	}

	if err := SwitchUser(hostname, targetUser); err != nil {
		fmt.Fprintf(os.Stderr, "[gh-wrapper] failed to switch to user %q: %v\n", targetUser, err)
		return 1
	}
	// Restore original user after gh exits; signal handling is added in Phase 6.
	defer func() {
		if err := SwitchUser(hostname, currentUser); err != nil {
			fmt.Fprintf(os.Stderr, "[gh-wrapper] failed to restore user %q: %v\n", currentUser, err)
		}
	}()

	return RunGh(args)
}
