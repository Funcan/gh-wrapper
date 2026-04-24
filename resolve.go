package main

import (
	"errors"
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

// resolveUser is the top-level convenience used by main. It wires together git
// config lookup, remote URL parsing, conf file parsing, and user resolution.
// It returns the target user (empty string = no switch needed) and the GitHub
// hostname derived from the remote URL (defaulting to "github.com").
func resolveUser(cwd string) (targetUser, hostname string, err error) {
	hostname = "github.com"

	var gitConfigPath, remoteURL string
	gitConfigPath, err = FindGitConfig(cwd)
	if err != nil && !errors.Is(err, ErrGitConfigNotFound) {
		return "", "", err
	}
	if gitConfigPath != "" {
		remoteURL, _ = ReadRemoteURL(gitConfigPath)
	}
	if remoteURL != "" {
		if h, herr := ParseHostname(remoteURL); herr == nil {
			hostname = h
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	rules, err := ParseConfFile(filepath.Join(home, ".gh-wrapper.conf"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", "", err
	}

	targetUser, err = ResolveUser(cwd, gitConfigPath, remoteURL, rules)
	return targetUser, hostname, err
}
