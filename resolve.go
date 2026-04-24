package main

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveUser determines which GitHub user to switch to for the given context.
//
// Priority:
//  1. [gh-wrapper] user in gitConfigPath (if non-empty)
//  2. First matching rule in rules (top-to-bottom)
//
// Returns "" if no user is configured (caller should skip the switch).
func ResolveUser(cwd, gitConfigPath, remoteURL string, rules []Rule) (string, error) {
	if gitConfigPath != "" {
		user, err := ReadGhWrapperUser(gitConfigPath)
		if err != nil {
			return "", err
		}
		if user != "" {
			return user, nil
		}
	}

	// Parse remote URL once; failure means github rules that need it will skip.
	var remoteOrg, remoteRepo string
	if remoteURL != "" {
		remoteOrg, remoteRepo, _ = ParseOrgRepo(remoteURL)
	}

	for _, rule := range rules {
		switch rule.Type {
		case "directory":
			if rule.Path == "" {
				return rule.User, nil
			}
			expanded, err := expandTilde(rule.Path)
			if err != nil {
				return "", err
			}
			expanded = filepath.Clean(expanded)
			clean := filepath.Clean(cwd)
			if clean == expanded || strings.HasPrefix(clean, expanded+string(filepath.Separator)) {
				return rule.User, nil
			}
		case "github":
			if rule.Org == "" {
				// catch-all
				return rule.User, nil
			}
			if remoteOrg == "" {
				// no parseable remote; can't match org-specific rules
				continue
			}
			if rule.Repo != "" {
				if remoteOrg == rule.Org && remoteRepo == rule.Repo {
					return rule.User, nil
				}
			} else {
				if remoteOrg == rule.Org {
					return rule.User, nil
				}
			}
		}
	}
	return "", nil
}

// expandTilde replaces a leading ~ or ~/ with the current user's home directory.
// Only bare ~ and ~/... forms are expanded; ~otheruser/... is returned unchanged.
func expandTilde(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home + path[1:], nil
	}
	return path, nil
}

// resolveUser is the top-level convenience used by main. Wired up in Phase 7.
func resolveUser(cwd string) (string, error) {
	return "", nil
}
