package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	args := os.Args[1:]

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gh-wrapper] failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	targetUser, err := resolveUser(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gh-wrapper] failed to resolve user: %v\n", err)
		os.Exit(1)
	}

	exitCode, caught := run(args, targetUser, "github.com")
	if caught != nil {
		// Re-raise so the shell sees signal termination (not a plain exit code).
		signal.Reset(caught)
		_ = syscall.Kill(syscall.Getpid(), caught.(syscall.Signal))
	}
	os.Exit(exitCode)
}
