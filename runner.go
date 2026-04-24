package main

// runner.go: exec gh, handle signals, restore user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
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
// Signals received on sigCh are forwarded to the child process.
// Returns the exit code from gh and the signal that caused the child to exit (if any).
func RunGh(args []string, sigCh <-chan os.Signal) (int, os.Signal) {
	cmd := exec.Command("gh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "[gh-wrapper] failed to start gh: %v\n", err)
		return 1, nil
	}

	// Forward signals to the child. caughtCh carries the signal back after the
	// goroutine exits so the caller can re-raise after cleanup.
	caughtCh := make(chan os.Signal, 1)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer close(caughtCh)
		select {
		case sig, ok := <-sigCh:
			if ok {
				caughtCh <- sig
				_ = cmd.Process.Signal(sig)
			}
		case <-ctx.Done():
		}
	}()

	err := cmd.Wait()
	cancel()
	caught := <-caughtCh // wait for goroutine to finish

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), caught
		}
		fmt.Fprintf(os.Stderr, "[gh-wrapper] failed to run gh: %v\n", err)
		return 1, caught
	}
	return 0, caught
}

// run executes gh with the given args, switching to targetUser first if needed.
// Signal handling covers the full switch/run/restore lifecycle.
// Returns the exit code from gh and any signal that caused the child to exit.
func run(args []string, targetUser, hostname string) (int, os.Signal) {
	// Capture signals for the entire lifecycle so no signal can escape cleanup.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	if targetUser == "" {
		return RunGh(args, sigCh)
	}

	currentUser, err := GetCurrentUser(hostname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gh-wrapper] failed to get current user: %v\n", err)
		return 1, nil
	}

	if currentUser == targetUser {
		return RunGh(args, sigCh)
	}

	if err := SwitchUser(hostname, targetUser); err != nil {
		fmt.Fprintf(os.Stderr, "[gh-wrapper] failed to switch to user %q: %v\n", targetUser, err)
		return 1, nil
	}
	defer func() {
		if err := SwitchUser(hostname, currentUser); err != nil {
			fmt.Fprintf(os.Stderr, "[gh-wrapper] failed to restore user %q: %v\n", currentUser, err)
		}
	}()

	return RunGh(args, sigCh)
}
