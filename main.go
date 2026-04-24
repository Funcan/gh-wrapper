package main

import (
	"fmt"
	"os"
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

	exitCode := run(args, targetUser)
	os.Exit(exitCode)
}
