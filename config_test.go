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
