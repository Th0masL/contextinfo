package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// newRepo creates a temporary git repository on branch "main" with one committed
// file and an scp-style origin remote, and returns its path. It skips the test
// if git is missing.
func newRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "t@e.com")
	runGit(t, dir, "config", "user.name", "T")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	runGit(t, dir, "config", "tag.gpgsign", "false")
	runGit(t, dir, "remote", "add", "origin", "git@github.com:acme/widgets.git")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "init")
	runGit(t, dir, "branch", "-M", "main") // deterministic branch name across git versions
	return dir
}

// runGit runs a git command in dir and fails the test on a non-zero exit.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// On a real repo the helpers report the commit, branch, clean/dirty state, origin
// remote, and tag. They take the repo dir, so no chdir is needed.
func TestGitOnRepo(t *testing.T) {
	dir := newRepo(t)
	ctx := context.Background()

	if !InRepo(ctx, dir) {
		t.Error("InRepo should be true inside a work tree")
	}
	if sha := Output(ctx, dir, "rev-parse", "HEAD"); len(sha) != 40 {
		t.Errorf("sha = %q (len %d), want 40 hex chars", sha, len(sha))
	}
	if b, src := Branch(ctx, dir, "", ""); b != "main" || src != "git symbolic-ref --short HEAD" {
		t.Errorf("branch = %q (source %q), want main from symbolic-ref", b, src)
	}
	if Dirty(ctx, dir) {
		t.Error("expected a clean tree")
	}
	if got := RemoteURL(ctx, dir); got != "git@github.com:acme/widgets.git" {
		t.Errorf("remote = %q", got)
	}
	if got := Output(ctx, dir, "describe", "--tags", "--exact-match"); got != "" {
		t.Errorf("unexpected tag %q", got)
	}

	runGit(t, dir, "tag", "v1.0.0")
	if got := Output(ctx, dir, "describe", "--tags", "--exact-match"); got != "v1.0.0" {
		t.Errorf("tag = %q, want v1.0.0", got)
	}

	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !Dirty(ctx, dir) {
		t.Error("expected a dirty tree after modification")
	}
}

// Commit returns HEAD's sha, parent count, and subject; parent count >= 2 marks a
// merge commit.
func TestCommit(t *testing.T) {
	dir := newRepo(t)
	ctx := context.Background()

	sha, parents, subject := Commit(ctx, dir)
	if len(sha) != 40 {
		t.Errorf("sha = %q, want 40 hex chars", sha)
	}
	if parents != 0 {
		t.Errorf("parents = %d, want 0 (root commit)", parents)
	}
	if subject != "init" {
		t.Errorf("subject = %q, want init", subject)
	}

	// Build a merge commit; HEAD should then have two parents.
	runGit(t, dir, "checkout", "-q", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "g.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "feature work")
	runGit(t, dir, "checkout", "-q", "main")
	runGit(t, dir, "merge", "--no-ff", "-m", "Merge feature", "feature")

	_, parents, subject = Commit(ctx, dir)
	if parents != 2 {
		t.Errorf("after merge: parents = %d, want 2", parents)
	}
	if subject != "Merge feature" {
		t.Errorf("subject = %q, want \"Merge feature\"", subject)
	}
}

// On a detached HEAD (as in CI) symbolic-ref fails, so Branch returns the supplied
// hint, or "" when there is none.
func TestBranchDetachedUsesHint(t *testing.T) {
	dir := newRepo(t)
	runGit(t, dir, "checkout", "--detach") // mimic a CI checkout
	ctx := context.Background()

	if b, src := Branch(ctx, dir, "feature/x", "CI hint"); b != "feature/x" || src != "CI hint" {
		t.Errorf("detached: branch = %q (source %q), want hint feature/x labelled 'CI hint'", b, src)
	}
	if b, _ := Branch(ctx, dir, "", ""); b != "" {
		t.Errorf("detached without hint: branch = %q, want empty", b)
	}
}

// Outside a repository the helpers return empty/false rather than erroring.
func TestGitOutsideRepo(t *testing.T) {
	dir := t.TempDir() // not a git repo
	ctx := context.Background()
	if InRepo(ctx, dir) {
		t.Error("InRepo should be false outside a repo")
	}
	if sha := Output(ctx, dir, "rev-parse", "HEAD"); sha != "" {
		t.Errorf("sha outside a repo = %q, want empty", sha)
	}
	if Dirty(ctx, dir) {
		t.Error("dirty outside a repo should be false")
	}
	if r := RemoteURL(ctx, dir); r != "" {
		t.Errorf("remote outside a repo = %q, want empty", r)
	}
}
