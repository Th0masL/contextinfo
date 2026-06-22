package contextinfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// getter returns a getenv-style lookup backed by m (missing keys yield "").
func getter(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// newRepo creates a temporary git repository on branch "main" with one committed
// file and an scp-style origin remote, and returns its path. It skips the test
// if git is missing. Used by the detect()/explain integration tests.
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
