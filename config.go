package main

// config.go: parse .git/config and ~/.gh-wrapper.conf

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrGitConfigNotFound is returned by FindGitConfig when no .git/config is
// found by walking up from the starting directory.
var ErrGitConfigNotFound = errors.New("no .git/config found")

// FindGitConfig walks up from startDir (which may be relative, e.g. ".")
// looking for a .git/config file. Returns the absolute path on success,
// or ErrGitConfigNotFound if no git repo is found.
func FindGitConfig(startDir string) (string, error) {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	current := abs
	for {
		candidate := filepath.Join(current, ".git", "config")
		_, err := os.Stat(candidate)
		if err == nil {
			return candidate, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", ErrGitConfigNotFound
		}
		current = parent
	}
}

// ReadGhWrapperUser reads the `user` key from the [gh-wrapper] section of the
// git config file at gitConfigPath. Returns "" if the section or key is absent.
func ReadGhWrapperUser(gitConfigPath string) (string, error) {
	return readGitConfigValue(gitConfigPath, "[gh-wrapper]", "user")
}

// ReadRemoteURL reads the `url` key from the [remote "origin"] section of the
// git config file at gitConfigPath. Returns "" if the section or key is absent.
func ReadRemoteURL(gitConfigPath string) (string, error) {
	return readGitConfigValue(gitConfigPath, `[remote "origin"]`, "url")
}

// readGitConfigValue returns the value of key within the named section of a
// git config file. Returns "" if the section or key is not present.
func readGitConfigValue(gitConfigPath, section, key string) (string, error) {
	f, err := os.Open(gitConfigPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	inSection := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "[") {
			inSection = line == section
			continue
		}

		if inSection {
			k, value, ok := strings.Cut(line, "=")
			if ok && strings.TrimSpace(k) == key {
				return strings.TrimSpace(value), nil
			}
		}
	}
	return "", scanner.Err()
}

// ParseOrgRepo extracts the org and repo name from a GitHub remote URL.
// Supports https:// and git@ formats, with or without a .git suffix.
func ParseOrgRepo(remoteURL string) (org, repo string, err error) {
	var path string

	switch {
	case strings.HasPrefix(remoteURL, "git@"):
		// git@github.com:org/repo.git
		_, after, ok := strings.Cut(remoteURL, ":")
		if !ok {
			return "", "", fmt.Errorf("invalid git@ URL: %q", remoteURL)
		}
		path = after
	case strings.HasPrefix(remoteURL, "https://"), strings.HasPrefix(remoteURL, "http://"):
		// https://github.com/org/repo.git
		_, after, _ := strings.Cut(remoteURL, "://")
		_, path, _ = strings.Cut(after, "/")
	default:
		return "", "", fmt.Errorf("unsupported remote URL format: %q", remoteURL)
	}

	path = strings.TrimSuffix(path, ".git")

	org, repo, ok := strings.Cut(path, "/")
	if !ok || org == "" || repo == "" {
		return "", "", fmt.Errorf("cannot parse org/repo from URL: %q", remoteURL)
	}

	return org, repo, nil
}
