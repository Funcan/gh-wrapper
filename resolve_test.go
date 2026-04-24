package main

import (
	"os"
	"path/filepath"
	"testing"
)

// makeRepoDir creates a temp dir with a .git/config containing the given
// content and returns (dir, gitConfigPath).
func makeRepoDir(t *testing.T, content string) (dir string, gitConfigPath string) {
	t.Helper()
	dir = t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	gitConfigPath = filepath.Join(gitDir, "config")
	if err := os.WriteFile(gitConfigPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return dir, gitConfigPath
}

func TestResolveUser(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not get home dir: %v", err)
	}

	t.Run("no config, no rules returns empty string", func(t *testing.T) {
		got, err := ResolveUser("/some/dir", "", "", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	// --- Priority 1: git config ---

	t.Run("git config user takes priority over rules", func(t *testing.T) {
		_, cfgPath := makeRepoDir(t, "[gh-wrapper]\n\tuser = gituser\n")
		rules := []Rule{
			{Type: "github", Org: "", User: "catchall"},
		}
		got, err := ResolveUser("/any/dir", cfgPath, "", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "gituser" {
			t.Errorf("got %q, want %q", got, "gituser")
		}
	})

	t.Run("empty git config user falls through to rules", func(t *testing.T) {
		_, cfgPath := makeRepoDir(t, "[core]\n\tbare = false\n")
		rules := []Rule{
			{Type: "github", Org: "", User: "catchall"},
		}
		got, err := ResolveUser("/any/dir", cfgPath, "", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "catchall" {
			t.Errorf("got %q, want %q", got, "catchall")
		}
	})

	t.Run("git config path error is returned", func(t *testing.T) {
		_, err := ResolveUser("/any/dir", "/nonexistent/path/.git/config", "", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	// --- Directory rules ---

	t.Run("directory rule matches exact path", func(t *testing.T) {
		dir := t.TempDir()
		rules := []Rule{
			{Type: "directory", Path: dir, User: "diruser"},
		}
		got, err := ResolveUser(dir, "", "", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "diruser" {
			t.Errorf("got %q, want %q", got, "diruser")
		}
	})

	t.Run("directory rule matches subdirectory", func(t *testing.T) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "sub", "deep")
		rules := []Rule{
			{Type: "directory", Path: dir, User: "diruser"},
		}
		got, err := ResolveUser(sub, "", "", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "diruser" {
			t.Errorf("got %q, want %q", got, "diruser")
		}
	})

	t.Run("directory rule does not match sibling with shared prefix", func(t *testing.T) {
		// /tmp/work should not match /tmp/work2
		base := t.TempDir()
		dir := filepath.Join(base, "work")
		sibling := filepath.Join(base, "work2")
		rules := []Rule{
			{Type: "directory", Path: dir, User: "diruser"},
		}
		got, err := ResolveUser(sibling, "", "", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty string (sibling should not match)", got)
		}
	})

	t.Run("directory rule does not match unrelated path", func(t *testing.T) {
		rules := []Rule{
			{Type: "directory", Path: "/home/user/work", User: "diruser"},
		}
		got, err := ResolveUser("/home/other/project", "", "", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("directory rule with tilde expands to home dir", func(t *testing.T) {
		sub := filepath.Join(home, "myproject", "sub")
		rules := []Rule{
			{Type: "directory", Path: "~/myproject", User: "homeuser"},
		}
		got, err := ResolveUser(sub, "", "", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "homeuser" {
			t.Errorf("got %q, want %q", got, "homeuser")
		}
	})

	t.Run("directory catch-all (empty path) always matches", func(t *testing.T) {
		rules := []Rule{
			{Type: "directory", Path: "", User: "catchalldir"},
		}
		got, err := ResolveUser("/any/random/path", "", "", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "catchalldir" {
			t.Errorf("got %q, want %q", got, "catchalldir")
		}
	})

	// --- GitHub rules ---

	t.Run("github org/repo rule matches", func(t *testing.T) {
		rules := []Rule{
			{Type: "github", Org: "myorg", Repo: "myrepo", User: "repouser"},
		}
		got, err := ResolveUser("/any", "", "git@github.com:myorg/myrepo.git", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "repouser" {
			t.Errorf("got %q, want %q", got, "repouser")
		}
	})

	t.Run("github org/repo rule does not match wrong repo", func(t *testing.T) {
		rules := []Rule{
			{Type: "github", Org: "myorg", Repo: "myrepo", User: "repouser"},
		}
		got, err := ResolveUser("/any", "", "git@github.com:myorg/otherrepo.git", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("github org/repo rule does not match wrong org", func(t *testing.T) {
		rules := []Rule{
			{Type: "github", Org: "myorg", Repo: "myrepo", User: "repouser"},
		}
		got, err := ResolveUser("/any", "", "git@github.com:otherorg/myrepo.git", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("github org-only rule matches any repo in that org", func(t *testing.T) {
		rules := []Rule{
			{Type: "github", Org: "myorg", Repo: "", User: "orguser"},
		}
		got, err := ResolveUser("/any", "", "https://github.com/myorg/anyrepo.git", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "orguser" {
			t.Errorf("got %q, want %q", got, "orguser")
		}
	})

	t.Run("github org-only rule does not match different org", func(t *testing.T) {
		rules := []Rule{
			{Type: "github", Org: "myorg", Repo: "", User: "orguser"},
		}
		got, err := ResolveUser("/any", "", "https://github.com/otherorg/repo.git", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("github catch-all matches any remote", func(t *testing.T) {
		rules := []Rule{
			{Type: "github", Org: "", Repo: "", User: "defaultuser"},
		}
		got, err := ResolveUser("/any", "", "git@github.com:anything/repo.git", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "defaultuser" {
			t.Errorf("got %q, want %q", got, "defaultuser")
		}
	})

	t.Run("github catch-all matches even with no remote", func(t *testing.T) {
		rules := []Rule{
			{Type: "github", Org: "", Repo: "", User: "defaultuser"},
		}
		got, err := ResolveUser("/any", "", "", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "defaultuser" {
			t.Errorf("got %q, want %q", got, "defaultuser")
		}
	})

	t.Run("org-specific github rule skipped when remote is unparseable", func(t *testing.T) {
		rules := []Rule{
			{Type: "github", Org: "myorg", Repo: "", User: "orguser"},
		}
		got, err := ResolveUser("/any", "", "not-a-valid-url", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("org-specific rule skipped with bad remote, catch-all still matches", func(t *testing.T) {
		rules := []Rule{
			{Type: "github", Org: "myorg", Repo: "", User: "orguser"},
			{Type: "github", Org: "", Repo: "", User: "defaultuser"},
		}
		got, err := ResolveUser("/any", "", "not-a-valid-url", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "defaultuser" {
			t.Errorf("got %q, want %q", got, "defaultuser")
		}
	})

	t.Run("org-specific rule skipped with empty remote, catch-all still matches", func(t *testing.T) {
		rules := []Rule{
			{Type: "github", Org: "myorg", Repo: "", User: "orguser"},
			{Type: "github", Org: "", Repo: "", User: "defaultuser"},
		}
		got, err := ResolveUser("/any", "", "", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "defaultuser" {
			t.Errorf("got %q, want %q", got, "defaultuser")
		}
	})

	// --- Priority / ordering ---

	t.Run("first matching rule wins", func(t *testing.T) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "sub")
		rules := []Rule{
			{Type: "directory", Path: sub, User: "specific"},
			{Type: "directory", Path: dir, User: "parent"},
			{Type: "github", Org: "", User: "catchall"},
		}
		got, err := ResolveUser(sub, "", "", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "specific" {
			t.Errorf("got %q, want %q", got, "specific")
		}
	})

	t.Run("falls through to second rule when first does not match", func(t *testing.T) {
		rules := []Rule{
			{Type: "github", Org: "myorg", Repo: "myrepo", User: "repouser"},
			{Type: "github", Org: "myorg", Repo: "", User: "orguser"},
		}
		// Remote is myorg/otherrepo — first rule won't match, second should.
		got, err := ResolveUser("/any", "", "git@github.com:myorg/otherrepo.git", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "orguser" {
			t.Errorf("got %q, want %q", got, "orguser")
		}
	})

	t.Run("full priority: git config beats rules", func(t *testing.T) {
		_, cfgPath := makeRepoDir(t, "[gh-wrapper]\n\tuser = gitcfguser\n")
		dir := t.TempDir()
		rules := []Rule{
			{Type: "directory", Path: dir, User: "diruser"},
			{Type: "github", Org: "", User: "catchall"},
		}
		got, err := ResolveUser(dir, cfgPath, "", rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "gitcfguser" {
			t.Errorf("got %q, want %q", got, "gitcfguser")
		}
	})
}

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not get home dir: %v", err)
	}

	tests := []struct {
		input string
		want  string
	}{
		{"~", home},
		{"~/foo/bar", home + "/foo/bar"},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~otheruser/foo", "~otheruser/foo"}, // not expanded
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := expandTilde(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("expandTilde(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
