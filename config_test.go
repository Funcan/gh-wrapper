package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func makeGitConfig(t *testing.T, dir string) string {
	t.Helper()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(gitDir, "config")
	if err := os.WriteFile(configPath, []byte("[core]\n\trepositoryformatversion = 0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return configPath
}

func TestFindGitConfig(t *testing.T) {
	t.Run("found in start directory", func(t *testing.T) {
		root := t.TempDir()
		want := makeGitConfig(t, root)

		got, err := FindGitConfig(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("found in parent", func(t *testing.T) {
		root := t.TempDir()
		want := makeGitConfig(t, root)
		child := filepath.Join(root, "child")
		if err := os.Mkdir(child, 0755); err != nil {
			t.Fatal(err)
		}

		got, err := FindGitConfig(child)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("found several levels up", func(t *testing.T) {
		root := t.TempDir()
		want := makeGitConfig(t, root)
		deep := filepath.Join(root, "a", "b", "c")
		if err := os.MkdirAll(deep, 0755); err != nil {
			t.Fatal(err)
		}

		got, err := FindGitConfig(deep)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("not found returns ErrGitConfigNotFound", func(t *testing.T) {
		// A temp dir with no .git anywhere in its ancestry up to root.
		// We can't guarantee the real filesystem root has no .git, so we
		// synthesise a directory tree under TempDir without any .git.
		root := t.TempDir()
		_, err := FindGitConfig(root)
		if !errors.Is(err, ErrGitConfigNotFound) {
			t.Errorf("got %v, want ErrGitConfigNotFound", err)
		}
	})

	t.Run("relative dot resolves to cwd", func(t *testing.T) {
		root := t.TempDir()
		makeGitConfig(t, root)

		// Resolve symlinks so the expected path matches what filepath.Abs(".")
		// returns after os.Chdir (on macOS /var -> /private/var).
		realRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(realRoot, ".git", "config")

		orig, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chdir(orig) })
		if err := os.Chdir(root); err != nil {
			t.Fatal(err)
		}

		got, err := FindGitConfig(".")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("dot inside nested dir finds ancestor config", func(t *testing.T) {
		root := t.TempDir()
		makeGitConfig(t, root)
		deep := filepath.Join(root, "pkg", "sub")
		if err := os.MkdirAll(deep, 0755); err != nil {
			t.Fatal(err)
		}

		realRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(realRoot, ".git", "config")

		orig, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chdir(orig) })
		if err := os.Chdir(deep); err != nil {
			t.Fatal(err)
		}

		got, err := FindGitConfig(".")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("nearest .git/config wins over ancestor", func(t *testing.T) {
		root := t.TempDir()
		makeGitConfig(t, root) // ancestor — should be shadowed
		inner := filepath.Join(root, "nested")
		if err := os.Mkdir(inner, 0755); err != nil {
			t.Fatal(err)
		}
		want := makeGitConfig(t, inner)

		got, err := FindGitConfig(inner)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// writeGitConfig writes a .git/config with the given content and returns the path.
func writeGitConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(gitDir, "config")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadGhWrapperUser(t *testing.T) {
	t.Run("returns user from gh-wrapper section", func(t *testing.T) {
		path := writeGitConfig(t, "[core]\n\trepositoryformatversion = 0\n[gh-wrapper]\n\tuser = alice\n")
		got, err := ReadGhWrapperUser(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "alice" {
			t.Errorf("got %q, want %q", got, "alice")
		}
	})

	t.Run("returns empty string when section absent", func(t *testing.T) {
		path := writeGitConfig(t, "[core]\n\trepositoryformatversion = 0\n")
		got, err := ReadGhWrapperUser(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("returns empty string when section present but no user key", func(t *testing.T) {
		path := writeGitConfig(t, "[gh-wrapper]\n\tother = value\n")
		got, err := ReadGhWrapperUser(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("finds section when not first", func(t *testing.T) {
		path := writeGitConfig(t, "[core]\n\tbare = false\n[remote \"origin\"]\n\turl = git@github.com:org/repo.git\n[gh-wrapper]\n\tuser = bob\n")
		got, err := ReadGhWrapperUser(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "bob" {
			t.Errorf("got %q, want %q", got, "bob")
		}
	})

	t.Run("trims whitespace from value", func(t *testing.T) {
		path := writeGitConfig(t, "[gh-wrapper]\n\tuser =   carol   \n")
		got, err := ReadGhWrapperUser(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "carol" {
			t.Errorf("got %q, want %q", got, "carol")
		}
	})

	t.Run("stops reading after gh-wrapper section ends", func(t *testing.T) {
		// user key appears in a later section — must not be picked up
		path := writeGitConfig(t, "[gh-wrapper]\n\tother = x\n[other]\n\tuser = impostor\n")
		got, err := ReadGhWrapperUser(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		_, err := ReadGhWrapperUser("/nonexistent/path/.git/config")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	// Integration: FindGitConfig + ReadGhWrapperUser round-trip.
	t.Run("integration: FindGitConfig then ReadGhWrapperUser", func(t *testing.T) {
		root := t.TempDir()
		gitDir := filepath.Join(root, ".git")
		if err := os.MkdirAll(gitDir, 0755); err != nil {
			t.Fatal(err)
		}
		content := "[core]\n\trepositoryformatversion = 0\n[gh-wrapper]\n\tuser = integrationuser\n"
		if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		cfgPath, err := FindGitConfig(root)
		if err != nil {
			t.Fatalf("FindGitConfig: %v", err)
		}
		got, err := ReadGhWrapperUser(cfgPath)
		if err != nil {
			t.Fatalf("ReadGhWrapperUser: %v", err)
		}
		if got != "integrationuser" {
			t.Errorf("got %q, want %q", got, "integrationuser")
		}
	})
}

func TestReadRemoteURL(t *testing.T) {
	t.Run("returns url from remote origin section", func(t *testing.T) {
		path := writeGitConfig(t, "[core]\n\tbare = false\n[remote \"origin\"]\n\turl = git@github.com:org/repo.git\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n")
		got, err := ReadRemoteURL(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "git@github.com:org/repo.git" {
			t.Errorf("got %q, want %q", got, "git@github.com:org/repo.git")
		}
	})

	t.Run("returns empty string when remote origin section absent", func(t *testing.T) {
		path := writeGitConfig(t, "[core]\n\tbare = false\n")
		got, err := ReadRemoteURL(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("does not match other remote sections", func(t *testing.T) {
		path := writeGitConfig(t, "[remote \"upstream\"]\n\turl = git@github.com:other/repo.git\n")
		got, err := ReadRemoteURL(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		_, err := ReadRemoteURL("/nonexistent/path/.git/config")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	// Integration: FindGitConfig + ReadRemoteURL round-trip.
	t.Run("integration: FindGitConfig then ReadRemoteURL", func(t *testing.T) {
		root := t.TempDir()
		gitDir := filepath.Join(root, ".git")
		if err := os.MkdirAll(gitDir, 0755); err != nil {
			t.Fatal(err)
		}
		content := "[core]\n\tbare = false\n[remote \"origin\"]\n\turl = https://github.com/org/repo.git\n"
		if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		cfgPath, err := FindGitConfig(root)
		if err != nil {
			t.Fatalf("FindGitConfig: %v", err)
		}
		got, err := ReadRemoteURL(cfgPath)
		if err != nil {
			t.Fatalf("ReadRemoteURL: %v", err)
		}
		if got != "https://github.com/org/repo.git" {
			t.Errorf("got %q, want %q", got, "https://github.com/org/repo.git")
		}
	})
}

func writeConfFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "gh-wrapper*.conf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestParseConfFile(t *testing.T) {
	t.Run("empty file returns empty slice", func(t *testing.T) {
		path := writeConfFile(t, "")
		rules, err := ParseConfFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rules) != 0 {
			t.Errorf("got %d rules, want 0", len(rules))
		}
	})

	t.Run("blank lines and comments are ignored", func(t *testing.T) {
		path := writeConfFile(t, "\n# this is a comment\n\n# another comment\n")
		rules, err := ParseConfFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rules) != 0 {
			t.Errorf("got %d rules, want 0", len(rules))
		}
	})

	t.Run("directory rule", func(t *testing.T) {
		path := writeConfFile(t, "directory ~/work: alice\n")
		rules, err := ParseConfFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rules) != 1 {
			t.Fatalf("got %d rules, want 1", len(rules))
		}
		r := rules[0]
		if r.Type != "directory" || r.Path != "~/work" || r.User != "alice" {
			t.Errorf("got %+v, want {Type:directory Path:~/work User:alice}", r)
		}
	})

	t.Run("github org/repo rule", func(t *testing.T) {
		path := writeConfFile(t, "github myorg/myrepo: bob\n")
		rules, err := ParseConfFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rules) != 1 {
			t.Fatalf("got %d rules, want 1", len(rules))
		}
		r := rules[0]
		if r.Type != "github" || r.Org != "myorg" || r.Repo != "myrepo" || r.User != "bob" {
			t.Errorf("got %+v, want {Type:github Org:myorg Repo:myrepo User:bob}", r)
		}
	})

	t.Run("github org-only rule", func(t *testing.T) {
		path := writeConfFile(t, "github myorg: carol\n")
		rules, err := ParseConfFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rules) != 1 {
			t.Fatalf("got %d rules, want 1", len(rules))
		}
		r := rules[0]
		if r.Type != "github" || r.Org != "myorg" || r.Repo != "" || r.User != "carol" {
			t.Errorf("got %+v, want {Type:github Org:myorg Repo: User:carol}", r)
		}
	})

	t.Run("github catch-all rule", func(t *testing.T) {
		path := writeConfFile(t, "github :defaultuser\n")
		rules, err := ParseConfFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rules) != 1 {
			t.Fatalf("got %d rules, want 1", len(rules))
		}
		r := rules[0]
		if r.Type != "github" || r.Org != "" || r.Repo != "" || r.User != "defaultuser" {
			t.Errorf("got %+v, want {Type:github Org: Repo: User:defaultuser}", r)
		}
	})

	t.Run("multiple rules preserve order", func(t *testing.T) {
		content := "# comment\ndirectory ~/work: alice\ngithub myorg/myrepo: bob\ngithub myorg: carol\ngithub :dave\n"
		path := writeConfFile(t, content)
		rules, err := ParseConfFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rules) != 4 {
			t.Fatalf("got %d rules, want 4", len(rules))
		}
		wantUsers := []string{"alice", "bob", "carol", "dave"}
		for i, r := range rules {
			if r.User != wantUsers[i] {
				t.Errorf("rule[%d]: got user %q, want %q", i, r.User, wantUsers[i])
			}
		}
	})

	t.Run("whitespace trimmed from path and user", func(t *testing.T) {
		path := writeConfFile(t, "directory  ~/some/path : myuser \n")
		rules, err := ParseConfFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rules) != 1 {
			t.Fatalf("got %d rules, want 1", len(rules))
		}
		r := rules[0]
		if r.Path != "~/some/path" || r.User != "myuser" {
			t.Errorf("got path=%q user=%q, want path=~/some/path user=myuser", r.Path, r.User)
		}
	})

	t.Run("file not found returns error", func(t *testing.T) {
		_, err := ParseConfFile("/nonexistent/path/gh-wrapper.conf")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("unrecognised line returns error", func(t *testing.T) {
		path := writeConfFile(t, "unknown rule here\n")
		_, err := ParseConfFile(path)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("directory catch-all rule", func(t *testing.T) {
		path := writeConfFile(t, "directory :defaultuser\n")
		rules, err := ParseConfFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rules) != 1 {
			t.Fatalf("got %d rules, want 1", len(rules))
		}
		r := rules[0]
		if r.Type != "directory" || r.Path != "" || r.User != "defaultuser" {
			t.Errorf("got %+v, want {Type:directory Path: User:defaultuser}", r)
		}
	})

	t.Run("directory rule missing colon returns error", func(t *testing.T) {
		path := writeConfFile(t, "directory ~/work alice\n")
		_, err := ParseConfFile(path)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("github rule missing colon returns error", func(t *testing.T) {
		path := writeConfFile(t, "github myorg alice\n")
		_, err := ParseConfFile(path)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("github rule with empty user returns error", func(t *testing.T) {
		path := writeConfFile(t, "github myorg:\n")
		_, err := ParseConfFile(path)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestParseOrgRepo(t *testing.T) {
	tests := []struct {
		name      string
		remoteURL string
		wantOrg   string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "https with .git suffix",
			remoteURL: "https://github.com/myorg/myrepo.git",
			wantOrg:   "myorg",
			wantRepo:  "myrepo",
		},
		{
			name:      "https without .git suffix",
			remoteURL: "https://github.com/myorg/myrepo",
			wantOrg:   "myorg",
			wantRepo:  "myrepo",
		},
		{
			name:      "http with .git suffix",
			remoteURL: "http://github.com/myorg/myrepo.git",
			wantOrg:   "myorg",
			wantRepo:  "myrepo",
		},
		{
			name:      "git@ with .git suffix",
			remoteURL: "git@github.com:myorg/myrepo.git",
			wantOrg:   "myorg",
			wantRepo:  "myrepo",
		},
		{
			name:      "git@ without .git suffix",
			remoteURL: "git@github.com:myorg/myrepo",
			wantOrg:   "myorg",
			wantRepo:  "myrepo",
		},
		{
			name:      "empty string",
			remoteURL: "",
			wantErr:   true,
		},
		{
			name:      "unsupported scheme",
			remoteURL: "ssh://git@github.com/org/repo.git",
			wantErr:   true,
		},
		{
			name:      "git@ missing colon",
			remoteURL: "git@github.com/org/repo.git",
			wantErr:   true,
		},
		{
			name:      "https missing repo",
			remoteURL: "https://github.com/onlyorg",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, repo, err := ParseOrgRepo(tt.remoteURL)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got org=%q repo=%q", org, repo)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if org != tt.wantOrg || repo != tt.wantRepo {
				t.Errorf("got org=%q repo=%q, want org=%q repo=%q", org, repo, tt.wantOrg, tt.wantRepo)
			}
		})
	}
}
