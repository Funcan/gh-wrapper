package main

// config.go: parse .git/config and ~/.gh-wrapper.conf

import (
	"bufio"
	"errors"
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
