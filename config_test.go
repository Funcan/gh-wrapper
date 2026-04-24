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
